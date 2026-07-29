//go:build windows

package helpers

import (
	"image"

	"github.com/ebinovel/kag3/e2e/driver"
)

// MessageWindowRegion is the logical-coordinate rect scene1.ks's message
// box occupies — set once, right at *start before any dialogue
// (left=160 top=500 width=1000 height=200), and never repositioned again
// for the rest of the script (unlike the bundled TyranoScript sample's
// scene1.ks, which redesigned it partway through — "たそがれ図書室"'s
// scene1.ks doesn't). Comparing this region across two screenshots is how
// flow tests check "is the same line of dialogue showing" without OCR —
// kag3 has no way to read back rendered text content directly (see
// CLAUDE.md's ReadPixels note). Being the very first [position] a fresh
// run ever shows, it also doubles as the most sensitive place to catch a
// leaked textStyle/textPosition from a previous playthrough (see the
// title-return leak test) — scene1.ks's own [position] call hasn't had a
// chance to paper over anything yet.
var MessageWindowRegion = image.Rect(160, 500, 160+1000, 500+200)

// CaptureRegion screenshots sess and crops to the logical rect region,
// scaled the same way ClickLogical scales click coordinates (straight
// ratio against the current client-area size).
func CaptureRegion(sess *driver.Session, region image.Rectangle) (image.Image, error) {
	full, err := sess.Screenshot()
	if err != nil {
		return nil, err
	}
	r, err := sess.WindowRect()
	if err != nil {
		return nil, err
	}
	scaled := image.Rect(
		region.Min.X*r.Dx()/logicalWidth, region.Min.Y*r.Dy()/logicalHeight,
		region.Max.X*r.Dx()/logicalWidth, region.Max.Y*r.Dy()/logicalHeight,
	)
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	si, ok := full.(subImager)
	if !ok {
		return nil, errNoSubImage
	}
	return si.SubImage(scaled.Add(full.Bounds().Min)), nil
}

var errNoSubImage = &noSubImageError{}

type noSubImageError struct{}

func (*noSubImageError) Error() string {
	return "screenshot image type does not support SubImage (unexpected PNG decode result)"
}

// RegionsEqual reports whether a and b are near-identical (see nearlyEqual
// and stableDiffThreshold, stable.go) — not byte-exact. It used to be, on
// the reasoning that both captures come from the same process on the same
// machine in the same test run, so there's no cross-machine font-rendering
// or GPU-driver variance to tolerate the way a golden-image regression
// test would need to — but the waiting-for-click glyph mark
// (drawGlyph/glyphBounceOffset, tags_sysdesign.go) bounces continuously
// the whole time it's shown, so it lands at a different phase between any
// two captures even when nothing else on screen changed, and a byte-exact
// comparison flagged that alone as a difference (same root cause as
// WaitStable's fuzzy comparison — see its doc comment).
func RegionsEqual(a, b image.Image) bool {
	return nearlyEqual(toRGBA(a), toRGBA(b), stableDiffThreshold)
}
