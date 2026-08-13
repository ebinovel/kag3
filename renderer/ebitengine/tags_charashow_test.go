package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestCharaShowReappliesAttributesOnReDisplay is the regression test for
// #17: handleCharaShow used to parse every attribute (left=, top=, face=,
// storage=, ...) only inside the charaNew==true branch, so re-showing a
// character that had been [chara_hide]'d silently ignored every attribute
// on the re-[chara_show] call — [chara_show name=x left=800] right after
// [chara_hide name=x] left the character exactly where it was before being
// hidden instead of moving it to left=800.
func TestCharaShowReappliesAttributesOnReDisplay(t *testing.T) {
	origCharas, origViewCharas := charas, viewCharas
	t.Cleanup(func() { charas, viewCharas = origCharas, origViewCharas })
	charas = make(map[string]*kag3.Character)
	viewCharas = nil

	r := newTestRendererWithImageFS(t, map[string][]byte{
		"akane.png": tinyPNG(t),
	})

	newTag := kag3.TagObject{Name: "chara_new", Pm: map[string]string{"name": "akane", "storage": "akane.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), newTag, &i, 0); err != nil {
		t.Fatalf("dispatchTag(chara_new): %v", err)
	}

	showTag := kag3.TagObject{Name: "chara_show", Pm: map[string]string{"name": "akane", "left": "100", "wait": "false"}}
	if err := dispatchTag(r, fakeYield(), showTag, &i, 0); err != nil {
		t.Fatalf("dispatchTag(chara_show #1): %v", err)
	}
	if len(viewCharas) != 1 || viewCharas[0].Left != 100 {
		t.Fatalf("after first chara_show: viewCharas = %+v, want one entry with Left=100", viewCharas)
	}

	hideTag := kag3.TagObject{Name: "chara_hide", Pm: map[string]string{"name": "akane"}}
	if err := dispatchTag(r, fakeYield(), hideTag, &i, 0); err != nil {
		t.Fatalf("dispatchTag(chara_hide): %v", err)
	}
	if !viewCharas[0].IsRemove {
		t.Fatalf("after chara_hide: viewCharas[0].IsRemove = false, want true")
	}

	reShowTag := kag3.TagObject{Name: "chara_show", Pm: map[string]string{"name": "akane", "left": "800", "wait": "false"}}
	if err := dispatchTag(r, fakeYield(), reShowTag, &i, 0); err != nil {
		t.Fatalf("dispatchTag(chara_show #2): %v", err)
	}
	if len(viewCharas) != 1 {
		t.Fatalf("after re-chara_show: len(viewCharas) = %d, want 1 (re-display must not append a duplicate entry)", len(viewCharas))
	}
	if viewCharas[0].Left != 800 {
		t.Errorf("after re-chara_show: viewCharas[0].Left = %d, want 800 (left= on the re-display call must not be ignored)", viewCharas[0].Left)
	}
	if viewCharas[0].IsRemove {
		t.Error("after re-chara_show: viewCharas[0].IsRemove = true, want false")
	}
}

// TestApplyCharaShowAttrsUnit is a focused unit test on the extracted
// applyCharaShowAttrs, independent of handleCharaShow's charaNew branching
// and positioning logic.
func TestApplyCharaShowAttrsUnit(t *testing.T) {
	origCharas := charas
	t.Cleanup(func() { charas = origCharas })
	charas = map[string]*kag3.Character{"akane": {Name: "akane", Faces: map[string]string{"smile": "akane_smile.png"}}}

	chara := &kag3.CharaShow{Left: 42, Top: 42}
	pm := map[string]string{"left": "800", "face": "smile"}
	r := newTestRenderer()
	if err := applyCharaShowAttrs(r, chara, "akane", pm); err != nil {
		t.Fatalf("applyCharaShowAttrs: %v", err)
	}
	if chara.Left != 800 {
		t.Errorf("Left = %d, want 800", chara.Left)
	}
	if chara.Top != 42 {
		t.Errorf("Top = %d, want 42 (unspecified attribute must stay unchanged)", chara.Top)
	}
	if chara.Face != "akane_smile.png" {
		t.Errorf("Face = %q, want %q", chara.Face, "akane_smile.png")
	}
}
