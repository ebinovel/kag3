package ebitengine

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("button", handleButton)
	register("glink", handleGLink)
	register("glink_config", handleGLinkConfig)
	register("link", handleLink)
	register("clickable", handleClickable)
}

var (
	buttons []*kag3.Button
	glinks  []*kag3.GLink
	links   []*kag3.Link
)

// glinkDefault* are [glink_config]'s settings, used by handleGLink for any
// attribute a particular [glink] call doesn't specify — most notably Color,
// since drawScene fills a glink's background with it unconditionally
// (a glink with no configured color and no default would fill with a nil
// *color.RGBA and panic).
var (
	glinkDefaultColor = &color.RGBA{70, 70, 180, 220}
	glinkDefaultSize  int
	glinkDefaultFace  string
)

func handleGLinkConfig(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["color"]; ok {
		r, g, b, err := parseColor(v)
		if err != nil {
			return err
		}
		glinkDefaultColor = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if v, ok := ctx.tag.Pm["size"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		glinkDefaultSize = n
	}
	if v, ok := ctx.tag.Pm["face"]; ok {
		glinkDefaultFace = v
	}
	return nil
}

func handleButton(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	button := &kag3.Button{}
	if v, ok := getString(pm, "graphic"); ok {
		img, err := loadImage(r, pm["folder"], v)
		if err != nil {
			return err
		}
		button.Graphic = img
	}
	if v, ok := getString(pm, "storage"); ok {
		button.Storage = v
	}
	if v, ok := getString(pm, "target"); ok {
		button.Target = v
	}
	if v, ok := getString(pm, "name"); ok {
		button.Name = v
	}
	if v, ok, err := getInt(pm, "x"); err != nil {
		return err
	} else if ok {
		button.X = v
	}
	if v, ok, err := getInt(pm, "y"); err != nil {
		return err
	} else if ok {
		button.Y = v
	}
	if v, ok, err := getInt(pm, "width"); err != nil {
		return err
	} else if ok {
		button.Width = v
	}
	if v, ok, err := getInt(pm, "height"); err != nil {
		return err
	} else if ok {
		button.Height = v
	}
	if v, ok, err := getBool(pm, "fix"); err != nil {
		return err
	} else if ok {
		button.Fix = v
	}
	if v, ok := getString(pm, "role"); ok {
		button.Role = v
	}
	if v, ok := getString(pm, "hint"); ok {
		button.Hint = v
	}
	if v, ok := getString(pm, "clickse"); ok {
		button.ClickSE = v
	}
	if v, ok := getString(pm, "enterse"); ok {
		button.EnterSE = v
	}
	if v, ok := getString(pm, "leavese"); ok {
		button.LeaveSE = v
	}
	if v, ok := getString(pm, "activeimg"); ok {
		button.ActiveImg = v
	}
	if v, ok := getString(pm, "clickimg"); ok {
		button.ClickImg = v
	}
	if v, ok := getString(pm, "enterimg"); ok {
		img, err := loadImage(r, pm["folder"], v)
		if err != nil {
			return err
		}
		button.EnterImg = img
	}
	if v, ok := getString(pm, "autoimg"); ok {
		button.AutoImg = v
	}
	if v, ok := getString(pm, "skipimg"); ok {
		button.SkipImg = v
	}
	if v, ok, err := getBool(pm, "visible"); err != nil {
		return err
	} else if ok {
		button.Visible = v
	}
	if v, ok, err := getBool(pm, "auto_next"); err != nil {
		return err
	} else if ok {
		button.AutoNext = v
	}
	if v, ok, err := getBool(pm, "savesnap"); err != nil {
		return err
	} else if ok {
		button.SaveSnap = v
	}
	if v, ok, err := getInt(pm, "keyforcus"); err != nil {
		return err
	} else if ok {
		button.KeyForcus = v
	}
	if v, ok := getString(pm, "exp"); ok {
		button.Exp = v
	}
	if v, ok := getString(pm, "preexp"); ok {
		button.PreExp = v
	}
	// Falling back to the graphic's own size only works when there *is* a
	// graphic — a [button] carrying neither graphic= nor width=/height= (an
	// invisible hit zone, the same thing [clickable] registers) would
	// otherwise nil-deref here. Such a button stays 0x0 and is simply never
	// hit (isColision against an empty rect can't match), which is the
	// honest outcome: nothing to draw, nothing to click.
	if button.Width == 0 && button.Height == 0 && button.Graphic != nil {
		button.Width, button.Height = button.Graphic.Bounds().Dx(), button.Graphic.Bounds().Dy()
	}
	if traceTags {
		fmt.Printf("button: %+v\n", button)
	}
	buttons = append(buttons, button)
	return nil
}

func handleGLink(ctx *tagCtx) error {
	object := ctx.tag
	glink := &kag3.GLink{}
	for key, value := range object.Pm {
		switch key {
		case "color":
			r, g, b, err := parseColor(value)
			if err != nil {
				return err
			}
			glink.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
		case "font_color":
			glink.FontColor = value
		case "storage":
			glink.Storage = value
		case "target":
			glink.Target = value
		case "name":
			glink.Name = value
		case "text":
			glink.Text = value
		case "x":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			glink.X = v
		case "y":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			glink.Y = v
		case "width":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			glink.Width = v
		case "height":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			glink.Height = v
		case "size":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			glink.Size = v
		case "face":
			glink.Face = value
		case "graphic":
			glink.Graphic = value
		case "enterimg":
			glink.EnterImg = value
		case "clickse":
			glink.ClickSE = value
		case "enterse":
			glink.EnterSE = value
		case "leavese":
			glink.LeaveSE = value
		}
	}
	if glink.Height == 0 {
		_, h := text.Measure(glink.Text, ctx.r.fontFace, 0)
		glink.Height = int(h) + 20
	}
	if glink.Color == nil {
		glink.Color = glinkDefaultColor
	}
	if glink.Size == 0 {
		glink.Size = glinkDefaultSize
	}
	if glink.Face == "" {
		glink.Face = glinkDefaultFace
	}
	glinks = append(glinks, glink)
	return nil
}

// handleClickable implements [clickable]: an invisible clickable rectangle,
// for regions that need a hit zone without a visible button graphic.
// Reuses the existing Button/buttons plumbing (Update()'s click handling,
// drawScene's hover/enterimg logic) rather than a parallel mechanism —
// drawScene skips drawing a button with no Graphic/EnterImg, so this is
// simply a Button that never supplies either.
func handleClickable(ctx *tagCtx) error {
	object := ctx.tag
	b := &kag3.Button{Visible: true}
	for key, value := range object.Pm {
		var err error
		switch key {
		case "x":
			b.X, err = strconv.Atoi(value)
		case "y":
			b.Y, err = strconv.Atoi(value)
		case "width":
			b.Width, err = strconv.Atoi(value)
		case "height":
			b.Height, err = strconv.Atoi(value)
		case "storage":
			b.Storage = value
		case "target":
			b.Target = value
		case "name":
			b.Name = value
		}
		if err != nil {
			return err
		}
	}
	buttons = append(buttons, b)
	return nil
}

func handleLink(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	link := &kag3.Link{}
	for key, value := range object.Pm {
		switch key {
		case "storage":
			link.Storage = value
		case "target":
			link.Target = value
		case "keyforcus":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			link.KeyForcus = v
		}
	}
	// Bounded scan, not a bare `for {}` walking *ctx.i forward until it
	// happens to find an [endlink]: a [link] with no matching [endlink] —
	// one unclosed tag in one .ks file — would run straight off the end of
	// r.scripts with an index-out-of-range panic. Stopping at the end of the
	// script keeps whatever text was collected and lets the scenario carry
	// on, matching how every other scan-forward handler here tolerates a
	// missing terminator (see handleIgnore, tags_flow.go).
	for *ctx.i < len(r.scripts) {
		if v, ok := r.scripts[*ctx.i].(kag3.TagObject); ok && v.Name == "endlink" {
			break
		}
		if v, ok := r.scripts[*ctx.i].(kag3.TextObject); ok {
			link.Texts = append(link.Texts, v)
		}
		(*ctx.i)++
	}
	// Ran out of script without finding [endlink]: leave *ctx.i on the last
	// item so the enclosing loop's own increment ends the script cleanly,
	// rather than leaving it one past the end for later code to read.
	if *ctx.i >= len(r.scripts) {
		*ctx.i = len(r.scripts) - 1
	}
	links = append(links, link)
	if traceTags {
		fmt.Printf("links:%+v\n", links)
	}
	return nil
}
