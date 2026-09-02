package ebitengine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// --- quick menu (button role="menu") ---
//
// Deprecated: reachable only via role="menu" or the (also deprecated)
// [showmenubutton] corner icon (tags_sysdesign.go) — the redesigned message
// window's own operation row (tags_oprow.go) covers everything this popup
// offers (SAVE/LOAD/SKIP/BACK TO TITLE) except HIDE MESSAGE (case 2 below;
// just buttonRoles["window"]'s toggle, reachable via a script-placed
// [button role="window"]). Stays implemented for any script that opens it
// directly.
//
// Laid out to match real Tyrano's own system menu screen (built from the
// same bundled resources/system/images assets: bg_base.png, label_menu.png,
// menu_button_close.png for the "BACK" button top-right — same as the slot
// picker's — and the five menu_button_*/menu_message_close.png pill
// buttons), at a 1920x1080 canvas. scene1.ks already places individual
// save/load/skip/auto/backlog buttons directly on screen too, so this
// doesn't need a "閉じる" item of its own — the top-right BACK button
// covers that, consistently with the slot picker.
//
// These are the raw 1920x1080-tuned values — every call site scales them by
// systemChromeScale(r) (system_chrome.go) before use, so a game running at
// a different ScreenWidth (e.g. example_tyrano_official_backup's 1280x720)
// doesn't get this chrome drawn at an oversized literal pixel size.
const (
	quickMenuLabelX, quickMenuLabelY    = 15, 15
	quickMenuButtonX, quickMenuButtonY0 = 570, 285
	quickMenuButtonW, quickMenuButtonH  = 780, 105
	quickMenuButtonGap                  = 38
)

var (
	menuOpen        bool
	menuOpenedFrame int
)

// quickMenuButtonSpec is one pill button: its normal/hover image pair and
// hit-test rect. The order here is the on-screen top-to-bottom order and is
// what quickMenuButtons()'s index maps to in handleQuickMenuClick.
type quickMenuButtonSpec struct {
	Normal, Hover string
	X, Y, W, H    int
}

func quickMenuButtons(r *Renderer) []quickMenuButtonSpec {
	s := systemChromeScale(r)
	names := [...][2]string{
		{"menu_button_save.png", "menu_button_save2.png"},
		{"menu_button_load.png", "menu_button_load2.png"},
		{"menu_message_close.png", "menu_message_close2.png"},
		{"menu_button_skip.png", "menu_button_skip2.png"},
		{"menu_button_title.png", "menu_button_title2.png"},
	}
	items := make([]quickMenuButtonSpec, len(names))
	for i, n := range names {
		y0 := quickMenuButtonY0 + i*(quickMenuButtonH+quickMenuButtonGap)
		items[i] = quickMenuButtonSpec{
			Normal: n[0], Hover: n[1],
			X: int(float64(quickMenuButtonX) * s), Y: int(float64(y0) * s),
			W: int(float64(quickMenuButtonW) * s), H: int(float64(quickMenuButtonH) * s),
		}
	}
	return items
}

// quickMenuButtonImageName picks btn.Hover/Normal depending on whether
// (mX, mY) is currently over it — split out from drawQuickMenu so it's
// testable without a real ebiten.CursorPosition(), same idea as
// backButtonImageName in tags_uiscreens.go.
func quickMenuButtonImageName(btn quickMenuButtonSpec, mX, mY int, touch bool) string {
	if isColisionTouch(mX, mY, btn.X, btn.Y, btn.W, btn.H, touch) {
		return btn.Hover
	}
	return btn.Normal
}

func drawQuickMenu(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	s := systemChromeScale(r)
	if bgImg := loadSystemImage(r, "bg_base.png"); bgImg != nil {
		op := &ebiten.DrawImageOptions{}
		bw, bh := bgImg.Bounds().Dx(), bgImg.Bounds().Dy()
		op.GeoM.Scale(float64(w)/float64(bw), float64(h)/float64(bh))
		buf.DrawImage(bgImg, op)
	} else {
		fillRect(buf, 0, 0, float64(w), float64(h), color.RGBA{0, 0, 0, 160})
	}

	if labelImg := loadSystemImage(r, "label_menu.png"); labelImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(quickMenuLabelX*s, quickMenuLabelY*s)
		buf.DrawImage(labelImg, op)
	}

	back := backButtonRect(r)
	mX, mY, _, _, touch := pointerState()
	if backImg := loadSystemImage(r, backButtonImageName(back, mX, mY, touch)); backImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(back.X), float64(back.Y))
		buf.DrawImage(backImg, op)
	}

	for _, btn := range quickMenuButtons(r) {
		if img := loadSystemImage(r, quickMenuButtonImageName(btn, mX, mY, touch)); img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(btn.X), float64(btn.Y))
			buf.DrawImage(img, op)
		}
	}
}

func (r *Renderer) handleQuickMenuClick(screenW, screenH int) {
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	if t == menuOpenedFrame {
		return
	}

	back := backButtonRect(r)
	if isColisionTouch(mX, mY, back.X, back.Y, back.W, back.H, touch) {
		menuOpen = false
		return
	}

	for idx, btn := range quickMenuButtons(r) {
		if !isColisionTouch(mX, mY, btn.X, btn.Y, btn.W, btn.H, touch) {
			continue
		}
		switch idx {
		case 0: // SAVE — the slot picker (tags_uiscreens.go)
			openSlotPicker(slotPickerSave)
		case 1: // LOAD
			openSlotPicker(slotPickerLoad)
		case 2: // HIDE MESSAGE — same toggle as button role="window"
			menuOpen = false
			textPosition.Visible = !textPosition.Visible
		case 3: // MESSAGE SKIP — same toggle as button role="skip"
			menuOpen = false
			isSkip = !isSkip
			if isSkip {
				isAuto = false
			}
		case 4: // BACK TO TITLE
			menuOpen = false
			confirmGoToTitle(r)
		}
		return
	}
}
