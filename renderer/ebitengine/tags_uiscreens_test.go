package ebitengine

import (
	"bytes"
	"image/color"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func resetUIScreenState() {
	backlogViewing, menuOpen = false, false
	slotPickerActive = slotPickerNone
	editState = nil
	backlog = nil
}

// newTestFontFace builds a real *text.GoTextFace from the engine's bundled
// font, for tests that exercise a draw path going through text.Measure/
// text.Draw (which panic against the zero-value face newTestRenderer()
// otherwise leaves unset).
func newTestFontFace(t *testing.T) *text.GoTextFace {
	t.Helper()
	b, err := fs.ReadFile(kag3.Fonts, "NotoSansJP-Regular.ttf")
	if err != nil {
		t.Fatalf("failed to read embedded test font: %v", err)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed to parse embedded test font: %v", err)
	}
	return &text.GoTextFace{Source: src, Size: 16}
}

// newTestVerticalFontFace mirrors newTestFontFace but with the "vert"
// OpenType feature enabled — matches Manager.VerticalFontFace (manager.go),
// which is what makes [position vertical=true] substitute glyphs into
// their vertical-writing forms (punctuation moves to the upper-right of
// its cell, long vowel marks rotate, etc.) instead of just rotating
// horizontal-form glyphs into a vertical column.
func newTestVerticalFontFace(t *testing.T) *text.GoTextFace {
	t.Helper()
	b, err := fs.ReadFile(kag3.Fonts, "NotoSansJP-Regular.ttf")
	if err != nil {
		t.Fatalf("failed to read embedded test font: %v", err)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed to parse embedded test font: %v", err)
	}
	face := &text.GoTextFace{Source: src, Size: 16}
	face.SetFeature(text.MustParseTag("vert"), 1)
	return face
}

func TestAnyModalActiveReflectsEachOverlay(t *testing.T) {
	resetUIScreenState()
	if anyModalActive() {
		t.Fatal("expected no modal active initially")
	}
	backlogViewing = true
	if !anyModalActive() {
		t.Error("expected backlogViewing to count as a modal")
	}
	backlogViewing = false

	menuOpen = true
	if !anyModalActive() {
		t.Error("expected menuOpen to count as a modal")
	}
	menuOpen = false

	slotPickerActive = slotPickerSave
	if !anyModalActive() {
		t.Error("expected an open slot picker to count as a modal")
	}
	slotPickerActive = slotPickerNone

	editState = &editBoxState{Active: true}
	if !anyModalActive() {
		t.Error("expected an active edit box to count as a modal")
	}
	editState.Active = false
	if anyModalActive() {
		t.Error("expected an inactive (already committed) edit box to not count")
	}
	resetUIScreenState()
}

func TestShowSaveShowLoadOpenPicker(t *testing.T) {
	resetUIScreenState()
	menuOpen = true // showsave/showload must close the quick menu too
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), kag3.TagObject{Name: "showsave"}, &i, 0); err != nil {
		t.Fatalf("showsave dispatch error: %v", err)
	}
	if slotPickerActive != slotPickerSave || menuOpen {
		t.Errorf("after showsave: slotPickerActive=%v menuOpen=%v, want slotPickerSave/false", slotPickerActive, menuOpen)
	}

	if err := dispatchTag(newTestRenderer(), fakeYield(), kag3.TagObject{Name: "showload"}, &i, 0); err != nil {
		t.Fatalf("showload dispatch error: %v", err)
	}
	if slotPickerActive != slotPickerLoad {
		t.Errorf("slotPickerActive = %v, want slotPickerLoad", slotPickerActive)
	}
	resetUIScreenState()
}

func TestShowMenuShowLog(t *testing.T) {
	resetUIScreenState()
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), kag3.TagObject{Name: "showmenu"}, &i, 0); err != nil {
		t.Fatalf("showmenu dispatch error: %v", err)
	}
	if !menuOpen {
		t.Error("expected showmenu to open the quick menu")
	}
	if err := dispatchTag(newTestRenderer(), fakeYield(), kag3.TagObject{Name: "showlog"}, &i, 0); err != nil {
		t.Fatalf("showlog dispatch error: %v", err)
	}
	if !backlogViewing {
		t.Error("expected showlog to open the backlog viewer")
	}
	resetUIScreenState()
}

func TestSlotPickerRowsCountMatchesConfig(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720, ConfigSaveSlotNum: 3}
	rows := slotPickerRows(r)
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	for i, row := range rows {
		if row.Slot != i+1 {
			t.Errorf("rows[%d].Slot = %d, want %d", i, row.Slot, i+1)
		}
		if row.HasData {
			t.Errorf("rows[%d].HasData = true, want false (nothing saved yet)", i)
		}
		if row.StatusText != "まだ、保存されているデータがありません。" {
			t.Errorf("rows[%d].StatusText = %q, want the no-data message", i, row.StatusText)
		}
	}
}

func TestSlotPickerRowsReflectSavedSlot(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	// A prior test may have left a real decoded image in lastSnapshot (or
	// a renderBuffer that saveSlot's own auto-capture would turn into
	// one), which saveSlot would try to PNG-encode — that reads pixels
	// back from the GPU and panics outside a running ebiten game loop
	// (see tags_uiscreens_test.go's TestHasSaveSlotReflectsDiskState for
	// the same guard).
	renderBuffer, lastSnapshot = nil, nil

	r := newSaveTestRendererWithVars(t)
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720, ConfigSaveSlotNum: 3}
	r.texts = map[int][]Text{0: {{Text: "帰るか。。。"}}}
	if err := r.saveSlot(2); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}
	rows := slotPickerRows(r)
	if !rows[1].HasData {
		t.Error("expected slot 2's row to report HasData=true after saving")
	}
	if rows[1].StatusText == "まだ、保存されているデータがありません。" {
		t.Error("expected slot 2's row to show a timestamp, not the no-data message")
	}
	if rows[1].Message != "帰るか。。。" {
		t.Errorf("rows[1].Message = %q, want the message displayed at save time %q", rows[1].Message, "帰るか。。。")
	}
	if rows[0].HasData || rows[2].HasData {
		t.Errorf("expected only slot 2 to have data, got rows=%+v", rows)
	}
	if rows[0].Message != "" || rows[2].Message != "" {
		t.Errorf("expected empty slots to have no Message, got rows=%+v", rows)
	}
}

func TestBackButtonRectSizeFallsBackWithoutImage(t *testing.T) {
	systemImageCache = map[string]*ebiten.Image{}
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{}}
	rect := backButtonRect(r)
	if rect.W != 100 || rect.H != 100 {
		t.Errorf("backButtonRect fallback size = %dx%d, want 100x100", rect.W, rect.H)
	}
	if rect.X != 1280-100-slotPickerBackMargin {
		t.Errorf("backButtonRect.X = %d, want flush against the right margin", rect.X)
	}
}

func TestBackButtonImageNameSwapsOnHover(t *testing.T) {
	back := modalRect{X: 100, Y: 100, W: 50, H: 50}
	if got := backButtonImageName(back, 0, 0, false); got != "menu_button_close.png" {
		t.Errorf("cursor outside the button = %q, want menu_button_close.png", got)
	}
	if got := backButtonImageName(back, 125, 125, false); got != "menu_button_close2.png" {
		t.Errorf("cursor inside the button = %q, want menu_button_close2.png", got)
	}
}

func TestQuickMenuButtonsOrderAndImages(t *testing.T) {
	btns := quickMenuButtons()
	if len(btns) != 5 {
		t.Fatalf("len(quickMenuButtons()) = %d, want 5", len(btns))
	}
	want := []struct{ normal, hover string }{
		{"menu_button_save.png", "menu_button_save2.png"},
		{"menu_button_load.png", "menu_button_load2.png"},
		{"menu_message_close.png", "menu_message_close2.png"},
		{"menu_button_skip.png", "menu_button_skip2.png"},
		{"menu_button_title.png", "menu_button_title2.png"},
	}
	for i, w := range want {
		if btns[i].Normal != w.normal || btns[i].Hover != w.hover {
			t.Errorf("btns[%d] = %+v, want Normal=%q Hover=%q", i, btns[i], w.normal, w.hover)
		}
	}
	// Rows must not overlap and must be in top-to-bottom order.
	for i := 1; i < len(btns); i++ {
		if btns[i].Y <= btns[i-1].Y {
			t.Errorf("btns[%d].Y = %d, want greater than btns[%d].Y = %d", i, btns[i].Y, i-1, btns[i-1].Y)
		}
	}
}

func TestQuickMenuButtonImageNameSwapsOnHover(t *testing.T) {
	btn := quickMenuButtonSpec{Normal: "a.png", Hover: "a2.png", X: 100, Y: 100, W: 50, H: 50}
	if got := quickMenuButtonImageName(btn, 0, 0, false); got != "a.png" {
		t.Errorf("cursor outside the button = %q, want a.png", got)
	}
	if got := quickMenuButtonImageName(btn, 125, 125, false); got != "a2.png" {
		t.Errorf("cursor inside the button = %q, want a2.png", got)
	}
}

func TestSlotPickerScrollClampAndWraparound(t *testing.T) {
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720, ConfigSaveSlotNum: 5}
	slotPickerScrollY = -50
	clampSlotPickerScroll(r, 5)
	if slotPickerScrollY != 0 {
		t.Errorf("scrollY after clamping a negative value = %d, want 0", slotPickerScrollY)
	}
	slotPickerScrollY = 999999
	clampSlotPickerScroll(r, 5)
	_, vh := slotPickerViewport(720)
	want := slotPickerMaxScroll(5, vh)
	if slotPickerScrollY != want {
		t.Errorf("scrollY after clamping an overlarge value = %d, want max %d", slotPickerScrollY, want)
	}
	slotPickerScrollY = 0
}

func TestLoadSystemImageCachesAndFallsBackWhenMissing(t *testing.T) {
	systemImageCache = map[string]*ebiten.Image{}
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{}}
	if got := loadSystemImage(r, "bg_base.png"); got != nil {
		t.Errorf("loadSystemImage with no bg_base.png = %v, want nil (fall back to plain dim)", got)
	}
	// The miss itself should be cached too (no repeated log/lookup).
	if _, ok := systemImageCache["bg_base.png"]; !ok {
		t.Error("expected the failed load to be cached as a nil entry")
	}

	systemImageCache = map[string]*ebiten.Image{}
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{"bg_base.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	got := loadSystemImage(r, "bg_base.png")
	if got == nil {
		t.Fatal("expected bg_base.png to load successfully")
	}
	if again := loadSystemImage(r, "bg_base.png"); again != got {
		t.Error("expected loadSystemImage to return the cached image on a second call")
	}
	systemImageCache = map[string]*ebiten.Image{}
}

// TestSlotPickerViewportBufReusesSameSizeReallocatesOnResize covers
// slotPickerViewportBuf's whole reason for existing: drawSlotPicker used to
// allocate a fresh slotPickerRowW x viewportH *ebiten.Image every single
// frame the slot picker was open (a full-width chunk of the message
// window, freshly allocated 60 times a second) — the same class of GPU
// memory churn captureSnapshot's own doc comment (tags_save.go) documents
// actually causing memory-warning stalls on a real, RAM-constrained iOS
// device. slotPickerViewportBuf follows that same fix shape: reuse
// (Clear) when the requested size matches what's already allocated,
// reallocate only when it doesn't.
func TestSlotPickerViewportBufReusesSameSizeReallocatesOnResize(t *testing.T) {
	slotPickerViewportImg = nil
	defer func() { slotPickerViewportImg = nil }()

	first := slotPickerViewportBuf(1500, 600)
	if first == nil {
		t.Fatal("expected a non-nil image")
	}
	second := slotPickerViewportBuf(1500, 600)
	if second != first {
		t.Error("expected the same size requested twice to reuse the same *ebiten.Image, not allocate a new one")
	}

	third := slotPickerViewportBuf(1500, 400) // viewportH shrinks (e.g. window resize)
	if third == first {
		t.Error("expected a different size to allocate a new *ebiten.Image, not keep reusing the old (wrong-sized) one")
	}
	if third.Bounds().Dx() != 1500 || third.Bounds().Dy() != 400 {
		t.Errorf("size after resize = %dx%d, want 1500x400", third.Bounds().Dx(), third.Bounds().Dy())
	}
}

func TestDrawSlotPickerWithAndWithoutSystemImages(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720, ConfigSaveSlotNum: 3}
	r.fontFace = newTestFontFace(t)
	buf := newTestImage(1280, 720)

	systemImageCache = map[string]*ebiten.Image{}
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{}}
	drawSlotPicker(r, buf) // none of bg_base/label_load/menu_button_close/saveslot present: must fall back, not panic

	systemImageCache = map[string]*ebiten.Image{}
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{
		"bg_base.png":           &fstest.MapFile{Data: tinyPNG(t)},
		"label_load.png":        &fstest.MapFile{Data: tinyPNG(t)},
		"label_save.png":        &fstest.MapFile{Data: tinyPNG(t)},
		"menu_button_close.png": &fstest.MapFile{Data: tinyPNG(t)},
		"saveslot.png":          &fstest.MapFile{Data: tinyPNG(t)},
	}}
	drawSlotPicker(r, buf) // all real assets present: must draw them, not panic
	systemImageCache = map[string]*ebiten.Image{}
}

func TestDrawQuickMenuWithAndWithoutSystemImages(t *testing.T) {
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	buf := newTestImage(1280, 720)

	systemImageCache = map[string]*ebiten.Image{}
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{}}
	drawQuickMenu(r, buf) // no assets at all: must fall back, not panic

	systemImageCache = map[string]*ebiten.Image{}
	mapFS := fstest.MapFS{
		"bg_base.png":           &fstest.MapFile{Data: tinyPNG(t)},
		"label_menu.png":        &fstest.MapFile{Data: tinyPNG(t)},
		"menu_button_close.png": &fstest.MapFile{Data: tinyPNG(t)},
	}
	for _, btn := range quickMenuButtons() {
		mapFS[btn.Normal] = &fstest.MapFile{Data: tinyPNG(t)}
		mapFS[btn.Hover] = &fstest.MapFile{Data: tinyPNG(t)}
	}
	r.fses = map[string]fs.FS{"system/images": mapFS}
	drawQuickMenu(r, buf) // all real assets present: must draw them, not panic
	systemImageCache = map[string]*ebiten.Image{}
}

// TestSlotPickerClickNoopWithoutAnActualClick mirrors
// TestMenuButtonClickOpensQuickMenu's approach (tags_sysdesign_test.go):
// ebiten's real mouse-press state can't be synthesized in a headless unit
// test, so this only verifies handleSlotPickerClick doesn't do anything —
// or panic — when there's no just-pressed click to act on. The picker's
// item layout/labels are covered separately by
// TestSlotPickerItemsCountMatchesConfig.
func TestSlotPickerClickNoopWithoutAnActualClick(t *testing.T) {
	resetUIScreenState()
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720, ConfigSaveSlotNum: 3}
	openSlotPicker(slotPickerSave)
	r.handleSlotPickerClick()
	if slotPickerActive != slotPickerSave {
		t.Error("expected no click => picker stays open")
	}
	resetUIScreenState()
}

func TestHasSaveSlotReflectsDiskState(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	// A prior test may have left a real decoded image in lastSnapshot
	// (tags_sysdesign_test.go's save_img test, notably); saveSlot would
	// then try to PNG-encode it, which reads pixels back from the GPU and
	// panics outside a running ebiten game loop. Not what this test is
	// about, so clear it first.
	lastSnapshot = nil
	r := newSaveTestRendererWithVars(t)
	if hasSaveSlot(r, 2) {
		t.Fatal("expected slot 2 to be empty before any save")
	}
	if err := r.saveSlot(2); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}
	if !hasSaveSlot(r, 2) {
		t.Error("expected slot 2 to be reported as present after saving")
	}
}

func TestEditAndCommitWritesVariable(t *testing.T) {
	resetUIScreenState()
	r := newSaveTestRendererWithVars(t)
	tag := kag3.TagObject{Name: "edit", Pm: map[string]string{"name": "f.username", "default": "akane", "limit": "10"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("edit dispatch error: %v", err)
	}
	if editState == nil || !editState.Active || string(editState.Value) != "akane" {
		t.Fatalf("editState = %+v, want Active with Value=akane", editState)
	}

	commit := kag3.TagObject{Name: "commit"}
	if err := dispatchTag(r, fakeYield(), commit, &i, 0); err != nil {
		t.Fatalf("commit dispatch error: %v", err)
	}
	if editState != nil {
		t.Error("expected commit to clear editState")
	}
	if got := r.vm.EvalString("f.username"); got != "akane" {
		t.Errorf("f.username = %q, want %q", got, "akane")
	}
	resetUIScreenState()
}

func TestCommitWithNoActiveEditIsNoop(t *testing.T) {
	resetUIScreenState()
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "commit"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("commit dispatch error with no active edit: %v", err)
	}
}

func TestPreloadWaitPreloadUnload(t *testing.T) {
	preloadCache = map[string]*ebiten.Image{}
	r := newTestRenderer()
	mapFS := fstest.MapFS{"pic.png": &fstest.MapFile{Data: tinyPNG(t)}}
	r.fses = map[string]fs.FS{"images": mapFS}

	i := 0
	tag := kag3.TagObject{Name: "preload", Pm: map[string]string{"storage": "pic.png"}}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("preload dispatch error: %v", err)
	}
	if _, ok := preloadCache[preloadKey("images", "pic.png")]; !ok {
		t.Fatal("expected preload to populate preloadCache")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "wait_preload"}, &i, 0); err != nil {
		t.Fatalf("wait_preload dispatch error: %v", err)
	}

	unloadTag := kag3.TagObject{Name: "unload", Pm: map[string]string{"storage": "pic.png"}}
	if err := dispatchTag(r, fakeYield(), unloadTag, &i, 0); err != nil {
		t.Fatalf("unload dispatch error: %v", err)
	}
	if _, ok := preloadCache[preloadKey("images", "pic.png")]; ok {
		t.Error("expected unload to remove the cached entry")
	}
}

func TestPreloadMissingFileReturnsError(t *testing.T) {
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"images": fstest.MapFS{}}
	tag := kag3.TagObject{Name: "preload", Pm: map[string]string{"storage": "missing.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error preloading a missing file")
	}
}

func TestLoadJSEvaluatesFileContents(t *testing.T) {
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"senarios": fstest.MapFS{
		"setup.js": &fstest.MapFile{Data: []byte("f.loaded = 42;")},
	}}
	tag := kag3.TagObject{Name: "loadjs", Pm: map[string]string{"storage": "setup.js"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("loadjs dispatch error: %v", err)
	}
	if got := r.vm.EvalString("f.loaded"); got != "42" {
		t.Errorf("f.loaded = %q, want %q", got, "42")
	}
}

func TestPluginTracksNameAndOptionallyLoadsJS(t *testing.T) {
	loadedPlugins = map[string]bool{}
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"senarios": fstest.MapFS{
		"plug.js": &fstest.MapFile{Data: []byte("f.plugged = true;")},
	}}
	tag := kag3.TagObject{Name: "plugin", Pm: map[string]string{"name": "myplugin", "storage": "plug.js"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("plugin dispatch error: %v", err)
	}
	if !loadedPlugins["myplugin"] {
		t.Error("expected plugin name to be tracked")
	}
	if got := r.vm.EvalString("f.plugged"); got != "true" {
		t.Errorf("f.plugged = %q, want %q (storage= should behave like loadjs)", got, "true")
	}
}

func TestGLinkConfigDefaultsApplyWhenUnset(t *testing.T) {
	glinkDefaultColor = &color.RGBA{10, 20, 30, 255}
	glinkDefaultSize = 18
	glinkDefaultFace = "Default.ttf"
	glinks = nil

	cfg := kag3.TagObject{Name: "glink_config", Pm: map[string]string{"size": "30"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), cfg, &i, 0); err != nil {
		t.Fatalf("glink_config dispatch error: %v", err)
	}
	if glinkDefaultSize != 30 {
		t.Errorf("glinkDefaultSize = %d, want 30", glinkDefaultSize)
	}

	// height= is given explicitly so handleGLink skips its text.Measure
	// fallback, which needs a real font face newTestRenderer() doesn't set up.
	tag := kag3.TagObject{Name: "glink", Pm: map[string]string{"text": "hi", "height": "40"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("glink dispatch error: %v", err)
	}
	if len(glinks) != 1 {
		t.Fatalf("len(glinks) = %d, want 1", len(glinks))
	}
	g := glinks[0]
	if g.Color != glinkDefaultColor {
		t.Errorf("glink.Color = %v, want the configured default %v", g.Color, glinkDefaultColor)
	}
	if g.Size != 30 {
		t.Errorf("glink.Size = %d, want the configured default 30", g.Size)
	}
	glinks = nil
}

func TestClickableAppendsInvisibleButton(t *testing.T) {
	buttons = nil
	tag := kag3.TagObject{Name: "clickable", Pm: map[string]string{
		"x": "10", "y": "20", "width": "100", "height": "50", "target": "*next",
	}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("clickable dispatch error: %v", err)
	}
	if len(buttons) != 1 {
		t.Fatalf("len(buttons) = %d, want 1", len(buttons))
	}
	b := buttons[0]
	if b.Graphic != nil || b.EnterImg != nil {
		t.Error("expected a [clickable] button to have no graphic (invisible hit zone)")
	}
	if b.X != 10 || b.Y != 20 || b.Width != 100 || b.Height != 50 || b.Target != "*next" {
		t.Errorf("button = %+v, want the given geometry/target", b)
	}
	buttons = nil
}
