//go:build windows

package helpers

import (
	"image"

	"github.com/ebinovel/kag3/e2e/driver"
)

// MessageWindowRegion is the logical-coordinate rect scene1.ks's message
// box occupies once repositioned to left=20 top=400 width=920 height=200
// (its steady-state box for most of the script — see the "メッセージボックス
// は、自分の好きな画像を使うこともできるよ" section). Comparing this region
// across two screenshots is how flow tests check "is the same line of
// dialogue showing" without OCR — kag3 has no way to read back rendered
// text content directly (see CLAUDE.md's ReadPixels note).
var MessageWindowRegion = image.Rect(20, 400, 20+920, 400+200)

// OpeningMessageWindowRegion is scene1.ks's *very first* [position]
// (left=160 top=500 width=1000 height=200, set right at *start before any
// dialogue) — different from MessageWindowRegion, which only applies from
// partway through the script onward. Used to compare the opening line's
// rendering across two playthroughs (see the title-return leak test):
// this is deliberately the *first* box a fresh scene1.ks run ever shows,
// so any textStyle/textPosition state goToTitle failed to reset would
// show up here immediately, before scene1.ks's own [position]/[font]
// calls have a chance to paper over it.
var OpeningMessageWindowRegion = image.Rect(160, 500, 160+1000, 500+200)

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

// RegionsEqual reports whether a and b hash identically after normalizing
// to RGBA (see hashImage in stable.go). This is a byte-exact comparison —
// deliberately not a fuzzy/perceptual one — because both captures come
// from the same process on the same machine in the same test run, so
// there's no cross-machine font-rendering or GPU-driver variance to
// tolerate the way a golden-image regression test would need to.
func RegionsEqual(a, b image.Image) bool {
	return hashImage(a) == hashImage(b)
}
