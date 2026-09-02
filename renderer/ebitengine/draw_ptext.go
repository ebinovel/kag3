package ebitengine

import (
	"image/color"
	"slices"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// drawPTexts renders every named [ptext] area. The one registered as the
// character name-plate via [chara_config ptext="..."] shows charaName;
// every other one shows its own literal .Text. Sorted by name for
// deterministic draw order.
//
// A [ptext bg="..."] area draws its BgImage first, with the text offset by
// (ptextBgPaddingX, ptextBgPaddingY) into it (draw_messagebox.go) — a plain
// "image's own top-left + fixed padding" rule, not per-image 9-slice
// metadata. When the resolved content is empty (e.g. the monologue case,
// charaName == "") the whole area — background image included — is skipped
// entirely, so an empty name never leaves an orphaned tab graphic on screen.
func drawPTexts(r *Renderer, screen *ebiten.Image) {
	if len(ptexts) == 0 {
		return
	}
	names := make([]string, 0, len(ptexts))
	for name := range ptexts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		pt := ptexts[name]
		content := ptextContent(name)
		if content == "" {
			continue
		}
		textX, textY := float64(pt.X), float64(pt.Y)
		if pt.BgImage != nil {
			bgOp := &ebiten.DrawImageOptions{}
			bgOp.GeoM.Translate(textX, textY)
			screen.DrawImage(pt.BgImage, bgOp)
			textX += ptextBgPaddingX
			textY += ptextBgPaddingY
		}
		op := &text.DrawOptions{}
		op.GeoM.Translate(textX, textY)
		if pt.Color != nil {
			op.ColorScale.ScaleWithColor(pt.Color)
		} else {
			op.ColorScale.ScaleWithColor(color.White)
		}
		text.Draw(screen, content, ptextFace(r, pt), op)
	}
}

// ptextFace builds the font face a [ptext] area draws with: its own size=
// (config.ks's settings screen needs several distinct sizes — 34/26/22/20 —
// on screen at once) falling back to r.nameFontFace's size when size= was
// never given, which is what the character name-plate relies on.
func ptextFace(r *Renderer, pt *kag3.PText) *text.GoTextFace {
	size := float64(pt.Size)
	if size == 0 {
		size = r.nameFontFace.Size
	}
	return &text.GoTextFace{Source: r.nameFontFace.Source, Size: size, Language: r.nameFontFace.Language}
}

// ptextContent is the string a named ptext area should currently display:
// displayCharaName(charaName) for the one registered via
// [chara_config ptext=...], its own literal .Text otherwise. Split out from
// drawPTexts so the name-plate resolution logic is testable without an
// ebiten screen/font.
func ptextContent(name string) string {
	if name == charaNamePText {
		return displayCharaName(charaName)
	}
	if pt, ok := ptexts[name]; ok {
		return pt.Text
	}
	return ""
}

// displayCharaName resolves charaName (the internal name every other
// lookup keys by — playCharaVoice, [speak_config], [fuki_chara], save data)
// to what the name-plate should actually show: charas[name].JName when the
// character was registered with jname= ([chara_new]/[chara_new_psd]), the
// internal name unchanged otherwise (no jname given, or name not a
// registered character at all — including "" for a monologue line, which
// ptextContent then treats as empty content and skips the area entirely).
func displayCharaName(name string) string {
	if c, ok := charas[name]; ok && c.JName != "" {
		return c.JName
	}
	return name
}
