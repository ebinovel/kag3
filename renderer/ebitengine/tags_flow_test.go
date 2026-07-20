package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// runFlow drives scripts through execItem exactly like initScript does and
// returns every non-empty Text.Text rendered along the way, in map form for
// easy membership checks. r.scripts is set to scripts because handlers like
// [if]/[elsif]/[ignore] scan r.scripts directly (matching how initScript
// always calls execItem with r.scripts, not an arbitrary slice — see the
// same limitation noted on [jump]/[call]/[link] in macro.go).
func runFlow(t *testing.T, r *Renderer, scripts []any) map[string]bool {
	t.Helper()
	r.scripts = scripts
	for i := 0; i < len(scripts); i++ {
		if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
			t.Fatalf("execItem error: %v", err)
		}
	}
	seen := map[string]bool{}
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text != "" {
				seen[seg.Text] = true
			}
		}
	}
	return seen
}

func ifChainScript(ifExp, elsifExp string) []any {
	return []any{
		kag3.TagObject{Name: "if", Line: 0, Pm: map[string]string{"exp": ifExp}, IfCount: 1},
		kag3.TextObject{Line: 1, Name: "text", Val: "in-if"},
		kag3.TagObject{Name: "elsif", Line: 2, Pm: map[string]string{"exp": elsifExp}, IfCount: 1},
		kag3.TextObject{Line: 3, Name: "text", Val: "in-elsif"},
		kag3.TagObject{Name: "else", Line: 4, IfCount: 1},
		kag3.TextObject{Line: 5, Name: "text", Val: "in-else"},
		kag3.TagObject{Name: "endif", Line: 6, IfCount: 1},
		kag3.TextObject{Line: 7, Name: "text", Val: "after"},
	}
}

func TestIfTakesOwnBranch(t *testing.T) {
	r := newTestRenderer()
	seen := runFlow(t, r, ifChainScript("true", "true"))
	want := map[string]bool{"in-if": true, "after": true}
	for k := range want {
		if !seen[k] {
			t.Errorf("expected %q to be rendered; got %v", k, seen)
		}
	}
	for _, unwanted := range []string{"in-elsif", "in-else"} {
		if seen[unwanted] {
			t.Errorf("did not expect %q to be rendered; got %v", unwanted, seen)
		}
	}
}

func TestIfFalseElsifTrue(t *testing.T) {
	r := newTestRenderer()
	seen := runFlow(t, r, ifChainScript("false", "true"))
	if !seen["in-elsif"] || !seen["after"] {
		t.Errorf("expected in-elsif and after; got %v", seen)
	}
	if seen["in-if"] || seen["in-else"] {
		t.Errorf("did not expect in-if or in-else; got %v", seen)
	}
}

func TestIfFalseElsifFalseElseTaken(t *testing.T) {
	r := newTestRenderer()
	seen := runFlow(t, r, ifChainScript("false", "false"))
	if !seen["in-else"] || !seen["after"] {
		t.Errorf("expected in-else and after; got %v", seen)
	}
	if seen["in-if"] || seen["in-elsif"] {
		t.Errorf("did not expect in-if or in-elsif; got %v", seen)
	}
}

func TestNestedIfSkippedWhenOuterFalse(t *testing.T) {
	r := newTestRenderer()
	scripts := []any{
		kag3.TagObject{Name: "if", Line: 0, Pm: map[string]string{"exp": "false"}, IfCount: 1},
		kag3.TagObject{Name: "if", Line: 1, Pm: map[string]string{"exp": "true"}, IfCount: 2},
		kag3.TextObject{Line: 2, Name: "text", Val: "inner-in-if"},
		kag3.TagObject{Name: "endif", Line: 3, IfCount: 2},
		kag3.TagObject{Name: "endif", Line: 4, IfCount: 1},
		kag3.TextObject{Line: 5, Name: "text", Val: "after"},
	}
	seen := runFlow(t, r, scripts)
	if seen["inner-in-if"] {
		t.Errorf("inner if body should have been skipped along with the outer false branch; got %v", seen)
	}
	if !seen["after"] {
		t.Errorf("expected after; got %v", seen)
	}
}

func TestIgnoreSkipsBody(t *testing.T) {
	r := newTestRenderer()
	scripts := []any{
		kag3.TextObject{Line: 0, Name: "text", Val: "before"},
		kag3.TagObject{Name: "ignore", Line: 1},
		kag3.TextObject{Line: 2, Name: "text", Val: "ignored"},
		kag3.TagObject{Name: "endignore", Line: 3},
		kag3.TextObject{Line: 4, Name: "text", Val: "after"},
	}
	seen := runFlow(t, r, scripts)
	if seen["ignored"] {
		t.Errorf("expected ignored body to be skipped; got %v", seen)
	}
	if !seen["before"] || !seen["after"] {
		t.Errorf("expected before and after; got %v", seen)
	}
}

func TestIgnoreNested(t *testing.T) {
	r := newTestRenderer()
	scripts := []any{
		kag3.TagObject{Name: "ignore", Line: 0},
		kag3.TagObject{Name: "ignore", Line: 1},
		kag3.TextObject{Line: 2, Name: "text", Val: "deep-ignored"},
		kag3.TagObject{Name: "endignore", Line: 3},
		kag3.TextObject{Line: 4, Name: "text", Val: "still-ignored"},
		kag3.TagObject{Name: "endignore", Line: 5},
		kag3.TextObject{Line: 6, Name: "text", Val: "after"},
	}
	seen := runFlow(t, r, scripts)
	if seen["deep-ignored"] || seen["still-ignored"] {
		t.Errorf("expected nested ignore body to be fully skipped; got %v", seen)
	}
	if !seen["after"] {
		t.Errorf("expected after; got %v", seen)
	}
}

func TestClearStack(t *testing.T) {
	r := newTestRenderer()
	r.callStack = []callFrame{{Storage: "a.ks", Index: 1}, {Storage: "b.ks", Index: 2}}

	tag := kag3.TagObject{Name: "clearstack"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(r.callStack) != 0 {
		t.Errorf("callStack = %+v, want empty", r.callStack)
	}
}
