package ebitengine

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// tinyPNG encodes a minimal valid 1x1 PNG in memory so chara_layer-style
// tests can exercise real image decoding without a checked-in fixture.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// tinyGIF encodes a minimal valid 1x1 GIF, for TestGIFImagesDecode below.
func tinyGIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.White})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test GIF: %v", err)
	}
	return buf.Bytes()
}

// TestGIFImagesDecode covers a real reachable asset in the bundled example
// project: config.ks's volume/speed slider buttons use
// tf.btn_path_off = tf.img_path + 'c_btn.gif' as their graphic, and Go's
// image.Decode (which ebitenutil.NewImageFromFileSystem uses under the
// hood) only supports formats whose package has been blank-imported
// somewhere — renderer.go registers jpeg/png but, until now, not gif,
// so this would have failed with "image: unknown format" the moment that
// button was drawn.
func TestGIFImagesDecode(t *testing.T) {
	mapFS := fstest.MapFS{"c_btn.gif": &fstest.MapFile{Data: tinyGIF(t)}}
	if _, _, err := ebitenutil.NewImageFromFileSystem(mapFS, "c_btn.gif"); err != nil {
		t.Errorf("decoding a .gif failed: %v (is image/gif blank-imported in renderer.go?)", err)
	}
}

func newTestRendererWithImageFS(t *testing.T, files map[string][]byte) *Renderer {
	t.Helper()
	mapFS := fstest.MapFS{}
	for name, data := range files {
		mapFS[name] = &fstest.MapFile{Data: data}
	}
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"images": mapFS}
	return r
}

func TestHandleCharaHideAll(t *testing.T) {
	viewCharas = []*kag3.CharaShow{{Name: "a"}, {Name: "b"}}
	tag := kag3.TagObject{Name: "chara_hide_all"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	for _, c := range viewCharas {
		if !c.IsRemove {
			t.Errorf("expected %s.IsRemove = true", c.Name)
		}
	}
}

func TestHandleCharaDelete(t *testing.T) {
	charas["akane"] = &kag3.Character{Name: "akane"}
	viewCharas = []*kag3.CharaShow{{Name: "akane"}, {Name: "yamato"}}

	tag := kag3.TagObject{Name: "chara_delete", Pm: map[string]string{"name": "akane"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if _, ok := charas["akane"]; ok {
		t.Error("expected akane's definition to be deleted")
	}
	if len(viewCharas) != 1 || viewCharas[0].Name != "yamato" {
		t.Errorf("viewCharas = %+v, want only yamato remaining", viewCharas)
	}
}

func TestHandleCharaMoveMissingCharacterErrors(t *testing.T) {
	viewCharas = nil
	tag := kag3.TagObject{Name: "chara_move", Pm: map[string]string{"name": "nobody", "left": "10"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for a character that isn't currently shown")
	}
}

func TestHandleCharaMoveSetsSlideTarget(t *testing.T) {
	c := &kag3.CharaShow{Name: "akane", Left: 100, Top: 50}
	viewCharas = []*kag3.CharaShow{c}

	// wait=false so this only checks the state change, not the blocking
	// path (covered separately below, since it depends on the
	// package-level tick counter "t" which fakeYield never advances).
	tag := kag3.TagObject{Name: "chara_move", Pm: map[string]string{
		"name": "akane", "left": "300", "top": "60", "time": "500", "wait": "false",
	}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if c.NewLeft != 300 {
		t.Errorf("NewLeft = %d, want 300", c.NewLeft)
	}
	if c.Top != 60 {
		t.Errorf("Top = %d, want 60", c.Top)
	}
	if c.Time != 500 {
		t.Errorf("Time = %d, want 500", c.Time)
	}
	if !c.IsSlide {
		t.Error("expected IsSlide = true")
	}
}

// TestHandleCharaMoveWaitBlocksUntilElapsed uses "tt" instead of "t" for the
// *testing.T parameter because this test needs to advance the
// package-level tick counter "t" that the wait=true path compares against
// (same reason as TestHandleWTCompletesWhenElapsed).
func TestHandleCharaMoveWaitBlocksUntilElapsed(tt *testing.T) {
	c := &kag3.CharaShow{Name: "akane"}
	viewCharas = []*kag3.CharaShow{c}
	t = 0

	y := coro.Yield(func() bool {
		t++
		return true
	})
	tag := kag3.TagObject{Name: "chara_move", Pm: map[string]string{
		"name": "akane", "left": "300", "time": "100", "wait": "true",
	}}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if t <= 0 {
		tt.Errorf("expected t to have advanced past 0, got %d", t)
	}
}

func TestHandleCharaConfigSetsNamePText(t *testing.T) {
	charaNamePText = ""
	tag := kag3.TagObject{Name: "chara_config", Pm: map[string]string{"ptext": "chara_name_area"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if charaNamePText != "chara_name_area" {
		t.Errorf("charaNamePText = %q, want %q", charaNamePText, "chara_name_area")
	}
}

func TestPTextContentResolvesNamePlateVsLiteral(t *testing.T) {
	ptexts = map[string]*kag3.PText{
		"chara_name_area": {Name: "chara_name_area", Text: "should not be shown"},
		"clock":           {Name: "clock", Text: "12:00"},
	}
	charaNamePText = "chara_name_area"
	charaName = "akane"

	if got := ptextContent("chara_name_area"); got != "akane" {
		t.Errorf("ptextContent(chara_name_area) = %q, want %q", got, "akane")
	}
	if got := ptextContent("clock"); got != "12:00" {
		t.Errorf("ptextContent(clock) = %q, want %q", got, "12:00")
	}
	if got := ptextContent("missing"); got != "" {
		t.Errorf("ptextContent(missing) = %q, want empty", got)
	}
}

func TestHandlePTextStoresByNameWithoutClobbering(t *testing.T) {
	ptexts = map[string]*kag3.PText{}
	r := newTestRenderer()

	first := kag3.TagObject{Name: "ptext", Pm: map[string]string{"name": "a", "text": "hello", "x": "1", "y": "2"}}
	second := kag3.TagObject{Name: "ptext", Pm: map[string]string{"name": "b", "text": "world", "x": "3", "y": "4"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), first, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if err := dispatchTag(r, fakeYield(), second, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(ptexts) != 2 {
		t.Fatalf("ptexts = %+v, want 2 entries", ptexts)
	}
	if ptexts["a"].Text != "hello" || ptexts["b"].Text != "world" {
		t.Errorf("ptexts = %+v, want a=hello b=world", ptexts)
	}
}

func TestHandlePTextBgLoadsBackgroundImage(t *testing.T) {
	ptexts = map[string]*kag3.PText{}
	r := newTestRendererWithImageFS(t, map[string][]byte{"ui/name_tab.png": tinyPNG(t)})

	tag := kag3.TagObject{Name: "ptext", Pm: map[string]string{"name": "chara_name_area", "text": "凪", "bg": "ui/name_tab.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	pt, ok := ptexts["chara_name_area"]
	if !ok {
		t.Fatal("expected chara_name_area to be registered")
	}
	if pt.BgStorage != "ui/name_tab.png" {
		t.Errorf("BgStorage = %q, want ui/name_tab.png", pt.BgStorage)
	}
	if pt.BgImage == nil {
		t.Error("expected BgImage to be loaded")
	}
}

func TestHandlePTextWithoutBgLeavesBgImageNil(t *testing.T) {
	ptexts = map[string]*kag3.PText{}
	r := newTestRenderer()

	tag := kag3.TagObject{Name: "ptext", Pm: map[string]string{"name": "plain", "text": "hello"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if pt := ptexts["plain"]; pt.BgImage != nil || pt.BgStorage != "" {
		t.Errorf("ptexts[plain] = %+v, want BgImage/BgStorage unset when bg= is not given", pt)
	}
}

func TestHandlePTextBgMissingFileErrors(t *testing.T) {
	ptexts = map[string]*kag3.PText{}
	r := newTestRendererWithImageFS(t, map[string][]byte{})

	tag := kag3.TagObject{Name: "ptext", Pm: map[string]string{"name": "chara_name_area", "text": "凪", "bg": "does_not_exist.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error when bg= names a file that doesn't exist")
	}
}

func TestCharaLayerPartAndReset(t *testing.T) {
	charas["akane"] = &kag3.Character{Name: "akane"}
	r := newTestRendererWithImageFS(t, map[string][]byte{"face_smile.png": tinyPNG(t)})

	layerTag := kag3.TagObject{Name: "chara_layer", Pm: map[string]string{
		"name": "akane", "layer": "face", "part": "smile", "storage": "face_smile.png",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), layerTag, &i, 0); err != nil {
		t.Fatalf("chara_layer error: %v", err)
	}
	if _, ok := charas["akane"].Parts["face"]["smile"]; !ok {
		t.Fatal("expected part image to be registered under Parts[face][smile]")
	}

	partTag := kag3.TagObject{Name: "chara_part", Pm: map[string]string{
		"name": "akane", "layer": "face", "part": "smile",
	}}
	if err := dispatchTag(r, fakeYield(), partTag, &i, 0); err != nil {
		t.Fatalf("chara_part error: %v", err)
	}
	if charas["akane"].ActivePart["face"] != "smile" {
		t.Errorf("ActivePart[face] = %q, want %q", charas["akane"].ActivePart["face"], "smile")
	}

	resetOne := kag3.TagObject{Name: "chara_part_reset", Pm: map[string]string{"name": "akane", "layer": "face"}}
	if err := dispatchTag(r, fakeYield(), resetOne, &i, 0); err != nil {
		t.Fatalf("chara_part_reset error: %v", err)
	}
	if _, ok := charas["akane"].ActivePart["face"]; ok {
		t.Error("expected face layer to be cleared")
	}
}

func TestCharaLayerModUsesSameHandler(t *testing.T) {
	charas["akane"] = &kag3.Character{Name: "akane"}
	r := newTestRendererWithImageFS(t, map[string][]byte{"face_sad.png": tinyPNG(t)})

	tag := kag3.TagObject{Name: "chara_layer_mod", Pm: map[string]string{
		"name": "akane", "layer": "face", "part": "sad", "storage": "face_sad.png",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_layer_mod error: %v", err)
	}
	if _, ok := charas["akane"].Parts["face"]["sad"]; !ok {
		t.Error("expected chara_layer_mod to register the part the same way chara_layer does")
	}
}

func TestCharaPartResetAllLayers(t *testing.T) {
	charas["akane"] = &kag3.Character{
		Name:       "akane",
		ActivePart: map[string]string{"face": "smile", "accessory": "glasses"},
	}
	tag := kag3.TagObject{Name: "chara_part_reset", Pm: map[string]string{"name": "akane"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(charas["akane"].ActivePart) != 0 {
		t.Errorf("ActivePart = %+v, want empty after reset with no layer=", charas["akane"].ActivePart)
	}
}

func TestDrawCharaPartsHandlesNilAndEmpty(t *testing.T) {
	// Must not panic.
	drawCharaParts(nil, nil, 0, 0)
	drawCharaParts(nil, &kag3.Character{Name: "akane"}, 0, 0)
}

// TestHandleCharaNewSetsStorage confirms [chara_new] records the image path
// it loaded, not just the decoded image — save/load (tags_save.go) needs it
// to re-register the character in a fresh process.
func TestHandleCharaNewSetsStorage(t *testing.T) {
	delete(charas, "akane")
	r := newTestRendererWithImageFS(t, map[string][]byte{"akane.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "chara_new", Pm: map[string]string{"name": "akane", "storage": "akane.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_new dispatch error: %v", err)
	}
	if charas["akane"].Storage != "akane.png" {
		t.Errorf("charas[akane].Storage = %q, want %q", charas["akane"].Storage, "akane.png")
	}
	if charas["akane"].Faces["default"] != "akane.png" {
		t.Errorf("charas[akane].Faces[default] = %q, want %q", charas["akane"].Faces["default"], "akane.png")
	}
}

// TestHandleCharaModUpdatesStorage confirms a face swap keeps Storage
// pointing at whatever's actually showing, so a save made afterward
// restores that face rather than the original [chara_new] default.
func TestHandleCharaModUpdatesStorage(t *testing.T) {
	r := newTestRendererWithImageFS(t, map[string][]byte{
		"akane.png":       tinyPNG(t),
		"akane_smile.png": tinyPNG(t),
	})
	charas["akane"] = &kag3.Character{
		Name:    "akane",
		Storage: "akane.png",
		Faces:   map[string]string{"default": "akane.png", "smile": "akane_smile.png"},
	}
	tag := kag3.TagObject{Name: "chara_mod", Pm: map[string]string{"name": "akane", "face": "smile"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_mod dispatch error: %v", err)
	}
	if charas["akane"].Storage != "akane_smile.png" {
		t.Errorf("charas[akane].Storage = %q, want %q", charas["akane"].Storage, "akane_smile.png")
	}
}

// TestHandleCharaFaceUnregisteredNameReportsError is the regression test for
// a nil-pointer crash: [chara_face] used to index charas directly
// (charas[name].Faces[...] = ...), so any name= that wasn't registered — a
// typo, or a [chara_face] reached before its own [chara_new] — dereferenced
// nil and took the whole game down with an opaque runtime error instead of
// naming the missing character. Every other [chara_*] handler resolves
// through mustChara; this one now does too.
func TestHandleCharaFaceUnregisteredNameReportsError(t *testing.T) {
	delete(charas, "nosuch")
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "chara_face", Pm: map[string]string{
		"name": "nosuch", "face": "happy", "storage": "happy.png",
	}}
	i := 0
	// A panic fails the test on its own; the error is what should come back.
	err := dispatchTag(r, fakeYield(), tag, &i, 0)
	if err == nil {
		t.Fatal("chara_face on an unregistered name returned nil, want an error naming the character")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error = %q, want it to name the missing character", err)
	}
}

// TestHandleCharaFaceRegistersOntoNilFacesMap covers a *kag3.Character built
// without a Faces map (nothing constructs one that way today, but assigning
// into a nil map is the same class of crash the fix above removes).
func TestHandleCharaFaceRegistersOntoNilFacesMap(t *testing.T) {
	charas["akane"] = &kag3.Character{Name: "akane"} // Faces deliberately nil
	defer delete(charas, "akane")
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "chara_face", Pm: map[string]string{
		"name": "akane", "face": "happy", "storage": "akane_happy.png",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_face dispatch error: %v", err)
	}
	if got := charas["akane"].Faces["happy"]; got != "akane_happy.png" {
		t.Errorf("Faces[happy] = %q, want %q", got, "akane_happy.png")
	}
}

// TestHandleCharaModMissingFaceDoesNotError is the regression test for a
// real reported crash: loading a save that resumes mid-script (jumpIndex
// straight to the saved position, see the save/load gaps note in
// tags_save.go) skips whatever [chara_face] declarations came earlier in
// the file, so charas["akane"].Faces["happy"] doesn't exist even though the
// character is registered (reconcileViewCharas only seeds a "default"
// face). Before this, that missing lookup silently became storage="" ,
// which ebitenutil.NewImageFromFileSystem turned into an "open : invalid
// argument" error — and initScript's loop panics on any error a tag handler
// returns, killing the whole coroutine. It must now log and leave the
// character's current image untouched instead.
func TestHandleCharaModMissingFaceDoesNotError(t *testing.T) {
	r := newTestRendererWithImageFS(t, map[string][]byte{"akane.png": tinyPNG(t)})
	charas["akane"] = &kag3.Character{
		Name:    "akane",
		Storage: "akane.png",
		Faces:   map[string]string{"default": "akane.png"},
	}
	origImage := charas["akane"].Image
	tag := kag3.TagObject{Name: "chara_mod", Pm: map[string]string{"name": "akane", "face": "happy"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_mod dispatch error = %v, want nil (missing face should be logged and skipped)", err)
	}
	if charas["akane"].Storage != "akane.png" {
		t.Errorf("charas[akane].Storage = %q, want unchanged %q", charas["akane"].Storage, "akane.png")
	}
	if charas["akane"].Image != origImage {
		t.Error("charas[akane].Image changed even though the requested face was never registered")
	}
}

// TestHandleCharaModMissingCharacterDoesNotError covers the other half: a
// [chara_mod] for a name that was never [chara_new]'d (or was deleted via
// [chara_delete]) must not panic on charas[name] being nil either.
func TestHandleCharaModMissingCharacterDoesNotError(t *testing.T) {
	delete(charas, "nobody")
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "chara_mod", Pm: map[string]string{"name": "nobody", "face": "happy"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_mod dispatch error = %v, want nil (unregistered character should be logged and skipped)", err)
	}
}

// TestHandleCharaPTextSetsCharaName covers [chara_ptext name=]'s core
// effect: it assigns the package-level charaName exactly like a "#name"
// scenario line does (parser.go's characterPText + macro.go's execItem),
// letting scripts trigger the name-plate/fuki-positioning update explicitly
// instead of only via that shorthand.
func TestHandleCharaPTextSetsCharaName(t *testing.T) {
	defer func() { charaName = "" }()
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "chara_ptext", Pm: map[string]string{"name": "akane"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_ptext dispatch error: %v", err)
	}
	if charaName != "akane" {
		t.Errorf("charaName = %q, want %q", charaName, "akane")
	}
}

// TestHandleCharaPTextEmptyNameClearsMonologue covers [chara_ptext name=""]
// (or name= omitted) clearing charaName the same way a bare "#" line does —
// ptextContent (draw_ptext.go) then resolves the name-plate to empty and
// skips drawing it entirely.
func TestHandleCharaPTextEmptyNameClearsMonologue(t *testing.T) {
	charaName = "akane"
	defer func() { charaName = "" }()
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "chara_ptext", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_ptext dispatch error: %v", err)
	}
	if charaName != "" {
		t.Errorf("charaName = %q, want cleared", charaName)
	}
}

// TestHandleCharaPTextFaceSwapsImage covers [chara_ptext face=]: the same
// face-swap path [chara_mod face=] uses (applyCharaFace), not the
// differential-parts system ([chara_part]) — matching upstream Tyrano's
// documented scope ("表情差分パーツ機能（[chara_part]タグ）には対応していません").
func TestHandleCharaPTextFaceSwapsImage(t *testing.T) {
	defer func() { charaName = "" }()
	r := newTestRendererWithImageFS(t, map[string][]byte{
		"akane.png":       tinyPNG(t),
		"akane_smile.png": tinyPNG(t),
	})
	charas["akane"] = &kag3.Character{
		Name:    "akane",
		Storage: "akane.png",
		Faces:   map[string]string{"default": "akane.png", "smile": "akane_smile.png"},
	}
	tag := kag3.TagObject{Name: "chara_ptext", Pm: map[string]string{"name": "akane", "face": "smile"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_ptext dispatch error: %v", err)
	}
	if charas["akane"].Storage != "akane_smile.png" {
		t.Errorf("charas[akane].Storage = %q, want %q", charas["akane"].Storage, "akane_smile.png")
	}
}

// TestHandleCharaPTextMissingFaceDoesNotError mirrors
// TestHandleCharaModMissingFaceDoesNotError: a face= that was never
// registered via [chara_face] must be logged and skipped, not crash the
// coroutine (a real gap after a save/load resume — see that test's own
// comment for the full scenario this guards against).
func TestHandleCharaPTextMissingFaceDoesNotError(t *testing.T) {
	defer func() { charaName = "" }()
	r := newTestRendererWithImageFS(t, map[string][]byte{"akane.png": tinyPNG(t)})
	charas["akane"] = &kag3.Character{
		Name:    "akane",
		Storage: "akane.png",
		Faces:   map[string]string{"default": "akane.png"},
	}
	origImage := charas["akane"].Image
	tag := kag3.TagObject{Name: "chara_ptext", Pm: map[string]string{"name": "akane", "face": "happy"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("chara_ptext dispatch error = %v, want nil (missing face should be logged and skipped)", err)
	}
	if charas["akane"].Image != origImage {
		t.Error("charas[akane].Image changed even though the requested face was never registered")
	}
}
