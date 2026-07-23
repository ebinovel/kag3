package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// hitLinks hit-tests every [link]'s text choices against the cursor,
// setting hoveringClickable on hover and, on click, loading link.Storage
// (if set) and jumping to link.Target.
func hitLinks(r *Renderer) {
	for i, link := range links {
		mX, mY := ebiten.CursorPosition()
		for j, t := range link.Texts {
			x, y := textPosition.Left, textPosition.Top
			w, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + textPosition.MarginLeft
			marginTop := y + textPosition.MarginTop + int(h)*(i+j)
			if isColision(mX, mY, marginLeft, marginTop, int(w), int(h)) {
				hoveringClickable = true
				//fmt.Println("isCollsion", mX, mY)
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					if link.Storage != "" {
						screenChanged = true
						r.loadScript(link.Storage)
					}
					if v, ok := r.labels[link.Target[1:]]; ok {
						fmt.Printf("click label:%+v\n", v)
						jumpIndex = v.Index
						isJump = true
					}
				}
			}
		}
	}
}

// hitGLinks hit-tests every [glink]'s rect against the cursor, setting
// hoveringClickable on hover and, on click, loading glink.Storage (if set)
// and jumping to glink.Target.
func hitGLinks(r *Renderer) {
	for _, glink := range glinks {
		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, glink.X, glink.Y, glink.Width, glink.Height) {
			hoveringClickable = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if glink.Storage != "" {
					screenChanged = true
					r.loadScript(glink.Storage)
				}
				if v, ok := r.labels[glink.Target]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
				if v, ok := r.labels[glink.Target[1:]]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
			}
		}
	}
}

// hitButtons hit-tests every [button]/[clickable] against the cursor,
// setting hoveringClickable on hover and, on click, evaluating exp=/preexp=,
// loading button.Storage (with role="sleepgame"'s extra return-address
// bookkeeping), dispatching button.Role, and finally jumping to
// button.Target.
func hitButtons(r *Renderer) {
	for _, button := range buttons {
		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, button.X, button.Y, button.Width, button.Height) {
			hoveringClickable = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				fmt.Printf("click button:%+v\n", button)
				fmt.Printf("labels:%+v\n", r.labels)
				// Must run before Storage/Role/Target below: config.ks's
				// volume buttons set tf.current_bgm_vol etc. via exp=,
				// which *vol_bgm_change (jumped to next) reads.
				r.vm.EvalButtonExp(button.PreExp, button.Exp)
				if button.Storage != "" {
					fmt.Printf("button.Storage:%+v\n", button.Storage)
					// role="sleepgame" must record its return address
					// (see [awakegame]/handleAwakeGame in tags_system.go)
					// against the *old* storage/position, before loadScript
					// below overwrites r.currentStorage. sleepStack, not
					// callStack — see the Renderer.sleepStack doc comment.
					if button.Role == "sleepgame" {
						r.sleepStack = append(r.sleepStack, sleepFrame{
							Storage:      r.currentStorage,
							Index:        currentScriptIndex,
							Buttons:      append([]*kag3.Button(nil), buttons...),
							Bg:           *bg,
							TextPosition: *textPosition,
						})
					}
					screenChanged = true
					r.loadScript(button.Storage)
					if button.Target == "" {
						jumpIndex = 0
						isJump = true
					}
				}
				if button.Role != "" {
					switch button.Role {
					case "save":
						// Per tyrano.jp/tag's [button] reference, role="save"
						// opens the save-slot screen rather than acting on a
						// fixed slot directly — that's what quicksave is
						// for. Reuses Phase 9's slot picker (tags_uiscreens.go).
						openSlotPicker(slotPickerSave)
					case "load":
						openSlotPicker(slotPickerLoad)
					case "quicksave":
						if err := r.saveSlot(quickSaveSlot); err != nil {
							fmt.Printf("quicksave failed: %v\n", err)
						}
					case "quickload":
						if err := r.loadSlot(quickSaveSlot); err != nil {
							fmt.Printf("quickload failed: %v\n", err)
						}
					case "backlog":
						backlogViewing = !backlogViewing
						if backlogViewing {
							backlogOpenedFrame = t
						}
					case "menu":
						menuOpen = !menuOpen
						if menuOpen {
							menuOpenedFrame = t
						}
					case "fullscreen":
						fmt.Printf("button.Role:%s\n", button.Role)
						ebiten.SetFullscreen(!ebiten.IsFullscreen())
					case "title":
						confirmGoToTitle(r)
					case "skip":
						isSkip = !isSkip
						if isSkip {
							isAuto = false
						}
					case "auto":
						isAuto = !isAuto
						if isAuto {
							isSkip = false
							autoStartT = t
						}
					case "window":
						textPosition.Visible = !textPosition.Visible
					case "sleepgame":
						// storage load + return-frame push already handled
						// above, before the switch.
					}
				}
				if button.Target != "" {
					r.buttonTargetJump(button.Target)
				}
			}
		}
	}
}
