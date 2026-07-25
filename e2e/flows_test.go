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
// tags_message.go), every [p]/[l] resolves on exactly one *registered*
// Enter with no "reveal the rest of this line first" intermediate step,
// so "did the screen change" is a reliable proxy for "did the advance
// actually happen" here.
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
// scene1.ks's [p]/[l] lines by a known count.
func advance(t *testing.T, sess *driver.Session, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		advanceOne(t, sess)
	}
}

// openingLinesBeforeGlink is scene1.ks's monologue line count from *start
// to the first [glink] choice. The 9th [p] ("もしかして、ノベルゲームの
// 開発に興味があるの？[p]") is what's on screen when the count reaches 9,
// but it takes a 10th Enter to actually dismiss that line and reveal the
// glink choices — confirmed empirically frame-by-frame (each hand-counted
// [p] corresponds to what's already showing *before* the next Enter, not
// after it).
const openingLinesBeforeGlink = 10

// linesAfterGlinkToRoleButtons is scene1.ks's line count from
// *selectinterest to "はぁ、はぁ[p]" — confirmed empirically
// frame-by-frame, landing deliberately *past* the role_button block's own
// first two lines and the "標準で用意されているのは、[l] / セーブ、[l] /
// ロード、[l][cm] / タイトルへ戻る、...[p]" block that follows it, not on
// them: [l] accumulates multiple lines into one page without a [cm]
// between them, and a quicksave/quickload taken mid-accumulation only
// restores the *current* line, not the ones stacked above it (kag3
// doesn't capture partial-page accumulation state in its save format) —
// landing past the whole [l] run avoids that gap entirely rather than
// working around it. Overshooting further is safe — kag3 just stops at
// whatever [p]/[s] it reaches next — undershooting would leave the
// role_button row unregistered, which the caller catches via the
// save-file check below rather than by asserting screen content.
const linesAfterGlinkToRoleButtons = 69

// glinkChoice1X/Y is the center of scene1.ks's first glink
// ("はい。興味あります", x=360 width=500 y=150). Y offset (58, not
// height/2 computed from drawGLinks' text.Measure(...)+20 formula) is an
// empirically-measured value against an actual screenshot — the rendered
// box's vertical center did not match a from-first-principles calculation
// closely enough to trust, so this was recalibrated by eye instead. If
// scene1.ks's font/glink config ever changes, re-measure against a fresh
// screenshot rather than trusting the formula.
const glinkChoice1X, glinkChoice1Y = 360 + 500/2, 150 + 58

// advanceScene1ToRoleButtons drives a freshly-started scene1.ks from its
// opening monologue through the glink choice, past the role_button row
// (see nav.go's ClickQuickSave doc comment for why the row only appears
// once chara_name_area's ptext has already been redefined to x=100) and
// past the [l]-accumulated block right after it (see
// linesAfterGlinkToRoleButtons), landing on "はぁ、はぁ[p]" — a plain
// single-line [p] with the role_button row still on screen and no
// partial-page accumulation state to worry about. Whether it actually
// landed there is verified by the caller attempting a quicksave and
// checking the file was written — a wrong Enter count fails loudly there
// rather than silently clicking empty screen.
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

// linesAfterRoleButtonsBeforeReload is scene1.ks's line count from
// advanceScene1ToRoleButtons' landing point ("はぁ、はぁ[p]") to a few
// lines further in ("さて、もちろん音楽を鳴らすこともできるよ[l][cm]") —
// enough to guarantee the on-screen text has visibly changed before
// quickload jumps back. Deliberately stops short of the [link] choice
// scene1.ks reaches one line later ("それじゃあ、再生するよ？[l][cm]" then
// [s]): advanceOne only sends Enter, and a [link] choice only resolves on
// a click, so advancing that far would make advanceOne retry-fail with
// "screen did not change" — confirmed empirically.
const linesAfterRoleButtonsBeforeReload = 3

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
