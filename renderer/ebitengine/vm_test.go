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

func TestVMPushMPFrame(t *testing.T) {
	v := newVM()
	if got := v.EvalString("mp.name"); got != "undefined" {
		t.Fatalf("mp.name before any frame = %q, want %q", got, "undefined")
	}

	popA := v.PushMPFrame(map[string]string{"name": "foo"})
	if got := v.EvalString("mp.name"); got != "foo" {
		t.Errorf("mp.name with frame A active = %q, want %q", got, "foo")
	}

	popB := v.PushMPFrame(map[string]string{"name": "bar"})
	if got := v.EvalString("mp.name"); got != "bar" {
		t.Errorf("mp.name with frame B active (nested) = %q, want %q", got, "bar")
	}

	popB()
	if got := v.EvalString("mp.name"); got != "foo" {
		t.Errorf("mp.name after popping frame B = %q, want restored %q", got, "foo")
	}

	popA()
	if got := v.EvalString("mp.name"); got != "undefined" {
		t.Errorf("mp.name after popping frame A = %q, want %q", got, "undefined")
	}
}

// TestTFSystemAndSFSystemPreinitialized reproduces the crash reported when
// pressing the back button in replay mode: example/resources/senarios/
// replay.ks unconditionally does `tf.system.flag_replay = false;` and
// config.ks does `tf.system.backlog.pop();`, both assuming tf.system (and
// its backlog array) already exist — real Tyrano creates them before any
// script runs. Without that, tf.system is undefined and the assignment
// throws "Cannot convert undefined or null to object".
func TestTFSystemAndSFSystemPreinitialized(t *testing.T) {
	v := newVM()
	if _, err := v.Eval("tf.system.flag_replay = false;"); err != nil {
		t.Errorf("tf.system.flag_replay assignment failed: %v", err)
	}
	if _, err := v.Eval("tf.system.backlog.pop();"); err != nil {
		t.Errorf("tf.system.backlog.pop() failed: %v", err)
	}
	if _, err := v.Eval("sf.system.foo = 1;"); err != nil {
		t.Errorf("sf.system.foo assignment failed: %v", err)
	}
}

// TestClearSFReinitializesSystemNamespace ensures [clearsysvar] doesn't
// reintroduce the same crash for any script that happens to touch
// sf.system afterward.
func TestClearSFReinitializesSystemNamespace(t *testing.T) {
	v := newVM()
	v.ClearSF()
	if _, err := v.Eval("sf.system.foo = 1;"); err != nil {
		t.Errorf("sf.system.foo assignment after ClearSF failed: %v", err)
	}
}

// TestRestoreSFReinitializesMissingSystemNamespace covers loading a save
// made before sf.system existed (or one where a script deleted it).
func TestRestoreSFReinitializesMissingSystemNamespace(t *testing.T) {
	v := newVM()
	v.RestoreSF(map[string]interface{}{"seen": true}) // no "system" key at all
	if _, err := v.Eval("sf.system.foo = 1;"); err != nil {
		t.Errorf("sf.system.foo assignment after RestoreSF (no system key) failed: %v", err)
	}
	if got := v.EvalString("sf.seen"); got != "true" {
		t.Errorf("sf.seen after RestoreSF = %q, want %q (existing keys must survive)", got, "true")
	}
}
