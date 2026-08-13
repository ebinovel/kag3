package ebitengine

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// lastSnapshot is the save slot thumbnail, written alongside the next
// save*Slot call. Kept fresh automatically every frame (see drawScene in
// renderer.go, which calls captureSnapshot right before drawModal — after
// the full scene is drawn but before any modal overlay is), so by the time
// any save action runs, whatever's here is always "the scene, with no
// modal on top", regardless of how it was reached (direct button, the
// quick menu's own SAVE item, [showsave], TG.menu.doSave, ...).
var lastSnapshot *ebiten.Image

// captureSnapshot copies buf into lastSnapshot. buf is passed explicitly
// (rather than reading the renderBuffer global directly) so drawScene can
// call this with the scene-so-far *before* it draws in the current frame's
// modal overlay (see drawScene's own comment) — same image, different
// point in its own draw sequence, not necessarily renderBuffer's final
// state for this frame.
//
// Reuses the existing lastSnapshot image (Clear + redraw) instead of
// allocating a new one every call: drawScene calls this unconditionally
// every frame, so a fresh ebiten.NewImage(1920, 1080) each time was ~8MB of
// GPU texture churn per frame (~250MB/s at 30 TPS) — fine on desktop/
// simulator GPUs and their fast GC, but enough sustained memory pressure on
// an old/RAM-constrained real iOS device (confirmed: a 2017 iPad Pro 10.5",
// 4GB RAM) to trigger system-wide memory-warning stalls within the first
// idle minute on the title screen, before any scenario-specific rendering.
// [save_img] (tags_sysdesign.go's handleSaveImg) still assigns lastSnapshot
// directly to a differently-sized loaded image; the size check below
// reallocates in that case rather than corrupting it via Clear.
func captureSnapshot(buf *ebiten.Image) {
	if buf == nil {
		return
	}
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	if lastSnapshot == nil || lastSnapshot.Bounds().Dx() != w || lastSnapshot.Bounds().Dy() != h {
		lastSnapshot = ebiten.NewImage(w, h)
	} else {
		lastSnapshot.Clear()
	}
	lastSnapshot.DrawImage(buf, &ebiten.DrawImageOptions{})
	snapshotCaptureCount++
}

// snapshotCaptureCount counts captureSnapshot calls — test-only hook to
// prove drawScene actually re-captured this frame now that lastSnapshot is
// reused in place (see captureSnapshot's own comment) rather than replaced
// with a fresh, identifiably-different object every time.
var snapshotCaptureCount int
