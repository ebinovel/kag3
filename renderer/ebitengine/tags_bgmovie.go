package ebitengine

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	govid "github.com/liqmix/govid"
	govidebiten "github.com/liqmix/govid/ebitengine"
)

func init() {
	register("bgmovie", handleBgMovie)
	register("wait_bgmovie", handleWaitBgMovie)
	register("stop_bgmovie", handleStopBgMovie)
}

// bgmovieSeBuf is the ses[] buffer [bgmovie se=] plays its companion audio
// under — a buffer of its own, distinct from movieSeBuf ([movie],
// tags_movie.go), so a project using both simultaneously (a background
// loop plus a later fullscreen cutscene) keeps them independently
// controllable via [stopse buf=]/[changevol buf=].
const bgmovieSeBuf = "bgmovie"

var (
	bgMovieVideo    *govidebiten.VideoImage
	bgMovieClosers  []io.Closer
	bgMovieFinished bool
	// bgMovieLoop mirrors whatever loop= handleBgMovie last resolved.
	// Tracked here rather than read back from govid.Player (which exposes
	// SetLoop but no matching getter) — handleWaitBgMovie needs to know
	// this without blocking forever on a loop that, by construction, never
	// reaches govid.StateStopped on its own.
	bgMovieLoop bool

	// bgMovieFadeStartT/bgMovieFadeDurTicks drive the fade-in via
	// bgMovieAlpha: a plain linear ramp over ticks, not
	// effects.Transitions — this is a video layer sitting behind the rest
	// of the scene, not a crossfade between two static [bg] images, so
	// none of that package's per-transition-style machinery applies.
	bgMovieFadeStartT   int
	bgMovieFadeDurTicks int
)

// stopBgMovie tears down whatever [bgmovie] is currently playing without
// marking it finished — used by title_flow.go's goToTitle (the coroutine is
// being abandoned entirely, not resumed past a blocking [wait_bgmovie]) and
// by handleBgMovie itself as a guard against leaking a previous bgmovie's
// decode goroutine if a second [bgmovie] arrives before the first was ever
// stopped.
func stopBgMovie() {
	closeQuietly(bgMovieClosers...)
	bgMovieVideo = nil
	bgMovieClosers = nil
	bgMovieFinished = false
}

// finishBgMovie ends the current [bgmovie] normally — a non-looping clip
// reaching its natural end (stepBgMovie). This is what actually unblocks
// [wait_bgmovie]'s y.Until.
func finishBgMovie() {
	closeQuietly(bgMovieClosers...)
	bgMovieVideo = nil
	bgMovieClosers = nil
	bgMovieFinished = true
}

// bgMovieAlpha is the current fade-in opacity (0..1): a linear ramp from
// bgMovieFadeStartT over bgMovieFadeDurTicks ticks, matching startFade's
// own "duration<=0 applies the target immediately" convention
// (tags_audio.go) for a zero/negative duration.
func bgMovieAlpha() float64 {
	if bgMovieFadeDurTicks <= 0 {
		return 1
	}
	elapsed := t - bgMovieFadeStartT
	switch {
	case elapsed >= bgMovieFadeDurTicks:
		return 1
	case elapsed <= 0:
		return 0
	default:
		return float64(elapsed) / float64(bgMovieFadeDurTicks)
	}
}

// stepBgMovie advances bgMovie playback — called from Update() alongside
// stepMovie. A looping [bgmovie] (bgMovieLoop) never reaches
// govid.StateStopped on its own (Player.SetLoop keeps it going), so only a
// non-looping video's natural end ever finishes it here; [stop_bgmovie] is
// the only way to end a looping one early.
func stepBgMovie() {
	if bgMovieVideo == nil {
		return
	}
	bgMovieVideo.Update()
	if bgMovieVideo.Player().State() == govid.StateStopped {
		finishBgMovie()
	}
}

// drawBgMovie draws the current frame of whatever [bgmovie] is playing,
// covering the full screen (Cover scaling — cropping overflow rather than
// letterboxing like [movie]'s drawMovie, since a black bar would look
// broken on a background layer) faded in via bgMovieAlpha. Called from
// drawScene *before* drawBackground (renderer.go), so [bg]/[bg2] and
// everything above them still draws normally on top of it.
func drawBgMovie(buf *ebiten.Image) {
	if bgMovieVideo == nil {
		return
	}
	img := bgMovieVideo.Image()
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return
	}
	sw, sh := buf.Bounds().Dx(), buf.Bounds().Dy()
	s := float64(sw) / float64(iw)
	if s2 := float64(sh) / float64(ih); s2 > s {
		s = s2
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.ColorScale.ScaleAlpha(float32(bgMovieAlpha()))
	op.GeoM.Scale(s, s)
	op.GeoM.Translate((float64(sw)-float64(iw)*s)/2, (float64(sh)-float64(ih)*s)/2)
	buf.DrawImage(img, op)
}

// handleBgMovie implements [bgmovie storage= se= volume= loop= time= wait=].
// Reuses openMovie (tags_movie.go) — the same govid demuxer/codec
// resolution, AV1/Theora rejection, and errVideosFSUnset "videos" fs.FS
// contract [movie] already has. This tag only differs in where the result
// is drawn (drawBgMovie, behind the rest of the scene, not fullscreen) and
// how playback loops by default (true here, false for [movie]).
//
//   - storage= (required): resolved against the "videos" fs.FS key, same as
//     [movie].
//   - se=: companion audio, since govid never decodes a video's own audio
//     track — plays through ses[] under its own "bgmovie" buffer.
//   - volume= is accepted for compatibility but has no effect: govid
//     doesn't decode any audio from the video itself, only se= does
//     anything audible (the same limitation [movie] has).
//   - loop= (default true, opposite of [movie]'s false): a background
//     video normally plays indefinitely until [stop_bgmovie].
//   - time= (default 500ms): fade-in duration.
//   - wait= (default true): blocks the coroutine until the fade-in
//     completes, not until playback itself ends — see [wait_bgmovie] for
//     that.
func handleBgMovie(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	storage, ok := getString(pm, "storage")
	if !ok || storage == "" {
		return fmt.Errorf("[bgmovie] には storage= が必要です")
	}

	// A previous [bgmovie] should always have been torn down by
	// finishBgMovie/stopBgMovie already, but guard against a second
	// [bgmovie] arriving while one is still open rather than leaking its
	// decode goroutine — same guard handleMovie has for [movie].
	stopBgMovie()

	video, closers, err := openMovie(r, storage)
	if err != nil {
		if errors.Is(err, errVideosFSUnset) {
			fmt.Fprintln(os.Stderr, "[bgmovie]: \"videos\" のfs.FSが設定されていないため、何もせずスキップします")
			return nil
		}
		return err
	}
	bgMovieVideo = video
	bgMovieClosers = closers
	bgMovieFinished = false

	loop := true
	if v, ok, gerr := getBool(pm, "loop"); gerr != nil {
		return gerr
	} else if ok {
		loop = v
	}
	bgMovieLoop = loop
	bgMovieVideo.Player().SetLoop(loop)

	fadeMS := 500
	if v, ok, gerr := getInt(pm, "time"); gerr != nil {
		return gerr
	} else if ok {
		fadeMS = v
	}
	bgMovieFadeStartT = t
	bgMovieFadeDurTicks = fadeMS * ebiten.TPS() / 1000

	if seStorage, ok := getString(pm, "se"); ok && seStorage != "" {
		se := &kag3.BGM{Volume: defaultSeVolume}
		if err := swapSEPlayer(r.fses["ses"], bgmovieSeBuf, seStorage, se); err != nil {
			fmt.Fprintln(os.Stderr, "[bgmovie]: se=", seStorage, "の読み込みに失敗したため、映像のみ再生します:", err)
		} else {
			delete(activeFades, "se:"+bgmovieSeBuf)
			se.Player.SetVolume(volFraction(defaultSeVolume))
			se.Player.Play()
		}
	}

	wait := true
	if v, ok, gerr := getBool(pm, "wait"); gerr != nil {
		return gerr
	} else if ok {
		wait = v
	}
	if !wait {
		return nil
	}
	startT, dur := bgMovieFadeStartT, bgMovieFadeDurTicks
	ctx.y.Until(true, func() bool { return t-startT >= dur })
	return nil
}

// handleWaitBgMovie implements [wait_bgmovie]: blocks until the current
// [bgmovie] finishes naturally. A looping bgmovie (bgMovieLoop, the
// default) never finishes on its own, so this logs and returns immediately
// rather than blocking forever — only a [bgmovie loop=false], or one
// already ended via [stop_bgmovie]/a natural end before this ran, is
// actually waited on.
func handleWaitBgMovie(ctx *tagCtx) error {
	if bgMovieVideo == nil {
		return nil
	}
	if bgMovieLoop {
		fmt.Fprintln(os.Stderr, "[wait_bgmovie]: ループ再生中のためすぐに戻ります([stop_bgmovie]で停止してください)")
		return nil
	}
	ctx.y.Until(true, func() bool { return bgMovieFinished })
	return nil
}

// handleStopBgMovie implements [stop_bgmovie].
func handleStopBgMovie(ctx *tagCtx) error {
	stopBgMovie()
	return nil
}
