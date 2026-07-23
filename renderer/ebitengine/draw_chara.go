package ebitengine

import (
	"slices"

	"github.com/ebinovel/kag3"
	"github.com/ebinovel/kag3/renderer/ebitengine/effects"
	"github.com/hajimehoshi/ebiten/v2"
)

// drawCharacters draws every entry in viewCharas — a slide-in, fade-out (on
// its way to being removed), or fade-in tween depending on its current
// state — plus each one's currently active [chara_layer] parts on top.
func drawCharacters(buf *ebiten.Image) {
	for _, chara := range viewCharas {
		// A registered=false entry here means a loaded save's character
		// couldn't be reconciled (see reconcileViewCharas in tags_save.go)
		// — that function is meant to filter these out before they ever
		// reach viewCharas, but skip defensively rather than crash the
		// whole renderer if that invariant is ever violated.
		registered, ok := charas[chara.Name]
		if !ok || registered.Image == nil {
			continue
		}
		if chara.IsSlide {
			e := &effects.SlideInLeft{}
			e.Draw(buf, registered.Image, chara, charaTick, t, chara.Time)
		} else {
			if chara.IsRemove {
				e := &effects.FadeOut{}
				e.Draw(buf, registered.Image, chara.Left, chara.Top, charaTick, t, chara.Time,
					chara.Opacity/255, chara.ScaleX, chara.ScaleY, chara.Rotation)
			} else {
				e := &effects.FadeIn{}
				e.Draw(buf, registered.Image, chara.Left, chara.Top, charaTick, t, chara.Time,
					chara.Opacity/255, chara.ScaleX, chara.ScaleY, chara.Rotation)
			}
		}
		if !chara.IsRemove {
			drawCharaParts(buf, registered, chara.Left, chara.Top)
		}
	}
}

// drawCharaParts overlays a character's currently active differential
// parts (see [chara_layer]/[chara_part]) on top of its base image, aligned
// to the same origin. Layers are drawn in sorted-name order for
// determinism since Tyrano-style z-index configuration isn't implemented.
func drawCharaParts(screen *ebiten.Image, c *kag3.Character, left, top int) {
	if c == nil || len(c.ActivePart) == 0 {
		return
	}
	layerNames := make([]string, 0, len(c.ActivePart))
	for layer := range c.ActivePart {
		layerNames = append(layerNames, layer)
	}
	slices.Sort(layerNames)
	for _, layer := range layerNames {
		part := c.ActivePart[layer]
		img, ok := c.Parts[layer][part]
		if !ok {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(left), float64(top))
		screen.DrawImage(img, op)
	}
}
