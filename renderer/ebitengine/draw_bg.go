package ebitengine

import (
	"github.com/ebinovel/kag3/renderer/ebitengine/effects"
	"github.com/hajimehoshi/ebiten/v2"
)

// drawBackground draws bg, then bg2 (a secondary background layer
// composited on top — see [bg2] in tags_background.go), each running its
// own crossfade/slide transition via effects.Transitions while
// bg.NextImage/bg2.NextImage is set.
func drawBackground(buf *ebiten.Image) {
	if bg.Image != nil {
		buf.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
		if bg.NextImage != nil {
			if transition, ok := effects.Transitions[bg.Method]; ok {
				transition.DrawBackground(buf, bg, bgTick, t, bg.Time)
			}
		}
	}
	if bg2.Image != nil {
		buf.DrawImage(bg2.Image, &ebiten.DrawImageOptions{})
		if bg2.NextImage != nil {
			if transition, ok := effects.Transitions[bg2.Method]; ok {
				transition.DrawBackground(buf, bg2, bg2Tick, t, bg2.Time)
			}
		}
	}
}
