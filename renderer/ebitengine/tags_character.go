package ebitengine

import (
	"fmt"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func init() {
	register("chara_new", handleCharaNew)
	register("chara_hide", handleCharaHide)
	register("chara_hide_all", handleCharaHideAll)
	register("chara_delete", handleCharaDelete)
	register("chara_face", handleCharaFace)
	register("chara_mod", handleCharaMod)
	register("chara_show", handleCharaShow)
	register("chara_move", handleCharaMove)
	register("chara_config", handleCharaConfig)
	register("chara_layer", handleCharaLayer)
	register("chara_layer_mod", handleCharaLayer)
	register("chara_part", handleCharaPart)
	register("chara_part_reset", handleCharaPartReset)
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
		Name:    name,
		Image:   charaImage,
		JName:   object.Pm["jname"],
		Faces:   make(map[string]string),
		Storage: object.Pm["storage"],
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
	name := object.Pm["name"]
	chara, ok := charas[name]
	if !ok {
		fmt.Printf("chara_mod: %s は登録されていないためスキップします\n", name)
		return nil
	}
	// A save/load resumed mid-script (see the save/load gaps note in
	// tags_save.go) skips whatever [chara_face] declarations came before the
	// jump point, so a face this character legitimately has in the real
	// scenario can be missing from the process-lifetime charas registry —
	// same class of gap charaShow's own "face" handling already guards
	// against (renderer.go's [chara_show face=...] case). Log and keep
	// whatever's currently showing rather than opening an empty path and
	// crashing the whole coroutine (see initScript's loop: any error from a
	// tag handler panics).
	storage, ok := chara.Faces[object.Pm["face"]]
	if !ok {
		fmt.Printf("chara_mod: %s の表情 %s は登録されていないためスキップします\n", name, object.Pm["face"])
		return nil
	}
	charaImage, _, err := ebitenutil.NewImageFromFileSystem(
		r.fses["images"],
		storage,
	)
	if err != nil {
		return err
	}
	chara.Image = charaImage
	// Keep Storage tracking whatever's actually showing, so a save made
	// after a face change restores the same face rather than the original
	// [chara_new] default.
	chara.Storage = storage
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

func handleCharaHideAll(ctx *tagCtx) error {
	for _, c := range viewCharas {
		c.IsRemove = true
	}
	return nil
}

// handleCharaDelete removes a character's definition entirely (unlike
// [chara_hide], which just fades it out of view but keeps the definition
// around for a later [chara_show]).
func handleCharaDelete(ctx *tagCtx) error {
	name := ctx.tag.Pm["name"]
	delete(charas, name)
	var remaining []*kag3.CharaShow
	for _, c := range viewCharas {
		if c.Name != name {
			remaining = append(remaining, c)
		}
	}
	viewCharas = remaining
	return nil
}

// handleCharaMove repositions an already-shown character, reusing the same
// slide-in tween (IsSlide/NewLeft) [chara_show] uses when a character first
// appears. left/top default to the character's current position when
// omitted; wait defaults to true.
func handleCharaMove(ctx *tagCtx) error {
	object := ctx.tag
	name := object.Pm["name"]
	var target *kag3.CharaShow
	for _, c := range viewCharas {
		if c.Name == name {
			target = c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("そのキャラクターは表示されてません name=%s", name)
	}

	moveTime := 1000
	wait := true
	newLeft := target.Left
	for key, value := range object.Pm {
		switch key {
		case "left":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			newLeft = v
		case "top":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			target.Top = v
		case "time":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			moveTime = v
		case "wait":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			wait = v
		}
	}
	target.NewLeft = newLeft
	target.Time = moveTime
	target.IsSlide = true
	charaTick = t
	if wait {
		ctx.y.Until(true, func() bool {
			return float64(t-charaTick)/float64(target.Time*ebiten.TPS()/1000) >= 1
		})
	}
	return nil
}

// handleCharaConfig records which named [ptext] area (if any) doubles as
// the #name-line name-plate — see drawPTexts in renderer.go.
func handleCharaConfig(ctx *tagCtx) error {
	if name, ok := ctx.tag.Pm["ptext"]; ok {
		charaNamePText = name
	}
	return nil
}

// handleCharaLayer (also registered for [chara_layer_mod], which redefines
// an existing part the same way) registers a differential-part image
// variant under a character's layer slot. [chara_part] switches which
// variant is active for that slot; drawCharaParts composites the active
// ones over the character's base image.
func handleCharaLayer(ctx *tagCtx) error {
	object := ctx.tag
	r := ctx.r
	name := object.Pm["name"]
	layer := object.Pm["layer"]
	part := object.Pm["part"]
	c, ok := charas[name]
	if !ok {
		return fmt.Errorf("そのキャラクターは登録されてません name=%s", name)
	}
	img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["images"], object.Pm["storage"])
	if err != nil {
		return err
	}
	if c.Parts == nil {
		c.Parts = map[string]map[string]*ebiten.Image{}
	}
	if c.Parts[layer] == nil {
		c.Parts[layer] = map[string]*ebiten.Image{}
	}
	c.Parts[layer][part] = img
	return nil
}

func handleCharaPart(ctx *tagCtx) error {
	object := ctx.tag
	name := object.Pm["name"]
	layer := object.Pm["layer"]
	part := object.Pm["part"]
	c, ok := charas[name]
	if !ok {
		return fmt.Errorf("そのキャラクターは登録されてません name=%s", name)
	}
	if c.ActivePart == nil {
		c.ActivePart = map[string]string{}
	}
	c.ActivePart[layer] = part
	return nil
}

// handleCharaPartReset clears one layer slot back to no override, or every
// slot if layer= is omitted.
func handleCharaPartReset(ctx *tagCtx) error {
	object := ctx.tag
	name := object.Pm["name"]
	c, ok := charas[name]
	if !ok {
		return fmt.Errorf("そのキャラクターは登録されてません name=%s", name)
	}
	layer := object.Pm["layer"]
	if layer == "" {
		c.ActivePart = map[string]string{}
		return nil
	}
	delete(c.ActivePart, layer)
	return nil
}
