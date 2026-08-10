package ebitengine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ebinovel/kag3"
)

// resetSpeechState clears every package-level var these tests touch, and
// restores it afterward — same rationale as resetVoiceState
// (tags_voice_test.go): tags_speech.go's state is process-lifetime by
// design, so leftovers from one test would corrupt a later one's result.
func resetSpeechState(t *testing.T) {
	t.Helper()
	origSynth := SpeechSynth
	origAutoPlay := speechAutoPlay
	origStyles := speechStyles
	origDefaultStyle := defaultSpeechStyle
	origSeq := speechSeq
	origLastApplied := lastAppliedSpeechSeq
	origSes := ses
	SpeechSynth = nil
	speechAutoPlay = false
	speechStyles = map[string]uint32{}
	defaultSpeechStyle = 0
	speechSeq = 0
	lastAppliedSpeechSeq = 0
	speechResultsMu.Lock()
	speechResults = nil
	speechResultsMu.Unlock()
	ses = map[string]*kag3.BGM{}
	t.Cleanup(func() {
		SpeechSynth = origSynth
		speechAutoPlay = origAutoPlay
		speechStyles = origStyles
		defaultSpeechStyle = origDefaultStyle
		speechSeq = origSeq
		lastAppliedSpeechSeq = origLastApplied
		ses = origSes
	})
}

// minimalMonoWAV16 builds the smallest valid 16-bit mono PCM WAV byte
// stream that satisfies wav.DecodeWithSampleRate, without depending on a
// real (and, per this package's own policy, uncommitted) audio fixture —
// same reasoning as tags_audio_test.go's silentPlayer, just at the byte
// level since this needs to survive an actual RIFF/WAVE parse rather than
// bypass one.
func minimalMonoWAV16(sampleRate int, samples []int16) []byte {
	dataSize := len(samples) * 2
	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(buf, binary.LittleEndian, uint16(2))            // block align
	binary.Write(buf, binary.LittleEndian, uint16(16))           // bits/sample
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	for _, s := range samples {
		binary.Write(buf, binary.LittleEndian, s)
	}
	return buf.Bytes()
}

// fakeWav wraps a []byte as the io.ReadCloser SpeechSynth stubs return,
// tracking whether Close was called.
type fakeWav struct {
	*bytes.Reader
	closed bool
}

func newFakeWav(b []byte) *fakeWav { return &fakeWav{Reader: bytes.NewReader(b)} }
func (w *fakeWav) Close() error    { w.closed = true; return nil }

// --- A. tag handlers ---

func TestHandleSpeakOnOffToggleAutoPlay(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "speak_on"}, &i, 0); err != nil {
		t.Fatalf("speak_on: %v", err)
	}
	if !speechAutoPlay {
		t.Error("speechAutoPlay = false after [speak_on], want true")
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "speak_off"}, &i, 0); err != nil {
		t.Fatalf("speak_off: %v", err)
	}
	if speechAutoPlay {
		t.Error("speechAutoPlay = true after [speak_off], want false")
	}
}

func TestHandleSpeakConfigSetsPerCharaAndDefaultStyle(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()
	i := 0
	perChara := kag3.TagObject{Name: "speak_config", Pm: map[string]string{"name": "akane", "style": "3"}}
	if err := dispatchTag(r, fakeYield(), perChara, &i, 0); err != nil {
		t.Fatalf("speak_config (name): %v", err)
	}
	if got, ok := speechStyles["akane"]; !ok || got != 3 {
		t.Errorf("speechStyles[akane] = (%d, %v), want (3, true)", got, ok)
	}
	global := kag3.TagObject{Name: "speak_config", Pm: map[string]string{"style": "8"}}
	if err := dispatchTag(r, fakeYield(), global, &i, 0); err != nil {
		t.Fatalf("speak_config (default): %v", err)
	}
	if defaultSpeechStyle != 8 {
		t.Errorf("defaultSpeechStyle = %d, want 8", defaultSpeechStyle)
	}
	// The per-character entry from the first call must survive the
	// default-only second call — same merge-not-replace contract as
	// [voconfig].
	if got := speechStyles["akane"]; got != 3 {
		t.Errorf("speechStyles[akane] = %d after an unrelated default-only call, want it left at 3", got)
	}
}

func TestHandleSpeakConfigRequiresStyle(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()
	i := 0
	tag := kag3.TagObject{Name: "speak_config", Pm: map[string]string{"name": "akane"}}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("speak_config with no style= returned nil error, want an error")
	}
}

// --- lineText ---

func TestLineTextConcatenatesSegmentsIgnoringRuby(t *testing.T) {
	segs := []Text{
		{Text: "単", Ruby: "たん"},
		{Text: "にできます"},
	}
	if got, want := lineText(segs), "単にできます"; got != want {
		t.Errorf("lineText(...) = %q, want %q", got, want)
	}
}

// --- B. speakLine ---

func TestSpeakLineSkipsWhenSynthNilOrAutoPlayOffOrEmptyText(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()

	SpeechSynth = nil
	speechAutoPlay = true
	r.speakLine("akane", "こんにちは")
	if speechSeq != 0 {
		t.Errorf("speechSeq = %d with SpeechSynth == nil, want 0 (no job dispatched)", speechSeq)
	}

	called := false
	SpeechSynth = func(text string, styleID uint32) (io.ReadCloser, error) {
		called = true
		return newFakeWav(nil), nil
	}
	speechAutoPlay = false
	r.speakLine("akane", "こんにちは")
	if speechSeq != 0 || called {
		t.Errorf("speakLine fired with speechAutoPlay == false: speechSeq=%d called=%v", speechSeq, called)
	}

	speechAutoPlay = true
	r.speakLine("akane", "")
	if speechSeq != 0 || called {
		t.Errorf("speakLine fired for empty text: speechSeq=%d called=%v", speechSeq, called)
	}
}

func TestSpeakLineUsesPerCharaStyleThenDefault(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()
	speechStyles["akane"] = 3
	defaultSpeechStyle = 8

	gotStyle := make(chan uint32, 2)
	SpeechSynth = func(text string, styleID uint32) (io.ReadCloser, error) {
		gotStyle <- styleID
		return newFakeWav(minimalMonoWAV16(24000, []int16{0, 0})), nil
	}
	speechAutoPlay = true

	r.speakLine("akane", "こんにちは")
	select {
	case s := <-gotStyle:
		if s != 3 {
			t.Errorf("style for akane = %d, want 3 (per-character override)", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SpeechSynth was not called for akane")
	}

	r.speakLine("kenta", "こんにちは")
	select {
	case s := <-gotStyle:
		if s != 8 {
			t.Errorf("style for kenta = %d, want 8 (default, no per-character entry)", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SpeechSynth was not called for kenta")
	}
}

// --- C. stepSpeechSynthesis ---

func TestStepSpeechSynthesisDropsStaleResults(t *testing.T) {
	resetSpeechState(t)
	lastAppliedSpeechSeq = 5
	speechResultsMu.Lock()
	speechResults = []speechResult{{seq: 3, wavBytes: minimalMonoWAV16(24000, []int16{1, 2, 3})}}
	speechResultsMu.Unlock()

	stepSpeechSynthesis()

	if _, ok := ses[speechSeBuf]; ok {
		t.Error("ses[speechSeBuf] set from a stale (seq <= lastAppliedSpeechSeq) result, want untouched")
	}
	if lastAppliedSpeechSeq != 5 {
		t.Errorf("lastAppliedSpeechSeq = %d after a dropped stale result, want it left at 5", lastAppliedSpeechSeq)
	}
}

func TestStepSpeechSynthesisAppliesNewestResult(t *testing.T) {
	resetSpeechState(t)
	speechResultsMu.Lock()
	speechResults = []speechResult{{seq: 1, wavBytes: minimalMonoWAV16(24000, make([]int16, 100))}}
	speechResultsMu.Unlock()

	stepSpeechSynthesis()

	se, ok := ses[speechSeBuf]
	if !ok || se.Player == nil {
		t.Fatal("ses[speechSeBuf] not populated after a valid result")
	}
	if lastAppliedSpeechSeq != 1 {
		t.Errorf("lastAppliedSpeechSeq = %d, want 1", lastAppliedSpeechSeq)
	}
}

func TestStepSpeechSynthesisHandlesSynthesisError(t *testing.T) {
	resetSpeechState(t)
	speechResultsMu.Lock()
	speechResults = []speechResult{{seq: 1, err: errors.New("synth failed")}}
	speechResultsMu.Unlock()

	stepSpeechSynthesis() // must not panic

	if _, ok := ses[speechSeBuf]; ok {
		t.Error("ses[speechSeBuf] set from a failed synthesis result, want untouched")
	}
	if lastAppliedSpeechSeq != 1 {
		t.Errorf("lastAppliedSpeechSeq = %d after a failed-but-handled result, want 1 (don't retry the same seq)", lastAppliedSpeechSeq)
	}
}

// --- D. speakLine -> stepSpeechSynthesis integration (the real goroutine) ---

func TestSpeakLineDispatchesAndStepSpeechSynthesisPlaysIt(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()
	SpeechSynth = func(text string, styleID uint32) (io.ReadCloser, error) {
		return newFakeWav(minimalMonoWAV16(24000, make([]int16, 100))), nil
	}
	speechAutoPlay = true

	r.speakLine("akane", "こんにちは")
	if speechSeq != 1 {
		t.Fatalf("speechSeq = %d after speakLine, want 1", speechSeq)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stepSpeechSynthesis()
		if _, ok := ses[speechSeBuf]; ok {
			return // success
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("ses[speechSeBuf] never became populated — speakLine's goroutine result never arrived")
}

// --- E. goToTitle ---

func TestGoToTitleResetsSpeechState(t *testing.T) {
	resetSpeechState(t)
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	// A nil FSes map would panic rather than error inside loadScript — see
	// TestGoToTitleHidesLeftoverGameplayChrome (renderer_test.go) for the
	// same setup and its own note on why.
	r.manager.FSes = map[string]fs.FS{"senarios": fstest.MapFS{}}
	defer func() { tick, oldTick = 0, 0 }()

	speechAutoPlay = true
	speechStyles["akane"] = 3
	defaultSpeechStyle = 9
	speechSeq = 5
	lastAppliedSpeechSeq = 2

	r.goToTitle()

	if speechAutoPlay {
		t.Error("speechAutoPlay = true after goToTitle, want false")
	}
	if len(speechStyles) != 0 {
		t.Errorf("speechStyles = %+v after goToTitle, want empty", speechStyles)
	}
	if defaultSpeechStyle != 0 {
		t.Errorf("defaultSpeechStyle = %d after goToTitle, want 0", defaultSpeechStyle)
	}
	if lastAppliedSpeechSeq != speechSeq {
		t.Errorf("lastAppliedSpeechSeq = %d after goToTitle, want it caught up to speechSeq (%d) so in-flight jobs are discarded", lastAppliedSpeechSeq, speechSeq)
	}
}
