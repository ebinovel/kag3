package ebitengine

import "testing"

func TestVMEvalBool(t *testing.T) {
	v := newVM()
	if _, err := v.Eval("f.hoge = 5"); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if !v.EvalBool("f.hoge > 3") {
		t.Error("expected f.hoge > 3 to be true")
	}
	if v.EvalBool("f.hoge > 10") {
		t.Error("expected f.hoge > 10 to be false")
	}
	if v.EvalBool("this is not valid js (") {
		t.Error("expected malformed expression to evaluate as false, not panic")
	}
}

func TestVMEvalString(t *testing.T) {
	v := newVM()
	if _, err := v.Eval(`f.name = "akane"`); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := v.EvalString("f.name"); got != "akane" {
		t.Errorf("EvalString(f.name) = %q, want %q", got, "akane")
	}
	if got := v.EvalString("f.missing"); got != "undefined" {
		t.Errorf("EvalString(f.missing) = %q, want %q", got, "undefined")
	}
}

func TestVMExpandParams(t *testing.T) {
	v := newVM()
	if _, err := v.Eval("f.hoge = 42"); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	pm := map[string]string{
		"plain": "left",
		"expr":  "&f.hoge",
		"mp":    "%name|default_name",
	}
	out := v.expandParams(pm)
	if out["plain"] != "left" {
		t.Errorf("plain = %q, want unchanged %q", out["plain"], "left")
	}
	if out["expr"] != "42" {
		t.Errorf("expr = %q, want %q", out["expr"], "42")
	}
	if out["mp"] != "default_name" {
		t.Errorf("mp = %q, want default %q (no active mp frame)", out["mp"], "default_name")
	}
}
