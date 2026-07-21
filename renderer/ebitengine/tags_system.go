package ebitengine

import (
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	register("title", handleTitle)
	register("wait", handleWait)
	register("wt", handleWT)
	register("wait_cancel", handleWaitCancel)
	register("close", handleClose)
	// hidemessage does the same thing [close] does here (hide the message
	// window) — real Tyrano distinguishes "close a system dialog" from
	// "hide the message layer", but kag3 doesn't have system dialogs yet.
	register("hidemessage", handleClose)
	register("sleepgame", handleCall)
	register("awakegame", handleReturn)
	register("breakgame", handleBreakGame)
}

func handleTitle(ctx *tagCtx) error {
	ebiten.SetWindowTitle(ctx.tag.Pm["name"])
	return nil
}

var (
	waitStartT    int
	waitCancelled bool
)

// handleWait blocks for time= milliseconds, or until [wait_cancel] sets
// waitCancelled. Note: because tags run strictly one at a time on a single
// coroutine, a [wait_cancel] later in the *same* script can never actually
// interrupt this — nothing else runs concurrently to reach it. It's wired
// up regardless in case a future event-driven trigger (e.g. a keybind
// checked in Update()) sets waitCancelled from outside the tag stream.
func handleWait(ctx *tagCtx) error {
	ms, _ := strconv.Atoi(ctx.tag.Pm["time"])
	waitStartT = t
	waitCancelled = false
	target := ms * ebiten.TPS() / 1000
	ctx.y.Until(true, func() bool {
		return waitCancelled || t-waitStartT >= target
	})
	return nil
}

func handleWaitCancel(ctx *tagCtx) error {
	waitCancelled = true
	return nil
}

// handleWT waits for the current background transition to finish. bg.IsEnd
// (the flag [bg wait=true] itself blocks on) is never actually set by the
// transition-drawing code yet — that's Phase 4 — so this approximates
// "finished" the same way the renderer times the transition visually:
// elapsed ticks against bg.Time from bgTick.
func handleWT(ctx *tagCtx) error {
	ctx.y.Until(true, func() bool {
		return t-bgTick >= bg.Time*ebiten.TPS()/1000
	})
	return nil
}

// handleClose hides the message window, mirroring what button role="window"
// already toggles. Real Tyrano's [close] closes whatever system dialog is
// currently open; this engine doesn't have those yet, so the message
// window is the only "window" concept available to close.
func handleClose(ctx *tagCtx) error {
	textPosition.Visible = false
	return nil
}

// handleBreakGame discards the most recent sleepgame (call) frame without
// resuming it, unlike [awakegame]/[return].
func handleBreakGame(ctx *tagCtx) error {
	r := ctx.r
	if n := len(r.callStack); n > 0 {
		r.callStack = r.callStack[:n-1]
	}
	return nil
}
