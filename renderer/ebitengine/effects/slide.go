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

// Draw reads chara.Opacity/ScaleX/ScaleY/Rotation directly (unlike
// FadeIn/FadeOut.Draw, which take them as parameters) since chara is
// already passed in here.
func (f *SlideInLeft) Draw(screen, img *ebiten.Image, chara *kag3.CharaShow, baseTick, tick, time int) {
	op := &ebiten.DrawImageOptions{}
	t := float64(tick - baseTick) / float64(time * ebiten.TPS() / 1000)
	if t > 1 {
		t = 1
	}
	applyPivotedTransform(op, img, chara.ScaleX, chara.ScaleY, chara.Rotation)
	op.GeoM.Translate(float64(chara.Left) + float64(chara.NewLeft - chara.Left) * t, float64(chara.Top))
	op.ColorScale.ScaleAlpha(float32(chara.Opacity / 255))
	screen.DrawImage(img, op)
	if t == 1 {
		chara.IsSlide = false
		chara.Left = chara.NewLeft
	}
}

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
