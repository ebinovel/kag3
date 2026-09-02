package ebitengine

import (
	"bytes"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

func init() {
	register("playbgm", handlePlayBGM)
	register("stopbgm", handleStopBGM)
	register("fadeinbgm", handleFadeInBGM)
	register("fadeoutbgm", handleFadeOutBGM)
	register("xchgbgm", handleXchgBGM)
	register("pausebgm", handlePauseBGM)
	register("resumebgm", handleResumeBGM)
	register("wbgm", handleWBGM)
	register("bgmopt", handleBGMOpt)
	register("playse", handlePlaySE)
	register("stopse", handleStopSE)
	register("fadeinse", handleFadeInSE)
	register("fadeoutse", handleFadeOutSE)
	register("pausese", handlePauseSE)
	register("resumese", handleResumeSE)
	register("wse", handleWSE)
	register("seopt", handleSEOpt)
	register("changevol", handleChangeVol)
	register("popopo", handlePopopo)
}

var (
	// currentBGM is the single background-music track. Real Tyrano supports
	// multiple bgm slots (Config.DefaultBgmSlotNum); this engine has one.
	currentBGM *kag3.BGM
	// ses holds concurrently-playing sound effects keyed by "buf".
	ses = map[string]*kag3.BGM{}
	// defaultBgmVolume/defaultSeVolume are the Volume a [playbgm]/[playse]
	// (and their fadein/xchg variants) starts at when the tag itself
	// doesn't specify volume= — [bgmopt]/[seopt] (handleBGMOpt/handleSEOpt
	// below) update these unconditionally, even with nothing currently
	// playing, so a settings screen's volume slider (config.ks) affects
	// future playback too, not just whatever happens to be playing right
	// now.
	defaultBgmVolume = 100
	defaultSeVolume  = 100
	// activeFades holds in-progress volume ramps, advanced once per frame
	// by stepAudioFades (called from Update()). Keys: "bgm" for the current
	// track, "bgm_old" for a track xchgbgm/fadeoutbgm is fading out on its
	// way to being closed, "se:<buf>" for a sound effect.
	activeFades  = map[string]*audioFade{}
	audioContext *audio.Context
	bgmTick      int
)

func init() {
	audioContext = audio.NewContext(44100)
}

type audioFade struct {
	player     *audio.Player
	startVol   float64
	targetVol  float64
	startT     int
	durTicks   int
	onComplete func()
}

func volFraction(vol int) float64 {
	return float64(vol) / 100
}

// loadAudioPlayer reads storage from fsys, decodes it as Ogg Vorbis, and
// returns a ready-to-play Player. When loop is true the decoded stream is
// wrapped in an InfiniteLoop first — vorbis.Stream already implements
// io.ReadSeeker and Length(), which is exactly what that needs.
func loadAudioPlayer(fsys fs.FS, storage string, loop bool) (*audio.Player, error) {
	b, err := fs.ReadFile(fsys, storage)
	if err != nil {
		return nil, err
	}
	v, err := vorbis.DecodeF32(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if loop {
		return audioContext.NewPlayerF32(audio.NewInfiniteLoopF32(v, v.Length()))
	}
	return audioContext.NewPlayerF32(v)
}

// startFade registers a volume ramp that stepAudioFades advances once per
// frame. ms<=0 applies the target volume immediately instead of scheduling
// a fade.
func startFade(key string, player *audio.Player, from, to float64, ms int, onComplete func()) {
	ticks := ms * ebiten.TPS() / 1000
	if ticks <= 0 {
		player.SetVolume(to)
		if onComplete != nil {
			onComplete()
		}
		delete(activeFades, key)
		return
	}
	player.SetVolume(from)
	activeFades[key] = &audioFade{player: player, startVol: from, targetVol: to, startT: t, durTicks: ticks, onComplete: onComplete}
}

// stepAudioFades advances every active fade by one frame. ebiten's
// audio.Player has no built-in fade support, so volume has to be walked
// forward externally; called from Update() alongside the coroutine step.
func stepAudioFades() {
	for key, f := range activeFades {
		elapsed := t - f.startT
		if elapsed >= f.durTicks {
			f.player.SetVolume(f.targetVol)
			if f.onComplete != nil {
				f.onComplete()
			}
			delete(activeFades, key)
			continue
		}
		frac := float64(elapsed) / float64(f.durTicks)
		f.player.SetVolume(f.startVol + (f.targetVol-f.startVol)*frac)
	}
}

// parseAudioOptions fills in the shared subset of attributes used across
// playbgm/fadeinbgm/xchgbgm/playse/fadeinse.
func parseAudioOptions(pm map[string]string, a *kag3.BGM) error {
	for key, value := range pm {
		switch key {
		case "loop":
			switch value {
			case "true":
				a.Loop = true
			case "false":
				a.Loop = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "sprite_time":
			a.SpriteTime = value
		case "volume":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			a.Volume = v
		case "seek":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			a.Seek = v
		case "time":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			a.Time = v
		}
	}
	return nil
}

func handlePlayBGM(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	bgmTick = t
	bgm := &kag3.BGM{Volume: defaultBgmVolume}
	if err := parseAudioOptions(object.Pm, bgm); err != nil {
		return err
	}
	player, err := loadAudioPlayer(r.fses["bgms"], object.Pm["storage"], bgm.Loop)
	if err != nil {
		return err
	}
	bgm.Player = player
	player.SetVolume(volFraction(bgm.Volume))
	player.Play()
	delete(activeFades, "bgm")
	currentBGM = bgm
	return nil
}

func handleStopBGM(ctx *tagCtx) error {
	if currentBGM != nil && currentBGM.Player != nil {
		currentBGM.Player.Pause()
		currentBGM.Player.Close()
	}
	delete(activeFades, "bgm")
	currentBGM = nil
	return nil
}

func handleFadeInBGM(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	bgm := &kag3.BGM{Volume: defaultBgmVolume, Time: 1000}
	if err := parseAudioOptions(object.Pm, bgm); err != nil {
		return err
	}
	player, err := loadAudioPlayer(r.fses["bgms"], object.Pm["storage"], bgm.Loop)
	if err != nil {
		return err
	}
	bgm.Player = player
	player.Play()
	currentBGM = bgm
	startFade("bgm", player, 0, volFraction(bgm.Volume), bgm.Time, nil)
	return nil
}

// handleFadeOutBGM fades the current track out under the "bgm_old" key
// (not "bgm") so a [playbgm]/[fadeinbgm] started before the fade finishes
// doesn't collide with — and orphan — this one.
func handleFadeOutBGM(ctx *tagCtx) error {
	if currentBGM == nil || currentBGM.Player == nil {
		return nil
	}
	ms, _ := strconv.Atoi(ctx.tag.Pm["time"])
	if ms <= 0 {
		ms = 1000
	}
	player := currentBGM.Player
	startFade("bgm_old", player, player.Volume(), 0, ms, func() {
		player.Pause()
		player.Close()
	})
	currentBGM = nil
	return nil
}

// handleXchgBGM crossfades: the outgoing track fades out under "bgm_old"
// while the incoming one plays and fades in under "bgm", same convention
// as handleFadeOutBGM.
func handleXchgBGM(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	ms, _ := strconv.Atoi(object.Pm["time"])
	if ms <= 0 {
		ms = 1000
	}
	if currentBGM != nil && currentBGM.Player != nil {
		old := currentBGM.Player
		startFade("bgm_old", old, old.Volume(), 0, ms, func() {
			old.Pause()
			old.Close()
		})
	}
	bgm := &kag3.BGM{Volume: defaultBgmVolume, Time: ms}
	if err := parseAudioOptions(object.Pm, bgm); err != nil {
		return err
	}
	player, err := loadAudioPlayer(r.fses["bgms"], object.Pm["storage"], bgm.Loop)
	if err != nil {
		return err
	}
	bgm.Player = player
	player.Play()
	currentBGM = bgm
	startFade("bgm", player, 0, volFraction(bgm.Volume), ms, nil)
	return nil
}

func handlePauseBGM(ctx *tagCtx) error {
	if currentBGM != nil && currentBGM.Player != nil {
		currentBGM.Player.Pause()
	}
	return nil
}

func handleResumeBGM(ctx *tagCtx) error {
	if currentBGM != nil && currentBGM.Player != nil && !currentBGM.Player.IsPlaying() {
		currentBGM.Player.Play()
	}
	return nil
}

func handleWBGM(ctx *tagCtx) error {
	ctx.y.Until(true, func() bool {
		return currentBGM == nil || currentBGM.Player == nil || !currentBGM.Player.IsPlaying()
	})
	return nil
}

// handleBGMOpt adjusts the current track's volume without restarting
// playback. loop can't be changed this way — looping is baked into the
// player's stream at creation time — so a loop= attribute here is ignored.
func handleBGMOpt(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["volume"]; ok {
		vol, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		defaultBgmVolume = vol
		if currentBGM != nil && currentBGM.Player != nil {
			currentBGM.Volume = vol
			currentBGM.Player.SetVolume(volFraction(vol))
		}
	}
	return nil
}

func swapSEPlayer(fsys fs.FS, buf, storage string, se *kag3.BGM) error {
	player, err := loadAudioPlayer(fsys, storage, se.Loop)
	if err != nil {
		return err
	}
	se.Player = player
	setSEPlayer(buf, se)
	return nil
}

// setSEPlayer registers se (its Player already built) under buf, pausing
// and closing whatever was there before. Split out of swapSEPlayer so
// callers that already have a ready-made *audio.Player from somewhere
// other than a file — [speak_on]'s synthesized WAV bytes
// (stepSpeechSynthesis, tags_speech.go) being the reason this exists —
// can register it without swapSEPlayer's fs.FS/loadAudioPlayer detour.
func setSEPlayer(buf string, se *kag3.BGM) {
	if old, ok := ses[buf]; ok && old.Player != nil {
		old.Player.Pause()
		old.Player.Close()
	}
	ses[buf] = se
}

func handlePlaySE(ctx *tagCtx) error {
	object := ctx.tag
	buf := object.Pm["buf"]
	storage := object.Pm["storage"]
	if buf == "" {
		buf = storage
	}
	se := &kag3.BGM{Volume: defaultSeVolume}
	if err := parseAudioOptions(object.Pm, se); err != nil {
		return err
	}
	if err := swapSEPlayer(ctx.r.fses["ses"], buf, storage, se); err != nil {
		return err
	}
	delete(activeFades, "se:"+buf)
	se.Player.SetVolume(volFraction(se.Volume))
	se.Player.Play()
	return nil
}

func handleStopSE(ctx *tagCtx) error {
	buf := ctx.tag.Pm["buf"]
	if buf == "" {
		for k, se := range ses {
			if se.Player != nil {
				se.Player.Pause()
				se.Player.Close()
			}
			delete(activeFades, "se:"+k)
			delete(ses, k)
		}
		return nil
	}
	if se, ok := ses[buf]; ok {
		if se.Player != nil {
			se.Player.Pause()
			se.Player.Close()
		}
		delete(activeFades, "se:"+buf)
		delete(ses, buf)
	}
	return nil
}

func handleFadeInSE(ctx *tagCtx) error {
	object := ctx.tag
	buf := object.Pm["buf"]
	storage := object.Pm["storage"]
	if buf == "" {
		buf = storage
	}
	se := &kag3.BGM{Volume: defaultSeVolume, Time: 1000}
	if err := parseAudioOptions(object.Pm, se); err != nil {
		return err
	}
	if err := swapSEPlayer(ctx.r.fses["ses"], buf, storage, se); err != nil {
		return err
	}
	se.Player.Play()
	startFade("se:"+buf, se.Player, 0, volFraction(se.Volume), se.Time, nil)
	return nil
}

func handleFadeOutSE(ctx *tagCtx) error {
	buf := ctx.tag.Pm["buf"]
	se, ok := ses[buf]
	if !ok || se.Player == nil {
		return nil
	}
	ms, _ := strconv.Atoi(ctx.tag.Pm["time"])
	if ms <= 0 {
		ms = 1000
	}
	player := se.Player
	delete(ses, buf)
	startFade("se:"+buf, player, player.Volume(), 0, ms, func() {
		player.Pause()
		player.Close()
	})
	return nil
}

func handlePauseSE(ctx *tagCtx) error {
	buf := ctx.tag.Pm["buf"]
	if buf == "" {
		for _, se := range ses {
			if se.Player != nil {
				se.Player.Pause()
			}
		}
		return nil
	}
	if se, ok := ses[buf]; ok && se.Player != nil {
		se.Player.Pause()
	}
	return nil
}

func handleResumeSE(ctx *tagCtx) error {
	buf := ctx.tag.Pm["buf"]
	if buf == "" {
		for _, se := range ses {
			if se.Player != nil && !se.Player.IsPlaying() {
				se.Player.Play()
			}
		}
		return nil
	}
	if se, ok := ses[buf]; ok && se.Player != nil && !se.Player.IsPlaying() {
		se.Player.Play()
	}
	return nil
}

func handleWSE(ctx *tagCtx) error {
	buf := ctx.tag.Pm["buf"]
	if buf != "" {
		ctx.y.Until(true, func() bool {
			se, ok := ses[buf]
			return !ok || se.Player == nil || !se.Player.IsPlaying()
		})
		return nil
	}
	ctx.y.Until(true, func() bool {
		for _, se := range ses {
			if se.Player != nil && se.Player.IsPlaying() {
				return false
			}
		}
		return true
	})
	return nil
}

// handleSEOpt adjusts buf='s volume live if it's currently playing. A
// volume= with no buf= (or a buf= naming a sound that isn't currently
// playing) still updates defaultSeVolume — a settings screen's SE-volume
// slider (config.ks) has no specific "buf" to name, and needs to affect
// future [playse] calls regardless of whether anything's playing right now.
func handleSEOpt(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["volume"]
	if !ok {
		return nil
	}
	vol, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	defaultSeVolume = vol
	buf := ctx.tag.Pm["buf"]
	if se, ok := ses[buf]; ok && se.Player != nil {
		se.Volume = vol
		se.Player.SetVolume(volFraction(vol))
	}
	return nil
}

// handleChangeVol changes the volume of the current BGM (buf omitted or
// "bgm") or a specific SE slot (buf set to its name), optionally ramped
// over time= milliseconds instead of applied instantly.
func handleChangeVol(ctx *tagCtx) error {
	object := ctx.tag
	vol, err := strconv.Atoi(object.Pm["vol"])
	if err != nil {
		return err
	}
	ms, _ := strconv.Atoi(object.Pm["time"])
	buf := object.Pm["buf"]

	var player *audio.Player
	var key string
	if buf == "" || buf == "bgm" {
		if currentBGM == nil || currentBGM.Player == nil {
			return nil
		}
		currentBGM.Volume = vol
		player = currentBGM.Player
		key = "bgm"
	} else {
		se, ok := ses[buf]
		if !ok || se.Player == nil {
			return nil
		}
		se.Volume = vol
		player = se.Player
		key = "se:" + buf
	}
	if ms <= 0 {
		player.SetVolume(volFraction(vol))
		return nil
	}
	startFade(key, player, player.Volume(), volFraction(vol), ms, nil)
	return nil
}

// handlePopopo plays Tyrano's signature built-in "popopo" sound effect
// from a conventional ses/popopo.ogg path. kag3 doesn't bundle that asset,
// so a missing file is logged and skipped rather than treated as an error.
func handlePopopo(ctx *tagCtx) error {
	player, err := loadAudioPlayer(ctx.r.fses["ses"], "popopo.ogg", false)
	if err != nil {
		fmt.Println("[popopo] popopo.ogg not found, skipping")
		return nil
	}
	player.SetVolume(1)
	player.Play()
	return nil
}
