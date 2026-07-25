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
// graphic size (all five title/button_*.png are 360x74 — confirmed via
// `file example/resources/images/title/button_*.png`). Re-derive if
// title.ks or its button graphics change.
const (
	titleStartX, titleStartY   = 135 + 360/2, 230 + 74/2
	titleLoadX, titleLoadY     = 135 + 360/2, 320 + 74/2
	titleCGX, titleCGY         = 135 + 360/2, 410 + 74/2
	titleReplayX, titleReplayY = 135 + 360/2, 500 + 74/2
	titleConfigX, titleConfigY = 135 + 360/2, 590 + 74/2
)

// ClickTitleStart clicks title.ks's "はじめから" button (target="gamestart"
// -> scene1.ks).
func ClickTitleStart(sess *driver.Session) error { return ClickLogical(sess, titleStartX, titleStartY) }

// ClickTitleLoad clicks title.ks's LOAD button (role="load", opens the
// slot picker).
func ClickTitleLoad(sess *driver.Session) error { return ClickLogical(sess, titleLoadX, titleLoadY) }

// ClickTitleCG clicks title.ks's CG button (storage="cg.ks").
func ClickTitleCG(sess *driver.Session) error { return ClickLogical(sess, titleCGX, titleCGY) }

// ClickTitleReplay clicks title.ks's REPLAY button (storage="replay.ks").
func ClickTitleReplay(sess *driver.Session) error {
	return ClickLogical(sess, titleReplayX, titleReplayY)
}

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

// scene1.ks's "role_button" row (name="role_button", all button/*.png are
// 96x26 — confirmed via `file example/resources/images/button/*.png`),
// registered partway through scene1.ks after chara_name_area's ptext is
// redefined at x=100 (see ClickQuickSave's doc comment for why that
// ordering matters). Fix isn't set on any of them, so — per
// clearNonFixButtons (renderer.go) — a screen-changing jump (a load,
// goToTitle, or any [jump]/[call] to a different storage) clears them all;
// only same-storage jumps ([link]/[glink] within scene1.ks) leave them in
// place.
const (
	roleQuickSaveX, roleQuickSaveY = 40 + 96/2, 690 + 26/2
	roleQuickLoadX, roleQuickLoadY = 140 + 96/2, 690 + 26/2
	roleSaveX, roleSaveY           = 240 + 96/2, 690 + 26/2
	roleLoadX, roleLoadY           = 340 + 96/2, 690 + 26/2
	roleTitleX, roleTitleY         = 1140 + 96/2, 690 + 26/2
)

// ClickQuickSave clicks scene1.ks's role="quicksave" button (writes
// slot 0 — see helpers.QuickSaveSlot). Only reachable once scene1.ks has
// advanced past the role_button block, i.e. after chara_name_area's ptext
// has already been redefined to x=100 — there is no in-script UI path to
// save while it's still at its original x=180.
func ClickQuickSave(sess *driver.Session) error {
	return ClickLogical(sess, roleQuickSaveX, roleQuickSaveY)
}

// ClickQuickLoad clicks scene1.ks's role="quickload" button (reads
// slot 0). This is a screen-changing jump (applySaveData sets
// screenChanged=true unconditionally — tags_save.go), so every non-Fix
// button including the role_button row itself is gone from the next frame
// on; don't chain another role_button click after this without navigating
// back to where scene1.ks re-registers them.
func ClickQuickLoad(sess *driver.Session) error {
	return ClickLogical(sess, roleQuickLoadX, roleQuickLoadY)
}

// ClickSave clicks scene1.ks's role="save" button (opens the slot picker,
// same as title.ks's LOAD/CONFIG buttons' role wiring).
func ClickSave(sess *driver.Session) error { return ClickLogical(sess, roleSaveX, roleSaveY) }

// ClickLoad clicks scene1.ks's role="load" button (opens the slot picker).
func ClickLoad(sess *driver.Session) error { return ClickLogical(sess, roleLoadX, roleLoadY) }

// ClickRoleTitle clicks scene1.ks's role="title" button — opens
// confirmGoToTitle's confirmation dialog (renderer.go), it does not jump
// immediately.
func ClickRoleTitle(sess *driver.Session) error { return ClickLogical(sess, roleTitleX, roleTitleY) }

// Confirm dialog OK/NG button centers, computed from
// tags_save.go's dialogButtonRects(screenW, screenH) at the fixed logical
// 1280x720: w,h=160,50; y=screenH/2+40=400; OK.X=screenW/2-w-20=460;
// NG.X=screenW/2+20=660. Used by confirmGoToTitle's "タイトルに戻ります。
// よろしいですか？" dialog (role="title" — see ClickRoleTitle) and any
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
