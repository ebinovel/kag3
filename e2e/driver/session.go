//go:build windows

// Package driver is a minimal WinAppDriver (W3C WebDriver Protocol) HTTP
// client, scoped to exactly what's needed to drive an ebitengine window:
// attach to a running process's window, click/key-press, and screenshot.
// ebitengine apps have no UI Automation tree worth speaking of (a single
// canvas), so element-based WebDriver calls (FindElement etc.) are
// deliberately not implemented here.
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
type Session struct {
	ID      string
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
func AttachSession(hwnd uintptr) (*Session, error) {
	s := &Session{baseURL: DefaultBaseURL, client: &http.Client{Timeout: 30 * time.Second}}
	body := map[string]any{
		"capabilities": map[string]any{
			"alwaysMatch": map[string]any{
				"platformName":             "windows",
				"appium:appTopLevelWindow": fmt.Sprintf("%X", hwnd),
			},
		},
	}
	var resp struct {
		Value struct {
			SessionID string `json:"sessionId"`
			Error     string `json:"error"`
			Message   string `json:"message"`
		} `json:"value"`
	}
	if err := s.post("/session", body, &resp); err != nil {
		return nil, fmt.Errorf("creating WinAppDriver session: %w", err)
	}
	if resp.Value.SessionID == "" {
		return nil, fmt.Errorf("WinAppDriver returned no sessionId (error=%q message=%q) — is WinAppDriver.exe running?", resp.Value.Error, resp.Value.Message)
	}
	s.ID = resp.Value.SessionID
	return s, nil
}

// Close ends the WinAppDriver session and, if this Session started its own
// process (via NewSession), kills it. Safe to call on a Session whose
// session creation failed partway (nil ID is skipped).
func (s *Session) Close() error {
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
	hwnd, err := s.topLevelWindow()
	if err != nil {
		return image.Rectangle{}, err
	}
	return clientRectOnScreen(hwnd)
}

// topLevelWindow re-derives the Win32 window handle WinAppDriver attached
// to. WinAppDriver's own /window/rect endpoint reports the *outer* window
// rect (title bar + borders included), which is the wrong reference frame
// for converting kag3's client-area logical coordinates — so WindowRect
// above deliberately goes through clientRectOnScreen (Win32
// GetClientRect+ClientToScreen) instead of WinAppDriver's endpoint. This
// means the session doesn't otherwise need to track the handle after
// attach; re-fetching it via GET /session/{id}/window costs one extra
// round trip but avoids a second source of truth.
func (s *Session) topLevelWindow() (uintptr, error) {
	var resp struct {
		Value string `json:"value"`
	}
	if err := s.get(fmt.Sprintf("/session/%s/window", s.ID), &resp); err != nil {
		return 0, err
	}
	var hwnd uintptr
	if _, err := fmt.Sscanf(resp.Value, "%X", &hwnd); err != nil {
		return 0, fmt.Errorf("parsing window handle %q: %w", resp.Value, err)
	}
	return hwnd, nil
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
