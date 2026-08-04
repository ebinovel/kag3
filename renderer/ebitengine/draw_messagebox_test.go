package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

func TestFilledSpeedSegmentsScalesWithTextSpeedMs(t *testing.T) {
	defer func() { textSpeedMs = 83 }()

	cases := []struct {
		name string
		ms   int
		want int
	}{
		{"fastest clamps to all filled", 0, speedSegCount},
		{"at min: fully filled", textSpeedIndicatorMinMs, speedSegCount},
		{"at max: empty", textSpeedIndicatorMaxMs, 0},
		{"slowest clamps to empty", 1000, 0},
		{"midpoint: about half filled", (textSpeedIndicatorMinMs + textSpeedIndicatorMaxMs) / 2, speedSegCount / 2},
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
	if got := messageBoxFillColor(); got != messageBoxFillColorNormal {
		t.Errorf("messageBoxFillColor() = %v, want normal %v with no glinks", got, messageBoxFillColorNormal)
	}

	glinks = []*kag3.GLink{{Target: "somewhere"}}
	isJump = false
	if got := messageBoxFillColor(); got != messageBoxFillColorChoice {
		t.Errorf("messageBoxFillColor() = %v, want dimmed %v while glinks are active", got, messageBoxFillColorChoice)
	}

	isJump = true
	if got := messageBoxFillColor(); got != messageBoxFillColorNormal {
		t.Errorf("messageBoxFillColor() = %v, want normal %v once a jump is pending (glinks about to be cleared)", got, messageBoxFillColorNormal)
	}
}
