package ebitengine

import "github.com/hajimehoshi/ebiten/v2"

// drawImages draws every [image]/[graph], applying its Opacity/ScaleX/
// ScaleY/Rotation and any per-layer blend mode ([layermode]/[free_layermode],
// tags_effects.go).
func drawImages(buf *ebiten.Image) {
	for _, img := range imgs {
		imgOp := &ebiten.DrawImageOptions{}
		applyPivotedImageTransform(imgOp, img.Image, img.ScaleX, img.ScaleY, img.Rotation)
		imgOp.GeoM.Translate(float64(img.X), float64(img.Y))
		imgOp.ColorScale.ScaleAlpha(float32(img.Opacity / 255))
		if blend, ok := layerBlend[img.Layer]; ok {
			imgOp.Blend = blend
		}
		buf.DrawImage(img.Image, imgOp)
	}
}

// applyPivotedImageTransform scales/rotates op around img's own center —
// the same convention as effects.applyPivotedTransform, duplicated here
// (unexported there) since [image]'s Opacity/ScaleX/ScaleY/Rotation are
// applied directly in drawImages rather than through an effects.* type.
func applyPivotedImageTransform(op *ebiten.DrawImageOptions, img *ebiten.Image, scaleX, scaleY, rotation float64) {
	if scaleX == 1 && scaleY == 1 && rotation == 0 {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Scale(scaleX, scaleY)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(float64(w)/2, float64(h)/2)
}
