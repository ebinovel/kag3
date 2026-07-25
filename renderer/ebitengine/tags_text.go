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

var (
	// ptexts holds every named [ptext] area, keyed by its "name"
	// attribute — ptext is general-purpose text placement, not just the
	// character name-plate. Which one (if any) doubles as the name-plate
	// is set by [chara_config ptext="..."] into charaNamePText.
	ptexts         map[string]*kag3.PText
	charaNamePText string
	pendingRuby    string
	// isWait marks "the current line has finished revealing, waiting for a
	// click to advance" — text objects block on this specifically (see [s]
	// below and execItem in macro.go), independent of isJump.
	isWait     bool
	isTextEnd  bool
	textStartT int
	prevLine   int
)

func init() {
	ptexts = make(map[string]*kag3.PText)
}

func handleP(ctx *tagCtx) error {
	r := ctx.r
	ctx.y.Until(true, isTextEndedOrJumped)
	recordBacklog(r)
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
	recordBacklog(r)
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
	return applyFontAttrs(ctx)
}

// applyFontAttrs implements [font]/[mark]'s attribute parsing, shared with
// [deffont] (handleDefFont, tags_message.go) which applies the same
// attributes to defaultTextStyle instead of the live textStyle.
func applyFontAttrs(ctx *tagCtx) error {
	r := ctx.r
	tag := ctx.tag
	if textStyle == nil {
		textStyle = &kag3.TextStyle{}
	}
	pm := tag.Pm
	if v, ok, err := getInt(pm, "size"); err != nil {
		return err
	} else if ok {
		textStyle.Size = v
	}
	// parseColor's error is intentionally ignored here, matching the
	// pre-existing behavior of this handler (see resolveFolderImage's
	// sibling color cases elsewhere for the one place that does check it).
	if v, ok := getString(pm, "color"); ok {
		r, g, b, _ := parseColor(v)
		textStyle.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if v, ok := getString(pm, "edge"); ok {
		r, g, b, _ := parseColor(v)
		textStyle.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if v, ok := getString(pm, "shadow"); ok {
		r, g, b, _ := parseColor(v)
		textStyle.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if _, ok := getString(pm, "bold"); ok {
		textStyle.IsBold = true
	}
	if _, ok := getString(pm, "itaric"); ok {
		textStyle.IsItaric = true
	}
	r.texts[tag.Line] = append(r.texts[tag.Line], Text{TextStyle: textStyle})
	return nil
}

// handleResetFont reverts to [deffont]'s configured default (nil, i.e. the
// renderer's built-in look, if none was ever set) rather than always
// clearing to nil — see tags_message.go.
func handleResetFont(ctx *tagCtx) error {
	textStyle = defaultTextStyle
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
