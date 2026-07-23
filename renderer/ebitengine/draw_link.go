package ebitengine

import (
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

// drawGLinks draws every [glink]'s filled background rect plus its
// centered label text.
func drawGLinks(r *Renderer, buf *ebiten.Image) {
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
