package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestImagePosDoesNotOverwriteDepth pins [image pos=...] to img.Pos:
// writing it into img.Depth instead (an easy copy-paste from the depth=
// case right above it) silently overwrites whatever depth= just set and
// leaves img.Pos permanently empty. Neither field feeds drawing today, so
// this only checks the two attributes land in the fields they're named
// after.
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
