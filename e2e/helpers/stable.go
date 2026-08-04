//go:build windows

package helpers

import (
	"fmt"
	"image"
	"image/draw"
	"time"

	"github.com/ebinovel/kag3/e2e/driver"
)

// pollInterval/requiredStableFrames control WaitStable's polling: 3
// consecutive near-identical screenshots 100ms apart is long enough to be
// confident a transition (fade, text reveal, dialog open) has actually
// finished, not just paused mid-frame between two draws.
//
// stableDiffThreshold is why "near-identical" and not byte-exact: the
// waiting-for-click glyph mark (drawGlyph/glyphBounceOffset,
// tags_sysdesign.go) bounces continuously the entire time it's shown —
// deliberately, per its own doc comment, "the animation itself keeps
// moving smoothly the whole time the mark is shown" — so on any screen
// resting at a [p]/[s] wait, literally no two consecutive frames are
// pixel-identical and the old byte-exact WaitStable could never return
// short of its timeout. The mark is a handful of pixels out of a
// 1920x1080+ screenshot, so capping the allowed differing-pixel fraction
// comfortably distinguishes "just the glyph bouncing" from "a real scene
// transition still in progress" without needing to know the mark's
// on-screen position.
const (
	pollInterval         = 100 * time.Millisecond
	requiredStableFrames = 3
	stableDiffThreshold  = 0.001 // fraction of pixels allowed to differ
)

// WaitStable polls sess.Screenshot until requiredStableFrames consecutive
// captures are near-identical (see stableDiffThreshold), or returns an
// error if timeout elapses first. This is the general-purpose "wait for
// whatever animation/transition is happening to finish" primitive every
// flow test uses between an action and its assertion — kag3 has no
// external "are you idle" signal (see KAG3_E2E_FAST in tags_message.go for
// the one animation category that *can* be short-circuited; camera/quake/
// fade tweens, window transitions, and the waiting-for-click glyph cannot).
func WaitStable(sess *driver.Session, timeout time.Duration) error {
	var lastImg *image.RGBA
	stableCount := 0
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		img, err := sess.Screenshot()
		if err != nil {
			return fmt.Errorf("WaitStable: %w", err)
		}
		rgba := toRGBA(img)
		if lastImg != nil && nearlyEqual(lastImg, rgba, stableDiffThreshold) {
			stableCount++
			if stableCount >= requiredStableFrames {
				return nil
			}
		} else {
			stableCount = 0
		}
		lastImg = rgba
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("WaitStable: frame did not stabilize within %s", timeout)
}

// nearlyEqual reports whether a and b differ in at most threshold fraction
// of their pixels. Mismatched bounds (shouldn't happen across consecutive
// captures of the same window) count as unequal.
func nearlyEqual(a, b *image.RGBA, threshold float64) bool {
	if a.Bounds() != b.Bounds() || len(a.Pix) != len(b.Pix) {
		return false
	}
	if len(a.Pix) == 0 {
		return true
	}
	diff := 0
	for i := 0; i+3 < len(a.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] || a.Pix[i+3] != b.Pix[i+3] {
			diff++
		}
	}
	pixelCount := len(a.Pix) / 4
	return float64(diff)/float64(pixelCount) <= threshold
}

// toRGBA normalizes img to *image.RGBA — Screenshot returns whatever
// image.Image the PNG decoder produced (commonly *image.NRGBA), and its
// internal byte layout isn't guaranteed comparable across calls the way a
// fixed format's is. Also used by RegionsEqual (region.go).
func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, img, b.Min, draw.Src)
	return rgba
}
