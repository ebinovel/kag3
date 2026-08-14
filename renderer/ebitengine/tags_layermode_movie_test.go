package ebitengine

import (
	"io/fs"
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

// resetLayerMovieState mirrors resetBgMovieState/resetMovieState: package-
// level state persists across tests in this suite, and a leftover
// *govid.Player here specifically means a leaked decode goroutine.
func resetLayerMovieState(t *testing.T) {
	t.Helper()
	stopLayerMovie()
	layerMovieLoop = false
	layerMovieBlend = ebiten.Blend{}
	layerMovieOpacity = 1
	t.Cleanup(func() {
		stopLayerMovie()
		layerMovieLoop = false
		layerMovieBlend = ebiten.Blend{}
		layerMovieOpacity = 1
	})
}

func TestHandleLayerModeMovieMissingVideo(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, nil)
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for [layermode_movie] with no video=, got nil")
	}
}

func TestHandleLayerModeMovieNoVideosFSSilentlyNoOps(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRenderer()
	r.fses = map[string]fs.FS{}
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "op.webm"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v (expected a silent no-op)", err)
	}
	if layerMovieVideo != nil {
		t.Error("layerMovieVideo should stay nil when \"videos\" fs.FS is unset")
	}
}

// TestHandleLayerModeMovieDefaults covers the documented defaults:
// loop="true" and mode="multiply" when neither attribute is given.
// time="0" keeps the fade-in's y.Until resolving on the very first
// predicate check, the same reasoning TestHandleBgMovieDefaultsToLoopingTrue
// uses.
func TestHandleLayerModeMovieDefaults(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode_movie dispatch error: %v", err)
	}
	if layerMovieVideo == nil {
		t.Fatal("layerMovieVideo is nil after a successful [layermode_movie]")
	}
	if !layerMovieLoop {
		t.Error("layerMovieLoop = false, want true (the loop= default)")
	}
	if layerMovieBlend != blendMultiply {
		t.Errorf("layerMovieBlend = %+v, want blendMultiply (the mode= default)", layerMovieBlend)
	}
}

func TestHandleLayerModeMovieExplicitModeScreen(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0", "mode": "screen"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode_movie dispatch error: %v", err)
	}
	if layerMovieBlend != blendScreen {
		t.Errorf("layerMovieBlend = %+v, want blendScreen", layerMovieBlend)
	}
}

func TestHandleLayerModeMovieUnsupportedModeErrors(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0", "mode": "bogus"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for an unsupported layermode_movie mode")
	}
}

// TestHandleLayerModeMovieOpacityScalesAlpha covers opacity= (0-255): a
// static multiplier on top of the fade-in, applied via layerMovieOpacity.
func TestHandleLayerModeMovieOpacityScalesAlpha(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0", "opacity": "128"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode_movie dispatch error: %v", err)
	}
	want := 128.0 / 255.0
	if layerMovieOpacity != want {
		t.Errorf("layerMovieOpacity = %v, want %v", layerMovieOpacity, want)
	}
}

// TestHandleLayerModeMovieSpeedIgnoredDoesNotError covers speed=: govid has
// no playback-rate API at all (confirmed against its Player type — only
// Play/Pause/SetLoop/State exist), so this is accepted and ignored rather
// than erroring.
func TestHandleLayerModeMovieSpeedIgnoredDoesNotError(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0", "speed": "2.0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode_movie dispatch error = %v, want nil (speed= should be ignored, not error)", err)
	}
}

// TestFreeLayerModeAllStopsLayerModeMovie documents kag3's own
// interpretation of an undocumented gap: upstream Tyrano's own
// [layermode_movie] reference never says how to stop a (default-looping)
// instance mid-scene, and there is no separate [stop_layermode_movie]/
// [wait_layermode_movie] tag pair in the official V6 tag list. [free_layermode]
// with no layer= already means "reset every per-layer effect" — extending
// that to also stop an active [layermode_movie] is the least surprising
// reading, and gives scripts *some* way to end a looping one without
// waiting for a full scene/title transition. A targeted
// [free_layermode layer=...] (an existing, unrelated layer's blend) must
// NOT stop it — only the "reset everything" form does.
func TestFreeLayerModeAllStopsLayerModeMovie(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode_movie dispatch error: %v", err)
	}
	if layerMovieVideo == nil {
		t.Fatal("layerMovieVideo is nil after a successful [layermode_movie]")
	}

	freeTag := kag3.TagObject{Name: "free_layermode", Pm: map[string]string{}}
	i = 0
	if err := dispatchTag(r, fakeYield(), freeTag, &i, 0); err != nil {
		t.Fatalf("free_layermode dispatch error: %v", err)
	}
	if layerMovieVideo != nil {
		t.Error("layerMovieVideo should be nil after [free_layermode] with no layer=")
	}
}

func TestFreeLayerModeTargetedLayerDoesNotStopLayerModeMovie(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "layermode_movie", Pm: map[string]string{"video": "fx.webm", "time": "0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode_movie dispatch error: %v", err)
	}

	freeTag := kag3.TagObject{Name: "free_layermode", Pm: map[string]string{"layer": "1"}}
	i = 0
	if err := dispatchTag(r, fakeYield(), freeTag, &i, 0); err != nil {
		t.Fatalf("free_layermode dispatch error: %v", err)
	}
	if layerMovieVideo == nil {
		t.Error("layerMovieVideo should survive a targeted [free_layermode layer=...]")
	}
}

func TestDrawLayerMovieDoesNotPanic(t *testing.T) {
	resetLayerMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"fx.webm": bakerVP8WebM(t)})
	video, closers, err := openMovie(r, "fx.webm")
	if err != nil {
		t.Fatalf("openMovie failed on a real VP8/WebM fixture: %v", err)
	}
	layerMovieVideo = video
	layerMovieClosers = closers
	layerMovieBlend = blendScreen
	layerMovieOpacity = 1

	buf := newTestImage(1920, 1080)
	drawLayerMovie(buf) // must not panic

	layerMovieVideo = nil
	drawLayerMovie(buf) // must not panic when nothing is playing either
}
