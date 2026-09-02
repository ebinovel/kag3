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
	procSetCursorPos             = user32.NewProc("SetCursorPos")
	procMouseEvent               = user32.NewProc("mouse_event")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
)

// SetWindowPos z-order constants/flags (winuser.h). hwndTopmost/
// hwndNotopmost are sentinel HWND values, not real handles.
const (
	hwndTopmost   = ^uintptr(0) // (HWND)-1
	hwndNotopmost = ^uintptr(1) // (HWND)-2
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001
	swpShowWindow = 0x0040
)

// mouse_event/keybd_event flag and virtual-key constants (winuser.h).
const (
	mouseEventFLeftDown = 0x0002
	mouseEventFLeftUp   = 0x0004
	keyEventFKeyUp      = 0x0002
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

// bringToForeground pins hwnd as an always-on-top window
// (SetWindowPos(HWND_TOPMOST)), not just SetForegroundWindow.
//
// Why: on a real multi-window desktop (confirmed empirically against the
// actual machine this runs on — a normal dev desktop with a terminal,
// editor, browser, etc. all open), whatever process launched the test
// (e.g. a terminal window running `go test`) is very often *already*
// covering the same screen region the newly-launched kag3 window occupies
// — EnumWindows' Z-order confirmed a full-screen-sized terminal window
// sitting above kag3 in the stack. SetCursorPos+mouse_event delivers a
// click to whatever window is topmost at that screen position, not to
// whichever window merely has keyboard focus — so a click "at kag3's
// button" lands on the terminal sitting on top of it instead, making that
// terminal jump to the front. Plain SetForegroundWindow does not change
// Z-order, only focus, so it doesn't fix this. HWND_TOPMOST does. See
// unforeground for the matching cleanup.
func bringToForeground(hwnd uintptr) {
	procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	time.Sleep(200 * time.Millisecond)
}

// unforeground undoes bringToForeground's topmost pin. Best-effort: called
// from Session.Close() after the underlying process may already be gone,
// in which case this is a harmless no-op (the OS has already destroyed the
// window).
func unforeground(hwnd uintptr) {
	procSetWindowPos.Call(hwnd, hwndNotopmost, 0, 0, 0, 0, swpNoMove|swpNoSize)
}

// clickAtScreenPos left-clicks at an absolute screen coordinate via
// SetCursorPos + mouse_event(LEFTDOWN/LEFTUP).
//
// This deliberately bypasses WinAppDriver's own click machinery (its
// POST .../actions endpoint rejects a mouse pointer source outright —
// "Currently only pen and touch pointer input source types are
// supported" — and its JSON Wire Protocol fallback, POST .../moveto, was
// confirmed empirically to move the cursor by some fraction of the
// requested offset rather than to an absolute position, both in this
// environment). SetCursorPos + mouse_event is what actually moves the
// real cursor and delivers a real click here — confirmed empirically
// against a running kag3 window: SendInput's MOUSEEVENTF_ABSOLUTE path
// was tried first and did *not* move the cursor at all in this
// environment, while SetCursorPos does.
func clickAtScreenPos(hwnd uintptr, x, y int) error {
	if ret, _, err := procSetCursorPos.Call(uintptr(int32(x)), uintptr(int32(y))); ret == 0 {
		return fmt.Errorf("SetCursorPos(%d,%d) failed: %w", x, y, err)
	}
	time.Sleep(50 * time.Millisecond)
	procMouseEvent.Call(mouseEventFLeftDown, 0, 0, 0, 0)
	time.Sleep(50 * time.Millisecond)
	procMouseEvent.Call(mouseEventFLeftUp, 0, 0, 0, 0)
	return nil
}

// virtualKeyCodes maps the handful of keys e2e tests need to their Win32
// virtual-key code (winuser.h VK_* constants).
var virtualKeyCodes = map[string]uintptr{
	"Enter":  0x0D, // VK_RETURN
	"Escape": 0x1B, // VK_ESCAPE
	"Space":  0x20, // VK_SPACE
	"Tab":    0x09, // VK_TAB
}

// pressKey sends a key down+up via keybd_event — same rationale as
// clickAtScreenPos: WinAppDriver's JSON Wire Protocol POST .../keys
// endpoint returns 200 OK but was confirmed empirically to not actually
// deliver the keystroke to the target window in this environment (screen
// content never changed across repeated calls), while keybd_event does.
// hwnd must already be foreground (see clickAtScreenPos) — keybd_event
// has no window-targeting parameter of its own, it goes to whatever
// currently has keyboard focus.
func pressKey(key string) error {
	vk, ok := virtualKeyCodes[key]
	if !ok {
		return fmt.Errorf("unknown key %q (add it to virtualKeyCodes if needed)", key)
	}
	procKeybdEvent.Call(vk, 0, 0, 0)
	time.Sleep(50 * time.Millisecond)
	procKeybdEvent.Call(vk, 0, keyEventFKeyUp, 0)
	return nil
}
