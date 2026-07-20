package ebitengine

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func init() {
	register("bg", handleBG)
}

func handleBG(ctx *tagCtx) error {
	r := ctx.r
	object := ctx.tag
	fmt.Printf("bg:%+v\n", bg)
	bgTick = t
	bg.IsEnd = false
	images := "images"
	for key, value := range object.Pm {
		var err error
		switch key {
		case "time":
			bg.Time, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "wait":
			switch value {
			case "true":
				bg.IsWait = true
			case "false":
				bg.IsWait = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "cross":
			switch value {
			case "true":
				bg.IsCross = true
			case "false":
				bg.IsCross = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "position":
			switch value {
			case "left", "center", "right", "top", "bottom":
				bg.Position = value
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "method":
			if slices.Contains(kag3.BackgroundMethod, value) {
				bg.Method = value
			} else {
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "system":
			switch value {
			case "true":
				bg.IsSystem = true
				images = "system/images"
			case "false":
				bg.IsSystem = false
				images = "images"
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		}
	}
	var err error
	bg.NextImage, _, err = ebitenutil.NewImageFromFileSystem(
		r.fses[images],
		object.Pm["storage"],
	)
	if err != nil {
		return err
	}
	if bg.IsWait {
		ctx.y.Until(true, func() bool {
			return bg.IsEnd
		})
	}
	return nil
}
