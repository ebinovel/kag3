// [movie]: fullscreen video playback, using github.com/liqmix/govid (pure
// Go, no cgo, no ffmpeg — MP4/H.264, WebM/VP8, and MPEG-1). No equivalent
// audio decoding exists in govid, so a video's soundtrack is provided
// separately via se= (an ordinary audio file played through the same
// [playse]/ses[] machinery as any other sound effect, see tags_audio.go) —
// see docs/VIDEO.md for the reasoning and how to prepare assets.
//
// AV1 is deliberately never used: govid's own README says its AV1 decoder
// is "not usable — parses structure, output is wrong". An MP4/WebM file
// encoded with AV1 is rejected with a clear error instead of silently
// producing garbage frames.
package ebitengine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	govid "github.com/liqmix/govid"
	govidebiten "github.com/liqmix/govid/ebitengine"
	"github.com/liqmix/govid/h264"
	"github.com/liqmix/govid/mp4"
	"github.com/liqmix/govid/mpeg1"
	"github.com/liqmix/govid/vp8"
	"github.com/liqmix/govid/webm"
)

func init() {
	register("movie", handleMovie)
}

// movieDecodeAhead is how many frames govid's background decode goroutine
// keeps queued — the same value govid's own ebitengine example uses.
// H.264 at 720p costs ~15ms/frame (govid's own benchmark), close to the
// whole 60 TPS tick budget, so decoding must happen off the game thread
// (NewAsyncPlayer) with enough of a queue to absorb a slow frame without
// stalling playback.
const movieDecodeAhead = 4

// movieSeBuf is the ses[] (tags_audio.go) key a [movie se=] companion
// track plays under — a buffer of its own, like speechSeBuf
// (tags_speech.go) and defaultVoiceSeBuf (tags_voice.go), so a project
// using [movie] alongside BGM/voice/SE keeps every category independently
// controllable via [stopse buf=]/[changevol buf=]/etc.
const movieSeBuf = "movie"

// errVideosFSUnset is openMovie's sentinel for "this project's fs.FS map
// (see Manager.Init) never registered a \"videos\" key at all" — as
// opposed to a real load/decode failure. renderer/ebitengine is shared
// with other projects (see SpeechSynth's own doc comment, tags_speech.go)
// that may have no use for [movie]; handleMovie treats this one specific
// case as a silent no-op, the same "optional feature just isn't wired up"
// contract every other config-gated feature in this package follows,
// rather than the harder [chara_new]-style failure a genuinely broken
// video file produces.
var errVideosFSUnset = errors.New("kag3: \"videos\" のfs.FSが設定されていません")

var (
	movieVideo    *govidebiten.VideoImage
	movieClosers  []io.Closer
	movieSkip     bool
	movieFinished bool
)

// closeQuietly closes every non-nil closer, ignoring errors — used on
// every error-return path in openMovie (a half-constructed player has
// nothing meaningful to report on Close) and by stopMovie/finishMovie for
// an already-working one.
func closeQuietly(closers ...io.Closer) {
	for _, c := range closers {
		if c != nil {
			c.Close()
		}
	}
}

// openSeekable opens name from fsys and returns it as an io.ReadSeeker,
// which every govid demuxer constructor requires. fs.File values returned
// by the FS implementations this engine actually uses (embed.FS,
// testing/fstest.MapFS, os.DirFS) already implement io.Seeker, so the
// common path is a type assertion with no copy. A theoretical fs.FS that
// doesn't falls back to reading the whole file into memory — videos are
// large, so this path is a last resort, not the expected one.
func openSeekable(fsys fs.FS, name string) (io.ReadSeeker, io.Closer, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, nil, err
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		return rs, f, nil
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, nil, err
	}
	return bytes.NewReader(data), nil, nil
}

// openMovie resolves storage against r.fses["videos"], picks a demuxer/
// codec pair by file extension, and returns a ready-to-draw VideoImage
// plus every io.Closer that must be released once playback ends —
// **in the order they must be closed**: the player first (so its decode
// goroutine stops touching the demuxer before the demuxer or file are
// touched again — see govid's own NewAsyncPlayer doc comment), then the
// demuxer, then the underlying file.
func openMovie(r *Renderer, storage string) (*govidebiten.VideoImage, []io.Closer, error) {
	fsys, ok := r.fses["videos"]
	if !ok || fsys == nil {
		return nil, nil, errVideosFSUnset
	}
	name := path.Clean(storage)
	reader, fileCloser, err := openSeekable(fsys, name)
	if err != nil {
		return nil, nil, fmt.Errorf("動画ファイル %s の読み込みに失敗しました: %w", storage, err)
	}

	var demuxCloser io.Closer
	var player *govid.Player
	switch ext := strings.ToLower(path.Ext(name)); ext {
	case ".mp4":
		demuxer, derr := mp4.NewDemuxer(reader)
		if derr != nil {
			closeQuietly(fileCloser)
			return nil, nil, fmt.Errorf("MP4 %s の解析に失敗しました: %w", storage, derr)
		}
		demuxCloser = demuxer
		if demuxer.CodecType() == "av1" {
			closeQuietly(demuxCloser, fileCloser)
			return nil, nil, fmt.Errorf("動画 %s: AV1コーデックは対応していません(govidのAV1デコーダは未完成のため。docs/VIDEO.md参照)", storage)
		}
		player, err = govid.NewAsyncPlayer(demuxer, h264.NewCodec(), movieDecodeAhead, govid.WithRGBA())
	case ".webm":
		demuxer, derr := webm.NewDemuxer(reader)
		if derr != nil {
			closeQuietly(fileCloser)
			return nil, nil, fmt.Errorf("WebM %s の解析に失敗しました: %w", storage, derr)
		}
		demuxCloser = demuxer
		if demuxer.CodecID() == "V_AV1" {
			closeQuietly(demuxCloser, fileCloser)
			return nil, nil, fmt.Errorf("動画 %s: AV1コーデックは対応していません(govidのAV1デコーダは未完成のため。docs/VIDEO.md参照)", storage)
		}
		player, err = govid.NewAsyncPlayer(demuxer, vp8.NewCodec(), movieDecodeAhead, govid.WithRGBA())
	case ".mpg", ".mpeg":
		// mpeg1.Source is both Demuxer and Codec (README's own usage
		// shape) — SetAudioEnabled(false) internally, matching every
		// other format here: none of them decode audio.
		source, serr := mpeg1.NewSource(reader)
		if serr != nil {
			closeQuietly(fileCloser)
			return nil, nil, fmt.Errorf("MPEG-1 %s の解析に失敗しました: %w", storage, serr)
		}
		demuxCloser = source
		player, err = govid.NewAsyncPlayer(source, source, movieDecodeAhead, govid.WithRGBA())
	default:
		closeQuietly(fileCloser)
		return nil, nil, fmt.Errorf("動画 %s: 拡張子 %q には対応していません(.mp4 / .webm / .mpg / .mpeg のみ)", storage, ext)
	}
	if err != nil {
		closeQuietly(demuxCloser, fileCloser)
		return nil, nil, fmt.Errorf("動画 %s の再生準備に失敗しました: %w", storage, err)
	}

	player.Play()
	video := govidebiten.New(player)
	closers := []io.Closer{player, demuxCloser}
	if fileCloser != nil {
		closers = append(closers, fileCloser)
	}
	return video, closers, nil
}

// stopMovie tears down whatever [movie] is currently playing without
// marking it "finished" from the story's point of view — used by
// goToTitle, where the coroutine is being abandoned entirely rather than
// resumed past a blocking [movie wait=true].
func stopMovie() {
	closeQuietly(movieClosers...)
	movieVideo = nil
	movieClosers = nil
	movieFinished = false
}

// finishMovie ends the current [movie] normally: [movie wait=true]'s
// y.Until reads movieFinished, so this is what actually unblocks it (or,
// for wait=false, simply what a later frame's stepMovie call reports has
// already happened).
func finishMovie() {
	closeQuietly(movieClosers...)
	movieVideo = nil
	movieClosers = nil
	movieFinished = true
}

// movieShouldFinish is stepMovie's skip/stop decision, split out into a
// pure function the same way skipShouldAdvance (renderer.go) is — this
// package has no precedent for faking real mouse/touch input directly in
// a test (see skipShouldAdvance's own doc comment), so the decision logic
// needs to be testable independent of doNext()'s real hardware read.
//
// resetOldTick distinguishes the two ways a movie can end:
//   - skip&&clicked: a real click ended it. That same click already
//     satisfies Update()'s own doNext()-driven oldTick=tick branch, so
//     whatever [p] immediately follows [movie] gets the same few-tick
//     grace window any other click-driven tag transition does (see
//     CLAUDE.md's Execution model note on oldTick) — no reset needed.
//   - stopped (natural end, no click this frame): oldTick could still be
//     "recent enough" from some earlier, unrelated click during playback,
//     letting the very next [p] spuriously pass isTextEnded without the
//     player actually clicking past it. Same fix goToTitle/applySaveData
//     use for the analogous jump-destination case
//     (tags_save.go/title_flow.go) — resetOldTick tells the caller to
//     apply it here too.
func movieShouldFinish(skip, clicked, stopped bool) (finish, resetOldTick bool) {
	switch {
	case skip && clicked:
		return true, false
	case stopped:
		return true, true
	default:
		return false, false
	}
}

// stepMovie is called from Update(), alongside stepAudioFades/
// stepSpeechSynthesis/stepAnimations, once per frame — before the
// coroutine is resumed, so a [movie wait=true] blocked on movieFinished
// sees this frame's result immediately rather than one frame late.
func stepMovie() {
	if movieVideo == nil {
		return
	}
	movieVideo.Update()
	stopped := movieVideo.Player().State() == govid.StateStopped
	finish, resetOldTick := movieShouldFinish(movieSkip, doNext(), stopped)
	if !finish {
		return
	}
	finishMovie()
	if resetOldTick {
		oldTick = tick - 4
	}
}

// drawMovie draws the current frame of whatever [movie] is playing,
// letterboxed to fit the screen while preserving aspect ratio — the same
// "Fit" scaling govid's own ebitengine example uses. Drawn in drawScene
// between drawEditBox and captureSnapshot: after every other scene
// element, but before modal overlays, so a video plays fullscreen under
// (for example) a save/load prompt if one is somehow triggered mid-movie.
func drawMovie(buf *ebiten.Image) {
	if movieVideo == nil {
		return
	}
	img := movieVideo.Image()
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return
	}
	sw, sh := buf.Bounds().Dx(), buf.Bounds().Dy()
	s := float64(sw) / float64(iw)
	if s2 := float64(sh) / float64(ih); s2 < s {
		s = s2
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(s, s)
	op.GeoM.Translate((float64(sw)-float64(iw)*s)/2, (float64(sh)-float64(ih)*s)/2)
	buf.DrawImage(img, op)
}

// handleMovie implements [movie storage= se= skip= loop= wait=].
//
//   - storage= (required): the video file, resolved against the "videos"
//     fs.FS key (see Manager.Init) — set up your own entrypoint's fs.FS
//     map the same way "images"/"bgms"/"ses" already are, or [movie]
//     silently no-ops (see errVideosFSUnset above).
//   - se=: an ordinary audio file played alongside the video, since govid
//     cannot decode a video's own audio track at all. Goes through the
//     same ses[]/[stopse]/[changevol] machinery [playse] uses, under its
//     own "movie" buffer name.
//   - skip= (default true): whether a click ends playback early.
//   - loop= (default false): repeats the video indefinitely — pair with
//     wait=false, since a looping [movie wait=true] would never return.
//   - wait= (default true): block the coroutine until playback ends
//     (naturally, or via a skip click).
func handleMovie(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	storage, ok := getString(pm, "storage")
	if !ok || storage == "" {
		return fmt.Errorf("[movie] には storage= が必要です")
	}

	// A previous [movie] should always have been torn down by finishMovie/
	// stopMovie already, but guard against a second [movie] arriving while
	// one is still open (e.g. wait=false followed immediately by another
	// [movie]) rather than leaking its decode goroutine.
	stopMovie()

	video, closers, err := openMovie(r, storage)
	if err != nil {
		if errors.Is(err, errVideosFSUnset) {
			fmt.Fprintln(os.Stderr, "[movie]: \"videos\" のfs.FSが設定されていないため、何もせずスキップします")
			return nil
		}
		return err
	}
	movieVideo = video
	movieClosers = closers
	movieFinished = false

	movieSkip = true
	if v, ok, gerr := getBool(pm, "skip"); gerr != nil {
		return gerr
	} else if ok {
		movieSkip = v
	}

	loop := false
	if v, ok, gerr := getBool(pm, "loop"); gerr != nil {
		return gerr
	} else if ok {
		loop = v
	}
	movieVideo.Player().SetLoop(loop)

	wait := true
	if v, ok, gerr := getBool(pm, "wait"); gerr != nil {
		return gerr
	} else if ok {
		wait = v
	}

	if seStorage, ok := getString(pm, "se"); ok && seStorage != "" {
		se := &kag3.BGM{Volume: defaultSeVolume}
		if err := swapSEPlayer(r.fses["ses"], movieSeBuf, seStorage, se); err != nil {
			fmt.Fprintln(os.Stderr, "[movie]: se=", seStorage, "の読み込みに失敗したため、映像のみ再生します:", err)
		} else {
			delete(activeFades, "se:"+movieSeBuf)
			se.Player.SetVolume(volFraction(defaultSeVolume))
			se.Player.Play()
		}
	}

	if !wait {
		return nil
	}
	ctx.y.Until(true, func() bool { return movieFinished })
	return nil
}
