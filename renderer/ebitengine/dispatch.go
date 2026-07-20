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
