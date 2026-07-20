package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

func TestHandleIScriptRunsBody(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "iscript", Body: "f.x = 99;"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if got := r.vm.EvalString("f.x"); got != "99" {
		t.Errorf("f.x = %q, want %q", got, "99")
	}
}

func TestHandleEval(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "eval", Pm: map[string]string{"exp": "f.hoge = 42"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if got := r.vm.EvalString("f.hoge"); got != "42" {
		t.Errorf("f.hoge = %q, want %q", got, "42")
	}
}

func TestHandleEmbInsertsInlineText(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "emb", Line: 5, Pm: map[string]string{"exp": "1+1"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	segs := r.texts[5]
	if len(segs) != 1 || segs[0].Text != "2" {
		t.Errorf("r.texts[5] = %+v, want a single Text{Text: \"2\"}", segs)
	}
}

func TestHandleTraceDoesNotError(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "trace", Pm: map[string]string{"exp": "\"hello\""}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
}

func TestHandleClearVarSingleName(t *testing.T) {
	r := newTestRenderer()
	r.vm.Eval(`f.a = 1; f.b = 2;`)
	tag := kag3.TagObject{Name: "clearvar", Pm: map[string]string{"name": "a"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if got := r.vm.EvalString("f.a"); got != "undefined" {
		t.Errorf("f.a = %q, want %q (cleared)", got, "undefined")
	}
	if got := r.vm.EvalString("f.b"); got != "2" {
		t.Errorf("f.b = %q, want %q (untouched)", got, "2")
	}
}

func TestHandleClearVarAll(t *testing.T) {
	r := newTestRenderer()
	r.vm.Eval(`f.a = 1; f.b = 2;`)
	tag := kag3.TagObject{Name: "clearvar", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if got := r.vm.EvalString("f.a"); got != "undefined" {
		t.Errorf("f.a = %q, want %q", got, "undefined")
	}
	if got := r.vm.EvalString("f.b"); got != "undefined" {
		t.Errorf("f.b = %q, want %q", got, "undefined")
	}
}

func TestHandleClearSysVar(t *testing.T) {
	r := newTestRenderer()
	r.vm.Eval(`sf.a = 1;`)
	tag := kag3.TagObject{Name: "clearsysvar"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if got := r.vm.EvalString("sf.a"); got != "undefined" {
		t.Errorf("sf.a = %q, want %q", got, "undefined")
	}
}

func TestHandleEraseMacro(t *testing.T) {
	r := newTestRenderer()
	r.manager.Macros["greet"] = &kag3.Macro{Name: "greet"}
	tag := kag3.TagObject{Name: "erasemacro", Pm: map[string]string{"name": "greet"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if _, ok := r.manager.Macros["greet"]; ok {
		t.Errorf("macro %q still registered after erasemacro", "greet")
	}
}
