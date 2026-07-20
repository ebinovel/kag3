package ebitengine

import (
	"bytes"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

func init() {
	register("playbgm", handlePlayBGM)
}

func handlePlayBGM(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	bgmTick = t
	fmt.Println("playbgm")
	bgm := &kag3.BGM{}
	bgm.Volume = 100
	b, err := fs.ReadFile(r.fses["bgms"], object.Pm["storage"])
	if err != nil {
		return err
	}
	v, err := vorbis.DecodeF32(bytes.NewReader(b))
	if err != nil {
		return err
	}
	bgm.Player, err = audioContext.NewPlayerF32(v)
	if err != nil {
		return err
	}
	for key, value := range object.Pm {
		switch key {
		case "loop":
			switch value {
			case "true":
				bgm.Loop = true
			case "false":
				bgm.Loop = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "sprite_time":
			bgm.SpriteTime = value
		case "volume":
			bgm.Volume, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "pause":
			switch value {
			case "true":
				bgm.Pause = true
			case "false":
				bgm.Pause = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "seek":
			bgm.Seek, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "restart":
			switch value {
			case "true":
				bgm.Restart = true
			case "false":
				bgm.Restart = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "time":
			bgm.Time, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		}
	}
	bgm.Player.SetVolume(float64(bgm.Volume / 100))
	if bgm.Restart {
		bgm.Player.Rewind()
	}
	if !bgm.Player.IsPlaying() {
		bgm.Player.Rewind()
		bgm.Player.Play()
	}
	return nil
}
