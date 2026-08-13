package ebitengine

import (
	"image/color"
	"strconv"
	"strings"

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

// --- backlog (button role="backlog"/LOG in the operation row) ---
//
// Redesigned per the "3a バックログ" mockup: full-screen dim, a header
// ("BACKLOG"/履歴 + 閉じる✕), a fixed-width name column distinct from the
// text column (recordBacklog, tags_message.go, now keeps them separate),
// recency-fade opacity on the oldest few visible rows, a proportional
// scrollbar, and a footer with scroll/navigation hints. Coordinates are
// the source design's own 1920x1080 pixel values, unscaled — see the
// design plan's "画面解像度" note (example/ now runs at 1920x1080).

var (
	backlogViewing     bool
	backlogOpenedFrame int
	// backlogScrollY is how far scrolled *up* from the newest entry (0 =
	// showing the most recent entries at the bottom, the default/rest
	// state) — the opposite sense from slotPickerScrollY (which measures
	// down from the top), because backlog reads newest-at-bottom like a
	// chat transcript.
	backlogScrollY   float64
	backlogDragging  bool
	backlogDragLastY int
	// backlogDidDrag is "has the pointer moved since it was first pressed"
	// — deliberately a separate flag from backlogDragging ("is the pointer
	// currently held down at all"). See updateBacklogDrag's doc comment for
	// why collapsing these into one flag doesn't work.
	backlogDidDrag bool
)

const (
	backlogScrollStep = 40.0

	backlogPaddingTop       = 56.0
	backlogPaddingLeftRight = 96.0
	backlogPaddingBottom    = 44.0
	backlogHeaderTitleSize  = 34.0
	backlogHeaderSubSize    = 20.0
	backlogHeaderPaddingGap = 20.0
	backlogHeaderPaddingBtm = 20.0
	backlogCloseSize        = 22.0
	backlogBodyPaddingTop   = 34.0
	backlogNameColW         = 220.0
	backlogColGap           = 40.0
	backlogNameFontSize     = 26.0
	backlogTextFontSize     = 30.0
	backlogTextLineHeight   = 1.7
	backlogScrollbarColGap  = 28.0
	backlogScrollbarW       = 6.0
	backlogFooterPaddingTop = 20.0
	backlogFooterMarginTop  = 14.0
	backlogFooterFontSize   = 20.0
	backlogFooterItemGap    = 26.0
)

var (
	backlogDimColor          = color.RGBA{0x0a, 0x0c, 0x10, 0xe6} // rgba(10,12,16,0.9)
	backlogHeaderBorderColor = color.RGBA{0x8f, 0xc0, 0xd8, 0x66} // rgba(143,192,216,0.4)
	backlogTitleColor        = color.RGBA{0xf2, 0xf5, 0xf8, 0xff}
	backlogSubColor          = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogCloseColor        = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	backlogNameColor         = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	backlogNarrationColor    = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogTextColor         = color.RGBA{0xe6, 0xeb, 0xf0, 0xff}
	backlogScrollTrackColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0x2e} // rgba(143,192,216,0.18)
	backlogScrollThumbColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	backlogFooterBorderColor = color.RGBA{0x8f, 0xc0, 0xd8, 0x40} // rgba(143,192,216,0.25)
	backlogFooterDimColor    = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogFooterTextColor   = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	// backlogFadeSteps are the opacities applied to the oldest few visible
	// rows (top of the viewport), newest-first fading in from there —
	// matches the mockup's 0.45/0.6/0.8/1.0 sample. Rows beyond this are
	// fully opaque.
	backlogFadeSteps = []float32{0.45, 0.6, 0.8, 1.0}
)

// backlogEntryHeight is a fixed per-row height (text column's line-height
// at its font size) — entries aren't word-wrapped (matching this package's
// existing "no clipping, no wrap" simplicity elsewhere for secondary UI),
// so this is exact, not an estimate.
const backlogEntryHeight = backlogTextFontSize * backlogTextLineHeight

func backlogContentHeight() float64 {
	if len(backlog) == 0 {
		return 0
	}
	return float64(len(backlog)) * backlogEntryHeight
}

func backlogMaxScroll(viewportH float64) float64 {
	max := backlogContentHeight() - viewportH
	if max < 0 {
		max = 0
	}
	return max
}

func clampBacklogScroll(viewportH float64) {
	max := backlogMaxScroll(viewportH)
	if backlogScrollY > max {
		backlogScrollY = max
	}
	if backlogScrollY < 0 {
		backlogScrollY = 0
	}
}

func backlogFace(r *Renderer, size float64) *text.GoTextFace {
	return &text.GoTextFace{Source: r.fontFace.Source, Size: size, Language: r.fontFace.Language}
}

// backlogNameDisplay is the name column's text for entry — "──" (dimmed)
// for a nameless/narration entry ([pushlog] or a bare "#" line), matching
// the source design's placeholder-narration style.
func backlogNameDisplay(name string) (string, color.RGBA) {
	if name == "" {
		return "──", backlogNarrationColor
	}
	return name, backlogNameColor
}

func drawBacklog(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	fillRect(buf, 0, 0, float64(w), float64(h), backlogDimColor)

	contentX := backlogPaddingLeftRight
	contentRight := float64(w) - backlogPaddingLeftRight
	contentW := contentRight - contentX

	// --- header ---
	titleFace := backlogFace(r, backlogHeaderTitleSize)
	titleW, titleH := text.Measure("BACKLOG", titleFace, 0)
	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Translate(contentX, backlogPaddingTop)
	titleOp.ColorScale.ScaleWithColor(backlogTitleColor)
	text.Draw(buf, "BACKLOG", titleFace, titleOp)

	subFace := backlogFace(r, backlogHeaderSubSize)
	subOp := &text.DrawOptions{}
	subOp.GeoM.Translate(contentX+titleW+backlogHeaderPaddingGap, backlogPaddingTop+(titleH-backlogHeaderSubSize))
	subOp.ColorScale.ScaleWithColor(backlogSubColor)
	text.Draw(buf, "履歴", subFace, subOp)

	const closeLabel = "閉じる ✕"
	closeFace := backlogFace(r, backlogCloseSize)
	closeW, _ := text.Measure(closeLabel, closeFace, 0)
	closeOp := &text.DrawOptions{}
	closeOp.GeoM.Translate(contentRight-closeW, backlogPaddingTop+(titleH-backlogCloseSize))
	closeOp.ColorScale.ScaleWithColor(backlogCloseColor)
	text.Draw(buf, closeLabel, closeFace, closeOp)

	headerBottom := backlogPaddingTop + titleH + backlogHeaderPaddingBtm
	fillRect(buf, contentX, headerBottom, contentW, 1, backlogHeaderBorderColor)

	// --- footer ---
	footerFace := backlogFace(r, backlogFooterFontSize)
	const scrollHint = "ホイール／ドラッグでスクロール"
	_, footerH := text.Measure(scrollHint, footerFace, 0)
	footerBorderY := float64(h) - backlogPaddingBottom - footerH - backlogFooterPaddingTop
	fillRect(buf, contentX, footerBorderY, contentW, 1, backlogFooterBorderColor)
	footerTextY := footerBorderY + backlogFooterPaddingTop
	hintOp := &text.DrawOptions{}
	hintOp.GeoM.Translate(contentX, footerTextY)
	hintOp.ColorScale.ScaleWithColor(backlogFooterDimColor)
	text.Draw(buf, scrollHint, footerFace, hintOp)

	const backHint = "右クリックで戻る"
	const latestHint = "最新へ ▼"
	backW, _ := text.Measure(backHint, footerFace, 0)
	latestW, _ := text.Measure(latestHint, footerFace, 0)
	backOp := &text.DrawOptions{}
	backOp.GeoM.Translate(contentRight-backW, footerTextY)
	backOp.ColorScale.ScaleWithColor(backlogFooterTextColor)
	text.Draw(buf, backHint, footerFace, backOp)
	latestOp := &text.DrawOptions{}
	latestOp.GeoM.Translate(contentRight-backW-backlogFooterItemGap-latestW, footerTextY)
	latestOp.ColorScale.ScaleWithColor(backlogFooterTextColor)
	text.Draw(buf, latestHint, footerFace, latestOp)

	// --- body: scrollable name/text columns + scrollbar ---
	bodyTop := headerBottom + backlogBodyPaddingTop
	bodyBottom := footerBorderY - backlogFooterMarginTop
	viewportH := bodyBottom - bodyTop
	if viewportH < 0 {
		viewportH = 0
	}
	clampBacklogScroll(viewportH)

	textColW := contentW - backlogNameColW - backlogColGap - backlogScrollbarColGap - backlogScrollbarW
	nameFace := backlogFace(r, backlogNameFontSize)
	textFace := backlogFace(r, backlogTextFontSize)

	contentH := backlogContentHeight()
	// scrollTop is the content-space Y of the viewport's own top edge:
	// content is bottom-anchored (newest entry's bottom sits at
	// bodyBottom) when backlogScrollY == 0, and moves up as it increases.
	scrollTop := contentH - viewportH - backlogScrollY

	for i, entry := range backlog {
		rowTop := float64(i) * backlogEntryHeight
		y := bodyTop + (rowTop - scrollTop)
		if y+backlogEntryHeight < bodyTop || y > bodyBottom {
			continue
		}
		// Fade the first few rows *from the top of the viewport*, not by
		// absolute recency — matches the mockup's "fades in as you scroll
		// up toward older lines" read (the bottom-most/newest rows are
		// always fully opaque regardless of scroll position).
		rowIndexFromViewportTop := int((y - bodyTop) / backlogEntryHeight)
		alpha := float32(1.0)
		if rowIndexFromViewportTop >= 0 && rowIndexFromViewportTop < len(backlogFadeSteps) {
			alpha = backlogFadeSteps[rowIndexFromViewportTop]
		}

		name, nameColor := backlogNameDisplay(entry.Name)
		nameColor.A = uint8(float32(nameColor.A) * alpha)
		nameOp := &text.DrawOptions{}
		nameOp.GeoM.Translate(contentX, y)
		nameOp.ColorScale.ScaleWithColor(nameColor)
		text.Draw(buf, name, nameFace, nameOp)

		txtColor := backlogTextColor
		txtColor.A = uint8(float32(txtColor.A) * alpha)
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(contentX+backlogNameColW+backlogColGap, y)
		textOp.ColorScale.ScaleWithColor(txtColor)
		txt := entry.Text
		if w, _ := text.Measure(txt, textFace, 0); w > textColW {
			// No word-wrap for backlog text (matches the rest of this
			// package's secondary-UI simplicity) — truncate instead of
			// overflowing into the scrollbar column.
			for len([]rune(txt)) > 0 {
				rn := []rune(txt)
				txt = string(rn[:len(rn)-1])
				if ww, _ := text.Measure(txt+"…", textFace, 0); ww <= textColW {
					txt += "…"
					break
				}
			}
		}
		text.Draw(buf, txt, textFace, textOp)
	}

	if max := backlogMaxScroll(viewportH); max > 0 {
		trackX := contentRight - backlogScrollbarW
		fillRect(buf, trackX, bodyTop, backlogScrollbarW, viewportH, backlogScrollTrackColor)
		thumbH := viewportH * viewportH / contentH
		if thumbH < 20 {
			thumbH = 20
		}
		if thumbH > viewportH {
			thumbH = viewportH
		}
		// backlogScrollY == 0 anchors the thumb to the bottom (newest
		// visible) — the inverse of scrollY's own top-anchored sense.
		thumbY := bodyTop + (viewportH-thumbH)*(1-backlogScrollY/max)
		fillRect(buf, trackX, thumbY, backlogScrollbarW, thumbH, backlogScrollThumbColor)
	}
}

// updateBacklogDrag advances the backlog's drag-to-scroll state by one
// frame given this frame's raw pointer state, and reports how much to add
// to backlogScrollY (0 if nothing changed). Split out from
// handleBacklogClick so the drag/click distinction is testable without
// faking ebiten's real input state — this package has no way to do that
// directly (see skipShouldAdvance's own doc comment, renderer.go, for the
// same constraint on a different feature).
//
// backlogDidDrag exists as a separate flag from backlogDragging
// specifically because collapsing them doesn't work: a plain click's very
// first frame already has pressed=true with backlogDragging previously
// false, so the "not currently dragging -> start dragging" branch below
// sets backlogDragging = true on that exact frame — before
// handleBacklogClick's own justPressed check ever runs. Using
// backlogDragging there to mean "was this a click or a drag" meant every
// click, including one landing outside the backlog specifically to dismiss
// it, was misclassified as a drag before dismissal could ever be
// evaluated: the "click outside closes the backlog" behavior documented on
// handleBacklogClick below was unreachable. backlogDidDrag instead only
// ever becomes true once the pointer actually moves while held down, and
// is reset the moment a fresh press begins — so a click that never moves
// still reports backlogDidDrag == false on its own justPressed frame.
func updateBacklogDrag(pressed bool, mY int) (scrollDelta float64) {
	if pressed {
		if !backlogDragging {
			backlogDragging = true
			backlogDidDrag = false
			backlogDragLastY = mY
		} else if mY != backlogDragLastY {
			// Dragging down reveals older entries (content moves down with
			// the pointer), so backlogScrollY — which measures up from the
			// newest entry — increases.
			scrollDelta = float64(backlogDragLastY - mY)
			backlogDragLastY = mY
			backlogDidDrag = true
		}
	} else {
		backlogDragging = false
	}
	return scrollDelta
}

// handleBacklogClick drives the backlog screen's input: wheel/drag-to-scroll,
// the header's 閉じる✕ button, and any-other-click/right-click to close
// (preserving the pre-redesign "any click dismisses" behavior for clicks
// that land outside the close button, except on the very frame the screen
// opened — that click is the button press that opened it).
func (r *Renderer) handleBacklogClick() {
	w, h := r.manager.Config.ScreenWidth, r.manager.Config.ScreenHeight
	footerFace := backlogFace(r, backlogFooterFontSize)
	_, footerH := text.Measure("ホイール／ドラッグでスクロール", footerFace, 0)
	footerBorderY := float64(h) - backlogPaddingBottom - footerH - backlogFooterPaddingTop
	titleFace := backlogFace(r, backlogHeaderTitleSize)
	_, titleH := text.Measure("BACKLOG", titleFace, 0)
	headerBottom := backlogPaddingTop + titleH + backlogHeaderPaddingBtm
	bodyTop := headerBottom + backlogBodyPaddingTop
	viewportH := footerBorderY - backlogFooterMarginTop - bodyTop
	if viewportH < 0 {
		viewportH = 0
	}

	if _, wheelY := ebiten.Wheel(); wheelY != 0 {
		backlogScrollY += wheelY * backlogScrollStep
		clampBacklogScroll(viewportH)
	}

	mX, mY, justPressed, pressed, touch := pointerState()
	if delta := updateBacklogDrag(pressed, mY); delta != 0 {
		backlogScrollY += delta
		clampBacklogScroll(viewportH)
	}

	if t == backlogOpenedFrame {
		return
	}
	// Right-click-to-close is a desktop-only convenience — touch has no
	// equivalent gesture here, but the explicit close button and
	// tap-outside-to-dismiss below already cover it.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		backlogViewing = false
		return
	}
	if justPressed {
		contentRight := float64(w) - backlogPaddingLeftRight
		closeFace := backlogFace(r, backlogCloseSize)
		closeW, _ := text.Measure("閉じる ✕", closeFace, 0)
		closeX := contentRight - closeW
		closeY := backlogPaddingTop + (titleH - backlogCloseSize)
		if isColisionTouch(mX, mY, int(closeX), int(closeY), int(closeW), int(backlogCloseSize), touch) {
			backlogViewing = false
			return
		}
		// backlogDidDrag, not backlogDragging — see updateBacklogDrag's doc
		// comment for why the latter is always true by this point on a
		// plain click's own justPressed frame, and would make this branch
		// unreachable.
		if !backlogDidDrag {
			backlogViewing = false
		}
	}
}

// --- quick menu (button role="menu") ---
//
// Deprecated: reachable only via role="menu" or the (also deprecated)
// [showmenubutton] corner icon (tags_sysdesign.go) — the redesigned message
// window's own operation row (tags_oprow.go) now covers everything this
// popup offered (SAVE/LOAD/SKIP/BACK TO TITLE) except HIDE MESSAGE
// (case 2 below; still just buttonRoles["window"]'s toggle, reachable via a
// script-placed [button role="window"] if needed). Left implemented, not
// deleted, for any script that still opens it directly.
//
// Laid out to match real Tyrano's own system menu screen (built from the
// same bundled resources/system/images assets: bg_base.png, label_menu.png,
// menu_button_close.png for the "BACK" button top-right — same as the slot
// picker's — and the five menu_button_*/menu_message_close.png pill
// buttons), at a 1920x1080 canvas (×1.5 from the original 1280x720 layout —
// both these position/size constants and the underlying
// resources/system/images/*.png assets were scaled together, see the
// upscale note in the project history). scene1.ks already places individual
// save/load/skip/auto/backlog buttons directly on screen too, so this
// doesn't need a "閉じる" item of its own — the top-right BACK button
// covers that, consistently with the slot picker.
const (
	quickMenuLabelX, quickMenuLabelY    = 15, 15
	quickMenuButtonX, quickMenuButtonY0 = 570, 285
	quickMenuButtonW, quickMenuButtonH  = 780, 105
	quickMenuButtonGap                  = 38
)

var (
	menuOpen        bool
	menuOpenedFrame int
)

// quickMenuButtonSpec is one pill button: its normal/hover image pair and
// hit-test rect. The order here is the on-screen top-to-bottom order and is
// what quickMenuButtons()'s index maps to in handleQuickMenuClick.
type quickMenuButtonSpec struct {
	Normal, Hover string
	X, Y, W, H    int
}

func quickMenuButtons() []quickMenuButtonSpec {
	names := [...][2]string{
		{"menu_button_save.png", "menu_button_save2.png"},
		{"menu_button_load.png", "menu_button_load2.png"},
		{"menu_message_close.png", "menu_message_close2.png"},
		{"menu_button_skip.png", "menu_button_skip2.png"},
		{"menu_button_title.png", "menu_button_title2.png"},
	}
	items := make([]quickMenuButtonSpec, len(names))
	for i, n := range names {
		items[i] = quickMenuButtonSpec{
			Normal: n[0], Hover: n[1],
			X: quickMenuButtonX, Y: quickMenuButtonY0 + i*(quickMenuButtonH+quickMenuButtonGap),
			W: quickMenuButtonW, H: quickMenuButtonH,
		}
	}
	return items
}

// quickMenuButtonImageName picks btn.Hover/Normal depending on whether
// (mX, mY) is currently over it — split out from drawQuickMenu so it's
// testable without a real ebiten.CursorPosition(), same idea as
// backButtonImageName in tags_uiscreens.go.
func quickMenuButtonImageName(btn quickMenuButtonSpec, mX, mY int, touch bool) string {
	if isColisionTouch(mX, mY, btn.X, btn.Y, btn.W, btn.H, touch) {
		return btn.Hover
	}
	return btn.Normal
}

func drawQuickMenu(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	if bgImg := loadSystemImage(r, "bg_base.png"); bgImg != nil {
		op := &ebiten.DrawImageOptions{}
		bw, bh := bgImg.Bounds().Dx(), bgImg.Bounds().Dy()
		op.GeoM.Scale(float64(w)/float64(bw), float64(h)/float64(bh))
		buf.DrawImage(bgImg, op)
	} else {
		fillRect(buf, 0, 0, float64(w), float64(h), color.RGBA{0, 0, 0, 160})
	}

	if labelImg := loadSystemImage(r, "label_menu.png"); labelImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(quickMenuLabelX, quickMenuLabelY)
		buf.DrawImage(labelImg, op)
	}

	back := backButtonRect(r)
	mX, mY, _, _, touch := pointerState()
	if backImg := loadSystemImage(r, backButtonImageName(back, mX, mY, touch)); backImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(back.X), float64(back.Y))
		buf.DrawImage(backImg, op)
	}

	for _, btn := range quickMenuButtons() {
		if img := loadSystemImage(r, quickMenuButtonImageName(btn, mX, mY, touch)); img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(btn.X), float64(btn.Y))
			buf.DrawImage(img, op)
		}
	}
}

func (r *Renderer) handleQuickMenuClick(screenW, screenH int) {
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	if t == menuOpenedFrame {
		return
	}

	back := backButtonRect(r)
	if isColisionTouch(mX, mY, back.X, back.Y, back.W, back.H, touch) {
		menuOpen = false
		return
	}

	for idx, btn := range quickMenuButtons() {
		if !isColisionTouch(mX, mY, btn.X, btn.Y, btn.W, btn.H, touch) {
			continue
		}
		switch idx {
		case 0: // SAVE — Phase 9's slot picker (tags_uiscreens.go)
			openSlotPicker(slotPickerSave)
		case 1: // LOAD
			openSlotPicker(slotPickerLoad)
		case 2: // HIDE MESSAGE — same toggle as button role="window"
			menuOpen = false
			textPosition.Visible = !textPosition.Visible
		case 3: // MESSAGE SKIP — same toggle as button role="skip"
			menuOpen = false
			isSkip = !isSkip
			if isSkip {
				isAuto = false
			}
		case 4: // BACK TO TITLE
			menuOpen = false
			confirmGoToTitle(r)
		}
		return
	}
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
