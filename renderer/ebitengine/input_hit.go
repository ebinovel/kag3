package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// pointerState returns the effective pointer position and press state,
// unifying mouse (desktop/wasm) and touch (mobile). ebiten.CursorPosition()
// always reports (0,0) on mobile native apps (see its own doc comment) —
// hit-testing there needs the touch API instead. pressed is true for as
// long as the button/touch is held (e.g. backlog's drag-to-scroll);
// justPressed is true only on the frame it started (e.g. tap-to-advance,
// same as a mouse left-click just-pressed). touch reports which branch fired
// — callers thread it into isColisionTouch (renderer.go) so tap targets get
// a little extra forgiveness on a touchscreen without changing anything a
// mouse user sees.
func pointerState() (x, y int, justPressed, pressed, touch bool) {
	if ids := ebiten.AppendTouchIDs(nil); len(ids) > 0 {
		x, y := ebiten.TouchPosition(ids[0])
		just := false
		for _, jid := range inpututil.AppendJustPressedTouchIDs(nil) {
			if jid == ids[0] {
				just = true
				break
			}
		}
		return x, y, just, true, true
	}
	x, y = ebiten.CursorPosition()
	return x, y, inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), false
}

// hitLinks hit-tests every [link]'s text choices against the cursor,
// setting hoveringClickable on hover and, on click, loading link.Storage
// (if set) and jumping to link.Target. Returns as soon as one choice
// consumes a justPressed click — necessary now that isColisionTouch can pad
// adjacent hit-boxes into overlapping each other on a touchscreen (see
// hitButtons' own note on the same fix); without this, a single tap could
// dispatch two choices in the same frame.
func hitLinks(r *Renderer) {
	for i, link := range links {
		mX, mY, justPressed, _, touch := pointerState()
		for j, t := range link.Texts {
			x, y := textPosition.Left, textPosition.Top
			w, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + textPosition.MarginLeft
			marginTop := y + textPosition.MarginTop + int(h)*(i+j)
			if isColisionTouch(mX, mY, marginLeft, marginTop, int(w), int(h), touch) {
				hoveringClickable = true
				//fmt.Println("isCollsion", mX, mY)
				if justPressed {
					if link.Storage != "" {
						screenChanged = true
						r.loadScript(link.Storage)
					}
					// lookupLabel (renderer.go), not a bare Target[1:] —
					// a [link storage="x.ks"] with no target= at all is
					// ordinary script, and slicing "" panicked the game on
					// click. A missing/unknown target now simply means "this
					// link only changes storage", which is exactly what such
					// a link is asking for.
					if v, ok := r.lookupLabel(link.Target); ok {
						if traceTags {
							fmt.Printf("click label:%+v\n", v)
						}
						jumpIndex = v.Index
						isJump = true
					}
					return
				}
			}
		}
	}
}

// hitGLinks hit-tests every [glink]'s rect against the cursor, setting
// hoveringClickable on hover and, on click, loading glink.Storage (if set)
// and jumping to glink.Target. Returns as soon as one glink consumes a
// justPressed click — see hitButtons' note on why this matters once
// isColisionTouch is padding hit-boxes on a touchscreen.
func hitGLinks(r *Renderer) {
	for _, glink := range glinks {
		mX, mY, justPressed, _, touch := pointerState()
		if isColisionTouch(mX, mY, glink.X, glink.Y, glink.Width, glink.Height, touch) {
			hoveringClickable = true
			if justPressed {
				if glink.Storage != "" {
					screenChanged = true
					r.loadScript(glink.Storage)
				}
				// One lookupLabel call (renderer.go) replaces what used to be
				// two lookups — raw, then Target[1:] — whose "second hit
				// wins" ordering was arbitrary, and whose second half
				// panicked on a [glink] with no target= of its own.
				if v, ok := r.lookupLabel(glink.Target); ok {
					if traceTags {
						fmt.Printf("label:%+v\n", v)
					}
					jumpIndex = v.Index
					isJump = true
				}
				return
			}
		}
	}
}

// buttonHitTestable reports whether a button can be clicked at all. A
// zero-area button can't: isColision's bounds are inclusive on both edges,
// so a 0x0 rect still "contains" its own top-left corner — and with
// isColisionTouch's padding that corner becomes a 32x32 tap zone. Left
// unguarded, any sizeless button (a [clickable] with no width=/height=, or a
// [button] with neither a graphic to measure nor an explicit size — see
// handleButton, tags_link.go) would sit at the screen's top-left corner
// silently swallowing clicks meant for whatever is actually drawn there.
//
// Split out as its own function rather than inlined into hitButtons' loop
// so it's testable without faking ebiten's real input state — the same
// reason skipShouldAdvance (renderer.go) and movieShouldFinish
// (tags_movie.go) are separate functions.
func buttonHitTestable(b *kag3.Button) bool {
	return b.Width > 0 && b.Height > 0
}

// hitButtons hit-tests every [button]/[clickable] against the cursor,
// setting hoveringClickable on hover and, on click, evaluating exp=/preexp=,
// loading button.Storage (with role="sleepgame"'s extra return-address
// bookkeeping), dispatching button.Role via buttonRoles (role_dispatch.go),
// and finally jumping to button.Target. Returns as soon as one button
// consumes a justPressed click, rather than letting every button in the
// slice react to the same tap — this used to be harmless when hit-boxes
// never overlapped, but isColisionTouch's touch padding (renderer.go) can
// now make two buttons meant to sit flush against each other (e.g.
// config.ks's 2-choice toggle rows, each half exactly touching the other
// with zero gap) overlap by touchHitPadding*2 px in the middle. Without
// this return, a tap landing in that sliver dispatched *both* buttons'
// exp= in the same frame — whichever button came later in this slice won
// the tf.set_* assignment, silently overriding the one actually tapped
// (regression: Android's スキップ対象 toggle showing the wrong highlight).
func hitButtons(r *Renderer) {
	for _, button := range buttons {
		if !buttonHitTestable(button) {
			continue
		}
		mX, mY, justPressed, _, touch := pointerState()
		if isColisionTouch(mX, mY, button.X, button.Y, button.Width, button.Height, touch) {
			hoveringClickable = true
			if justPressed {
				if traceTags {
					fmt.Printf("click button:%+v\n", button)
					fmt.Printf("labels:%+v\n", r.labels)
				}
				// Must run before Storage/Role/Target below: config.ks's
				// volume buttons set tf.current_bgm_vol etc. via exp=,
				// which *vol_bgm_change (jumped to next) reads.
				r.vm.EvalButtonExp(button.PreExp, button.Exp)
				if button.Storage != "" {
					if traceTags {
						fmt.Printf("button.Storage:%+v\n", button.Storage)
					}
					// role="sleepgame" must record its return address
					// (see [awakegame]/handleAwakeGame in tags_system.go)
					// against the *old* storage/position, before loadScript
					// below overwrites r.currentStorage. sleepStack, not
					// callStack — see the Renderer.sleepStack doc comment.
					if button.Role == "sleepgame" {
						r.sleepStack = append(r.sleepStack, sleepFrame{
							Storage:        r.currentStorage,
							Index:          currentScriptIndex,
							Buttons:        append([]*kag3.Button(nil), buttons...),
							Bg:             *bg,
							TextPosition:   *textPosition,
							Ptexts:         ptexts,
							CharaNamePText: charaNamePText,
							ViewCharas:     viewCharas,
						})
						// The sleepgame target (e.g. config.ks) is a
						// full-screen layout with no ptext areas or standing
						// characters of its own inherited from wherever it
						// was opened — see sleepFrame's doc comment
						// (renderer.go). [awakegame] restores the snapshot
						// above.
						ptexts = make(map[string]*kag3.PText)
						viewCharas = nil
					}
					screenChanged = true
					r.loadScript(button.Storage)
					if button.Target == "" {
						jumpIndex = 0
						isJump = true
					}
				}
				if fn, ok := buttonRoles[button.Role]; ok {
					fn(r)
				}
				if button.Target != "" {
					r.buttonTargetJump(button.Target)
				}
				return
			}
		}
	}
}
