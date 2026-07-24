//go:build windows

package driver

import "fmt"

// Click performs a left-click at (x, y) using the W3C WebDriver Actions
// API. x/y are interpreted by WinAppDriver relative to the attached
// window's client area origin (top-left = 0,0) — NOT absolute screen
// coordinates. e2e/helpers.ClickLogical is what converts kag3's 1280x720
// logical tag coordinates into this space.
func (s *Session) Click(x, y int) error {
	body := map[string]any{
		"actions": []any{
			map[string]any{
				"type":       "pointer",
				"id":         "mouse1",
				"parameters": map[string]any{"pointerType": "mouse"},
				"actions": []any{
					map[string]any{"type": "pointerMove", "duration": 0, "x": x, "y": y, "origin": "viewport"},
					map[string]any{"type": "pointerDown", "button": 0},
					map[string]any{"type": "pause", "duration": 50},
					map[string]any{"type": "pointerUp", "button": 0},
				},
			},
		},
	}
	if err := s.post(fmt.Sprintf("/session/%s/actions", s.ID), body, nil); err != nil {
		return fmt.Errorf("click at (%d,%d): %w", x, y, err)
	}
	return nil
}

// webDriverKeyCodes maps the handful of keys e2e tests need to their W3C
// WebDriver "normalized key value" — Unicode Private Use Area code points
// defined by the WebDriver spec
// (https://www.w3.org/TR/webdriver2/#keyboard-actions), not the literal
// key label.
var webDriverKeyCodes = map[string]string{
	"Enter":  "\uE007",
	"Escape": "\uE00C",
	"Space":  "\uE00D",
	"Tab":    "\uE004",
}

// KeyPress sends a single key down+up. key must be one of the names in
// webDriverKeyCodes.
func (s *Session) KeyPress(key string) error {
	code, ok := webDriverKeyCodes[key]
	if !ok {
		return fmt.Errorf("unknown key %q (add it to webDriverKeyCodes if needed)", key)
	}
	body := map[string]any{
		"actions": []any{
			map[string]any{
				"type": "key",
				"id":   "keyboard1",
				"actions": []any{
					map[string]any{"type": "keyDown", "value": code},
					map[string]any{"type": "pause", "duration": 30},
					map[string]any{"type": "keyUp", "value": code},
				},
			},
		},
	}
	if err := s.post(fmt.Sprintf("/session/%s/actions", s.ID), body, nil); err != nil {
		return fmt.Errorf("key press %q: %w", key, err)
	}
	return nil
}
