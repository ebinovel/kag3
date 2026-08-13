package ebitengine

import (
	"strings"
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

// TestJumpStorageClearsNonFixButtons is the regression test for「はじめから
// をクリックしたら、タイトルのボタンが残ってしまっています」: title.ks's
// *gamestart label does a plain `@jump storage="scene1.ks"` (in-coroutine,
// not a click), which never went through Update()'s isJump/screenChanged
// handling at all — so title.ks's own non-fix buttons (start/load/cg/replay)
// stayed in `buttons` forever, sitting underneath scene1.ks's role_button
// set. [jump storage=...] must sweep non-fix buttons itself.
func TestJumpStorageClearsNonFixButtons(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks": "[button name=\"stale\" width=10 height=10]\n[jump storage=\"sub.ks\"]",
		"sub.ks":  "reached sub",
	})
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript(main.ks): %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
	}
	buttons = []*kag3.Button{{Name: "fixed", Fix: true}}
	defer func() { buttons = nil }()

	for i := 0; i < len(r.scripts); i++ {
		if err := r.execItem(fakeYield(), r.scripts, &i, 0); err != nil {
			t.Fatalf("execItem error: %v", err)
		}
	}

	if r.currentStorage != "sub.ks" {
		t.Fatalf("currentStorage = %q, want %q", r.currentStorage, "sub.ks")
	}
	for _, b := range buttons {
		if b.Name == "stale" {
			t.Errorf("buttons = %+v, want main.ks's non-fix \"stale\" button cleared after [jump storage=...]", buttons)
		}
	}
	found := false
	for _, b := range buttons {
		if b.Name == "fixed" {
			found = true
		}
	}
	if !found {
		t.Errorf("buttons = %+v, want the fix=true \"fixed\" button to survive the storage change", buttons)
	}
}

// runJumpScript drives main.ks through execItem exactly like initScript
// does — including re-reading len(r.scripts) every iteration, since
// [jump storage=...] swaps the slice out mid-loop — and returns everything
// rendered along the way as one string.
//
// One string rather than runFlow's per-segment set: execItem merges
// consecutive untagged text lines into a single segment ("タグを挟まない
// 連続するテキスト行を1つに連結"), so "first line\nsecond line" arrives as
// the single segment "first linesecond line" and an exact-match set can't
// answer "did this line run". Callers assert with strings.Contains instead.
func runJumpScript(t *testing.T, senarios map[string]string) string {
	t.Helper()
	m := newTestManager(t, senarios)
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript(main.ks): %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
	}
	for i := 0; i < len(r.scripts); i++ {
		if err := r.execItem(fakeYield(), r.scripts, &i, 0); err != nil {
			t.Fatalf("execItem error: %v", err)
		}
	}
	var sb strings.Builder
	for _, segs := range r.texts {
		for _, seg := range segs {
			sb.WriteString(seg.Text)
		}
	}
	return sb.String()
}

// TestJumpStorageWithTargetSeeksInNewFile is the regression test for
// [jump storage="x.ks" target="*label"]: target= used to be honored only in
// the branch taken when storage= was *absent*, so this combination loaded
// the new file and then resumed from whatever index the [jump] tag itself
// had occupied in the old one. replay.ks's own
// `@jump storage=&... target=&...` is exactly this shape.
func TestJumpStorageWithTargetSeeksInNewFile(t *testing.T) {
	got := runJumpScript(t, map[string]string{
		"main.ks": "[jump storage=\"sub.ks\" target=\"*here\"]",
		"sub.ks":  "before label\n*here\nafter label",
	})
	if !strings.Contains(got, "after label") {
		t.Errorf("rendered %q, want the text after *here to have run", got)
	}
	if strings.Contains(got, "before label") {
		t.Errorf("rendered %q, want the text before *here to have been skipped", got)
	}
}

// TestJumpStorageWithoutTargetRunsFirstItem is the regression test for the
// off-by-one: [jump storage=...] with no target= set *ctx.i to 0, but the
// enclosing loop increments once more afterward, so the new script's very
// first item never ran. handleCall's own -1 (and its comment) had this
// right already.
func TestJumpStorageWithoutTargetRunsFirstItem(t *testing.T) {
	got := runJumpScript(t, map[string]string{
		"main.ks": "[jump storage=\"sub.ks\"]",
		"sub.ks":  "first line\n[r]\nsecond line",
	})
	if !strings.Contains(got, "first line") {
		t.Errorf("rendered %q, want sub.ks's first item to have run", got)
	}
	if !strings.Contains(got, "second line") {
		t.Errorf("rendered %q, want sub.ks's remaining items to have run", got)
	}
}

// TestJumpStorageWithUnknownTargetStartsAtTop covers the fallback: a
// target= that doesn't resolve in the newly loaded file must leave
// execution at that file's top, not at the stale index the [jump] tag
// occupied in the file being left behind.
func TestJumpStorageWithUnknownTargetStartsAtTop(t *testing.T) {
	got := runJumpScript(t, map[string]string{
		"main.ks": "padding one\n[r]\npadding two\n[jump storage=\"sub.ks\" target=\"*nosuch\"]",
		"sub.ks":  "first line\n[r]\nsecond line",
	})
	if !strings.Contains(got, "first line") || !strings.Contains(got, "second line") {
		t.Errorf("rendered %q, want all of sub.ks to have run from its top", got)
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
