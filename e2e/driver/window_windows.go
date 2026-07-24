//go:build windows

package driver

import (
	"fmt"
	"image"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procClientToScreen           = user32.NewProc("ClientToScreen")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
)

type win32Rect struct {
	Left, Top, Right, Bottom int32
}

type win32Point struct {
	X, Y int32
}

// findWindowByPID returns the handle of the first visible top-level window
// owned by pid, found via EnumWindows + GetWindowThreadProcessId (there's
// no direct "windows for this PID" Win32 call). ebitengine apps only ever
// have one such window, so the first match is unambiguous.
func findWindowByPID(pid uint32) (uintptr, error) {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var wndPID uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wndPID)))
		if wndPID != pid {
			return 1 // continue enumeration
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		found = hwnd
		return 0 // stop enumeration
	})
	if ret, _, err := procEnumWindows.Call(cb, 0); ret == 0 && found == 0 {
		// EnumWindows returning FALSE with no window found is a real
		// failure; returning FALSE because our callback stopped it early
		// (found != 0) is the expected/success path.
		return 0, fmt.Errorf("EnumWindows failed: %w", err)
	}
	if found == 0 {
		return 0, fmt.Errorf("no visible window found for pid %d", pid)
	}
	return found, nil
}

// waitForWindowByPID polls findWindowByPID until a window appears or
// timeout elapses — ebitengine apps take a noticeable moment (embedded FS
// decode, font load) between process start and the first window showing up.
func waitForWindowByPID(pid uint32, timeout time.Duration) (uintptr, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		hwnd, err := findWindowByPID(pid)
		if err == nil {
			return hwnd, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return 0, fmt.Errorf("no window appeared for pid %d within %s: %w", pid, timeout, lastErr)
}

// clientRectOnScreen returns hwnd's client area (the actual drawable
// surface, excluding title bar/borders) in screen coordinates. This is a
// real measurement (GetClientRect + ClientToScreen), not a value computed
// from the window's outer rect and assumed border widths — it's correct
// regardless of DPI scaling or window chrome.
func clientRectOnScreen(hwnd uintptr) (image.Rectangle, error) {
	var r win32Rect
	if ret, _, err := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return image.Rectangle{}, fmt.Errorf("GetClientRect failed: %w", err)
	}
	origin := win32Point{0, 0}
	if ret, _, err := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin))); ret == 0 {
		return image.Rectangle{}, fmt.Errorf("ClientToScreen failed: %w", err)
	}
	w := int(r.Right - r.Left)
	h := int(r.Bottom - r.Top)
	return image.Rect(int(origin.X), int(origin.Y), int(origin.X)+w, int(origin.Y)+h), nil
}
