package ebitengine

import (
	"fmt"
	"strconv"
)

func init() {
	register("layopt", handleLayopt)
	register("position", handlePosition)
	register("locate", handleLocate)
	register("clearfix", handleClearFix)
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
	return ctx.r.position(ctx.tag)
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

// handleClearFix clears the "fix" layer's content. kag3 doesn't have a
// general per-layer visibility system yet (layopt.Layer is tracked but
// nothing filters rendering by it), so this targets glinks specifically —
// the fixed navigation-link elements that are the actual real-world use of
// a "fix" layer in the example scenarios.
func handleClearFix(ctx *tagCtx) error {
	glinks = nil
	return nil
}
