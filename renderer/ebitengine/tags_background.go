package ebitengine

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func init() {
	register("bg", handleBG)
	register("bg2", handleBG2)
}

// fileExists reports whether name exists in fsys — used by handleBG's bg/
// prefix probing below. A nil fsys (as in some minimal test Renderers)
// reports false rather than panicking.
func fileExists(fsys fs.FS, name string) bool {
	if fsys == nil {
		return false
	}
	_, err := fs.Stat(fsys, name)
	return err == nil
}

var (
	bg, bg2 *kag3.Background
	bgTick  int
	// bg2Tick drives bg2's own crossfade/slide transition independently of
	// bg's (see [bg2] in this file), for things like weather overlays.
	bg2Tick int
	// backImgs/backPtexts are a simplified fore/back "page" buffer:
	// [backlay] snapshots imgs/ptexts into them, [trans] swaps them in.
	backImgs   []*kag3.Image
	backPtexts map[string]*kag3.PText
)

func init() {
	bg = &kag3.Background{
		Time:   3000,
		IsWait: true,
		Method: "crossfade",
	}
	bg2 = &kag3.Background{
		Time:   3000,
		IsWait: false,
		Method: "crossfade",
	}
}

func handleBG(ctx *tagCtx) error {
	return applyBGTag(ctx, bg, &bgTick)
}

// handleBG2 drives a second, independent background layer (bg2/bg2Tick),
// composited on top of the main one in Draw. Same attributes and
// transition machinery as [bg], just a separate slot — useful for things
// like weather overlays layered over the main scene.
func handleBG2(ctx *tagCtx) error {
	return applyBGTag(ctx, bg2, &bg2Tick)
}

// applyBGTag implements [bg]/[bg2]: both take identical attributes and
// only differ in which *kag3.Background/tick they drive.
func applyBGTag(ctx *tagCtx, target *kag3.Background, tick *int) error {
	r := ctx.r
	object := ctx.tag
	if traceTags {
		fmt.Printf("bg:%+v\n", target)
	}
	*tick = t
	target.IsEnd = false
	images := "images"
	for key, value := range object.Pm {
		var err error
		switch key {
		case "time":
			target.Time, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "wait":
			switch value {
			case "true":
				target.IsWait = true
			case "false":
				target.IsWait = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "cross":
			switch value {
			case "true":
				target.IsCross = true
			case "false":
				target.IsCross = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "position":
			switch value {
			case "left", "center", "right", "top", "bottom":
				target.Position = value
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "method":
			if slices.Contains(kag3.BackgroundMethod, value) {
				target.Method = value
			} else {
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "system":
			switch value {
			case "true":
				target.IsSystem = true
				images = "system/images"
			case "false":
				target.IsSystem = false
				images = "images"
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		}
	}
	// Real TyranoScript's [bg storage=] is relative to its own bgimage/
	// folder, never spelled out by the author — matching that (and this
	// editor's own images/bg/ project convention, see
	// internal/project/asset.go's ImageSubdirs in ebinovel-editor) means
	// storage="room.jpg" alone must resolve to images/bg/room.jpg, not
	// require storage="bg/room.jpg". Only applies to the plain project
	// "images" root; system=true switches to system/images, which has no
	// such bg/ subfolder convention (it holds UI chrome like buttons and
	// cursors), so an author-provided path there is used exactly as
	// written.
	//
	// The bg/ prefix is only added when images/bg/<storage> actually
	// exists — tried first, falling back to the literal author-written
	// path otherwise. A blind, unconditional path.Join("bg", storage)
	// broke any [bg] call whose storage= already names a different
	// images/-relative subfolder — e.g. config.ks's
	// [bg storage="&tf.img_path+'bg_config.png'"] (tf.img_path="config/"),
	// which lives at images/config/bg_config.png, not images/bg/config/
	// bg_config.png — by rewriting it into a path that never exists and
	// crashing the whole renderer instead of just failing to find a
	// background.
	storagePath := object.Pm["storage"]
	if storagePath != "" && images == "images" {
		if withBg := path.Join("bg", storagePath); fileExists(r.fses[images], withBg) {
			storagePath = withBg
		}
	}
	var err error
	target.NextImage, _, err = ebitenutil.NewImageFromFileSystem(
		r.fses[images],
		storagePath,
	)
	if err != nil {
		return err
	}
	// Storage tracks whatever's currently requested (the *resolved*
	// images/-relative path, including the bg/ prefix above — NOT the
	// raw author-written attribute), independent of whether the
	// transition has visually finished — save/load (see save_apply.go)
	// uses it to reconstruct the background image in a fresh process,
	// where NextImage/Image can't be persisted directly, by feeding it
	// straight back into the same fs.FS lookup (applyBgFromSnapshot)
	// with no further resolution of its own.
	target.Storage = storagePath
	// [mode_effect enabled="false"] (tags_sysdesign.go) skips the animated
	// crossfade/etc. entirely: swap straight to the new image instead of
	// staging it as NextImage for drawScene's transition to animate.
	if !effectsEnabled {
		target.Image = target.NextImage
		target.NextImage = nil
		target.IsEnd = true
		return nil
	}
	if target.IsWait {
		ctx.y.Until(true, func() bool {
			return target.IsEnd
		})
	}
	return nil
}
