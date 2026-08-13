package ebitengine

import (
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

// bakerVP8WebM/bakerAV1MP4 return testdata/movie fixtures borrowed from
// govid's own examples/videos/ (MIT-licensed): a real, short (2.4s,
// 1280x720) VP8-in-WebM clip for exercising actual decoding, and an
// AV1-in-MP4 clip specifically to prove [movie] rejects AV1 rather than
// silently producing garbage frames (govid's own README: its AV1 decoder
// is "not usable — parses structure, output is wrong").
func bakerVP8WebM(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/movie/baker_vp8.webm")
	if err != nil {
		t.Fatalf("failed to read testdata fixture: %v", err)
	}
	return b
}

func bakerAV1MP4(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/movie/baker_av1.mp4")
	if err != nil {
		t.Fatalf("failed to read testdata fixture: %v", err)
	}
	return b
}

func newTestRendererWithVideoFS(t *testing.T, files map[string][]byte) *Renderer {
	t.Helper()
	mapFS := fstest.MapFS{}
	for name, data := range files {
		mapFS[name] = &fstest.MapFile{Data: data}
	}
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"videos": mapFS}
	return r
}

// resetMovieState clears every [movie]-related package-level var, both
// before and after a test — package-level state persists across tests in
// this suite (see other tags_*_test.go files' own reset helpers), and a
// leftover *govid.Player here specifically means a leaked decode
// goroutine, not just stale data.
func resetMovieState(t *testing.T) {
	t.Helper()
	stopMovie()
	t.Cleanup(func() { stopMovie() })
}

func TestHandleMovieMissingStorage(t *testing.T) {
	resetMovieState(t)
	r := newTestRendererWithVideoFS(t, nil)
	tag := kag3.TagObject{Name: "movie", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for [movie] with no storage=, got nil")
	}
}

// TestHandleMovieNoVideosFSSilentlyNoOps covers renderer/ebitengine being
// shared with projects that have no use for [movie] (same contract
// SpeechSynth==nil gets, tags_speech.go) — a project whose fs.FS map
// (Manager.Init) never registered "videos" at all must not crash or error,
// just skip the tag.
func TestHandleMovieNoVideosFSSilentlyNoOps(t *testing.T) {
	resetMovieState(t)
	r := newTestRenderer()
	r.fses = map[string]fs.FS{} // no "videos" key
	tag := kag3.TagObject{Name: "movie", Pm: map[string]string{"storage": "op.webm"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v (expected a silent no-op)", err)
	}
	if movieVideo != nil {
		t.Error("movieVideo should stay nil when \"videos\" fs.FS is unset")
	}
}

func TestHandleMovieUnsupportedExtension(t *testing.T) {
	resetMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"clip.txt": []byte("not a video")})
	tag := kag3.TagObject{Name: "movie", Pm: map[string]string{"storage": "clip.txt"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for an unsupported file extension, got nil")
	}
}

func TestOpenMovieRejectsAV1MP4(t *testing.T) {
	resetMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"op.mp4": bakerAV1MP4(t)})
	_, closers, err := openMovie(r, "op.mp4")
	closeQuietly(closers...)
	if err == nil {
		t.Fatal("expected an error for an AV1-encoded MP4, got nil")
	}
}

// TestOpenMovieDecodesRealWebm exercises the real govid decode path end to
// end: a genuine VP8/WebM clip must produce a VideoImage sized to the
// stream's actual resolution (1280x720, confirmed via govid's own
// VideoInfo against this fixture), and every returned closer must close
// cleanly in order (player, demuxer, file).
func TestOpenMovieDecodesRealWebm(t *testing.T) {
	resetMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"op.webm": bakerVP8WebM(t)})
	video, closers, err := openMovie(r, "op.webm")
	if err != nil {
		t.Fatalf("openMovie failed on a real VP8/WebM fixture: %v", err)
	}
	defer closeQuietly(closers...)
	if len(closers) != 3 {
		t.Errorf("len(closers) = %d, want 3 (player, demuxer, file)", len(closers))
	}
	img := video.Image()
	if b := img.Bounds(); b.Dx() != 1280 || b.Dy() != 720 {
		t.Errorf("decoded frame bounds = %v, want 1280x720", b)
	}
}

// TestHandleMovieWaitFalseDoesNotBlock covers loop=/wait=false, the shape
// a looping background movie needs (a wait=true loop=true [movie] would
// never return) — dispatchTag must return immediately without needing
// fakeYield()'s isWait-flipping behavior to unblock anything.
func TestHandleMovieWaitFalseDoesNotBlock(t *testing.T) {
	resetMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "movie", Pm: map[string]string{"storage": "bg.webm", "wait": "false", "loop": "true"}}
	i := 0
	if err := dispatchTag(r, nil, tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if movieVideo == nil {
		t.Fatal("movieVideo is nil after a successful [movie wait=false]")
	}
}

// TestStopMovieTearsDownRealPlayer is the resource-leak guard for
// goToTitle, which calls stopMovie unconditionally (see title_flow.go) so
// backing out to title mid-movie can't leave a decode goroutine running
// forever. Exercised directly against stopMovie rather than through a full
// goToTitle() call: goToTitle also resets a wide set of unrelated
// package-level globals (tick/oldTick/isJump/viewCharas/...) that would
// need careful restoration afterward to avoid polluting every later test
// in this package — see TestGoToTitleEscapesTextWaitingOnIsWait's own
// comment (confirm_title_flow_test.go) for a concrete instance of that
// exact hazard. stopMovie's own behavior is what matters here regardless
// of which caller reaches it.
func TestStopMovieTearsDownRealPlayer(t *testing.T) {
	resetMovieState(t)
	r := newTestRendererWithVideoFS(t, map[string][]byte{"bg.webm": bakerVP8WebM(t)})
	tag := kag3.TagObject{Name: "movie", Pm: map[string]string{"storage": "bg.webm", "wait": "false"}}
	i := 0
	if err := dispatchTag(r, nil, tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if movieVideo == nil {
		t.Fatal("movieVideo is nil after a successful [movie wait=false]")
	}
	stopMovie()
	if movieVideo != nil {
		t.Error("movieVideo should be nil after stopMovie")
	}
	if movieClosers != nil {
		t.Error("movieClosers should be nil after stopMovie")
	}
}

func TestMovieShouldFinish(t *testing.T) {
	cases := []struct {
		name                         string
		skip, clicked, stopped       bool
		wantFinish, wantResetOldTick bool
	}{
		{"skip click ends it, no oldTick reset", true, true, false, true, false},
		{"skip click also covers a coincident natural stop", true, true, true, true, false},
		{"natural stop resets oldTick", true, false, true, true, true},
		{"natural stop resets oldTick even with skip disabled", false, false, true, true, true},
		{"unskippable click does nothing", false, true, false, false, false},
		{"still playing, no click", true, false, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			finish, resetOldTick := movieShouldFinish(c.skip, c.clicked, c.stopped)
			if finish != c.wantFinish || resetOldTick != c.wantResetOldTick {
				t.Errorf("movieShouldFinish(%v, %v, %v) = (%v, %v), want (%v, %v)",
					c.skip, c.clicked, c.stopped, finish, resetOldTick, c.wantFinish, c.wantResetOldTick)
			}
		})
	}
}
