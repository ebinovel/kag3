package ebitengine

import "github.com/hajimehoshi/ebiten/v2"

func init() {
	register("title", handleTitle)
}

func handleTitle(ctx *tagCtx) error {
	ebiten.SetWindowTitle(ctx.tag.Pm["name"])
	return nil
}
