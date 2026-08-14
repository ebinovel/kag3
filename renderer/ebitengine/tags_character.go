package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
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
	register("chara_ptext", handleCharaPText)
}

var (
	charas     map[string]*kag3.Character
	viewCharas []*kag3.CharaShow
	// charaName is the current speaker, set by a "#name" line (parser.go's
	// characterPText produces a Chara-tagged TextObject that macro.go's
	// execItem assigns this from) and cleared by a bare "#" line the same
	// way. It's deliberately *not* touched by [p]/[cm] (tags_text.go) —
	// real Tyrano scripts (e.g. scene1.ks) declare "#name" once and then
	// write several [p]-separated lines under it with no repeated "#name"
	// in between, so it has to persist across those or the name-plate
	// ptext (ptextContent, renderer.go) and fuki-mode positioning
	// (applyFukiPosition, tags_message.go) would both go blank after the
	// very first line.
	charaName string
	charaTick int
)

func init() {
	charas = make(map[string]*kag3.Character)
}

// mustChara looks up name in charas, returning the same error message every
// [chara_*] handler already constructs by hand when the name isn't
// registered ([chara_show]/[chara_layer]/[chara_layer_mod]/[chara_part]).
func mustChara(name string) (*kag3.Character, error) {
	c, ok := charas[name]
	if !ok {
		return nil, fmt.Errorf("そのキャラクターは登録されてません name=%s", name)
	}
	return c, nil
}

func handleCharaNew(ctx *tagCtx) error {
	r := ctx.r
	object := ctx.tag
	name := object.Pm["name"]
	charaImage, err := loadImage(r, "", object.Pm["storage"])
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
		if traceTags {
			fmt.Printf("chara_hide:%+v\n", chara)
		}
	}
	return nil
}

// handleCharaFace registers one face variant against an already-[chara_new]'d
// character. Resolved through mustChara like every other [chara_*] handler
// in this file: this one alone used to index charas directly and nil-deref
// on any name that isn't registered — a plain typo in name=, or a
// [chara_face] reached before its own [chara_new] ran, both of which took
// the whole game down instead of reporting which character was missing.
// The Faces nil-guard covers a *kag3.Character built without one (only
// [chara_new]/[chara_new_psd]/reconcileViewCharas populate it today, but a
// nil map assignment would be the same class of crash this fix exists to
// remove).
func handleCharaFace(ctx *tagCtx) error {
	object := ctx.tag
	c, err := mustChara(object.Pm["name"])
	if err != nil {
		return err
	}
	if c.Faces == nil {
		c.Faces = make(map[string]string)
	}
	c.Faces[object.Pm["face"]] = object.Pm["storage"]
	return nil
}

func handleCharaMod(ctx *tagCtx) error {
	return applyCharaFace(ctx.r, ctx.tag.Pm["name"], ctx.tag.Pm["face"])
}

// applyCharaFace swaps name's standing image to whatever [chara_face]
// registered under face, keeping Storage in sync — the face-change logic
// shared by [chara_mod face=] and [chara_ptext face=]. Deliberately doesn't
// touch Parts/ActivePart ([chara_part]'s differential-parts system):
// upstream Tyrano's own [chara_ptext] docs say as much explicitly
// ("表情差分パーツ機能（[chara_part]タグ）には対応していません"), and [chara_mod]
// never has either.
//
// A save/load resumed mid-script (see the save/load gaps note in
// save_data.go) skips whatever [chara_face] declarations came before the
// jump point, so a face this character legitimately has in the real
// scenario can be missing from the process-lifetime charas registry — same
// class of gap charaShow's own "face" handling already guards against
// (renderer.go's [chara_show face=...] case). Both failure cases below log
// and leave whatever's currently showing untouched rather than opening an
// empty path and crashing the whole coroutine (see initScript's loop: any
// error from a tag handler panics).
func applyCharaFace(r *Renderer, name, face string) error {
	chara, err := mustChara(name)
	if err != nil {
		fmt.Printf("chara face 変更: %s は登録されていないためスキップします\n", name)
		return nil
	}
	storage, ok := chara.Faces[face]
	if !ok {
		fmt.Printf("chara face 変更: %s の表情 %s は登録されていないためスキップします\n", name, face)
		return nil
	}
	charaImage, err := loadImage(r, "", storage)
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

// handleCharaPText implements [chara_ptext name= face=]: assigns charaName
// exactly like a "#name" scenario line does (parser.go's characterPText +
// macro.go's execItem), including the same auto-voice hookup via
// playCharaVoice — this tag just lets a script trigger that update
// explicitly instead of only via the "#name" shorthand. name="" (or
// omitted) clears the speaker, the same monologue state a bare "#" line
// produces: ptextContent (draw_ptext.go) then resolves the name-plate to
// empty and skips drawing it entirely. face=, if given, additionally swaps
// the standing image via applyCharaFace, same as [chara_mod face=].
func handleCharaPText(ctx *tagCtx) error {
	r := ctx.r
	name := ctx.tag.Pm["name"]
	charaName = name
	r.playCharaVoice(charaName)
	if face, ok := ctx.tag.Pm["face"]; ok {
		return applyCharaFace(r, name, face)
	}
	return nil
}

// applyCharaShowAttrs parses [chara_show]'s attributes onto an existing
// *kag3.CharaShow, overwriting only the fields whose attribute was actually
// given — split out of handleCharaShow so it runs identically whether the
// character is being shown for the first time or re-shown after
// [chara_hide] (see handleCharaShow's own comment on why re-display can't
// skip this).
func applyCharaShowAttrs(r *Renderer, chara *kag3.CharaShow, name string, pm map[string]string) error {
	if v, ok := getString(pm, "name"); ok {
		chara.Name = v
	}
	if v, ok, err := getInt(pm, "time"); err != nil {
		return err
	} else if ok {
		chara.Time = v
	}
	if v, ok, err := getInt(pm, "zindex"); err != nil {
		return err
	} else if ok {
		chara.Zindex = v
	}
	if v, ok := getString(pm, "depth"); ok {
		chara.Depth = v
	}
	if v, ok := getString(pm, "page"); ok {
		chara.Page = v
	}
	if v, ok, err := getBool(pm, "wait"); err != nil {
		return err
	} else if ok {
		chara.Wait = v
	}
	if v, ok := getString(pm, "face"); ok {
		if fv, ok := charas[name].Faces[v]; ok {
			chara.Face = fv
		}
	}
	if v, ok := getString(pm, "storage"); ok {
		charaImage, err := loadImage(r, "", v)
		if err != nil {
			return err
		}
		charas[name].Image = charaImage
	}
	if v, ok, err := getBool(pm, "refrect"); err != nil {
		return err
	} else if ok {
		chara.Reflect = v
	}
	if v, ok, err := getInt(pm, "width"); err != nil {
		return err
	} else if ok {
		chara.Width = v
	}
	if v, ok, err := getInt(pm, "height"); err != nil {
		return err
	} else if ok {
		chara.Height = v
	}
	if v, ok, err := getInt(pm, "left"); err != nil {
		return err
	} else if ok {
		chara.Left = v
	}
	if v, ok, err := getInt(pm, "top"); err != nil {
		return err
	} else if ok {
		chara.Top = v
	}
	return nil
}

func handleCharaShow(ctx *tagCtx) error {
	r := ctx.r
	object := ctx.tag
	chara := &kag3.CharaShow{}
	charaTick = t
	charaNew := true
	name := object.Pm["name"]
	if _, err := mustChara(name); err != nil {
		return err
	}
	for _, c := range viewCharas {
		if c.Name == name && c.IsRemove {
			c.IsRemove = false
			charaNew = false
			chara = c
		}
	}
	if charaNew {
		chara.Wait = true
		chara.Time = 1000
		chara.Opacity = 255
		chara.ScaleX = 1
		chara.ScaleY = 1
	}
	// Applied regardless of charaNew: a re-[chara_show] of a [chara_hide]'d
	// character (charaNew==false) must still honor its own attributes
	// (e.g. [chara_show name=x left=800] after [chara_hide name=x]) rather
	// than silently ignoring every attribute but leaving whatever the
	// character's position/face/etc. happened to be before it was hidden.
	// Unspecified attributes simply leave chara's existing field alone,
	// which is exactly re-display's intended semantics (and, for charaNew,
	// is applied on top of the zero-value defaults just above).
	if err := applyCharaShowAttrs(r, chara, name, object.Pm); err != nil {
		return err
	}
	herfWidth := charas[name].Image.Bounds().Dx() / 2
	charaSpace := r.manager.Config.ScreenWidth / (len(viewCharas) + 2)
	currentLeft := charaSpace
	if chara.Left == 0 && chara.Top == 0 {
		chara.Left = charaSpace - herfWidth
		chara.Top = r.manager.Config.ScreenHeight - charas[name].Image.Bounds().Dy()
	}
	for _, c := range viewCharas {
		currentLeft += charaSpace
		// c ranges over every currently-shown character, not just the one
		// this call is about (already guarded at the top via name) — a
		// sibling could in principle be an unreconciled load-restored entry
		// (see reconcileViewCharas in save_apply.go), so re-check here too.
		sibling, ok := charas[c.Name]
		if !ok || sibling.Image == nil {
			continue
		}
		left := currentLeft - (sibling.Image.Bounds().Dx() / 2)
		c.NewLeft = left
		if charaNew {
			c.IsSlide = true
		}
	}
	if charaNew {
		viewCharas = append(viewCharas, chara)
	}
	if traceTags {
		fmt.Printf("viewCharas:%+v\n", chara)
		fmt.Printf("viewCharas:%+v\n", viewCharas)
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
	target := findViewChara(name)
	if target == nil {
		return fmt.Errorf("そのキャラクターは表示されてません name=%s", name)
	}

	pm := object.Pm
	moveTime := 1000
	wait := true
	newLeft := target.Left
	if v, ok, err := getInt(pm, "left"); err != nil {
		return err
	} else if ok {
		newLeft = v
	}
	if v, ok, err := getInt(pm, "top"); err != nil {
		return err
	} else if ok {
		target.Top = v
	}
	if v, ok, err := getInt(pm, "time"); err != nil {
		return err
	} else if ok {
		moveTime = v
	}
	if v, ok, err := getBool(pm, "wait"); err != nil {
		return err
	} else if ok {
		wait = v
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
	c, err := mustChara(name)
	if err != nil {
		return err
	}
	img, err := loadImage(r, "", object.Pm["storage"])
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
	c, err := mustChara(name)
	if err != nil {
		return err
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
	c, err := mustChara(name)
	if err != nil {
		return err
	}
	layer := object.Pm["layer"]
	if layer == "" {
		c.ActivePart = map[string]string{}
		return nil
	}
	delete(c.ActivePart, layer)
	return nil
}
