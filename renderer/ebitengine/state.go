package ebitengine

import "github.com/eihigh/coro"

// isFirst/loop/co drive the single tag coroutine (see initScript in
// macro.go and Update() in renderer.go) — control-flow flags with no
// single tag or domain of their own, unlike the rest of the package-level
// state that lives in the tags_*.go file for the tags that own it.
var (
	isFirst                bool
	loop                   func(y coro.Yield)
	isClicked, isTextEnded func() bool
	t, tick, oldTick       int
	isJump                 bool
	isJumped               func() bool
	jumpIndex              int
	// screenChanged marks that whatever is about to consume isJump represents
	// a real screen change (a different storage loaded, or goToTitle/save-load
	// tearing down the previous screen's state) rather than a same-storage
	// label jump ([link]/[glink]/[button target=] to a label in the same
	// file). Set at the point loadScript/goToTitle/applySaveData actually
	// changes screens, read (and cleared) whenever the isJump handling in
	// Update() runs — which can be a later frame than where it was set, e.g.
	// a title confirm dialog resolves and returns early the same frame
	// (anyModalActive), so isJump isn't processed until the next Update().
	// Without this, clearNonFixButtons() had to run unconditionally on every
	// isJump, which wiped scene1.ks's role_button set (registered without
	// fix="true", matching the real bundled sample) on every in-scene
	// [link]/[glink] click even though nothing about the screen changed.
	screenChanged bool
	co            *coro.Coro
)

func init() {
	isClicked = func() bool {
		return oldTick+3 >= tick
	}
	isTextEnded = func() bool {
		return oldTick+3 >= tick && isWait
	}
	isJumped = func() bool {
		return isJump
	}
}
