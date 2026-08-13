package ebitengine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// --- shared modal plumbing: dialog / backlog / quick menu ---

// modalRect is a simple hit-testable label button, used by all three
// overlays below.
type modalRect struct {
	Label      string
	X, Y, W, H int
}

func drawModalRect(buf *ebiten.Image, face *text.GoTextFace, m modalRect) {
	fillRect(buf, float64(m.X), float64(m.Y), float64(m.W), float64(m.H), color.RGBA{255, 255, 255, 230})
	tw, th := text.Measure(m.Label, face, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.Black)
	top.GeoM.Translate(float64(m.X)+(float64(m.W)-tw)/2, float64(m.Y)+(float64(m.H)-th)/2)
	text.Draw(buf, m.Label, face, top)
}

// drawModal renders whichever overlay (if any) is currently active, on top
// of the normal scene. Returning bool isn't needed by drawScene (Update
// tracks activity itself via anyModalActive/activeDialog — tags_uiscreens.go),
// so this only draws.
func drawModal(r *Renderer, buf *ebiten.Image) {
	switch {
	case activeDialog != nil:
		drawDialog(r, buf)
	case slotPickerActive != slotPickerNone:
		drawSlotPicker(r, buf)
	case backlogViewing:
		drawBacklog(r, buf)
	case menuOpen:
		drawQuickMenu(r, buf)
	}
}
