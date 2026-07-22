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
	register("if", handleIf)
	register("elsif", handleElsif)
	register("else", handleElse)
	register("endif", handleEndif)
	register("ignore", handleIgnore)
	register("endignore", handleNoop)
	register("clearstack", handleClearStack)
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
		clearNonFixButtons()
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
		clearNonFixButtons()
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
		clearNonFixButtons()
	}
	// -1 to compensate for the enclosing loop's increment, same as [call].
	*ctx.i = frame.Index - 1
	return nil
}

// ifTaken tracks, per IfCount depth, whether a branch has already run in the
// current [if]...[endif] chain. Parser-assigned IfCount is shared by an
// [if] and its [elsif]/[else]/[endif] (see parser.go's makeTag), so it
// doubles as a chain identifier; nesting works because a nested [if] gets
// its own (deeper) IfCount, and [if] always resets its own depth's entry on
// entry, so depth values reused by sibling if-chains never see stale state.
var ifTaken = map[int]bool{}

// handleIf evaluates exp; if true it clears the way for the branch body to
// run by simple fall-through, if false it hops to the chain's next
// elsif/else/endif (same IfCount depth) for that tag to decide what happens
// next. Only correct against r.scripts, matching the existing limitation on
// jump/call/link inside macro bodies.
func handleIf(ctx *tagCtx) error {
	r := ctx.r
	depth := ctx.tag.IfCount
	ifTaken[depth] = false
	if r.vm.EvalBool(ctx.tag.Pm["exp"]) {
		ifTaken[depth] = true
		return nil
	}
	skipIfChain(r, ctx.i, depth)
	return nil
}

// handleElsif is reached either by falling through from a taken branch
// above it (skip to endif) or by handleIf/handleElsif hopping to it with no
// branch taken yet (evaluate its own exp).
func handleElsif(ctx *tagCtx) error {
	r := ctx.r
	depth := ctx.tag.IfCount
	if ifTaken[depth] {
		skipIfChain(r, ctx.i, depth)
		return nil
	}
	if r.vm.EvalBool(ctx.tag.Pm["exp"]) {
		ifTaken[depth] = true
		return nil
	}
	skipIfChain(r, ctx.i, depth)
	return nil
}

// handleElse has no condition: skip to endif if a branch already ran,
// otherwise fall through into its body.
func handleElse(ctx *tagCtx) error {
	depth := ctx.tag.IfCount
	if ifTaken[depth] {
		skipIfChain(ctx.r, ctx.i, depth)
	}
	return nil
}

func handleEndif(ctx *tagCtx) error {
	delete(ifTaken, ctx.tag.IfCount)
	return nil
}

// skipIfChain moves *i to just before the next elsif/else/endif at depth,
// so the enclosing loop's increment lands exactly on it for dispatch. A
// chain with several elsifs is resolved by hopping through this function
// once per intermediate tag rather than in one jump — simpler, and each hop
// is cheap relative to the coroutine's per-frame step budget.
func skipIfChain(r *Renderer, i *int, depth int) {
	for idx := *i + 1; idx < len(r.scripts); idx++ {
		tag, ok := r.scripts[idx].(kag3.TagObject)
		if !ok || tag.IfCount != depth {
			continue
		}
		switch tag.Name {
		case "elsif", "else", "endif":
			*i = idx - 1
			return
		}
	}
	*i = len(r.scripts) - 1
}

// handleIgnore skips to the matching [endignore], supporting nesting.
func handleIgnore(ctx *tagCtx) error {
	r := ctx.r
	depth := 1
	idx := *ctx.i
	for idx+1 < len(r.scripts) {
		idx++
		if tag, ok := r.scripts[idx].(kag3.TagObject); ok {
			switch tag.Name {
			case "ignore":
				depth++
			case "endignore":
				depth--
				if depth == 0 {
					*ctx.i = idx - 1
					return nil
				}
			}
		}
	}
	// No matching [endignore]: skip to the end of the script.
	*ctx.i = idx - 1
	return nil
}

func handleClearStack(ctx *tagCtx) error {
	ctx.r.callStack = nil
	return nil
}

// handleNoop does nothing; used for tags that only matter as scan targets
// for other handlers (e.g. [endignore] is where [ignore] lands).
func handleNoop(ctx *tagCtx) error {
	return nil
}
