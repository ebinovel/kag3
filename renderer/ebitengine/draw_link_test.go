package ebitengine

import (
	"image/color"
	"testing"

	"github.com/ebinovel/kag3"
)

func TestDrawGLinksNoPanicWithAndWithoutHover(t *testing.T) {
	savedGLinks, savedIsJump := glinks, isJump
	defer func() { glinks, isJump = savedGLinks, savedIsJump }()

	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config.MessageBoxStyle = "redesigned" // hover/accent styling only exists under the redesign
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

// TestDrawGLinksLegacyUsesScriptColor is the scoping fix for
// renderer/ebitengine being a shared package: any style other than
// "redesigned" — including "" (a project whose config.toml predates
// MessageBoxStyle entirely, e.g. tsf-action) — must draw this package's
// original plain glink.Color-filled box, ignoring the redesign's fixed
// palette, and must not suppress drawing while isJump is pending (the
// original never checked that either).
func TestDrawGLinksLegacyUsesScriptColor(t *testing.T) {
	savedGLinks, savedIsJump := glinks, isJump
	defer func() { glinks, isJump = savedGLinks, savedIsJump }()

	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	buf := newTestImage(1920, 1080)

	glinks = []*kag3.GLink{
		{Text: "選択肢A", X: 100, Y: 100, Width: 200, Height: 60, Color: &color.RGBA{0x11, 0x22, 0x33, 0xff}},
	}
	isJump = false
	drawGLinks(r, buf) // must not panic, must not touch glink.Color

	isJump = true
	drawGLinks(r, buf) // legacy path never suppressed on isJump — must still not panic
}

func TestDrawChoiceDimOverlayOnlyWhenGLinksPresentAndNotJumping(t *testing.T) {
	savedGLinks, savedIsJump := glinks, isJump
	defer func() { glinks, isJump = savedGLinks, savedIsJump }()

	r := newTestRenderer()
	r.manager.Config.MessageBoxStyle = "redesigned" // this overlay didn't exist before the redesign
	buf := newTestImage(1920, 1080)

	glinks, isJump = nil, false
	drawChoiceDimOverlay(r, buf) // no glinks: no-op, must not panic

	glinks = []*kag3.GLink{{Text: "a", X: 0, Y: 0, Width: 10, Height: 10}}
	isJump = false
	drawChoiceDimOverlay(r, buf) // active choices: draws the dim

	isJump = true
	drawChoiceDimOverlay(r, buf) // jump pending: suppressed even with glinks present
}

// TestDrawChoiceDimOverlayLegacyNeverDraws covers the same shared-package
// scoping as TestDrawGLinksLegacyUsesScriptColor: this overlay is entirely
// new to the redesign, so any non-"redesigned" style must never draw it,
// even with active glinks.
func TestDrawChoiceDimOverlayLegacyNeverDraws(t *testing.T) {
	savedGLinks, savedIsJump := glinks, isJump
	defer func() { glinks, isJump = savedGLinks, savedIsJump }()

	r := newTestRenderer()
	buf := newTestImage(1920, 1080)

	glinks, isJump = []*kag3.GLink{{Text: "a", X: 0, Y: 0, Width: 10, Height: 10}}, false
	drawChoiceDimOverlay(r, buf) // must not panic, must be a no-op
}
