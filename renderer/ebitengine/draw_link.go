package ebitengine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// drawLinks draws every [link]'s text choices, stacked below the message
// text at the current textPosition. Suppressed (still laid out, just not
// drawn) while isJump is pending, matching the rest of drawScene's
// jump-in-progress handling.
func drawLinks(r *Renderer, buf *ebiten.Image) {
	for i, link := range links {
		for j, t := range link.Texts {
			linkOp := &text.DrawOptions{}
			x, y := float64(textPosition.Left), float64(textPosition.Top)
			_, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + float64(textPosition.MarginLeft)
			marginTop := y + float64(textPosition.MarginTop) + h*float64(i+j)
			linkOp.GeoM.Translate(marginLeft, marginTop)
			applyTextStyle(r, linkOp, Text{})
			if !isJump {
				text.Draw(buf, t.Val, r.fontFace, linkOp)
			}
		}
	}
}

// --- [glink] choice boxes ---
//
// Under Config.MessageBoxStyle == "redesigned" only: glink.Color (a
// script-set [glink color=...]) is intentionally NOT used — the redesigned
// choice UI is a fixed, unified look (dark fill + a left-edge accent bar
// that lights up on hover) rather than a per-glink-configurable flat color,
// matching the source design's single choice-box style. Hover state is
// computed fresh every frame from the cursor position — the same "no
// cross-frame state" approach backButtonImageName/quickMenuButtonImageName
// already use elsewhere — rather than tracked in a package var. Any other
// style value draws this package's original plain glink.Color-filled box
// instead (see drawGLinksLegacy) — renderer/ebitengine is a shared package
// (this repo's example is one importer among several, e.g. tsf-action), so
// a script's own color= choice must keep working for anyone who hasn't
// opted into the redesign.
var (
	glinkBoxBg       = color.RGBA{0x10, 0x13, 0x18, 0xe6} // rgba(16,19,24,0.9)
	glinkTextDefault = color.RGBA{0xb9, 0xc3, 0xcd, 0xff}
	glinkTextHover   = color.RGBA{0xf2, 0xf5, 0xf8, 0xff}
	glinkAccent      = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
)

const glinkAccentWidth = 8

// drawGLinks dispatches to the redesigned or legacy [glink] rendering per
// Config.MessageBoxStyle — see the var block above.
func drawGLinks(r *Renderer, buf *ebiten.Image) {
	if r.manager.Config.MessageBoxStyle != "redesigned" {
		drawGLinksLegacy(r, buf)
		return
	}
	if isJump {
		return
	}
	mX, mY := ebiten.CursorPosition()
	for _, glink := range glinks {
		x, y, w, h := float64(glink.X), float64(glink.Y), float64(glink.Width), float64(glink.Height)
		hovered := isColision(mX, mY, glink.X, glink.Y, glink.Width, glink.Height)

		fillRect(buf, x, y, w, h, glinkBoxBg)
		if hovered {
			fillRect(buf, x, y, glinkAccentWidth, h, glinkAccent)
		}

		textColor := glinkTextDefault
		if hovered {
			textColor = glinkTextHover
		}
		glinkOp := &text.DrawOptions{}
		tw, th := text.Measure(glink.Text, r.fontFace, 0)
		glinkOp.GeoM.Translate(x+w/2-tw/2, y+h/2-th/2)
		glinkOp.ColorScale.ScaleWithColor(textColor)
		text.Draw(buf, glink.Text, r.fontFace, glinkOp)
	}
}

// drawGLinksLegacy is this package's original [glink] rendering, byte-for-
// byte: a script-colored flat rect (glink.Color, from [glink color=...])
// plus centered text — no hover state, no isJump suppression (unlike
// drawLinks/the redesigned path above, this never checked isJump; kept
// exactly as it always behaved rather than "fixed" here).
func drawGLinksLegacy(r *Renderer, buf *ebiten.Image) {
	for _, glink := range glinks {
		backgroundOp := &ebiten.DrawImageOptions{}
		backgroundOp.GeoM.Translate(float64(glink.X), float64(glink.Y))
		img := ebiten.NewImage(glink.Width, glink.Height)
		img.Fill(glink.Color)
		buf.DrawImage(img, backgroundOp)
		glinkOp := &text.DrawOptions{}
		w, h := text.Measure(glink.Text, r.fontFace, 0)
		glinkOp.GeoM.Translate(float64(glink.Width/2+glink.X)-(w/2), float64(glink.Height/2+glink.Y)-(h/2))
		text.Draw(buf, glink.Text, r.fontFace, glinkOp)
	}
}

// choiceDimOverlayColor dims the whole screen behind an active [glink]
// choice set (rgba(10,12,15,0.5) in the source design).
var choiceDimOverlayColor = color.RGBA{0x0a, 0x0c, 0x0f, 0x80}

// drawChoiceDimOverlay draws the full-screen dim behind the message box's
// operation row and [glink] choices — only while choices are actually up
// (the same len(glinks)>0 && !isJump guard messageBoxFillColor uses), so it
// never lingers a frame after a choice-triggered jump starts. Part of the
// メッセージ欄 redesign — this package had no such overlay before it,
// so it's gated the same way as everything else that redesign added (see
// messageBoxFillColor's doc comment).
func drawChoiceDimOverlay(r *Renderer, buf *ebiten.Image) {
	if r.manager.Config.MessageBoxStyle != "redesigned" {
		return
	}
	if len(glinks) == 0 || isJump {
		return
	}
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	fillRect(buf, 0, 0, float64(w), float64(h), choiceDimOverlayColor)
}
