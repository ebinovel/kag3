//go:build windows

package helpers

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/draw"
	"time"

	"github.com/ebinovel/kag3/e2e/driver"
)

// pollInterval/requiredStableFrames control WaitStable's polling: 3
// consecutive identical screenshots 100ms apart is long enough to be
// confident a transition (fade, text reveal, dialog open) has actually
// finished, not just paused mid-frame between two draws.
const (
	pollInterval         = 100 * time.Millisecond
	requiredStableFrames = 3
)

// WaitStable polls sess.Screenshot until requiredStableFrames consecutive
// captures hash identically, or returns an error if timeout elapses first.
// This is the general-purpose "wait for whatever animation/transition is
// happening to finish" primitive every flow test uses between an action
// and its assertion — kag3 has no external "are you idle" signal (see
// KAG3_E2E_FAST in tags_message.go for the one animation category that
// *can* be short-circuited; camera/quake/fade tweens and window
// transitions cannot).
func WaitStable(sess *driver.Session, timeout time.Duration) error {
	var lastHash [32]byte
	stableCount := 0
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		img, err := sess.Screenshot()
		if err != nil {
			return fmt.Errorf("WaitStable: %w", err)
		}
		h := hashImage(img)
		if h == lastHash {
			stableCount++
			if stableCount >= requiredStableFrames {
				return nil
			}
		} else {
			stableCount = 0
			lastHash = h
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("WaitStable: frame did not stabilize within %s", timeout)
}

// hashImage normalizes img to RGBA before hashing — Screenshot returns
// whatever image.Image the PNG decoder produced (commonly *image.NRGBA),
// and its internal byte layout isn't guaranteed comparable across calls
// the way a fixed format's is.
func hashImage(img image.Image) [32]byte {
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, img, b.Min, draw.Src)
	return sha256.Sum256(rgba.Pix)
}
