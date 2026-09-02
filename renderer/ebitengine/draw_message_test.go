package ebitengine

import (
	"math"
	"strings"
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// TestDrawMessageWindowSkipsBoxWithNoBackImage is the regression test for a
// real crash: [ct] (handleCT, tags_message.go) replaces textPosition with a
// fresh struct that keeps Visible=true but zeroes everything else,
// including BackImage — Width/Height land at 0 too, so nothing re-allocates
// BackImage until whatever [position] call follows [ct] actually gives it a
// size. Any frame drawn in that gap must not reach
// textPosition.BackImage.Fill on a nil *ebiten.Image, which crashes the
// whole renderer over a single tag with nothing else wrong in the script.
func TestDrawMessageWindowSkipsBoxWithNoBackImage(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	defer func() { textPosition = nil }()

	// The exact shape handleCT produces: Visible survives, everything else
	// (including BackImage) is back at its zero value.
	textPosition = &kag3.TextPosition{Visible: true}
	r.texts = map[int][]Text{}
	r.line = 0

	buf := newTestImage(1920, 1080)
	drawMessageWindow(r, buf) // must not panic
}

// TestDrawMessageWindowStillDrawsBoxWhenBackImagePresent guards the ordinary
// path alongside the regression test above: the switch in drawMessageWindow
// must still take the BackImage branch (Fill + DrawImage) when one exists,
// not fall through to the "nothing to draw" case added for the nil-BackImage
// fix. This package has no precedent for reading pixels back out of an
// *ebiten.Image in a test (ReadPixels needs a real running game loop, not
// available here), so this only proves the call sequence doesn't panic —
// same limit every other draw* test in this package already has.
func TestDrawMessageWindowStillDrawsBoxWhenBackImagePresent(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	defer func() { textPosition = nil }()

	textPosition = &kag3.TextPosition{Visible: true, Width: 200, Height: 100}
	textPosition.BackImage = newTestImage(200, 100)
	r.texts = map[int][]Text{}
	r.line = 0

	buf := newTestImage(1920, 1080)
	drawMessageWindow(r, buf) // must not panic
}

// TestDrawMessageHorizontalWrapPositionsWaitMarkAfterLastLine is the
// regression test for a real reported bug: when a line's text auto-wraps
// (the maxWidth check in drawMessageHorizontal) into two or more physical
// rows, the wait-mark (textEndX/textEndY, read by drawGlyph in
// tags_sysdesign.go) landed right after line 1's last character instead of
// the actual last line's. xOffset accumulates text.Measure's width for the
// segment's *widest* wrapped row (its whole-block bounding width), not
// "how far the cursor ended up," so marginLeft+xOffset was always line 1's
// end position regardless of how many rows the wrap added.
func TestDrawMessageHorizontalWrapPositionsWaitMarkAfterLastLine(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	beforeTextSize = r.fontFace.Size
	defer func() { isWait, isTextEnd, textEndX, textEndY = false, false, 0, 0 }()

	// Sized so "あいうえお" (5 characters) auto-wraps into exactly two
	// rows ("あいう" / "えお") — wide enough for 3 characters, not 5.
	threeCharsWidth, _ := text.Measure("あいう", r.fontFace, r.fontFace.Size)
	textPosition = &kag3.TextPosition{Visible: true, Width: int(threeCharsWidth) + 2, Height: 500}
	defer func() { textPosition = nil }()

	r.texts = map[int][]Text{0: {{Text: "あいうえお"}}}
	r.line = 0
	isWait = true // already fully revealed and waiting, regardless of count

	buf := newTestImage(int(threeCharsWidth)+2, 500)
	drawMessageHorizontal(r, buf, 0, 0, []int{0}, 9999)

	if !isTextEnd {
		t.Fatal("expected isTextEnd = true")
	}
	wantY := 2 * beforeTextSize
	if math.Abs(textEndY-wantY) > 0.5 {
		t.Errorf("textEndY = %v, want ~%v (two wrapped rows tall, not one)", textEndY, wantY)
	}

	lastLineWidth, _ := text.Measure("えお", r.fontFace, r.fontFace.Size)
	if math.Abs(textEndX-lastLineWidth) > 0.5 {
		t.Errorf("textEndX = %v, want ~%v (width of the second wrapped row \"えお\", not the whole block's widest row)", textEndX, lastLineWidth)
	}
}

// TestWrapTextWrapsRepeatedlyForVeryLongText guards the auto-wrap in
// drawMessageHorizontal/drawMessage against breaking only at the first
// overflow point: text more than roughly 2x maxWidth wide (e.g.
// demo_movie.ks's Wikimedia credit line, several times that) must wrap onto
// as many rows as it needs, not overflow off the right edge of the message
// box on every row after the first.
func TestWrapTextWrapsRepeatedlyForVeryLongText(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)

	threeCharsWidth, _ := text.Measure("あいう", r.fontFace, r.fontFace.Size)
	maxWidth := threeCharsWidth + 2

	got := wrapText("あいうえおかきくけこ", r.fontFace, r.fontFace.Size, maxWidth)
	gotLines := strings.Split(got, "\n")
	wantLines := []string{"あいう", "えおか", "きくけ", "こ"}
	if len(gotLines) != len(wantLines) {
		t.Fatalf("wrapText produced %d lines %q, want %d lines %q", len(gotLines), gotLines, len(wantLines), wantLines)
	}
	for i, want := range wantLines {
		if gotLines[i] != want {
			t.Errorf("line %d = %q, want %q", i, gotLines[i], want)
		}
	}
	for i, line := range gotLines {
		if w, _ := text.Measure(line, r.fontFace, r.fontFace.Size); w > maxWidth {
			t.Errorf("line %d %q measures %v, want <= maxWidth %v (every row must actually fit, not just the first)", i, line, w, maxWidth)
		}
	}
}

// TestDrawMessageHorizontalAppliesLineHeightRatio covers the redesigned
// message window's line-height (bodyLineHeightRatio,
// draw_messagebox.go): advancing from one [p]-separated line (lineNum) to
// the next must move down by rowHeight*bodyLineHeightRatio, not a bare
// rowHeight — unlike TestDrawMessageHorizontalWrapPositionsWaitMarkAfterLastLine
// above, which covers a *single* line auto-wrapping into multiple physical
// rows (unaffected by this ratio), this covers two separate lineNums.
func TestDrawMessageHorizontalAppliesLineHeightRatio(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config.MessageBoxStyle = "redesigned" // bodyLineHeightRatio only applies under the redesign (see messageBoxFillColor's doc comment for why)
	beforeTextSize = r.fontFace.Size
	defer func() { isWait, isTextEnd, textEndX, textEndY = false, false, 0, 0 }()

	// Wide enough that neither line wraps.
	wideWidth, _ := text.Measure("あああああああああああ", r.fontFace, r.fontFace.Size)
	textPosition = &kag3.TextPosition{Visible: true, Width: int(wideWidth) + 100, Height: 1000}
	defer func() { textPosition = nil }()

	r.texts = map[int][]Text{
		0: {{Text: "あ"}},
		1: {{Text: "い"}},
	}
	r.line = 1
	isWait = true // line 1 already fully revealed and waiting

	buf := newTestImage(int(wideWidth)+100, 1000)
	drawMessageHorizontal(r, buf, 0, 0, []int{0, 1}, 9999)

	if !isTextEnd {
		t.Fatal("expected isTextEnd = true")
	}
	wantY := beforeTextSize*bodyLineHeightRatio + beforeTextSize
	if math.Abs(textEndY-wantY) > 0.5 {
		t.Errorf("textEndY = %v, want ~%v (line 0's height*bodyLineHeightRatio + line 1's own height)", textEndY, wantY)
	}
}
