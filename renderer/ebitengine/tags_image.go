package ebitengine

import "github.com/ebinovel/kag3"

func init() {
	register("image", handleImage)
	register("free", handleFree)
	register("freeimage", handleFreeImage)
	register("backlay", handleBackLay)
	register("trans", handleTrans)
}

func handleImage(ctx *tagCtx) error {
	return ctx.r.image(ctx.tag)
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
