//go:build windows

package e2e

import (
	"testing"
	"time"

	"github.com/ebinovel/kag3/e2e/driver"
	"github.com/ebinovel/kag3/e2e/helpers"
)

// stepDelay is the pause after each Enter/click before checking whether
// the screen actually changed (see advanceOne). Click/KeyPress go straight
// to Win32 (SetCursorPos+mouse_event, keybd_event — see
// driver/window_windows.go), not through WinAppDriver's own
// (non-functional in this environment) input endpoints, so each call
// returns almost instantly; stepDelay is what actually paces the flow.
const stepDelay = 300 * time.Millisecond

// maxAdvanceAttempts bounds advanceOne's retry loop. keybd_event
// keystrokes were confirmed empirically to occasionally not register with
// kag3's Update() loop (screen unchanged after the usual stepDelay) even
// at generous delays — likely a timing race in however this environment
// delivers the synthesized key event, not something a longer fixed delay
// reliably avoids. Retrying (rather than trusting a single KeyPress) is
// what actually gets a reliable result.
const maxAdvanceAttempts = 5

// advanceOne sends Enter and confirms the screen actually changed
// (RegionsEqual on a full screenshot), retrying up to maxAdvanceAttempts
// times if not — see maxAdvanceAttempts' doc comment for why a retry loop
// is necessary at all. Under KAG3_E2E_FAST (textNoWait forced true — see
// tags_message.go), every [p] resolves on exactly one *registered* Enter
// with no "reveal the rest of this line first" intermediate step, so "did
// the screen change" is a reliable proxy for "did the advance actually
// happen" here.
func advanceOne(t *testing.T, sess *driver.Session) {
	t.Helper()
	before, err := sess.Screenshot()
	if err != nil {
		t.Fatalf("advanceOne: screenshot before: %v", err)
	}
	for attempt := 1; attempt <= maxAdvanceAttempts; attempt++ {
		if err := sess.KeyPress("Enter"); err != nil {
			t.Fatalf("advanceOne: KeyPress Enter (attempt %d/%d): %v", attempt, maxAdvanceAttempts, err)
		}
		time.Sleep(stepDelay)
		after, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("advanceOne: screenshot after (attempt %d/%d): %v", attempt, maxAdvanceAttempts, err)
		}
		if !helpers.RegionsEqual(before, after) {
			return
		}
	}
	t.Fatalf("advanceOne: screen did not change after %d Enter attempts", maxAdvanceAttempts)
}

// advance calls advanceOne n times — the workhorse for stepping through
// scene1.ks's [p] lines by a known count.
func advance(t *testing.T, sess *driver.Session, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		advanceOne(t, sess)
	}
}

// scene1ParagraphCount is scene1.ks's total [p] count (`grep -c '\[p\]'`),
// past which it @jumps straight to scene2.ks with no further wait — advance
// past this many and the next advanceOne would retry-fail with "screen did
// not change" once scene2.ks's own first [p] is reached and waiting (a
// *different* wait, but advanceOne can't tell that from "nothing
// happened"). Both flows below deliberately stop short of it.
const scene1ParagraphCount = 22

// TestTitleReturnDoesNotLeakPreviousPlaythroughStyle is the E2E regression
// test for goToTitle's reset contract (renderer.go): [font]/[deffont]/
// [position] only ever *merge* the attributes a given call specifies, so
// anything a previous playthrough set and never explicitly cleared would
// otherwise bleed into the next one. scene1.ks's opening [position]
// (left=96 top=736 width=1728 height=300, set once at *start and never
// repositioned again — see MessageWindowRegion's doc comment) is compared
// across two playthroughs: advance partway into scene1.ks (past
// [chara_show]/[chara_mod] calls, so there's some actual state for
// goToTitle to fail to reset), return to title via the quick menu, start a
// fresh playthrough, and check the very first line renders identically.
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
	baseline, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing baseline region: %v", err)
	}

	// Advance a bit further (character shown, a couple of face changes)
	// before returning to title — exercises more of goToTitle's reset path
	// than bailing out immediately would. Deliberately well short of
	// scene1ParagraphCount.
	advance(t, g.Session, 8)

	clickGlinkRetry(t, g.Session, helpers.OpRowTitleX, helpers.OpRowTitleY, "operation row Title")
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
	afterReturn, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing 2nd-playthrough region: %v", err)
	}

	if !helpers.RegionsEqual(baseline, afterReturn) {
		t.Error("scene1.ks's opening line renders differently on the 2nd playthrough — a previous playthrough's textStyle/textPosition/frame leaked through goToTitle instead of being reset")
	}
}

// TestQuickSaveThenLoadRestoresSceneText is the E2E regression test for
// same-process save/load: save at one point in scene1.ks via the
// operation row's SAVE label + slot picker, advance further (so the screen
// is showing something else), load the same slot back, and check the
// message window's content matches what was on screen at save time.
func TestQuickSaveThenLoadRestoresSceneText(t *testing.T) {
	g := helpers.LaunchGame(t)

	if err := helpers.ClickTitleStart(g.Session); err != nil {
		t.Fatalf("clicking title start: %v", err)
	}
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for scene1 to start: %v", err)
	}

	advance(t, g.Session, 5)
	if err := helpers.WaitStable(g.Session, 3*time.Second); err != nil {
		t.Fatalf("waiting for pre-save line to settle: %v", err)
	}
	before, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing pre-save region: %v", err)
	}

	clickGlinkRetry(t, g.Session, helpers.OpRowSaveX, helpers.OpRowSaveY, "operation row SAVE")
	clickGlinkRetry(t, g.Session, helpers.SlotPickerRow1X, helpers.SlotPickerRow1Y, "slot picker row 1 (save)")
	time.Sleep(500 * time.Millisecond) // saveSlot's file write

	if _, err := helpers.ReadSaveSlot(g.SaveDir, helpers.ManualSaveSlot); err != nil {
		t.Fatalf("reading save slot %d after saving (slot picker click may not have landed): %v", helpers.ManualSaveSlot, err)
	}

	advance(t, g.Session, 3)
	if err := helpers.WaitStable(g.Session, 3*time.Second); err != nil {
		t.Fatalf("waiting for post-advance line to settle: %v", err)
	}
	during, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing mid-flow region: %v", err)
	}
	if helpers.RegionsEqual(before, during) {
		t.Fatal("screen did not change after advancing further — test setup problem, not a kag3 bug")
	}

	clickGlinkRetry(t, g.Session, helpers.OpRowLoadX, helpers.OpRowLoadY, "operation row LOAD")
	clickGlinkRetry(t, g.Session, helpers.SlotPickerRow1X, helpers.SlotPickerRow1Y, "slot picker row 1 (load)")
	if err := helpers.WaitStable(g.Session, 5*time.Second); err != nil {
		t.Fatalf("waiting for load to settle: %v", err)
	}
	after, err := helpers.CaptureRegion(g.Session, helpers.MessageWindowRegion)
	if err != nil {
		t.Fatalf("capturing post-load region: %v", err)
	}

	if !helpers.RegionsEqual(before, after) {
		t.Error("message window content after load does not match the state at save time — same-process save/load did not restore scene position")
	}
}
