package effects

import (
	"image"
	"math"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

// This file implements the kag3.BackgroundMethod values that aren't already
// covered by fade.go/slide.go (fadeIn, crossfade, slide/slideInRight). Each
// named method is a thin type wrapping one of the shared parameterized
// primitives below, so the ~35 remaining transitions share their actual
// tween logic instead of each reimplementing progress/completion handling.

// progress is the linear [0,1] fraction of a transition that started at
// baseTick and runs for timeMs milliseconds.
func progress(baseTick, tick, timeMs int) float64 {
	if timeMs <= 0 {
		return 1
	}
	p := float64(tick-baseTick) / float64(timeMs*ebiten.TPS()/1000)
	if p > 1 {
		return 1
	}
	if p < 0 {
		return 0
	}
	return p
}

// finishBackground swaps in the new image and marks the transition
// complete once p reaches 1. Every DrawBackground in this package calls
// this, so [bg wait=true]/[wt] reliably unblock for every BackgroundMethod,
// not just the original three.
func finishBackground(bg *kag3.Background, p float64) {
	if p >= 1 {
		bg.Image = bg.NextImage
		bg.NextImage = nil
		bg.IsEnd = true
	}
}

// easeOutBack overshoots slightly past 1 before settling, approximating a
// bounce/spring settle for the bounceIn* family.
func easeOutBack(p float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1
	x := p - 1
	return 1 + c3*x*x*x + c1*x*x
}

type edge int

const (
	edgeLeft edge = iota
	edgeRight
	edgeTop
	edgeBottom
)

// edgeOffset is the (dx, dy) an element at progress p should be displaced
// by distance units from its resting position, coming from e.
func edgeOffset(e edge, distance, p float64) (dx, dy float64) {
	remaining := 1 - p
	switch e {
	case edgeLeft:
		return -distance * remaining, 0
	case edgeRight:
		return distance * remaining, 0
	case edgeTop:
		return 0, -distance * remaining
	case edgeBottom:
		return 0, distance * remaining
	}
	return 0, 0
}

func screenDistance(screen *ebiten.Image, e edge) float64 {
	if e == edgeLeft || e == edgeRight {
		return float64(screen.Bounds().Dx())
	}
	return float64(screen.Bounds().Dy())
}

// drawSlideIn: the new image slides in fully opaque from edge e, fully
// covering the old one as it arrives (no fade). Generalizes SlideInRight.
func drawSlideIn(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int, e edge, ease func(float64) float64) {
	p := progress(baseTick, tick, timeMs)
	eased := p
	if ease != nil {
		eased = ease(p)
	}
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	dx, dy := edgeOffset(e, screenDistance(screen, e), eased)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(dx, dy)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawFadeDirectional: the new image fades in (alpha 0→1) while easing in
// from a short distance in direction e — a subtle directional entrance,
// unlike drawSlideIn's full-width traverse.
func drawFadeDirectional(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int, e edge) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	dx, dy := edgeOffset(e, screenDistance(screen, e)*0.15, p)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.Translate(dx, dy)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawZoom: the new image scales from startScale up to 1 around screen
// center while fading in; if e is non-nil it also eases in slightly from
// that edge (zoomInLeft/Right/Up/Down).
func drawZoom(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int, startScale float64, e *edge) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	w := float64(screen.Bounds().Dx())
	h := float64(screen.Bounds().Dy())
	scale := startScale + (1-startScale)*p
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(w/2, h/2)
	if e != nil {
		dx, dy := edgeOffset(*e, screenDistance(screen, *e)*0.3, p)
		op.GeoM.Translate(dx, dy)
	}
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawRotateIn: the new image rotates from ±angle to 0 around screen
// center while fading in.
func drawRotateIn(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int, startAngle float64) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	w := float64(screen.Bounds().Dx())
	h := float64(screen.Bounds().Dy())
	angle := startAngle * (1 - p)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Rotate(angle)
	op.GeoM.Translate(w/2, h/2)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawRollIn: slide-from-left combined with a settling rotation.
func drawRollIn(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	w := float64(screen.Bounds().Dx())
	h := float64(screen.Bounds().Dy())
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Rotate(math.Pi / 4 * (1 - p))
	op.GeoM.Translate(w/2, h/2)
	dx, _ := edgeOffset(edgeLeft, w, p)
	op.GeoM.Translate(dx, 0)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawPuff: the new image scales down from startScale (>1) to 1 while
// fading in — an outward "puff" settling to normal size.
func drawPuff(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int, startScale float64) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	w := float64(screen.Bounds().Dx())
	h := float64(screen.Bounds().Dy())
	scale := startScale - (startScale-1)*p
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(w/2, h/2)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawLightSpeedIn: fast horizontal slide-in from the right combined with a
// shear-like skew that settles out, plus fade.
func drawLightSpeedIn(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	w := float64(screen.Bounds().Dx())
	skew := 0.3 * (1 - p)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.SetElement(0, 1, skew)
	dx, _ := edgeOffset(edgeRight, w, p)
	op.GeoM.Translate(dx, 0)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawExplode: the old image scales up and fades out, revealing the new
// one (already drawn beneath) as if bursting apart.
func drawExplode(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.NextImage, &ebiten.DrawImageOptions{})
	w := float64(screen.Bounds().Dx())
	h := float64(screen.Bounds().Dy())
	scale := 1 + 0.8*p
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(1 - p))
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(w/2, h/2)
	screen.DrawImage(bg.Image, op)
	finishBackground(bg, p)
}

// drawFold: the old image squashes horizontally to nothing (like a page
// folding shut) over the new image drawn beneath it.
func drawFold(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.NextImage, &ebiten.DrawImageOptions{})
	scaleX := 1 - p
	if scaleX > 0.001 {
		w := float64(screen.Bounds().Dx())
		h := float64(screen.Bounds().Dy())
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-w/2, -h/2)
		op.GeoM.Scale(scaleX, 1)
		op.GeoM.Translate(w/2, h/2)
		screen.DrawImage(bg.Image, op)
	}
	finishBackground(bg, p)
}

// drawShake: the new image fades in quickly, then jitters horizontally
// with decaying amplitude before settling centered.
func drawShake(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	amplitude := 20 * (1 - p)
	offset := amplitude * math.Sin(p*30)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(float32(p))
	op.GeoM.Translate(offset, 0)
	screen.DrawImage(bg.NextImage, op)
	finishBackground(bg, p)
}

// drawBlind reveals the new image left-to-right through a set of
// horizontal stripes, like venetian blinds opening.
func drawBlind(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	bounds := bg.NextImage.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	revealW := int(float64(w) * p)
	if revealW > 0 {
		const stripes = 10
		stripeH := h / stripes
		if stripeH <= 0 {
			stripeH = 1
		}
		for i := 0; i < stripes; i++ {
			y0 := i * stripeH
			y1 := y0 + stripeH
			if i == stripes-1 {
				y1 = h
			}
			sub, ok := bg.NextImage.SubImage(image.Rect(0, y0, revealW, y1)).(*ebiten.Image)
			if !ok || sub == nil {
				continue
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(0, float64(y0))
			screen.DrawImage(sub, op)
		}
	}
	finishBackground(bg, p)
}

// drawClip reveals the new image through a rectangular clip expanding
// outward from screen center.
func drawClip(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, timeMs int) {
	p := progress(baseTick, tick, timeMs)
	screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
	bounds := bg.NextImage.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	cw := int(float64(w) * p)
	ch := int(float64(h) * p)
	if cw > 0 && ch > 0 {
		x0 := (w - cw) / 2
		y0 := (h - ch) / 2
		sub, ok := bg.NextImage.SubImage(image.Rect(x0, y0, x0+cw, y0+ch)).(*ebiten.Image)
		if ok && sub != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x0), float64(y0))
			screen.DrawImage(sub, op)
		}
	}
	finishBackground(bg, p)
}

// --- Named BackgroundMethod types, one per kag3.BackgroundMethod entry not
// already covered by fade.go/slide.go. ---

type Explode struct{}

func (e *Explode) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawExplode(screen, bg, baseTick, tick, time)
}

type Blind struct{}

func (e *Blind) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawBlind(screen, bg, baseTick, tick, time)
}

type Bounce struct{}

func (e *Bounce) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeBottom, easeOutBack)
}

type Clip struct{}

func (e *Clip) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawClip(screen, bg, baseTick, tick, time)
}

type Drop struct{}

func (e *Drop) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeTop, easeOutBack)
}

type Fold struct{}

func (e *Fold) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawFold(screen, bg, baseTick, tick, time)
}

type Puff struct{}

func (e *Puff) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawPuff(screen, bg, baseTick, tick, time, 1.5)
}

type Scale struct{}

func (e *Scale) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawZoom(screen, bg, baseTick, tick, time, 0.5, nil)
}

type Shake struct{}

func (e *Shake) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawShake(screen, bg, baseTick, tick, time)
}

type Size struct{}

func (e *Size) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawZoom(screen, bg, baseTick, tick, time, 0, nil)
}

type FadeInDown struct{}

func (e *FadeInDown) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawFadeDirectional(screen, bg, baseTick, tick, time, edgeTop)
}

type FadeInLeft struct{}

func (e *FadeInLeft) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawFadeDirectional(screen, bg, baseTick, tick, time, edgeLeft)
}

type FadeInRight struct{}

func (e *FadeInRight) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawFadeDirectional(screen, bg, baseTick, tick, time, edgeRight)
}

type FadeInUp struct{}

func (e *FadeInUp) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawFadeDirectional(screen, bg, baseTick, tick, time, edgeBottom)
}

type LightSpeedIn struct{}

func (e *LightSpeedIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawLightSpeedIn(screen, bg, baseTick, tick, time)
}

type RotateIn struct{}

func (e *RotateIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawRotateIn(screen, bg, baseTick, tick, time, math.Pi/3)
}

type RotateInDownLeft struct{}

func (e *RotateInDownLeft) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawRotateIn(screen, bg, baseTick, tick, time, math.Pi/4)
}

type RotateInDownRight struct{}

func (e *RotateInDownRight) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawRotateIn(screen, bg, baseTick, tick, time, -math.Pi/4)
}

type RotateInUpLeft struct{}

func (e *RotateInUpLeft) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawRotateIn(screen, bg, baseTick, tick, time, -math.Pi/4)
}

type RotateInUpRight struct{}

func (e *RotateInUpRight) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawRotateIn(screen, bg, baseTick, tick, time, math.Pi/4)
}

type ZoomIn struct{}

func (e *ZoomIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawZoom(screen, bg, baseTick, tick, time, 0.3, nil)
}

type ZoomInDown struct{}

func (e *ZoomInDown) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	edg := edgeTop
	drawZoom(screen, bg, baseTick, tick, time, 0.3, &edg)
}

type ZoomInLeft struct{}

func (e *ZoomInLeft) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	edg := edgeLeft
	drawZoom(screen, bg, baseTick, tick, time, 0.3, &edg)
}

type ZoomInRight struct{}

func (e *ZoomInRight) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	edg := edgeRight
	drawZoom(screen, bg, baseTick, tick, time, 0.3, &edg)
}

type ZoomInUp struct{}

func (e *ZoomInUp) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	edg := edgeBottom
	drawZoom(screen, bg, baseTick, tick, time, 0.3, &edg)
}

type SlideInDown struct{}

func (e *SlideInDown) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeTop, nil)
}

type SlideInUp struct{}

func (e *SlideInUp) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeBottom, nil)
}

type BounceIn struct{}

func (e *BounceIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawZoom(screen, bg, baseTick, tick, time, 0.3, nil)
}

type BounceInDown struct{}

func (e *BounceInDown) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeTop, easeOutBack)
}

type BounceInLeft struct{}

func (e *BounceInLeft) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeLeft, easeOutBack)
}

type BounceInRight struct{}

func (e *BounceInRight) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeRight, easeOutBack)
}

type BounceInUp struct{}

func (e *BounceInUp) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawSlideIn(screen, bg, baseTick, tick, time, edgeBottom, easeOutBack)
}

type RollIn struct{}

func (e *RollIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawRollIn(screen, bg, baseTick, tick, time)
}

type VanishIn struct{}

func (e *VanishIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawZoom(screen, bg, baseTick, tick, time, 0.6, nil)
}

type PuffIn struct{}

func (e *PuffIn) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	drawPuff(screen, bg, baseTick, tick, time, 1.3)
}
