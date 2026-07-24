//go:build windows

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// winAppDriverPath is WinAppDriver's default install location
// (github.com/microsoft/WinAppDriver's .msi installer). Override by
// setting WINAPPDRIVER_PATH if installed elsewhere.
const winAppDriverPath = `C:\Program Files (x86)\Windows Application Driver\WinAppDriver.exe`

// TestMain ensures a WinAppDriver instance is listening before any flow
// test runs, starting one from winAppDriverPath if nothing's already up on
// its default port. Only a WinAppDriver process this TestMain itself
// started is killed on exit — one already running (e.g. launched manually
// by the person running the tests) is left alone.
func TestMain(m *testing.M) {
	proc, err := ensureWinAppDriver()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: WinAppDriver:", err)
		os.Exit(1)
	}
	code := m.Run()
	if proc != nil {
		_ = proc.Kill()
	}
	os.Exit(code)
}

func ensureWinAppDriver() (*os.Process, error) {
	if winAppDriverListening() {
		return nil, nil
	}
	path := winAppDriverPath
	if v := os.Getenv("WINAPPDRIVER_PATH"); v != "" {
		path = v
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("WinAppDriver not running and not found at %s — install it (https://github.com/microsoft/WinAppDriver/releases), start it manually, or set WINAPPDRIVER_PATH: %w", path, err)
	}
	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting WinAppDriver at %s: %w", path, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if winAppDriverListening() {
			return cmd.Process, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	return nil, fmt.Errorf("WinAppDriver started at %s but never listened on %s within 10s", path, "http://127.0.0.1:4723")
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
