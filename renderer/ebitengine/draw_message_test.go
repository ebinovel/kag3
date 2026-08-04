package ebitengine

import (
	"math"
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

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

// TestDrawMessageHorizontalAppliesLineHeightRatio is the regression test for
// the redesigned message window's line-height (bodyLineHeightRatio,
// draw_messagebox.go): advancing from one [p]-separated line (lineNum) to
// the next must move down by rowHeight*bodyLineHeightRatio, not a bare
// rowHeight — unlike TestDrawMessageHorizontalWrapPositionsWaitMarkAfterLastLine
// above, which covers a *single* line auto-wrapping into multiple physical
// rows (unaffected by this ratio), this covers two separate lineNums.
func TestDrawMessageHorizontalAppliesLineHeightRatio(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
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
