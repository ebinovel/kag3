package ebitengine

import (
	"image/color"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

func newTestImage(w, h int) *ebiten.Image {
	return ebiten.NewImage(w, h)
}

// newTestRendererWithSystemImageFS is newTestRendererWithImageFS but keyed
// under "system/images", for tags that read from there (showmenubutton,
// cursor) instead of "images".
func newTestRendererWithSystemImageFS(t *testing.T, files map[string][]byte) *Renderer {
	t.Helper()
	mapFS := fstest.MapFS{}
	for name, data := range files {
		mapFS[name] = &fstest.MapFile{Data: data}
	}
	r := newTestRenderer()
	r.currentStorage = "scene1.ks"
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	r.fses = map[string]fs.FS{"system/images": mapFS}
	return r
}

func resetMenuButtonState() {
	menuButtonVisible = false
	menuButtonImg = nil
	sysViewVisible = true
	menuOpen = false
	menuOpenedFrame = 0
}

func TestShowHideMenuButton(t *testing.T) {
	resetMenuButtonState()
	r := newTestRendererWithSystemImageFS(t, map[string][]byte{"button_menu.png": tinyPNG(t)})

	tag := kag3.TagObject{Name: "showmenubutton"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("showmenubutton dispatch error: %v", err)
	}
	if !menuButtonVisible || menuButtonImg == nil {
		t.Fatal("expected showmenubutton to load the image and set visible=true")
	}

	hide := kag3.TagObject{Name: "hidemenubutton"}
	if err := dispatchTag(r, fakeYield(), hide, &i, 0); err != nil {
		t.Fatalf("hidemenubutton dispatch error: %v", err)
	}
	if menuButtonVisible {
		t.Error("expected hidemenubutton to clear visibility")
	}
}

func TestMenuButtonRectBottomRightCorner(t *testing.T) {
	resetMenuButtonState()
	r := newTestRendererWithSystemImageFS(t, map[string][]byte{"button_menu.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "showmenubutton"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("showmenubutton dispatch error: %v", err)
	}
	x, y, w, h := menuButtonRect(r)
	// menuButtonMargin itself gets menuButtonScale'd along with the button's
	// size (see menuButtonReferenceScreenWidth's doc comment) — this test's
	// Renderer runs at 1280x720 (newTestRendererWithSystemImageFS), not the
	// 1920x1080 menuButtonMargin's literal value assumes.
	margin := int(float64(menuButtonMargin) * menuButtonScale(r))
	if x+w+margin != r.manager.Config.ScreenWidth {
		t.Errorf("x=%d w=%d margin=%d, want flush against the right margin (screen width %d)", x, w, margin, r.manager.Config.ScreenWidth)
	}
	if y+h+margin != r.manager.Config.ScreenHeight {
		t.Errorf("y=%d h=%d margin=%d, want flush against the bottom margin (screen height %d)", y, h, margin, r.manager.Config.ScreenHeight)
	}
}

// TestMenuButtonRectScalesWithScreenWidth guards against the bundled
// resources/system/images/button_menu.png (96x96, tuned for the
// たそがれ図書室 example's 1920x1080) looking oversized on a game running at
// a smaller ScreenWidth — e.g. example_tyrano_official_backup's 1280x720
// default, found to be visibly too large next to real TyranoScript's own
// rendering. At the reference 1920x1080 the button must
// stay pixel-exact (96x96, unscaled) so the たそがれ図書室 example's look
// doesn't change at all.
func TestMenuButtonRectScalesWithScreenWidth(t *testing.T) {
	resetMenuButtonState()
	r := newTestRendererWithSystemImageFS(t, map[string][]byte{"button_menu.png": sizedPNG(t, 96, 96)})
	r.manager.Config.ScreenWidth, r.manager.Config.ScreenHeight = 1920, 1080
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "showmenubutton"}, &i, 0); err != nil {
		t.Fatalf("showmenubutton dispatch error: %v", err)
	}
	if _, _, w, h := menuButtonRect(r); w != 96 || h != 96 {
		t.Errorf("at 1920x1080 (reference resolution): w,h = %d,%d, want 96,96 (unscaled)", w, h)
	}

	resetMenuButtonState()
	r = newTestRendererWithSystemImageFS(t, map[string][]byte{"button_menu.png": sizedPNG(t, 96, 96)})
	// newTestRendererWithSystemImageFS already sets 1280x720, matching
	// example_tyrano_official_backup's own default.
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "showmenubutton"}, &i, 0); err != nil {
		t.Fatalf("showmenubutton dispatch error: %v", err)
	}
	wantW := 96 * 1280 / 1920 // 64
	wantH := 96 * 720 / 1080  // 64
	if _, _, w, h := menuButtonRect(r); w != wantW || h != wantH {
		t.Errorf("at 1280x720: w,h = %d,%d, want %d,%d (scaled down from the 1920x1080 reference this button is tuned for)", w, h, wantW, wantH)
	}
}

func TestMenuButtonClickOpensQuickMenu(t *testing.T) {
	resetMenuButtonState()
	r := newTestRendererWithSystemImageFS(t, map[string][]byte{"button_menu.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "showmenubutton"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("showmenubutton dispatch error: %v", err)
	}
	if menuOpen {
		t.Fatal("menu should start closed")
	}
	// handleMenuButtonClick requires an actual just-pressed mouse click to
	// do anything; without one it must be a no-op regardless of position.
	r.handleMenuButtonClick()
	if menuOpen {
		t.Error("expected no click => no menu open")
	}
}

func TestSysViewHidesMenuButton(t *testing.T) {
	resetMenuButtonState()
	r := newTestRendererWithSystemImageFS(t, map[string][]byte{"button_menu.png": tinyPNG(t)})
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "showmenubutton"}, &i, 0); err != nil {
		t.Fatalf("showmenubutton dispatch error: %v", err)
	}
	off := kag3.TagObject{Name: "sysview", Pm: map[string]string{"visible": "false"}}
	if err := dispatchTag(r, fakeYield(), off, &i, 0); err != nil {
		t.Fatalf("sysview dispatch error: %v", err)
	}
	if sysViewVisible {
		t.Fatal("expected sysview visible=false to clear sysViewVisible")
	}
	buf := newTestImage(200, 200)
	drawMenuButton(r, buf) // must not panic; nothing to assert visually here
	on := kag3.TagObject{Name: "sysview", Pm: map[string]string{"visible": "true"}}
	if err := dispatchTag(r, fakeYield(), on, &i, 0); err != nil {
		t.Fatalf("sysview dispatch error: %v", err)
	}
	if !sysViewVisible {
		t.Error("expected sysview visible=true to restore sysViewVisible")
	}
}

func TestGlyphConfigTagsUpdateState(t *testing.T) {
	white := color.RGBA{255, 255, 255, 220}
	glyphNormal = glyphConfig{Color: white, Size: 12, OffsetX: -20, OffsetY: -20}
	glyphSkip = glyphConfig{Color: white, Size: 12, OffsetX: -20, OffsetY: -20}
	glyphAuto = glyphConfig{Color: white, Size: 12, OffsetX: -20, OffsetY: -20}

	tag := kag3.TagObject{Name: "glyph", Pm: map[string]string{"size": "24", "x": "-30", "y": "-40"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("glyph dispatch error: %v", err)
	}
	if glyphNormal.Size != 24 || glyphNormal.OffsetX != -30 || glyphNormal.OffsetY != -40 {
		t.Errorf("glyphNormal = %+v, want Size=24 OffsetX=-30 OffsetY=-40", glyphNormal)
	}

	skip := kag3.TagObject{Name: "glyph_skip", Pm: map[string]string{"size": "8"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), skip, &i, 0); err != nil {
		t.Fatalf("glyph_skip dispatch error: %v", err)
	}
	if glyphSkip.Size != 8 {
		t.Errorf("glyphSkip.Size = %d, want 8", glyphSkip.Size)
	}
	if glyphSkip.OffsetX != -20 {
		t.Errorf("glyph_skip should only change what was specified; OffsetX = %d, want unchanged -20", glyphSkip.OffsetX)
	}
}

func TestDrawGlyphPicksModeByPriority(t *testing.T) {
	textPosition = &kag3.TextPosition{Visible: true, Left: 0, Top: 0, Width: 100, Height: 100}
	buf := newTestImage(200, 200)

	isSkip, isAuto, isTextEnd = false, false, false
	drawGlyph(buf) // nothing active: must not panic

	isTextEnd = true
	drawGlyph(buf) // normal glyph path

	isAuto = true
	drawGlyph(buf) // auto takes priority over the plain "text end" dot

	isSkip = true
	drawGlyph(buf) // skip takes priority over auto

	isSkip, isAuto, isTextEnd = false, false, false
}

// TestDrawGlyphHiddenWhileTextStillRevealingUnderAuto pins drawGlyph to
// being a no-op whenever isTextEnd is false, auto or not. Drawing the mark
// regardless of isTextEnd under isAuto=true anchors it at whatever
// textEndX/textEndY is left over from the *previous* line — so the instant
// auto-advance moves to a new line and it starts revealing (isTextEnd goes
// false again until that line finishes), the mark keeps bouncing at the old
// line's position for the whole reveal, reading as "the wait indicator never
// went away" even though the story has already advanced.
func TestDrawGlyphHiddenWhileTextStillRevealingUnderAuto(t *testing.T) {
	// Restores the pre-test textPosition rather than forcing nil — see
	// TestDrawContinueMarkPicksColorByModePriority's comment
	// (draw_messagebox_test.go) for why that matters.
	savedTextPosition := textPosition
	defer func() { isSkip, isAuto, isTextEnd, textPosition = false, false, false, savedTextPosition }()

	textPosition = &kag3.TextPosition{Visible: true, Left: 0, Top: 0, Width: 100, Height: 100}
	buf := newTestImage(200, 200)

	isAuto, isTextEnd = true, false
	drawGlyph(buf) // must not draw anything (and must not panic) mid-reveal

	isSkip, isAuto, isTextEnd = true, false, false
	drawGlyph(buf) // same for skip mode mid-reveal
}

func TestHandleCursorLoadsImagesAndEnablesCustomCursor(t *testing.T) {
	cursorDefaultImg, cursorOverImg, cursorEnabled = nil, nil, false
	r := newTestRendererWithSystemImageFS(t, map[string][]byte{
		"cursor_default.png": tinyPNG(t),
		"cursor_pointer.png": tinyPNG(t),
	})
	tag := kag3.TagObject{Name: "cursor", Pm: map[string]string{"default": "cursor_default.png", "over": "cursor_pointer.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("cursor dispatch error: %v", err)
	}
	if !cursorEnabled || cursorDefaultImg == nil || cursorOverImg == nil {
		t.Fatal("expected cursor to load both images and enable the custom cursor")
	}
}

func TestSaveImgSetsLastSnapshot(t *testing.T) {
	lastSnapshot = nil
	r := newTestRendererWithImageFS(t, map[string][]byte{"cover.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "save_img", Pm: map[string]string{"storage": "cover.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("save_img dispatch error: %v", err)
	}
	if lastSnapshot == nil {
		t.Error("expected save_img to populate lastSnapshot")
	}
}

func TestModeEffectDisablesInstantBackgroundSwap(t *testing.T) {
	effectsEnabled = true
	defer func() { effectsEnabled = true }()

	off := kag3.TagObject{Name: "mode_effect", Pm: map[string]string{"enabled": "false"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), off, &i, 0); err != nil {
		t.Fatalf("mode_effect dispatch error: %v", err)
	}
	if effectsEnabled {
		t.Fatal("expected mode_effect enabled=false to clear effectsEnabled")
	}

	r := newTestRendererWithImageFS(t, map[string][]byte{"bg/bg.png": tinyPNG(t)})
	bg = &kag3.Background{Time: 3000, IsWait: true, Method: "crossfade"}
	bgTag := kag3.TagObject{Name: "bg", Pm: map[string]string{"storage": "bg.png"}}
	if err := dispatchTag(r, fakeYield(), bgTag, &i, 0); err != nil {
		t.Fatalf("bg dispatch error: %v", err)
	}
	if bg.NextImage != nil {
		t.Error("expected bg.NextImage to be nil (swapped instantly), not staged for a transition")
	}
	if bg.Image == nil {
		t.Error("expected bg.Image to be set to the new image immediately")
	}
	if !bg.IsEnd {
		t.Error("expected bg.IsEnd = true so any [wt]/[bg wait=true] doesn't block")
	}
}

func TestStateOnlyStubTags(t *testing.T) {
	bodyBackgroundColor = nil
	resizeCallExpr = ""
	loadingLogEnabled = false
	currentLang = ""

	i := 0
	r := newTestRenderer()
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "body", Pm: map[string]string{"color": "0x112233"}}, &i, 0); err != nil {
		t.Fatalf("body dispatch error: %v", err)
	}
	if bodyBackgroundColor == nil {
		t.Error("expected body to record bodyBackgroundColor")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "set_resizecall", Pm: map[string]string{"exp": "onResize()"}}, &i, 0); err != nil {
		t.Fatalf("set_resizecall dispatch error: %v", err)
	}
	if resizeCallExpr != "onResize()" {
		t.Errorf("resizeCallExpr = %q, want %q", resizeCallExpr, "onResize()")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "loading_log"}, &i, 0); err != nil {
		t.Fatalf("loading_log dispatch error: %v", err)
	}
	if !loadingLogEnabled {
		t.Error("expected bare [loading_log] (no visible=) to enable it")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "lang_set", Pm: map[string]string{"lang": "en"}}, &i, 0); err != nil {
		t.Fatalf("lang_set dispatch error: %v", err)
	}
	if currentLang != "en" {
		t.Errorf("currentLang = %q, want %q", currentLang, "en")
	}
}
