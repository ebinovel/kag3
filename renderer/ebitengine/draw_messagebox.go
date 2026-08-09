package ebitengine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// ptextBgPaddingX/Y position a [ptext bg=]'s text inside its background
// image (tags_text.go/renderer.go's drawPTexts) — matches the padding baked
// into example/game/resources/images/ui/name_tab.png's generation (see the
// scratch generator script referenced in the design plan; regenerate that
// image if these change).
const (
	ptextBgPaddingX = 34
	ptextBgPaddingY = 12
)

// bodyLineHeightRatio is the redesigned message window's line-height
// (relative to font size) — applied in drawMessageHorizontal (draw_message.go)
// where it replaces the previous ~1.0x-tight row advance.
const bodyLineHeightRatio = 1.72

// messageBoxFillColorNormal/messageBoxFillColorChoice are the message box's
// flat fill color per the redesigned message window (color/opacity=0x78 rgba
// equivalents), used only when neither [position frame=] nor
// [position_filter]/[message_config color=] override it — see
// drawMessageWindow (draw_message.go).
var (
	messageBoxFillColorNormal = color.RGBA{0x10, 0x13, 0x18, 0xc7} // rgba(16,19,24,0.78)
	messageBoxFillColorChoice = color.RGBA{0x10, 0x13, 0x18, 0x99} // rgba(16,19,24,0.6), dimmed while [glink] choices are up
	// messageBoxFillColorLegacy is this package's original (pre-メッセージ欄)
	// box fill — see messageBoxFillColor's doc comment for why this
	// stays the default.
	messageBoxFillColorLegacy = color.RGBA{0, 0, 0, 128}
)

// messageBoxFillColor returns the box's default fill. renderer/ebitengine
// is a shared package (this repo's example is one importer among several,
// e.g. tsf-action), so the メッセージ欄 redesign's fill color only
// applies when style == "redesigned" (Config.MessageBoxStyle,
// example/game/resources/config.toml only) — anything else, including an empty
// string (a project whose config.toml predates this field entirely), keeps
// the original flat rgba(0,0,0,0.5) this package always drew before. Within
// "redesigned", the box dims while a set of [glink] choices is currently
// displayed (the same len(glinks)>0 && !isJump guard drawLinks/drawGLinks
// use elsewhere).
func messageBoxFillColor(style string) color.RGBA {
	if style != "redesigned" {
		return messageBoxFillColorLegacy
	}
	if len(glinks) > 0 && !isJump {
		return messageBoxFillColorChoice
	}
	return messageBoxFillColorNormal
}

// --- text-speed indicator: bottom-left of the box, a 4-segment bar next to
// a "文字送り" label. ---

const (
	speedSegW, speedSegH = 46.0, 8.0
	speedSegGap          = 6.0
	speedLabelFontSize   = 20
	speedLabelGap        = 14.0
)

// speedSteps mirrors first.ks's tf.speed_steps ([140, 110, 83, 50, 20]ms,
// slowest to fastest) — the 5 discrete choices config.ks's own text-speed
// slider offers (tf.speed_labels: 遅い/やや遅い/標準/やや速い/速い). Kept
// in sync by hand: this indicator used to scale continuously against a
// [5,100]ms range that matched neither this array's real extremes
// (20-140ms) nor its step count (5, not the indicator's old 4 segments),
// so e.g. selecting config.ks's "標準" (idx 2, 83ms) lit only 1 of 4
// segments instead of landing in the middle. Update this array (and
// speedSegCount, which derives from it) if first.ks's tf.speed_steps ever
// changes.
var speedSteps = [...]int{140, 110, 83, 50, 20}

const speedSegCount = len(speedSteps)

var (
	speedSegFilledColor = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	speedSegEmptyColor  = color.RGBA{0x45, 0x4c, 0x55, 0xff}
	speedLabelColor     = color.RGBA{0x93, 0xa0, 0xac, 0xff}
)

// nearestSpeedStepIndex returns the index into speedSteps whose ms value is
// closest to ms (ties favor the lower/faster index — arbitrary but
// deterministic). Used both to light up the indicator for whatever
// textSpeedMs [configdelay]/config.ks last set, and — via
// setFilledSpeedSegments — to snap a click on the indicator itself onto
// one of the same 5 real choices instead of an arbitrary in-between value.
func nearestSpeedStepIndex(ms int) int {
	best, bestDiff := 0, -1
	for i, s := range speedSteps {
		diff := ms - s
		if diff < 0 {
			diff = -diff
		}
		if bestDiff == -1 || diff < bestDiff {
			bestDiff = diff
			best = i
		}
	}
	return best
}

// filledSpeedSegments reports how many of the bar's segments should be
// lit: 1 + the index of whichever speedSteps entry textSpeedMs is
// currently closest to. More filled = faster (lower textSpeedMs) — a
// battery/power-level reading, not a "distance" one; this is a judgment
// call (the mockup doesn't specify direction) and this is the one place
// to flip it if it reads backwards on screen.
func filledSpeedSegments() int {
	return nearestSpeedStepIndex(textSpeedMs) + 1
}

// textSpeedIndicatorOrigin computes the segment bar's own top-left (segX,
// y) — shared by drawTextSpeedIndicator and
// (*Renderer).handleTextSpeedIndicatorClick so hit-testing always matches
// what's actually drawn. ok is false when the indicator isn't shown at all
// (matches drawTextSpeedIndicator's own early return).
func textSpeedIndicatorOrigin(r *Renderer) (segX, y float64, ok bool) {
	// Part of the メッセージ欄 redesign, gated the same way as the
	// operation row (opRowActive, tags_oprow.go) — renderer/ebitengine is a
	// shared package, so this indicator must stay off for any project that
	// hasn't opted into MessageBoxStyle="redesigned".
	if r.manager.Config.MessageBoxStyle != "redesigned" {
		return 0, 0, false
	}
	if textPosition == nil || !textPosition.Visible {
		return 0, 0, false
	}
	x := float64(textPosition.Left) + float64(textPosition.MarginLeft)
	y = float64(textPosition.Top) + float64(textPosition.Height) - float64(textPosition.MarginBottom) - speedSegH

	labelFace := &text.GoTextFace{Source: r.fontFace.Source, Size: speedLabelFontSize, Language: r.fontFace.Language}
	labelW, _ := text.Measure("文字送り", labelFace, 0)
	return x + labelW + speedLabelGap, y, true
}

func drawTextSpeedIndicator(r *Renderer, buf *ebiten.Image) {
	segX, y, ok := textSpeedIndicatorOrigin(r)
	if !ok {
		return
	}
	x := float64(textPosition.Left) + float64(textPosition.MarginLeft)
	labelFace := &text.GoTextFace{Source: r.fontFace.Source, Size: speedLabelFontSize, Language: r.fontFace.Language}
	label := "文字送り"
	_, labelH := text.Measure(label, labelFace, 0)
	labelOp := &text.DrawOptions{}
	labelOp.GeoM.Translate(x, y+(speedSegH-labelH)/2)
	labelOp.ColorScale.ScaleWithColor(speedLabelColor)
	text.Draw(buf, label, labelFace, labelOp)

	filled := filledSpeedSegments()
	for i := 0; i < speedSegCount; i++ {
		c := speedSegEmptyColor
		if i < filled {
			c = speedSegFilledColor
		}
		fillRect(buf, segX+float64(i)*(speedSegW+speedSegGap), y, speedSegW, speedSegH, c)
	}
}

// setFilledSpeedSegments is filledSpeedSegments' inverse: given the number
// of segments a click means to light up (1-indexed — clicking the 3rd
// segment means "3 filled", matching how a battery/signal-bar control
// reads), applies the speedSteps value at that index. Sets
// defaultTextSpeedMs too, matching [configdelay]'s own "this is the new
// baseline, not a one-off [delay]" semantics — a direct click on the
// message box's own indicator is exactly that kind of durable preference
// change, not a scripted temporary effect.
//
// Also mirrors the choice into tf.set_speed_idx via r.vm.MergeTF:
// config.ks's own slider reads tf.set_speed_idx/tf.speed_labels (not
// textSpeedMs) to draw itself, so without this a click here would leave
// that screen showing a stale selection if opened afterward, and
// [configsave] would persist that stale idx instead of the speed this
// click actually set.
func setFilledSpeedSegments(r *Renderer, filled int) {
	if filled < 1 {
		filled = 1
	}
	if filled > speedSegCount {
		filled = speedSegCount
	}
	idx := filled - 1
	ms := speedSteps[idx]
	textSpeedMs = ms
	defaultTextSpeedMs = ms
	r.vm.MergeTF(map[string]interface{}{"set_speed_idx": float64(idx)})
}

// handleTextSpeedIndicatorClick lets the player click directly on the
// message box's own text-speed bar to change it immediately, rather than
// only being adjustable from the full settings screen (config.ks). Clicking
// segment i (0-indexed) sets the reading to i+1 filled segments.
func (r *Renderer) handleTextSpeedIndicatorClick() {
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	r.dispatchTextSpeedIndicatorClickAt(mX, mY, touch)
}

// dispatchTextSpeedIndicatorClickAt is handleTextSpeedIndicatorClick's
// testable core, split out the same way
// (*Renderer).dispatchOperationRowClickAt is (tags_oprow.go) — no precedent
// in this package for faking ebiten's real mouse-press state in a test.
func (r *Renderer) dispatchTextSpeedIndicatorClickAt(mX, mY int, touch bool) {
	if len(glinks) > 0 && !isJump {
		// Choices are up — same guard opRowActive uses (tags_oprow.go) to
		// keep the operation row from stealing a glink click; the indicator
		// sits at the box's bottom-left, close enough to a stacked choice
		// list to warrant the same caution.
		return
	}
	segX, y, ok := textSpeedIndicatorOrigin(r)
	if !ok {
		return
	}
	for i := 0; i < speedSegCount; i++ {
		segLeft := segX + float64(i)*(speedSegW+speedSegGap)
		if isColisionTouch(mX, mY, int(segLeft), int(y), int(speedSegW), int(speedSegH), touch) {
			setFilledSpeedSegments(r, i+1)
			return
		}
	}
}

// --- continue mark: a fixed bottom-right "▽", enabled by
// kag3.Config.ContinueMarkStyle == "fixed" (draw_message.go) instead of the
// legacy text-following glyphNormal/glyphSkip/glyphAuto marks
// (tags_sysdesign.go), which are left completely untouched for scripts that
// still rely on them. ---

const continueMarkFontSize = 26

// drawContinueMark mirrors drawGlyph's (tags_sysdesign.go) isSkip/isAuto/
// isTextEnd priority and color choice, but at a fixed box-corner position
// instead of following the last revealed glyph.
func drawContinueMark(r *Renderer, buf *ebiten.Image) {
	// Mirrors drawGlyph's own isTextEnd guard (tags_sysdesign.go) — must
	// stay hidden for the whole reveal of whatever line is currently
	// showing, auto/skip included, or it reads as "the wait indicator never
	// went away" the instant auto-advance moves to a new line.
	if textPosition == nil || !textPosition.Visible || !isTextEnd {
		return
	}
	var col color.RGBA
	switch {
	case skipActive():
		col = glyphSkip.Color
	case isAuto:
		col = glyphAuto.Color
	default:
		col = glyphNormal.Color
	}
	const mark = "▽"
	face := &text.GoTextFace{Source: r.fontFace.Source, Size: continueMarkFontSize, Language: r.fontFace.Language}
	w, h := text.Measure(mark, face, 0)
	x := float64(textPosition.Left) + float64(textPosition.Width) - float64(textPosition.MarginRight) - w
	y := float64(textPosition.Top) + float64(textPosition.Height) - float64(textPosition.MarginBottom) - h
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(col)
	text.Draw(buf, mark, face, op)
}
