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
	"github.com/hajimehoshi/ebiten/v2/audio"
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

var (
	isFirst                                      bool
	isWait                                       bool
	loop                                         func(y coro.Yield)
	isClicked, isTextEnded                       func() bool
	t, tick, oldTick, bgTick, charaTick, bgmTick int
	charas                                       map[string]*kag3.Character
	viewCharas                                   []*kag3.CharaShow
	bg                                           *kag3.Background
	// bg2/bg2Tick drive a second background layer composited on top of bg
	// (see [bg2] in tags_background.go), for things like weather overlays.
	bg2                                           *kag3.Background
	bg2Tick                                       int
	// backImgs/backPtexts are a simplified fore/back "page" buffer:
	// [backlay] snapshots imgs/ptexts into them, [trans] swaps them in.
	backImgs                                      []*kag3.Image
	backPtexts                                    map[string]*kag3.PText
	textPosition                                 *kag3.TextPosition
	textStyle                                    *kag3.TextStyle
	beforeTextSize                               float64
	// ptexts holds every named [ptext] area, keyed by its "name"
	// attribute — ptext is general-purpose text placement, not just the
	// character name-plate. Which one (if any) doubles as the name-plate
	// is set by [chara_config ptext="..."] into charaNamePText.
	ptexts                                       map[string]*kag3.PText
	charaNamePText                               string
	textGlyphs                                   []text.Glyph
	charaName                                    string
	buttons                                      []*kag3.Button
	imgs                                         []*kag3.Image
	glinks                                       []*kag3.GLink
	links                                        []*kag3.Link
	isJump                                       bool
	isJumped                                     func() bool
	jumpIndex                                    int
	// screenChanged marks that whatever is about to consume isJump represents
	// a real screen change (a different storage loaded, or goToTitle/save-load
	// tearing down the previous screen's state) rather than a same-storage
	// label jump ([link]/[glink]/[button target=] to a label in the same
	// file). Set at the point loadScript/goToTitle/applySaveData actually
	// changes screens, read (and cleared) whenever the isJump handling in
	// Update() runs — which can be a later frame than where it was set, e.g.
	// a title confirm dialog resolves and returns early the same frame
	// (anyModalActive), so isJump isn't processed until the next Update().
	// Without this, clearNonFixButtons() had to run unconditionally on every
	// isJump, which wiped scene1.ks's role_button set (registered without
	// fix="true", matching the real bundled sample) on every in-scene
	// [link]/[glink] click even though nothing about the screen changed.
	screenChanged bool
	audioContext                                 *audio.Context
	layopt                                       *kag3.LayOpt
	isTextEnd                                    bool
	textStartT                                   int
	prevLine                                     int
	pendingRuby                                  string
	isSkip                                       bool
	isAuto                                       bool
	autoStartT                                   int
	co                                           *coro.Coro
)

func init() {
	charas = make(map[string]*kag3.Character)
	ptexts = make(map[string]*kag3.PText)
	textPosition = &kag3.TextPosition{}
	audioContext = audio.NewContext(44100)
	layopt = &kag3.LayOpt{}
	bg = &kag3.Background{
		Time:   3000,
		IsWait: true,
		Method: "crossfade",
	}
	bg2 = &kag3.Background{
		Time:   3000,
		IsWait: false,
		Method: "crossfade",
	}
	isClicked = func() bool {
		return oldTick+3 >= tick
	}
	isTextEnded = func() bool {
		return oldTick+3 >= tick && isWait
	}
	isJumped = func() bool {
		return isJump
	}
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
	for i, link := range links {
		mX, mY := ebiten.CursorPosition()
		for j, t := range link.Texts {
			x, y := textPosition.Left, textPosition.Top
			w, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + textPosition.MarginLeft
			marginTop := y + textPosition.MarginTop + int(h)*(i+j)
			if isColision(mX, mY, marginLeft, marginTop, int(w), int(h)) {
				hoveringClickable = true
				//fmt.Println("isCollsion", mX, mY)
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					if link.Storage != "" {
						screenChanged = true
						r.loadScript(link.Storage)
					}
					if v, ok := r.labels[link.Target[1:]]; ok {
						fmt.Printf("click label:%+v\n", v)
						jumpIndex = v.Index
						isJump = true
					}
				}
			}
		}
	}
	for _, glink := range glinks {
		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, glink.X, glink.Y, glink.Width, glink.Height) {
			hoveringClickable = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if glink.Storage != "" {
					screenChanged = true
					r.loadScript(glink.Storage)
				}
				if v, ok := r.labels[glink.Target]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
				if v, ok := r.labels[glink.Target[1:]]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
			}
		}
	}
	for _, button := range buttons {
		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, button.X, button.Y, button.Width, button.Height) {
			hoveringClickable = true
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				fmt.Printf("click button:%+v\n", button)
				fmt.Printf("labels:%+v\n", r.labels)
				// Must run before Storage/Role/Target below: config.ks's
				// volume buttons set tf.current_bgm_vol etc. via exp=,
				// which *vol_bgm_change (jumped to next) reads.
				r.vm.EvalButtonExp(button.PreExp, button.Exp)
				if button.Storage != "" {
					fmt.Printf("button.Storage:%+v\n", button.Storage)
					// role="sleepgame" must record its return address
					// (see [awakegame]/handleAwakeGame in tags_system.go)
					// against the *old* storage/position, before loadScript
					// below overwrites r.currentStorage. sleepStack, not
					// callStack — see the Renderer.sleepStack doc comment.
					if button.Role == "sleepgame" {
						r.sleepStack = append(r.sleepStack, sleepFrame{
							Storage:      r.currentStorage,
							Index:        currentScriptIndex,
							Buttons:      append([]*kag3.Button(nil), buttons...),
							Bg:           *bg,
							TextPosition: *textPosition,
						})
					}
					screenChanged = true
					r.loadScript(button.Storage)
					if button.Target == "" {
						jumpIndex = 0
						isJump = true
					}
				}
				if button.Role != "" {
					switch button.Role {
					case "save":
						// Per tyrano.jp/tag's [button] reference, role="save"
						// opens the save-slot screen rather than acting on a
						// fixed slot directly — that's what quicksave is
						// for. Reuses Phase 9's slot picker (tags_uiscreens.go).
						openSlotPicker(slotPickerSave)
					case "load":
						openSlotPicker(slotPickerLoad)
					case "quicksave":
						if err := r.saveSlot(quickSaveSlot); err != nil {
							fmt.Printf("quicksave failed: %v\n", err)
						}
					case "quickload":
						if err := r.loadSlot(quickSaveSlot); err != nil {
							fmt.Printf("quickload failed: %v\n", err)
						}
					case "backlog":
						backlogViewing = !backlogViewing
						if backlogViewing {
							backlogOpenedFrame = t
						}
					case "menu":
						menuOpen = !menuOpen
						if menuOpen {
							menuOpenedFrame = t
						}
					case "fullscreen":
						fmt.Printf("button.Role:%s\n", button.Role)
						ebiten.SetFullscreen(!ebiten.IsFullscreen())
					case "title":
						confirmGoToTitle(r)
					case "skip":
						isSkip = !isSkip
						if isSkip {
							isAuto = false
						}
					case "auto":
						isAuto = !isAuto
						if isAuto {
							isSkip = false
							autoStartT = t
						}
					case "window":
						textPosition.Visible = !textPosition.Visible
					case "sleepgame":
						// storage load + return-frame push already handled
						// above, before the switch.
					}
				}
				if button.Target != "" {
					r.buttonTargetJump(button.Target)
				}
			}
		}
	}
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

// goToTitle resets session state and jumps to title.ks, shared by button
// role="title" and the quick-menu's title item (see tags_save.go).
func (r *Renderer) goToTitle() {
	screenChanged = true
	r.loadScript("title.ks")
	r.texts = make(map[int][]Text)
	r.callStack = nil
	viewCharas = nil
	charaName = ""
	pendingRuby = ""
	// title.ks itself never touches these — real Tyrano's own title screen
	// only ever gets shown once, right after the boot iscript that hides
	// them initially. Since goToTitle can now re-enter title.ks at any
	// point in the middle of a playthrough (role="title", the quick menu's
	// "BACK TO TITLE"), gameplay UI left over from wherever we were has to
	// be reset explicitly here, or it bleeds through — not just onto the
	// title screen itself, but into whatever scenario starts next. [position]
	// (r.position in this file) only ever *merges* the attributes a given
	// tag call specifies, so a field a later [position] call never touches
	// again (most notably frame=, but also color/margins/vertical/...) stays
	// whatever the *previous* playthrough last set it to: reported as a
	// custom end-of-story message-window frame (scene1.ks's
	// [position frame="frame.png" ...] near the end) still showing behind
	// scene1.ks's very first line after choosing "はじめから" a second time,
	// since that early [position] call never specifies frame= to clear it.
	// A fresh struct matches exactly what NewRenderer starts a brand-new
	// process with. textStyle/defaultTextStyle ([font]/[deffont]) are the
	// same kind of never-explicitly-cleared global and get the same
	// treatment, for the same reason (scene1.ks's end-of-story
	// [deffont color="0x454D51"] otherwise recolors the next playthrough's
	// very first lines too).
	textPosition = &kag3.TextPosition{}
	textStyle = nil
	defaultTextStyle = nil
	menuButtonVisible = false
	closeAllModals()
	// true, not false: the tag coroutine may currently be blocked inside a
	// TextObject's y.Until(false, func() bool { return isWait }) — see
	// execItem in macro.go — waiting on this exact flag, which is
	// completely independent of isJump/[s]'s isJumped. Leaving it false
	// here (the old behavior) meant that if the source screen happened to
	// be mid-dialogue rather than resting at [s], the coroutine stayed
	// stuck there forever: isJump never gets a chance to be read until
	// whatever currently-blocked handler's own predicate resolves, and
	// isWait=false never does on its own. Setting it true releases that
	// wait immediately (harmlessly — the destination's own first text line
	// resets isWait=false again the moment it actually starts revealing).
	isWait = true
	isSkip = false
	isAuto = false
	jumpIndex = 0
	isJump = true
}

// confirmGoToTitle opens real Tyrano's own "タイトルに戻ります。よろしい
// ですか？" confirmation before actually calling goToTitle — role="title"
// and the quick-menu's "BACK TO TITLE" item both go through this instead of
// calling goToTitle directly, so a stray click can't discard the player's
// place in the story.
func confirmGoToTitle(r *Renderer) {
	activeDialog = &dialogState{
		Text:      "タイトルに戻ります。よろしいですか？",
		OKLabel:   dialogOKLabel,
		NGLabel:   dialogNGLabel,
		OnConfirm: func(r *Renderer) { r.goToTitle() },
	}
}

// resolveButtonDialog finishes a button-triggered confirm dialog (one with
// OnConfirm set — see confirmGoToTitle) once the user has picked OK or NG:
// runs OnConfirm on OK, then clears activeDialog either way. A [dialog]
// *tag*'s dialog never sets OnConfirm and resolves itself inside
// handleDialog via its own y.Until, so this is a no-op for those and safe
// to call unconditionally every frame activeDialog is non-nil.
func resolveButtonDialog(r *Renderer) {
	if activeDialog == nil || activeDialog.Result == 0 || activeDialog.OnConfirm == nil {
		return
	}
	if activeDialog.Result == 1 {
		activeDialog.OnConfirm(r)
	}
	activeDialog = nil
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
	glinks = nil
	links = nil
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

	op := &ebiten.DrawImageOptions{}
	cx, cy := float64(w)/2, float64(h)/2
	op.GeoM.Translate(-cx, -cy)
	op.GeoM.Scale(camera.Scale, camera.Scale)
	op.GeoM.Translate(cx, cy)
	op.GeoM.Translate(camera.X, camera.Y)
	sx, sy := currentShakeOffset()
	op.GeoM.Translate(sx, sy)
	if activeFilter != nil {
		op.ColorScale.ScaleWithColor(activeFilter.Tint)
		op.ColorScale.ScaleAlpha(activeFilter.Alpha)
	}
	screen.DrawImage(renderBuffer, op)

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
func applyTextStyle(r *Renderer, tOp *text.DrawOptions, v Text) {
	if textStyle != nil {
		if textStyle.Size != 0 && float64(textStyle.Size) != beforeTextSize {
			r.fontFace.Size = float64(textStyle.Size)
		}
		if textStyle.Color != nil {
			tOp.ColorScale.ScaleWithColor(textStyle.Color)
		} else {
			tOp.ColorScale.ScaleWithColor(color.White)
		}
	} else if v.TextStyle != nil {
		if v.TextStyle.Size != 0 && float64(v.TextStyle.Size) != beforeTextSize {
			r.fontFace.Size = float64(v.TextStyle.Size)
		}
		if v.TextStyle.Color != nil {
			tOp.ColorScale.ScaleWithColor(v.TextStyle.Color)
		} else {
			tOp.ColorScale.ScaleWithColor(color.White)
		}
	} else {
		r.fontFace.Size = beforeTextSize
		tOp.ColorScale.ScaleWithColor(color.White)
	}
}

func (r *Renderer) drawScene(buf *ebiten.Image) {
	drawBackground(buf)
	drawCharacters(buf)
	applyFukiPosition()
	if textPosition != nil && textPosition.Visible {
		op := &ebiten.DrawImageOptions{}
		x, y := float64(textPosition.Left), float64(textPosition.Top)
		op.GeoM.Translate(x, y)
		if textPosition.FrameImage != nil {
			op.ColorScale.SetA(float32(textPosition.Opacity))
			buf.DrawImage(textPosition.FrameImage, op)
		} else {
			if textPosition.FilterColor != nil {
				textPosition.BackImage.Fill(*textPosition.FilterColor)
			} else {
				textPosition.BackImage.Fill(color.RGBA{0, 0, 0, 128})
			}
			buf.DrawImage(textPosition.BackImage, op)
		}
		drawPTexts(buf, r.nameFontFace)

		marginLeft := x + float64(textPosition.MarginLeft)
		marginTop := y + float64(textPosition.MarginTop)
		count := math.MaxInt32
		if !textNoWait {
			count = (t - textStartT) / ticksPerChar()
		}
		isTextEnd = false
		lineNums := make([]int, 0, len(r.texts))
		for k := range r.texts {
			lineNums = append(lineNums, k)
		}
		slices.Sort(lineNums)
		if textPosition.Vertical {
			type runeEntry struct {
				ch    rune
				style *kag3.TextStyle
			}
			charSize := r.verticalFontFace.Size
			rightEdge := x + float64(textPosition.Width) - float64(textPosition.MarginRight)
			availableHeight := float64(textPosition.Height) - float64(textPosition.MarginTop) - float64(textPosition.MarginBottom)
			charsPerCol := int(availableHeight / charSize)
			if charsPerCol <= 0 {
				charsPerCol = 1
			}
			colIndex := 0
			for _, lineNum := range lineNums {
				segs := r.texts[lineNum]
				var runes []runeEntry
				for _, v := range segs {
					if len(v.Text) == 0 {
						continue
					}
					for _, ch := range v.Text {
						runes = append(runes, runeEntry{ch, v.TextStyle})
					}
				}
				if len(runes) == 0 {
					continue
				}
				showCount := len(runes)
				if lineNum == r.line {
					if !isWait {
						if count >= len(runes) {
							isWait = true
							isTextEnd = true
						} else {
							showCount = count
						}
					} else {
						isTextEnd = true
					}
				}
				for i, rs := range runes[:showCount] {
					col := colIndex + i/charsPerCol
					row := i % charsPerCol
					colX := rightEdge - float64(col+1)*charSize
					tOp := &text.DrawOptions{}
					tOp.GeoM.Translate(colX, marginTop+float64(row)*charSize)
					applyTextStyle(r, tOp, Text{TextStyle: rs.style})
					text.Draw(buf, string(rs.ch), r.verticalFontFace, tOp)
				}
				colIndex += (len(runes) + charsPerCol - 1) / charsPerCol
			}
		} else {
			rowY := 0.0
			for _, lineNum := range lineNums {
				segs := r.texts[lineNum]
				hasText := false
				for _, v := range segs {
					if len(v.Text) > 0 {
						hasText = true
						break
					}
				}
				if !hasText {
					continue
				}
				rubyLineHeight := 0.0
				for _, v := range segs {
					if v.Ruby != "" {
						rubyLineHeight = beforeTextSize * 0.5
						break
					}
				}
				if lineNum == r.line {
					totalGlyphs := 0
					for _, v := range segs {
						if len(v.Text) == 0 {
							continue
						}
						tOp2 := &text.DrawOptions{}
						tOp2.LineSpacing = r.fontFace.Size
						g := text.AppendGlyphs(nil, v.Text, r.fontFace, &tOp2.LayoutOptions)
						totalGlyphs += len(g)
					}
					if totalGlyphs > 0 {
						charsToShow := totalGlyphs
						if !isWait {
							if count >= totalGlyphs {
								isWait = true
								isTextEnd = true
							} else {
								charsToShow = count
							}
						} else {
							isTextEnd = true
						}
						segOffset := 0
						xOffset := 0.0
						for _, v := range segs {
							if len(v.Text) == 0 {
								continue
							}
							tOp := &text.DrawOptions{}
							tOp.LineSpacing = r.fontFace.Size
							vText := v.Text
							w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
							maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
							if w > maxWidth {
								rn := []rune(vText)
								key := len(rn) - 1
								for idx := key; idx >= 0; idx-- {
									ww, _ := text.Measure(string(rn[:idx]), r.fontFace, tOp.LineSpacing)
									if ww <= maxWidth {
										rn = append(rn[:idx+1], rn[idx:]...)
										rn[idx] = []rune("\n")[0]
										break
									}
								}
								vText = string(rn)
								w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
							}
							glyph := text.AppendGlyphs(nil, vText, r.fontFace, &tOp.LayoutOptions)
							segLen := len(glyph)
							applyTextStyle(r, tOp, v)
							segShowCount := charsToShow - segOffset
							if segShowCount >= segLen {
								tOp.GeoM.Reset()
								tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
								text.Draw(buf, vText, r.fontFace, tOp)
								if v.Ruby != "" {
									rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: beforeTextSize * 0.5, Language: r.fontFace.Language}
									rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
									rubyOp := &text.DrawOptions{}
									rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
									rubyOp.ColorScale.ScaleWithColor(color.White)
									text.Draw(buf, v.Ruby, rubyFace, rubyOp)
								}
							} else if segShowCount > 0 {
								for _, g := range glyph[:segShowCount] {
									if g.Image == nil {
										continue
									}
									tOp.GeoM.Reset()
									tOp.GeoM.Translate(marginLeft+xOffset+g.X, marginTop+rowY+rubyLineHeight+g.Y)
									buf.DrawImage(g.Image, &tOp.DrawImageOptions)
								}
							}
							xOffset += w
							segOffset += segLen
						}
					}
				} else {
					xOffset := 0.0
					for _, v := range segs {
						if len(v.Text) == 0 {
							continue
						}
						tOp := &text.DrawOptions{}
						tOp.LineSpacing = r.fontFace.Size
						vText := v.Text
						w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
						maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
						if w > maxWidth {
							rn := []rune(vText)
							key := len(rn) - 1
							for idx := key; idx >= 0; idx-- {
								ww, _ := text.Measure(string(rn[:idx]), r.fontFace, tOp.LineSpacing)
								if ww <= maxWidth {
									rn = append(rn[:idx+1], rn[idx:]...)
									rn[idx] = []rune("\n")[0]
									break
								}
							}
							vText = string(rn)
							w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
						}
						// applyTextStyle, not a bare color.White default: an
						// already fully-revealed line must keep using
						// whatever [font]/[deffont] color is *currently* in
						// effect, the same as the active line resolves it —
						// before this fix it fell straight to plain white
						// the instant it stopped being the active line,
						// which on a light/white message-box design (e.g.
						// scene1.ks's [deffont color="0x454D51"] custom
						// window) made every earlier line on the same page
						// effectively invisible against the background as
						// soon as the next line started revealing.
						applyTextStyle(r, tOp, v)
						tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
						text.Draw(buf, vText, r.fontFace, tOp)
						if v.Ruby != "" {
							rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: beforeTextSize * 0.5, Language: r.fontFace.Language}
							rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
							rubyOp := &text.DrawOptions{}
							rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
							rubyOp.ColorScale.ScaleWithColor(color.White)
							text.Draw(buf, v.Ruby, rubyFace, rubyOp)
						}
						xOffset += w
					}
				}
				rowY += beforeTextSize + rubyLineHeight
			}
		}
		textGlyphs = []text.Glyph{}
		drawGlyph(buf)
	}
	for i, link := range links {
		for j, t := range link.Texts {
			linkOp := &text.DrawOptions{}
			x, y := float64(textPosition.Left), float64(textPosition.Top)
			_, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + float64(textPosition.MarginLeft)
			marginTop := y + float64(textPosition.MarginTop) + h*float64(i+j)
			linkOp.GeoM.Translate(marginLeft, marginTop)
			applyTextStyle(r, linkOp, Text{})
			if !isJump {
				text.Draw(buf, t.Val, r.fontFace, linkOp)
			}
		}
	}
	for _, glink := range glinks {
		backgroundOp := &ebiten.DrawImageOptions{}
		backgroundOp.GeoM.Translate(float64(glink.X), float64(glink.Y))
		img := ebiten.NewImage(glink.Width, glink.Height)
		img.Fill(glink.Color)
		buf.DrawImage(img, backgroundOp)
		glinkOp := &text.DrawOptions{}
		w, h := text.Measure(glink.Text, r.fontFace, 0)
		glinkOp.GeoM.Translate(float64(glink.Width/2+glink.X)-(w/2), float64(glink.Height/2+glink.Y)-(h/2))
		text.Draw(buf, glink.Text, r.fontFace, glinkOp)
	}
	for _, button := range buttons {
		if button.Graphic == nil && button.EnterImg == nil {
			// An invisible hit zone (e.g. [clickable]) — nothing to draw.
			continue
		}
		buttonOp := &ebiten.DrawImageOptions{}
		buttonOp.GeoM.Translate(float64(button.X), float64(button.Y))

		mX, mY := ebiten.CursorPosition()
		// A button with no enterimg= (e.g. config.ks's volume/speed slider
		// buttons) just keeps showing its normal graphic on hover, rather
		// than crash trying to draw a nil hover image. Graphic itself can
		// also be nil (only enterimg= given, an unusual but not invalid
		// script) — draw whichever of the two applies is actually set.
		toDraw := button.Graphic
		if isColision(mX, mY, button.X, button.Y, button.Width, button.Height) && button.EnterImg != nil {
			toDraw = button.EnterImg
		}
		if toDraw != nil {
			buf.DrawImage(toDraw, buttonOp)
		}
	}
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
