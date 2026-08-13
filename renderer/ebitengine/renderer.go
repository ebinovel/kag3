package ebitengine

import (
	"image/color"
	"io/fs"
	"math"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Text struct {
	Text      string
	TextStyle *kag3.TextStyle
	Ruby      string
}

type Renderer struct {
	manager *kag3.Manager
	scripts []any
	// currentScripts is set by execItem (macro.go) on every TagObject it
	// processes, to whichever slice is actually being iterated right now —
	// see its doc comment there for why this exists (skipIfChain/[ignore]/
	// [keyframe] must not always assume r.scripts).
	currentScripts   []any
	labels           map[string]kag3.LabelInfo
	fontFace         *text.GoTextFace
	nameFontFace     *text.GoTextFace
	verticalFontFace *text.GoTextFace
	fses             map[string]fs.FS
	texts            map[int][]Text
	line             int
	Done             bool
	vm               *VM
	currentStorage   string
	callStack        []callFrame
	// sleepStack is role="sleepgame"/[sleepgame]'s own return-address stack,
	// deliberately separate from callStack: config.ks (the bundled sample)
	// calls [clearstack] right before [awakegame] to discard any leftover
	// [button target=...]-click call frames (see the button click handler
	// below) — if sleepgame/awakegame shared callStack, that same
	// [clearstack] would also wipe the frame awakegame needs to find its
	// way back to the scenario that opened the config screen.
	sleepStack []sleepFrame
}

// callFrame is a [call]'s return address: the storage it was called from
// and the item index to resume at. Plain string+int so it can round-trip
// through JSON once save/load exists.
type callFrame struct {
	Storage string
	Index   int
}

// sleepFrame is role="sleepgame"/[sleepgame]'s return-address entry. Real
// Tyrano's sleepgame is a layer-overlay mechanic — it pauses/hides the
// calling scene rather than tearing it down, so its buttons, background and
// message window are still there underneath when the overlay closes. kag3
// has no such layering: opening config.ks fully replaces r.scripts and
// repoints the single package-level bg/textPosition at config.ks's own
// background (bg_config.png) and message-window geometry — without a
// snapshot to restore, the caller's buttons (e.g. title.ks's), background
// (title.jpg) and message window would stay gone/wrong even though
// execution correctly resumes there. TextPosition matters even when the
// caller never shows a message box itself: config.ks's own
// *ch_speed_change repoints textPosition to a tiny "message1" preview box
// (for its text-speed sample) and never restores it — [layopt visible=...]
// is the real Tyrano tag that's supposed to hide it again, but kag3 has no
// per-layer visibility system yet (see handleLayopt's doc comment), so that
// call is a no-op and the preview box would otherwise stay on screen
// (wrong size/position, still Visible) after leaving config. Buttons/Bg/
// TextPosition deliberately aren't part of callFrame: only sleepgame
// crosses full scenes with visual teardown in between, so only it needs
// this.
//
// Ptexts/CharaNamePText are the same story, discovered later: drawPTexts
// used to only run while a message window was visible, so a stale ptexts
// map from whatever screen opened config.ks (e.g. title.ks's own title/
// menu labels) was invisible by accident. Once that gate was removed (see
// drawPTexts's doc comment), those areas started bleeding straight through
// config.ks's full-screen layout — and clearing them unconditionally on the
// way in, with no restore, would just move the bug to the other direction
// (losing scene1.ks's character name-plate on the way back). Snapshotting
// and restoring them here, the same way TextPosition already is, fixes
// both directions at once.
//
// ViewCharas is the exact same class of bug, just for standing-character
// sprites instead of ptexts: drawCharacters (renderer.go's drawScene) has
// no gate at all tied to which screen is active, so whatever characters
// were showing in the calling scene keep drawing straight through
// config.ks's full-screen layout, landing wherever the story scene's
// character positioning put them (bottom-anchored, centered) — which
// visually collides with config.ks's own rows since neither screen knows
// about the other. Same fix shape as Ptexts: snapshot and clear on the way
// in, restore on the way out.
type sleepFrame struct {
	Storage        string
	Index          int
	Buttons        []*kag3.Button
	Bg             kag3.Background
	TextPosition   kag3.TextPosition
	Ptexts         map[string]*kag3.PText
	CharaNamePText string
	ViewCharas     []*kag3.CharaShow
}

func NewRenderer(manager *kag3.Manager) (r *Renderer, err error) {
	r = &Renderer{
		manager:  manager,
		scripts:  manager.Senario,
		labels:   manager.Labels,
		fontFace: manager.FontFace,
		nameFontFace: &text.GoTextFace{
			Source:   manager.FontFace.Source,
			Size:     manager.FontFace.Size,
			Language: manager.FontFace.Language,
		},
		verticalFontFace: manager.VerticalFontFace,
		fses:             manager.FSes,
		vm:             newVM(),
		currentStorage: manager.CurrentStorage,
	}
	r.vm.SetConfig(manager.Config)
	r.vm.SetMenuHooks(r)
	r.vm.SetJQueryHooks(r)
	r.initScript()
	img := ebiten.NewImage(manager.Config.ScreenWidth, manager.Config.ScreenHeight)
	img.Fill(color.Black)
	bg.Image = img
	beforeTextSize = manager.FontFace.Size
	r.texts = make(map[int][]Text)
	return
}

func doNext() bool {
	_, _, justPressed, _, _ := pointerState()
	return justPressed || inpututil.IsKeyJustPressed(ebiten.KeyEnter)
}

func (r *Renderer) Update() {
	t++
	// wasModalActive/the check before the co.Next() loop below prevent a
	// single click (or keypress, for [edit]'s Enter-to-commit) from both
	// toggling/driving one of these overlays *and* being read by doNext()
	// as "advance the story" in the same frame — the tag coroutine may be
	// sitting blocked in an unrelated [s]/[wait] at the exact moment the
	// user opens/closes one of these overlays. See anyModalActive in
	// tags_uiscreens.go.
	wasModalActive := anyModalActive()
	// hoveringClickable drives [cursor]'s pointer-vs-default swap (see
	// tags_sysdesign.go); recomputed fresh below wherever a link/glink/
	// button already runs an isColision hit-test for its own purposes.
	hoveringClickable = false
	r.handleEditInput()
	if !isFirst {
		co = coro.New(loop)
		isFirst = true
	}
	if doNext() {
		if !isWait {
			isWait = true
		} else {
			oldTick = tick
		}
	}
	if !isWait {
		autoStartT = t
	}
	if isAuto && isWait && t-autoStartT >= autoWaitMs*ebiten.TPS()/1000 {
		oldTick = tick
	}
	// Skip mode used to force isWait+oldTick unconditionally, every single
	// frame, regardless of whether the line had even been drawn yet — a
	// line could complete its reveal and satisfy [p]'s wait within the same
	// frame it appeared, before ever showing on screen (reported as skip
	// feeling instantaneous rather than readable). ticksPerChar()
	// (tags_message.go) already reveals text faster than normal while
	// skipActive(), so by the time isWait naturally goes true (the reveal
	// finished, same mechanism a real click racing the reveal also hits —
	// draw_message.go), the line has actually been visible for a moment.
	// From there this mirrors the isAuto branch just above, only much
	// shorter: autoStartT already tracks "when did isWait last become
	// true", so skipWaitMs reuses it rather than needing its own clock.
	// skipEffective(), not skipActive() directly, so that unreadSkipEnabled
	// (既読SKIP / [unreadskip_config mode="read_only"]) can hold an unread
	// line at normal pace requiring a real click, same as skip being off.
	if skipEffective() && skipShouldAdvance(isWait, t, autoStartT, skipWaitMs) {
		oldTick = tick
	}
	hitLinks(r)
	hitGLinks(r)
	hitButtons(r)
	clearLinksOnJump()
	screenW, screenH := r.manager.Config.ScreenWidth, r.manager.Config.ScreenHeight
	switch {
	case activeDialog != nil:
		handleDialogClick(screenW, screenH)
		resolveButtonDialog(r)
	case slotPickerActive != slotPickerNone:
		r.handleSlotPickerClick()
	case menuOpen:
		r.handleQuickMenuClick(screenW, screenH)
	case backlogViewing:
		r.handleBacklogClick()
	default:
		r.handleMenuButtonClick()
		r.handleOperationRowClick()
		r.handleTextSpeedIndicatorClick()
	}
	if wasModalActive || anyModalActive() {
		return
	}
	stepAudioFades()
	stepSpeechSynthesis()
	stepAnimations()
	stepMovie()
	r.revealActiveLine()
	for i := 0; i < 1000; i++ {
		if !co.Next() {
			break
		}
		tick++
	}
}

// revealActiveLine decides whether the currently-revealing line (r.line)
// has finished its glyph-by-glyph reveal and, if so, sets isWait — the one
// flag that unblocks a bare TextObject's y.Until(false, func() bool {
// return isWait }) (execItem, macro.go), and everything downstream of that
// ([p]'s isTextEndedOrJumped, [l]'s isClicked, AUTO/SKIP in this very
// function above).
//
// This decision used to be made *only* inside drawMessageWindow/
// drawMessageHorizontal/drawMessageVertical (draw_message.go) — meaning
// whether the story could advance at all silently depended on Draw()
// having actually run this frame. ebitengine calls Update immediately
// followed by Draw on every platform this project ships to under normal
// conditions, so that dependency was invisible in practice — but Draw is
// skipped whenever the window isn't actually being presented (minimized,
// occluded on some platforms) while Update keeps running regardless.
// In that window the whole story — AUTO/SKIP included, both of which read
// isWait a few lines up in this same function — used to freeze completely
// until the window became visible again. It's also why no test in this
// package could ever exercise real glyph-by-glyph reveal: nothing in the
// suite calls Draw.
//
// Called right before the coroutine is stepped, matching drawMessageWindow's
// own timing: Draw normally runs immediately after Update, so a line
// finishing its reveal becomes visible to the coroutine on the very next
// frame's co.Next() call either way, whether this function or Draw is what
// actually flips isWait. drawMessageWindow's own isWait/isTextEnd
// bookkeeping is deliberately left in place, not removed or rerouted
// through this function — isTextEnd and textEndX/textEndY (the "waiting"
// mark's screen position, tags_sysdesign.go) are purely a draw-time
// concern with no bearing on whether the coroutine can proceed, and folding
// them in here would be a much larger rework (untangling glyph-position
// tracking from actual glyph drawing) for no correctness gain — this fix's
// scope is deliberately just the one flag that was actually able to freeze
// the game.
func (r *Renderer) revealActiveLine() {
	if isWait || textPosition == nil || !textPosition.Visible {
		return
	}
	total := activeLineGlyphCount(r, textPosition.Vertical)
	if total == 0 {
		return
	}
	count := math.MaxInt32
	if !textNoWait {
		count = (t - textStartT) / ticksPerChar()
	}
	if count >= total {
		isWait = true
	}
}

// skipShouldAdvance is Update()'s skip-mode pacing decision, split out into
// a pure function so it's testable without ebiten's real input/tick state —
// no precedent in this package for faking those directly (see e.g.
// dispatchOperationRowClickAt's own doc comment, tags_oprow.go). t and
// autoStartT are both in the same tick unit Update() already tracks them
// in; autoStartT is the tick isWait most recently became true (reset to t
// on every frame isWait is false).
func skipShouldAdvance(isWait bool, t, autoStartT, skipWaitMs int) bool {
	return isWait && t-autoStartT >= skipWaitMs*ebiten.TPS()/1000
}

// renderBuffer is where drawScene actually renders each frame; Draw then
// composites it onto the real screen with camera pan/zoom, screen-shake
// offset, and an optional color filter applied — see [camera]/[quake]/
// [filter] in tags_effects.go. mask (drawn separately, on top, unaffected
// by shake/camera) is [mask]'s overlay.
var renderBuffer *ebiten.Image

func (r *Renderer) Draw(screen *ebiten.Image) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if renderBuffer == nil || renderBuffer.Bounds().Dx() != w || renderBuffer.Bounds().Dy() != h {
		renderBuffer = ebiten.NewImage(w, h)
	}
	renderBuffer.Clear()
	r.drawScene(renderBuffer)

	var geoM ebiten.GeoM
	cx, cy := float64(w)/2, float64(h)/2
	geoM.Translate(-cx, -cy)
	geoM.Scale(camera.Scale, camera.Scale)
	geoM.Translate(cx, cy)
	geoM.Translate(camera.X, camera.Y)
	sx, sy := currentShakeOffset()
	geoM.Translate(sx, sy)

	if activeFilter != nil && activeFilter.needsShader() {
		shaderOp := &ebiten.DrawRectShaderOptions{GeoM: geoM}
		shaderOp.ColorScale.ScaleWithColor(activeFilter.Tint)
		shaderOp.ColorScale.ScaleAlpha(activeFilter.Alpha)
		shaderOp.Images[0] = renderBuffer
		hueRad := float64(activeFilter.HueDeg) * math.Pi / 180
		shaderOp.Uniforms = map[string]any{
			"Grayscale":  activeFilter.Grayscale,
			"Sepia":      activeFilter.Sepia,
			"Saturate":   activeFilter.Saturate,
			"HueSin":     float32(math.Sin(hueRad)),
			"HueCos":     float32(math.Cos(hueRad)),
			"Invert":     activeFilter.Invert,
			"Brightness": activeFilter.Brightness,
			"Contrast":   activeFilter.Contrast,
			"BlurRadius": activeFilter.Blur,
		}
		screen.DrawRectShader(w, h, compiledFilterShader(), shaderOp)
	} else {
		op := &ebiten.DrawImageOptions{GeoM: geoM}
		if activeFilter != nil {
			op.ColorScale.ScaleWithColor(activeFilter.Tint)
			op.ColorScale.ScaleAlpha(activeFilter.Alpha)
		}
		screen.DrawImage(renderBuffer, op)
	}

	if activeMask != nil {
		maskOp := &ebiten.DrawImageOptions{}
		maskOp.ColorScale.ScaleAlpha(activeMask.Opacity)
		screen.DrawImage(activeMask.Image, maskOp)
	}
	// Drawn directly to screen, not renderBuffer: the cursor must track the
	// real mouse position 1:1, unaffected by camera pan/zoom or screen shake.
	drawCursor(screen)
}

// applyTextStyle resolves the size/color a text segment should draw with
// and applies both: r.fontFace.Size as a side effect (glyph measurement
// reads it directly) and tOp.ColorScale. Precedence is the segment's own
// v.TextStyle first — the [font]/[deffont] state active when this segment
// was *created* (appendRubyText and applyFontAttrs's own marker segment
// both snapshot it at creation time, macro.go/tags_text.go) — falling back
// to the package-level textStyle (today's live setting) only when a caller
// deliberately passes no style of its own (draw_link.go's [link] rows,
// which always want whatever's current rather than a fixed snapshot), then
// the beforeTextSize/white default.
//
// v.TextStyle must win over the live textStyle, not the other way around:
// [l] (unlike [p]) doesn't clear the message window, so several segments
// created under different [font]/[resetfont] states can be on screen at
// once — an earlier segment must keep rendering the style active when
// *it* was created even after a later [font] call on the same page moves
// textStyle on. Preferring the live textStyle instead (this function's
// prior behavior) reskinned every such earlier segment to match whatever
// [font] ran last. applyFontAttrs' own doc comment covers the matching
// write-side half (copies rather than mutates textStyle in place, so an
// earlier segment's already-captured TextStyle pointer isn't silently
// rewritten out from under it too).
//
// r.fontFace.Size is set unconditionally on every branch below (falling
// back to beforeTextSize whenever the resolved style leaves Size
// unspecified/zero) — leaving it untouched when a style's Size happened to
// be 0 used to let whatever size the *previous* segment last set leak into
// this one (e.g. [font size=40]...[resetfont][font color=pink] kept
// drawing the pink text at size 40, since color=pink's own style never set
// a Size of its own to overwrite it). Callers must call this — which also
// means r.fontFace.Size is now correct — *before* measuring/laying out the
// segment's glyphs (text.Measure/text.AppendGlyphs in
// drawMessageHorizontal), not after: measuring with the *previous*
// segment's leftover size instead of this one's is what produced both the
// reported symptoms (overlapping/too-tight spacing right after a size
// change, and ruby text centered over the wrong width).
func applyTextStyle(r *Renderer, tOp *text.DrawOptions, v Text) {
	style := v.TextStyle
	if style == nil {
		style = textStyle
	}
	switch {
	case style != nil:
		if style.Size != 0 {
			r.fontFace.Size = float64(style.Size)
		} else {
			r.fontFace.Size = beforeTextSize
		}
		if style.Color != nil {
			tOp.ColorScale.ScaleWithColor(style.Color)
		} else {
			tOp.ColorScale.ScaleWithColor(color.White)
		}
	default:
		r.fontFace.Size = beforeTextSize
		tOp.ColorScale.ScaleWithColor(color.White)
	}
}

func (r *Renderer) drawScene(buf *ebiten.Image) {
	drawBackground(buf)
	drawCharacters(buf)
	applyFukiPosition()
	drawMessageWindow(r, buf)
	drawOperationRow(r, buf)
	drawLinks(r, buf)
	drawChoiceDimOverlay(r, buf)
	drawGLinks(r, buf)
	drawButtons(buf)
	drawImages(buf)
	// Drawn after buttons/images (not from inside drawMessageWindow, where
	// this used to live): a [ptext] area is independent of the message
	// window (textPosition.Visible gated drawMessageWindow's whole body,
	// silently hiding every ptext whenever no message box was on screen —
	// config.ks's full-screen settings redesign has no message window at
	// all) and toggle-style controls (config.ks's スキップ対象/画面表示 rows)
	// need their option labels drawn on top of the button graphic beneath
	// them.
	drawPTexts(r, buf)
	//mx, my := ebiten.CursorPosition()
	//ebitenutil.DebugPrint(buf, fmt.Sprintf("t:%+v bgTick:%+v mouseX:%+v mouseY:%+v", t, bgTick, mx, my))
	drawMenuButton(r, buf)
	drawEditBox(r, buf)
	// Fullscreen video, drawn last among ordinary scene content so it
	// covers everything drawn above (message window, buttons, ptexts...)
	// — see drawMovie's own doc comment (tags_movie.go) for why it still
	// sits before captureSnapshot/drawModal.
	drawMovie(buf)
	// Save slot thumbnails (see captureSnapshot in save_thumbnail.go) are kept
	// fresh here, every frame, specifically *before* drawModal — buf has
	// the full scene at this point but none of any modal overlay's own
	// drawing yet, regardless of which overlay (if any) is about to be
	// added. Capturing at openSlotPicker/saveSlot time instead (an earlier
	// version of this code did) was too late whenever a save was reached
	// through another modal first (e.g. the quick menu's own SAVE item):
	// renderBuffer by then already had *that* modal's last several frames
	// baked in, so the thumbnail showed the quick menu instead of the
	// scene underneath it.
	captureSnapshot(buf)
	drawModal(r, buf)
}
