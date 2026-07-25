//go:build windows

package driver

import "fmt"

// Click moves the real mouse cursor to (x, y) and left-clicks, via Win32
// (SetForegroundWindow + SetCursorPos + mouse_event — see clickAtScreenPos
// in window_windows.go for why this goes around WinAppDriver's own click
// machinery entirely rather than through it). x/y are absolute screen
// coordinates — e2e/helpers.ClickLogical is what converts kag3's 1280x720
// logical tag coordinates into this space using Session.WindowRect's
// client-area origin.
func (s *Session) Click(x, y int) error {
	if err := clickAtScreenPos(s.hwnd, x, y); err != nil {
		return fmt.Errorf("click at (%d,%d): %w", x, y, err)
	}
	return nil
}

// KeyPress sends a single key down+up via Win32 keybd_event — see pressKey
// in window_windows.go for why this goes around WinAppDriver's own /keys
// endpoint. key must be one of the names in virtualKeyCodes
// (window_windows.go). Delivered to whatever window currently has
// keyboard focus, which Click leaves as the session window (via
// SetForegroundWindow) — call Click at least once before relying on
// KeyPress reaching the right window.
func (s *Session) KeyPress(key string) error {
	if err := pressKey(key); err != nil {
		return fmt.Errorf("key press %q: %w", key, err)
	}
	return nil
}
