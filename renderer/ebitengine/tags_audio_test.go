package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

// silentPlayer builds a real *audio.Player from raw PCM silence (bypassing
// vorbis decode / file I/O entirely) so state-management logic — pause,
// resume, volume, fades — can be exercised without a checked-in audio
// fixture. ~0.1s of 16-bit stereo silence at the context's sample rate.
func silentPlayer(t *testing.T) *audio.Player {
	t.Helper()
	buf := make([]byte, 44100/10*4)
	return audioContext.NewPlayerFromBytes(buf)
}

func TestVolFraction(t *testing.T) {
	cases := map[int]float64{0: 0, 50: 0.5, 100: 1}
	for vol, want := range cases {
		if got := volFraction(vol); got != want {
			t.Errorf("volFraction(%d) = %v, want %v", vol, got, want)
		}
	}
}

// TestStartFadeRampsAndCompletes uses "tt" instead of "t" for the
// *testing.T parameter because this test needs to read/set the
// package-level tick counter "t" that startFade/stepAudioFades compare
// against (same reason as TestHandleWTCompletesWhenElapsed).
func TestStartFadeRampsAndCompletes(tt *testing.T) {
	activeFades = map[string]*audioFade{}
	t = 0

	p := silentPlayer(tt)
	completed := false
	startFade("test", p, 0, 1, 100, func() { completed = true })

	if _, ok := activeFades["test"]; !ok {
		tt.Fatal("expected fade to be registered")
	}
	t += 3 // partway through a 100ms fade (TPS default 60 -> 6 ticks)
	stepAudioFades()
	if completed {
		tt.Fatal("fade completed too early")
	}
	if v := p.Volume(); v <= 0 || v >= 1 {
		tt.Errorf("mid-fade volume = %v, want strictly between 0 and 1", v)
	}

	t += 1000 // well past the fade duration
	stepAudioFades()
	if !completed {
		tt.Error("expected onComplete to have fired")
	}
	if v := p.Volume(); v != 1 {
		tt.Errorf("final volume = %v, want 1", v)
	}
	if _, ok := activeFades["test"]; ok {
		tt.Error("expected fade to be removed from activeFades once complete")
	}
}

func TestStartFadeZeroDurationAppliesImmediately(t *testing.T) {
	activeFades = map[string]*audioFade{}
	p := silentPlayer(t)
	completed := false
	startFade("test", p, 0, 1, 0, func() { completed = true })
	if !completed {
		t.Error("expected onComplete to fire immediately for ms<=0")
	}
	if v := p.Volume(); v != 1 {
		t.Errorf("volume = %v, want 1", v)
	}
	if _, ok := activeFades["test"]; ok {
		t.Error("expected no fade registered for ms<=0")
	}
}

func TestHandleStopBGM(t *testing.T) {
	r := newTestRenderer()
	currentBGM = &kag3.BGM{Player: silentPlayer(t)}
	tag := kag3.TagObject{Name: "stopbgm"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if currentBGM != nil {
		t.Error("expected currentBGM to be nil after stopbgm")
	}
}

func TestHandlePauseResumeBGM(t *testing.T) {
	r := newTestRenderer()
	p := silentPlayer(t)
	p.Play()
	currentBGM = &kag3.BGM{Player: p}

	pause := kag3.TagObject{Name: "pausebgm"}
	i := 0
	if err := dispatchTag(r, fakeYield(), pause, &i, 0); err != nil {
		t.Fatalf("pausebgm error: %v", err)
	}
	if p.IsPlaying() {
		t.Error("expected player to be paused")
	}

	resume := kag3.TagObject{Name: "resumebgm"}
	if err := dispatchTag(r, fakeYield(), resume, &i, 0); err != nil {
		t.Fatalf("resumebgm error: %v", err)
	}
	if !p.IsPlaying() {
		t.Error("expected player to be playing again after resumebgm")
	}
}

func TestHandleWBGMCompletesWhenNotPlaying(t *testing.T) {
	r := newTestRenderer()
	currentBGM = &kag3.BGM{Player: silentPlayer(t)} // never Play()ed
	tag := kag3.TagObject{Name: "wbgm"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
}

func TestHandleBGMOptChangesVolume(t *testing.T) {
	r := newTestRenderer()
	currentBGM = &kag3.BGM{Player: silentPlayer(t), Volume: 100}
	tag := kag3.TagObject{Name: "bgmopt", Pm: map[string]string{"volume": "50"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if currentBGM.Volume != 50 {
		t.Errorf("currentBGM.Volume = %d, want 50", currentBGM.Volume)
	}
	if v := currentBGM.Player.Volume(); v != 0.5 {
		t.Errorf("player volume = %v, want 0.5", v)
	}
}

func TestHandlePlayBGMMissingFileReturnsError(t *testing.T) {
	r := newTestRendererWithFS(t)
	tag := kag3.TagObject{Name: "playbgm", Pm: map[string]string{"storage": "missing.ogg"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for a missing bgm file, got nil")
	}
}

func TestHandleStopSEAllAndSpecific(t *testing.T) {
	r := newTestRenderer()
	ses = map[string]*kag3.BGM{
		"a": {Player: silentPlayer(t)},
		"b": {Player: silentPlayer(t)},
	}

	tag := kag3.TagObject{Name: "stopse", Pm: map[string]string{"buf": "a"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if _, ok := ses["a"]; ok {
		t.Error("expected buf \"a\" to be removed")
	}
	if _, ok := ses["b"]; !ok {
		t.Error("expected buf \"b\" to remain untouched")
	}

	tag = kag3.TagObject{Name: "stopse", Pm: map[string]string{}}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(ses) != 0 {
		t.Errorf("ses = %+v, want empty after stopse with no buf", ses)
	}
}

func TestHandlePauseResumeSE(t *testing.T) {
	r := newTestRenderer()
	p := silentPlayer(t)
	p.Play()
	ses = map[string]*kag3.BGM{"a": {Player: p}}

	pause := kag3.TagObject{Name: "pausese", Pm: map[string]string{"buf": "a"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), pause, &i, 0); err != nil {
		t.Fatalf("pausese error: %v", err)
	}
	if p.IsPlaying() {
		t.Error("expected se to be paused")
	}

	resume := kag3.TagObject{Name: "resumese", Pm: map[string]string{"buf": "a"}}
	if err := dispatchTag(r, fakeYield(), resume, &i, 0); err != nil {
		t.Fatalf("resumese error: %v", err)
	}
	if !p.IsPlaying() {
		t.Error("expected se to be playing again after resumese")
	}
}

func TestHandleWSESpecificAndAll(t *testing.T) {
	r := newTestRenderer()
	ses = map[string]*kag3.BGM{"a": {Player: silentPlayer(t)}} // never Play()ed

	tag := kag3.TagObject{Name: "wse", Pm: map[string]string{"buf": "a"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("wse (specific) error: %v", err)
	}

	tag = kag3.TagObject{Name: "wse", Pm: map[string]string{}}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("wse (all) error: %v", err)
	}
}

func TestHandleSEOptChangesVolume(t *testing.T) {
	r := newTestRenderer()
	ses = map[string]*kag3.BGM{"a": {Player: silentPlayer(t), Volume: 100}}
	tag := kag3.TagObject{Name: "seopt", Pm: map[string]string{"buf": "a", "volume": "25"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if ses["a"].Volume != 25 {
		t.Errorf("ses[a].Volume = %d, want 25", ses["a"].Volume)
	}
	if v := ses["a"].Player.Volume(); v != 0.25 {
		t.Errorf("player volume = %v, want 0.25", v)
	}
}

func TestHandleChangeVolTargetsBGMByDefault(t *testing.T) {
	r := newTestRenderer()
	currentBGM = &kag3.BGM{Player: silentPlayer(t), Volume: 100}
	tag := kag3.TagObject{Name: "changevol", Pm: map[string]string{"vol": "30"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if currentBGM.Volume != 30 {
		t.Errorf("currentBGM.Volume = %d, want 30", currentBGM.Volume)
	}
	if v := currentBGM.Player.Volume(); v != 0.3 {
		t.Errorf("player volume = %v, want 0.3", v)
	}
}

func TestHandleChangeVolTargetsSE(t *testing.T) {
	r := newTestRenderer()
	ses = map[string]*kag3.BGM{"a": {Player: silentPlayer(t), Volume: 100}}
	tag := kag3.TagObject{Name: "changevol", Pm: map[string]string{"buf": "a", "vol": "10"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if ses["a"].Volume != 10 {
		t.Errorf("ses[a].Volume = %d, want 10", ses["a"].Volume)
	}
	if v := ses["a"].Player.Volume(); v != 0.1 {
		t.Errorf("player volume = %v, want 0.1", v)
	}
}

func TestHandlePopopoMissingFileIsGraceful(t *testing.T) {
	r := newTestRendererWithFS(t)
	tag := kag3.TagObject{Name: "popopo"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("expected popopo to no-op gracefully when the file is missing, got error: %v", err)
	}
}

func TestHandleXchgBGMRegistersOldFadeEvenIfNewLoadFails(t *testing.T) {
	r := newTestRendererWithFS(t)
	activeFades = map[string]*audioFade{}
	oldPlayer := silentPlayer(t)
	oldPlayer.Play()
	currentBGM = &kag3.BGM{Player: oldPlayer}

	tag := kag3.TagObject{Name: "xchgbgm", Pm: map[string]string{"storage": "missing.ogg", "time": "100"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Fatal("expected an error for a missing xchgbgm target file")
	}
	if _, ok := activeFades["bgm_old"]; !ok {
		t.Error("expected the outgoing track's fade-out to still be registered under bgm_old")
	}
}

// newTestRendererWithFS is like newTestRenderer but with fses wired to
// empty in-memory filesystems, for exercising the missing-file error paths
// of audio handlers without depending on a real (and, per policy, uncommitted)
// audio fixture.
func newTestRendererWithFS(t *testing.T) *Renderer {
	t.Helper()
	r := newTestRenderer()
	r.fses = map[string]fs.FS{
		"bgms": fstest.MapFS{},
		"ses":  fstest.MapFS{},
	}
	return r
}
