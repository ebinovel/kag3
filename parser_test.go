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
