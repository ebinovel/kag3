package ebitengine

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/ebinovel/kag3"
)

func init() {
	register("p", handleP)
	register("l", handleL)
	register("r", handleTextR)
	register("s", handleS)
	register("cm", handleCM)
	register("ruby", handleRuby)
	register("font", handleFont)
	register("resetfont", handleResetFont)
	register("ptext", handlePText)
}

func handleP(ctx *tagCtx) error {
	r := ctx.r
	ctx.y.Until(true, isTextEnded)
	r.texts = make(map[int][]Text)
	isWait = false
	charaName = ""
	pendingRuby = ""
	return nil
}

func handleL(ctx *tagCtx) error {
	ctx.y.Until(true, isClicked)
	return nil
}

// handleTextR handles the [r] (line break) tag. Named to avoid clashing
// with the receiver variable convention used elsewhere.
func handleTextR(ctx *tagCtx) error {
	return nil
}

func handleS(ctx *tagCtx) error {
	ctx.y.Until(true, isJumped)
	return nil
}

func handleCM(ctx *tagCtx) error {
	r := ctx.r
	r.texts = make(map[int][]Text)
	isWait = false
	charaName = ""
	pendingRuby = ""
	return nil
}

func handleRuby(ctx *tagCtx) error {
	pendingRuby = ctx.tag.Pm["text"]
	return nil
}

func handleFont(ctx *tagCtx) error {
	return ctx.r.textStyle(ctx.tag)
}

func handleResetFont(ctx *tagCtx) error {
	textStyle = nil
	return nil
}

func handlePText(ctx *tagCtx) error {
	object := ctx.tag
	pText := &kag3.PText{}
	rr, gg, bb, aa := color.White.RGBA()
	pText.Color = &color.RGBA{uint8(rr), uint8(gg), uint8(bb), uint8(aa)}
	for key, value := range object.Pm {
		switch key {
		case "name":
			pText.Name = value
		case "layer":
			pText.Layer = value
		case "page":
			pText.Page = value
		case "text":
			pText.Text = value
		case "x":
			pText.X, _ = strconv.Atoi(value)
		case "y":
			pText.Y, _ = strconv.Atoi(value)
		case "vertical":
			pText.Vertical, _ = strconv.ParseBool(value)
		case "size":
			pText.Size, _ = strconv.Atoi(value)
		case "face":
			pText.Face = value
		case "color", "edge", "shadow":
			r, g, b, _ := parseColor(value)
			switch key {
			case "color":
				pText.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			case "edge":
				pText.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			case "shadow":
				pText.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			}
		case "bold":
			switch value {
			case "true":
				pText.Bold = true
			case "false":
				pText.Bold = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "width":
			pText.Width, _ = strconv.Atoi(value)
		case "align":
			switch value {
			case "left", "center", "right":
				pText.Align = value
			}
		case "time":
			pText.Time, _ = strconv.Atoi(value)
		case "overwrite":
			switch value {
			case "true":
				pText.Overwrite = true
			case "false":
				pText.Overwrite = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "gradient":
			pText.Gradient = value
		}
	}
	ptexts[pText.Name] = pText
	return nil
}
