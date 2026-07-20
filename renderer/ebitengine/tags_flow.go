package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
)

func init() {
	register("jump", handleJump)
}

func handleJump(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	jump := &kag3.Jump{}
	for key, value := range object.Pm {
		switch key {
		case "storage":
			jump.Storage = value
		case "target":
			jump.Target = value
		}
	}
	fmt.Printf("jump:%+v\n", jump)
	if jump.Storage != "" {
		r.manager.LoadScript(jump.Storage)
		r.labels = r.manager.Labels
		r.scripts = r.manager.Senario
		if jump.Target == "" {
			*ctx.i = 0
		}
	} else {
		if v, ok := r.labels[jump.Target]; ok {
			fmt.Printf("label:%+v\n", v)
			jumpIndex = v.Index
			*ctx.i = v.Index
		}
		if v, ok := r.labels[jump.Target[1:]]; ok {
			fmt.Printf("label:%+v\n", v)
			jumpIndex = v.Index
			*ctx.i = v.Index
		}
	}
	return nil
}
