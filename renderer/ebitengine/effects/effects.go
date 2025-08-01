package effects

import (
	"github.com/hajimehoshi/ebiten/v2"
)

type Effecter interface {
	Draw(screen, image, nextImage *ebiten.Image, baseTick, tick, time int)
}
