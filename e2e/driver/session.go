//go:build windows

// Package driver is a minimal WinAppDriver HTTP client, scoped to exactly
// what's needed to drive an ebitengine window: attach to a running
// process's window, click/key-press, and screenshot. ebitengine apps have
// no UI Automation tree worth speaking of (a single canvas), so
// element-based WebDriver calls (FindElement etc.) are deliberately not
// implemented here.
//
// WinAppDriver speaks the legacy Selenium/Appium JSON Wire Protocol
// (request: {"desiredCapabilities":{...}}, response:
// {"sessionId":...,"status":...,"value":...} with sessionId top-level),
// not W3C WebDriver, despite some of its own docs showing W3C-shaped
// examples — confirmed empirically against a real WinAppDriver 1.x
// instance; see AttachSession's doc comment for specifics.
package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"os/exec"
	"time"
)

// DefaultBaseURL is WinAppDriver's default listen address.
const DefaultBaseURL = "http://127.0.0.1:4723"

// Session is one attached WinAppDriver session against a single window.
// Click/KeyPress go straight to Win32 (see window_windows.go) rather than
// through WinAppDriver — its own input-delivery endpoints don't actually
// work in this environment; hwnd is what makes that possible without a
// round trip through WinAppDriver's own (equally non-functional) window
// handle lookup.
type Session struct {
	ID      string
	hwnd    uintptr
	baseURL string
	client  *http.Client
	// cmd is set only when this Session started its own process (see
	// NewSession) — Close() kills it. AttachSession leaves this nil so
	// Close() never kills a window the caller didn't ask us to own.
	cmd *exec.Cmd
}

// NewSession starts appPath as a child process with env, waits for its
// main window to appear, and attaches a WinAppDriver session to that
// window via the appium:appTopLevelWindow capability. Starting the process
// ourselves (rather than letting WinAppDriver launch it via appium:app) is
// what lets the caller set environment variables — WinAppDriver's launch
// capabilities don't include one for that.
func NewSession(appPath string, env []string) (*Session, error) {
	cmd := exec.Command(appPath)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", appPath, err)
	}

	hwnd, err := waitForWindowByPID(uint32(cmd.Process.Pid), 15*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	s, err := AttachSession(hwnd)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	s.cmd = cmd
	return s, nil
}

// AttachSession creates a WinAppDriver session against an already-running
// window, identified by its Win32 handle. Close() on the returned Session
// will not kill the underlying process — the caller owns its lifetime.
//
// WinAppDriver speaks the legacy JSON Wire Protocol, not W3C WebDriver,
// despite its docs' capability examples looking W3C-shaped — confirmed
// empirically (probing http://127.0.0.1:4723/session directly): a W3C
// {"capabilities":{"alwaysMatch":{...}}} body gets rejected with "Bad
// capabilities. Specify either app or appTopLevelWindow to create a
// session" even when appTopLevelWindow is right there in alwaysMatch, but
// {"desiredCapabilities":{...}} (flat, no alwaysMatch wrapper) is accepted.
// The success response is also JSON Wire Protocol shaped:
// {"sessionId":"...","status":0,"value":{...}} — sessionId is top-level,
// not nested under value like a W3C response would put it.
func AttachSession(hwnd uintptr) (*Session, error) {
	s := &Session{hwnd: hwnd, baseURL: DefaultBaseURL, client: &http.Client{Timeout: 30 * time.Second}}
	body := map[string]any{
		"desiredCapabilities": map[string]any{
			"platformName":      "windows",
			"appTopLevelWindow": fmt.Sprintf("%X", hwnd),
		},
	}
	var resp struct {
		SessionID string `json:"sessionId"`
		Status    int    `json:"status"`
		Value     struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		} `json:"value"`
	}
	if err := s.post("/session", body, &resp); err != nil {
		return nil, fmt.Errorf("creating WinAppDriver session: %w", err)
	}
	if resp.SessionID == "" {
		return nil, fmt.Errorf("WinAppDriver returned no sessionId (status=%d error=%q message=%q) — is WinAppDriver.exe running?", resp.Status, resp.Value.Error, resp.Value.Message)
	}
	s.ID = resp.SessionID
	// Exactly once, not on every click — see bringToForeground's doc
	// comment (window_windows.go) for why repeating this breaks focus
	// instead of ensuring it.
	bringToForeground(hwnd)
	return s, nil
}

// Close ends the WinAppDriver session and, if this Session started its own
// process (via NewSession), kills it. Safe to call on a Session whose
// session creation failed partway (nil ID is skipped). Also undoes
// bringToForeground's always-on-top pin (best-effort — harmless if the
// window's already gone by the time this runs).
func (s *Session) Close() error {
	if s.hwnd != 0 {
		unforeground(s.hwnd)
	}
	var err error
	if s.ID != "" {
		err = s.delete(fmt.Sprintf("/session/%s", s.ID))
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
	return err
}

// WindowRect returns the session window's client area (the actual drawable
// surface — see clientRectOnScreen) in screen coordinates. Used by
// e2e/helpers to convert kag3's 1280x720 logical tag coordinates
// (button x=/y=) into real screen coordinates for Click.
func (s *Session) WindowRect() (image.Rectangle, error) {
	return clientRectOnScreen(s.hwnd)
}

func (s *Session) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	return s.do(req, out)
}

func (s *Session) post(path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, s.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.do(req, out)
}

func (s *Session) delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	return s.do(req, nil)
}

func (s *Session) do(req *http.Request, out any) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s %s: decoding response: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
