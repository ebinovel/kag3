// [speak_on]/[speak_off]/[speak_config]: text-to-speech narration via
// VOICEVOX CORE. This has no equivalent in real TyranoScript's actual
// engine behavior — tyrano.jp/tag documents speak_on/speak_off, but kag3's
// implementation (offline neural synthesis, not a browser TTS API) is a
// genuinely kag3-exclusive capability. See docs/VOICEVOX.md.
//
// renderer/ebitengine stays voicevox_core/nanoda-oblivious, the same
// "shared package stays mechanism-agnostic, an entrypoint-side file flips
// an exported hook" pattern as ConfigStorage/SaveDirFunc — see SpeechSynth
// below and example/game/speech_desktop.go (which actually loads nanoda).
package ebitengine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

func init() {
	register("speak_on", handleSpeakOn)
	register("speak_off", handleSpeakOff)
	register("speak_config", handleSpeakConfig)
}

// SpeechSynth synthesizes text as the given VOICEVOX style ID, returning a
// WAV stream. nil (the default) means no TTS backend is configured for
// this project — speakLine below turns that into a silent no-op rather
// than an error, matching every other "optional asset is simply missing"
// path in this package (handlePopopo, playCharaVoice).
var SpeechSynth func(text string, styleID uint32) (io.ReadCloser, error)

// SpeechWarmup, if set, runs once on speechWorker's goroutine before it
// processes any real job (see ensureSpeechWorker's doc comment for why).
// Deliberately a separate hook from SpeechSynth, not just an internal
// SpeechSynth("", 0) call: routing warm-up through SpeechSynth itself
// would make it indistinguishable from a real job to anything observing
// SpeechSynth calls (tests included — a mock counting/asserting on call
// order has no way to tell "the worker's own warm-up ping" from "a real
// line"). nil (the default, and every test's state) skips warm-up
// entirely — only speech_ios.go sets this, since the goroutine-affinity
// bug it works around is specific to that platform's purego backend.
var SpeechWarmup func()

var (
	// speechAutoPlay is [speak_on]/[speak_off]. Off by default — arming it
	// is opt-in, same as [vostart] for voice files (tags_voice.go).
	speechAutoPlay bool
	// speechStyles is [speak_config name= style=]'s registry: speaker name
	// (charaName, tags_character.go) -> VOICEVOX style ID. Process-lifetime,
	// not saved — same treatment as voiceConfigs (tags_voice.go) and reset
	// in goToTitle (title_flow.go) for the same reason.
	speechStyles = map[string]uint32{}
	// defaultSpeechStyle is [speak_config style=] with no name= — used for
	// monologue (charaName == "") and any speaker with no entry of their
	// own in speechStyles.
	defaultSpeechStyle uint32
)

func handleSpeakOn(ctx *tagCtx) error {
	speechAutoPlay = true
	return nil
}

func handleSpeakOff(ctx *tagCtx) error {
	speechAutoPlay = false
	return nil
}

// handleSpeakConfig implements [speak_config name= style=]. Merges into
// the existing registry rather than replacing it, same rationale as
// [voconfig] (tags_voice.go): re-running it for one character must not
// disturb another's.
func handleSpeakConfig(ctx *tagCtx) error {
	style, ok, err := getInt(ctx.tag.Pm, "style")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("[speak_config] requires style=")
	}
	if name, hasName := getString(ctx.tag.Pm, "name"); hasName && name != "" {
		speechStyles[name] = uint32(style)
	} else {
		defaultSpeechStyle = uint32(style)
	}
	return nil
}

// speechSeBuf is the ses[] (tags_audio.go) key speech playback lives
// under. Deliberately separate from defaultVoiceSeBuf ("voice",
// tags_voice.go): a project could conceivably use [voconfig] file
// playback and [speak_on] synthesis at once, and each should be
// independently controllable via [stopse buf=]/[changevol buf=]/etc.
const speechSeBuf = "speech"

// speechSeq/lastAppliedSpeechSeq resolve out-of-order completion: synthesis
// runs on a background goroutine per line (see speakLine), and a short
// line can finish synthesizing after a long line that started later —
// TTS latency doesn't correlate with queue order the way file loads do.
// Each job captures speechSeq at dispatch time; stepSpeechSynthesis only
// ever applies a result newer than the last one actually played, silently
// dropping (never playing) anything older. Reset in goToTitle by jumping
// lastAppliedSpeechSeq to the current speechSeq, so any job already in
// flight when the player returns to title is discarded on arrival instead
// of unexpectedly speaking over the title screen.
var (
	speechSeq            int
	lastAppliedSpeechSeq int
)

// speechResult is one finished synthesis job, produced by the speech
// worker goroutine (see speechWorker) and drained by stepSpeechSynthesis
// on the main Update() goroutine — audio.Player/ses must only ever be
// touched there, never from the background goroutine itself.
type speechResult struct {
	seq      int
	wavBytes []byte
	err      error
}

var (
	speechResultsMu sync.Mutex
	speechResults   []speechResult
)

// speechJob is one queued speakLine request, consumed by speechWorker.
type speechJob struct {
	seq   int
	text  string
	style uint32
}

var (
	// speechJobs feeds the single persistent speechWorker goroutine — see
	// ensureSpeechWorker's doc comment for why every job must go through
	// one long-lived goroutine rather than a fresh one per line. Buffered
	// so a burst of lines (e.g. auto-mode or skip racing ahead of TTS
	// latency) doesn't stall the caller; speakLine drops the job with a
	// log line rather than block if it's ever actually full.
	speechJobs     chan speechJob
	speechWorkerMu sync.Mutex
)

// ensureSpeechWorker lazily starts the one persistent goroutine that ever
// calls SpeechSynth, the first time it's actually needed (a project with
// TTS configured but never using [speak_on] never pays for it).
//
// Every call used to run on its own freshly spawned goroutine. On this
// package's iOS backend (speech_ios.go, purego-based — see docs/VOICEVOX.md)
// that turned out to reliably corrupt the *first* native call ever made on
// a brand-new goroutine specifically: voicevox_synthesizer_tts would return
// success (code 0) with a real, non-null output buffer pointer, but its
// *other* out-parameter (output_wav_length) silently stayed at its
// zero-initialized value — confirmed via a from-scratch isolation
// harness (bypassing nanoda/kag3 entirely, calling voicevox_core directly
// through purego) that a second call *on that same now-"warmed"
// goroutine* always succeeds correctly, every time. The leading theory is
// a purego/Go-runtime interaction around a freshly spawned goroutine's
// still-growing stack racing the low-level FFI trampoline
// (runtime_cgocall + a hand-written assembly syscall) — plausible but not
// definitively root-caused. Rather than chase that further, routing every
// call through one persistent, pre-warmed goroutine sidesteps the bug
// entirely: harmless (and arguably better practice — it also serializes
// calls into a native synthesizer object of unknown thread-safety) on
// every other platform's SpeechSynth backend too.
func ensureSpeechWorker() {
	speechWorkerMu.Lock()
	defer speechWorkerMu.Unlock()
	if speechJobs != nil {
		return
	}
	speechJobs = make(chan speechJob, 8)
	go speechWorker()
}

// speechWorker is the one persistent goroutine that ever calls
// SpeechSynth — see ensureSpeechWorker's doc comment for why. Runs
// SpeechWarmup once first, if set (see its own doc comment for why that's
// a separate hook from SpeechSynth), before processing any real job.
func speechWorker() {
	// Locked to its OS thread for the worker's entire lifetime. The
	// leading theory behind the goroutine-first-call corruption bug
	// SpeechWarmup works around (see its doc comment) is a race between
	// a still-growing goroutine stack and the low-level FFI trampoline —
	// if that's right, letting the Go scheduler migrate this goroutine to
	// a *different* OS thread mid-lifetime could reopen the exact same
	// bug on the new thread, undoing whatever SpeechWarmup paid down.
	// Locking removes that possibility outright; this goroutine never
	// does anything else that needs the scheduler's help.
	runtime.LockOSThread()
	if SpeechWarmup != nil {
		func() {
			defer func() { recover() }()
			SpeechWarmup()
		}()
	}
	for job := range speechJobs {
		runSpeechJob(job)
	}
}

// runSpeechJob does the actual synthesis + WAV read for one job,
// appending its speechResult for stepSpeechSynthesis to drain. Split out
// of speechWorker's loop body so a panic during one job (recovered here)
// can't take the whole worker goroutine down with it — the same
// per-job-recover shape the old per-line goroutine used.
func runSpeechJob(job speechJob) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintln(os.Stderr, "[speech] synthesis panicked, skipping:", p)
		}
	}()
	rc, err := SpeechSynth(job.text, job.style)
	if err != nil {
		speechResultsMu.Lock()
		speechResults = append(speechResults, speechResult{seq: job.seq, err: err})
		speechResultsMu.Unlock()
		return
	}
	// Read everything into a Go-owned copy, then close, in that order —
	// not the reverse. nanoda's WAV reader is a bytes.Reader over memory
	// obtained via unsafe.Slice directly against the native VOICEVOX
	// buffer; Close (voicevox_wav_free) invalidates that memory
	// immediately, and wav.DecodeWithSampleRate's own read timing (called
	// later, from stepSpeechSynthesis) isn't something this package
	// controls. Copying first is the only way to guarantee no
	// use-after-free regardless of how DecodeWithSampleRate happens to
	// read its source.
	b, readErr := io.ReadAll(rc)
	rc.Close()
	if readErr != nil {
		err = readErr
	}
	speechResultsMu.Lock()
	speechResults = append(speechResults, speechResult{seq: job.seq, wavBytes: b, err: err})
	speechResultsMu.Unlock()
}

// lineText concatenates a line's own Text segments (skipping Ruby/
// TextStyle), the same idea as currentMessageText (tags_message.go) but
// scoped to a single line instead of the whole visible page — what
// speakLine needs to know "what does this line actually say", plain text,
// no markup.
func lineText(segs []Text) string {
	var sb strings.Builder
	for _, seg := range segs {
		sb.WriteString(seg.Text)
	}
	return sb.String()
}

// speakLine is execItem's (macro.go) hook, called once per line right
// after r.texts[object.Line] is fully built (unlike playCharaVoice, which
// fires at the "#name" declaration itself — speech needs the line's
// actual text, not just who's speaking). Never blocks: synthesis is
// queued to the persistent speech worker (see ensureSpeechWorker) and
// applied later by stepSpeechSynthesis, so a slow (or even hung) TTS
// backend never stalls text reveal.
func (r *Renderer) speakLine(name, text string) {
	if SpeechSynth == nil || !speechAutoPlay || text == "" {
		return
	}
	style, ok := speechStyles[name]
	if !ok {
		style = defaultSpeechStyle
	}
	speechSeq++
	ensureSpeechWorker()
	select {
	case speechJobs <- speechJob{seq: speechSeq, text: text, style: style}:
	default:
		// The worker is badly backlogged (shouldn't happen in practice —
		// lines are paced by reading speed, not queued faster than one
		// synthesis call finishes). Drop rather than block the coroutine.
		fmt.Fprintln(os.Stderr, "[speech] synthesis queue full, dropping line")
	}
}

// stepSpeechSynthesis drains completed synthesis jobs once per frame,
// called from Update() alongside stepAudioFades — this is the only place
// that actually touches audioContext/ses for speech, keeping every
// ebitengine audio call on the main goroutine.
func stepSpeechSynthesis() {
	speechResultsMu.Lock()
	results := speechResults
	speechResults = nil
	speechResultsMu.Unlock()

	for _, res := range results {
		if res.seq <= lastAppliedSpeechSeq {
			// A newer line already started (and possibly already
			// finished) since this job was dispatched — playing it now
			// would speak over, or after, whatever's actually current.
			continue
		}
		lastAppliedSpeechSeq = res.seq
		if res.err != nil {
			// fmt.Fprintln(os.Stderr, ...), not fmt.Println: on Android
			// specifically, gomobile's os.Stdout-backed logcat pipe
			// (internal/mobileinit) was observed not delivering lines at
			// all during this feature's own bring-up, while the
			// os.Stderr-backed one worked reliably — see docs/VOICEVOX.md's
			// Android section. Using stderr for every log line in this
			// file sidesteps that regardless of platform.
			fmt.Fprintln(os.Stderr, "[speech] synthesis failed, skipping:", res.err)
			continue
		}
		stream, err := wav.DecodeWithSampleRate(audioContext.SampleRate(), bytes.NewReader(res.wavBytes))
		if err != nil {
			fmt.Fprintln(os.Stderr, "[speech] wav decode failed, skipping:", err)
			continue
		}
		player, err := audioContext.NewPlayer(stream)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[speech] player creation failed, skipping:", err)
			continue
		}
		player.SetVolume(volFraction(defaultSeVolume))
		player.Play()
		setSEPlayer(speechSeBuf, &kag3.BGM{Volume: defaultSeVolume, Player: player})
	}
}
