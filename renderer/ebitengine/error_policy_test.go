package ebitengine

import (
	"testing"

	"github.com/eihigh/coro"
)

// TestTagHandlerErrorDoesNotPanicAndScriptContinues is the regression test
// for #19: initScript's loop used to panic(err) on any tag handler error,
// so one malformed attribute value anywhere in a script — e.g.
// [delay speed="user"] (handleDelay's strconv.Atoi fails on a non-numeric
// speed=, tags_message.go) — crashed the whole game, unlike an *unknown*
// tag name, which dispatchTag already just logs and continues past. The
// loop now logs and skips the failing tag instead, matching that.
func TestTagHandlerErrorDoesNotPanicAndScriptContinues(t *testing.T) {
	m := newTestManager(t, map[string]string{
		"main.ks": "[delay speed=\"user\"]\nafter delay",
	})
	if err := m.LoadScript("main.ks"); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	r := &Renderer{
		texts:          make(map[int][]Text),
		vm:             newVM(),
		manager:        m,
		scripts:        m.Senario,
		labels:         m.Labels,
		currentStorage: m.CurrentStorage,
	}
	isJump = false
	isFirst = false
	r.initScript()
	co = coro.New(loop)
	isFirst = true
	defer func() { isFirst = false }()

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("script execution panicked: %v", rec)
		}
	}()
	for i := 0; i < 5; i++ {
		if !co.Next() {
			break
		}
	}

	found := false
	for _, segs := range r.texts {
		for _, seg := range segs {
			if seg.Text == "after delay" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("r.texts = %+v, want a segment with %q (the tag after the failing one must still run)", r.texts, "after delay")
	}
}
