//go:build windows

package helpers

import (
	"fmt"

	"github.com/ebinovel/kag3/e2e/driver"
)

// logicalWidth/logicalHeight are kag3's fixed design resolution — see
// config.go's LoadDefault() (ScreenWidth=1280, ScreenHeight=720) and
// example/resources/config.toml, which doesn't override them. Every
// button x=/y=/width=/height= in the bundled .ks scenarios is expressed in
// this coordinate space, regardless of the actual window size or DPI.
const (
	logicalWidth  = 1280
	logicalHeight = 720
)

// ClickLogical converts a kag3 logical coordinate (as written in a [button
// x= y=] tag) into the session window's actual coordinate space and clicks
// it. The conversion is a straight ratio against the current client-area
// size (driver.Session.WindowRect), so it's correct even if the window
// isn't exactly 1280x720 — no assumption is made that it is.
func ClickLogical(sess *driver.Session, lx, ly int) error {
	r, err := sess.WindowRect()
	if err != nil {
		return fmt.Errorf("ClickLogical(%d,%d): %w", lx, ly, err)
	}
	x := lx * r.Dx() / logicalWidth
	y := ly * r.Dy() / logicalHeight
	if err := sess.Click(x, y); err != nil {
		return fmt.Errorf("ClickLogical(%d,%d) -> window(%d,%d): %w", lx, ly, x, y, err)
	}
	return nil
}

// Title screen button centers, computed from example/resources/senarios/
// title.ks's [button x= y=] top-left coordinates plus half the actual
// graphic size (all five title/button_*.png are 360x74 — confirmed via
// `file example/resources/images/title/button_*.png`). Re-derive if
// title.ks or its button graphics change.
const (
	titleStartX, titleStartY   = 135 + 360/2, 230 + 74/2
	titleLoadX, titleLoadY     = 135 + 360/2, 320 + 74/2
	titleCGX, titleCGY         = 135 + 360/2, 410 + 74/2
	titleReplayX, titleReplayY = 135 + 360/2, 500 + 74/2
	titleConfigX, titleConfigY = 135 + 360/2, 590 + 74/2
)

// ClickTitleStart clicks title.ks's "はじめから" button (target="gamestart"
// -> scene1.ks).
func ClickTitleStart(sess *driver.Session) error { return ClickLogical(sess, titleStartX, titleStartY) }

// ClickTitleLoad clicks title.ks's LOAD button (role="load", opens the
// slot picker).
func ClickTitleLoad(sess *driver.Session) error { return ClickLogical(sess, titleLoadX, titleLoadY) }

// ClickTitleCG clicks title.ks's CG button (storage="cg.ks").
func ClickTitleCG(sess *driver.Session) error { return ClickLogical(sess, titleCGX, titleCGY) }

// ClickTitleReplay clicks title.ks's REPLAY button (storage="replay.ks").
func ClickTitleReplay(sess *driver.Session) error {
	return ClickLogical(sess, titleReplayX, titleReplayY)
}

// ClickTitleConfig clicks title.ks's CONFIG button (role="sleepgame",
// storage="config.ks").
func ClickTitleConfig(sess *driver.Session) error {
	return ClickLogical(sess, titleConfigX, titleConfigY)
}

// Advance sends Enter, kag3's [l]/[p] click-to-continue key (see doNext in
// renderer.go — IsKeyJustPressed(KeyEnter) advances waiting text same as a
// left click).
func Advance(sess *driver.Session) error {
	return sess.KeyPress("Enter")
}
