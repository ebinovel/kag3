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
	register("link", handleLink)
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
	glinks = append(glinks, glink)
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
