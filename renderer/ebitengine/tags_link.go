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
	return ctx.r.button(ctx.tag)
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
	for {
		if v, ok := r.scripts[*ctx.i].(kag3.TagObject); ok {
			if v.Name == "endlink" {
				break
			}
		}
		if v, ok := r.scripts[*ctx.i].(kag3.TextObject); ok {
			link.Texts = append(link.Texts, v)
		}
		(*ctx.i)++
	}
	links = append(links, link)
	fmt.Printf("links:%+v\n", links)
	return nil
}
