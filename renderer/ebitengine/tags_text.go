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
	// A pending isJump (goToTitle, applySaveData, or any [link]/[glink]/
	// [button target=] jump) means this [p]'s own screen is being abandoned
	// outright — see isTextEndedOrJumped's doc comment (state.go). Clearing
	// r.texts/isWait unconditionally here is fine for an ordinary
	// same-storage jump (whatever the destination shows next puts up its
	// own text before its own next [p]), but a same-process load
	// (applySaveData) already wrote the *destination*'s restored
	// r.texts/isWait by the time this runs — its isJump releases whatever
	// [p] the coroutine happened to be blocked on well before jumpIndex is
	// even consumed by initScript's outer loop (macro.go), i.e. before the
	// destination's own tags ever run — so clearing here would immediately
	// wipe out the just-restored state before the player ever sees it,
	// leaving the message window (and the loaded line's dialogue) blank
	// after every load. Skip the clear whenever a jump is why this [p] let
	// go; a real page-boundary reset still happens naturally for every
	// ordinary (non-jump) [p].
	if !isJump {
		r.texts = make(map[int][]Text)
		isWait = false
	}
	// charaName deliberately not cleared here — see its doc comment
	// (tags_character.go): the name-plate must survive [p] to keep
	// showing the current speaker across a multi-line "#name" block.
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
	isBlockedOnStop = true
	ctx.y.Until(true, isJumped)
	isBlockedOnStop = false
	return nil
}

func handleCM(ctx *tagCtx) error {
	r := ctx.r
	recordBacklog(r)
	r.texts = make(map[int][]Text)
	isWait = false
	// charaName deliberately not cleared here — see its doc comment
	// (tags_character.go). scene1.ks itself relies on this: "#あかね"
	// immediately followed by "[cm]" (still under the same speaker) would
	// otherwise blank the name-plate right back out before the next line
	// ever displays.
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
//
// Copies rather than mutates the existing textStyle object in place:
// earlier Text segments (appendRubyText, macro.go) and the marker segment
// appended below may hold a reference to the *previous* textStyle value,
// and [l] (unlike [p]) keeps them on screen alongside whatever this call
// creates — mutating that shared object's fields in place would silently
// reskin every one of them too, not just text created from here on. See
// applyTextStyle's doc comment for the matching read-side half of this.
func applyFontAttrs(ctx *tagCtx) error {
	r := ctx.r
	tag := ctx.tag
	next := copyTextStyle(textStyle)
	if next == nil {
		next = &kag3.TextStyle{}
	}
	pm := tag.Pm
	if v, ok, err := getInt(pm, "size"); err != nil {
		return err
	} else if ok {
		next.Size = v
	}
	// parseColor's error is intentionally ignored here, matching the
	// pre-existing behavior of this handler (see resolveFolderImage's
	// sibling color cases elsewhere for the one place that does check it).
	if v, ok := getString(pm, "color"); ok {
		r, g, b, _ := parseColor(v)
		next.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if v, ok := getString(pm, "edge"); ok {
		r, g, b, _ := parseColor(v)
		next.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if v, ok := getString(pm, "shadow"); ok {
		r, g, b, _ := parseColor(v)
		next.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if _, ok := getString(pm, "bold"); ok {
		next.IsBold = true
	}
	if _, ok := getString(pm, "itaric"); ok {
		next.IsItaric = true
	}
	textStyle = next
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
			// 255, not 0: an explicit [ptext color=/edge=/shadow=] means the
			// author wants a visible color — matches applyFontAttrs' own
			// convention just above for [font]/[deffont]. Color is drawn via
			// op.ColorScale.ScaleWithColor(pt.Color) (renderer.go's
			// drawPTexts), which scales alpha too, so 0 here would
			// make every explicitly-colored [ptext] transparent.
			r, g, b, _ := parseColor(value)
			switch key {
			case "color":
				pText.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "edge":
				pText.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "shadow":
				pText.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
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
		case "bg":
			img, err := loadImage(ctx.r, "", value)
			if err != nil {
				return err
			}
			pText.BgStorage = value
			pText.BgImage = img
		}
	}
	ptexts[pText.Name] = pText
	return nil
}
