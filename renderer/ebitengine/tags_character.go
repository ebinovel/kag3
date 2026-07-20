package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func init() {
	register("chara_new", handleCharaNew)
	register("chara_hide", handleCharaHide)
	register("chara_face", handleCharaFace)
	register("chara_mod", handleCharaMod)
	register("chara_show", handleCharaShow)
}

func handleCharaNew(ctx *tagCtx) error {
	r := ctx.r
	object := ctx.tag
	name := object.Pm["name"]
	charaImage, _, err := ebitenutil.NewImageFromFileSystem(
		r.fses["images"],
		object.Pm["storage"],
	)
	if err != nil {
		return err
	}
	charas[name] = &kag3.Character{
		Name:  name,
		Image: charaImage,
		JName: object.Pm["jname"],
		Faces: make(map[string]string),
	}
	charas[name].Faces["default"] = object.Pm["storage"]
	return nil
}

func handleCharaHide(ctx *tagCtx) error {
	object := ctx.tag
	for _, chara := range viewCharas {
		chara.Remove(object.Pm["name"])
		fmt.Printf("chara_hide:%+v\n", chara)
	}
	return nil
}

func handleCharaFace(ctx *tagCtx) error {
	object := ctx.tag
	charas[object.Pm["name"]].Faces[object.Pm["face"]] = object.Pm["storage"]
	return nil
}

func handleCharaMod(ctx *tagCtx) error {
	r := ctx.r
	object := ctx.tag
	charaImage, _, err := ebitenutil.NewImageFromFileSystem(
		r.fses["images"],
		charas[object.Pm["name"]].Faces[object.Pm["face"]],
	)
	if err != nil {
		return err
	}
	charas[object.Pm["name"]].Image = charaImage
	return nil
}

func handleCharaShow(ctx *tagCtx) error {
	chara, err := ctx.r.charaShow(ctx.tag)
	if err != nil {
		return err
	}
	if chara.Wait {
		ctx.y.Until(true, func() bool {
			return float64(t-charaTick)/float64(chara.Time*ebiten.TPS()/1000) >= 1
		})
	}
	return nil
}
