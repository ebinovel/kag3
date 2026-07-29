//go:build windows

package helpers

import (
	"fmt"

	"github.com/ebinovel/kag3/e2e/driver"
)

// logicalWidth/logicalHeight are kag3's fixed design resolution — see
// config.go's LoadDefault() (ScreenWidth=1280, ScreenHeight=720) and
// example/resources/config.toml, which doesn't override them. Every
// button x=/y=/width=/height= in the bundled .ks scenarios is expressed in
// this coordinate space, regardless of the actual window size or DPI.
const (
	logicalWidth  = 1280
	logicalHeight = 720
)

// ClickLogical converts a kag3 logical coordinate (as written in a [button
// x= y=] tag) into an absolute screen coordinate (driver.Session.Click's
// coordinate space — see its doc comment) and clicks it. The size part of
// the conversion is a straight ratio against the current client-area size
// (driver.Session.WindowRect), so it's correct even if the window isn't
// exactly 1280x720 — no assumption is made that it is. The position part
// adds WindowRect's own origin (r.Min): omitting that would target
// whatever's at the *screen's* (lx,ly), not the window's — on a
// multi-monitor desktop where the window isn't flush against (0,0) (the
// common case), that silently clicks something else entirely instead of
// erroring.
func ClickLogical(sess *driver.Session, lx, ly int) error {
	r, err := sess.WindowRect()
	if err != nil {
		return fmt.Errorf("ClickLogical(%d,%d): %w", lx, ly, err)
	}
	x := r.Min.X + lx*r.Dx()/logicalWidth
	y := r.Min.Y + ly*r.Dy()/logicalHeight
	if err := sess.Click(x, y); err != nil {
		return fmt.Errorf("ClickLogical(%d,%d) -> window(%d,%d): %w", lx, ly, x, y, err)
	}
	return nil
}

// Title screen button centers, computed from example/resources/senarios/
// title.ks's [button x= y=] top-left coordinates plus half the actual
// graphic size (all four title/button_*.png are 360x74 — confirmed via
// `file example/resources/images/title/button_*.png`). Re-derive if
// title.ks or its button graphics change. title.ks has no CG/replay
// buttons (the bundled TyranoScript sample's cg.ks/replay.ks were dropped
// when this became an original game) — don't add ClickTitleCG/Replay back
// without a matching [button] in title.ks.
const (
	titleStartX, titleStartY   = 135 + 360/2, 230 + 74/2
	titleLoadX, titleLoadY     = 135 + 360/2, 340 + 74/2
	titleDemoX, titleDemoY     = 135 + 360/2, 450 + 74/2
	titleConfigX, titleConfigY = 135 + 360/2, 560 + 74/2
)

// ClickTitleStart clicks title.ks's "START" button (target="gamestart"
// -> scene1.ks).
func ClickTitleStart(sess *driver.Session) error { return ClickLogical(sess, titleStartX, titleStartY) }

// ClickTitleLoad clicks title.ks's LOAD button (role="load", opens the
// slot picker).
func ClickTitleLoad(sess *driver.Session) error { return ClickLogical(sess, titleLoadX, titleLoadY) }

// ClickTitleDemo clicks title.ks's DEMO button (target="demostart" ->
// hub.ks, the tag-feature demo hub).
func ClickTitleDemo(sess *driver.Session) error { return ClickLogical(sess, titleDemoX, titleDemoY) }

// ClickTitleConfig clicks title.ks's CONFIG button (role="sleepgame",
// storage="config.ks").
func ClickTitleConfig(sess *driver.Session) error {
	return ClickLogical(sess, titleConfigX, titleConfigY)
}

// Advance sends Enter, kag3's [l]/[p] click-to-continue key (see doNext in
// renderer.go — IsKeyJustPressed(KeyEnter) advances waiting text same as a
// left click).
func Advance(sess *driver.Session) error {
	return sess.KeyPress("Enter")
}

// The quick menu (opened via the hamburger button, bottom-right corner) is
// how "たそがれ図書室" exposes save/load/title-return in-game — unlike the
// bundled TyranoScript sample's scene1.ks, the current scene1.ks/hub.ks/
// demo_*.ks register no role="save"/"quicksave"/"title" [button] of their
// own, so there's nothing resembling a role_button row to click directly
// anymore; everything routes through this menu instead. Coordinates are
// exported (not just wrapped in a Click* func below) so callers that need
// clickAndWaitChanged-style retry logic (flows_test.go) can drive
// ClickLogical themselves. See renderer/ebitengine/tags_sysdesign.go's
// menuButtonRect (the 64x64 button_menu.png icon at
// (ScreenWidth-64-20, ScreenHeight-64-20)) and tags_save.go's
// quickMenuButtons() (five 520x70 rows at X=380, Y=190+i*(70+25) for
// i=0..4: SAVE/LOAD/HIDE MESSAGE/SKIP/BACK TO TITLE).
const (
	MenuButtonX, MenuButtonY         = 1196 + 64/2, 636 + 64/2
	QuickMenuSaveX, QuickMenuSaveY   = 380 + 520/2, 190 + 70/2
	QuickMenuLoadX, QuickMenuLoadY   = 380 + 520/2, 190 + 1*(70+25) + 70/2
	QuickMenuTitleX, QuickMenuTitleY = 380 + 520/2, 190 + 4*(70+25) + 70/2
)

// ClickMenuButton opens the quick menu — a no-op if it's not currently
// visible (menuButtonVisible false, or the menu is already open — see
// handleMenuButtonClick, tags_sysdesign.go).
func ClickMenuButton(sess *driver.Session) error { return ClickLogical(sess, MenuButtonX, MenuButtonY) }

// ClickQuickMenuSave clicks the open quick menu's SAVE row, which opens
// the slot picker in save mode (openSlotPicker(slotPickerSave) —
// tags_sysdesign.go). Follow up with ClickSlotPickerRow1 (or another
// row) to actually write a slot.
func ClickQuickMenuSave(sess *driver.Session) error {
	return ClickLogical(sess, QuickMenuSaveX, QuickMenuSaveY)
}

// ClickQuickMenuLoad clicks the open quick menu's LOAD row, opening the
// slot picker in load mode.
func ClickQuickMenuLoad(sess *driver.Session) error {
	return ClickLogical(sess, QuickMenuLoadX, QuickMenuLoadY)
}

// ClickQuickMenuTitle clicks the open quick menu's "BACK TO TITLE" row —
// opens confirmGoToTitle's confirmation dialog (renderer.go), same as
// ClickDialogOK below is meant to follow up on; it does not jump
// immediately.
func ClickQuickMenuTitle(sess *driver.Session) error {
	return ClickLogical(sess, QuickMenuTitleX, QuickMenuTitleY)
}

// Slot picker layout — see tags_uiscreens.go's slotPickerRowX/Y0/W/H/Gap
// (X=130 Y0=170 W=1000 H=120 gap=10) and backButtonRect (100x100
// menu_button_close.png at (ScreenWidth-100-20, 35)). Row i (0-indexed) is
// slot i+1 (slotPickerRows) — row 1 is slot ManualSaveSlot (save.go).
const (
	SlotPickerRow1X, SlotPickerRow1Y = 130 + 1000/2, 170 + 120/2
	SlotPickerBackX, SlotPickerBackY = 1160 + 100/2, 35 + 100/2
)

// ClickSlotPickerRow1 clicks the slot picker's first row — save slot
// ManualSaveSlot — writing or reading it depending on whether the picker
// is currently in save or load mode (handleSlotPickerClick,
// tags_uiscreens.go).
func ClickSlotPickerRow1(sess *driver.Session) error {
	return ClickLogical(sess, SlotPickerRow1X, SlotPickerRow1Y)
}

// ClickSlotPickerBack closes the slot picker without saving/loading.
func ClickSlotPickerBack(sess *driver.Session) error {
	return ClickLogical(sess, SlotPickerBackX, SlotPickerBackY)
}

// Confirm dialog OK/NG button centers, computed from
// tags_save.go's dialogButtonRects(screenW, screenH) at the fixed logical
// 1280x720: w,h=160,50; y=screenH/2+40=400; OK.X=screenW/2-w-20=460;
// NG.X=screenW/2+20=660. Used by confirmGoToTitle's "タイトルに戻ります。
// よろしいですか？" dialog (opened via ClickQuickMenuTitle above) and any
// other activeDialog with OnConfirm set.
const (
	dialogOKX, dialogOKY = 460 + 160/2, 400 + 50/2
	dialogNGX, dialogNGY = 660 + 160/2, 400 + 50/2
)

// ClickDialogOK clicks the OK button of whatever confirm dialog is
// currently open (handleDialogClick, renderer.go).
func ClickDialogOK(sess *driver.Session) error { return ClickLogical(sess, dialogOKX, dialogOKY) }

// ClickDialogNG clicks the NG/cancel button of whatever confirm dialog is
// currently open.
func ClickDialogNG(sess *driver.Session) error { return ClickLogical(sess, dialogNGX, dialogNGY) }
