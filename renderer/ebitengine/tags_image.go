package ebitengine

import "github.com/ebinovel/kag3"

func init() {
	register("image", handleImage)
	register("free", handleFree)
	register("freeimage", handleFreeImage)
	register("backlay", handleBackLay)
	register("trans", handleTrans)
}

var imgs []*kag3.Image

func handleImage(ctx *tagCtx) error {
	r := ctx.r
	pm := ctx.tag.Pm
	img := &kag3.Image{Opacity: 255, ScaleX: 1, ScaleY: 1}
	if v, ok := getString(pm, "layer"); ok {
		img.Layer = v
	}
	if v, ok := getString(pm, "page"); ok {
		img.Page = v
	}
	if v, ok, err := getInt(pm, "left"); err != nil {
		return err
	} else if ok {
		img.Left = v
	}
	if v, ok, err := getInt(pm, "top"); err != nil {
		return err
	} else if ok {
		img.Top = v
	}
	if v, ok, err := getInt(pm, "x"); err != nil {
		return err
	} else if ok {
		img.X = v
	}
	if v, ok, err := getInt(pm, "y"); err != nil {
		return err
	} else if ok {
		img.Y = v
	}
	if v, ok, err := getInt(pm, "width"); err != nil {
		return err
	} else if ok {
		img.Width = v
	}
	if v, ok, err := getInt(pm, "height"); err != nil {
		return err
	} else if ok {
		img.Height = v
	}
	if v, ok := getString(pm, "folder"); ok {
		img.Folder = v
	}
	if v, ok := getString(pm, "name"); ok {
		img.Name = v
	}
	if v, ok, err := getInt(pm, "time"); err != nil {
		return err
	} else if ok {
		img.Time = v
	}
	if v, ok, err := getBool(pm, "wait"); err != nil {
		return err
	} else if ok {
		img.IsWait = v
	}
	if v, ok, err := getInt(pm, "zindex"); err != nil {
		return err
	} else if ok {
		img.ZIndex = v
	}
	if v, ok := getString(pm, "depth"); ok {
		img.Depth = v
	}
	if v, ok, err := getBool(pm, "refrect"); err != nil {
		return err
	} else if ok {
		img.Reflect = v
	}
	if v, ok := getString(pm, "pos"); ok {
		img.Pos = v
	}
	if v, ok, err := getBool(pm, "animimg"); err != nil {
		return err
	} else if ok {
		img.AnimImg = v
	}
	loaded, err := loadImage(r, pm["folder"], pm["storage"])
	if err != nil {
		return err
	}
	img.Image = loaded
	imgs = append(imgs, img)
	return nil
}

// handleFree releases a single named object — a [ptext] area or a [image]
// — matching Tyrano's [free name=...]. If the freed ptext was the
// character name-plate, that registration (see [chara_config]) is cleared
// too.
func handleFree(ctx *tagCtx) error {
	name := ctx.tag.Pm["name"]
	if _, ok := ptexts[name]; ok {
		delete(ptexts, name)
		if charaNamePText == name {
			charaNamePText = ""
		}
	}
	removeImageByName(name)
	return nil
}

func removeImageByName(name string) {
	if name == "" {
		return
	}
	var remaining []*kag3.Image
	for _, img := range imgs {
		if img.Name != name {
			remaining = append(remaining, img)
		}
	}
	imgs = remaining
}

// handleFreeImage clears every [image] on layer=, or every image if layer=
// is omitted.
func handleFreeImage(ctx *tagCtx) error {
	layer := ctx.tag.Pm["layer"]
	if layer == "" {
		imgs = nil
		return nil
	}
	var remaining []*kag3.Image
	for _, img := range imgs {
		if img.Layer != layer {
			remaining = append(remaining, img)
		}
	}
	imgs = remaining
	return nil
}

// handleBackLay snapshots the current foreground layer content (images and
// ptexts) into a "back page" buffer that [trans] later swaps in. This is a
// simplified stand-in for Tyrano's fore/back page double-buffering: instead
// of threading a Page filter through every render path, backlay/trans just
// hold and swap a full copy, which is enough to support "stage the next
// layout, then cut to it" scripts without touching the common rendering
// path at all.
func handleBackLay(ctx *tagCtx) error {
	backImgs = append([]*kag3.Image(nil), imgs...)
	backPtexts = make(map[string]*kag3.PText, len(ptexts))
	for k, v := range ptexts {
		backPtexts[k] = v
	}
	return nil
}

// handleTrans reveals whatever [backlay] staged. It's an instant cut, not
// an animated crossfade — building a real per-image-set transition would
// mean duplicating the background transition machinery for an arbitrary
// collection of images, which is out of scope here. A no-op if nothing was
// staged.
func handleTrans(ctx *tagCtx) error {
	if backImgs == nil && backPtexts == nil {
		return nil
	}
	imgs = backImgs
	if backPtexts != nil {
		ptexts = backPtexts
	}
	backImgs = nil
	backPtexts = nil
	return nil
}
