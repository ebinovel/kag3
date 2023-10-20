package kag3

import (
	"os"
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
		ifCount: 0,
	}
	
	want = append(want, tag)
	label := LabelObject{
		Name: "label",
		Val: "スタート",
		Info: LabelInfo{
			Line: 1,
			index: 1,
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
		ifCount: 0,
	}
	want = append(want, tag)
	text := TextObject{
		Name: "text",
		Line: 3,
		Chara: CharacterInfo{
			Name: "",
			Face: "",
		},
		Val: "こんにちは。",
	}
	want = append(want, text)

	t.Run("first", func(t *testing.T) {
		if !reflect.DeepEqual(result, want) {
			t.Errorf("result %+v, want %+v", result, want)
		}
	})
}

func TestParserSample(t *testing.T) {
	sample, err := os.ReadFile(path.Join(".", "test", "scene1.ks"))
	if err != nil {
		t.Errorf("cannot read scenario.")
	}

	ks := &KS{}
	t.Run("sample", func(t *testing.T) {
		_, _, err := ks.ParseScenario(string(sample))
		if err != nil {
			t.Errorf("%+v", err)
		}
	})
}
