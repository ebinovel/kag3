package ebitengine

import (
	"os"
	"testing"

	"github.com/ebinovel/kag3"
)

// groupLayerPSDBytes returns testdata/chara_psd/group_layer.psd — a tiny
// (64x64) MIT-licensed fixture borrowed from github.com/oov/psd's own test
// suite specifically because it already has the folder/nested-folder layer
// structure real character PSDs use:
//
//	レイヤー 0                          (top level)
//	レイヤー 1                          (top level)
//	グループ 1/レイヤー 2               (inside one folder)
//	グループ 1/グループ 2/レイヤー 3    (inside a nested folder)
func groupLayerPSDBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/chara_psd/group_layer.psd")
	if err != nil {
		t.Fatalf("failed to read testdata fixture: %v", err)
	}
	return b
}

// groupLayerPFV is a PSDToolFavorites file whose two presets exercise both
// a top-level+one-folder-deep combination (PoseA) and a top-level+
// nested-folder combination (PoseB), matching groupLayerPSDBytes's actual
// layer paths exactly.
const groupLayerPFV = `
[PSDToolFavorites-v1]
root-name/テスト
faview-mode/1

//PoseA
レイヤー 0
グループ 1/レイヤー 2

//PoseB
レイヤー 1
グループ 1/グループ 2/レイヤー 3
`

func newTestRendererWithPSDFS(t *testing.T) *Renderer {
	t.Helper()
	return newTestRendererWithImageFS(t, map[string][]byte{
		"chara_psd/group_layer.psd": groupLayerPSDBytes(t),
		"chara_psd/test.pfv":        []byte(groupLayerPFV),
	})
}

// resetPSDFaceCache clears the process-lifetime psdFaceGroups cache so a
// test can force loadPSDFaceGroup to actually regenerate from storage=/
// favorite= again, simulating what a fresh process (that never ran
// [chara_new_psd] itself) has to do on a save/load.
func resetPSDFaceCache(t *testing.T) {
	t.Helper()
	psdFaceGroups = map[string]*psdFaceGroup{}
}

func TestHandleCharaNewPSDRegistersEveryPresetAsAFace(t *testing.T) {
	resetPSDFaceCache(t)
	r := newTestRendererWithPSDFS(t)
	tag := kag3.TagObject{Name: "chara_new_psd", Pm: map[string]string{
		"name":     "psdchara1",
		"storage":  "chara_psd/group_layer.psd",
		"favorite": "chara_psd/test.pfv",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}

	c, ok := charas["psdchara1"]
	if !ok {
		t.Fatal("charas[psdchara1] was not registered")
	}
	for _, want := range []string{"PoseA", "PoseB", "default"} {
		if _, ok := c.Faces[want]; !ok {
			t.Errorf("Faces[%q] missing, got %v", want, c.Faces)
		}
	}
	if c.Image == nil {
		t.Fatal("Image is nil")
	}
	if b := c.Image.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Errorf("Image bounds = %v, want 64x64 (the PSD canvas size)", b)
	}
	// face= was omitted, so the default face must be the .pfv file's first
	// preset in file order (PoseA) — not whichever preset ppi.CreateImage's
	// internal map iteration happened to return first.
	if c.Faces["default"] != c.Faces["PoseA"] {
		t.Errorf("Faces[default] = %q, want it to match Faces[PoseA] = %q (first preset in file order)", c.Faces["default"], c.Faces["PoseA"])
	}
	if c.Storage != c.Faces["default"] {
		t.Errorf("Storage = %q, want it to match Faces[default] = %q", c.Storage, c.Faces["default"])
	}
}

func TestHandleCharaNewPSDExplicitFace(t *testing.T) {
	resetPSDFaceCache(t)
	r := newTestRendererWithPSDFS(t)
	tag := kag3.TagObject{Name: "chara_new_psd", Pm: map[string]string{
		"name":     "psdchara2",
		"storage":  "chara_psd/group_layer.psd",
		"favorite": "chara_psd/test.pfv",
		"face":     "PoseB",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	c := charas["psdchara2"]
	if c.Faces["default"] != c.Faces["PoseB"] {
		t.Errorf("Faces[default] = %q, want it to match Faces[PoseB] = %q (explicit face=)", c.Faces["default"], c.Faces["PoseB"])
	}
}

func TestHandleCharaNewPSDMissingRequiredAttrs(t *testing.T) {
	resetPSDFaceCache(t)
	r := newTestRendererWithPSDFS(t)
	cases := []map[string]string{
		{"storage": "chara_psd/group_layer.psd", "favorite": "chara_psd/test.pfv"}, // missing name
		{"name": "x", "favorite": "chara_psd/test.pfv"},                            // missing storage
		{"name": "x", "storage": "chara_psd/group_layer.psd"},                      // missing favorite
	}
	for _, pm := range cases {
		tag := kag3.TagObject{Name: "chara_new_psd", Pm: pm}
		i := 0
		if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
			t.Errorf("dispatchTag with %v: expected an error, got nil", pm)
		}
	}
}

func TestHandleCharaNewPSDUnknownFace(t *testing.T) {
	resetPSDFaceCache(t)
	r := newTestRendererWithPSDFS(t)
	tag := kag3.TagObject{Name: "chara_new_psd", Pm: map[string]string{
		"name":     "psdchara3",
		"storage":  "chara_psd/group_layer.psd",
		"favorite": "chara_psd/test.pfv",
		"face":     "NoSuchPreset",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for a face= that doesn't match any favorite preset, got nil")
	}
}

// TestLoadImageRegeneratesPSDFaceAfterCacheCleared is the save/load
// gap this feature has to close: charas[name].Faces values (and
// Character.Storage) are captured verbatim into the save file
// (buildSaveData's CharaFaces/CharaStorage, tags_save.go) as plain
// strings, but psdFaceGroups itself — the *ebiten.Image cache — is a
// process-lifetime registry that a fresh process loading that save never
// populates by actually running [chara_new_psd] again. loadImage must be
// able to regenerate the image from the storage string alone.
func TestLoadImageRegeneratesPSDFaceAfterCacheCleared(t *testing.T) {
	resetPSDFaceCache(t)
	r := newTestRendererWithPSDFS(t)
	tag := kag3.TagObject{Name: "chara_new_psd", Pm: map[string]string{
		"name":     "psdchara4",
		"storage":  "chara_psd/group_layer.psd",
		"favorite": "chara_psd/test.pfv",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	poseBStorage := charas["psdchara4"].Faces["PoseB"]

	// Simulate a fresh process: the in-memory group cache is gone, and
	// crucially [chara_new_psd] is NOT re-run — only loadImage(r, "",
	// poseBStorage), exactly what reconcileViewCharas (tags_save.go) calls
	// on a save/load.
	resetPSDFaceCache(t)
	img, err := loadImage(r, "", poseBStorage)
	if err != nil {
		t.Fatalf("loadImage failed to regenerate a PSD face from storage alone: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Errorf("regenerated image bounds = %v, want 64x64", b)
	}
}

// TestReconcileViewCharasRestoresPSDFaceInFreshProcess drives the actual
// save/load path (reconcileViewCharas), not just loadImage directly, with
// charas left completely empty — the true fresh-process shape: a save
// resumed via jumpIndex never re-runs [chara_new_psd], so the only things
// available are whatever buildSaveData captured (charaStorage/charaFaces)
// and viewCharas' saved CharaShow entries.
func TestReconcileViewCharasRestoresPSDFaceInFreshProcess(t *testing.T) {
	resetPSDFaceCache(t)
	r := newTestRendererWithPSDFS(t)

	// Build the storage strings the same way [chara_new_psd] would, without
	// actually registering charas["psdchara5"] — charas is left empty on
	// purpose, standing in for a fresh process.
	poseA := encodePSDFaceStorage("chara_psd/group_layer.psd", "chara_psd/test.pfv", "utf-8", "PoseA")
	poseB := encodePSDFaceStorage("chara_psd/group_layer.psd", "chara_psd/test.pfv", "utf-8", "PoseB")
	delete(charas, "psdchara5")

	restored := []*kag3.CharaShow{{Name: "psdchara5"}}
	charaStorage := map[string]string{"psdchara5": poseA}
	charaFaces := map[string]map[string]string{
		"psdchara5": {"default": poseA, "PoseA": poseA, "PoseB": poseB},
	}

	kept := reconcileViewCharas(r, restored, charaStorage, charaFaces)
	if len(kept) != 1 {
		t.Fatalf("reconcileViewCharas kept %d entries, want 1", len(kept))
	}
	c, ok := charas["psdchara5"]
	if !ok {
		t.Fatal("charas[psdchara5] was not re-registered")
	}
	if c.Image == nil {
		t.Fatal("Image is nil after reconcileViewCharas")
	}
	if b := c.Image.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Errorf("Image bounds = %v, want 64x64", b)
	}
	if c.Faces["PoseB"] != poseB {
		t.Errorf("Faces[PoseB] = %q, want %q (full face registry restored from charaFaces)", c.Faces["PoseB"], poseB)
	}
}
