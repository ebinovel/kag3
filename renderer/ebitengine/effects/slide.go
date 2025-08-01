package effects

import (
	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

type SlideInLeft struct {}

func (s *SlideInLeft) Draw(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	screenWidth := screen.Bounds().Dx()
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	nextOp.GeoM.Translate(float64(screenWidth) - float64(screenWidth) * t, 0)
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		screen.DrawImage(bg.Image, op)
	}
}

type SlideInRight struct {}

func (s *SlideInRight) Draw(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int) {
	screenWidth := screen.Bounds().Dx()
	op := &ebiten.DrawImageOptions{}
	nextOp := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	nextOp.GeoM.Translate(float64(screenWidth) * t - float64(screenWidth), 0)
	screen.DrawImage(bg.Image, op)
	screen.DrawImage(bg.NextImage, nextOp)
	if t == 1 {
		bg.Image = bg.NextImage
		screen.DrawImage(bg.Image, op)
	}
}
