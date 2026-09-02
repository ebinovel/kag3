//go:build windows

package helpers

import (
	"fmt"

	"github.com/ebinovel/kag3/e2e/driver"
)

// logicalWidth/logicalHeight are kag3's fixed design resolution — see
// example/game/resources/config.toml's ScreenWidth=1920/ScreenHeight=1080
// (overriding config.go's LoadDefault() 1280x720 default). Every
// button x=/y=/width=/height= in the bundled .ks scenarios is expressed in
// this coordinate space, regardless of the actual window size or DPI.
const (
	logicalWidth  = 1920
	logicalHeight = 1080
)

// ClickLogical converts a kag3 logical coordinate (as written in a [button
// x= y=] tag) into an absolute screen coordinate (driver.Session.Click's
// coordinate space — see its doc comment) and clicks it. The size part of
// the conversion is a straight ratio against the current client-area size
// (driver.Session.WindowRect), so it's correct even if the window isn't
// exactly 1920x1080 — no assumption is made that it is. The position part
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

// Title screen button centers, computed from example/game/resources/senarios/
// title.ks's [button x= y=] top-left coordinates (already in the
// 1920x1080 logical space — config.toml sets ScreenWidth=1920/
// ScreenHeight=1080, and title.ks's x=/y= are written directly against
// that) plus half each graphic's *actual* size. The title/button_*.png set
// is trimmed per-label, so each button has a different width — confirmed via
// `file example/game/resources/images/title/button_*.png`:
// button_start.png 144x74, button_load.png 110x74, button_demo.png 76x74,
// button_config.png 176x74 (height is 74 for all four). title.ks lays all
// four out in a single horizontal row (y=886, x=140/348/522/662). Re-derive
// if title.ks or these graphics change. title.ks has no CG/replay buttons
// (the bundled TyranoScript sample's cg.ks/replay.ks were dropped when
// this became an original game) — don't add ClickTitleCG/Replay back
// without a matching [button] in title.ks.
const (
	titleStartX, titleStartY   = 140 + 144/2, 886 + 74/2
	titleLoadX, titleLoadY     = 348 + 110/2, 886 + 74/2
	titleDemoX, titleDemoY     = 522 + 76/2, 886 + 74/2
	titleConfigX, titleConfigY = 662 + 176/2, 886 + 74/2
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

// Deprecated: the quick menu (opened via the hamburger button, bottom-right
// corner) exposes save/load/title-return, but scene1.ks/demo_save.ks don't
// call @showmenubutton — the redesigned message window's own operation row
// (OpRow* below) covers the same ground — so this corner button never
// appears in the bundled example and ClickMenuButton is a no-op there.
// [showmenubutton]/role="menu" are still implemented engine-side (see
// tags_sysdesign.go/tags_save.go), so these helpers stay for any script that
// calls them directly; flows_test.go doesn't. See
// renderer/ebitengine/tags_sysdesign.go's menuButtonRect (the 96x96
// button_menu.png icon at (ScreenWidth-96-30, ScreenHeight-96-30) =
// (1794,954) with ScreenWidth/Height=1920x1080) and tags_save.go's
// quickMenuButtons() (five 780x105 rows at a fixed X=570,
// Y=285+i*(105+38) for i=0..4: SAVE/LOAD/HIDE MESSAGE/SKIP/BACK TO TITLE —
// these five don't depend on ScreenWidth/Height at all).
const (
	MenuButtonX, MenuButtonY         = 1794 + 96/2, 954 + 96/2
	QuickMenuSaveX, QuickMenuSaveY   = 570 + 780/2, 285 + 105/2
	QuickMenuLoadX, QuickMenuLoadY   = 570 + 780/2, 285 + 1*(105+38) + 105/2
	QuickMenuTitleX, QuickMenuTitleY = 570 + 780/2, 285 + 4*(105+38) + 105/2
)

// ClickMenuButton opens the quick menu — a no-op if it's not currently
// visible (menuButtonVisible false, or the menu is already open — see
// handleMenuButtonClick, tags_sysdesign.go). Deprecated — see the const
// block above.
func ClickMenuButton(sess *driver.Session) error { return ClickLogical(sess, MenuButtonX, MenuButtonY) }

// ClickQuickMenuSave clicks the open quick menu's SAVE row, which opens
// the slot picker in save mode (openSlotPicker(slotPickerSave) —
// tags_sysdesign.go). Follow up with ClickSlotPickerRow1 (or another
// row) to actually write a slot. Deprecated — see the const block above.
func ClickQuickMenuSave(sess *driver.Session) error {
	return ClickLogical(sess, QuickMenuSaveX, QuickMenuSaveY)
}

// ClickQuickMenuLoad clicks the open quick menu's LOAD row, opening the
// slot picker in load mode. Deprecated — see the const block above.
func ClickQuickMenuLoad(sess *driver.Session) error {
	return ClickLogical(sess, QuickMenuLoadX, QuickMenuLoadY)
}

// ClickQuickMenuTitle clicks the open quick menu's "BACK TO TITLE" row —
// opens confirmGoToTitle's confirmation dialog (renderer.go), same as
// ClickDialogOK below is meant to follow up on; it does not jump
// immediately. Deprecated — see the const block above.
func ClickQuickMenuTitle(sess *driver.Session) error {
	return ClickLogical(sess, QuickMenuTitleX, QuickMenuTitleY)
}

// The redesigned message window's persistent operation row (tags_oprow.go)
// is the entry point scene1.ks actually exposes for SAVE/LOAD/Title, in
// place of the quick menu above. Centers computed via opRowLayout/
// opRowOrigin against scene1.ks's own [position] (left=96 top=736
// width=1728 height=300 marginright=56) and the built-in font — re-derive
// (e.g. temporarily add a t.Logf of opRowLayout's output to a Go test in
// renderer/ebitengine) if scene1.ks's [position] or the operation row's
// button set changes; unlike the quick menu's fixed pixel grid, this
// row is right-aligned and its item widths depend on font metrics, so
// there's no simple formula to hand-derive these from.
//
// The row's items and their horizontal spans, at startX=852.08 / total
// width=915.92, y spanning 698..720 (so 709 is every item's vertical
// center):
//
//	AUTO     852..909      SKIP     931..979     既読SKIP 1001..1093
//	LOG     1146..1188     Q.SAVE  1241..1316    Q.LOAD  1338..1416
//	SAVE    1469..1520     LOAD    1542..1598    CONFIG  1620..1700
//	Title   1722..1768
//
// A coordinate landing in a *gap between* labels (1520..1542, 1598..1620)
// misses the row entirely, and that miss is not self-announcing: Update()
// runs doNext() on any click at all, so a missed click still advances the
// dialogue and still changes the screen — the only thing clickGlinkRetry
// (flows_test.go) checks. The failure then surfaces several steps later as
// "the save file doesn't exist", not as "that click missed".
const (
	OpRowSaveX, OpRowSaveY   = 1495, 709
	OpRowLoadX, OpRowLoadY   = 1570, 709
	OpRowTitleX, OpRowTitleY = 1745, 709
)

// ClickOpRowSave clicks the operation row's SAVE label (buttonRoles["save"]
// — opens the slot picker in save mode, same as the deprecated quick
// menu's SAVE row). Follow up with ClickSlotPickerRow1.
func ClickOpRowSave(sess *driver.Session) error { return ClickLogical(sess, OpRowSaveX, OpRowSaveY) }

// ClickOpRowLoad clicks the operation row's LOAD label, opening the slot
// picker in load mode.
func ClickOpRowLoad(sess *driver.Session) error { return ClickLogical(sess, OpRowLoadX, OpRowLoadY) }

// ClickOpRowTitle clicks the operation row's "Title" label — opens
// confirmGoToTitle's confirmation dialog, same as ClickDialogOK below is
// meant to follow up on; it does not jump immediately.
func ClickOpRowTitle(sess *driver.Session) error {
	return ClickLogical(sess, OpRowTitleX, OpRowTitleY)
}

// Slot picker layout — see tags_uiscreens.go's slotPickerRowX/Y0/W/H/Gap
// (X=195 Y0=255 W=1500 H=180 gap=15, fixed regardless of ScreenWidth/
// Height, and matching saveslot.png's native size so it isn't stretched)
// and backButtonRect (150x150 menu_button_close.png at
// (ScreenWidth-150-30, 52) — with ScreenWidth=1920, that's (1740, 52)).
// Row i (0-indexed) is slot i+1 (slotPickerRows) — row 1 is slot
// ManualSaveSlot (save.go).
const (
	SlotPickerRow1X, SlotPickerRow1Y = 195 + 1500/2, 255 + 180/2
	SlotPickerBackX, SlotPickerBackY = 1740 + 150/2, 52 + 150/2
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
// 1920x1080: w,h=240,75; y=screenH/2+60=600; OK.X=screenW/2-w-30=690;
// NG.X=screenW/2+30=990. Used
// by confirmGoToTitle's "タイトルに戻ります。よろしいですか？" dialog
// (opened via ClickQuickMenuTitle above) and any other activeDialog with
// OnConfirm set.
const (
	dialogOKX, dialogOKY = 690 + 240/2, 600 + 75/2
	dialogNGX, dialogNGY = 990 + 240/2, 600 + 75/2
)

// ClickDialogOK clicks the OK button of whatever confirm dialog is
// currently open (handleDialogClick, renderer.go).
func ClickDialogOK(sess *driver.Session) error { return ClickLogical(sess, dialogOKX, dialogOKY) }

// ClickDialogNG clicks the NG/cancel button of whatever confirm dialog is
// currently open.
func ClickDialogNG(sess *driver.Session) error { return ClickLogical(sess, dialogNGX, dialogNGY) }
