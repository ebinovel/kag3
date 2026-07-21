package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

func TestHandleBG2SetsNextImageAndTick(t *testing.T) {
	bg2 = &kag3.Background{}
	r := newTestRendererWithImageFS(t, map[string][]byte{"weather.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "bg2", Pm: map[string]string{"storage": "weather.png", "method": "crossfade"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if bg2.NextImage == nil {
		t.Error("expected bg2.NextImage to be set")
	}
	if bg2.Method != "crossfade" {
		t.Errorf("bg2.Method = %q, want %q", bg2.Method, "crossfade")
	}
}

func TestHandleFreeRemovesNamedPTextAndImage(t *testing.T) {
	ptexts = map[string]*kag3.PText{"chara_name_area": {Name: "chara_name_area", Text: "x"}}
	charaNamePText = "chara_name_area"
	imgs = []*kag3.Image{{Name: "chara_name_area"}, {Name: "other"}}

	tag := kag3.TagObject{Name: "free", Pm: map[string]string{"name": "chara_name_area"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if _, ok := ptexts["chara_name_area"]; ok {
		t.Error("expected the named ptext to be removed")
	}
	if charaNamePText != "" {
		t.Errorf("charaNamePText = %q, want cleared", charaNamePText)
	}
	if len(imgs) != 1 || imgs[0].Name != "other" {
		t.Errorf("imgs = %+v, want only \"other\" remaining", imgs)
	}
}

func TestHandleFreeImageByLayerAndAll(t *testing.T) {
	imgs = []*kag3.Image{{Name: "a", Layer: "1"}, {Name: "b", Layer: "2"}}

	tag := kag3.TagObject{Name: "freeimage", Pm: map[string]string{"layer": "1"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(imgs) != 1 || imgs[0].Name != "b" {
		t.Errorf("imgs = %+v, want only layer 2's image remaining", imgs)
	}

	tag = kag3.TagObject{Name: "freeimage", Pm: map[string]string{}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(imgs) != 0 {
		t.Errorf("imgs = %+v, want empty after freeimage with no layer=", imgs)
	}
}

func TestBackLayThenTransSwapsInStagedContent(t *testing.T) {
	imgs = []*kag3.Image{{Name: "live"}}
	ptexts = map[string]*kag3.PText{"live": {Name: "live"}}
	backImgs = nil
	backPtexts = nil
	r := newTestRenderer()

	// Stage a different layout.
	imgs = []*kag3.Image{{Name: "staged"}}
	ptexts = map[string]*kag3.PText{"staged": {Name: "staged"}}
	backlay := kag3.TagObject{Name: "backlay"}
	i := 0
	if err := dispatchTag(r, fakeYield(), backlay, &i, 0); err != nil {
		t.Fatalf("backlay error: %v", err)
	}

	// Switch back to the "live" layout, simulating other tags running
	// in between backlay and trans.
	imgs = []*kag3.Image{{Name: "live"}}
	ptexts = map[string]*kag3.PText{"live": {Name: "live"}}

	trans := kag3.TagObject{Name: "trans"}
	if err := dispatchTag(r, fakeYield(), trans, &i, 0); err != nil {
		t.Fatalf("trans error: %v", err)
	}
	if len(imgs) != 1 || imgs[0].Name != "staged" {
		t.Errorf("imgs = %+v, want the staged layout revealed", imgs)
	}
	if _, ok := ptexts["staged"]; !ok {
		t.Errorf("ptexts = %+v, want the staged ptext revealed", ptexts)
	}
	if backImgs != nil || backPtexts != nil {
		t.Error("expected the back buffer to be cleared after trans")
	}
}

func TestTransWithNothingStagedIsNoop(t *testing.T) {
	backImgs = nil
	backPtexts = nil
	imgs = []*kag3.Image{{Name: "live"}}
	tag := kag3.TagObject{Name: "trans"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(imgs) != 1 || imgs[0].Name != "live" {
		t.Errorf("imgs = %+v, want unchanged when nothing was staged", imgs)
	}
}

func TestHandleLocateMovesTextPosition(t *testing.T) {
	textPosition.Left = 0
	textPosition.Top = 0
	tag := kag3.TagObject{Name: "locate", Pm: map[string]string{"x": "50", "y": "80"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if textPosition.Left != 50 || textPosition.Top != 80 {
		t.Errorf("textPosition = {Left:%d Top:%d}, want {50 80}", textPosition.Left, textPosition.Top)
	}
}

func TestHandleClearFixClearsGLinks(t *testing.T) {
	glinks = []*kag3.GLink{{Name: "a"}, {Name: "b"}}
	tag := kag3.TagObject{Name: "clearfix"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(glinks) != 0 {
		t.Errorf("glinks = %+v, want empty after clearfix", glinks)
	}
}

func TestHandleHideMessageHidesTextWindow(t *testing.T) {
	textPosition.Visible = true
	tag := kag3.TagObject{Name: "hidemessage"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if textPosition.Visible {
		t.Error("expected textPosition.Visible = false after hidemessage")
	}
}
