package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

func newTestManager(t *testing.T, senarios map[string]string) *kag3.Manager {
	t.Helper()
	senarioFS := fstest.MapFS{}
	for name, content := range senarios {
		senarioFS[name] = &fstest.MapFile{Data: []byte(content)}
	}
	m := &kag3.Manager{}
	m.Init(map[string]fs.FS{
		"resources": fstest.MapFS{},
		"senarios":  senarioFS,
	})
	return m
}

// TestCallReturnAcrossFiles drives a full [call storage=... target=...] /
// [return] round trip through two in-memory .ks files, exercising the
// production execItem loop exactly as initScript does (r.scripts/r.labels
// swapped out from under a live "for i" loop by loadScript).
func TestCallReturnAcrossFiles(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks": "[call storage=\"sub.ks\" target=\"*greet\"]\nafter call",
		"sub.ks":  "*greet\nin sub\n[return]",
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

	for i := 0; i < len(r.scripts); i++ {
		if err := r.execItem(fakeYield(), r.scripts, &i, 0); err != nil {
			t.Fatalf("execItem error: %v", err)
		}
	}

	var seen []string
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text != "" {
				seen = append(seen, seg.Text)
			}
		}
	}
	want := map[string]bool{"in sub": false, "after call": false}
	for _, s := range seen {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for text, found := range want {
		if !found {
			t.Errorf("expected text %q to have been rendered; got %v", text, seen)
		}
	}

	if r.currentStorage != "main.ks" {
		t.Errorf("currentStorage after return = %q, want %q", r.currentStorage, "main.ks")
	}
	if len(r.callStack) != 0 {
		t.Errorf("callStack after return = %+v, want empty (call/return should balance)", r.callStack)
	}
}

func TestCallPushesReturnAddress(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "test.ks"
	r.labels = map[string]kag3.LabelInfo{"sub": {Name: "sub", Index: 5}}

	tag := kag3.TagObject{Name: "call", Line: 2, Pm: map[string]string{"target": "*sub"}}
	i := 2
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}

	if len(r.callStack) != 1 {
		t.Fatalf("callStack = %+v, want 1 frame", r.callStack)
	}
	got := r.callStack[0]
	want := callFrame{Storage: "test.ks", Index: 3}
	if got != want {
		t.Errorf("callStack[0] = %+v, want %+v", got, want)
	}
	if i != 5 {
		t.Errorf("i after call = %d, want target label index 5", i)
	}
}

func TestReturnWithEmptyStackIsNoop(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "test.ks"

	tag := kag3.TagObject{Name: "return", Line: 0, Pm: map[string]string{}}
	i := 7
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if i != 7 {
		t.Errorf("i after no-op return = %d, want unchanged 7", i)
	}
}
