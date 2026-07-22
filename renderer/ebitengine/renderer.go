package ebitengine

import (
	"fmt"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/ebinovel/kag3/renderer/ebitengine/effects"
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
	manager        *kag3.Manager
	scripts        []any
	labels         map[string]kag3.LabelInfo
	fontFace       *text.GoTextFace
	nameFontFace   *text.GoTextFace
	fses           map[string]fs.FS
	texts          map[int][]Text
	line           int
	Done           bool
	vm             *VM
	currentStorage string
	callStack      []callFrame
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
		fses:           manager.FSes,
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
	if isJump {
		glinks = nil
		links = nil
		clearNonFixButtons()
	}
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
	// "BACK TO TITLE"), gameplay UI left over from wherever we were —
	// scene1.ks's @showmenubutton corner icon, the message window — has to
	// be hidden explicitly here instead, or it bleeds through on top of
	// the title screen.
	textPosition.Visible = false
	menuButtonVisible = false
	backlogViewing = false
	menuOpen = false
	slotPickerActive = slotPickerNone
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

func (r *Renderer) charaShow(object kag3.TagObject) (chara *kag3.CharaShow, err error) {
	chara = &kag3.CharaShow{}
	charaTick = t
	charaNew := true
	name := object.Pm["name"]
	if _, ok := charas[name]; !ok {
		return nil, fmt.Errorf("そのキャラクターは登録されてません name=%s", name)
	}
	for _, c := range viewCharas {
		if c.Name == name && c.IsRemove {
			c.IsRemove = false
			charaNew = false
			chara = c
		}
	}
	if charaNew {
		chara.Wait = true
		chara.Time = 1000
		chara.Opacity = 255
		chara.ScaleX = 1
		chara.ScaleY = 1
		for key, value := range object.Pm {
			switch key {
			case "name":
				chara.Name = value
			case "time":
				chara.Time, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "zindex":
				chara.Zindex, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "depth":
				chara.Depth = value
			case "page":
				chara.Page = value
			case "wait":
				chara.Wait, err = strconv.ParseBool(value)
				if err != nil {
					return
				}
			case "face":
				if v, ok := charas[name].Faces[value]; ok {
					chara.Face = v
				}
			case "storage":
				charaImage, _, err := ebitenutil.NewImageFromFileSystem(
					r.fses["images"],
					object.Pm["storage"],
				)
				if err != nil {
					return nil, err
				}
				charas[name].Image = charaImage
			case "refrect":
				chara.Reflect, err = strconv.ParseBool(value)
				if err != nil {
					return
				}
			case "width":
				chara.Width, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "height":
				chara.Height, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "left":
				chara.Left, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "top":
				chara.Top, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			}
		}
	}
	herfWidth := charas[name].Image.Bounds().Dx() / 2
	charaSpace := r.manager.Config.ScreenWidth / (len(viewCharas) + 2)
	currentLeft := charaSpace
	if chara.Left == 0 && chara.Top == 0 {
		chara.Left = charaSpace - herfWidth
		chara.Top = r.manager.Config.ScreenHeight - charas[name].Image.Bounds().Dy()
	}
	for _, c := range viewCharas {
		currentLeft += charaSpace
		// c ranges over every currently-shown character, not just the one
		// this call is about (already guarded at the top via name) — a
		// sibling could in principle be an unreconciled load-restored entry
		// (see reconcileViewCharas in tags_save.go), so re-check here too.
		sibling, ok := charas[c.Name]
		if !ok || sibling.Image == nil {
			continue
		}
		left := currentLeft - (sibling.Image.Bounds().Dx() / 2)
		c.NewLeft = left
		if charaNew {
			c.IsSlide = true
		}

	}
	if charaNew {
		viewCharas = append(viewCharas, chara)
	}
	fmt.Printf("viewCharas:%+v\n", chara)
	fmt.Printf("viewCharas:%+v\n", viewCharas)
	return
}

func (r *Renderer) position(tagObject kag3.TagObject) (err error) {
	for key, value := range tagObject.Pm {
		switch key {
		case "layer":
			textPosition.Layer = value
		case "page":
			textPosition.Page = value
		case "left":
			textPosition.Left, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "top":
			textPosition.Top, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "width":
			textPosition.Width, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "height":
			textPosition.Height, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "frame":
			textPosition.FrameImage, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses["images"],
				tagObject.Pm["frame"],
			)
			if err != nil {
				return
			}
			textPosition.FrameStorage = value
		case "color", "border_color":
			var r, g, b int
			r, g, b, err = parseColor(value)
			if key == "color" {
				textPosition.Color = color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			} else {
				textPosition.BorderColor = color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			}
		case "border_size":
			textPosition.BorderSize, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "opacity":
			textPosition.Opacity, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginl":
			textPosition.MarginLeft, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "margint":
			textPosition.MarginTop, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginr":
			textPosition.MarginRight, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginb":
			textPosition.MarginBottom, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginn":
			textPosition.MarginN, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "radius":
			textPosition.Radius, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "vertial", "vertical":
			textPosition.Vertical, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "visible":
			textPosition.Visible, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		}
	}
	if textPosition.Width != 0 && textPosition.Height != 0 {
		textPosition.BackImage = ebiten.NewImage(textPosition.Width, textPosition.Height)
	}
	return nil
}

func (r *Renderer) textStyle(tagObject kag3.TagObject) (err error) {
	if textStyle == nil {
		textStyle = &kag3.TextStyle{}
	}
	for key, value := range tagObject.Pm {
		switch key {
		case "size":
			textStyle.Size, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "color", "edge", "shadow":
			var r, g, b int
			r, g, b, err = parseColor(value)
			switch key {
			case "color":
				textStyle.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "edge":
				textStyle.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "shadow":
				textStyle.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			}
		case "bold":
			textStyle.IsBold = true
		case "itaric":
			textStyle.IsItaric = true
		}
	}
	r.texts[tagObject.Line] = append(r.texts[tagObject.Line], Text{TextStyle: textStyle})
	return nil
}

func (r *Renderer) button(object kag3.TagObject) (err error) {
	button := &kag3.Button{}
	for key, value := range object.Pm {
		switch key {
		case "graphic":
			folder := "images"
			if object.Pm["folder"] != "" {
				folder = object.Pm["folder"]
			}
			button.Graphic, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses[folder],
				object.Pm["graphic"],
			)
			if err != nil {
				return
			}
		case "storage":
			button.Storage = value
		case "target":
			button.Target = value
		case "name":
			button.Name = value
		case "x":
			button.X, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "y":
			button.Y, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "width":
			button.Width, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "height":
			button.Height, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "fix":
			button.Fix, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "role":
			button.Role = value
		case "hint":
			button.Hint = value
		case "clickse":
			button.ClickSE = value
		case "enterse":
			button.EnterSE = value
		case "leavese":
			button.LeaveSE = value
		case "activeimg":
			button.ActiveImg = value
		case "clickimg":
			button.ClickImg = value
		case "enterimg":
			folder := "images"
			if object.Pm["folder"] != "" {
				folder = object.Pm["folder"]
			}
			button.EnterImg, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses[folder],
				object.Pm["enterimg"],
			)
			if err != nil {
				return
			}
		case "autoimg":
			button.AutoImg = value
		case "skipimg":
			button.SkipImg = value
		case "visible":
			button.Visible, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "auto_next":
			button.AutoNext, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "savesnap":
			button.SaveSnap, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "keyforcus":
			button.KeyForcus, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "exp":
			button.Exp = value
		case "preexp":
			button.PreExp = value
		}
	}
	if button.Width == 0 && button.Height == 0 {
		button.Width, button.Height = button.Graphic.Bounds().Dx(), button.Graphic.Bounds().Dy()
	}
	fmt.Printf("button: %+v\n", button)
	buttons = append(buttons, button)
	return nil
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

// clearNonFixButtons drops every button without Fix=true from the buttons
// list — the reaction to any jump (see Update()). Fix=true buttons persist
// across jumps (that's what "fix" means, e.g. config.ks's entire button set,
// registered once at *config_page); only [clearfix] (tags_layer.go) removes
// those.
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
			var err error
			img, _, err = ebitenutil.NewImageFromFileSystem(r.fses["images"], path)
			if err != nil {
				fmt.Printf("$(%q).attr(\"src\", %q): %v\n", selector, path, err)
				return
			}
		}
		b.Graphic = img
	}
}

func (r *Renderer) image(object kag3.TagObject) (err error) {
	img := &kag3.Image{Opacity: 255, ScaleX: 1, ScaleY: 1}
	for key, value := range object.Pm {
		switch key {
		case "layer":
			img.Layer = value
		case "page":
			img.Page = value
		case "left":
			img.Left, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "top":
			img.Top, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "x":
			img.X, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "y":
			img.Y, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "width":
			img.Width, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "height":
			img.Height, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "folder":
			img.Folder = value
		case "name":
			img.Name = value
		case "time":
			img.Time, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "wait":
			img.IsWait, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "zindex":
			img.ZIndex, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "depth":
			img.Depth = value
		case "refrect":
			img.Reflect, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "pos":
			img.Depth = value
		case "animimg":
			img.AnimImg, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		}
	}
	folder := "images"
	if object.Pm["folder"] != "" {
		folder = object.Pm["folder"]
	}
	img.Image, _, err = ebitenutil.NewImageFromFileSystem(
		r.fses[folder],
		object.Pm["storage"],
	)
	if err != nil {
		return
	}
	imgs = append(imgs, img)
	return nil
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

func (r *Renderer) drawScene(buf *ebiten.Image) {
	if bg.Image != nil {
		buf.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
		if bg.NextImage != nil {
			if transition, ok := effects.Transitions[bg.Method]; ok {
				transition.DrawBackground(buf, bg, bgTick, t, bg.Time)
			}
		}
	}
	if bg2.Image != nil {
		buf.DrawImage(bg2.Image, &ebiten.DrawImageOptions{})
		if bg2.NextImage != nil {
			if transition, ok := effects.Transitions[bg2.Method]; ok {
				transition.DrawBackground(buf, bg2, bg2Tick, t, bg2.Time)
			}
		}
	}
	for _, chara := range viewCharas {
		// A registered=false entry here means a loaded save's character
		// couldn't be reconciled (see reconcileViewCharas in tags_save.go)
		// — that function is meant to filter these out before they ever
		// reach viewCharas, but skip defensively rather than crash the
		// whole renderer if that invariant is ever violated.
		registered, ok := charas[chara.Name]
		if !ok || registered.Image == nil {
			continue
		}
		if chara.IsSlide {
			e := &effects.SlideInLeft{}
			e.Draw(buf, registered.Image, chara, charaTick, t, chara.Time)
		} else {
			if chara.IsRemove {
				e := &effects.FadeOut{}
				e.Draw(buf, registered.Image, chara.Left, chara.Top, charaTick, t, chara.Time,
					chara.Opacity/255, chara.ScaleX, chara.ScaleY, chara.Rotation)
			} else {
				e := &effects.FadeIn{}
				e.Draw(buf, registered.Image, chara.Left, chara.Top, charaTick, t, chara.Time,
					chara.Opacity/255, chara.ScaleX, chara.ScaleY, chara.Rotation)
			}
		}
		if !chara.IsRemove {
			drawCharaParts(buf, registered, chara.Left, chara.Top)
		}
	}
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
			charSize := r.fontFace.Size
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
					if textStyle != nil && textStyle.Color != nil {
						tOp.ColorScale.ScaleWithColor(textStyle.Color)
					} else if rs.style != nil && rs.style.Color != nil {
						tOp.ColorScale.ScaleWithColor(rs.style.Color)
					} else {
						tOp.ColorScale.ScaleWithColor(color.White)
					}
					text.Draw(buf, string(rs.ch), r.fontFace, tOp)
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
						if v.TextStyle != nil {
							if v.TextStyle.Color != nil {
								tOp.ColorScale.ScaleWithColor(v.TextStyle.Color)
							} else {
								tOp.ColorScale.ScaleWithColor(color.White)
							}
						} else {
							tOp.ColorScale.ScaleWithColor(color.White)
						}
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
			if textStyle != nil {
				if textStyle.Size != 0 && float64(textStyle.Size) != beforeTextSize {
					r.fontFace.Size = float64(textStyle.Size)
				}
				if textStyle.Color != nil {
					linkOp.ColorScale.ScaleWithColor(textStyle.Color)
				}
			}
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
	for _, img := range imgs {
		imgOp := &ebiten.DrawImageOptions{}
		applyPivotedImageTransform(imgOp, img.Image, img.ScaleX, img.ScaleY, img.Rotation)
		imgOp.GeoM.Translate(float64(img.X), float64(img.Y))
		imgOp.ColorScale.ScaleAlpha(float32(img.Opacity / 255))
		if blend, ok := layerBlend[img.Layer]; ok {
			imgOp.Blend = blend
		}
		buf.DrawImage(img.Image, imgOp)
	}
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

// drawCharaParts overlays a character's currently active differential
// parts (see [chara_layer]/[chara_part]) on top of its base image, aligned
// to the same origin. Layers are drawn in sorted-name order for
// determinism since Tyrano-style z-index configuration isn't implemented.
// applyPivotedImageTransform scales/rotates op around img's own center —
// the same convention as effects.applyPivotedTransform, duplicated here
// (unexported there) since [image]'s Opacity/ScaleX/ScaleY/Rotation are
// applied directly in drawScene rather than through an effects.* type.
func applyPivotedImageTransform(op *ebiten.DrawImageOptions, img *ebiten.Image, scaleX, scaleY, rotation float64) {
	if scaleX == 1 && scaleY == 1 && rotation == 0 {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Scale(scaleX, scaleY)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(float64(w)/2, float64(h)/2)
}

func drawCharaParts(screen *ebiten.Image, c *kag3.Character, left, top int) {
	if c == nil || len(c.ActivePart) == 0 {
		return
	}
	layerNames := make([]string, 0, len(c.ActivePart))
	for layer := range c.ActivePart {
		layerNames = append(layerNames, layer)
	}
	slices.Sort(layerNames)
	for _, layer := range layerNames {
		part := c.ActivePart[layer]
		img, ok := c.Parts[layer][part]
		if !ok {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(left), float64(top))
		screen.DrawImage(img, op)
	}
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

func parseColor(value string) (r, g, b int, err error) {
	switch value {
	case "black":
		return 255, 255, 255, nil
	case "red":
		return 255, 0, 0, nil
	case "blue":
		return 0, 0, 255, nil
	}
	runes := []rune(value)
	rStr, gStr, bStr := string(runes[2:3]), string(runes[4:5]), string(runes[6:7])
	r, err = strconv.Atoi(rStr)
	if err != nil {
		return
	}
	g, err = strconv.Atoi(gStr)
	if err != nil {
		return
	}
	b, err = strconv.Atoi(bStr)
	if err != nil {
		return
	}
	return
}

func isColision(mX, mY, x, y, width, height int) bool {
	return mX >= x && mX <= x+width && mY >= y && mY <= y+height
}
