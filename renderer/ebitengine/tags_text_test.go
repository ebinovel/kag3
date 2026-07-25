package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestHandlePDoesNotClearCharaName is the regression test for a real
// reported bug: the character name-plate ptext (ptextContent, renderer.go)
// disappeared after showing for exactly one line. Real Tyrano scripts
// declare "#name" once and then write several [p]-separated lines under it
// with no repeated "#name" in between (e.g. the bundled scene1.ks) —
// charaName has to survive [p] or the name-plate goes blank on the very
// next line.
func TestHandlePDoesNotClearCharaName(t *testing.T) {
	charaName = "akane"
	defer func() { charaName = "" }()

	r := newTestRenderer()
	r.texts = map[int][]Text{0: {{Text: "hi"}}}
	tag := kag3.TagObject{Name: "p"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if charaName != "akane" {
		t.Errorf("charaName = %q after [p], want unchanged %q", charaName, "akane")
	}
}

// TestHandleCMDoesNotClearCharaName is the same regression for [cm] — the
// bundled scene1.ks declares "#あかね" immediately followed by "[cm]" while
// still under the same speaker, which would otherwise blank the name-plate
// right back out before the next line ever displays.
func TestHandleCMDoesNotClearCharaName(t *testing.T) {
	charaName = "akane"
	defer func() { charaName = "" }()

	r := newTestRenderer()
	r.texts = map[int][]Text{0: {{Text: "hi"}}}
	tag := kag3.TagObject{Name: "cm"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if charaName != "akane" {
		t.Errorf("charaName = %q after [cm], want unchanged %q", charaName, "akane")
	}
}
