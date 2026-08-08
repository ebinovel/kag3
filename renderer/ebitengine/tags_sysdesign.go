package ebitengine

import (
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func init() {
	register("showmenubutton", handleShowMenuButton)
	register("hidemenubutton", handleHideMenuButton)
	register("glyph", handleGlyph)
	register("glyph_skip", handleGlyphSkip)
	register("glyph_auto", handleGlyphAuto)
	register("cursor", handleCursor)
	register("sysview", handleSysView)
	register("save_img", handleSaveImg)
	register("set_resizecall", handleSetResizeCall)
	register("mode_effect", handleModeEffect)
	register("loading_log", handleLoadingLog)
	register("lang_set", handleLangSet)
	register("body", handleBody)
}

// --- showmenubutton / hidemenubutton ---
//
// Deprecated: the redesigned message window (draw_messagebox.go/
// tags_oprow.go) has its own persistent operation row covering SAVE/LOAD/
// SKIP/Title, making this corner button + its quick-menu popup
// (drawQuickMenu/handleQuickMenuClick below) redundant — example/'s own
// scenarios no longer call @showmenubutton (see scene1.ks/demo_save.ks).
// [showmenubutton]/[hidemenubutton] and the quick menu they open are left
// implemented (not deleted) for any script that still calls them directly
// — they still work exactly as before, just no longer wired into the
// bundled example. Rendered as a fixed corner button using the bundled
// resources/system/images/button_menu.png, wired to the same quick-menu
// overlay role="menu" opens — one discoverable entry point to the same
// feature, not a second menu system.
var (
	menuButtonVisible bool
	menuButtonImg     *ebiten.Image
	sysViewVisible    = true
)

func handleShowMenuButton(ctx *tagCtx) error {
	if menuButtonImg == nil {
		img, _, err := ebitenutil.NewImageFromFileSystem(ctx.r.fses["system/images"], "button_menu.png")
		if err != nil {
			return err
		}
		menuButtonImg = img
	}
	menuButtonVisible = true
	return nil
}

func handleHideMenuButton(ctx *tagCtx) error {
	menuButtonVisible = false
	return nil
}

// menuButtonMargin is ×1.5 of the original 1280x720-tuned value (button
// position itself is already screenW/screenH-relative — see
// menuButtonRect — only this corner-inset margin needed scaling).
const menuButtonMargin = 30

func menuButtonRect(r *Renderer) (x, y, w, h int) {
	if menuButtonImg == nil {
		return 0, 0, 0, 0
	}
	w, h = menuButtonImg.Bounds().Dx(), menuButtonImg.Bounds().Dy()
	x = r.manager.Config.ScreenWidth - w - menuButtonMargin
	y = r.manager.Config.ScreenHeight - h - menuButtonMargin
	return
}

func drawMenuButton(r *Renderer, buf *ebiten.Image) {
	if !sysViewVisible || !menuButtonVisible || menuButtonImg == nil {
		return
	}
	x, y, _, _ := menuButtonRect(r)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	buf.DrawImage(menuButtonImg, op)
}

// handleMenuButtonClick is only consulted while the quick menu itself is
// closed (see Update() in renderer.go) — once open, the button sits behind
// the quick menu's dim overlay and its own "閉じる" item is how you close it.
func (r *Renderer) handleMenuButtonClick() {
	if !sysViewVisible || !menuButtonVisible || menuButtonImg == nil {
		return
	}
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	x, y, w, h := menuButtonRect(r)
	if isColisionTouch(mX, mY, x, y, w, h, touch) {
		menuOpen = true
		menuOpenedFrame = t
	}
}

// --- sysview: master visibility toggle for fixed system-layer chrome ---
//
// Only the menu button lives on that "system layer" so far, so this just
// gates it, but it's named/scoped to cover whatever else joins it later.
func handleSysView(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["visible"]
	if !ok {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}
	sysViewVisible = b
	return nil
}

// --- glyph / glyph_skip / glyph_auto ---
//
// Real Tyrano shows an animated icon (image-based) prompting the player to
// click once text has finished revealing, plus separate variants while
// skip/auto mode is active. kag3 has no bundled glyph artwork, so each mode
// is a small flat-colored dot instead — genuinely functional (position/
// color are configurable and it actually reflects live isWait/isSkip/
// isAuto state), just undecorated.
type glyphConfig struct {
	Color            color.RGBA
	Size             int
	OffsetX, OffsetY int
}

var (
	// OffsetY: 0 — textEndY (draw_message.go) is already the bottom edge of
	// the current text row, so the mark's rest position (before
	// glyphBounceOffset is added) sits flush against it with no further
	// adjustment needed.
	glyphNormal = glyphConfig{Color: color.RGBA{255, 255, 255, 220}, Size: 6, OffsetX: 8, OffsetY: 0}
	glyphSkip   = glyphConfig{Color: color.RGBA{255, 210, 60, 220}, Size: 6, OffsetX: 8, OffsetY: 0}
	glyphAuto   = glyphConfig{Color: color.RGBA{90, 200, 255, 220}, Size: 6, OffsetX: 8, OffsetY: 0}
	// textEndX/textEndY is the screen position right after the last glyph
	// of the currently active line, once fully revealed — set alongside
	// isTextEnd (drawMessageHorizontal/drawMessageVertical, draw_message.go)
	// so drawGlyph below can anchor the "click to continue" mark to the
	// actual end of the displayed text instead of a fixed corner of the
	// message box.
	textEndX, textEndY float64
)

func parseGlyphConfig(pm map[string]string, cfg *glyphConfig) error {
	if v, ok := pm["color"]; ok {
		r, g, b, err := parseColor(v)
		if err != nil {
			return err
		}
		cfg.Color = color.RGBA{uint8(r), uint8(g), uint8(b), 220}
	}
	if v, ok := pm["size"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		cfg.Size = n
	}
	if v, ok := pm["x"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		cfg.OffsetX = n
	}
	if v, ok := pm["y"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		cfg.OffsetY = n
	}
	return nil
}

func handleGlyph(ctx *tagCtx) error     { return parseGlyphConfig(ctx.tag.Pm, &glyphNormal) }
func handleGlyphSkip(ctx *tagCtx) error { return parseGlyphConfig(ctx.tag.Pm, &glyphSkip) }
func handleGlyphAuto(ctx *tagCtx) error { return parseGlyphConfig(ctx.tag.Pm, &glyphAuto) }

// drawGlyph is called from drawScene right after the message text itself,
// so it sees this frame's isTextEnd/textEndX/textEndY. The mark only ever
// shows once the *current* line has actually finished revealing
// (isTextEnd) — skip/auto just pick which color/mark to use while that's
// true, they don't bypass the isTextEnd check. This used to let skip/auto
// show continuously regardless of isTextEnd, anchored at textEndX/textEndY
// (only updated when a line finishes revealing — draw_message.go) — the
// instant a new line started revealing under isAuto, the mark kept
// bouncing at the *previous* line's end position for the whole reveal,
// reading as "the wait indicator never went away" even though the story
// had already moved on to the next line.
func drawGlyph(buf *ebiten.Image) {
	if textPosition == nil || !textPosition.Visible || !isTextEnd {
		return
	}
	var cfg *glyphConfig
	switch {
	case skipActive():
		cfg = &glyphSkip
	case isAuto:
		cfg = &glyphAuto
	default:
		cfg = &glyphNormal
	}
	if cfg.Size <= 0 {
		return
	}
	x := textEndX + float64(cfg.OffsetX)
	y := textEndY + float64(cfg.OffsetY) + glyphBounceOffset()
	fillRect(buf, x, y, float64(cfg.Size), float64(cfg.Size), cfg.Color)
}

// glyphBounceOffset drives the mark's idle "waiting for a click" bounce —
// real Tyrano uses an animated nextpage.gif here, which kag3 has no bundled
// artwork to reproduce (see glyphConfig's doc comment), so a small
// procedural bob stands in instead. Keyed off t (every frame, ticking
// regardless of isWait/isSkip/isAuto) rather than tick (frozen while the
// coroutine is blocked — see the isTextEnded note in state.go) so the
// animation itself keeps moving smoothly the whole time the mark is shown.
func glyphBounceOffset() float64 {
	const (
		periodTicks = 40
		amplitude   = 4.0
	)
	phase := float64(t%periodTicks) / periodTicks * 2 * math.Pi
	return -amplitude * math.Abs(math.Sin(phase))
}

// --- cursor: custom mouse pointer ---
//
// ebiten has no cross-platform custom-image-cursor API, so this hides the
// OS cursor and draws the configured image at the tracked mouse position
// instead, swapping to the "over" image while hovering a link/glink/button
// (see hoveringClickable in renderer.go's Update()).
var (
	cursorDefaultImg  *ebiten.Image
	cursorOverImg     *ebiten.Image
	cursorEnabled     bool
	hoveringClickable bool
)

func handleCursor(ctx *tagCtx) error {
	r := ctx.r
	if v, ok := ctx.tag.Pm["default"]; ok {
		img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["system/images"], v)
		if err != nil {
			return err
		}
		cursorDefaultImg = img
	}
	if v, ok := ctx.tag.Pm["over"]; ok {
		img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["system/images"], v)
		if err != nil {
			return err
		}
		cursorOverImg = img
	}
	if cursorDefaultImg != nil {
		cursorEnabled = true
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}
	return nil
}

// drawCursor is called from Renderer.Draw directly against screen (not
// renderBuffer) so it's exactly at the OS mouse position regardless of
// camera pan/zoom/shake.
func drawCursor(screen *ebiten.Image) {
	if !cursorEnabled {
		return
	}
	img := cursorDefaultImg
	if hoveringClickable && cursorOverImg != nil {
		img = cursorOverImg
	}
	if img == nil {
		return
	}
	mX, mY := ebiten.CursorPosition()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(mX), float64(mY))
	screen.DrawImage(img, op)
}

// --- save_img: use a fixed image (rather than a live capture) as the next
// save's thumbnail. Shares lastSnapshot with [savesnap] (tags_save.go) —
// same consumer, just a different way of producing the image.
func handleSaveImg(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["storage"]
	if !ok {
		return nil
	}
	img, err := loadImage(ctx.r, "", v)
	if err != nil {
		return err
	}
	lastSnapshot = img
	return nil
}

// --- mode_effect: effects on/off ---
//
// Wired into the one place it's cheap and visible today — background
// transitions (see applyBGTag in tags_background.go), which go instant
// instead of animated when disabled. [anim]/[quake]/etc. don't check this
// yet; threading it through every tween would be a much bigger change for a
// tag with no real usage in the example scenarios.
var effectsEnabled = true

func handleModeEffect(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["enabled"]
	if !ok {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}
	effectsEnabled = b
	return nil
}

// --- honest state-only stubs: body / set_resizecall / loading_log / lang_set ---
//
// Each names a real Tyrano feature this engine has no backing concept for
// yet (DOM <body> styling, a window-resize event pipeline, a unified asset-
// loader hook, multi-language content branching). Tracked rather than
// silently dropped, same spirit as [current] in tags_message.go — if a
// script sets them, a later feature can start consulting them without the
// tag itself needing to change.
var (
	bodyBackgroundColor *color.RGBA
	resizeCallExpr      string
	loadingLogEnabled   bool
	currentLang         string
)

func handleBody(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["color"]
	if !ok {
		return nil
	}
	r, g, b, err := parseColor(v)
	if err != nil {
		return err
	}
	c := color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	bodyBackgroundColor = &c
	return nil
}

func handleSetResizeCall(ctx *tagCtx) error {
	resizeCallExpr = ctx.tag.Pm["exp"]
	return nil
}

func handleLoadingLog(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["visible"]
	if !ok {
		loadingLogEnabled = true
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}
	loadingLogEnabled = b
	return nil
}

func handleLangSet(ctx *tagCtx) error {
	currentLang = ctx.tag.Pm["lang"]
	return nil
}
