package ebitengine

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// buttonRoles maps a [button role="..."] value to what clicking it does,
// looked up by hitButtons (input_hit.go) — the registered-map equivalent of
// dispatch.go's tag handlers. role="sleepgame" is deliberately absent: its
// only effect (the return-address push onto r.sleepStack) already happens
// earlier in hitButtons, before Storage/Role/Target dispatch, so an absent
// map entry is the correct no-op.
var buttonRoles = map[string]func(r *Renderer){
	// Per tyrano.jp/tag's [button] reference, role="save" opens the
	// save-slot screen rather than acting on a fixed slot directly —
	// that's what quicksave is for. Reuses the slot picker
	// (tags_uiscreens.go).
	"save": func(r *Renderer) { openSlotPicker(slotPickerSave) },
	"load": func(r *Renderer) { openSlotPicker(slotPickerLoad) },
	"quicksave": func(r *Renderer) {
		if err := r.saveSlot(quickSaveSlot); err != nil {
			fmt.Printf("quicksave failed: %v\n", err)
		}
	},
	"quickload": func(r *Renderer) {
		if err := r.loadSlot(quickSaveSlot); err != nil {
			fmt.Printf("quickload failed: %v\n", err)
		}
	},
	"backlog": func(r *Renderer) {
		backlogViewing = !backlogViewing
		if backlogViewing {
			backlogOpenedFrame = t
		}
	},
	"menu": func(r *Renderer) {
		menuOpen = !menuOpen
		if menuOpen {
			menuOpenedFrame = t
		}
	},
	"fullscreen": func(r *Renderer) {
		fmt.Printf("button.Role:%s\n", "fullscreen")
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	},
	"title": func(r *Renderer) {
		confirmGoToTitle(r)
	},
	"skip": func(r *Renderer) {
		isSkip = !isSkip
		if isSkip {
			isAuto = false
		}
	},
	"auto": func(r *Renderer) {
		isAuto = !isAuto
		if isAuto {
			isSkip = false
			autoStartT = t
		}
	},
	"window": func(r *Renderer) {
		textPosition.Visible = !textPosition.Visible
	},
}
