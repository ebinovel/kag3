package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

// resetBG restores bg/bg2 to NewRenderer-equivalent zero state and returns
// a cleanup func that puts them back — the same "reset the globals this
// test touches" discipline every other tags_*_test.go in this package
// follows (see CLAUDE.md's Execution model note).
func resetBG(t *testing.T) {
	t.Helper()
	origBG, origBG2 := bg, bg2
	bg = &kag3.Background{Time: 3000, IsWait: true, Method: "crossfade"}
	bg2 = &kag3.Background{Time: 3000, IsWait: false, Method: "crossfade"}
	t.Cleanup(func() { bg, bg2 = origBG, origBG2 })
}

// TestBGStorageResolvesUnderBgSubfolder is the regression test for a real
// reported bug: kag3's [bg storage=] required the full images/-relative
// path including a "bg/" prefix the author had to spell out themselves
// (storage="bg/room.jpg"), unlike real TyranoScript's [bg], where
// storage="room.jpg" alone resolves against its own bgimage-equivalent
// folder — and unlike this editor's own images/bg/ project convention
// (internal/project/asset.go's ImageSubdirs in ebinovel-editor), which
// assumed exactly that automatic resolution already existed.
func TestBGStorageResolvesUnderBgSubfolder(t *testing.T) {
	resetBG(t)
	r := newTestRendererWithImageFS(t, map[string][]byte{
		"bg/room.jpg": tinyPNG(t), // ebitenutil decodes by content, not extension
	})

	// wait="false": [bg]'s default IsWait=true makes applyBGTag block on
	// ctx.y.Until(true, func() bool { return target.IsEnd }) — real inside
	// the actual Update()/Draw() loop, which advances target.IsEnd, but
	// nothing here drives that, so with the default left alone this would
	// hang forever. Irrelevant to what this test actually checks (storage
	// resolution happens earlier, unconditionally).
	tag := kag3.TagObject{Name: "bg", Pm: map[string]string{"storage": "room.jpg", "wait": "false"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag: %v", err)
	}
	if bg.NextImage == nil {
		t.Fatal("bg.NextImage is nil — storage=\"room.jpg\" did not resolve to bg/room.jpg")
	}
	if bg.Storage != "bg/room.jpg" {
		t.Errorf("bg.Storage = %q, want %q (the resolved path save/load reuses verbatim — see applyBgFromSnapshot)", bg.Storage, "bg/room.jpg")
	}
}

// TestBG2StorageAlsoResolvesUnderBgSubfolder confirms [bg2] gets the same
// treatment as [bg] — both go through applyBGTag.
func TestBG2StorageAlsoResolvesUnderBgSubfolder(t *testing.T) {
	resetBG(t)
	r := newTestRendererWithImageFS(t, map[string][]byte{
		"bg/weather.png": tinyPNG(t),
	})

	tag := kag3.TagObject{Name: "bg2", Pm: map[string]string{"storage": "weather.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag: %v", err)
	}
	if bg2.NextImage == nil {
		t.Fatal("bg2.NextImage is nil — storage=\"weather.png\" did not resolve to bg/weather.png")
	}
}

// TestBGStorageWithSystemTrueIsNotPrefixed confirms the bg/ auto-prefix is
// scoped to the project's own images/ root only: system=true switches to
// system/images (UI chrome — buttons, cursors — with no bg/ subfolder
// convention of its own), so a storage= there must be used exactly as
// written, unprefixed.
func TestBGStorageWithSystemTrueIsNotPrefixed(t *testing.T) {
	resetBG(t)
	r := newTestRenderer()
	r.fses = map[string]fs.FS{
		"images":        fstest.MapFS{},
		"system/images": fstest.MapFS{"panel.png": &fstest.MapFile{Data: tinyPNG(t)}},
	}

	tag := kag3.TagObject{Name: "bg", Pm: map[string]string{"storage": "panel.png", "system": "true", "wait": "false"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag: %v", err)
	}
	if bg.NextImage == nil {
		t.Fatal("bg.NextImage is nil — system=\"true\" storage=\"panel.png\" should resolve directly against system/images, unprefixed")
	}
}

// TestBGSaveLoadRoundTripsResolvedStorage confirms applyBgFromSnapshot
// (tags_save.go), which feeds bg.Storage straight back into the same
// fs.FS lookup with no resolution step of its own, keeps working now that
// bg.Storage holds the *resolved* bg/-prefixed path rather than the raw
// author-written attribute.
func TestBGSaveLoadRoundTripsResolvedStorage(t *testing.T) {
	resetBG(t)
	r := newTestRendererWithImageFS(t, map[string][]byte{
		"bg/room.jpg": tinyPNG(t),
	})

	tag := kag3.TagObject{Name: "bg", Pm: map[string]string{"storage": "room.jpg", "wait": "false"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag: %v", err)
	}

	snapshot := bgSaveState{Storage: bg.Storage, Time: bg.Time, Method: bg.Method}
	bg.Storage, bg.NextImage, bg.Image = "", nil, nil // simulate a fresh process

	if err := applyBgFromSnapshot(r, bg, snapshot); err != nil {
		t.Fatalf("applyBgFromSnapshot: %v", err)
	}
	if bg.Image == nil {
		t.Error("bg.Image is nil after applyBgFromSnapshot — the resolved bg/-prefixed Storage did not round-trip")
	}
}
