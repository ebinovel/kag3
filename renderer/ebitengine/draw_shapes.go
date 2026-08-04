package ebitengine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// pixelImage is a 1x1 white image, scaled+translated per call by fillRect —
// avoids the per-call ebiten.NewImage(w,h) allocation the message window's
// new chrome (name tab padding math aside, text-speed segments, operation
// row dividers, glink accents) would otherwise repeat every frame.
var pixelImage *ebiten.Image

func init() {
	pixelImage = ebiten.NewImage(1, 1)
	pixelImage.Fill(color.White)
}

// fillRect draws a flat-colored w x h rectangle with its top-left at (x, y).
func fillRect(buf *ebiten.Image, x, y, w, h float64, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w, h)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	buf.DrawImage(pixelImage, op)
}
