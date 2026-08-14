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
	register("layermode_movie", handleLayerModeMovie)
}

// layerMovieSeBuf is the ses[] buffer [layermode_movie]'s companion audio
// plays under — a buffer of its own, distinct from movieSeBuf ([movie])
// and bgmovieSeBuf ([bgmovie]), so a project using more than one of these
// simultaneously keeps each independently controllable via
// [stopse buf=]/[changevol buf=].
const layerMovieSeBuf = "layermode_movie"

var (
	layerMovieVideo    *govidebiten.VideoImage
	layerMovieClosers  []io.Closer
	layerMovieFinished bool
	// layerMovieLoop mirrors whatever loop= handleLayerModeMovie last
	// resolved — same reason bgMovieLoop exists (tags_bgmovie.go): govid's
	// Player exposes SetLoop but no matching getter.
	layerMovieLoop bool
	// layerMovieBlend is whatever mode= resolved to (resolveBlendMode,
	// tags_effects.go), applied in drawLayerMovie.
	layerMovieBlend ebiten.Blend
	// layerMovieOpacity is opacity= (0-255) as a 0..1 fraction, a static
	// multiplier applied on top of the fade-in (layerMovieAlpha) in
	// drawLayerMovie.
	layerMovieOpacity float64 = 1

	// layerMovieFadeStartT/layerMovieFadeDurTicks drive the fade-in via
	// layerMovieAlpha — the same linear-ramp-over-ticks approach
	// bgMovieAlpha uses (tags_bgmovie.go), not effects.Transitions: this
	// composites over the existing scene with an arbitrary blend mode, not
	// a crossfade between two [bg]-style static images.
	layerMovieFadeStartT   int
	layerMovieFadeDurTicks int
)

// stopLayerMovie tears down whatever [layermode_movie] is currently playing
// without marking it finished — used by title_flow.go's goToTitle (the
// coroutine is being abandoned entirely) and by handleLayerModeMovie itself
// as a guard against leaking a previous instance's decode goroutine if a
// second [layermode_movie] arrives before the first was ever stopped.
//
// Also reachable from [free_layermode] with no layer= (handleFreeLayerMode,
// tags_effects.go) — upstream Tyrano's own [layermode_movie] reference
// never documents how to stop a (default-looping) instance mid-scene, and
// there is no separate [stop_layermode_movie]/[wait_layermode_movie] tag
// pair in the official V6 tag list. [free_layermode] with no layer= already
// means "reset every per-layer effect"; extending that to also stop an
// active [layermode_movie] is the least surprising reading available and
// gives scripts *some* way to end a looping one without a full scene/title
// transition. A targeted [free_layermode layer=...] deliberately does NOT
// reach here — only the "reset everything" form does.
func stopLayerMovie() {
	closeQuietly(layerMovieClosers...)
	layerMovieVideo = nil
	layerMovieClosers = nil
	layerMovieFinished = false
}

// finishLayerMovie ends the current [layermode_movie] normally — a
// non-looping clip reaching its natural end (stepLayerMovie).
func finishLayerMovie() {
	closeQuietly(layerMovieClosers...)
	layerMovieVideo = nil
	layerMovieClosers = nil
	layerMovieFinished = true
}

// layerMovieAlpha is the current fade-in opacity (0..1) — see bgMovieAlpha
// (tags_bgmovie.go) for the identical linear-ramp reasoning.
func layerMovieAlpha() float64 {
	if layerMovieFadeDurTicks <= 0 {
		return 1
	}
	elapsed := t - layerMovieFadeStartT
	switch {
	case elapsed >= layerMovieFadeDurTicks:
		return 1
	case elapsed <= 0:
		return 0
	default:
		return float64(elapsed) / float64(layerMovieFadeDurTicks)
	}
}

// stepLayerMovie advances layerMovie playback — called from Update()
// alongside stepMovie/stepBgMovie. A looping [layermode_movie]
// (layerMovieLoop) never reaches govid.StateStopped on its own.
func stepLayerMovie() {
	if layerMovieVideo == nil {
		return
	}
	layerMovieVideo.Update()
	if layerMovieVideo.Player().State() == govid.StateStopped {
		finishLayerMovie()
	}
}

// drawLayerMovie draws [layermode_movie]'s current frame, Cover-scaled like
// drawBgMovie (cropping overflow rather than letterboxing — a black bar
// would look broken composited over the rest of the scene), faded in via
// layerMovieAlpha × layerMovieOpacity and composited with layerMovieBlend.
// Called from drawScene at the same position as drawMovie ([movie],
// tags_movie.go): after the rest of ordinary scene content, before modal
// overlays — a "layer" video in upstream Tyrano composites over everything
// already on screen, the opposite placement from [bgmovie]'s "behind
// everything" (drawBgMovie runs before drawBackground).
func drawLayerMovie(buf *ebiten.Image) {
	if layerMovieVideo == nil {
		return
	}
	img := layerMovieVideo.Image()
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
	op.Blend = layerMovieBlend
	op.ColorScale.ScaleAlpha(float32(layerMovieAlpha() * layerMovieOpacity))
	op.GeoM.Scale(s, s)
	op.GeoM.Translate((float64(sw)-float64(iw)*s)/2, (float64(sh)-float64(ih)*s)/2)
	buf.DrawImage(img, op)
}

// handleLayerModeMovie implements
// [layermode_movie name= video= se= volume= loop= speed= mode= opacity= time= wait=].
// Reuses openMovie (tags_movie.go) — the same govid demuxer/codec
// resolution, AV1/Theora rejection, and errVideosFSUnset "videos" fs.FS
// contract [movie]/[bgmovie] already have.
//
//   - name= (accepted, unused): upstream docs describe it as a class name
//     for CSS targeting, which has no equivalent in this canvas-based
//     engine.
//   - video= (required): resolved against the "videos" fs.FS key — note
//     the attribute is video=, not storage= like [movie]/[bgmovie].
//   - se=: a kag3 extension, not part of upstream's documented attribute
//     list, following the exact same [movie]/[bgmovie] convention: govid
//     never decodes a video's own audio track, so this is the only way to
//     get audible sound. Plays through ses[] under its own
//     "layermode_movie" buffer.
//   - volume= is accepted for compatibility but has no effect, same
//     reason as [bgmovie]'s own volume=.
//   - loop= (default true, matching upstream docs).
//   - speed= is accepted but ignored and logged: govid's Player exposes no
//     playback-rate API at all (only Play/Pause/SetLoop/State).
//   - mode= (default "multiply", matching upstream docs) — shares
//     resolveBlendMode with [layermode] (tags_effects.go); an unsupported
//     value is a hard error, same as [layermode]'s own mode= validation.
//   - opacity= (0-255, default 255) — a static multiplier on top of the
//     fade-in.
//   - time= (default 500ms): fade-in duration.
//   - wait= (default true): blocks until the fade-in completes, not until
//     playback ends — [layermode_movie] has no [wait_bgmovie] equivalent
//     in the official tag list.
func handleLayerModeMovie(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	storage, ok := getString(pm, "video")
	if !ok || storage == "" {
		return fmt.Errorf("[layermode_movie] には video= が必要です")
	}

	stopLayerMovie()

	video, closers, err := openMovie(r, storage)
	if err != nil {
		if errors.Is(err, errVideosFSUnset) {
			fmt.Fprintln(os.Stderr, "[layermode_movie]: \"videos\" のfs.FSが設定されていないため、何もせずスキップします")
			return nil
		}
		return err
	}
	layerMovieVideo = video
	layerMovieClosers = closers
	layerMovieFinished = false

	if _, ok := pm["speed"]; ok {
		fmt.Fprintln(os.Stderr, "[layermode_movie]: speed= は対応していません(govidに再生速度を変えるAPIが無いため無視します)")
	}

	mode := "multiply"
	if v, ok := getString(pm, "mode"); ok {
		mode = v
	}
	blend, ok := resolveBlendMode(mode)
	if !ok {
		return fmt.Errorf("未対応の値です %s", mode)
	}
	layerMovieBlend = blend

	opacity := 255
	if v, ok, gerr := getInt(pm, "opacity"); gerr != nil {
		return gerr
	} else if ok {
		opacity = v
	}
	layerMovieOpacity = float64(opacity) / 255

	loop := true
	if v, ok, gerr := getBool(pm, "loop"); gerr != nil {
		return gerr
	} else if ok {
		loop = v
	}
	layerMovieLoop = loop
	layerMovieVideo.Player().SetLoop(loop)

	fadeMS := 500
	if v, ok, gerr := getInt(pm, "time"); gerr != nil {
		return gerr
	} else if ok {
		fadeMS = v
	}
	layerMovieFadeStartT = t
	layerMovieFadeDurTicks = fadeMS * ebiten.TPS() / 1000

	if seStorage, ok := getString(pm, "se"); ok && seStorage != "" {
		se := &kag3.BGM{Volume: defaultSeVolume}
		if err := swapSEPlayer(r.fses["ses"], layerMovieSeBuf, seStorage, se); err != nil {
			fmt.Fprintln(os.Stderr, "[layermode_movie]: se=", seStorage, "の読み込みに失敗したため、映像のみ再生します:", err)
		} else {
			delete(activeFades, "se:"+layerMovieSeBuf)
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
	startT, dur := layerMovieFadeStartT, layerMovieFadeDurTicks
	ctx.y.Until(true, func() bool { return t-startT >= dur })
	return nil
}
