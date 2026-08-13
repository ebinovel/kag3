package ebitengine

import (
	"fmt"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

// lookupLabel resolves a jump target against r.labels, accepting it written
// either with or without its leading "*". r.labels is keyed on
// kag3.LabelInfo's bare name (parser.go strips the "*" when it records a
// label, and reindexLabels keeps that keying), but scripts always write
// targets as "*name" — and some callers (kag3-runner's -label flag) pass
// either form.
//
// This is the single label-resolution path for every caller that turns a
// target= into a position: [jump] (handleJump, tags_flow.go), [link]/[glink]
// clicks (hitLinks/hitGLinks, input_hit.go), [button target=]
// (buttonTargetJump below) and StartAtLabel. Each of those used to inline
// its own variant, and two problems came with that:
//
//   - A bare target[1:] slice panics outright ("slice bounds out of range
//     [1:0]") when target is empty — reachable from perfectly ordinary
//     script, e.g. clicking a [link storage="scene2.ks"]...[endlink] that
//     names no target= at all, since the click handler ran the lookup
//     unconditionally before checking anything.
//   - Several sites looked the target up *twice* (raw, then target[1:]) and
//     let whichever hit came second win. Labels are only ever stored bare,
//     so TrimPrefix covers both spellings in one lookup with no such
//     ambiguity.
//
// An empty target (or a bare "*") reports not-found rather than resolving:
// "no target given" must not accidentally match a label whose own name
// parsed as empty.
func (r *Renderer) lookupLabel(target string) (kag3.LabelInfo, bool) {
	name := strings.TrimPrefix(target, "*")
	if name == "" {
		return kag3.LabelInfo{}, false
	}
	v, ok := r.labels[name]
	return v, ok
}

// StartAtLabel makes the script loop begin at the named label instead of
// index 0. It must be called between NewRenderer and the first Update():
// initScript's loop checks isJump unconditionally at the top of its very
// first iteration (see its own doc comment), so a pending jump set here
// beforehand is picked up before anything else executes.
//
// name may be given with or without its leading "*" — see lookupLabel.
//
// Returns false, doing nothing, if name isn't a known label — the caller
// is expected to report that rather than silently starting at the top of
// the script.
func (r *Renderer) StartAtLabel(name string) bool {
	v, ok := r.lookupLabel(name)
	if !ok {
		return false
	}
	jumpIndex = v.Index
	isJump = true
	return true
}

// loadScript loads a scenario file and re-points the renderer at it,
// keeping currentStorage in sync so [call]/[return] can record and resume
// call frames as plain (storage, index) pairs.
func (r *Renderer) loadScript(name string) error {
	if err := r.manager.LoadScript(name); err != nil {
		return err
	}
	r.labels = r.manager.Labels
	r.scripts = r.manager.Senario
	r.currentStorage = name
	return nil
}

func (r *Renderer) initScript() {
	loop = func(y coro.Yield) {
		// Deliberately not a "for i := 0; i < len(r.scripts); i++" loop:
		// that shape checks the bound *before* the body's isJump check on
		// every iteration, including the very first one after i++. A jump
		// that lands while a *different* script (e.g. [call storage=...]
		// or goToTitle swapping r.scripts out mid-loop) had left i sitting
		// at a high index — say config.ks's [s] at index 63 — would then
		// have that stale i compared against the *new* r.scripts' length
		// (title.ks, much shorter) before isJump ever got a chance to
		// overwrite it, silently ending the whole loop (r.Done = true)
		// instead of jumping. Checking isJump first, unconditionally,
		// before the bound check fixes that: whatever i was doesn't matter
		// once a jump is pending.
		i := 0
		for {
			if isJump {
				i = jumpIndex
				isJump = false
			}
			if i >= len(r.scripts) {
				break
			}
			// A tag handler error (bad attribute value, missing asset, ...)
			// must not take the whole game down — this used to panic here,
			// so a single malformed attribute anywhere in a script (e.g.
			// [delay speed="user"], strconv.Atoi failing on a non-numeric
			// value) crashed every player instantly, unlike an *unknown* tag
			// name, which dispatchTag already just logs and continues past
			// (see its own comment). Logging and moving on to the next tag
			// makes both cases behave the same way. expandMacro's own
			// execItem calls (macro.go) return their error the same way
			// execItem itself does, so a failing tag inside a [macro] body
			// is caught here too — the whole macro call is skipped, which is
			// far better than panicking, even though it means the macro's
			// remaining tags don't run either.
			if err := r.execItem(y, r.scripts, &i, 0); err != nil {
				fmt.Printf("タグの実行でエラーが発生したためスキップします (index=%d): %v\n", i, err)
			}
			i++
			y()
		}
		r.Done = true
	}
}

// buttonTargetJump resolves a clicked button's target= against r.labels and,
// if found, jumps there — call-style, not a plain [jump]: real Tyrano's
// official config.ks (this repo's example/game/resources/senarios/config.ks) ends
// every target label (*vol_bgm_change etc.) with [return], expecting to land
// back exactly where the button was clicked. Pushing currentScriptIndex, not
// +1, matches role="sleepgame"'s push in Update() — both happen outside the
// coroutine, so [return]'s "-1 to compensate for the enclosing loop's
// increment" lands back on whatever tag is currently blocking (typically
// [s]), re-entering it cleanly.
func (r *Renderer) buttonTargetJump(target string) {
	v, ok := r.lookupLabel(target)
	if !ok {
		return
	}
	if traceTags {
		fmt.Printf("label:%+v\n", v)
	}
	r.callStack = append(r.callStack, callFrame{
		Storage: r.currentStorage,
		Index:   currentScriptIndex,
	})
	jumpIndex = v.Index
	isJump = true
}

// clearLinksOnJump runs once per Update() frame, right before isJump is
// consumed by the coroutine (see initScript's loop): any pending link/glink
// choice list is always dropped (it belonged to whatever line just advanced
// past), but buttons are only swept by clearNonFixButtons when screenChanged
// says this jump is an actual screen change — a same-storage label jump
// ([link]/[glink]/[button target=] to a label in the current file) must
// leave persistent UI like scene1.ks's role_button set alone, matching real
// Tyrano: a jump within a scenario doesn't tear down the screen.
func clearLinksOnJump() {
	if !isJump {
		return
	}
	if preserveLinksOnJump {
		preserveLinksOnJump = false
	} else {
		glinks = nil
		links = nil
	}
	if screenChanged {
		clearNonFixButtons()
		screenChanged = false
	}
}

// clearNonFixButtons drops every button without Fix=true from the buttons
// list — the reaction to a screen-changing jump (see clearLinksOnJump).
// Fix=true buttons persist across jumps (that's what "fix" means, e.g.
// config.ks's entire button set, registered once at *config_page); only
// [clearfix] (tags_layer.go) removes those.
func clearNonFixButtons() {
	kept := buttons[:0]
	for _, b := range buttons {
		if b.Fix {
			kept = append(kept, b)
		}
	}
	buttons = kept
}
