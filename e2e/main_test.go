//go:build windows

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// winAppDriverRelPath is WinAppDriver's path below whichever Program Files
// tree its .msi installer (github.com/microsoft/WinAppDriver) put it in.
const winAppDriverRelPath = `Windows Application Driver\WinAppDriver.exe`

// winAppDriverCandidates lists the install locations to probe, in order.
// The installer's documented default is the 32-bit Program Files tree, but
// it has been observed installing into the 64-bit one instead (confirmed on
// this project's own Windows dev machine, where only
// "C:\Program Files\Windows Application Driver" existed) — so both are
// tried rather than only the documented default, which otherwise fails with
// a "not found" that looks like WinAppDriver isn't installed at all when it
// plainly is. Built from the ProgramFiles/ProgramFiles(x86) environment
// variables so a non-C: Windows install still resolves, with the
// conventional absolute paths appended as a last resort in case neither var
// is set. WINAPPDRIVER_PATH (see findWinAppDriver) skips this search
// entirely.
func winAppDriverCandidates() []string {
	var paths []string
	seen := map[string]bool{}
	add := func(base string) {
		if base == "" {
			return
		}
		p := filepath.Join(base, winAppDriverRelPath)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	add(os.Getenv("ProgramFiles(x86)"))
	add(os.Getenv("ProgramFiles"))
	add(`C:\Program Files (x86)`)
	add(`C:\Program Files`)
	return paths
}

// findWinAppDriver resolves WinAppDriver.exe: WINAPPDRIVER_PATH if set
// (reported as an error of its own if it points at nothing, rather than
// silently falling back — an explicitly configured path that's wrong is a
// mistake worth surfacing), otherwise the first winAppDriverCandidates entry
// that exists.
func findWinAppDriver() (string, error) {
	if v := os.Getenv("WINAPPDRIVER_PATH"); v != "" {
		if _, err := os.Stat(v); err != nil {
			return "", fmt.Errorf("WINAPPDRIVER_PATH=%s does not exist: %w", v, err)
		}
		return v, nil
	}
	tried := winAppDriverCandidates()
	for _, p := range tried {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("WinAppDriver not running and not found in any of %v — install it (https://github.com/microsoft/WinAppDriver/releases), start it manually, or set WINAPPDRIVER_PATH", tried)
}

// startedWinAppDriver is a WinAppDriver process this TestMain launched
// itself, bundled with the stdin pipe that has to stay open for the whole
// run — see ensureWinAppDriver for why. Both fields must outlive m.Run():
// if the write end of the pipe were garbage collected, its finalizer would
// close the handle and WinAppDriver would exit mid-run, so TestMain holds
// this value across the call rather than just the *os.Process.
type startedWinAppDriver struct {
	cmd    *exec.Cmd
	stdinW *os.File
}

func (w *startedWinAppDriver) stop() {
	if w == nil {
		return
	}
	_ = w.cmd.Process.Kill()
	_ = w.stdinW.Close()
}

// TestMain ensures a WinAppDriver instance is listening before any flow
// test runs, starting one from findWinAppDriver's resolved path if nothing's
// already up on its default port. Only a WinAppDriver process this TestMain
// itself started is killed on exit — one already running (e.g. launched
// manually by the person running the tests) is left alone.
func TestMain(m *testing.M) {
	started, err := ensureWinAppDriver()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: WinAppDriver:", err)
		os.Exit(1)
	}
	code := m.Run()
	started.stop()
	os.Exit(code)
}

func ensureWinAppDriver() (*startedWinAppDriver, error) {
	if winAppDriverListening() {
		return nil, nil
	}
	path, err := findWinAppDriver()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path)
	// WinAppDriver's console loop prints "Press ENTER to exit." and quits the
	// instant its stdin reports EOF. os/exec hands a child with a nil Stdin
	// the null device, which reads EOF immediately — a WinAppDriver started
	// that way binds its port, prints "listening", and exits again within
	// milliseconds, leaving the poll below to time out and report the
	// thoroughly misleading "started ... but never listened". Handing it the
	// read end of a pipe nothing ever writes to or closes keeps that read
	// blocked for as long as this test binary lives (confirmed: the same
	// launch works when stdin is a real console, and dies when it's NUL).
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating WinAppDriver stdin pipe: %w", err)
	}
	cmd.Stdin = pr
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, fmt.Errorf("starting WinAppDriver at %s: %w", path, err)
	}
	// The child holds its own copy of the read end now; this process doesn't
	// need it. pw deliberately stays open (see startedWinAppDriver).
	pr.Close()
	started := &startedWinAppDriver{cmd: cmd, stdinW: pw}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if winAppDriverListening() {
			return started, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	started.stop()
	return nil, fmt.Errorf("WinAppDriver started at %s but never listened on %s within 10s — if it exited immediately, check that Windows Developer Mode is enabled (WinAppDriver refuses to initialize without it)", path, "http://127.0.0.1:4723")
}

func winAppDriverListening() bool {
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:4723/status")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}
