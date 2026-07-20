package kag3

import "testing"

// TestExtractMacrosAndReindexLabels guards against the main correctness
// risk in macro extraction: a LabelInfo.Index computed by ParseScenario is
// a position within the *original* Senario. Once extractMacros removes the
// [macro]...[endmacro] block, every later item shifts left, so anything
// still using the original Labels map would jump to the wrong place. This
// verifies reindexLabels fixes that up.
func TestExtractMacrosAndReindexLabels(t *testing.T) {
	src := "[macro name=\"greet\"]\nhello\n[endmacro]\n*start\n[greet]"
	ks := &KS{}
	senario, _, err := ks.ParseScenario(src)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	macros := make(map[string]*Macro)
	trimmed := extractMacros(senario, macros)

	m, ok := macros["greet"]
	if !ok {
		t.Fatalf("macro %q not registered; macros=%+v", "greet", macros)
	}
	if len(m.Body) != 1 {
		t.Fatalf("macro body length = %d, want 1; body=%+v", len(m.Body), m.Body)
	}
	text, ok := m.Body[0].(TextObject)
	if !ok || text.Val != "hello" {
		t.Fatalf("macro body[0] = %+v, want text 'hello'", m.Body[0])
	}

	// The macro block itself (3 source items: [macro], "hello", [endmacro])
	// must be gone, leaving only the label and the [greet] call.
	if len(trimmed) != 2 {
		t.Fatalf("len(trimmed) = %d, want 2; trimmed=%+v", len(trimmed), trimmed)
	}
	label, ok := trimmed[0].(LabelObject)
	if !ok || label.Info.Name != "start" {
		t.Fatalf("trimmed[0] = %+v, want label 'start'", trimmed[0])
	}
	call, ok := trimmed[1].(TagObject)
	if !ok || call.Name != "greet" {
		t.Fatalf("trimmed[1] = %+v, want tag 'greet'", trimmed[1])
	}

	labels := reindexLabels(trimmed)
	got, ok := labels["start"]
	if !ok {
		t.Fatalf("label 'start' missing after reindex; labels=%+v", labels)
	}
	if got.Index != 0 {
		t.Errorf("start.Index = %d, want 0 (ParseScenario originally computed 3, before macro removal)", got.Index)
	}
}

func TestExtractMacrosLeavesNonMacroContentUntouched(t *testing.T) {
	src := "*start\nこんにちは。"
	ks := &KS{}
	senario, _, err := ks.ParseScenario(src)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	macros := make(map[string]*Macro)
	trimmed := extractMacros(senario, macros)

	if len(macros) != 0 {
		t.Errorf("macros = %+v, want empty", macros)
	}
	if len(trimmed) != len(senario) {
		t.Errorf("len(trimmed) = %d, want unchanged %d", len(trimmed), len(senario))
	}
}
