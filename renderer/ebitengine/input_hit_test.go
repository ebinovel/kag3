package ebitengine

import "testing"

// TestIsColisionTouchPadsOnlyWhenTouch is the coverage for the Android
// "hard to tap" fix: a point just outside a small rect must miss when
// touch=false (unchanged mouse behavior) but hit when touch=true, within
// touchHitPadding px of the rect's edge.
func TestIsColisionTouchPadsOnlyWhenTouch(t *testing.T) {
	// A tiny rect similar to the operation row's real dimensions
	// (opRowDividerH = 22, tags_oprow.go) — small enough that a real
	// fingertip routinely lands just outside it.
	x, y, w, h := 100, 100, 10, 10

	justOutsideRight := x + w + touchHitPadding/2
	if isColisionTouch(justOutsideRight, y, x, y, w, h, false) {
		t.Errorf("isColisionTouch(touch=false) hit %dpx outside the rect, want a miss (mouse clicks must stay exact)", touchHitPadding/2)
	}
	if !isColisionTouch(justOutsideRight, y, x, y, w, h, true) {
		t.Errorf("isColisionTouch(touch=true) missed %dpx outside the rect, want a hit within touchHitPadding=%d", touchHitPadding/2, touchHitPadding)
	}

	wayOutsideRight := x + w + touchHitPadding + 1
	if isColisionTouch(wayOutsideRight, y, x, y, w, h, true) {
		t.Errorf("isColisionTouch(touch=true) hit %dpx outside the rect, want a miss beyond touchHitPadding=%d", touchHitPadding+1, touchHitPadding)
	}

	// A point already inside the rect must hit regardless of touch, and
	// isColisionTouch(touch=false) must behave exactly like isColision.
	if !isColisionTouch(x+w/2, y+h/2, x, y, w, h, false) {
		t.Error("isColisionTouch(touch=false) missed a point already inside the rect")
	}
	if !isColisionTouch(x+w/2, y+h/2, x, y, w, h, true) {
		t.Error("isColisionTouch(touch=true) missed a point already inside the rect")
	}
}
