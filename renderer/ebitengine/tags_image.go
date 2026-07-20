package ebitengine

func init() {
	register("image", handleImage)
}

func handleImage(ctx *tagCtx) error {
	return ctx.r.image(ctx.tag)
}
