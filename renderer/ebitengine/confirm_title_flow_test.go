package ebitengine

import (
	"image/color"
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

// TestIsJumpUnblocksBlockedSTagAfterFrozenFrames is a direct empirical
// check of the mechanism confirmGoToTitle/resolveButtonDialog depend on:
// [s] blocks the tag coroutine via y.Until(true, isJumped) (handleS in
// tags_text.go), reading the *same* package-level isJump flag Update()'s
// button/dialog handling sets. This drives the real coro.Coro (not
// fakeYield, which never actually suspends) through several Next() calls
// while [s] is blocking, "freezes" it by simply not calling Next() for a
// few iterations (simulating the confirm dialog being open), then sets
// isJump — mimicking what OnConfirm does — and resumes, to verify the
// jump is actually picked up rather than silently lost.
func TestIsJumpUnblocksBlockedSTagAfterFrozenFrames(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks": "[s]\nafter s",
	})
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
	}
	isJump = false
	isFirst = false
	r.initScript()
	co = coro.New(loop)
	isFirst = true
	defer func() { isFirst = false }()

	// Drive it until [s] is blocking (isJumped() keeps returning false).
	for i := 0; i < 5; i++ {
		if !co.Next() {
			t.Fatal("coroutine finished before reaching the blocking [s] tag")
		}
	}
	if len(r.texts) != 0 {
		t.Fatalf("texts before jump = %+v, want empty ([s] should still be blocking)", r.texts)
	}

	// "Freeze": several Next()-less frames go by (this is what
	// wasModalActive's early return in Update() does while a confirm
	// dialog is open) — isJump must still be sitting there true afterward.
	jumpIndex = 1 // the "after s" TextObject's index in scripts
	isJump = true

	// Resume — this is the frame after the confirm dialog closes.
	for i := 0; i < 5; i++ {
		if !co.Next() {
			break
		}
	}

	found := false
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text == "after s" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected the jump set while [s] was blocking to be picked up once Next() resumed; r.texts = %+v", r.texts)
	}
	if isJump {
		t.Error("expected isJump to have been consumed (cleared) by the outer script loop")
	}
}

// TestIsJumpAloneDoesNotUnblockTextWaitingOnIsWait checks the *other* real
// blocking point besides [s]: a displayed line of dialogue blocks on
// isWait (see execItem's TextObject case: y.Until(false, func() bool {
// return isWait })), which is a completely separate flag from isJump.
// Setting isJump alone — without isWait also becoming true — must NOT be
// enough to move past it; this documents why goToTitle (jumpIndex/isJump)
// is not, by itself, sufficient to escape a screen that's mid-dialogue.
func TestIsJumpAloneDoesNotUnblockTextWaitingOnIsWait(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks": "waiting text\nafter jump",
	})
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
		line:           -1, // so execItem's isNewLine check (r.line != object.Line) is true for line 0
	}
	isJump = false
	isWait = false
	isFirst = false
	r.initScript()
	co = coro.New(loop)
	isFirst = true
	defer func() { isFirst = false }()

	for i := 0; i < 5; i++ {
		if !co.Next() {
			t.Fatal("coroutine finished before blocking on the waiting text line")
		}
	}
	if isWait {
		t.Fatal("expected isWait to still be false (text line not yet acknowledged)")
	}

	// Simulate exactly what resolveButtonDialog's OnConfirm does: only
	// isJump/jumpIndex, nothing about isWait.
	jumpIndex = 1
	isJump = true

	for i := 0; i < 5; i++ {
		if !co.Next() {
			break
		}
	}

	found := false
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text == "after jump" {
				found = true
			}
		}
	}
	if found {
		t.Error("expected isJump alone to NOT be enough to move past text blocked on isWait — if this now passes, something changed and goToTitle's isJump-only approach may have become safe")
	}
}

// TestGoToTitleEscapesTextWaitingOnIsWait is the regression test for the
// actual reported bug: role="title" (via confirmGoToTitle, which only
// resolves a frame or more after the click that opened the dialog) landing
// while the *source* screen is mid-dialogue rather than resting at [s].
// goToTitle must force isWait=true (see its comment) so this doesn't get
// stuck forever the way TestIsJumpAloneDoesNotUnblockTextWaitingOnIsWait
// demonstrates a bare isJump=true does.
func TestGoToTitleEscapesTextWaitingOnIsWait(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks":  "waiting text\nnever reached",
		"title.ks": "reached title",
	})
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
		line:           -1,
	}
	isJump = false
	isWait = false
	isFirst = false
	r.initScript()
	co = coro.New(loop)
	isFirst = true
	defer func() { isFirst = false }()

	for i := 0; i < 5; i++ {
		if !co.Next() {
			t.Fatal("coroutine finished before blocking on the waiting text line")
		}
	}
	if isWait {
		t.Fatal("expected isWait to still be false (text line not yet acknowledged)")
	}

	// Simulate clicking OK on the confirm dialog while main.ks is
	// mid-dialogue, exactly as resolveButtonDialog's OnConfirm does.
	// goToTitle sets oldTick = tick - 4 (see its comment); this test drives
	// co.Next() directly rather than through Update(), so tick never
	// advances past 0 here and that offset would otherwise stick around as
	// -4 for every later test in the package that checks isTextEnded
	// (oldTick+3>=tick) via fakeYield — permanently false since tick only
	// ever increases. Reset both back to neutral once this test is done
	// with them.
	defer func() { tick, oldTick = 0, 0 }()
	r.goToTitle()

	for i := 0; i < 10; i++ {
		if !co.Next() {
			break
		}
	}

	if r.currentStorage != "title.ks" {
		t.Errorf("currentStorage after goToTitle = %q, want %q", r.currentStorage, "title.ks")
	}
	found := false
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text == "reached title" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected to reach title.ks's content after goToTitle escaped the isWait block; r.texts = %+v", r.texts)
	}
}

// TestIsTextEndedOrJumpedReleasesOnPendingJump covers role="title" clicked
// mid-dialogue: [p]'s own block (handleP, tags_text.go) waits on isTextEnded, a
// completely separate condition from the bare TextObject wait
// TestGoToTitleEscapesTextWaitingOnIsWait covers above. goToTitle forcing
// isWait=true and setting isJump=true does nothing to release a coroutine
// currently nested inside handleP's y.Until(true, ...) call — that call is
// several stack frames below initScript's outer isJump check (macro.go),
// which is what actually reads isJump, so isJump sitting there true is
// simply never observed until [p]'s own condition independently becomes
// true first — which takes a *further* real click landing on whatever screen
// is still showing (not yet the jump's destination), consumed uselessly on
// the retreating source screen. isTextEndedOrJumped (state.go) is what handleP
// actually blocks on; it must treat a pending isJump as sufficient on its
// own, independent of isTextEnded's own oldTick/tick bookkeeping.
func TestIsTextEndedOrJumpedReleasesOnPendingJump(t *testing.T) {
	defer func() { isJump, isWait, tick, oldTick = false, false, 0, 0 }()

	// isWait=true (mid-dialogue, already fully revealed) but oldTick
	// pinned behind tick — exactly the state goToTitle/applySaveData leave
	// behind via oldTick = tick - 4 (see their comments) to stop a
	// freshly-loaded/jumped-to [p] from spuriously auto-completing.
	isWait = true
	tick = 100
	oldTick = tick - 4
	isJump = false
	if isTextEndedOrJumped() {
		t.Fatal("expected isTextEndedOrJumped = false with isTextEnded false and no pending jump")
	}

	isJump = true
	if !isTextEndedOrJumped() {
		t.Error("expected isTextEndedOrJumped = true once a jump is pending, even with isTextEnded still false — a pending jump abandons the *current* screen outright, so its own [p] must not block that")
	}
}

// TestGoToTitleResetsTextPositionAndStyle is the regression test for a real
// reported bug: finishing the story, returning to title, then choosing
// "はじめから" a second time showed scene1.ks's very first line inside the
// *end-of-story* custom message window (scene1.ks's own
// [position frame="frame.png" ...] from near the end) instead of the
// default box. [position] (r.position) only merges the attributes a given
// tag call specifies, so a field no later call ever touches again (frame=,
// color=, ...) survives forever unless something resets it — goToTitle is
// that reset point. textStyle/defaultTextStyle ([font]/[deffont]) are the
// same class of never-cleared global (scene1.ks's end-of-story
// [deffont color=...] would otherwise recolor the next playthrough's text
// too) and must reset the same way.
func TestGoToTitleResetsTextPositionAndStyle(t *testing.T) {
	m := newTestManager(t, map[string]string{"title.ks": "reached title"})
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		currentStorage: "scene1.ks",
	}
	isJump = false
	isWait = false
	textPosition = &kag3.TextPosition{
		Left: 0, Top: 510, Width: 1280, Height: 210,
		FrameImage: newTestImage(1280, 210), FrameStorage: "frame.png",
		Color: color.RGBA{0xFA, 0xFA, 0xFA, 0xff},
	}
	textStyle = &kag3.TextStyle{Color: &color.RGBA{0x45, 0x4D, 0x51, 0xff}}
	defaultTextStyle = &kag3.TextStyle{Color: &color.RGBA{0x45, 0x4D, 0x51, 0xff}}
	defer func() { textPosition, textStyle, defaultTextStyle = nil, nil, nil }()

	r.goToTitle()

	if textPosition.FrameImage != nil || textPosition.FrameStorage != "" {
		t.Errorf("textPosition after goToTitle = %+v, want FrameImage/FrameStorage cleared", textPosition)
	}
	if textPosition.Width != 0 || textPosition.Height != 0 || textPosition.Left != 0 {
		t.Errorf("textPosition after goToTitle = %+v, want a fresh zero-value struct", textPosition)
	}
	if textStyle != nil {
		t.Errorf("textStyle after goToTitle = %+v, want nil", textStyle)
	}
	if defaultTextStyle != nil {
		t.Errorf("defaultTextStyle after goToTitle = %+v, want nil", defaultTextStyle)
	}
}

// TestApplySaveDataEscapesTextWaitingOnIsWait is the same regression, for
// applySaveData (load/quickload/rollback via the slot picker — another
// multi-frame modal that can just as easily resolve while the source
// screen is mid-dialogue, not resting at [s]).
func TestApplySaveDataEscapesTextWaitingOnIsWait(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks": "waiting text\nnever reached",
		"sub.ks":  "reached sub",
	})
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
		line:           -1,
	}
	isJump = false
	isWait = false
	isFirst = false
	r.initScript()
	co = coro.New(loop)
	isFirst = true
	defer func() { isFirst = false }()

	for i := 0; i < 5; i++ {
		if !co.Next() {
			t.Fatal("coroutine finished before blocking on the waiting text line")
		}
	}
	if isWait {
		t.Fatal("expected isWait to still be false (text line not yet acknowledged)")
	}

	// Simulate picking a slot in the save/load screen while main.ks is
	// mid-dialogue. Same tick/oldTick reset rationale as
	// TestGoToTitleEscapesTextWaitingOnIsWait above — applySaveData sets
	// oldTick = tick - 4 too.
	defer func() { tick, oldTick = 0, 0 }()
	if err := r.applySaveData(&saveData{Storage: "sub.ks", Index: 0}); err != nil {
		t.Fatalf("applySaveData: %v", err)
	}

	for i := 0; i < 10; i++ {
		if !co.Next() {
			break
		}
	}

	if r.currentStorage != "sub.ks" {
		t.Errorf("currentStorage after applySaveData = %q, want %q", r.currentStorage, "sub.ks")
	}
	found := false
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text == "reached sub" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected to reach sub.ks's content after applySaveData escaped the isWait block; r.texts = %+v", r.texts)
	}
}
