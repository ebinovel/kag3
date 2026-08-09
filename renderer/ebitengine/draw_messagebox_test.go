package ebitengine

import (
	"strconv"
	"testing"

	"github.com/ebinovel/kag3"
)

// TestFilledSpeedSegmentsMatchesNearestSpeedStep is the regression test for
// the indicator and config.ks's own text-speed slider disagreeing: this
// used to scale continuously against a [5,100]ms range that matched
// neither speedSteps' real extremes (20-140ms) nor its step count (5, not
// this indicator's old 4 segments) — e.g. selecting config.ks's "標準"
// (83ms) lit only 1 of 4 segments instead of landing in the middle.
// filledSpeedSegments must now snap to whichever of the 5 real
// speedSteps entries textSpeedMs is closest to.
func TestFilledSpeedSegmentsMatchesNearestSpeedStep(t *testing.T) {
	defer func() { textSpeedMs = 83 }()

	cases := []struct {
		name string
		ms   int
		want int
	}{
		{"exact match: slowest (idx0, 140ms = 遅い)", 140, 1},
		{"exact match: idx1 (110ms = やや遅い)", 110, 2},
		{"exact match: standard (idx2, 83ms = 標準)", 83, 3},
		{"exact match: idx3 (50ms = やや速い)", 50, 4},
		{"exact match: fastest (idx4, 20ms = 速い)", 20, 5},
		{"faster than fastest clamps to fastest", 0, 5},
		{"slower than slowest clamps to slowest", 1000, 1},
		{"nearest: between idx2(83) and idx3(50), closer to 83", 70, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			textSpeedMs = c.ms
			if got := filledSpeedSegments(); got != c.want {
				t.Errorf("filledSpeedSegments() with textSpeedMs=%d = %d, want %d", c.ms, got, c.want)
			}
		})
	}
}

func TestDrawTextSpeedIndicatorNoopWithoutVisibleTextPosition(t *testing.T) {
	// See TestDrawContinueMarkPicksColorByModePriority's comment: restore
	// rather than null out, since other tests rely on textPosition being
	// left non-nil by whatever ran before them.
	saved := textPosition
	defer func() { textPosition = saved }()

	textPosition = nil
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	buf := newTestImage(1920, 1080)
	drawTextSpeedIndicator(r, buf) // must not panic

	textPosition = &kag3.TextPosition{Visible: false}
	drawTextSpeedIndicator(r, buf) // must not panic, and must be a no-op
}

// TestSetFilledSpeedSegmentsRoundTripsWithFilledSpeedSegments is the
// deliverable for the message box's own text-speed indicator becoming
// clickable: clicking segment i (1-indexed count) must set textSpeedMs to
// whatever value filledSpeedSegments would, in turn, read back as i.
func TestSetFilledSpeedSegmentsRoundTripsWithFilledSpeedSegments(t *testing.T) {
	defer func() { textSpeedMs, defaultTextSpeedMs = 83, 83 }()
	r := newTestRenderer()

	for filled := 1; filled <= speedSegCount; filled++ {
		setFilledSpeedSegments(r, filled)
		if got := filledSpeedSegments(); got != filled {
			t.Errorf("setFilledSpeedSegments(%d) then filledSpeedSegments() = %d, want %d (textSpeedMs=%d)", filled, got, filled, textSpeedMs)
		}
		if defaultTextSpeedMs != textSpeedMs {
			t.Errorf("setFilledSpeedSegments(%d): defaultTextSpeedMs = %d, want it to match textSpeedMs = %d", filled, defaultTextSpeedMs, textSpeedMs)
		}
	}
}

// TestSetFilledSpeedSegmentsSyncsTfSetSpeedIdx is the regression test for
// clicking the message box's own indicator leaving config.ks's slider
// (which reads tf.set_speed_idx, not textSpeedMs, to draw itself) showing
// a stale selection, and [configsave] persisting that stale idx instead
// of whatever speed the click actually set.
func TestSetFilledSpeedSegmentsSyncsTfSetSpeedIdx(t *testing.T) {
	defer func() { textSpeedMs, defaultTextSpeedMs = 83, 83 }()
	r := newTestRenderer()

	for filled := 1; filled <= speedSegCount; filled++ {
		setFilledSpeedSegments(r, filled)
		wantIdx := filled - 1
		if got := r.vm.EvalString("tf.set_speed_idx"); got != strconv.Itoa(wantIdx) {
			t.Errorf("setFilledSpeedSegments(%d): tf.set_speed_idx = %q, want %q", filled, got, strconv.Itoa(wantIdx))
		}
	}
}

// TestSetFilledSpeedSegmentsClampsOutOfRange covers the 0-and-below /
// above-speedSegCount edges a stray click shouldn't be able to reach in
// practice, but the function should still behave sanely for.
func TestSetFilledSpeedSegmentsClampsOutOfRange(t *testing.T) {
	defer func() { textSpeedMs, defaultTextSpeedMs = 83, 83 }()
	r := newTestRenderer()

	setFilledSpeedSegments(r, 0)
	if got := filledSpeedSegments(); got != 1 {
		t.Errorf("setFilledSpeedSegments(0) then filledSpeedSegments() = %d, want clamped to 1", got)
	}
	setFilledSpeedSegments(r, speedSegCount+5)
	if got := filledSpeedSegments(); got != speedSegCount {
		t.Errorf("setFilledSpeedSegments(overshoot) then filledSpeedSegments() = %d, want clamped to %d", got, speedSegCount)
	}
}

// TestDispatchTextSpeedIndicatorClickAtHitsCorrectSegment drives a click at
// each segment's real drawn position (via textSpeedIndicatorOrigin, the
// same helper drawTextSpeedIndicator itself uses) and confirms it lands on
// that segment, not a neighbor.
func TestDispatchTextSpeedIndicatorClickAtHitsCorrectSegment(t *testing.T) {
	savedTextPosition, savedMs, savedDefaultMs := textPosition, textSpeedMs, defaultTextSpeedMs
	defer func() { textPosition, textSpeedMs, defaultTextSpeedMs = savedTextPosition, savedMs, savedDefaultMs }()

	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginLeft: 56, MarginBottom: 44}
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config.MessageBoxStyle = "redesigned" // the indicator only exists under the redesign

	segX, y, ok := textSpeedIndicatorOrigin(r)
	if !ok {
		t.Fatal("textSpeedIndicatorOrigin: ok = false, want true with a visible textPosition")
	}
	for i := 0; i < speedSegCount; i++ {
		centerX := segX + float64(i)*(speedSegW+speedSegGap) + speedSegW/2
		centerY := y + speedSegH/2
		r.dispatchTextSpeedIndicatorClickAt(int(centerX), int(centerY), false)
		if got := filledSpeedSegments(); got != i+1 {
			t.Errorf("click on segment %d: filledSpeedSegments() = %d, want %d", i, got, i+1)
		}
	}
}

// TestDispatchTextSpeedIndicatorClickAtIgnoredWhileGLinksActive mirrors
// opRowActive's own guard test — a click landing on the indicator's own
// coordinates must not change anything while a choice list is up and no
// jump is pending yet.
func TestDispatchTextSpeedIndicatorClickAtIgnoredWhileGLinksActive(t *testing.T) {
	savedTextPosition, savedMs, savedDefaultMs, savedGLinks, savedIsJump := textPosition, textSpeedMs, defaultTextSpeedMs, glinks, isJump
	defer func() {
		textPosition, textSpeedMs, defaultTextSpeedMs, glinks, isJump = savedTextPosition, savedMs, savedDefaultMs, savedGLinks, savedIsJump
	}()

	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginLeft: 56, MarginBottom: 44}
	textSpeedMs, defaultTextSpeedMs = 83, 83
	glinks = []*kag3.GLink{{Target: "somewhere"}}
	isJump = false

	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config.MessageBoxStyle = "redesigned" // exercise the glinks guard itself, not just the style gate
	segX, y, _ := textSpeedIndicatorOrigin(r)
	r.dispatchTextSpeedIndicatorClickAt(int(segX+speedSegW/2), int(y+speedSegH/2), false)

	if textSpeedMs != 83 {
		t.Errorf("textSpeedMs = %d after a click while glinks active, want unchanged 83", textSpeedMs)
	}
}

func TestDrawContinueMarkPicksColorByModePriority(t *testing.T) {
	// Restores the pre-test textPosition rather than forcing nil: other
	// tests in this package rely on whatever an earlier test left
	// textPosition as (a known cross-test-ordering fragility documented in
	// this repo's CLAUDE.md) — leaving it nil here breaks any test that
	// runs after this one and doesn't set textPosition itself.
	savedTextPosition, savedIsSkip, savedIsAuto, savedIsTextEnd := textPosition, isSkip, isAuto, isTextEnd
	defer func() {
		textPosition, isSkip, isAuto, isTextEnd = savedTextPosition, savedIsSkip, savedIsAuto, savedIsTextEnd
	}()

	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginRight: 56, MarginBottom: 44}
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	buf := newTestImage(1920, 1080)

	// No mode active: no-op, must not panic.
	isSkip, isAuto, isTextEnd = false, false, false
	drawContinueMark(r, buf)

	// isTextEnd=false must stay a no-op even under auto/skip — the mark
	// must not show while the current line is still mid-reveal, regression
	// coverage for the same "wait indicator never goes away under auto"
	// bug drawGlyph's own test covers (tags_sysdesign_test.go).
	isSkip, isAuto, isTextEnd = false, true, false
	drawContinueMark(r, buf)
	isSkip, isAuto, isTextEnd = true, false, false
	drawContinueMark(r, buf)

	// Each of the three modes must draw without panicking; priority order
	// (skip > auto > textEnd) mirrors drawGlyph's own switch.
	isSkip, isAuto, isTextEnd = true, true, true
	drawContinueMark(r, buf)

	isSkip, isAuto, isTextEnd = false, true, true
	drawContinueMark(r, buf)

	isSkip, isAuto, isTextEnd = false, false, true
	drawContinueMark(r, buf)
}

func TestMessageBoxFillColorDimsWhileGLinksActive(t *testing.T) {
	defer func() { glinks, isJump = nil, false }()

	glinks, isJump = nil, false
	if got := messageBoxFillColor("redesigned"); got != messageBoxFillColorNormal {
		t.Errorf("messageBoxFillColor() = %v, want normal %v with no glinks", got, messageBoxFillColorNormal)
	}

	glinks = []*kag3.GLink{{Target: "somewhere"}}
	isJump = false
	if got := messageBoxFillColor("redesigned"); got != messageBoxFillColorChoice {
		t.Errorf("messageBoxFillColor() = %v, want dimmed %v while glinks are active", got, messageBoxFillColorChoice)
	}

	isJump = true
	if got := messageBoxFillColor("redesigned"); got != messageBoxFillColorNormal {
		t.Errorf("messageBoxFillColor() = %v, want normal %v once a jump is pending (glinks about to be cleared)", got, messageBoxFillColorNormal)
	}
}

// TestMessageBoxFillColorLegacyIgnoresStyle is the scoping fix for
// renderer/ebitengine being a shared package: any style other than
// "redesigned" — including "" (a project whose config.toml predates
// MessageBoxStyle entirely, e.g. tsf-action) — must draw this package's
// original flat box, glink dimming included, regardless of glinks/isJump.
func TestMessageBoxFillColorLegacyIgnoresStyle(t *testing.T) {
	defer func() { glinks, isJump = nil, false }()

	for _, style := range []string{"", "legacy", "something-unrecognized"} {
		glinks, isJump = nil, false
		if got := messageBoxFillColor(style); got != messageBoxFillColorLegacy {
			t.Errorf("messageBoxFillColor(%q) = %v, want legacy %v", style, got, messageBoxFillColorLegacy)
		}
		glinks = []*kag3.GLink{{Target: "somewhere"}}
		if got := messageBoxFillColor(style); got != messageBoxFillColorLegacy {
			t.Errorf("messageBoxFillColor(%q) with glinks active = %v, want still legacy %v (no redesigned dim effect)", style, got, messageBoxFillColorLegacy)
		}
	}
}
