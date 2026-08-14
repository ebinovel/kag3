package ebitengine

import (
	"io/fs"
	"testing"

	"github.com/ebinovel/kag3"
)

// resetBgMovieState mirrors resetMovieState ([movie]'s own equivalent,
// tags_movie_test.go): package-level state persists across tests in this
// suite, and a leftover *govid.Player here specifically means a leaked
// decode goroutine, not just stale data.
func resetBgMovieState(t *testing.T) {
	t.Helper()
	stopBgMovie()
	bgMovieLoop = false
	t.Cleanup(func() {
		stopBgMovie()
		bgMovieLoop = false
	})
}

func TestHandleBgMovieMissingStorage(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRendererWithVideoFS(t, nil)
	tag := kag3.TagObject{Name: "bgmovie", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for [bgmovie] with no storage=, got nil")
	}
}

// TestHandleBgMovieNoVideosFSSilentlyNoOps mirrors
// TestHandleMovieNoVideosFSSilentlyNoOps: a project whose fs.FS map never
// registered "videos" at all must not crash or error, just skip the tag —
// same contract [movie] has (errVideosFSUnset, tags_movie.go).
func TestHandleBgMovieNoVideosFSSilentlyNoOps(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRenderer()
	r.fses = map[string]fs.FS{}
	tag := kag3.TagObject{Name: "bgmovie", Pm: map[string]string{"storage": "op.webm"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v (expected a silent no-op)", err)
	}
	if bgMovieVideo != nil {
		t.Error("bgMovieVideo should stay nil when \"videos\" fs.FS is unset")
	}
}

// TestHandleBgMovieDefaultsToLoopingTrue covers [bgmovie]'s loop= default
// being true (the opposite of [movie]'s false) — a background video is
// meant to play indefinitely until [stop_bgmovie] unless told otherwise.
// time="0" keeps the fade-in's y.Until resolving on the very first
// predicate check (see bgMovieAlpha: a zero-duration fade is immediately
// "complete"), so this doesn't need loop=false's own end-of-clip detection
// or a real coroutine driving t forward.
func TestHandleBgMovieDefaultsToLoopingTrue(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "bgmovie", Pm: map[string]string{"storage": "bg.webm", "time": "0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("bgmovie dispatch error: %v", err)
	}
	if bgMovieVideo == nil {
		t.Fatal("bgMovieVideo is nil after a successful [bgmovie]")
	}
	if !bgMovieLoop {
		t.Error("bgMovieLoop = false, want true (the loop= default)")
	}
}

func TestHandleBgMovieExplicitLoopFalse(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "bgmovie", Pm: map[string]string{"storage": "bg.webm", "time": "0", "loop": "false"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("bgmovie dispatch error: %v", err)
	}
	if bgMovieLoop {
		t.Error("bgMovieLoop = true, want false (loop=\"false\" was given)")
	}
}

// TestHandleWaitBgMovieLoopingReturnsImmediately covers [wait_bgmovie]'s
// guard against blocking forever on a looping (the default) [bgmovie]:
// govid's Player never reaches StateStopped with SetLoop(true), so waiting
// on bgMovieFinished would hang the coroutine forever without this check.
// Dispatched with a nil coro.Yield (like TestHandleMovieWaitFalseDoesNotBlock
// does for [movie wait=false]) — if this ever actually tried to block, the
// nil y would panic on the first call, making a regression fail loudly
// rather than hang.
func TestHandleWaitBgMovieLoopingReturnsImmediately(t *testing.T) {
	resetBgMovieState(t)
	bgMovieLoop = true
	tag := kag3.TagObject{Name: "wait_bgmovie", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(newTestRenderer(), nil, tag, &i, 0); err != nil {
		t.Fatalf("wait_bgmovie dispatch error: %v", err)
	}
}

// TestHandleWaitBgMovieNilVideoReturnsImmediately covers [wait_bgmovie]
// with no [bgmovie] currently playing at all (never started, or already
// stopped) — nothing to wait for, so this must not block either.
func TestHandleWaitBgMovieNilVideoReturnsImmediately(t *testing.T) {
	resetBgMovieState(t)
	tag := kag3.TagObject{Name: "wait_bgmovie", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(newTestRenderer(), nil, tag, &i, 0); err != nil {
		t.Fatalf("wait_bgmovie dispatch error: %v", err)
	}
}

// TestHandleWaitBgMovieAlreadyFinishedReturnsImmediately covers the
// non-looping case once bgMovieFinished is already true (e.g. the clip
// naturally ended, or [stop_bgmovie]/finishBgMovie already ran) — the
// y.Until predicate is satisfied on its very first check, so this resolves
// without needing a real coroutine to drive playback forward.
func TestHandleWaitBgMovieAlreadyFinishedReturnsImmediately(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	video, closers, err := openMovie(r, "bg.webm")
	if err != nil {
		t.Fatalf("openMovie failed on a real VP8/WebM fixture: %v", err)
	}
	bgMovieVideo = video
	bgMovieClosers = closers
	bgMovieLoop = false
	bgMovieFinished = true

	tag := kag3.TagObject{Name: "wait_bgmovie", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("wait_bgmovie dispatch error: %v", err)
	}
}

// TestHandleStopBgMovieTearsDownRealPlayer mirrors
// TestStopMovieTearsDownRealPlayer: [stop_bgmovie] must release the decode
// goroutine, not just clear the video reference.
func TestHandleStopBgMovieTearsDownRealPlayer(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "bgmovie", Pm: map[string]string{"storage": "bg.webm", "time": "0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("bgmovie dispatch error: %v", err)
	}
	if bgMovieVideo == nil {
		t.Fatal("bgMovieVideo is nil after a successful [bgmovie]")
	}

	stopTag := kag3.TagObject{Name: "stop_bgmovie", Pm: map[string]string{}}
	i = 0
	if err := dispatchTag(r, fakeYield(), stopTag, &i, 0); err != nil {
		t.Fatalf("stop_bgmovie dispatch error: %v", err)
	}
	if bgMovieVideo != nil {
		t.Error("bgMovieVideo should be nil after [stop_bgmovie]")
	}
	if bgMovieClosers != nil {
		t.Error("bgMovieClosers should be nil after [stop_bgmovie]")
	}
}

// TestBgMovieAlphaLinearFade covers bgMovieAlpha's pure fade-in math
// directly: 0 before the fade starts, a linear ramp midway through, 1 once
// the duration has elapsed, and always 1 for a zero/negative duration (the
// same "apply immediately" convention startFade uses for ms<=0,
// tags_audio.go). Named "tt" rather than "t" for the *testing.T parameter,
// like TestHandleAnimMovesCharaOverTime (tags_animation_test.go): this test
// assigns the package-level tick counter "t" directly.
func TestBgMovieAlphaLinearFade(tt *testing.T) {
	defer func() { t, bgMovieFadeStartT, bgMovieFadeDurTicks = 0, 0, 0 }()

	cases := []struct {
		name         string
		startT, durT int
		now          int
		want         float64
	}{
		{"zero duration", 100, 0, 100, 1},
		{"before start", 100, 60, 50, 0},
		{"exactly at start", 100, 60, 100, 0},
		{"halfway", 100, 60, 130, 0.5},
		{"exactly done", 100, 60, 160, 1},
		{"long past done", 100, 60, 500, 1},
	}
	for _, c := range cases {
		tt.Run(c.name, func(tt *testing.T) {
			bgMovieFadeStartT, bgMovieFadeDurTicks = c.startT, c.durT
			t = c.now
			if got := bgMovieAlpha(); got != c.want {
				tt.Errorf("bgMovieAlpha() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestDrawBgMovieDoesNotPanic is a headless no-panic check (this package's
// existing style for draw*, see TestDrawPTextsWithBgImageDoesNotPanic) —
// not a pixel comparison.
func TestDrawBgMovieDoesNotPanic(t *testing.T) {
	resetBgMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	video, closers, err := openMovie(r, "bg.webm")
	if err != nil {
		t.Fatalf("openMovie failed on a real VP8/WebM fixture: %v", err)
	}
	bgMovieVideo = video
	bgMovieClosers = closers

	buf := newTestImage(1920, 1080)
	drawBgMovie(buf) // must not panic

	bgMovieVideo = nil
	drawBgMovie(buf) // must not panic when nothing is playing either
}
