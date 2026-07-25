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
	isTextEndedOrJumped    func() bool
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
	// isTextEndedOrJumped is handleP's actual wait condition ([p], tags_text.go)
	// — isTextEnded alone, plus a pending isJump. A pending jump (goToTitle,
	// applySaveData, or any button/link/glink target jump) means the *current*
	// screen — the one this [p] belongs to — is being abandoned outright, so
	// its own text-wait must not block that: the coroutine is nested inside
	// this Until call, several frames deep below initScript's outer isJump
	// check (macro.go), so without this, isJump sitting there true does
	// nothing at all until isTextEnded *also* becomes true on its own — which
	// requires a further real click that has nowhere correct to land, since
	// whatever screen it was aimed at hasn't been reached yet. isJump is
	// consumed (reset false) by the outer loop before the destination's own
	// first tag ever runs, so this never lets a freshly-loaded/jumped-to [p]
	// skip its own wait — only ever the *source* screen's.
	isTextEndedOrJumped = func() bool {
		return isTextEnded() || isJump
	}
	isJumped = func() bool {
		return isJump
	}
}
