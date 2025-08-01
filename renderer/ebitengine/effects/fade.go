package effects

import (
	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

type FadeIn struct {}

func (f *FadeIn) Draw(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
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
		screen.DrawImage(bg.Image, op)
	}
}

type FadeOut struct {}

func (f *FadeOut) Draw(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
}

type CrossFade struct {}

func (f *CrossFade) Draw(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	op.ColorScale.ScaleAlpha(float32(1 - t))
	nextOp.ColorScale.ScaleAlpha(float32(t))
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		screen.DrawImage(bg.Image, op)
	}
}
