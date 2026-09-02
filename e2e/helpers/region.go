//go:build windows

package helpers

import (
	"image"

	"github.com/ebinovel/kag3/e2e/driver"
)

// MessageWindowRegion is the logical-coordinate rect scene1.ks's message
// box occupies — set once, right at *start before any dialogue
// (left=96 top=736 width=1728 height=300, the redesigned message window's
// geometry at 1920x1080), and never repositioned again for the rest of the script
// (unlike the bundled TyranoScript sample's scene1.ks, which redesigned it
// partway through — "たそがれ図書室"'s scene1.ks doesn't). Comparing this
// region across two screenshots is how flow tests check "is the same line
// of dialogue showing" without OCR — kag3 has no way to read back rendered
// text content directly (see CLAUDE.md's ReadPixels note). Being the very
// first [position] a fresh run ever shows, it also doubles as the most
// sensitive place to catch a leaked textStyle/textPosition from a previous
// playthrough (see the title-return leak test) — scene1.ks's own
// [position] call hasn't had a chance to paper over anything yet.
var MessageWindowRegion = image.Rect(96, 736, 96+1728, 736+300)

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
// and stableDiffThreshold, stable.go) — deliberately not byte-exact, even
// though both captures come from the same process on the same machine in
// the same test run: the waiting-for-click glyph mark
// (drawGlyph/glyphBounceOffset, tags_sysdesign.go) bounces continuously the
// whole time it's shown, so it lands at a different phase between any two
// captures even when nothing else on screen changed, and a byte-exact
// comparison flags that alone as a difference (same root cause as
// WaitStable's fuzzy comparison — see its doc comment).
func RegionsEqual(a, b image.Image) bool {
	return nearlyEqual(toRGBA(a), toRGBA(b), stableDiffThreshold)
}

// clickChangeThreshold is deliberately much larger than RegionsEqual's
// stableDiffThreshold (0.001) — a glink/button's hover/focus indicator
// (e.g. the highlight bar drawn on the currently-focused glink,
// draw_link.go) changes only a sliver of pixels, which was empirically
// confirmed to occasionally clear stableDiffThreshold on its own: the
// underlying mouse_event click didn't actually register (a known
// flakiness class in this environment — see window_windows.go's
// clickAtScreenPos doc comment), but the cursor landing on the glink and
// nudging its focus state was enough to make RegionsEqual report "the
// screen changed", so a retry loop using it could return success after a
// click that never really landed. A real click's consequence — a scene
// transition, a new background, the message box swapping content — moves
// far more than a sliver of the screen, so this threshold is set well
// above what a lone focus indicator can produce while still comfortably
// below a real transition.
const clickChangeThreshold = 0.02

// ClickRegistered reports whether b differs from a by more than
// clickChangeThreshold — a stricter bar than RegionsEqual, meant
// specifically for "did this click actually do something" retry loops
// (glink/button clicks), where a hover-only false positive would let the
// loop return before the click's real effect ever happened. Not a
// replacement for RegionsEqual elsewhere (e.g. text-advance or
// content-equality checks), which need the finer-grained threshold.
func ClickRegistered(a, b image.Image) bool {
	return !nearlyEqual(toRGBA(a), toRGBA(b), clickChangeThreshold)
}
