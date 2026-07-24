//go:build windows

package e2e

import (
	"testing"
	"time"

	"github.com/ebinovel/kag3/e2e/helpers"
)

// stepDelay is the pause after each Enter/click before the next one — kag3
// only reads input once per Update() tick, and WinAppDriver's own action
// dispatch has latency too; this is cheap insurance against sending two
// inputs before the first has been processed. Not a substitute for
// WaitStable, which is still used before any assertion.
const stepDelay = 150 * time.Millisecond

// advance sends Enter and waits stepDelay — the workhorse for stepping
// through scene1.ks's [p]/[l] lines. Under KAG3_E2E_FAST (textNoWait
// forced true — see tags_message.go), every [p]/[l] resolves on exactly
// one Enter with no "reveal the rest of this line first" intermediate
// step, so counting Enters this way is meaningful.
func advance(t *testing.T, sess interface{ KeyPress(string) error }, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := sess.KeyPress("Enter"); err != nil {
			t.Fatalf("advance: KeyPress Enter (%d/%d): %v", i+1, n, err)
		}
		time.Sleep(stepDelay)
	}
}

// openingLinesBeforeGlink is scene1.ks's monologue line count from *start
// to the first [glink] choice ("もしかして、ノベルゲームの開発に興味が
// あるの？[p]" is the 9th [p]) — hand-counted against the bundled script.
const openingLinesBeforeGlink = 9

// linesAfterGlinkToRoleButtons is scene1.ks's line count from
// *selectinterest to the first [p] after the role_button block
// ("こんな風にゲームに必要な機能を..."), padded above the hand-counted
// ~58 for slack. Overshooting is safe here — kag3 just stops at whatever
// [p]/[s] it reaches next — undershooting would leave the role_button row
// unregistered, which the caller catches via the save-file check below
// rather than by asserting screen content.
const linesAfterGlinkToRoleButtons = 65

// glinkChoice1X/Y is the center of scene1.ks's first glink
// ("はい。興味あります", x=360 width=500 y=150 — kag3.GLink has no
// documented default height, but drawScene sizes an unset one to
// text.Measure(...)+20 for a size=28 face, comfortably inside a
// y+10..y+50 click target).
const glinkChoice1X, glinkChoice1Y = 360 + 500/2, 150 + 20

// advanceScene1ToRoleButtons drives a freshly-started scene1.ks from its
// opening monologue through the glink choice to the role_button row (see
// nav.go's ClickQuickSave doc comment for why the row only appears once
// chara_name_area's ptext has already been redefined to x=100). Whether it
// actually landed on the role_button row is verified by the caller
// attempting a quicksave and checking the file was written — a wrong Enter
// count fails loudly there rather than silently clicking empty screen.
func advanceScene1ToRoleButtons(t *testing.T, g *helpers.Game) {
	t.Helper()
	advance(t, g.Session, openingLinesBeforeGlink)
	if err := helpers.ClickLogical(g.Session, glinkChoice1X, glinkChoice1Y); err != nil {
		t.Fatalf("clicking glink choice: %v", err)
	}
	time.Sleep(stepDelay)
	advance(t, g.Session, linesAfterGlinkToRoleButtons)
}

// TestQuickSaveWritesCurrentPtextsPosition is the E2E regression test for
// a real reported bug (fixed in tags_save.go): the character name-plate
// ptext (chara_name_area) is redefined partway through scene1.ks from
// x=180 to x=100 (a custom message-window redesign further into the
// story). A save taken after that point must record x=100 — the position
// actually in effect at save time — not some other value. This only
// exercises the write side (see TestQuickSaveThenLoadRestoresSceneText for
// the load-side round trip); Go's own TestSaveSlotRoundTripRestoresPtexts
// already covers "does a loaded ptexts value actually get applied" at the
// unit level.
func TestQuickSaveWritesCurrentPtextsPosition(t *testing.T) {
	g := helpers.LaunchGame(t)

	if err := helpers.ClickTitleStart(g.Session); err != nil {
		t.Fatalf("clicking title start: %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for scene1 to start: %v", err)
	}

	advanceScene1ToRoleButtons(t, g)

	if err := helpers.ClickQuickSave(g.Session); err != nil {
		t.Fatalf("clicking quicksave: %v", err)
	}
	time.Sleep(500 * time.Millisecond) // saveSlot's file write

	save, err := helpers.ReadSaveSlot(g.SaveDir, helpers.QuickSaveSlot)
	if err != nil {
		t.Fatalf("reading quicksave slot (role_button row may not have been reached — check advanceScene1ToRoleButtons's Enter counts against the current scene1.ks): %v", err)
	}

	pt, ok := save.Ptexts["chara_name_area"]
	if !ok {
		t.Fatal("expected save data to include Ptexts[chara_name_area]")
	}
	if pt.X != 100 {
		t.Errorf("Ptexts[chara_name_area].X = %d, want 100 (the redesigned position in effect at save time, not the original x=180)", pt.X)
	}
	if save.CharaNamePText != "chara_name_area" {
		t.Errorf("CharaNamePText = %q, want %q", save.CharaNamePText, "chara_name_area")
	}
}
