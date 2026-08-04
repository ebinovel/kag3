package ebitengine

import (
	"fmt"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
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
type sleepFrame struct {
	Storage      string
	Index        int
	Buttons      []*kag3.Button
	Bg           kag3.Background
	TextPosition kag3.TextPosition
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

// StartAtLabel makes the script loop begin at the named label instead of
// index 0. It must be called between NewRenderer and the first Update():
// initScript's loop checks isJump unconditionally at the top of its very
// first iteration (see its own doc comment), so a pending jump set here
// beforehand is picked up before anything else executes.
//
// name may be given with or without its leading "*", matching how
// handleJump ([jump]'s own handler) looks labels up — r.labels is keyed
// on kag3.LabelInfo's bare name, but callers (like kag3-runner's -label
// flag) may pass either form.
//
// Returns false, doing nothing, if name isn't a known label — the caller
// is expected to report that rather than silently starting at the top of
// the script.
func (r *Renderer) StartAtLabel(name string) bool {
	v, ok := r.labels[name]
	if !ok {
		v, ok = r.labels[strings.TrimPrefix(name, "*")]
	}
	if !ok {
		return false
	}
	jumpIndex = v.Index
	isJump = true
	return true
}

func doNext() bool {
	return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsKeyJustPressed(ebiten.KeyEnter)
}

// loadScript loads a scenario file and re-points the renderer at it,
// keeping currentStorage in sync so [call]/[return] can record and resume
// call frames as plain (storage, index) pairs.
func (r *Renderer) loadScript(name string) error {
	if err := r.manager.LoadScript(name); err != nil {
		return err
	}
	r.labels = r.manager.Labels
	r.scripts = r.manager.Senario
	r.currentStorage = name
	return nil
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
	if isSkip {
		isWait = true
		oldTick = tick
	}
	if !isWait {
		autoStartT = t
	}
	if isAuto && isWait && t-autoStartT >= autoWaitMs*ebiten.TPS()/1000 {
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
	default:
		r.handleMenuButtonClick()
	}
	// Backlog has no per-item hit-test — any click dismisses it, except on
	// the very frame that opened it (that click is the role="backlog"
	// button press, or [showlog], already handled above/this frame).
	if backlogViewing && t != backlogOpenedFrame && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		backlogViewing = false
	}
	if wasModalActive || anyModalActive() {
		return
	}
	stepAudioFades()
	stepAnimations()
	for i := 0; i < 1000; i++ {
		if !co.Next() {
			break
		}
		tick++
	}
}

func (r *Renderer) initScript() {
	loop = func(y coro.Yield) {
		// Deliberately not a "for i := 0; i < len(r.scripts); i++" loop:
		// that shape checks the bound *before* the body's isJump check on
		// every iteration, including the very first one after i++. A jump
		// that lands while a *different* script (e.g. [call storage=...]
		// or goToTitle swapping r.scripts out mid-loop) had left i sitting
		// at a high index — say config.ks's [s] at index 63 — would then
		// have that stale i compared against the *new* r.scripts' length
		// (title.ks, much shorter) before isJump ever got a chance to
		// overwrite it, silently ending the whole loop (r.Done = true)
		// instead of jumping. Checking isJump first, unconditionally,
		// before the bound check fixes that: whatever i was doesn't matter
		// once a jump is pending.
		i := 0
		for {
			if isJump {
				i = jumpIndex
				isJump = false
			}
			if i >= len(r.scripts) {
				break
			}
			if err := r.execItem(y, r.scripts, &i, 0); err != nil {
				panic(err)
			}
			i++
			y()
		}
		r.Done = true
	}
}

// resolveFolderImage loads a [button]-style graphic against folder=,
// matching real Tyrano's convention that folder names a *subdirectory* of
// the images root (e.g. folder="bgimage" -> images/bgimage/…) rather than a
// separate top-level asset root. tyrano.ks's own bundled cg_image_button/
// replay_image_button macros also lean on a real-Tyrano-specific relative
// escape for their shared "no image" placeholder (folder="bgimage" plus a
// graphic of "../../tyrano/images/system/noimage.png") that plain
// fs.Sub(images, folder) can't resolve — Go's io/fs deliberately rejects any
// ".." path component. After path.Clean, if the joined path still tries to
// climb above the images root, this looks for a "system/" path component to
// redirect the remainder to r.fses["system/images"] (kag3's own equivalent
// of Tyrano's bundled engine-asset folder) — the one real asset this
// pattern needs, noimage.png, lives exactly there.
func resolveFolderImage(r *Renderer, folder, graphic string) (imgFS fs.FS, name string) {
	full := graphic
	if folder != "" && folder != "images" {
		full = path.Join(folder, graphic)
	}
	full = path.Clean(full)
	if full == ".." || strings.HasPrefix(full, "../") {
		if idx := strings.LastIndex(full, "system/"); idx >= 0 {
			return r.fses["system/images"], full[idx+len("system/"):]
		}
		full = strings.TrimPrefix(full, "../")
		for strings.HasPrefix(full, "../") {
			full = full[len("../"):]
		}
	}
	return r.fses["images"], full
}

// loadImage resolves folder/storage via resolveFolderImage and loads the
// result, folding the two-step "resolve fs.FS + path, then load" sequence
// that's repeated at every [button]/[image]/chara call site into one call.
func loadImage(r *Renderer, folder, storage string) (*ebiten.Image, error) {
	imgFS, name := resolveFolderImage(r, folder, storage)
	img, _, err := ebitenutil.NewImageFromFileSystem(imgFS, name)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// buttonTargetJump resolves a clicked button's target= against r.labels and,
// if found, jumps there — call-style, not a plain [jump]: real Tyrano's
// official config.ks (this repo's example/resources/senarios/config.ks) ends
// every target label (*vol_bgm_change etc.) with [return], expecting to land
// back exactly where the button was clicked. Pushing currentScriptIndex, not
// +1, matches role="sleepgame"'s push in Update() — both happen outside the
// coroutine, so [return]'s "-1 to compensate for the enclosing loop's
// increment" lands back on whatever tag is currently blocking (typically
// [s]), re-entering it cleanly.
func (r *Renderer) buttonTargetJump(target string) {
	v, ok := r.labels[target]
	if !ok {
		v, ok = r.labels[target[1:]]
	}
	if !ok {
		return
	}
	fmt.Printf("label:%+v\n", v)
	r.callStack = append(r.callStack, callFrame{
		Storage: r.currentStorage,
		Index:   currentScriptIndex,
	})
	jumpIndex = v.Index
	isJump = true
}

// clearLinksOnJump runs once per Update() frame, right before isJump is
// consumed by the coroutine (see initScript's loop): any pending link/glink
// choice list is always dropped (it belonged to whatever line just advanced
// past), but buttons are only swept by clearNonFixButtons when screenChanged
// says this jump is an actual screen change — a same-storage label jump
// ([link]/[glink]/[button target=] to a label in the current file) must
// leave persistent UI like scene1.ks's role_button set alone, matching real
// Tyrano: a jump within a scenario doesn't tear down the screen.
func clearLinksOnJump() {
	if !isJump {
		return
	}
	if preserveLinksOnJump {
		preserveLinksOnJump = false
	} else {
		glinks = nil
		links = nil
	}
	if screenChanged {
		clearNonFixButtons()
		screenChanged = false
	}
}

// clearNonFixButtons drops every button without Fix=true from the buttons
// list — the reaction to a screen-changing jump (see clearLinksOnJump).
// Fix=true buttons persist across jumps (that's what "fix" means, e.g.
// config.ks's entire button set, registered once at *config_page); only
// [clearfix] (tags_layer.go) removes those.
func clearNonFixButtons() {
	kept := buttons[:0]
	for _, b := range buttons {
		if b.Fix {
			kept = append(kept, b)
		}
	}
	buttons = kept
}

// setButtonImageByClass implements the one real effect of the $ shim's
// attr("src", ...) — see SetJQueryHooks/browserShimJS in vm.go. Every
// button whose Name (comma-separated, e.g. "bgmvol,bgmvol_10") contains
// selector (with its leading "." stripped) as a token gets its Graphic
// reloaded from path. Config.ks's own volume/speed/skip buttons are
// exactly this pattern: [button name="bgmvol,bgmvol_10" ...] plus an
// iscript that resets the whole "bgmvol" group to the off graphic, then
// sets just the "bgmvol_10" one to the on graphic. A path that fails to
// load is logged and skipped, not fatal — matching how a missing/renamed
// asset is handled elsewhere in this engine.
func (r *Renderer) setButtonImageByClass(selector, path string) {
	class := strings.TrimPrefix(selector, ".")
	if class == "" {
		return
	}
	var img *ebiten.Image
	for _, b := range buttons {
		matched := false
		for _, name := range strings.Split(b.Name, ",") {
			if strings.TrimSpace(name) == class {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if img == nil {
			loaded, err := loadImage(r, "", path)
			if err != nil {
				fmt.Printf("$(%q).attr(\"src\", %q): %v\n", selector, path, err)
				return
			}
			img = loaded
		}
		b.Graphic = img
	}
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
// reads it directly) and tOp.ColorScale. Precedence is the package-level
// textStyle (the currently active [font]/[deffont] setting) first, then the
// segment's own v.TextStyle (set by [font]'s own empty marker Text, see
// r.textStyle), then the beforeTextSize/white default. Used for both the
// currently-revealing line and every earlier line still on screen — they
// must resolve identically, since a line doesn't stop being e.g.
// [deffont color=...]-styled just because it's no longer the one being
// typed.
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
	switch {
	case textStyle != nil:
		if textStyle.Size != 0 {
			r.fontFace.Size = float64(textStyle.Size)
		} else {
			r.fontFace.Size = beforeTextSize
		}
		if textStyle.Color != nil {
			tOp.ColorScale.ScaleWithColor(textStyle.Color)
		} else {
			tOp.ColorScale.ScaleWithColor(color.White)
		}
	case v.TextStyle != nil:
		if v.TextStyle.Size != 0 {
			r.fontFace.Size = float64(v.TextStyle.Size)
		} else {
			r.fontFace.Size = beforeTextSize
		}
		if v.TextStyle.Color != nil {
			tOp.ColorScale.ScaleWithColor(v.TextStyle.Color)
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
	drawLinks(r, buf)
	drawGLinks(r, buf)
	drawButtons(buf)
	drawImages(buf)
	//mx, my := ebiten.CursorPosition()
	//ebitenutil.DebugPrint(buf, fmt.Sprintf("t:%+v bgTick:%+v mouseX:%+v mouseY:%+v", t, bgTick, mx, my))
	drawMenuButton(r, buf)
	drawEditBox(r, buf)
	// Save slot thumbnails (see captureSnapshot in tags_save.go) are kept
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

// drawPTexts renders every named [ptext] area. The one registered as the
// character name-plate via [chara_config ptext="..."] shows charaName;
// every other one shows its own literal .Text. Sorted by name for
// deterministic draw order.
func drawPTexts(screen *ebiten.Image, face *text.GoTextFace) {
	if len(ptexts) == 0 {
		return
	}
	names := make([]string, 0, len(ptexts))
	for name := range ptexts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		pt := ptexts[name]
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(pt.X), float64(pt.Y))
		if pt.Color != nil {
			op.ColorScale.ScaleWithColor(pt.Color)
		} else {
			op.ColorScale.ScaleWithColor(color.White)
		}
		text.Draw(screen, ptextContent(name), face, op)
	}
}

// ptextContent is the string a named ptext area should currently display:
// charaName for the one registered via [chara_config ptext=...], its own
// literal .Text otherwise. Split out from drawPTexts so the name-plate
// resolution logic is testable without an ebiten screen/font.
func ptextContent(name string) string {
	if name == charaNamePText {
		return charaName
	}
	if pt, ok := ptexts[name]; ok {
		return pt.Text
	}
	return ""
}

// parseColor accepts either a handful of named colors or a "0xRRGGBB" hex
// triplet (the two forms every bundled .ks color=/edge=/shadow=/
// border_color= attribute actually uses). The hex branch used to slice out
// only the first hex digit of each byte pair (e.g. "0x454D51" -> "4","4","5")
// and feed it to strconv.Atoi as a *decimal* number — "black" separately
// returned white (255,255,255) instead of black, and "white"/"pink" weren't
// recognized as named colors at all, falling into the same broken hex path
// and erroring (which panics the whole game, since any tag handler error
// propagates up through initScript's coroutine loop) or reading garbage
// runes past the string's end. Every one of these is actually used by the
// bundled example scripts (color="0x454D51"/"0xFAFAFA" for the custom
// message window, color="pink"/"white" elsewhere), so all were live bugs.
func parseColor(value string) (r, g, b int, err error) {
	switch value {
	case "black":
		return 0, 0, 0, nil
	case "white":
		return 255, 255, 255, nil
	case "red":
		return 255, 0, 0, nil
	case "blue":
		return 0, 0, 255, nil
	case "pink":
		return 255, 192, 203, nil
	}
	hex := strings.TrimPrefix(value, "0x")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("不正なカラーコードです: %s", value)
	}
	var v int64
	v, err = strconv.ParseInt(hex[0:2], 16, 32)
	if err != nil {
		return
	}
	r = int(v)
	v, err = strconv.ParseInt(hex[2:4], 16, 32)
	if err != nil {
		return
	}
	g = int(v)
	v, err = strconv.ParseInt(hex[4:6], 16, 32)
	if err != nil {
		return
	}
	b = int(v)
	return
}

func isColision(mX, mY, x, y, width, height int) bool {
	return mX >= x && mX <= x+width && mY >= y && mY <= y+height
}
