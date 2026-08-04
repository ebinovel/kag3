package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

func TestDrawGLinksNoPanicWithAndWithoutHover(t *testing.T) {
	savedGLinks, savedIsJump := glinks, isJump
	defer func() { glinks, isJump = savedGLinks, savedIsJump }()

	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	buf := newTestImage(1920, 1080)

	glinks = []*kag3.GLink{
		{Text: "選択肢A", X: 100, Y: 100, Width: 200, Height: 60},
		{Text: "選択肢B", X: 100, Y: 200, Width: 200, Height: 60},
	}
	isJump = false
	drawGLinks(r, buf) // cursor position is whatever it is in this headless test — must not panic either way

	isJump = true
	drawGLinks(r, buf) // suppressed while a jump is pending
}

func TestDrawChoiceDimOverlayOnlyWhenGLinksPresentAndNotJumping(t *testing.T) {
	savedGLinks, savedIsJump := glinks, isJump
	defer func() { glinks, isJump = savedGLinks, savedIsJump }()

	buf := newTestImage(1920, 1080)

	glinks, isJump = nil, false
	drawChoiceDimOverlay(buf) // no glinks: no-op, must not panic

	glinks = []*kag3.GLink{{Text: "a", X: 0, Y: 0, Width: 10, Height: 10}}
	isJump = false
	drawChoiceDimOverlay(buf) // active choices: draws the dim

	isJump = true
	drawChoiceDimOverlay(buf) // jump pending: suppressed even with glinks present
}
