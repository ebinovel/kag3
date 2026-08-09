package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

// fakeYield stands in for the coroutine's real Yield outside of Update()'s
// per-frame driving. TextObject handling can block on y.Until(...) waiting
// for the package-level isWait flag (normally flipped by a simulated user
// click in Update()), so each call here flips it too, acting like an
// instant click and guaranteeing any such wait resolves after one yield.
func fakeYield() coro.Yield {
	return func() bool {
		isWait = true
		return true
	}
}

func newTestRenderer() *Renderer {
	return &Renderer{
		texts: make(map[int][]Text),
		vm:    newVM(),
		// Config is a real (zero-value) *kag3.Config, not nil — real
		// NewRenderer always has one (Load/LoadDefault populate it before a
		// Renderer ever exists), and code paths that read
		// r.manager.Config.* (e.g. ContinueMarkStyle in draw_message.go)
		// would otherwise nil-panic under this package's tests.
		manager: &kag3.Manager{Macros: make(map[string]*kag3.Macro), Config: &kag3.Config{}},
	}
}

// TestDispatchTagExpandsMacro is the concrete deliverable from the plan:
// a [macro name="say"]hi[endmacro] definition, invoked as [say], must run
// its body ("hi" ends up in the text buffer) even though "say" has no
// built-in handler.
func TestDispatchTagExpandsMacro(t *testing.T) {
	r := newTestRenderer()
	r.manager.Macros["say"] = &kag3.Macro{
		Name: "say",
		Body: []any{
			kag3.TextObject{Line: 100, Name: "text", Val: "hi"},
		},
	}

	tag := kag3.TagObject{Name: "say", Line: 1, Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag returned error: %v", err)
	}

	got := r.texts[100]
	if len(got) != 1 || got[0].Text != "hi" {
		t.Fatalf("r.texts[100] = %+v, want a single Text{Text: \"hi\"}", got)
	}
}

func TestDispatchTagMacroRecursionLimit(t *testing.T) {
	r := newTestRenderer()
	// A macro that calls itself must eventually error out rather than
	// recursing forever.
	r.manager.Macros["loop"] = &kag3.Macro{
		Name: "loop",
		Body: []any{
			kag3.TagObject{Name: "loop", Pm: map[string]string{}},
		},
	}

	tag := kag3.TagObject{Name: "loop", Line: 1, Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Fatal("expected an error for runaway macro recursion, got nil")
	}
}

var macroTestProbe string

func init() {
	register("macro_test_probe", func(ctx *tagCtx) error {
		macroTestProbe = ctx.tag.Pm["whom"]
		return nil
	})
}

// TestExpandMacroSetsMPFrame verifies the macro-call arguments are visible
// as "mp.*" (readable from JS, and via "%name" tag-argument expansion)
// while the body runs, and that the frame is restored afterward.
func TestExpandMacroSetsMPFrame(t *testing.T) {
	r := newTestRenderer()
	m := &kag3.Macro{
		Name: "greet",
		Body: []any{
			kag3.TagObject{Name: "macro_test_probe", Pm: map[string]string{"whom": "%name"}},
		},
	}
	macroTestProbe = ""

	if err := r.expandMacro(fakeYield(), m, map[string]string{"name": "akane"}, 0); err != nil {
		t.Fatalf("expandMacro returned error: %v", err)
	}
	if macroTestProbe != "akane" {
		t.Errorf("macro_test_probe captured %q, want %q (mp.name should resolve during macro body execution)", macroTestProbe, "akane")
	}
	if got := r.vm.EvalString("mp.name"); got != "undefined" {
		t.Errorf("mp.name after expandMacro returned = %q, want %q (frame should be popped)", got, "undefined")
	}
}

// TestExpandMacroIfElseTakesElseBranch is the regression test for a real
// reported bug: tyrano.ks's own bundled replay_image_button/cg_image_button
// macros (the ones behind title.ks's CG/回想 gallery buttons) wrap a
// [button] in [if exp=...]...[else]...[endif], and the false branch was
// silently dropping everything from [else] onward — no error, just nothing
// rendered — because skipIfChain/handleIgnore scanned r.scripts (the
// top-level script) instead of the macro's own Body, landing *i on a
// position far outside Body's bounds and making expandMacro's own body loop
// exit early as if it had reached the end. A macro whose [if] evaluates
// false must still run its [else] body.
func TestExpandMacroIfElseTakesElseBranch(t *testing.T) {
	r := newTestRenderer()
	macroTestProbe = ""
	m := &kag3.Macro{
		Name: "conditional",
		Body: []any{
			kag3.TagObject{Name: "if", Line: 0, Pm: map[string]string{"exp": "false"}, IfCount: 1},
			kag3.TagObject{Name: "macro_test_probe", Line: 1, Pm: map[string]string{"whom": "if-branch"}, IfCount: 0},
			kag3.TagObject{Name: "else", Line: 2, IfCount: 1},
			kag3.TagObject{Name: "macro_test_probe", Line: 3, Pm: map[string]string{"whom": "else-branch"}, IfCount: 0},
			kag3.TagObject{Name: "endif", Line: 4, IfCount: 1},
		},
	}

	if err := r.expandMacro(fakeYield(), m, map[string]string{}, 0); err != nil {
		t.Fatalf("expandMacro returned error: %v", err)
	}
	if macroTestProbe != "else-branch" {
		t.Errorf("macroTestProbe = %q, want %q (a false [if] inside a macro body must still run its [else])", macroTestProbe, "else-branch")
	}
}

// TestRubyAppliesOnlyToFirstCharacter is the regression test for a real
// reported bug: "[ruby text=たん]単にできます" (no further tag before the
// text object ends, so the parser hands execItem one six-rune TextObject)
// attached "たん" to the *entire* run instead of just "単", visibly
// centering the ruby over six characters instead of one. Real Tyrano's
// [ruby text=...] annotates exactly the single character immediately
// following it — appendRubyText (macro.go) splits the ruby'd rune off into
// its own segment so drawMessage*'s per-segment centering (draw_message.go)
// lands on just that character.
func TestRubyAppliesOnlyToFirstCharacter(t *testing.T) {
	r := newTestRenderer()
	pendingRuby = ""
	defer func() { pendingRuby = "" }()

	scripts := []any{
		kag3.TagObject{Name: "ruby", Line: 1, Pm: map[string]string{"text": "たん"}},
		kag3.TextObject{Line: 1, Name: "text", Val: "単にできます"},
	}
	i := 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem (ruby tag) error: %v", err)
	}
	i = 1
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem (text) error: %v", err)
	}

	segs := r.texts[1]
	if len(segs) != 2 {
		t.Fatalf("r.texts[1] = %+v, want 2 segments (ruby'd first rune + plain rest)", segs)
	}
	if segs[0].Text != "単" || segs[0].Ruby != "たん" {
		t.Errorf("segs[0] = %+v, want Text=単 Ruby=たん", segs[0])
	}
	if segs[1].Text != "にできます" || segs[1].Ruby != "" {
		t.Errorf("segs[1] = %+v, want Text=にできます Ruby=\"\"", segs[1])
	}
}

// TestRubyOnSingleCharacterStaysOneSegment makes sure the split above only
// happens when there's actually more than one rune to split off — a
// single-character run like "[ruby text=かん]簡" needs no second segment.
func TestRubyOnSingleCharacterStaysOneSegment(t *testing.T) {
	r := newTestRenderer()
	pendingRuby = ""
	defer func() { pendingRuby = "" }()

	scripts := []any{
		kag3.TagObject{Name: "ruby", Line: 1, Pm: map[string]string{"text": "かん"}},
		kag3.TextObject{Line: 1, Name: "text", Val: "簡"},
	}
	i := 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem (ruby tag) error: %v", err)
	}
	i = 1
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem (text) error: %v", err)
	}

	segs := r.texts[1]
	if len(segs) != 1 || segs[0].Text != "簡" || segs[0].Ruby != "かん" {
		t.Errorf("r.texts[1] = %+v, want a single Text{Text: \"簡\", Ruby: \"かん\"}", segs)
	}
}

// TestExecItemMarksLineReadOnFirstDisplayOnly is the read-tracking
// deliverable's integration point: execItem must record (currentStorage,
// object.Line) via markLineRead exactly when a new line starts revealing
// (the same isNewLine gate that resets textStartT/isWait), and
// currentLineAlreadyRead must reflect "was this line already read *before*
// this visit" — false the first time a line displays, true on any later
// visit to the same (storage, line), e.g. after a rollback/replay.
func TestExecItemMarksLineReadOnFirstDisplayOnly(t *testing.T) {
	origRead, origAlreadyRead := readLines, currentLineAlreadyRead
	readLines = map[readLineKey]bool{}
	defer func() { readLines, currentLineAlreadyRead = origRead, origAlreadyRead }()

	r := newTestRenderer()
	r.currentStorage = "first.ks"
	r.line = 0
	scripts := []any{
		kag3.TextObject{Line: 5, Name: "text", Val: "hello"},
	}
	i := 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem (first visit) error: %v", err)
	}
	if currentLineAlreadyRead {
		t.Error("currentLineAlreadyRead = true on a line's first-ever display, want false")
	}
	if !readLines[readLineKey{"first.ks", 5}] {
		t.Error("readLines[{first.ks, 5}] = false after displaying it, want true")
	}

	// Simulate revisiting the same line (e.g. rollback, or replaying the
	// scenario from a save) — a fresh r.line so isNewLine is true again.
	r.line = -1
	i = 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem (second visit) error: %v", err)
	}
	if !currentLineAlreadyRead {
		t.Error("currentLineAlreadyRead = false on a line's second display, want true (already read)")
	}
}
