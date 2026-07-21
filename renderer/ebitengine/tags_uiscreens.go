package ebitengine

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("showsave", handleShowSave)
	register("showload", handleShowLoad)
	register("showmenu", handleShowMenu)
	register("showlog", handleShowLog)
	register("edit", handleEdit)
	register("commit", handleCommit)
	register("preload", handlePreload)
	register("wait_preload", handleWaitPreload)
	register("unload", handleUnload)
}

// anyModalActive reports whether any of the overlays that should freeze
// normal script advancement (see Update() in renderer.go) is currently up:
// the quick menu, the backlog viewer, the save/load slot picker, an active
// [edit] text field, or a button-triggered confirm dialog (activeDialog
// with OnConfirm set — see confirmGoToTitle in renderer.go). A [dialog]
// *tag*'s dialog isn't included here — it blocks the tag coroutine
// directly via y.Until (see handleDialog in tags_save.go), so it doesn't
// need this separate freeze mechanism; only the button-triggered kind runs
// outside the coroutine and needs Update() to hold the story back itself.
func anyModalActive() bool {
	return backlogViewing || menuOpen || slotPickerActive != slotPickerNone ||
		(editState != nil && editState.Active) ||
		(activeDialog != nil && activeDialog.OnConfirm != nil)
}

// --- showsave / showload: a slot picker built on Phase 7's saveSlot/loadSlot ---

type slotPickerMode int

const (
	slotPickerNone slotPickerMode = iota
	slotPickerSave
	slotPickerLoad
)

var (
	slotPickerActive      slotPickerMode
	slotPickerOpenedFrame int
)

func handleShowSave(ctx *tagCtx) error {
	openSlotPicker(slotPickerSave)
	return nil
}

func handleShowLoad(ctx *tagCtx) error {
	openSlotPicker(slotPickerLoad)
	return nil
}

func openSlotPicker(mode slotPickerMode) {
	menuOpen = false
	slotPickerActive = mode
	slotPickerOpenedFrame = t
}

func handleShowMenu(ctx *tagCtx) error {
	menuOpen = true
	menuOpenedFrame = t
	return nil
}

func handleShowLog(ctx *tagCtx) error {
	backlogViewing = true
	backlogOpenedFrame = t
	return nil
}

// Layout constants for the save/load slot picker, chosen to match real
// Tyrano's own DATA SAVE/DATA LOAD screen (built from the same bundled
// resources/system/images assets: bg_base.png, label_save.png/
// label_load.png, menu_button_close.png, saveslot.png, thumbnail.png) at a
// 1280x720 canvas. slotPickerRowH/W deliberately equal saveslot.png's native
// size so it's never stretched.
const (
	slotPickerTitleX, slotPickerTitleY           = 20, 15
	slotPickerBackMargin, slotPickerBackY        = 20, 35
	slotPickerRowX, slotPickerRowY0              = 130, 170
	slotPickerRowW, slotPickerRowH               = 1000, 120
	slotPickerRowGap                             = 10
	slotPickerViewportBottomMargin               = 20
	slotPickerThumbInsetX, slotPickerThumbInsetY = 15, 12
	slotPickerThumbW, slotPickerThumbH           = 166, 96
	slotPickerTextInsetX, slotPickerTextInsetY   = 210, 62
	slotPickerScrollStep                         = 40
	slotPickerScrollbarW                         = 12
	slotPickerScrollbarMargin                    = 30
)

// slotPickerScrollY is how far the row list has been scrolled down (0 = top).
var slotPickerScrollY int

// systemImageCache holds every resources/system/images/* file the slot
// picker (and anything else that calls loadSystemImage) has loaded, so
// repeated per-frame draws don't re-decode from the embedded FS. A failed
// load is cached as nil too, so a missing asset only logs once instead of
// every frame.
var systemImageCache = map[string]*ebiten.Image{}

func loadSystemImage(r *Renderer, name string) *ebiten.Image {
	if img, ok := systemImageCache[name]; ok {
		return img
	}
	img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["system/images"], name)
	if err != nil {
		fmt.Printf("save/load screen: %s not found: %v\n", name, err)
		systemImageCache[name] = nil
		return nil
	}
	systemImageCache[name] = img
	return img
}

// slotThumbnailCache holds each slot's decoded savesnap thumbnail
// (slot_<N>.png, written by saveSlot — see tags_save.go). Unlike
// systemImageCache this is invalidated per-slot on every successful save
// (in saveSlot), since the file on disk can change.
var slotThumbnailCache = map[int]*ebiten.Image{}

func loadSlotThumbnail(r *Renderer, slot int) *ebiten.Image {
	if img, ok := slotThumbnailCache[slot]; ok {
		return img
	}
	dir, err := saveDir(r)
	if err != nil {
		return nil
	}
	img, _, err := ebitenutil.NewImageFromFileSystem(os.DirFS(dir), fmt.Sprintf("slot_%d.png", slot))
	if err != nil {
		// No thumbnail for this slot (savesnap/save_img was never used
		// before saving it) — saveslot.png's baked-in "NO DATA" graphic
		// shows through on its own, so this isn't an error worth logging.
		slotThumbnailCache[slot] = nil
		return nil
	}
	slotThumbnailCache[slot] = img
	return img
}

// saveSlotInfo reports whether slot has a save file on disk and, if so,
// when it was last written — the picker uses the latter as the row's status
// text in place of real Tyrano's save-time metadata (which this engine's
// minimal save format doesn't track separately; the file's own mtime is
// equivalent information for free).
func saveSlotInfo(r *Renderer, slot int) (exists bool, modTime time.Time) {
	dir, err := saveDir(r)
	if err != nil {
		return false, time.Time{}
	}
	info, err := os.Stat(slotPath(dir, slot, "json"))
	if err != nil {
		return false, time.Time{}
	}
	return true, info.ModTime()
}

// hasSaveSlot reports whether slot has a save file on disk.
func hasSaveSlot(r *Renderer, slot int) bool {
	exists, _ := saveSlotInfo(r, slot)
	return exists
}

// slotRowInfo is one row of the picker. Y is content-relative (as if
// slotPickerScrollY were 0) — both drawing and hit-testing subtract the
// current scroll offset themselves, so a row's on-screen position always
// reflects the same underlying number.
type slotRowInfo struct {
	Slot       int
	Y          int
	HasData    bool
	StatusText string
}

func slotPickerRows(r *Renderer) []slotRowInfo {
	n := r.manager.Config.ConfigSaveSlotNum
	if n <= 0 {
		n = 5
	}
	rows := make([]slotRowInfo, n)
	for i := 0; i < n; i++ {
		slot := i + 1
		exists, modTime := saveSlotInfo(r, slot)
		text := "まだ、保存されているデータがありません。"
		if exists {
			text = modTime.Format("2006/01/02 15:04:05")
		}
		rows[i] = slotRowInfo{
			Slot:       slot,
			Y:          slotPickerRowY0 + i*(slotPickerRowH+slotPickerRowGap),
			HasData:    exists,
			StatusText: text,
		}
	}
	return rows
}

// slotPickerViewport is the visible, scrollable region the row list draws
// into — from just below the title/back button down to a small bottom
// margin.
func slotPickerViewport(screenH int) (y, h int) {
	y = slotPickerRowY0
	h = screenH - y - slotPickerViewportBottomMargin
	if h < slotPickerRowH {
		h = slotPickerRowH
	}
	return
}

func slotPickerMaxScroll(rowCount, viewportH int) int {
	if rowCount == 0 {
		return 0
	}
	contentH := rowCount*(slotPickerRowH+slotPickerRowGap) - slotPickerRowGap
	max := contentH - viewportH
	if max < 0 {
		max = 0
	}
	return max
}

func clampSlotPickerScroll(r *Renderer, rowCount int) {
	_, vh := slotPickerViewport(r.manager.Config.ScreenHeight)
	max := slotPickerMaxScroll(rowCount, vh)
	if slotPickerScrollY > max {
		slotPickerScrollY = max
	}
	if slotPickerScrollY < 0 {
		slotPickerScrollY = 0
	}
}

// backButtonRect is real Tyrano's circular "BACK" button
// (menu_button_close.png), fixed at the top-right corner regardless of
// scroll.
func backButtonRect(r *Renderer) modalRect {
	size := 100
	if img := loadSystemImage(r, "menu_button_close.png"); img != nil {
		size = img.Bounds().Dx()
	}
	return modalRect{
		X: r.manager.Config.ScreenWidth - size - slotPickerBackMargin,
		Y: slotPickerBackY,
		W: size, H: size,
	}
}

// backButtonImageName picks menu_button_close.png/close2.png depending on
// whether (mX, mY) is currently hovering the back button — split out from
// drawSlotPicker so it's testable without a real ebiten.CursorPosition().
func backButtonImageName(back modalRect, mX, mY int) string {
	if isColision(mX, mY, back.X, back.Y, back.W, back.H) {
		return "menu_button_close2.png"
	}
	return "menu_button_close.png"
}

func drawSlotPicker(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	if bgImg := loadSystemImage(r, "bg_base.png"); bgImg != nil {
		op := &ebiten.DrawImageOptions{}
		bw, bh := bgImg.Bounds().Dx(), bgImg.Bounds().Dy()
		op.GeoM.Scale(float64(w)/float64(bw), float64(h)/float64(bh))
		buf.DrawImage(bgImg, op)
	} else {
		dim := ebiten.NewImage(w, h)
		dim.Fill(color.RGBA{0, 0, 0, 180})
		buf.DrawImage(dim, &ebiten.DrawImageOptions{})
	}

	titleFile := "label_load.png"
	if slotPickerActive == slotPickerSave {
		titleFile = "label_save.png"
	}
	if titleImg := loadSystemImage(r, titleFile); titleImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(slotPickerTitleX, slotPickerTitleY)
		buf.DrawImage(titleImg, op)
	}

	back := backButtonRect(r)
	mX, mY := ebiten.CursorPosition()
	if backImg := loadSystemImage(r, backButtonImageName(back, mX, mY)); backImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(back.X), float64(back.Y))
		buf.DrawImage(backImg, op)
	}

	rows := slotPickerRows(r)
	clampSlotPickerScroll(r, len(rows))
	viewportY, viewportH := slotPickerViewport(h)
	rowBg := loadSystemImage(r, "saveslot.png")

	// Rows are composited into their own buffer first and blitted at a
	// fixed screen position — the simplest way to clip the scrolling list
	// to its viewport without a partially-scrolled row bleeding into the
	// title/back button area above it.
	viewport := ebiten.NewImage(slotPickerRowW, viewportH)
	for _, row := range rows {
		y := row.Y - slotPickerScrollY - viewportY
		if y+slotPickerRowH <= 0 || y >= viewportH {
			continue
		}
		if rowBg != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(0, float64(y))
			viewport.DrawImage(rowBg, op)
		}
		if row.HasData {
			if thumb := loadSlotThumbnail(r, row.Slot); thumb != nil {
				op := &ebiten.DrawImageOptions{}
				tw, th := thumb.Bounds().Dx(), thumb.Bounds().Dy()
				op.GeoM.Scale(float64(slotPickerThumbW)/float64(tw), float64(slotPickerThumbH)/float64(th))
				op.GeoM.Translate(slotPickerThumbInsetX, float64(y+slotPickerThumbInsetY))
				viewport.DrawImage(thumb, op)
			}
		}
		textOp := &text.DrawOptions{}
		textOp.ColorScale.ScaleWithColor(color.RGBA{70, 70, 70, 255})
		textOp.GeoM.Translate(slotPickerTextInsetX, float64(y+slotPickerTextInsetY))
		text.Draw(viewport, row.StatusText, r.fontFace, textOp)
	}
	vpOp := &ebiten.DrawImageOptions{}
	vpOp.GeoM.Translate(slotPickerRowX, float64(viewportY))
	buf.DrawImage(viewport, vpOp)

	if max := slotPickerMaxScroll(len(rows), viewportH); max > 0 {
		drawSlotPickerScrollbar(buf, w, viewportY, viewportH, len(rows), slotPickerScrollY, max)
	}
}

// drawSlotPickerScrollbar is a plain flat-color track+thumb (no bundled
// scrollbar asset exists) — same minimal style as the rest of this
// package's hand-drawn UI chrome (see modalRect/drawModalRect).
func drawSlotPickerScrollbar(buf *ebiten.Image, screenW, viewportY, viewportH, rowCount, scrollY, maxScroll int) {
	trackX := screenW - slotPickerScrollbarMargin - slotPickerScrollbarW
	track := ebiten.NewImage(slotPickerScrollbarW, viewportH)
	track.Fill(color.RGBA{60, 100, 200, 140})
	trackOp := &ebiten.DrawImageOptions{}
	trackOp.GeoM.Translate(float64(trackX), float64(viewportY))
	buf.DrawImage(track, trackOp)

	contentH := rowCount*(slotPickerRowH+slotPickerRowGap) - slotPickerRowGap
	thumbH := viewportH * viewportH / contentH
	if thumbH < 30 {
		thumbH = 30
	}
	if thumbH > viewportH {
		thumbH = viewportH
	}
	thumbY := viewportY + (viewportH-thumbH)*scrollY/maxScroll
	thumb := ebiten.NewImage(slotPickerScrollbarW, thumbH)
	thumb.Fill(color.RGBA{40, 70, 220, 255})
	thumbOp := &ebiten.DrawImageOptions{}
	thumbOp.GeoM.Translate(float64(trackX), float64(thumbY))
	buf.DrawImage(thumb, thumbOp)
}

func (r *Renderer) handleSlotPickerClick() {
	if slotPickerActive == slotPickerNone {
		return
	}
	if _, wheelY := ebiten.Wheel(); wheelY != 0 {
		slotPickerScrollY -= int(wheelY * slotPickerScrollStep)
		clampSlotPickerScroll(r, r.manager.Config.ConfigSaveSlotNum)
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	if t == slotPickerOpenedFrame {
		return
	}
	mX, mY := ebiten.CursorPosition()

	back := backButtonRect(r)
	if isColision(mX, mY, back.X, back.Y, back.W, back.H) {
		slotPickerActive = slotPickerNone
		return
	}

	viewportY, viewportH := slotPickerViewport(r.manager.Config.ScreenHeight)
	for _, row := range slotPickerRows(r) {
		screenY := row.Y - slotPickerScrollY
		if screenY+slotPickerRowH <= viewportY || screenY >= viewportY+viewportH {
			continue // scrolled out of the visible viewport
		}
		if !isColision(mX, mY, slotPickerRowX, screenY, slotPickerRowW, slotPickerRowH) {
			continue
		}
		var err error
		if slotPickerActive == slotPickerSave {
			err = r.saveSlot(row.Slot)
		} else {
			err = r.loadSlot(row.Slot)
		}
		if err != nil {
			fmt.Printf("slot %d action failed: %v\n", row.Slot, err)
		}
		slotPickerActive = slotPickerNone
		return
	}
}

// --- [edit]/[commit]: a minimal ASCII text input field ---
//
// [edit] doesn't block the tag coroutine — matching real Tyrano, where the
// script continues right on to whatever button/label follows (typically an
// "OK" button whose target jumps to a label starting with [commit]). While
// active it's kept alive purely by Update()'s per-frame keyboard capture
// (handleEditInput) and the anyModalActive() freeze — see renderer.go.
// Only ASCII printable input is supported: ebitenutil has no IME-aware text
// widget to build on here.
type editBoxState struct {
	Prompt string
	// Target is a JS-assignable expression (e.g. "f.username"), written via
	// r.vm.Eval on commit.
	Target string
	Value  []rune
	Limit  int
	Active bool
}

var editState *editBoxState

func handleEdit(ctx *tagCtx) error {
	target, ok := ctx.tag.Pm["name"]
	if !ok {
		return nil
	}
	limit := 20
	if v, ok := ctx.tag.Pm["limit"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		limit = n
	}
	editState = &editBoxState{
		Prompt: ctx.tag.Pm["prompt"],
		Target: target,
		Value:  []rune(ctx.tag.Pm["default"]),
		Limit:  limit,
		Active: true,
	}
	return nil
}

// commitEdit writes the current edit box's value into its target variable
// and deactivates it. Shared by [commit] and the Enter-key shortcut in
// handleEditInput.
func commitEdit(r *Renderer) error {
	if editState == nil || !editState.Active {
		return nil
	}
	expr := editState.Target + " = " + strconv.Quote(string(editState.Value))
	_, err := r.vm.Eval(expr)
	editState.Active = false
	editState = nil
	return err
}

// handleCommit is tolerant of there being no active edit (matching e.g.
// [return] with an empty call stack elsewhere in this codebase) since it's
// commonly reached via a jump/button click whose timing a script author
// can't fully control.
func handleCommit(ctx *tagCtx) error {
	return commitEdit(ctx.r)
}

// handleEditInput captures keyboard input for the active edit box, once per
// Update() frame. Enter commits immediately (real Tyrano's own default
// shortcut), Backspace deletes the last character, and any other typed
// ASCII-printable rune is appended up to Limit.
func (r *Renderer) handleEditInput() {
	if editState == nil || !editState.Active {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if err := commitEdit(r); err != nil {
			fmt.Printf("commit failed: %v\n", err)
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(editState.Value) > 0 {
		editState.Value = editState.Value[:len(editState.Value)-1]
	}
	for _, ch := range ebiten.AppendInputChars(nil) {
		if ch < 0x20 || ch > 0x7E {
			continue // ASCII printable only
		}
		if editState.Limit > 0 && len(editState.Value) >= editState.Limit {
			continue
		}
		editState.Value = append(editState.Value, ch)
	}
}

func drawEditBox(r *Renderer, buf *ebiten.Image) {
	if editState == nil || !editState.Active {
		return
	}
	w, h := 460, 50
	x := r.manager.Config.ScreenWidth/2 - w/2
	y := r.manager.Config.ScreenHeight - 160
	box := ebiten.NewImage(w, h)
	box.Fill(color.RGBA{255, 255, 255, 240})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	buf.DrawImage(box, op)

	display := string(editState.Value) + "_"
	if editState.Prompt != "" {
		display = editState.Prompt + ": " + display
	}
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.Black)
	top.GeoM.Translate(float64(x)+12, float64(y)+12)
	text.Draw(buf, display, r.fontFace, top)
}

// --- preload / wait_preload / unload ---
//
// This engine's asset loading is already synchronous local-filesystem reads
// (embed.FS), so there's no real async I/O to get ahead of. [preload] still
// does something genuine: it eagerly decodes the image now (surfacing a
// missing/corrupt file as an error immediately, before whatever [image]/
// [chara_new]/etc. would have needed it) and keeps the decoded result
// around so [unload] has something concrete to release. [wait_preload] is
// consequently a no-op — by the time it runs, every prior [preload] in the
// same synchronous tag stream has already completed.
var preloadCache = map[string]*ebiten.Image{}

func preloadKey(folder, storage string) string {
	return folder + "/" + storage
}

func handlePreload(ctx *tagCtx) error {
	storage, ok := ctx.tag.Pm["storage"]
	if !ok {
		return nil
	}
	folder := "images"
	if v, ok := ctx.tag.Pm["folder"]; ok {
		folder = v
	}
	img, _, err := ebitenutil.NewImageFromFileSystem(ctx.r.fses[folder], storage)
	if err != nil {
		return err
	}
	preloadCache[preloadKey(folder, storage)] = img
	return nil
}

func handleWaitPreload(ctx *tagCtx) error {
	return nil
}

func handleUnload(ctx *tagCtx) error {
	storage, ok := ctx.tag.Pm["storage"]
	if !ok {
		preloadCache = map[string]*ebiten.Image{}
		return nil
	}
	folder := "images"
	if v, ok := ctx.tag.Pm["folder"]; ok {
		folder = v
	}
	delete(preloadCache, preloadKey(folder, storage))
	return nil
}
