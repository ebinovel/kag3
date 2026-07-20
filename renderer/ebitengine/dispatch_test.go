package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestDispatchTagUnknownTagIsNoop covers the Foundation-phase requirement
// that a tag with no registered handler and no matching [macro] logs a
// diagnostic (未実装のタグです) but does not error or panic — staged tag
// rollout must never crash the game on a gap.
func TestDispatchTagUnknownTagIsNoop(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "totally_unimplemented_tag", Line: 1, Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag on unknown tag returned error: %v", err)
	}
}

// TestDispatchTagUnknownTagDoesNotHaltExecution drives a small script
// containing an unimplemented tag followed by real content, confirming the
// unknown tag is skipped rather than stopping the run.
func TestDispatchTagUnknownTagDoesNotHaltExecution(t *testing.T) {
	r := newTestRenderer()
	scripts := []any{
		kag3.TagObject{Name: "totally_unimplemented_tag", Line: 0, Pm: map[string]string{}},
		kag3.TextObject{Line: 1, Name: "text", Val: "still runs"},
	}

	for i := 0; i < len(scripts); i++ {
		if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
			t.Fatalf("execItem error: %v", err)
		}
	}

	got := r.texts[1]
	if len(got) != 1 || got[0].Text != "still runs" {
		t.Fatalf("r.texts[1] = %+v, want a single Text{Text: \"still runs\"}", got)
	}
}
