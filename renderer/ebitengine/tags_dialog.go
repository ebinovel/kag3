package ebitengine

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("dialog", handleDialog)
	register("dialog_config", handleDialogConfig)
	register("dialog_config_ok", handleDialogConfigOK)
	register("dialog_config_ng", handleDialogConfigNG)
	register("dialog_config_filter", handleDialogConfigFilter)
}

// --- [dialog]: a minimal built-in OK/Cancel modal ---
//
// Real Tyrano's [dialog] shows a native/system dialog; kag3 draws its own
// tiny modal instead (dim overlay + two text buttons), since there's no
// cross-platform native dialog wired into ebiten here. It blocks the
// calling tag via the same y.Until pattern [wait]/[wse] already use, so no
// special-case coroutine freezing is needed — that's only required for
// backlog/menu below, which are triggered from button clicks outside the
// tag coroutine entirely.
type dialogState struct {
	Text        string
	OKLabel     string
	NGLabel     string
	Target      string
	FalseTarget string
	// Result: 0 pending, 1 OK clicked, 2 NG clicked.
	Result int
	// OnConfirm, if set, marks this as a dialog opened from a button click
	// (role="title" — see confirmGoToTitle) rather than a [dialog] tag:
	// there's no coroutine y.Until to block on outside a tag handler, so
	// Update() polls Result itself and calls OnConfirm once the user picks
	// OK, then clears activeDialog — see anyModalActive (tags_uiscreens.go)
	// and the resolution check in Update() (renderer.go).
	OnConfirm func(*Renderer)
}

var (
	activeDialog      *dialogState
	dialogOKLabel     = "OK"
	dialogNGLabel     = "キャンセル"
	dialogFilterColor = color.RGBA{0, 0, 0, 160}
)

func handleDialog(ctx *tagCtx) error {
	r := ctx.r
	d := &dialogState{
		Text:        ctx.tag.Pm["text"],
		OKLabel:     dialogOKLabel,
		NGLabel:     dialogNGLabel,
		Target:      strings.TrimPrefix(ctx.tag.Pm["target"], "*"),
		FalseTarget: strings.TrimPrefix(ctx.tag.Pm["false_target"], "*"),
	}
	activeDialog = d
	ctx.y.Until(true, func() bool {
		return d.Result != 0
	})
	activeDialog = nil
	label := d.Target
	if d.Result == 2 && d.FalseTarget != "" {
		label = d.FalseTarget
	}
	if label == "" {
		return nil
	}
	if v, ok := r.labels[label]; ok {
		*ctx.i = v.Index
	}
	return nil
}

func handleDialogConfig(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["ok"]; ok {
		dialogOKLabel = v
	}
	if v, ok := ctx.tag.Pm["ng"]; ok {
		dialogNGLabel = v
	}
	return handleDialogConfigFilter(ctx)
}

func handleDialogConfigOK(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["text"]; ok {
		dialogOKLabel = v
	}
	return nil
}

func handleDialogConfigNG(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["text"]; ok {
		dialogNGLabel = v
	}
	return nil
}

func handleDialogConfigFilter(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["color"]
	if !ok {
		return nil
	}
	rr, g, b, err := parseColor(v)
	if err != nil {
		return err
	}
	alpha := 160
	if a, ok := ctx.tag.Pm["opacity"]; ok {
		n, err := strconv.Atoi(a)
		if err != nil {
			return err
		}
		alpha = n
	}
	dialogFilterColor = color.RGBA{uint8(rr), uint8(g), uint8(b), uint8(alpha)}
	return nil
}

// dialogButtonRects computes the OK/NG buttons centered below the dialog
// text. w/h/the button gap and the +60 vertical offset are ×1.5 of the
// original 1280x720-tuned values (X/Y positioning itself was already
// screenW/screenH-relative and needed no change).
func dialogButtonRects(screenW, screenH int) (ok, ng modalRect) {
	w, h := 240, 75
	y := screenH/2 + 60
	ok = modalRect{Label: dialogOKLabelOr(), X: screenW/2 - w - 30, Y: y, W: w, H: h}
	ng = modalRect{Label: dialogNGLabelOr(), X: screenW/2 + 30, Y: y, W: w, H: h}
	return
}

func dialogOKLabelOr() string {
	if activeDialog != nil {
		return activeDialog.OKLabel
	}
	return dialogOKLabel
}

func dialogNGLabelOr() string {
	if activeDialog != nil {
		return activeDialog.NGLabel
	}
	return dialogNGLabel
}

func handleDialogClick(screenW, screenH int) {
	if activeDialog == nil || activeDialog.Result != 0 {
		return
	}
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	ok, ng := dialogButtonRects(screenW, screenH)
	switch {
	case isColisionTouch(mX, mY, ok.X, ok.Y, ok.W, ok.H, touch):
		activeDialog.Result = 1
	case isColisionTouch(mX, mY, ng.X, ng.Y, ng.W, ng.H, touch):
		activeDialog.Result = 2
	}
}

func drawDialog(r *Renderer, buf *ebiten.Image) {
	d := activeDialog
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	fillRect(buf, 0, 0, float64(w), float64(h), dialogFilterColor)
	tw, th := text.Measure(d.Text, r.fontFace, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.White)
	top.GeoM.Translate(float64(w)/2-tw/2, float64(h)/2-40-th/2)
	text.Draw(buf, d.Text, r.fontFace, top)
	ok, ng := dialogButtonRects(w, h)
	drawModalRect(buf, r.fontFace, ok)
	drawModalRect(buf, r.fontFace, ng)
}
