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

// dispatchTag looks up and runs the handler for a tag. Unknown tags are
// logged rather than silently dropped, so gaps are visible during staged
// tag rollout.
func dispatchTag(r *Renderer, y coro.Yield, tag kag3.TagObject, i *int) error {
	tag.Pm = r.vm.expandParams(tag.Pm)
	ctx := &tagCtx{r: r, y: y, tag: tag, i: i}
	if h, ok := handlers[tag.Name]; ok {
		return h(ctx)
	}
	log.Printf("未実装のタグです: %s (line %d)", tag.Name, tag.Line)
	return nil
}
