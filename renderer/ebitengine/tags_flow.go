package ebitengine

import (
	"fmt"
	"strings"

	"github.com/ebinovel/kag3"
)

func init() {
	register("jump", handleJump)
	register("call", handleCall)
	register("return", handleReturn)
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
		if err := r.loadScript(jump.Storage); err != nil {
			return err
		}
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

// handleCall implements [call storage=... target=...]: pushes a return
// address (the current storage and the item right after this tag) onto
// r.callStack, then repositions execution like [jump] does — optionally
// switching storage first, then optionally seeking to a target label.
// Unlike [jump], [call] is meant to be resumed via [return].
func handleCall(ctx *tagCtx) error {
	r := ctx.r
	storage := ctx.tag.Pm["storage"]
	target := strings.TrimPrefix(ctx.tag.Pm["target"], "*")

	r.callStack = append(r.callStack, callFrame{
		Storage: r.currentStorage,
		Index:   *ctx.i + 1,
	})

	if storage != "" && storage != r.currentStorage {
		if err := r.loadScript(storage); err != nil {
			return err
		}
		// Land exactly on the new script's first item if no target
		// follows: the enclosing loop increments *ctx.i once more after
		// this handler returns.
		*ctx.i = -1
	}
	if target != "" {
		if v, ok := r.labels[target]; ok {
			*ctx.i = v.Index
		}
	}
	return nil
}

// handleReturn implements [return]: pops the most recent [call] frame and
// resumes right after that call, reloading its storage first if execution
// has since moved to a different file. A [return] with no matching [call]
// is a no-op, matching Tyrano's tolerant behavior.
func handleReturn(ctx *tagCtx) error {
	r := ctx.r
	if len(r.callStack) == 0 {
		return nil
	}
	n := len(r.callStack) - 1
	frame := r.callStack[n]
	r.callStack = r.callStack[:n]

	if frame.Storage != "" && frame.Storage != r.currentStorage {
		if err := r.loadScript(frame.Storage); err != nil {
			return err
		}
	}
	// -1 to compensate for the enclosing loop's increment, same as [call].
	*ctx.i = frame.Index - 1
	return nil
}
