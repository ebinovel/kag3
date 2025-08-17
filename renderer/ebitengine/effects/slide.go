package effects

import (
	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

type SlideInRight struct {}

func (f *SlideInRight) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	screenWidth := float64(screen.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	nextOp.GeoM.Translate(screenWidth * t - screenWidth, 0)
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		bg.NextImage = nil
		bg.IsEnd = true
	}
}

type SlideInLeft struct {}

func (f *SlideInLeft) DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	screenWidth := float64(screen.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	nextOp.GeoM.Translate(screenWidth - screenWidth * t, 0)
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		bg.NextImage = nil
		bg.IsEnd = true
	}
}
