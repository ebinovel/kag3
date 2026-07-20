package effects

import (
	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

type FadeIn struct {}

func (f *FadeIn) Draw(screen, image *ebiten.Image, x, y, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	op.ColorScale.ScaleAlpha(float32(t))
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(image, op)
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

func (f *FadeOut) Draw(screen, image *ebiten.Image, x, y, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t >= 1 {
		op.ColorScale.ScaleAlpha(1 - float32(t))
		op.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(image, op)
	}
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
