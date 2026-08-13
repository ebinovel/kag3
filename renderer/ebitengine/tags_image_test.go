package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestImagePosDoesNotOverwriteDepth is the regression test for #15:
// [image pos=...] used to write into img.Depth (a straight copy-paste from
// the depth= case just above it), silently overwriting whatever depth=
// had just set instead of populating img.Pos, which stayed permanently
// empty. Neither field feeds drawing today (see kag3.Image's own doc
// comment / the plan note this fix comes from) — this test only checks
// the two attributes land in the fields they're named after.
func TestImagePosDoesNotOverwriteDepth(t *testing.T) {
	origImgs := imgs
	t.Cleanup(func() { imgs = origImgs })
	imgs = nil

	r := newTestRendererWithImageFS(t, map[string][]byte{
		"room.jpg": tinyPNG(t),
	})

	tag := kag3.TagObject{Name: "image", Pm: map[string]string{
		"storage": "room.jpg",
		"depth":   "front",
		"pos":     "left",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag(image): %v", err)
	}
	if len(imgs) != 1 {
		t.Fatalf("len(imgs) = %d, want 1", len(imgs))
	}
	got := imgs[0]
	if got.Depth != "front" {
		t.Errorf("Depth = %q, want %q (pos= must not overwrite it)", got.Depth, "front")
	}
	if got.Pos != "left" {
		t.Errorf("Pos = %q, want %q", got.Pos, "left")
	}
}
