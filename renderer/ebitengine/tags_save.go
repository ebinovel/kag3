package ebitengine

import (
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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

// bgSaveState is the subset of *kag3.Background that's actually
// serializable (Image/NextImage are GPU textures, not JSON data) and worth
// keeping — see saveData's doc comment for what's deliberately left out.
type bgSaveState struct {
	Time     int
	IsWait   bool
	IsCross  bool
	Position string
	Method   string
	IsSystem bool
}

// saveData is the plan's minimal in-memory-first save format: enough to
// resume script execution and restore variables faithfully, not a full
// visual snapshot. Notably absent, deliberately: bg.Image/NextImage
// (no filename is tracked anywhere once [bg] loads them — see
// tags_background.go — so there's nothing to persist) and charas' Image
// (viewCharas only records position/state; the underlying registered
// Character images stay in the process-lifetime `charas` map, so loading a
// save works within the same run but won't re-populate that map after a
// fresh process start unless the target script's own [chara_new] calls run
// again first).
type saveData struct {
	Storage    string
	Index      int
	CallStack  []callFrame
	SFVars     map[string]interface{}
	FVars      map[string]interface{}
	ViewCharas []*kag3.CharaShow
	Bg         bgSaveState
}

func (r *Renderer) buildSaveData() *saveData {
	return &saveData{
		Storage:    r.currentStorage,
		Index:      currentScriptIndex,
		CallStack:  append([]callFrame(nil), r.callStack...),
		SFVars:     r.vm.ExportSF(),
		FVars:      r.vm.ExportF(),
		ViewCharas: append([]*kag3.CharaShow(nil), viewCharas...),
		Bg: bgSaveState{
			Time:     bg.Time,
			IsWait:   bg.IsWait,
			IsCross:  bg.IsCross,
			Position: bg.Position,
			Method:   bg.Method,
			IsSystem: bg.IsSystem,
		},
	}
}

// applySaveData restores everything saveData captured, then defers to the
// existing jump mechanism (jumpIndex/isJump) to actually move execution
// there — same as [jump]/button clicks, so it takes effect at the next
// script-loop iteration boundary rather than tearing r.scripts out from
// under a suspended coroutine.
func (r *Renderer) applySaveData(d *saveData) error {
	if d.Storage != "" && d.Storage != r.currentStorage {
		if err := r.loadScript(d.Storage); err != nil {
			return err
		}
	}
	r.callStack = append([]callFrame(nil), d.CallStack...)
	r.vm.RestoreF(d.FVars)
	r.vm.RestoreSF(d.SFVars)
	viewCharas = append([]*kag3.CharaShow(nil), d.ViewCharas...)
	bg.Time = d.Bg.Time
	bg.IsWait = d.Bg.IsWait
	bg.IsCross = d.Bg.IsCross
	bg.Position = d.Bg.Position
	bg.Method = d.Bg.Method
	bg.IsSystem = d.Bg.IsSystem
	r.texts = make(map[int][]Text)
	charaName = ""
	pendingRuby = ""
	isWait = false
	isSkip = false
	isAuto = false
	jumpIndex = d.Index
	isJump = true
	return nil
}

// saveBaseDirOverride lets tests point saveDir at a temp directory instead
// of the real OS config dir.
var saveBaseDirOverride string

func saveDir(r *Renderer) (string, error) {
	base := saveBaseDirOverride
	if base == "" {
		b, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = b
	}
	name := "kag3"
	if r.manager != nil && r.manager.Config != nil && r.manager.Config.Title != "" {
		name = r.manager.Config.Title
	}
	dir := filepath.Join(base, "kag3", name, "saves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func slotPath(dir string, slot int, ext string) string {
	return filepath.Join(dir, fmt.Sprintf("slot_%d.%s", slot, ext))
}

// lastSnapshot is [savesnap]'s captured thumbnail, written alongside the
// next save*Slot call. It's deliberately not cleared by saveSlot — real
// Tyrano's savesnap is meant to be called once, shortly before whichever
// save action follows.
var lastSnapshot *ebiten.Image

func (r *Renderer) saveSlot(slot int) error {
	dir, err := saveDir(r)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(r.buildSaveData(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(slotPath(dir, slot, "json"), b, 0o644); err != nil {
		return err
	}
	if lastSnapshot != nil {
		if f, err := os.Create(slotPath(dir, slot, "png")); err == nil {
			_ = png.Encode(f, lastSnapshot)
			f.Close()
		}
	}
	return nil
}

func (r *Renderer) loadSlot(slot int) error {
	dir, err := saveDir(r)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(slotPath(dir, slot, "json"))
	if err != nil {
		return err
	}
	var data saveData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	return r.applySaveData(&data)
}

// handleSaveSnap captures the current frame (before any save-menu overlay
// would be drawn on top of it) as a thumbnail for the *next* save*Slot call.
func handleSaveSnap(ctx *tagCtx) error {
	if renderBuffer == nil {
		return nil
	}
	w, h := renderBuffer.Bounds().Dx(), renderBuffer.Bounds().Dy()
	snap := ebiten.NewImage(w, h)
	snap.DrawImage(renderBuffer, &ebiten.DrawImageOptions{})
	lastSnapshot = snap
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
	box := ebiten.NewImage(m.W, m.H)
	box.Fill(color.RGBA{255, 255, 255, 230})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(m.X), float64(m.Y))
	buf.DrawImage(box, op)
	tw, th := text.Measure(m.Label, face, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.Black)
	top.GeoM.Translate(float64(m.X)+(float64(m.W)-tw)/2, float64(m.Y)+(float64(m.H)-th)/2)
	text.Draw(buf, m.Label, face, top)
}

func dialogButtonRects(screenW, screenH int) (ok, ng modalRect) {
	w, h := 160, 50
	y := screenH/2 + 40
	ok = modalRect{Label: dialogOKLabelOr(), X: screenW/2 - w - 20, Y: y, W: w, H: h}
	ng = modalRect{Label: dialogNGLabelOr(), X: screenW/2 + 20, Y: y, W: w, H: h}
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
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mX, mY := ebiten.CursorPosition()
	ok, ng := dialogButtonRects(screenW, screenH)
	switch {
	case isColision(mX, mY, ok.X, ok.Y, ok.W, ok.H):
		activeDialog.Result = 1
	case isColision(mX, mY, ng.X, ng.Y, ng.W, ng.H):
		activeDialog.Result = 2
	}
}

func drawDialog(r *Renderer, buf *ebiten.Image) {
	d := activeDialog
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	dim := ebiten.NewImage(w, h)
	dim.Fill(dialogFilterColor)
	buf.DrawImage(dim, &ebiten.DrawImageOptions{})
	tw, th := text.Measure(d.Text, r.fontFace, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.White)
	top.GeoM.Translate(float64(w)/2-tw/2, float64(h)/2-40-th/2)
	text.Draw(buf, d.Text, r.fontFace, top)
	ok, ng := dialogButtonRects(w, h)
	drawModalRect(buf, r.fontFace, ok)
	drawModalRect(buf, r.fontFace, ng)
}

// --- backlog (button role="backlog") ---

var (
	backlogViewing     bool
	backlogOpenedFrame int
)

func drawBacklog(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	dim := ebiten.NewImage(w, h)
	dim.Fill(color.RGBA{0, 0, 0, 200})
	buf.DrawImage(dim, &ebiten.DrawImageOptions{})

	margin := 40.0
	lineHeight := r.fontFace.Size + 8
	// Most recent entries at the bottom, like a chat transcript — show as
	// many as fit, walking backward from the end of backlog.
	maxLines := int((float64(h) - 2*margin) / lineHeight)
	start := len(backlog) - maxLines
	if start < 0 {
		start = 0
	}
	y := margin
	for _, line := range backlog[start:] {
		op := &text.DrawOptions{}
		op.ColorScale.ScaleWithColor(color.White)
		op.GeoM.Translate(margin, y)
		text.Draw(buf, line, r.fontFace, op)
		y += lineHeight
	}
}

// --- quick menu (button role="menu") ---
//
// scene1.ks already places individual save/load/skip/auto/backlog/etc.
// buttons directly on screen, so this doesn't need to reproduce all of
// them — just the actions real Tyrano's role="menu" panel is most often
// used for that aren't already one-click away.

var (
	menuOpen        bool
	menuOpenedFrame int
)

func quickMenuItems(screenW, screenH int) []modalRect {
	w, h, gap := 220, 50, 10
	labels := []string{"セーブ", "ロード", "タイトルへ", "閉じる"}
	total := len(labels)*(h+gap) - gap
	startY := screenH/2 - total/2
	items := make([]modalRect, len(labels))
	for i, label := range labels {
		items[i] = modalRect{Label: label, X: screenW/2 - w/2, Y: startY + i*(h+gap), W: w, H: h}
	}
	return items
}

func drawQuickMenu(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	dim := ebiten.NewImage(w, h)
	dim.Fill(color.RGBA{0, 0, 0, 160})
	buf.DrawImage(dim, &ebiten.DrawImageOptions{})
	for _, item := range quickMenuItems(w, h) {
		drawModalRect(buf, r.fontFace, item)
	}
}

func (r *Renderer) handleQuickMenuClick(screenW, screenH int) {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	if t == menuOpenedFrame {
		return
	}
	mX, mY := ebiten.CursorPosition()
	for idx, item := range quickMenuItems(screenW, screenH) {
		if !isColision(mX, mY, item.X, item.Y, item.W, item.H) {
			continue
		}
		switch idx {
		case 0:
			if err := r.saveSlot(manualSaveSlot); err != nil {
				fmt.Printf("save failed: %v\n", err)
			}
		case 1:
			if err := r.loadSlot(manualSaveSlot); err != nil {
				fmt.Printf("load failed: %v\n", err)
			}
		case 2:
			menuOpen = false
			r.goToTitle()
		case 3:
			menuOpen = false
		}
		return
	}
}

// drawModal renders whichever overlay (if any) is currently active, on top
// of the normal scene. Returning bool isn't needed by drawScene (Update
// tracks activity itself via backlogViewing/menuOpen/activeDialog), so this
// only draws.
func drawModal(r *Renderer, buf *ebiten.Image) {
	switch {
	case activeDialog != nil:
		drawDialog(r, buf)
	case backlogViewing:
		drawBacklog(r, buf)
	case menuOpen:
		drawQuickMenu(r, buf)
	}
}
