package ebitengine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// --- backlog (button role="backlog"/LOG in the operation row) ---
//
// Full-screen dim, a header ("BACKLOG"/履歴 + 閉じる✕), a fixed-width name
// column distinct from the text column (recordBacklog, tags_message.go,
// keeps them separate), recency-fade opacity on the oldest few visible
// rows, a proportional scrollbar, and a footer with scroll/navigation
// hints. Coordinates are unscaled 1920x1080 pixel values — the canvas
// example/ runs at.

var (
	backlogViewing     bool
	backlogOpenedFrame int
	// backlogScrollY is how far scrolled *up* from the newest entry (0 =
	// showing the most recent entries at the bottom, the default/rest
	// state) — the opposite sense from slotPickerScrollY (which measures
	// down from the top), because backlog reads newest-at-bottom like a
	// chat transcript.
	backlogScrollY   float64
	backlogDragging  bool
	backlogDragLastY int
	// backlogDidDrag is "has the pointer moved since it was first pressed"
	// — deliberately a separate flag from backlogDragging ("is the pointer
	// currently held down at all"). See updateBacklogDrag's doc comment for
	// why collapsing these into one flag doesn't work.
	backlogDidDrag bool
)

const (
	backlogScrollStep = 40.0

	backlogPaddingTop       = 56.0
	backlogPaddingLeftRight = 96.0
	backlogPaddingBottom    = 44.0
	backlogHeaderTitleSize  = 34.0
	backlogHeaderSubSize    = 20.0
	backlogHeaderPaddingGap = 20.0
	backlogHeaderPaddingBtm = 20.0
	backlogCloseSize        = 22.0
	backlogBodyPaddingTop   = 34.0
	backlogNameColW         = 220.0
	backlogColGap           = 40.0
	backlogNameFontSize     = 26.0
	backlogTextFontSize     = 30.0
	backlogTextLineHeight   = 1.7
	backlogScrollbarColGap  = 28.0
	backlogScrollbarW       = 6.0
	backlogFooterPaddingTop = 20.0
	backlogFooterMarginTop  = 14.0
	backlogFooterFontSize   = 20.0
	backlogFooterItemGap    = 26.0
)

var (
	backlogDimColor          = color.RGBA{0x0a, 0x0c, 0x10, 0xe6} // rgba(10,12,16,0.9)
	backlogHeaderBorderColor = color.RGBA{0x8f, 0xc0, 0xd8, 0x66} // rgba(143,192,216,0.4)
	backlogTitleColor        = color.RGBA{0xf2, 0xf5, 0xf8, 0xff}
	backlogSubColor          = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogCloseColor        = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	backlogNameColor         = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	backlogNarrationColor    = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogTextColor         = color.RGBA{0xe6, 0xeb, 0xf0, 0xff}
	backlogScrollTrackColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0x2e} // rgba(143,192,216,0.18)
	backlogScrollThumbColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	backlogFooterBorderColor = color.RGBA{0x8f, 0xc0, 0xd8, 0x40} // rgba(143,192,216,0.25)
	backlogFooterDimColor    = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogFooterTextColor   = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	// backlogFadeSteps are the opacities applied to the oldest few visible
	// rows (top of the viewport), newest-first fading in from there. Rows
	// beyond this are fully opaque.
	backlogFadeSteps = []float32{0.45, 0.6, 0.8, 1.0}
)

// backlogEntryHeight is a fixed per-row height (text column's line-height
// at its font size) — entries aren't word-wrapped (matching this package's
// existing "no clipping, no wrap" simplicity elsewhere for secondary UI),
// so this is exact, not an estimate.
const backlogEntryHeight = backlogTextFontSize * backlogTextLineHeight

func backlogContentHeight() float64 {
	if len(backlog) == 0 {
		return 0
	}
	return float64(len(backlog)) * backlogEntryHeight
}

func backlogMaxScroll(viewportH float64) float64 {
	max := backlogContentHeight() - viewportH
	if max < 0 {
		max = 0
	}
	return max
}

func clampBacklogScroll(viewportH float64) {
	max := backlogMaxScroll(viewportH)
	if backlogScrollY > max {
		backlogScrollY = max
	}
	if backlogScrollY < 0 {
		backlogScrollY = 0
	}
}

func backlogFace(r *Renderer, size float64) *text.GoTextFace {
	return &text.GoTextFace{Source: r.fontFace.Source, Size: size, Language: r.fontFace.Language}
}

// backlogNameDisplay is the name column's text for entry — "──" (dimmed)
// for a nameless/narration entry ([pushlog] or a bare "#" line).
func backlogNameDisplay(name string) (string, color.RGBA) {
	if name == "" {
		return "──", backlogNarrationColor
	}
	return name, backlogNameColor
}

func drawBacklog(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	fillRect(buf, 0, 0, float64(w), float64(h), backlogDimColor)

	contentX := backlogPaddingLeftRight
	contentRight := float64(w) - backlogPaddingLeftRight
	contentW := contentRight - contentX

	// --- header ---
	titleFace := backlogFace(r, backlogHeaderTitleSize)
	titleW, titleH := text.Measure("BACKLOG", titleFace, 0)
	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Translate(contentX, backlogPaddingTop)
	titleOp.ColorScale.ScaleWithColor(backlogTitleColor)
	text.Draw(buf, "BACKLOG", titleFace, titleOp)

	subFace := backlogFace(r, backlogHeaderSubSize)
	subOp := &text.DrawOptions{}
	subOp.GeoM.Translate(contentX+titleW+backlogHeaderPaddingGap, backlogPaddingTop+(titleH-backlogHeaderSubSize))
	subOp.ColorScale.ScaleWithColor(backlogSubColor)
	text.Draw(buf, "履歴", subFace, subOp)

	const closeLabel = "閉じる ✕"
	closeFace := backlogFace(r, backlogCloseSize)
	closeW, _ := text.Measure(closeLabel, closeFace, 0)
	closeOp := &text.DrawOptions{}
	closeOp.GeoM.Translate(contentRight-closeW, backlogPaddingTop+(titleH-backlogCloseSize))
	closeOp.ColorScale.ScaleWithColor(backlogCloseColor)
	text.Draw(buf, closeLabel, closeFace, closeOp)

	headerBottom := backlogPaddingTop + titleH + backlogHeaderPaddingBtm
	fillRect(buf, contentX, headerBottom, contentW, 1, backlogHeaderBorderColor)

	// --- footer ---
	footerFace := backlogFace(r, backlogFooterFontSize)
	const scrollHint = "ホイール／ドラッグでスクロール"
	_, footerH := text.Measure(scrollHint, footerFace, 0)
	footerBorderY := float64(h) - backlogPaddingBottom - footerH - backlogFooterPaddingTop
	fillRect(buf, contentX, footerBorderY, contentW, 1, backlogFooterBorderColor)
	footerTextY := footerBorderY + backlogFooterPaddingTop
	hintOp := &text.DrawOptions{}
	hintOp.GeoM.Translate(contentX, footerTextY)
	hintOp.ColorScale.ScaleWithColor(backlogFooterDimColor)
	text.Draw(buf, scrollHint, footerFace, hintOp)

	const backHint = "右クリックで戻る"
	const latestHint = "最新へ ▼"
	backW, _ := text.Measure(backHint, footerFace, 0)
	latestW, _ := text.Measure(latestHint, footerFace, 0)
	backOp := &text.DrawOptions{}
	backOp.GeoM.Translate(contentRight-backW, footerTextY)
	backOp.ColorScale.ScaleWithColor(backlogFooterTextColor)
	text.Draw(buf, backHint, footerFace, backOp)
	latestOp := &text.DrawOptions{}
	latestOp.GeoM.Translate(contentRight-backW-backlogFooterItemGap-latestW, footerTextY)
	latestOp.ColorScale.ScaleWithColor(backlogFooterTextColor)
	text.Draw(buf, latestHint, footerFace, latestOp)

	// --- body: scrollable name/text columns + scrollbar ---
	bodyTop := headerBottom + backlogBodyPaddingTop
	bodyBottom := footerBorderY - backlogFooterMarginTop
	viewportH := bodyBottom - bodyTop
	if viewportH < 0 {
		viewportH = 0
	}
	clampBacklogScroll(viewportH)

	textColW := contentW - backlogNameColW - backlogColGap - backlogScrollbarColGap - backlogScrollbarW
	nameFace := backlogFace(r, backlogNameFontSize)
	textFace := backlogFace(r, backlogTextFontSize)

	contentH := backlogContentHeight()
	// scrollTop is the content-space Y of the viewport's own top edge:
	// content is bottom-anchored (newest entry's bottom sits at
	// bodyBottom) when backlogScrollY == 0, and moves up as it increases.
	scrollTop := contentH - viewportH - backlogScrollY

	for i, entry := range backlog {
		rowTop := float64(i) * backlogEntryHeight
		y := bodyTop + (rowTop - scrollTop)
		if y+backlogEntryHeight < bodyTop || y > bodyBottom {
			continue
		}
		// Fade the first few rows *from the top of the viewport*, not by
		// absolute recency, so content fades in as the reader scrolls up
		// toward older lines — the bottom-most/newest rows stay fully
		// opaque regardless of scroll position.
		rowIndexFromViewportTop := int((y - bodyTop) / backlogEntryHeight)
		alpha := float32(1.0)
		if rowIndexFromViewportTop >= 0 && rowIndexFromViewportTop < len(backlogFadeSteps) {
			alpha = backlogFadeSteps[rowIndexFromViewportTop]
		}

		name, nameColor := backlogNameDisplay(entry.Name)
		nameColor.A = uint8(float32(nameColor.A) * alpha)
		nameOp := &text.DrawOptions{}
		nameOp.GeoM.Translate(contentX, y)
		nameOp.ColorScale.ScaleWithColor(nameColor)
		text.Draw(buf, name, nameFace, nameOp)

		txtColor := backlogTextColor
		txtColor.A = uint8(float32(txtColor.A) * alpha)
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(contentX+backlogNameColW+backlogColGap, y)
		textOp.ColorScale.ScaleWithColor(txtColor)
		txt := entry.Text
		if w, _ := text.Measure(txt, textFace, 0); w > textColW {
			// No word-wrap for backlog text (matches the rest of this
			// package's secondary-UI simplicity) — truncate instead of
			// overflowing into the scrollbar column.
			for len([]rune(txt)) > 0 {
				rn := []rune(txt)
				txt = string(rn[:len(rn)-1])
				if ww, _ := text.Measure(txt+"…", textFace, 0); ww <= textColW {
					txt += "…"
					break
				}
			}
		}
		text.Draw(buf, txt, textFace, textOp)
	}

	if max := backlogMaxScroll(viewportH); max > 0 {
		trackX := contentRight - backlogScrollbarW
		fillRect(buf, trackX, bodyTop, backlogScrollbarW, viewportH, backlogScrollTrackColor)
		thumbH := viewportH * viewportH / contentH
		if thumbH < 20 {
			thumbH = 20
		}
		if thumbH > viewportH {
			thumbH = viewportH
		}
		// backlogScrollY == 0 anchors the thumb to the bottom (newest
		// visible) — the inverse of scrollY's own top-anchored sense.
		thumbY := bodyTop + (viewportH-thumbH)*(1-backlogScrollY/max)
		fillRect(buf, trackX, thumbY, backlogScrollbarW, thumbH, backlogScrollThumbColor)
	}
}

// updateBacklogDrag advances the backlog's drag-to-scroll state by one
// frame given this frame's raw pointer state, and reports how much to add
// to backlogScrollY (0 if nothing changed). Split out from
// handleBacklogClick so the drag/click distinction is testable without
// faking ebiten's real input state — this package has no way to do that
// directly (see skipShouldAdvance's own doc comment, renderer.go, for the
// same constraint on a different feature).
//
// backlogDidDrag exists as a separate flag from backlogDragging
// specifically because collapsing them doesn't work: a plain click's very
// first frame already has pressed=true with backlogDragging still false, so
// the "not currently dragging -> start dragging" branch below sets
// backlogDragging = true on that exact frame — before handleBacklogClick's
// own justPressed check ever runs. Reading backlogDragging there as "was
// this a click or a drag" therefore misclassifies every click as a drag,
// including one landing outside the backlog specifically to dismiss it,
// making handleBacklogClick's "click outside closes the backlog" branch
// unreachable. backlogDidDrag only ever becomes true once the pointer
// actually moves while held down, and is reset the moment a fresh press
// begins — so a click that never moves still reports backlogDidDrag ==
// false on its own justPressed frame.
func updateBacklogDrag(pressed bool, mY int) (scrollDelta float64) {
	if pressed {
		if !backlogDragging {
			backlogDragging = true
			backlogDidDrag = false
			backlogDragLastY = mY
		} else if mY != backlogDragLastY {
			// Dragging down reveals older entries (content moves down with
			// the pointer), so backlogScrollY — which measures up from the
			// newest entry — increases.
			scrollDelta = float64(backlogDragLastY - mY)
			backlogDragLastY = mY
			backlogDidDrag = true
		}
	} else {
		backlogDragging = false
	}
	return scrollDelta
}

// handleBacklogClick drives the backlog screen's input: wheel/drag-to-scroll,
// the header's 閉じる✕ button, and any-other-click/right-click to close —
// any click landing outside the close button dismisses the screen, except
// on the very frame it opened, where that click is the button press that
// opened it.
func (r *Renderer) handleBacklogClick() {
	w, h := r.manager.Config.ScreenWidth, r.manager.Config.ScreenHeight
	footerFace := backlogFace(r, backlogFooterFontSize)
	_, footerH := text.Measure("ホイール／ドラッグでスクロール", footerFace, 0)
	footerBorderY := float64(h) - backlogPaddingBottom - footerH - backlogFooterPaddingTop
	titleFace := backlogFace(r, backlogHeaderTitleSize)
	_, titleH := text.Measure("BACKLOG", titleFace, 0)
	headerBottom := backlogPaddingTop + titleH + backlogHeaderPaddingBtm
	bodyTop := headerBottom + backlogBodyPaddingTop
	viewportH := footerBorderY - backlogFooterMarginTop - bodyTop
	if viewportH < 0 {
		viewportH = 0
	}

	if _, wheelY := ebiten.Wheel(); wheelY != 0 {
		backlogScrollY += wheelY * backlogScrollStep
		clampBacklogScroll(viewportH)
	}

	mX, mY, justPressed, pressed, touch := pointerState()
	if delta := updateBacklogDrag(pressed, mY); delta != 0 {
		backlogScrollY += delta
		clampBacklogScroll(viewportH)
	}

	if t == backlogOpenedFrame {
		return
	}
	// Right-click-to-close is a desktop-only convenience — touch has no
	// equivalent gesture here, but the explicit close button and
	// tap-outside-to-dismiss below already cover it.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		backlogViewing = false
		return
	}
	if justPressed {
		contentRight := float64(w) - backlogPaddingLeftRight
		closeFace := backlogFace(r, backlogCloseSize)
		closeW, _ := text.Measure("閉じる ✕", closeFace, 0)
		closeX := contentRight - closeW
		closeY := backlogPaddingTop + (titleH - backlogCloseSize)
		if isColisionTouch(mX, mY, int(closeX), int(closeY), int(closeW), int(backlogCloseSize), touch) {
			backlogViewing = false
			return
		}
		// backlogDidDrag, not backlogDragging — see updateBacklogDrag's doc
		// comment for why the latter is always true by this point on a
		// plain click's own justPressed frame, and would make this branch
		// unreachable.
		if !backlogDidDrag {
			backlogViewing = false
		}
	}
}
