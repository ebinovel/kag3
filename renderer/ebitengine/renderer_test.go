package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestDrawSceneButtonWithoutEnterImgDoesNotPanicOnHover reproduces a real
// crash: config.ks's volume/speed slider buttons (e.g.
// [button graphic="&tf.btn_path_off" ... ] with no enterimg= at all) have
// button.EnterImg == nil while button.Graphic is set. drawScene's old hover
// check only skipped drawing when *both* were nil, so as soon as the mouse
// hovered one of these buttons it tried buf.DrawImage(nil, ...) and
// panicked.
func TestDrawSceneButtonWithoutEnterImgDoesNotPanicOnHover(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	textPosition = &kag3.TextPosition{Visible: false}
	bg = &kag3.Background{}
	bg2 = &kag3.Background{}
	viewCharas = nil
	links = nil
	glinks = nil
	imgs = nil
	// A huge rect starting at the origin so it covers whatever position a
	// headless test environment reports for ebiten.CursorPosition() — the
	// point is exercising the hover branch, not any specific coordinate.
	buttons = []*kag3.Button{
		{Graphic: newTestImage(10, 10), EnterImg: nil, X: 0, Y: 0, Width: 100000, Height: 100000},
	}
	defer func() { buttons = nil }()

	buf := newTestImage(1280, 720)
	r.drawScene(buf) // must not panic
}
