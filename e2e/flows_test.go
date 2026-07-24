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

// TestTitleReturnDoesNotLeakPreviousPlaythroughStyle is the E2E regression
// test for goToTitle's reset contract (renderer.go): [font]/[deffont]/
// [position] only ever *merge* the attributes a given call specifies, so
// anything a previous playthrough set and never explicitly cleared (most
// notably [deffont color=...] and [position frame=...]) would otherwise
// bleed into the next one. This drives scene1.ks far enough to trigger
// scene1.ks's own [deffont color="0x454D51"] call (partway through, right
// before the role_button block), returns to title via role="title", starts
// a second playthrough, and checks that scene1.ks's very first line (its
// opening [position] at left=160 top=500 width=1000 height=200, before any
// [font]/[deffont] call of its own) renders identically both times — see
// OpeningMessageWindowRegion's doc comment for why that specific box is
// the most sensitive point to check.
func TestTitleReturnDoesNotLeakPreviousPlaythroughStyle(t *testing.T) {
	g := helpers.LaunchGame(t)

	if err := helpers.ClickTitleStart(g.Session); err != nil {
		t.Fatalf("clicking title start (1st playthrough): %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for scene1 to start (1st playthrough): %v", err)
	}
	advance(t, g.Session, 1)
	if err := helpers.WaitStable(g.Session, 3*time.Second); err != nil {
		t.Fatalf("waiting for opening line to settle (1st playthrough): %v", err)
	}
	baseline, err := helpers.CaptureRegion(g.Session, helpers.OpeningMessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing baseline region: %v", err)
	}

	// Continue through scene1.ks's [deffont color="0x454D51"] call
	// (partway through, right before the role_button block) so there's
	// actually something for goToTitle to fail to reset.
	advance(t, g.Session, openingLinesBeforeGlink-1)
	if err := helpers.ClickLogical(g.Session, glinkChoice1X, glinkChoice1Y); err != nil {
		t.Fatalf("clicking glink choice: %v", err)
	}
	time.Sleep(stepDelay)
	advance(t, g.Session, linesAfterGlinkToRoleButtons)

	if err := helpers.ClickRoleTitle(g.Session); err != nil {
		t.Fatalf("clicking role=title button: %v", err)
	}
	time.Sleep(stepDelay)
	if err := helpers.ClickDialogOK(g.Session); err != nil {
		t.Fatalf("clicking confirm dialog OK: %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for title screen to settle: %v", err)
	}

	if err := helpers.ClickTitleStart(g.Session); err != nil {
		t.Fatalf("clicking title start (2nd playthrough): %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for scene1 to start (2nd playthrough): %v", err)
	}
	advance(t, g.Session, 1)
	if err := helpers.WaitStable(g.Session, 3*time.Second); err != nil {
		t.Fatalf("waiting for opening line to settle (2nd playthrough): %v", err)
	}
	afterReturn, err := helpers.CaptureRegion(g.Session, helpers.OpeningMessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing 2nd-playthrough region: %v", err)
	}

	if !helpers.RegionsEqual(baseline, afterReturn) {
		t.Error("scene1.ks's opening line renders differently on the 2nd playthrough — a previous playthrough's textStyle/textPosition/frame leaked through goToTitle instead of being reset")
	}
}

// linesAfterRoleButtonsBeforeReload is scene1.ks's line count from the
// role_button block's first line ("こんな風にゲームに必要な機能を...") to
// a few lines further in ("はぁ、はぁ[p]") — enough to guarantee the
// on-screen text has visibly changed before quickload jumps back, without
// running past scene1.ks's own role_button re-registration (there is
// none — the row is only ever set up once).
const linesAfterRoleButtonsBeforeReload = 5

// TestQuickSaveThenLoadRestoresSceneText is the E2E regression test for
// same-process save/load: quicksave at one point in scene1.ks, advance
// further (so the screen is showing something else), quickload, and check
// the message window's content matches what was on screen at save time.
// Both quicksave/quickload are same-storage operations in terms of button
// availability up to the load itself — [p]/[l] advancement never triggers
// screenChanged, so the role_button row (registered once, never Fix=true)
// stays clickable right up until quickload's applySaveData call, which
// unconditionally sets screenChanged=true and would clear it afterward
// (renderer.go's clearNonFixButtons) — hence no attempt to click anything
// after the load in this test.
func TestQuickSaveThenLoadRestoresSceneText(t *testing.T) {
	g := helpers.LaunchGame(t)

	if err := helpers.ClickTitleStart(g.Session); err != nil {
		t.Fatalf("clicking title start: %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for scene1 to start: %v", err)
	}

	advanceScene1ToRoleButtons(t, g)
	if err := helpers.WaitStable(g.Session, 3*time.Second); err != nil {
		t.Fatalf("waiting for role_button line to settle: %v", err)
	}
	before, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing pre-save region: %v", err)
	}

	if err := helpers.ClickQuickSave(g.Session); err != nil {
		t.Fatalf("clicking quicksave: %v", err)
	}
	time.Sleep(500 * time.Millisecond) // saveSlot's file write

	advance(t, g.Session, linesAfterRoleButtonsBeforeReload)
	if err := helpers.WaitStable(g.Session, 3*time.Second); err != nil {
		t.Fatalf("waiting for post-advance line to settle: %v", err)
	}
	during, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing mid-flow region: %v", err)
	}
	if helpers.RegionsEqual(before, during) {
		t.Fatal("screen did not change after advancing further — test setup problem (adjust linesAfterRoleButtonsBeforeReload), not a kag3 bug")
	}

	if err := helpers.ClickQuickLoad(g.Session); err != nil {
		t.Fatalf("clicking quickload: %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for load to settle: %v", err)
	}
	after, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing post-load region: %v", err)
	}

	if !helpers.RegionsEqual(before, after) {
		t.Error("message window content after quickload does not match the state at quicksave time — same-process save/load did not restore scene position")
	}
}
