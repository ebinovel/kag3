package ebitengine

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	register("layopt", handleLayopt)
	register("position", handlePosition)
	register("locate", handleLocate)
	register("clearfix", handleClearFix)
}

var layopt *kag3.LayOpt

func init() {
	layopt = &kag3.LayOpt{}
}

func handleLayopt(ctx *tagCtx) error {
	for key, value := range ctx.tag.Pm {
		switch key {
		case "layer":
			layopt.Layer = value
		case "page":
			layopt.Page = value
		case "visible":
			switch value {
			case "true":
				layopt.Visible = true
			case "false":
				layopt.Visible = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "left":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			layopt.Left = v
		case "top":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			layopt.Top = v
		case "opacity":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			layopt.Opacity = v
		}
	}
	return nil
}

func handlePosition(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	if v, ok := getString(pm, "layer"); ok {
		textPosition.Layer = v
	}
	if v, ok := getString(pm, "page"); ok {
		textPosition.Page = v
	}
	if v, ok, err := getInt(pm, "left"); err != nil {
		return err
	} else if ok {
		textPosition.Left = v
	}
	if v, ok, err := getInt(pm, "top"); err != nil {
		return err
	} else if ok {
		textPosition.Top = v
	}
	if v, ok, err := getInt(pm, "width"); err != nil {
		return err
	} else if ok {
		textPosition.Width = v
	}
	if v, ok, err := getInt(pm, "height"); err != nil {
		return err
	} else if ok {
		textPosition.Height = v
	}
	if v, ok := getString(pm, "frame"); ok {
		img, err := loadImage(r, "", v)
		if err != nil {
			return err
		}
		textPosition.FrameImage = img
		textPosition.FrameStorage = v
	}
	if v, ok := getString(pm, "color"); ok {
		r, g, b, err := parseColor(v)
		if err != nil {
			return err
		}
		textPosition.Color = color.RGBA{uint8(r), uint8(g), uint8(b), 0}
	}
	if v, ok := getString(pm, "border_color"); ok {
		r, g, b, err := parseColor(v)
		if err != nil {
			return err
		}
		textPosition.BorderColor = color.RGBA{uint8(r), uint8(g), uint8(b), 0}
	}
	if v, ok, err := getInt(pm, "border_size"); err != nil {
		return err
	} else if ok {
		textPosition.BorderSize = v
	}
	if v, ok, err := getInt(pm, "opacity"); err != nil {
		return err
	} else if ok {
		textPosition.Opacity = v
	}
	if v, ok, err := getInt(pm, "marginl"); err != nil {
		return err
	} else if ok {
		textPosition.MarginLeft = v
	}
	if v, ok, err := getInt(pm, "margint"); err != nil {
		return err
	} else if ok {
		textPosition.MarginTop = v
	}
	if v, ok, err := getInt(pm, "marginr"); err != nil {
		return err
	} else if ok {
		textPosition.MarginRight = v
	}
	if v, ok, err := getInt(pm, "marginb"); err != nil {
		return err
	} else if ok {
		textPosition.MarginBottom = v
	}
	if v, ok, err := getInt(pm, "marginn"); err != nil {
		return err
	} else if ok {
		textPosition.MarginN = v
	}
	if v, ok, err := getInt(pm, "radius"); err != nil {
		return err
	} else if ok {
		textPosition.Radius = v
	}
	// "vertial" is a real-Tyrano typo (not kag3's own) — kept for
	// compatibility with scripts that use it.
	if v, ok, err := getBool(pm, "vertial"); err != nil {
		return err
	} else if ok {
		textPosition.Vertical = v
	}
	if v, ok, err := getBool(pm, "vertical"); err != nil {
		return err
	} else if ok {
		textPosition.Vertical = v
	}
	if v, ok, err := getBool(pm, "visible"); err != nil {
		return err
	} else if ok {
		textPosition.Visible = v
	}
	if textPosition.Width != 0 && textPosition.Height != 0 {
		textPosition.BackImage = ebiten.NewImage(textPosition.Width, textPosition.Height)
	}
	return nil
}

// handleLocate repositions the message window's origin. Real Tyrano's
// [locate] moves a text cursor within the current layer; kag3's text
// drawing has no per-character cursor concept, so this instead moves
// textPosition itself — a simpler but still real and visible effect.
func handleLocate(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["x"]; ok {
		x, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		textPosition.Left = x
	}
	if v, ok := ctx.tag.Pm["y"]; ok {
		y, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		textPosition.Top = y
	}
	return nil
}

// handleClearFix clears the "fix" layer's content: all glinks (kag3 doesn't
// have a general per-layer visibility system yet, so glinks are always
// treated as living on the fix layer) plus any [button fix="true"] —
// buttons with Fix=false are already cleared on every jump (see Update() in
// renderer.go); Fix=true ones are deliberately exempted from that so they
// survive normal navigation, and this tag is the only thing that removes
// them, matching real Tyrano's config.ks calling [clearfix] on the way out.
func handleClearFix(ctx *tagCtx) error {
	glinks = nil
	kept := buttons[:0]
	for _, b := range buttons {
		if !b.Fix {
			kept = append(kept, b)
		}
	}
	buttons = kept
	return nil
}
