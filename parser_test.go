package kag3

import (
	"os"
	"encoding/json"
	"path"
	"reflect"
	"testing"
)

func TestParserFirst(t *testing.T) {
	first := `[wait time=200]
*start|スタート
[cm]
こんにちは。`
	ks := &KS{}
	result, _, err := ks.ParseScenario(first)
	if err != nil {
		t.Errorf("%+v", err)
	}

	var want []interface{}
	tag := TagObject{
		Name: "wait",
		Line: 0,
		Pm: map[string]string{
			"time": "200",
		},
		Val: "",
		IfCount: 0,
	}
	
	want = append(want, tag)
	label := LabelObject{
		Name: "label",
		Val: "スタート",
		Info: LabelInfo{
			Line: 1,
			Index: 1,
			Name: "start",
			Val: "スタート",
		},
	}
	want = append(want, label)
	tag = TagObject{
		Name: "cm",
		Line: 2,
		Pm: map[string]string{},
		Val: "",
		IfCount: 0,
	}
	want = append(want, tag)
	text := TextObject{
		Name: "text",
		Line: 3,
		Chara: nil,
		Val: "こんにちは。",
	}
	want = append(want, text)

	t.Run("first", func(t *testing.T) {
		if !reflect.DeepEqual(result, want) {
			t.Errorf("result %+v, want %+v", result, want)
		}
	})
}

func TestParserIScriptBody(t *testing.T) {
	src := "[iscript]\nf.hoge = 1;\nf.fuga = 2;\n[endscript]\nafter"
	ks := &KS{}
	result, _, err := ks.ParseScenario(src)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3 (iscript, endscript, text); result=%+v", len(result), result)
	}
	iscriptTag, ok := result[0].(TagObject)
	if !ok || iscriptTag.Name != "iscript" {
		t.Fatalf("result[0] = %+v, want iscript tag", result[0])
	}
	wantBody := "f.hoge = 1;\nf.fuga = 2;"
	if iscriptTag.Body != wantBody {
		t.Errorf("iscript.Body = %q, want %q", iscriptTag.Body, wantBody)
	}
	endTag, ok := result[1].(TagObject)
	if !ok || endTag.Name != "endscript" {
		t.Fatalf("result[1] = %+v, want endscript tag", result[1])
	}
	textObj, ok := result[2].(TextObject)
	if !ok || textObj.Val != "after" {
		t.Fatalf("result[2] = %+v, want text 'after'", result[2])
	}
}

func TestParserIScriptBodyAtSign(t *testing.T) {
	src := "@iscript\nf.hoge = 1;\n@endscript"
	ks := &KS{}
	result, _, err := ks.ParseScenario(src)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2; result=%+v", len(result), result)
	}
	iscriptTag, ok := result[0].(TagObject)
	if !ok || iscriptTag.Name != "iscript" {
		t.Fatalf("result[0] = %+v, want iscript tag", result[0])
	}
	if want := "f.hoge = 1;"; iscriptTag.Body != want {
		t.Errorf("iscript.Body = %q, want %q", iscriptTag.Body, want)
	}
}

// TestParserSpacedEquals guards against a regression in joinSpacedEquals
// (parser.go): an earlier version silently dropped any "key = value"
// attribute (whitespace touching either side of "=") from TagObject.Pm
// entirely, via an off-by-one in a since-replaced index-arithmetic merge
// loop. This wasn't a theoretical edge case — the bundled
// example/game/resources/senarios/title.ks has "@wait time = 200" and
// tyrano.ks has "[freeimage layer = %layer]", both of which silently did
// nothing before this fix.
func TestParserSpacedEquals(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want map[string]string
	}{
		{"space on both sides", "@wait time = 200", map[string]string{"time": "200"}},
		{"space only before equals", "@wait time =200", map[string]string{"time": "200"}},
		{"space only after equals", "@wait time= 200", map[string]string{"time": "200"}},
		{"space around a quoted value", `@glink text = "Yes"`, map[string]string{"text": "Yes"}},
		{"no space still works", "@wait time=200", map[string]string{"time": "200"}},
		{"multiple attrs, only one spaced", `@glink text="Yes" target = "*a"`, map[string]string{"text": "Yes", "target": "*a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ks := &KS{}
			result, _, err := ks.ParseScenario(c.src)
			if err != nil {
				t.Fatalf("%+v", err)
			}
			if len(result) != 1 {
				t.Fatalf("len(result) = %d, want 1; result=%+v", len(result), result)
			}
			tag, ok := result[0].(TagObject)
			if !ok {
				t.Fatalf("result[0] = %+v, want a TagObject", result[0])
			}
			if !reflect.DeepEqual(tag.Pm, c.want) {
				t.Errorf("Pm = %+v, want %+v", tag.Pm, c.want)
			}
		})
	}
}

// TestParserQuotedValuesPreserved guards the two lossy substitutions
// makeTag used to perform on quoted attribute values: a space inside quotes
// was deleted outright, and an "=" inside quotes was swapped for "#" and
// then mapped back with a blanket ReplaceAll("#", "="), which corrupted any
// value that legitimately contained a "#".
func TestParserQuotedValuesPreserved(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want map[string]string
	}{
		{
			"spaces inside quotes survive",
			`@ptext name="n" text="Hello World"`,
			map[string]string{"name": "n", "text": "Hello World"},
		},
		{
			"a literal # is not turned into =",
			`@ptext color="#FF0000"`,
			map[string]string{"color": "#FF0000"},
		},
		{
			"# and spaces together",
			`@ptext text="a # b" color="#0f0"`,
			map[string]string{"text": "a # b", "color": "#0f0"},
		},
		{
			// The reason "=" needs hiding at all: it must not be mistaken
			// for the key/value separator while tokenizing.
			"an = inside quotes stays a value character",
			`@eval exp="f.a = 1"`,
			map[string]string{"exp": "f.a = 1"},
		},
		{
			"repeated = inside quotes (the [if] case)",
			`@if exp="1==1"`,
			map[string]string{"exp": "1==1"},
		},
		{
			// Unquoted whitespace around an attribute is still ordinary
			// separator whitespace and must still be trimmed, even though
			// quoted whitespace is now preserved.
			"quoted padding kept, unquoted padding trimmed",
			`@ptext text = "  padded  "`,
			map[string]string{"text": "  padded  "},
		},
		{
			// Control characters can't be smuggled in to forge an attribute
			// boundary: makeTag strips them from its input first.
			"raw stand-in characters in source are stripped, not honored",
			"@ptext text=\"a\x00b\x01c\"",
			map[string]string{"text": "abc"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ks := &KS{}
			result, _, err := ks.ParseScenario(c.src)
			if err != nil {
				t.Fatalf("%+v", err)
			}
			if len(result) != 1 {
				t.Fatalf("len(result) = %d, want 1; result=%+v", len(result), result)
			}
			tag, ok := result[0].(TagObject)
			if !ok {
				t.Fatalf("result[0] = %+v, want a TagObject", result[0])
			}
			if !reflect.DeepEqual(tag.Pm, c.want) {
				t.Errorf("Pm = %+v, want %+v", tag.Pm, c.want)
			}
		})
	}
}

func TestParserIfEndifMatchDepth(t *testing.T) {
	src := "[if exp=\"1==1\"]\nfoo\n[endif]"
	ks := &KS{}
	result, _, err := ks.ParseScenario(src)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3; result=%+v", len(result), result)
	}
	ifTag, ok := result[0].(TagObject)
	if !ok || ifTag.Name != "if" {
		t.Fatalf("result[0] = %+v, want if tag", result[0])
	}
	endifTag, ok := result[2].(TagObject)
	if !ok || endifTag.Name != "endif" {
		t.Fatalf("result[2] = %+v, want endif tag", result[2])
	}
	if ifTag.IfCount != 1 {
		t.Errorf("if.IfCount = %d, want 1", ifTag.IfCount)
	}
	if endifTag.IfCount != ifTag.IfCount {
		t.Errorf("endif.IfCount = %d, want match if.IfCount = %d", endifTag.IfCount, ifTag.IfCount)
	}
}

func TestParserSample(t *testing.T) {
	sample, err := os.ReadFile(path.Join(".", "test", "scene1.ks"))
	if err != nil {
		t.Errorf("cannot read scenario.")
	}

	ks := &KS{}
	t.Run("sample", func(t *testing.T) {
		r, _, err := ks.ParseScenario(string(sample))
		_, err = json.MarshalIndent(r, "", "    ")
		if err != nil {
			t.Errorf("%+v", err)
		}
	})
}
