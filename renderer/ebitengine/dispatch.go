package ebitengine

import (
	"log"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

// tagCtx carries everything a tag handler needs: the renderer, the coroutine
// yield handle (for blocking tags like [l]/[p]/[s]/[bg wait=true]), the tag
// itself, and a pointer to the running script index (mutated by tags such as
// [jump] and [link] that need to move execution elsewhere).
type tagCtx struct {
	r   *Renderer
	y   coro.Yield
	tag kag3.TagObject
	i   *int
}

type tagHandler func(ctx *tagCtx) error

var handlers = map[string]tagHandler{}

// currentScriptIndex mirrors *i whenever a top-level (depth 0) tag is
// dispatched, i.e. the position within r.scripts of whatever's currently
// running. Button-role clicks (role="sleepgame", [checkpoint], save/load —
// see tags_save.go) happen from Update(), outside the tag-execution
// coroutine, so they have no *i of their own; this is how they read "where
// are we right now" to build a resumable position.
var currentScriptIndex int

// register associates a tag name with its handler. Called from init() in
// each tags_*.go file.
func register(name string, h tagHandler) {
	handlers[name] = h
}

// dispatchTag looks up and runs the handler for a tag. If no built-in
// handler is registered, a user-defined [macro] of the same name is tried
// next. depth counts macro-expansion nesting (0 at the top level) and is
// threaded through so expandMacro can enforce a recursion limit. Tags that
// match neither are logged rather than silently dropped, so gaps are
// visible during staged tag rollout.
func dispatchTag(r *Renderer, y coro.Yield, tag kag3.TagObject, i *int, depth int) error {
	if depth == 0 {
		currentScriptIndex = *i
	}
	tag.Pm = r.vm.expandParams(tag.Pm)
	ctx := &tagCtx{r: r, y: y, tag: tag, i: i}
	if h, ok := handlers[tag.Name]; ok {
		return h(ctx)
	}
	if m, ok := r.manager.Macros[tag.Name]; ok {
		return r.expandMacro(y, m, tag.Pm, depth+1)
	}
	log.Printf("未実装のタグです: %s (line %d)", tag.Name, tag.Line)
	return nil
}
