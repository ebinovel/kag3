package ebitengine

import "github.com/hajimehoshi/ebiten/v2"

// drawButtons draws every [button]/[clickable]'s current graphic —
// EnterImg while the cursor is hovering it (if set), Graphic otherwise.
// Buttons with neither (an invisible hit zone, e.g. [clickable]) draw
// nothing.
func drawButtons(buf *ebiten.Image) {
	for _, button := range buttons {
		if button.Graphic == nil && button.EnterImg == nil {
			// An invisible hit zone (e.g. [clickable]) — nothing to draw.
			continue
		}
		buttonOp := &ebiten.DrawImageOptions{}
		buttonOp.GeoM.Translate(float64(button.X), float64(button.Y))

		// mX/mY come from the touch position while a finger is held over the
		// button on mobile (ebiten.CursorPosition() always reports (0,0)
		// there), giving the same enterimg= pressed-state feedback a mouse
		// hover gets on desktop.
		mX, mY, _, _, touch := pointerState()
		// A button with no enterimg= (e.g. config.ks's volume/speed slider
		// buttons) just keeps showing its normal graphic on hover, rather
		// than crash trying to draw a nil hover image. Graphic itself can
		// also be nil (only enterimg= given, an unusual but not invalid
		// script) — draw whichever of the two applies is actually set.
		toDraw := button.Graphic
		if isColisionTouch(mX, mY, button.X, button.Y, button.Width, button.Height, touch) && button.EnterImg != nil {
			toDraw = button.EnterImg
		}
		if toDraw != nil {
			buf.DrawImage(toDraw, buttonOp)
		}
	}
}
