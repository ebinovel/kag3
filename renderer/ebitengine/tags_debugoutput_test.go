package ebitengine

import (
	"io"
	"os"
	"testing"

	"github.com/ebinovel/kag3"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. Used to confirm the traceTags-gated debug
// fmt.Printf calls (input_hit.go, tags_flow.go, tags_character.go,
// tags_link.go, tags_background.go, renderer.go) stay silent by default.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	return string(out)
}

// TestDebugOutputSilentByDefault pins every tag handler's debug fmt.Printf
// to the traceTags (KAG3_TRACE_TAGS) gate every other per-item trace in this
// package uses (see macro.go) — ungated, hitButtons alone dumps the entire
// r.labels map on every click. With traceTags=false (the default), none of
// these should write anything to stdout.
func TestDebugOutputSilentByDefault(t *testing.T) {
	origTrace := traceTags
	traceTags = false
	t.Cleanup(func() { traceTags = origTrace })
	resetBG(t)

	r := newTestRendererWithImageFS(t, map[string][]byte{
		"bg/room.jpg": tinyPNG(t),
	})

	out := captureStdout(t, func() {
		// applyBGTag ([bg]/[bg2]) — wait="false" so it doesn't block on
		// ctx.y.Until with nothing driving target.IsEnd.
		tag := kag3.TagObject{Name: "bg", Pm: map[string]string{"storage": "room.jpg", "wait": "false"}}
		i := 0
		if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
			t.Fatalf("dispatchTag(bg): %v", err)
		}

		// handleCharaHide ([chara_hide]) — safe to call with no characters
		// registered; the loop over viewCharas is simply a no-op.
		hideTag := kag3.TagObject{Name: "chara_hide", Pm: map[string]string{"name": "nobody"}}
		if err := dispatchTag(r, fakeYield(), hideTag, &i, 0); err != nil {
			t.Fatalf("dispatchTag(chara_hide): %v", err)
		}
	})

	if out != "" {
		t.Errorf("stdout was not empty with traceTags=false: %q", out)
	}
}

// TestDebugOutputAppearsWhenTraceTagsEnabled confirms that guard is a gate,
// not a removal — with traceTags=true the same calls above still produce
// their trace output, matching KAG3_TRACE_TAGS's contract for the rest of
// the package.
func TestDebugOutputAppearsWhenTraceTagsEnabled(t *testing.T) {
	origTrace := traceTags
	traceTags = true
	t.Cleanup(func() { traceTags = origTrace })
	resetBG(t)

	r := newTestRendererWithImageFS(t, map[string][]byte{
		"bg/room.jpg": tinyPNG(t),
	})

	out := captureStdout(t, func() {
		tag := kag3.TagObject{Name: "bg", Pm: map[string]string{"storage": "room.jpg", "wait": "false"}}
		i := 0
		if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
			t.Fatalf("dispatchTag(bg): %v", err)
		}
	})

	if out == "" {
		t.Error("stdout was empty with traceTags=true — expected the bg trace line")
	}
}
