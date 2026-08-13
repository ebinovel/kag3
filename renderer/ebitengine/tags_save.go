package ebitengine

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("savesnap", handleSaveSnap)
	register("autosave", handleAutoSave)
	register("autoload", handleAutoLoad)
	register("checkpoint", handleCheckpoint)
	register("rollback", handleRollback)
	register("clear_checkpoint", handleClearCheckpoint)
	register("screen_full", handleScreenFull)
	register("dialog", handleDialog)
	register("dialog_config", handleDialogConfig)
	register("dialog_config_ok", handleDialogConfigOK)
	register("dialog_config_ng", handleDialogConfigNG)
	register("dialog_config_filter", handleDialogConfigFilter)
	register("start_keyconfig", handleStartKeyConfig)
	register("stop_keyconfig", handleStopKeyConfig)
	register("closeconfirm_on", handleCloseConfirmOn)
	register("closeconfirm_off", handleCloseConfirmOff)
}

// Reserved slot numbers for the button roles that don't carry an explicit
// slot attribute: real Tyrano's save/load roles open a slot-picker screen
// (that's Phase 9's showsave/showload), so until that exists role="save"/
// "load" target one fixed slot and role="quicksave"/"quickload" another.
const (
	manualSaveSlot = 1
	quickSaveSlot  = 0
	autoSaveSlot   = -1
)

// handleSaveSnap captures the current frame as a thumbnail for the *next*
// save*Slot call — redundant with drawScene's own automatic per-frame
// capture in the common case, but harmless to keep: real Tyrano scripts
// may still call [savesnap] explicitly, and this keeps that working.
func handleSaveSnap(ctx *tagCtx) error {
	captureSnapshot(renderBuffer)
	return nil
}

func handleAutoSave(ctx *tagCtx) error { return ctx.r.saveSlot(autoSaveSlot) }
func handleAutoLoad(ctx *tagCtx) error { return ctx.r.loadSlot(autoSaveSlot) }

// checkpointData is [checkpoint]'s single in-memory snapshot — not
// persisted to disk, and not a full history stack (real Tyrano's rollback
// can step back through many prior points; this remembers only the most
// recent [checkpoint]).
var checkpointData *saveData

func handleCheckpoint(ctx *tagCtx) error {
	checkpointData = ctx.r.buildSaveData()
	return nil
}

func handleRollback(ctx *tagCtx) error {
	if checkpointData == nil {
		return nil
	}
	return ctx.r.applySaveData(checkpointData)
}

func handleClearCheckpoint(ctx *tagCtx) error {
	checkpointData = nil
	return nil
}

func handleScreenFull(ctx *tagCtx) error {
	ebiten.SetFullscreen(!ebiten.IsFullscreen())
	return nil
}

// --- start_keyconfig / stop_keyconfig / closeconfirm_on / closeconfirm_off ---
//
// kag3 has no rebindable-key system and no window-close interception yet,
// so these are honest state flags rather than faked behavior — tracked in
// case a future input layer wants to consult them, same spirit as
// [current] in tags_message.go.
var (
	keyConfigEnabled    = true
	closeConfirmEnabled bool
)

func handleStartKeyConfig(ctx *tagCtx) error { keyConfigEnabled = true; return nil }
func handleStopKeyConfig(ctx *tagCtx) error  { keyConfigEnabled = false; return nil }
func handleCloseConfirmOn(ctx *tagCtx) error { closeConfirmEnabled = true; return nil }
func handleCloseConfirmOff(ctx *tagCtx) error {
	closeConfirmEnabled = false
	return nil
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
