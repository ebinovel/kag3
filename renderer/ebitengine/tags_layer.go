package ebitengine

import (
	"fmt"
	"strconv"
)

func init() {
	register("layopt", handleLayopt)
	register("position", handlePosition)
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
