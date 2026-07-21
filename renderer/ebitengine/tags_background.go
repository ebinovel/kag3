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
	register("bg2", handleBG2)
}

func handleBG(ctx *tagCtx) error {
	return applyBGTag(ctx, bg, &bgTick)
}

// handleBG2 drives a second, independent background layer (bg2/bg2Tick),
// composited on top of the main one in Draw. Same attributes and
// transition machinery as [bg], just a separate slot — useful for things
// like weather overlays layered over the main scene.
func handleBG2(ctx *tagCtx) error {
	return applyBGTag(ctx, bg2, &bg2Tick)
}

// applyBGTag implements [bg]/[bg2]: both take identical attributes and
// only differ in which *kag3.Background/tick they drive.
func applyBGTag(ctx *tagCtx, target *kag3.Background, tick *int) error {
	r := ctx.r
	object := ctx.tag
	fmt.Printf("bg:%+v\n", target)
	*tick = t
	target.IsEnd = false
	images := "images"
	for key, value := range object.Pm {
		var err error
		switch key {
		case "time":
			target.Time, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "wait":
			switch value {
			case "true":
				target.IsWait = true
			case "false":
				target.IsWait = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "cross":
			switch value {
			case "true":
				target.IsCross = true
			case "false":
				target.IsCross = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "position":
			switch value {
			case "left", "center", "right", "top", "bottom":
				target.Position = value
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "method":
			if slices.Contains(kag3.BackgroundMethod, value) {
				target.Method = value
			} else {
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "system":
			switch value {
			case "true":
				target.IsSystem = true
				images = "system/images"
			case "false":
				target.IsSystem = false
				images = "images"
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		}
	}
	var err error
	target.NextImage, _, err = ebitenutil.NewImageFromFileSystem(
		r.fses[images],
		object.Pm["storage"],
	)
	if err != nil {
		return err
	}
	if target.IsWait {
		ctx.y.Until(true, func() bool {
			return target.IsEnd
		})
	}
	return nil
}
