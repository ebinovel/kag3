package effects

import (
	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

type FadeIn struct {}

// Draw fades image in from transparent to opaque over [baseTick, baseTick+time]
// at position (x, y). opacity/scaleX/scaleY/rotation are an additional,
// independent transform — e.g. from [anim] — applied on top of (multiplied
// with, for opacity) the fade-in progress itself; scale/rotation pivot on
// the image's own center. Pass opacity=1, scaleX=scaleY=1, rotation=0 for
// no extra transform.
func (f *FadeIn) Draw(screen, image *ebiten.Image, x, y, baseTick, tick, time int, opacity, scaleX, scaleY, rotation float64) {
	op := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	applyPivotedTransform(op, image, scaleX, scaleY, rotation)
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleAlpha(float32(t * opacity))
	screen.DrawImage(image, op)
}

// applyPivotedTransform scales and rotates around img's own center, so
// callers can then Translate to the final on-screen position without the
// image jumping around its top-left corner.
func applyPivotedTransform(op *ebiten.DrawImageOptions, img *ebiten.Image, scaleX, scaleY, rotation float64) {
	if scaleX == 1 && scaleY == 1 && rotation == 0 {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Scale(scaleX, scaleY)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(float64(w)/2, float64(h)/2)
}

func (f *FadeIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	nextOp.ColorScale.ScaleAlpha(float32(t))
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		bg.NextImage = nil
		bg.IsEnd = true
	}
}

type FadeOut struct {}

// Draw fades image out from opaque to transparent. Same opacity/scale/
// rotation extension as FadeIn.Draw.
//
// Fixes a pre-existing bug: the alpha-fade draw was gated on t>=1 (i.e.
// only once the fade had *finished*, at which point computed alpha is
// already ~0), so a fading-out character never actually rendered any
// visible fade — it just vanished on the first frame. Draws every frame
// now, like FadeIn does.
func (f *FadeOut) Draw(screen, image *ebiten.Image, x, y, baseTick, tick, time int, opacity, scaleX, scaleY, rotation float64) {
	op := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	applyPivotedTransform(op, image, scaleX, scaleY, rotation)
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleAlpha(float32((1 - t) * opacity))
	screen.DrawImage(image, op)
}

func (f *FadeOut) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	op.ColorScale.ScaleAlpha(float32(t))
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		bg.NextImage = nil
		bg.IsEnd = true
	}
}

type CrossFade struct {}

func (f *CrossFade) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	op.ColorScale.ScaleAlpha(float32(1-t))
	nextOp.ColorScale.ScaleAlpha(float32(t))
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		bg.NextImage = nil
		bg.IsEnd = true
	}
}
