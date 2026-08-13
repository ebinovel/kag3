package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestHandleLinkCollectsTextsUntilEndlink is the ordinary, well-formed case —
// kept alongside the malformed one below so the bound added to handleLink's
// scan can't quietly break normal [link]...[endlink] parsing.
func TestHandleLinkCollectsTextsUntilEndlink(t *testing.T) {
	defer func() { links = nil }()
	links = nil
	r := newTestRenderer()
	r.scripts = []any{
		kag3.TagObject{Name: "link", Pm: map[string]string{"target": "*a"}},
		kag3.TextObject{Line: 1, Name: "text", Val: "choice"},
		kag3.TagObject{Name: "endlink"},
		kag3.TextObject{Line: 3, Name: "text", Val: "after"},
	}
	i := 0
	if err := r.execItem(fakeYield(), r.scripts, &i, 0); err != nil {
		t.Fatalf("execItem error: %v", err)
	}
	if i != 2 {
		t.Errorf("i = %d, want 2 ([endlink]'s own index, so the enclosing loop resumes after it)", i)
	}
	if len(links) != 1 || len(links[0].Texts) != 1 || links[0].Texts[0].Val != "choice" {
		t.Errorf("links = %+v, want one link carrying just the \"choice\" text", links)
	}
}

// TestHandleLinkWithoutEndlinkStopsAtScriptEnd is the regression test for a
// [link] whose [endlink] is missing entirely: handleLink's scan used to be an
// unbounded `for {}` that walked off the end of r.scripts and panicked with
// an index-out-of-range, killing the whole game over one unclosed tag.
func TestHandleLinkWithoutEndlinkStopsAtScriptEnd(t *testing.T) {
	defer func() { links = nil }()
	links = nil
	r := newTestRenderer()
	r.scripts = []any{
		kag3.TagObject{Name: "link", Pm: map[string]string{"target": "*a"}},
		kag3.TextObject{Line: 1, Name: "text", Val: "choice"},
	}
	i := 0
	// A panic here fails the test on its own; the assertions below cover the
	// "stopped somewhere sensible" half.
	if err := r.execItem(fakeYield(), r.scripts, &i, 0); err != nil {
		t.Fatalf("execItem error: %v", err)
	}
	if i != len(r.scripts)-1 {
		t.Errorf("i = %d, want %d (last item, so the enclosing loop's increment ends the script)", i, len(r.scripts)-1)
	}
	if len(links) != 1 || len(links[0].Texts) != 1 {
		t.Errorf("links = %+v, want the one link with the text collected before the script ran out", links)
	}
}

// TestHandleButtonWithoutGraphicOrSize covers a [button] carrying neither
// graphic= nor width=/height= — an invisible hit zone, the same shape
// [clickable] registers. handleButton's "no size given, use the graphic's
// own" fallback used to dereference a nil Graphic and crash.
func TestHandleButtonWithoutGraphicOrSize(t *testing.T) {
	defer func() { buttons = nil }()
	buttons = nil
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "button", Pm: map[string]string{"target": "*somewhere"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(buttons) != 1 {
		t.Fatalf("buttons = %+v, want the button registered", buttons)
	}
	if buttons[0].Width != 0 || buttons[0].Height != 0 {
		t.Errorf("button size = %dx%d, want 0x0 (no graphic to measure, no size given)",
			buttons[0].Width, buttons[0].Height)
	}
}

// TestButtonHitTestable is the other half of the sizeless-button fix above:
// hitButtons must skip a zero-area button entirely, because isColision's
// inclusive bounds mean a 0x0 rect still matches its own top-left corner
// (a 32x32 tap zone once isColisionTouch's touch padding applies) — an
// invisible click trap at the screen origin.
func TestButtonHitTestable(t *testing.T) {
	// Guard the premise: if isColision ever stops matching a 0x0 rect at its
	// own corner, this skip becomes redundant rather than load-bearing, and
	// the test should be revisited instead of passing for the wrong reason.
	if !isColision(0, 0, 0, 0, 0, 0) {
		t.Fatal("isColision no longer matches a 0x0 rect at the origin — buttonHitTestable's reason to exist has changed")
	}
	cases := []struct {
		name string
		btn  kag3.Button
		want bool
	}{
		{"sizeless", kag3.Button{}, false},
		{"zero width", kag3.Button{Height: 40}, false},
		{"zero height", kag3.Button{Width: 40}, false},
		{"normal", kag3.Button{Width: 40, Height: 40}, true},
	}
	for _, c := range cases {
		if got := buttonHitTestable(&c.btn); got != c.want {
			t.Errorf("buttonHitTestable(%s: %dx%d) = %v, want %v",
				c.name, c.btn.Width, c.btn.Height, got, c.want)
		}
	}
}
