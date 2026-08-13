package ebitengine

import (
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ebinovel/kag3"
)

// newSaveTestRendererWithVars is newTestRenderer() plus a couple of f/sf
// variables set through the VM, mirroring what a real [iscript] would leave
// behind before a save.
func newSaveTestRendererWithVars(t *testing.T) *Renderer {
	r := newTestRenderer()
	r.currentStorage = "scene1.ks"
	if _, err := r.vm.Eval("f.hoge = 5; sf.seen = true;"); err != nil {
		t.Fatalf("seeding f/sf failed: %v", err)
	}
	// applySaveData (called via loadSlot/rollback/applySaveData directly by
	// most tests using this helper) sets oldTick = tick - 4 (see its comment
	// in tags_save.go). These tests never drive tick forward through
	// Update() afterward, so that offset would otherwise stick around and
	// break later tests' isTextEnded checks (oldTick+3>=tick, permanently
	// false once oldTick is pinned behind tick) — see the matching comment
	// in confirm_title_flow_test.go.
	t.Cleanup(func() { tick, oldTick = 0, 0 })
	return r
}

// TestSaveDirRespectsKAG3SaveDirEnvVar covers the e2e/ harness's only way
// to sandbox save files: it's a separate OS process, so it can't set the
// in-package saveBaseDirOverride var the way Go tests do — KAG3_SAVE_DIR
// has to win outright and be used verbatim (no kag3/<title>/saves suffix).
func TestSaveDirRespectsKAG3SaveDirEnvVar(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	override := filepath.Join(t.TempDir(), "custom-save-dir")
	t.Setenv("KAG3_SAVE_DIR", override)

	r := newSaveTestRendererWithVars(t)
	got, err := saveDir(r)
	if err != nil {
		t.Fatalf("saveDir error: %v", err)
	}
	if got != override {
		t.Errorf("saveDir() = %q, want %q (KAG3_SAVE_DIR should win over saveBaseDirOverride)", got, override)
	}
	if info, err := os.Stat(override); err != nil || !info.IsDir() {
		t.Errorf("expected saveDir to create directory %q: %v", override, err)
	}
}

// TestSaveDirRespectsSaveDirFunc covers the embedding-app override hook
// (e.g. example/mobile's android-only JNI bridge, obtaining
// Context.getFilesDir() — see SaveDirFunc's own doc comment): when set (and
// saveBaseDirOverride/KAG3_SAVE_DIR aren't), saveDir must use whatever it
// returns as the base directory, still applying the usual kag3/<title>/
// saves suffix on top.
func TestSaveDirRespectsSaveDirFunc(t *testing.T) {
	saveBaseDirOverride = ""
	base := t.TempDir()
	SaveDirFunc = func() (string, error) { return base, nil }
	defer func() { SaveDirFunc = nil }()

	r := newSaveTestRendererWithVars(t)
	r.manager.Config = &kag3.Config{Title: "TestGame"}
	got, err := saveDir(r)
	if err != nil {
		t.Fatalf("saveDir error: %v", err)
	}
	want := filepath.Join(base, "kag3", "TestGame", "saves")
	if got != want {
		t.Errorf("saveDir() = %q, want %q (SaveDirFunc's result + kag3/<title>/saves suffix)", got, want)
	}
}

// TestSaveDirSaveDirFuncLosesToSaveBaseDirOverride confirms priority order:
// saveBaseDirOverride (the in-package Go test override) still wins over
// SaveDirFunc when both are set, matching KAG3_SAVE_DIR's own precedence
// over both.
func TestSaveDirSaveDirFuncLosesToSaveBaseDirOverride(t *testing.T) {
	override := t.TempDir()
	saveBaseDirOverride = override
	defer func() { saveBaseDirOverride = "" }()

	SaveDirFunc = func() (string, error) {
		t.Fatal("SaveDirFunc should not be consulted when saveBaseDirOverride is set")
		return "", nil
	}
	defer func() { SaveDirFunc = nil }()

	r := newSaveTestRendererWithVars(t)
	if _, err := saveDir(r); err != nil {
		t.Fatalf("saveDir error: %v", err)
	}
}

func TestSaveSlotRoundTrip(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	defer delete(charas, "akane")

	r := newSaveTestRendererWithVars(t)
	r.callStack = []callFrame{{Storage: "sub.ks", Index: 3}}
	// Registered in charas (same-process load), like a real [chara_new]
	// would leave it — reconcileViewCharas's character-reconstruction path
	// (fresh-process load, no prior registration) is covered separately by
	// TestApplySaveDataReconstructsCharaAfterFreshProcess.
	charas["akane"] = &kag3.Character{Name: "akane"}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 10, Top: 20}}
	bg.Time, bg.Method, bg.Position = 1234, "slide", "left"
	currentScriptIndex = 42

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Clobber everything the save should restore, so the round trip is
	// actually observed rather than coincidentally already correct.
	r.callStack = nil
	viewCharas = nil
	bg.Time, bg.Method, bg.Position = 0, "", ""
	r.vm.ClearF()
	r.vm.ClearSF()
	jumpIndex, isJump = 0, false

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	if len(r.callStack) != 1 || r.callStack[0] != (callFrame{Storage: "sub.ks", Index: 3}) {
		t.Errorf("callStack after load = %+v, want [{sub.ks 3}]", r.callStack)
	}
	if len(viewCharas) != 1 || viewCharas[0].Name != "akane" || viewCharas[0].Left != 10 {
		t.Errorf("viewCharas after load = %+v", viewCharas)
	}
	if bg.Time != 1234 || bg.Method != "slide" || bg.Position != "left" {
		t.Errorf("bg after load = %+v, want Time=1234 Method=slide Position=left", bg)
	}
	if got := r.vm.EvalString("f.hoge"); got != "5" {
		t.Errorf("f.hoge after load = %q, want %q", got, "5")
	}
	if got := r.vm.EvalString("sf.seen"); got != "true" {
		t.Errorf("sf.seen after load = %q, want %q", got, "true")
	}
	if !isJump || jumpIndex != 42 {
		t.Errorf("isJump/jumpIndex after load = %v/%d, want true/42", isJump, jumpIndex)
	}
}

// TestSaveSlotRoundTripRestoresTextStyle is the regression test for a real
// reported bug: text was white at save time, later turned black ([font]/
// [deffont] further into the story), and loading the earlier (white) save
// kept showing black — because textStyle/defaultTextStyle weren't part of
// saveData at all, so nothing reset them back to what was active when the
// save was actually taken.
func TestSaveSlotRoundTripRestoresTextStyle(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	// bg/textPosition are package-level and may carry leftover state from
	// an earlier test in this run — reset so buildSaveData doesn't capture
	// a Bg.Storage/TextPosition.FrameStorage that would send applySaveData
	// down the fs.FS-reload path this minimal renderer (no r.fses) can't
	// serve.
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	white := &color.RGBA{0xff, 0xff, 0xff, 0xff}
	textStyle = &kag3.TextStyle{Color: white}
	defaultTextStyle = &kag3.TextStyle{Color: white}
	defer func() { textStyle, defaultTextStyle = nil, nil }()

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate the story continuing past the save point and changing color.
	black := &color.RGBA{0x00, 0x00, 0x00, 0xff}
	textStyle = &kag3.TextStyle{Color: black}
	defaultTextStyle = &kag3.TextStyle{Color: black}

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	if textStyle == nil || textStyle.Color == nil || *textStyle.Color != *white {
		t.Errorf("textStyle after load = %+v, want Color=%+v (the color active at save time)", textStyle, white)
	}
	if defaultTextStyle == nil || defaultTextStyle.Color == nil || *defaultTextStyle.Color != *white {
		t.Errorf("defaultTextStyle after load = %+v, want Color=%+v", defaultTextStyle, white)
	}
}

// TestRollbackDoesNotAliasCheckpointTextStyle ensures [checkpoint]'s
// in-memory saveData isn't corrupted by gameplay after it: a [font] call
// following a [rollback] mutates the *live* textStyle struct in place (see
// r.textStyle in renderer.go), so if applySaveData ever assigned d.TextStyle
// directly instead of copying it, a second rollback to the same checkpoint
// would incorrectly show the post-rollback color.
func TestRollbackDoesNotAliasCheckpointTextStyle(t *testing.T) {
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	white := &color.RGBA{0xff, 0xff, 0xff, 0xff}
	textStyle = &kag3.TextStyle{Color: white}
	defer func() { textStyle, checkpointData = nil, nil }()

	checkpointData = r.buildSaveData()

	// First rollback, then simulate a [font color=...] mutating the live
	// struct in place exactly like r.textStyle does.
	if err := r.applySaveData(checkpointData); err != nil {
		t.Fatalf("applySaveData (1st rollback) error: %v", err)
	}
	textStyle.Color = &color.RGBA{0x00, 0x00, 0x00, 0xff}

	// Second rollback to the *same* checkpoint must still restore white.
	if err := r.applySaveData(checkpointData); err != nil {
		t.Fatalf("applySaveData (2nd rollback) error: %v", err)
	}
	if textStyle == nil || textStyle.Color == nil || *textStyle.Color != *white {
		t.Errorf("textStyle after 2nd rollback = %+v, want Color=%+v (checkpoint must not have been mutated by the 1st rollback's aftermath)", textStyle, white)
	}
}

// TestCheckpointRollbackRestoresCharaShowLeft is the regression test for
// #16: buildSaveData used to append viewCharas' *CharaShow pointers
// directly into checkpointData rather than copying them, so a [chara_show]/
// [anim] mutating a character in place (stepAnimations, tags_animation.go)
// after [checkpoint] silently rewrote the checkpoint too — [rollback] then
// "restored" the already-mutated value instead of the one actually
// captured.
func TestCheckpointRollbackRestoresCharaShowLeft(t *testing.T) {
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	origCharas, origViewCharas := charas, viewCharas
	defer func() {
		bg, textPosition = &kag3.Background{}, &kag3.TextPosition{}
		charas, viewCharas, checkpointData = origCharas, origViewCharas, nil
	}()

	charas = map[string]*kag3.Character{"akane": {Name: "akane", Storage: "akane.png"}}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 100}}

	r := newSaveTestRendererWithVars(t)
	checkpointData = r.buildSaveData()

	// Simulate [anim]/stepAnimations mutating the live CharaShow in place
	// after the checkpoint was taken.
	viewCharas[0].Left = 999

	if err := r.applySaveData(checkpointData); err != nil {
		t.Fatalf("applySaveData (rollback): %v", err)
	}
	if len(viewCharas) != 1 || viewCharas[0].Left != 100 {
		t.Errorf("viewCharas after rollback = %+v, want Left=100 (the value at checkpoint time)", viewCharas)
	}
}

// TestRollbackDoesNotAliasCheckpointCharaShow mirrors
// TestRollbackDoesNotAliasCheckpointTextStyle for viewCharas: applySaveData
// used to hand reconcileViewCharas the checkpoint's own *CharaShow pointers
// unmodified, so a second rollback to the same checkpoint would reflect
// whatever the first rollback's aftermath (e.g. a post-rollback [anim]) did
// to them, not the state actually captured.
func TestRollbackDoesNotAliasCheckpointCharaShow(t *testing.T) {
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	origCharas, origViewCharas := charas, viewCharas
	defer func() {
		bg, textPosition = &kag3.Background{}, &kag3.TextPosition{}
		charas, viewCharas, checkpointData = origCharas, origViewCharas, nil
	}()

	charas = map[string]*kag3.Character{"akane": {Name: "akane", Storage: "akane.png"}}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 100}}

	r := newSaveTestRendererWithVars(t)
	checkpointData = r.buildSaveData()

	if err := r.applySaveData(checkpointData); err != nil {
		t.Fatalf("applySaveData (1st rollback): %v", err)
	}
	viewCharas[0].Left = 999

	if err := r.applySaveData(checkpointData); err != nil {
		t.Fatalf("applySaveData (2nd rollback): %v", err)
	}
	if len(viewCharas) != 1 || viewCharas[0].Left != 100 {
		t.Errorf("viewCharas after 2nd rollback = %+v, want Left=100 (checkpoint must not have been mutated by the 1st rollback's aftermath)", viewCharas)
	}
}

// TestSaveSlotRoundTripRestoresMenuButtonVisible is the regression test for
// a real reported bug: the corner @showmenubutton icon was visible at save
// time, later hidden by [hidemenubutton] further into the story, and
// loading the earlier save kept it hidden — because menuButtonVisible
// wasn't part of saveData, so nothing restored it to what was active when
// the save was actually taken.
func TestSaveSlotRoundTripRestoresMenuButtonVisible(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	menuButtonVisible = true
	menuButtonImg = newTestImage(10, 10) // already loaded, same-process
	defer func() { menuButtonVisible, menuButtonImg = false, nil }()

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate the story continuing past the save point and hiding it.
	menuButtonVisible = false

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	if !menuButtonVisible {
		t.Error("menuButtonVisible after load = false, want true (the state active at save time)")
	}
}

// TestSaveSlotRoundTripRestoresPtexts is the regression test for a real
// reported bug: the character name-plate ptext area was at one position at
// save time, later repositioned by a further [chara_config]/[ptext] call,
// and loading the earlier save kept showing the *repositioned* name-plate —
// because ptexts/charaNamePText weren't part of saveData at all, so nothing
// reset them back to what was active when the save was actually taken.
func TestSaveSlotRoundTripRestoresPtexts(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	ptexts = map[string]*kag3.PText{
		"chara_name_area": {Name: "chara_name_area", X: 10, Y: 20, Text: "あかね"},
	}
	charaNamePText = "chara_name_area"
	defer func() { ptexts, charaNamePText = map[string]*kag3.PText{}, "" }()

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate the story continuing past the save point and moving the
	// name-plate to a new position (e.g. a redesigned message window).
	ptexts["chara_name_area"] = &kag3.PText{Name: "chara_name_area", X: 100, Y: 200, Text: "あかね"}

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	got, ok := ptexts["chara_name_area"]
	if !ok || got.X != 10 || got.Y != 20 {
		t.Errorf("ptexts[chara_name_area] after load = %+v, want X=10 Y=20 (the position active at save time)", got)
	}
	if charaNamePText != "chara_name_area" {
		t.Errorf("charaNamePText after load = %q, want %q", charaNamePText, "chara_name_area")
	}
}

// TestSaveSlotRoundTripReloadsPtextBgImage is the regression test for a
// real crash: kag3.PText.BgImage ([ptext bg=], tags_text.go) is a raw
// *ebiten.Image, which used to round-trip straight through the save JSON
// like every other PText field. json.Unmarshal produced a zero-value
// Image indistinguishable from a disposed one (not nil), and the first
// drawPTexts call after any load — even same-process — panicked with
// "ebiten: the given image to DrawImage must not be disposed". BgImage now
// carries `json:"-"` and must be reloaded from BgStorage explicitly after
// Ptexts is restored, the same pattern already used for
// TextPosition.FrameStorage/FrameImage.
func TestSaveSlotRoundTripReloadsPtextBgImage(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{"ui/name_tab.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	ptexts = map[string]*kag3.PText{
		"chara_name_area": {
			Name: "chara_name_area", X: 10, Y: 20, Text: "あかね",
			BgStorage: "ui/name_tab.png", BgImage: newTestImage(300, 60),
		},
	}
	charaNamePText = "chara_name_area"
	defer func() { ptexts, charaNamePText = map[string]*kag3.PText{}, "" }()

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate the story continuing past the save point (or a fresh
	// process): BgImage is whatever json.Unmarshal produced for it
	// (effectively unusable, since it's excluded from JSON), not the live
	// image the session originally loaded.
	ptexts["chara_name_area"] = &kag3.PText{Name: "chara_name_area", X: 999, Y: 999, Text: "wrong"}

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	got, ok := ptexts["chara_name_area"]
	if !ok {
		t.Fatal("expected chara_name_area to be restored")
	}
	if got.BgImage == nil {
		t.Fatal("expected BgImage to be reloaded from BgStorage after loadSlot, got nil — drawPTexts would silently skip the background instead of crashing, but the name tab would be missing")
	}
}

// TestSaveSlotRoundTripRestoresTextsAndCharaName is the regression test for
// a real reported bug: applySaveData resumes execution via jumpIndex
// straight at the saved [p]/[s]/[l] tag, never re-running whatever
// TextObject(s) before it in the script actually put text on screen — so
// the message window (and the name-plate, driven separately by charaName)
// came back completely blank after every load, not just one predating
// this fix's own save format.
func TestSaveSlotRoundTripRestoresTextsAndCharaName(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	r.texts = map[int][]Text{0: {{Text: "こんにちは"}}}
	charaName = "akane"
	defer func() { charaName = "" }()

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate the story continuing past the save point.
	r.texts = map[int][]Text{0: {{Text: "違う内容"}}}
	charaName = "yamato"

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	got, ok := r.texts[0]
	if !ok || len(got) != 1 || got[0].Text != "こんにちは" {
		t.Errorf("r.texts[0] after load = %+v, want [{Text: \"こんにちは\"}] (the text active at save time)", got)
	}
	if charaName != "akane" {
		t.Errorf("charaName after load = %q, want %q", charaName, "akane")
	}
}

// TestApplySaveDataRestoresLinksAndSetsPreserveFlag is the regression test
// for a real reported bug: a save taken right after a [link] choice (e.g.
// landing on the [s] that follows a [link]/[endlink] pair) lost the choice
// itself on load — applySaveData resumes via jumpIndex straight at that
// [s], never re-running the [link] tags that originally registered it.
// applySaveData must restore links/glinks directly from the save and set
// preserveLinksOnJump so the very next Update() frame's clearLinksOnJump
// doesn't immediately wipe them out again (see
// TestClearLinksOnJumpPreservesRestoredLinksOnLoad, renderer_test.go).
func TestApplySaveDataRestoresLinksAndSetsPreserveFlag(t *testing.T) {
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	defer func() { bg, textPosition = &kag3.Background{}, &kag3.TextPosition{} }()

	r := newSaveTestRendererWithVars(t)
	links = []*kag3.Link{{Target: "*playmusic"}}
	glinks = nil
	defer func() { links, glinks, preserveLinksOnJump = nil, nil, false }()

	d := r.buildSaveData()
	if len(d.Links) != 1 || d.Links[0].Target != "*playmusic" {
		t.Fatalf("buildSaveData().Links = %+v, want a single Link{Target: \"*playmusic\"}", d.Links)
	}

	// Simulate the story continuing past the save point and clicking that
	// very choice, which normally clears links/glinks.
	links, glinks = nil, nil
	preserveLinksOnJump = false

	if err := r.applySaveData(d); err != nil {
		t.Fatalf("applySaveData error: %v", err)
	}

	if len(links) != 1 || links[0].Target != "*playmusic" {
		t.Errorf("links after applySaveData = %+v, want the saved choice restored", links)
	}
	if !preserveLinksOnJump {
		t.Error("expected applySaveData to set preserveLinksOnJump so the restored choice survives the next clearLinksOnJump sweep")
	}
}

// TestApplySaveDataLoadsMenuButtonImageForFreshProcess covers the other
// half: a fresh process never ran [showmenubutton], so menuButtonImg is
// nil — restoring MenuButtonVisible=true alone isn't enough, since
// drawMenuButton/handleMenuButtonClick both also gate on menuButtonImg !=
// nil. applySaveData must load it itself.
func TestApplySaveDataLoadsMenuButtonImageForFreshProcess(t *testing.T) {
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	menuButtonVisible, menuButtonImg = false, nil
	defer func() {
		bg, textPosition = &kag3.Background{}, &kag3.TextPosition{}
		menuButtonVisible, menuButtonImg = false, nil
	}()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{
		"button_menu.png": &fstest.MapFile{Data: tinyPNG(t)},
	}}

	d := &saveData{Storage: r.currentStorage, MenuButtonVisible: true}
	if err := r.applySaveData(d); err != nil {
		t.Fatalf("applySaveData error: %v", err)
	}

	if !menuButtonVisible {
		t.Error("menuButtonVisible after load = false, want true")
	}
	if menuButtonImg == nil {
		t.Error("menuButtonImg after load = nil, want it lazily loaded from system/images")
	}
}

// TestSlotStoreOverrideBypassesFilesystem is the deliverable for the wasm
// save backend (storage_js.go): with slotStore registered, every save-slot
// read and write must route through it instead of touching a real file —
// and, critically, without ever calling saveDir. On GOOS=js saveDir fails
// outright (os.UserConfigDir/os.UserHomeDir both error with no $HOME), so
// this test deliberately leaves saveBaseDirOverride and KAG3_SAVE_DIR
// unset: anything still reaching for a directory would surface here as an
// error rather than silently working off the maintainer's real config dir.
func TestSlotStoreOverrideBypassesFilesystem(t *testing.T) {
	saveBaseDirOverride = ""
	// saveSlot PNG-encodes lastSnapshot, which reads pixels back from the
	// GPU and panics outside a running ebiten game loop (same guard as
	// TestSlotPickerRowsReflectSavedSlot, tags_uiscreens_test.go).
	renderBuffer, lastSnapshot = nil, nil

	type storedSlot struct {
		data    []byte
		savedAt time.Time
	}
	stored := map[string]storedSlot{}
	key := func(slot int, ext string) string { return fmt.Sprintf("%d.%s", slot, ext) }

	// With saveBaseDirOverride/KAG3_SAVE_DIR both unset, SaveDirFunc is the
	// next thing saveDir consults — so this doubles as a tripwire proving
	// no code path below reaches for a directory at all (on Windows saveDir
	// would otherwise quietly succeed against the real %AppData%, hiding
	// exactly the bug that breaks the browser build).
	SaveDirFunc = func() (string, error) {
		t.Error("saveDir must not be consulted while slotStore is registered (it fails outright on GOOS=js)")
		return "", errors.New("saveDir should not have been called")
	}
	defer func() { SaveDirFunc = nil }()

	slotStore.Save = func(r *Renderer, slot int, ext string, data []byte) error {
		stored[key(slot, ext)] = storedSlot{data: append([]byte(nil), data...), savedAt: time.Unix(1700000000, 0)}
		return nil
	}
	slotStore.Load = func(r *Renderer, slot int, ext string) ([]byte, error) {
		s, ok := stored[key(slot, ext)]
		if !ok {
			return nil, fmt.Errorf("no data for slot %d (%s)", slot, ext)
		}
		return s.data, nil
	}
	slotStore.Info = func(r *Renderer, slot int) (bool, time.Time) {
		s, ok := stored[key(slot, "json")]
		if !ok {
			return false, time.Time{}
		}
		return true, s.savedAt
	}
	defer func() { slotStore.Save, slotStore.Load, slotStore.Info = nil, nil, nil }()
	// loadSlotThumbnail below negatively-caches slot 2; clear it so a later
	// test isn't handed this test's "no thumbnail" answer (see CLAUDE.md on
	// this package's shared package-level state).
	defer func() { clear(slotThumbnailCache) }()

	r := newSaveTestRendererWithVars(t)
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720, ConfigSaveSlotNum: 3}
	r.texts = map[int][]Text{0: {{Text: "またね。"}}}

	if err := r.saveSlot(2); err != nil {
		t.Fatalf("saveSlot through slotStore: %v", err)
	}
	if _, ok := stored[key(2, "json")]; !ok {
		t.Fatal("expected saveSlot to write slot 2's JSON through slotStore.Save")
	}

	// Round-trip back through the same hook.
	r2 := newSaveTestRendererWithVars(t)
	r2.manager.Config = r.manager.Config
	if err := r2.loadSlot(2); err != nil {
		t.Fatalf("loadSlot through slotStore: %v", err)
	}

	// The picker's three readers must agree with what was stored.
	exists, modTime := saveSlotInfo(r, 2)
	if !exists || !modTime.Equal(time.Unix(1700000000, 0)) {
		t.Errorf("saveSlotInfo(2) = (%v, %v), want (true, the timestamp slotStore.Info reported)", exists, modTime)
	}
	if exists, _ := saveSlotInfo(r, 1); exists {
		t.Error("saveSlotInfo(1) reported data for a slot that was never saved")
	}
	if got := saveSlotLastMessage(r, 2); got != "またね。" {
		t.Errorf("saveSlotLastMessage(2) = %q, want %q", got, "またね。")
	}
	rows := slotPickerRows(r)
	if !rows[1].HasData || rows[1].Message != "またね。" {
		t.Errorf("slotPickerRows()[1] = %+v, want HasData=true and the saved message", rows[1])
	}

	// A slot with JSON but no thumbnail must degrade to "no thumbnail",
	// not error — loadSlotThumbnail's own long-standing tolerance.
	delete(slotThumbnailCache, 2)
	if img := loadSlotThumbnail(r, 2); img != nil {
		t.Error("expected no thumbnail for a slot stored without a PNG")
	}
}

// TestSettingsStoreOverrideBypassesFilesystem is the [configsave]/
// [configload] half of the same wasm backend, and pins the precedence
// between the two hooks: the exported ConfigStorage (an embedding app's
// explicit choice, e.g. Android's DataStore bridge) must win over the
// in-package settingsStore when both are set.
func TestSettingsStoreOverrideBypassesFilesystem(t *testing.T) {
	saveBaseDirOverride = ""

	var stored []byte
	settingsStore.Save = func(r *Renderer, data []byte) error {
		stored = append([]byte(nil), data...)
		return nil
	}
	settingsStore.Load = func(r *Renderer) ([]byte, error) { return stored, nil }
	defer func() { settingsStore.Save, settingsStore.Load = nil, nil }()

	// Same tripwire as TestSlotStoreOverrideBypassesFilesystem: settingsPath
	// goes through saveDir, which must never be reached here.
	SaveDirFunc = func() (string, error) {
		t.Error("saveDir must not be consulted while settingsStore is registered")
		return "", errors.New("saveDir should not have been called")
	}
	defer func() { SaveDirFunc = nil }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "iscript", Body: "tf.set_speed_idx = 4;"}, &i, 0); err != nil {
		t.Fatalf("iscript: %v", err)
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "configsave"}, &i, 0); err != nil {
		t.Fatalf("configsave: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected [configsave] to write through settingsStore.Save")
	}

	r2 := newTestRenderer()
	if err := dispatchTag(r2, fakeYield(), kag3.TagObject{Name: "configload"}, &i, 0); err != nil {
		t.Fatalf("configload: %v", err)
	}
	if got := r2.vm.EvalString("tf.set_speed_idx"); got != "4" {
		t.Errorf("tf.set_speed_idx after configload = %q, want %q", got, "4")
	}

	// ConfigStorage set as well: it must take precedence, leaving
	// settingsStore untouched.
	configStorageUsed := false
	ConfigStorage.Save = func(data []byte) error { configStorageUsed = true; return nil }
	defer func() { ConfigStorage.Save = nil }()
	settingsStore.Save = func(r *Renderer, data []byte) error {
		t.Error("settingsStore.Save must not be used while ConfigStorage.Save is set")
		return nil
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "configsave"}, &i, 0); err != nil {
		t.Fatalf("configsave (ConfigStorage precedence): %v", err)
	}
	if !configStorageUsed {
		t.Error("expected ConfigStorage.Save to win over settingsStore.Save")
	}
}

func TestLoadSlotMissingFileReturnsError(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	if err := r.loadSlot(99); err == nil {
		t.Fatal("expected an error loading a slot that was never saved")
	}
}

func TestCheckpointRollback(t *testing.T) {
	checkpointData = nil
	r := newSaveTestRendererWithVars(t)
	r.callStack = []callFrame{{Storage: "a.ks", Index: 1}}

	tag := kag3.TagObject{Name: "checkpoint"}
	// i (not a separately-set currentScriptIndex) is what dispatchTag uses
	// to update currentScriptIndex for a depth-0 dispatch — see dispatch.go.
	i := 7
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("checkpoint dispatch error: %v", err)
	}
	if checkpointData == nil {
		t.Fatal("expected checkpointData to be set")
	}

	// Diverge state after the checkpoint...
	r.callStack = nil
	jumpIndex, isJump = 0, false

	rollback := kag3.TagObject{Name: "rollback"}
	if err := dispatchTag(r, fakeYield(), rollback, &i, 0); err != nil {
		t.Fatalf("rollback dispatch error: %v", err)
	}
	if len(r.callStack) != 1 || r.callStack[0].Index != 1 {
		t.Errorf("callStack after rollback = %+v, want restored from checkpoint", r.callStack)
	}
	if !isJump || jumpIndex != 7 {
		t.Errorf("isJump/jumpIndex after rollback = %v/%d, want true/7", isJump, jumpIndex)
	}

	clear := kag3.TagObject{Name: "clear_checkpoint"}
	if err := dispatchTag(r, fakeYield(), clear, &i, 0); err != nil {
		t.Fatalf("clear_checkpoint dispatch error: %v", err)
	}
	if checkpointData != nil {
		t.Error("expected clear_checkpoint to clear checkpointData")
	}
}

func TestRollbackWithNoCheckpointIsNoop(t *testing.T) {
	checkpointData = nil
	r := newTestRenderer()
	r.callStack = []callFrame{{Storage: "x.ks", Index: 9}}
	tag := kag3.TagObject{Name: "rollback"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("rollback dispatch error: %v", err)
	}
	if len(r.callStack) != 1 || r.callStack[0].Index != 9 {
		t.Errorf("callStack changed by a no-checkpoint rollback: %+v", r.callStack)
	}
}

func TestHandleDialogJumpsToTargetOnOK(t *testing.T) {
	activeDialog = nil
	r := newTestRenderer()
	r.labels = map[string]kag3.LabelInfo{
		"yes": {Index: 10},
		"no":  {Index: 20},
	}
	tag := kag3.TagObject{Name: "dialog", Pm: map[string]string{
		"text": "本当に終了しますか？", "target": "*yes", "false_target": "*no",
	}}
	i := 5
	y := func() bool {
		if activeDialog != nil && activeDialog.Result == 0 {
			activeDialog.Result = 1 // simulate an OK click
		}
		return true
	}
	if err := dispatchTag(r, y, tag, &i, 0); err != nil {
		t.Fatalf("dialog dispatch error: %v", err)
	}
	if i != 10 {
		t.Errorf("i after OK = %d, want 10 (target)", i)
	}
	if activeDialog != nil {
		t.Error("expected activeDialog to be cleared after the dialog resolves")
	}
}

func TestHandleDialogJumpsToFalseTargetOnNG(t *testing.T) {
	activeDialog = nil
	r := newTestRenderer()
	r.labels = map[string]kag3.LabelInfo{
		"yes": {Index: 10},
		"no":  {Index: 20},
	}
	tag := kag3.TagObject{Name: "dialog", Pm: map[string]string{
		"text": "本当に終了しますか？", "target": "*yes", "false_target": "*no",
	}}
	i := 5
	y := func() bool {
		if activeDialog != nil && activeDialog.Result == 0 {
			activeDialog.Result = 2 // simulate a Cancel click
		}
		return true
	}
	if err := dispatchTag(r, y, tag, &i, 0); err != nil {
		t.Fatalf("dialog dispatch error: %v", err)
	}
	if i != 20 {
		t.Errorf("i after NG = %d, want 20 (false_target)", i)
	}
}

func TestDialogConfigTagsUpdateLabels(t *testing.T) {
	dialogOKLabel, dialogNGLabel = "OK", "キャンセル"
	ok := kag3.TagObject{Name: "dialog_config_ok", Pm: map[string]string{"text": "はい"}}
	ng := kag3.TagObject{Name: "dialog_config_ng", Pm: map[string]string{"text": "いいえ"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), ok, &i, 0); err != nil {
		t.Fatalf("dialog_config_ok dispatch error: %v", err)
	}
	if err := dispatchTag(newTestRenderer(), fakeYield(), ng, &i, 0); err != nil {
		t.Fatalf("dialog_config_ng dispatch error: %v", err)
	}
	if dialogOKLabel != "はい" || dialogNGLabel != "いいえ" {
		t.Errorf("labels = %q/%q, want はい/いいえ", dialogOKLabel, dialogNGLabel)
	}
}

func TestKeyConfigAndCloseConfirmFlags(t *testing.T) {
	i := 0
	r := newTestRenderer()
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "stop_keyconfig"}, &i, 0); err != nil {
		t.Fatalf("stop_keyconfig error: %v", err)
	}
	if keyConfigEnabled {
		t.Error("expected keyConfigEnabled = false after stop_keyconfig")
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "start_keyconfig"}, &i, 0); err != nil {
		t.Fatalf("start_keyconfig error: %v", err)
	}
	if !keyConfigEnabled {
		t.Error("expected keyConfigEnabled = true after start_keyconfig")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "closeconfirm_on"}, &i, 0); err != nil {
		t.Fatalf("closeconfirm_on error: %v", err)
	}
	if !closeConfirmEnabled {
		t.Error("expected closeConfirmEnabled = true after closeconfirm_on")
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "closeconfirm_off"}, &i, 0); err != nil {
		t.Fatalf("closeconfirm_off error: %v", err)
	}
	if closeConfirmEnabled {
		t.Error("expected closeConfirmEnabled = false after closeconfirm_off")
	}
}

func TestHandleSaveSnapNoopWithoutRenderBuffer(t *testing.T) {
	renderBuffer = nil
	lastSnapshot = nil
	tag := kag3.TagObject{Name: "savesnap"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("savesnap dispatch error: %v", err)
	}
	if lastSnapshot != nil {
		t.Error("expected lastSnapshot to stay nil when renderBuffer hasn't been drawn yet")
	}
}

// TestCaptureSnapshotCopiesBuf covers the piece drawScene now calls
// automatically every frame (see its own doc comment in renderer.go): real
// Tyrano's bundled scripts, including this project's example/, never call
// [savesnap] themselves, so relying on it left every save slot
// thumbnail-less in practice. Only DrawImage here, deliberately no
// png.Encode(lastSnapshot) — that reads pixels back from the GPU and
// panics outside a running ebiten game loop (see
// TestSlotPickerRowsReflectSavedSlot's own note on the same constraint).
func TestCaptureSnapshotCopiesBuf(t *testing.T) {
	buf := newTestImage(4, 4)
	lastSnapshot = nil
	defer func() { lastSnapshot = nil }()

	captureSnapshot(buf)
	if lastSnapshot == nil {
		t.Fatal("expected captureSnapshot to populate lastSnapshot from buf")
	}
	if lastSnapshot == buf {
		t.Error("expected lastSnapshot to be an independent copy, not buf itself")
	}
}

// TestBuildSaveDataCapturesCurrentMessage covers the save slot's other new
// preview field: whatever's in the message window at save time.
func TestBuildSaveDataCapturesCurrentMessage(t *testing.T) {
	r := newSaveTestRendererWithVars(t)
	r.texts = map[int][]Text{0: {{Text: "帰るか"}}, 1: {{Text: "。。。"}}}
	data := r.buildSaveData()
	if data.LastMessage != "帰るか。。。" {
		t.Errorf("LastMessage = %q, want %q", data.LastMessage, "帰るか。。。")
	}
}

func TestAutoSaveAutoLoadUseReservedSlot(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	tag := kag3.TagObject{Name: "autosave"}
	i := 3
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("autosave dispatch error: %v", err)
	}

	jumpIndex, isJump = 0, false
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "autoload"}, &i, 0); err != nil {
		t.Fatalf("autoload dispatch error: %v", err)
	}
	if !isJump || jumpIndex != 3 {
		t.Errorf("isJump/jumpIndex after autoload = %v/%d, want true/3", isJump, jumpIndex)
	}
	// Confirm it didn't collide with the manual slot.
	if err := r.loadSlot(manualSaveSlot); err == nil {
		t.Error("expected no manual-slot save to exist yet")
	}
}

// TestApplySaveDataReconstructsCharaAfterFreshProcess is the crash this
// whole fix is for: loading a save right after starting a brand-new process
// (charas empty, none of the scenario's [chara_new] calls have run yet)
// must re-register the character from CharaStorage instead of leaving a
// dangling viewCharas entry for drawScene to crash on.
func TestApplySaveDataReconstructsCharaAfterFreshProcess(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	defer delete(charas, "akane")
	defer func() { viewCharas = nil }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{"akane.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	charas["akane"] = &kag3.Character{Name: "akane", Storage: "akane.png", Image: newTestImage(1, 1)}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 10, Top: 20}}

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate a fresh process: charas starts empty, nothing has run
	// [chara_new] yet.
	delete(charas, "akane")
	viewCharas = nil

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	got, ok := charas["akane"]
	if !ok || got.Image == nil {
		t.Fatalf("charas[akane] after load = %+v, want reconstructed with a non-nil Image", got)
	}
	if got.Storage != "akane.png" {
		t.Errorf("charas[akane].Storage = %q, want %q", got.Storage, "akane.png")
	}
	if len(viewCharas) != 1 || viewCharas[0].Name != "akane" {
		t.Errorf("viewCharas after load = %+v, want akane kept (not dropped)", viewCharas)
	}
}

// TestApplySaveDataRestoresCharaFacesAfterFreshProcess is the regression
// test for the actual reported crash: loading a save that resumes mid-
// script (jumpIndex straight to the saved position) skipped whatever
// [chara_face] tags ran earlier in the file, so charas["akane"].Faces only
// ever got reconstructed with a "default" entry — and the very next
// [chara_mod name="akane" face="happy"] found nothing, computed storage="",
// and panicked the whole coroutine (see handleCharaMod's fix). CharaFaces
// must round-trip through save/load so a non-default face works right after
// a fresh-process load, exactly like the real slot_4.json scenario.
func TestApplySaveDataRestoresCharaFacesAfterFreshProcess(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	defer delete(charas, "akane")
	defer func() { viewCharas = nil }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{
		"akane.png":       &fstest.MapFile{Data: tinyPNG(t)},
		"akane_happy.png": &fstest.MapFile{Data: tinyPNG(t)},
	}}
	charas["akane"] = &kag3.Character{
		Name:    "akane",
		Storage: "akane.png",
		Image:   newTestImage(1, 1),
		Faces:   map[string]string{"default": "akane.png", "happy": "akane_happy.png"},
	}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 10, Top: 20}}

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate a fresh process: charas starts empty, nothing has run
	// [chara_new]/[chara_face] yet — same as the real slot_4.json report.
	delete(charas, "akane")
	viewCharas = nil

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	if got := charas["akane"].Faces["happy"]; got != "akane_happy.png" {
		t.Fatalf(`charas["akane"].Faces["happy"] = %q, want "akane_happy.png"`, got)
	}

	tag := kag3.TagObject{Name: "chara_mod", Pm: map[string]string{"name": "akane", "face": "happy"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_mod after fresh-process load errored (this is the reported crash): %v", err)
	}
	if charas["akane"].Storage != "akane_happy.png" {
		t.Errorf(`charas["akane"].Storage after chara_mod = %q, want "akane_happy.png"`, charas["akane"].Storage)
	}
}

// TestReconcileViewCharasDropsUnrestorableCharaWithoutPanicking covers the
// fallback when a character can't be reconstructed at all (no recorded
// path, or the file no longer exists) — it must be silently dropped from
// viewCharas, never left dangling for drawScene to panic on.
func TestReconcileViewCharasDropsUnrestorableCharaWithoutPanicking(t *testing.T) {
	delete(charas, "ghost")
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"images": fstest.MapFS{}}
	restored := []*kag3.CharaShow{{Name: "ghost", Left: 5}}

	got := reconcileViewCharas(r, restored, map[string]string{}, nil) // no CharaStorage entry at all
	if len(got) != 0 {
		t.Errorf("reconcileViewCharas with no storage path = %+v, want dropped (empty)", got)
	}

	got = reconcileViewCharas(r, restored, map[string]string{"ghost": "missing.png"}, nil) // path given but file absent
	if len(got) != 0 {
		t.Errorf("reconcileViewCharas with an unreadable path = %+v, want dropped (empty)", got)
	}
	if _, ok := charas["ghost"]; ok {
		t.Error("expected charas[ghost] to stay unregistered after a failed reconstruction")
	}
}

// TestApplySaveDataReconstructsBackgroundAfterFreshProcess mirrors the
// character fix for bg: a fresh process's bg.Image starts as NewRenderer's
// solid-black placeholder, and load should replace it with the actual saved
// background rather than leaving that placeholder in place.
func TestApplySaveDataReconstructsBackgroundAfterFreshProcess(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{"bg.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	bg = &kag3.Background{Time: 3000, Method: "crossfade", Storage: "bg.png", Image: newTestImage(1, 1)}

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate a fresh process: NewRenderer's default solid-black fill, no
	// Storage tracked yet.
	placeholder := newTestImage(1, 1)
	bg = &kag3.Background{Image: placeholder}

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}
	if bg.Storage != "bg.png" {
		t.Errorf("bg.Storage after load = %q, want %q", bg.Storage, "bg.png")
	}
	if bg.Image == nil || bg.Image == placeholder {
		t.Error("expected bg.Image to be replaced with the reloaded background, not left as the placeholder")
	}
	if bg.Method != "crossfade" {
		t.Errorf("bg.Method after load = %q, want %q", bg.Method, "crossfade")
	}
}

// TestApplySaveDataReconstructsMessageWindowAfterFreshProcess covers the
// reported bug: applySaveData resumes execution via jumpIndex straight into
// the middle of a script, skipping whatever one-time [position]/[layopt]
// setup configured the message window earlier in the file. On a fresh
// process textPosition starts at its zero value (Visible=false, Width=
// Height=0, no BackImage), so without restoring it from the save, the
// message box and its text never appear again after a load.
func TestApplySaveDataReconstructsMessageWindowAfterFreshProcess(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{"frame.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	textPosition = &kag3.TextPosition{
		Visible: true, Left: 160, Top: 500, Width: 1000, Height: 200,
		MarginLeft: 50, MarginTop: 45, MarginRight: 70, MarginBottom: 60,
		FrameStorage: "frame.png",
	}

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate a fresh process: NewRenderer's zero-valued textPosition, no
	// [position]/[layopt] setup tags have run yet.
	textPosition = &kag3.TextPosition{}

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}
	if !textPosition.Visible {
		t.Error("expected textPosition.Visible = true after load")
	}
	if textPosition.Width != 1000 || textPosition.Height != 200 {
		t.Errorf("textPosition.Width/Height after load = %d/%d, want 1000/200", textPosition.Width, textPosition.Height)
	}
	if textPosition.MarginLeft != 50 || textPosition.MarginTop != 45 {
		t.Errorf("textPosition margins after load = %+v, want MarginLeft=50 MarginTop=45", textPosition)
	}
	if textPosition.BackImage == nil {
		t.Error("expected BackImage to be regenerated at the restored Width/Height")
	}
	if textPosition.FrameImage == nil {
		t.Error("expected FrameImage to be reloaded from FrameStorage")
	}
}

// TestApplySaveDataFromOldSaveFormatDoesNotClobberTextPosition guards the
// backward-compat path: a save file written before TextPosition existed in
// saveData unmarshals it as the Go zero value, which must not stomp a
// same-process textPosition that's already correctly configured.
func TestApplySaveDataFromOldSaveFormatDoesNotClobberTextPosition(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}
	// buildSaveData always populates TextPosition now, so hand-craft the old
	// (pre-fix) on-disk shape directly instead — a JSON object missing the
	// "TextPosition" key entirely, as any save written before this field
	// existed would be.
	dir, err := saveDir(r)
	if err != nil {
		t.Fatalf("saveDir error: %v", err)
	}
	if err := os.WriteFile(slotPath(dir, manualSaveSlot, "json"), []byte(`{"Storage":"scene1.ks","Index":42}`), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	textPosition = &kag3.TextPosition{Visible: true, Width: 1000, Height: 200}
	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}
	if !textPosition.Visible || textPosition.Width != 1000 {
		t.Errorf("textPosition after loading an old-format save = %+v, want unchanged (Visible=true Width=1000)", textPosition)
	}
}

func TestRecordBacklogCapturesNameAndText(t *testing.T) {
	backlog, backlogPaused = nil, false
	defer func() { backlog, backlogPaused, charaName = nil, false, "" }()

	r := newTestRenderer()
	r.texts = map[int][]Text{0: {{Text: "こんにちは"}}}
	charaName = "凪"
	recordBacklog(r)

	if len(backlog) != 1 {
		t.Fatalf("backlog = %+v, want 1 entry", backlog)
	}
	if backlog[0].Name != "凪" || backlog[0].Text != "こんにちは" {
		t.Errorf("backlog[0] = %+v, want Name=凪 Text=こんにちは", backlog[0])
	}

	// Monologue (no speaker): Name stays empty.
	r.texts = map[int][]Text{0: {{Text: "静かな部屋"}}}
	charaName = ""
	recordBacklog(r)
	if len(backlog) != 2 || backlog[1].Name != "" || backlog[1].Text != "静かな部屋" {
		t.Errorf("backlog[1] = %+v, want Name=\"\" Text=静かな部屋", backlog[1])
	}
}

func TestHandlePushLogAppendsNamelessEntry(t *testing.T) {
	backlog = nil
	defer func() { backlog = nil }()

	tag := kag3.TagObject{Name: "pushlog", Pm: map[string]string{"text": "manual entry"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(backlog) != 1 || backlog[0].Name != "" || backlog[0].Text != "manual entry" {
		t.Errorf("backlog = %+v, want one nameless entry \"manual entry\"", backlog)
	}
}

func TestBacklogScrollClampAndWraparound(t *testing.T) {
	backlog = make([]backlogEntry, 10)
	defer func() { backlog, backlogScrollY = nil, 0 }()

	const viewportH = 100.0 // fits fewer than 10 rows at backlogEntryHeight each

	backlogScrollY = -50
	clampBacklogScroll(viewportH)
	if backlogScrollY != 0 {
		t.Errorf("scrollY after clamping a negative value = %v, want 0", backlogScrollY)
	}

	backlogScrollY = 999999
	clampBacklogScroll(viewportH)
	want := backlogMaxScroll(viewportH)
	if backlogScrollY != want {
		t.Errorf("scrollY after clamping an overlarge value = %v, want max %v", backlogScrollY, want)
	}
}

func TestBacklogNameDisplayFallsBackToNarrationDash(t *testing.T) {
	if name, _ := backlogNameDisplay("凪"); name != "凪" {
		t.Errorf("backlogNameDisplay(凪) name = %q, want 凪", name)
	}
	if name, col := backlogNameDisplay(""); name != "──" || col != backlogNarrationColor {
		t.Errorf("backlogNameDisplay(\"\") = %q/%v, want ──/%v", name, col, backlogNarrationColor)
	}
}

func TestDrawBacklogNoPanic(t *testing.T) {
	savedBacklog, savedScrollY := backlog, backlogScrollY
	defer func() { backlog, backlogScrollY = savedBacklog, savedScrollY }()

	backlog = []backlogEntry{
		{Name: "凪", Text: "こんにちは"},
		{Text: "静かな部屋だった"},
	}
	backlogScrollY = 0
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config = &kag3.Config{ScreenWidth: 1920, ScreenHeight: 1080}
	buf := newTestImage(1920, 1080)
	drawBacklog(r, buf) // must not panic
}

// TestUpdateBacklogDragDistinguishesClickFromDrag is the regression test
// for a real bug: handleBacklogClick's "click outside the backlog closes
// it" behavior used backlogDragging to decide "was this a click or a
// drag," but backlogDragging goes true on a plain click's very first
// pressed frame too (pressed starts false, so that frame always takes the
// "not currently dragging -> start dragging" branch) — before
// handleBacklogClick's own justPressed check ever ran. That made the
// dismiss branch permanently unreachable: no click, no matter where it
// landed, could ever close the backlog by clicking outside it. Confirmed
// by reproducing the exact one-frame sequence a plain click produces
// before this fix existed.
//
// backlogDidDrag is the fix: it only ever becomes true once the pointer
// actually moves while held down, which this test drives directly.
func TestUpdateBacklogDragDistinguishesClickFromDrag(t *testing.T) {
	savedDragging, savedDidDrag, savedLastY, savedScrollY := backlogDragging, backlogDidDrag, backlogDragLastY, backlogScrollY
	defer func() {
		backlogDragging, backlogDidDrag, backlogDragLastY, backlogScrollY = savedDragging, savedDidDrag, savedLastY, savedScrollY
	}()

	t.Run("a plain click that never moves must not be classified as a drag", func(t *testing.T) {
		backlogDragging, backlogDidDrag, backlogDragLastY = false, false, 0

		// Frame 1: press begins (this is also justPressed's frame in the
		// real Update() loop). No movement yet.
		if delta := updateBacklogDrag(true, 500); delta != 0 {
			t.Errorf("scrollDelta on press-begin = %v, want 0 (no movement yet)", delta)
		}
		if backlogDidDrag {
			t.Fatal("BUG: backlogDidDrag is already true on the very first pressed frame — a click could never dismiss the backlog by clicking outside it, since handleBacklogClick's dismiss check runs on this same justPressed frame")
		}

		// Frame 2: released, no movement ever happened.
		if delta := updateBacklogDrag(false, 500); delta != 0 {
			t.Errorf("scrollDelta on release = %v, want 0", delta)
		}
		if backlogDidDrag {
			t.Error("backlogDidDrag became true on release with no movement, want it to stay false for a plain click")
		}
	})

	t.Run("an actual drag is still detected and scrolled", func(t *testing.T) {
		backlogDragging, backlogDidDrag, backlogDragLastY = false, false, 0

		updateBacklogDrag(true, 500) // press begins at y=500
		// Pointer moves up by 30px (dragging up reveals newer entries —
		// backlogScrollY decreases, matching the field's own doc comment
		// on the opposite direction).
		delta := updateBacklogDrag(true, 470)
		if delta != 30 {
			t.Errorf("scrollDelta after moving from y=500 to y=470 = %v, want 30 (backlogDragLastY - mY)", delta)
		}
		if !backlogDidDrag {
			t.Error("backlogDidDrag = false after the pointer actually moved while held, want true")
		}
	})

	t.Run("a fresh press resets backlogDidDrag from a previous drag", func(t *testing.T) {
		backlogDragging, backlogDidDrag, backlogDragLastY = false, true, 0 // leftover true from a prior drag

		updateBacklogDrag(true, 300) // a brand new press-begin frame
		if backlogDidDrag {
			t.Error("backlogDidDrag = true on a fresh press-begin frame, want it reset to false regardless of what a previous gesture left it at")
		}
	})
}

func TestHandleBacklogClickClosesOnCloseButton(t *testing.T) {
	savedBacklogViewing, savedOpenedFrame := backlogViewing, backlogOpenedFrame
	savedDragging, savedDidDrag := backlogDragging, backlogDidDrag
	defer func() {
		backlogViewing, backlogOpenedFrame = savedBacklogViewing, savedOpenedFrame
		backlogDragging, backlogDidDrag = savedDragging, savedDidDrag
	}()

	backlogViewing = true
	backlogOpenedFrame = t2Sentinel() // definitely not the current frame
	backlogDragging, backlogDidDrag = false, false
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config = &kag3.Config{ScreenWidth: 1920, ScreenHeight: 1080}

	r.handleBacklogClick() // no real click state in a headless test: must not panic and must not force-close
}

// t2Sentinel returns a frame counter value guaranteed to differ from the
// package-level tick t at call time, for tests that need backlogOpenedFrame
// to definitely not match the current frame.
func t2Sentinel() int {
	return t - 1000000
}
