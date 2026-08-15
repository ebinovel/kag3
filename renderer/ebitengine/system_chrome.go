package ebitengine

// systemChromeReferenceScreenWidth is the ScreenWidth every bundled
// resources/system/images/* asset — and every pixel-tuned layout constant
// that positions/sizes it (the corner menu button in tags_sysdesign.go, the
// quick menu in quickmenu.go, the save/load slot picker and its BACK button
// in tags_uiscreens.go) — was actually tuned for: the たそがれ図書室
// example's 1920x1080 (×1.5 from an original 1280x720 layout — see each
// call site's own doc comment for that history).
//
// A game running at any other ScreenWidth (e.g.
// example_tyrano_official_backup's 1280x720 default) gets this shared
// system-chrome asset set scaled down proportionally via systemChromeScale
// below, rather than drawn at its literal pixel size — otherwise the exact
// same assets read visibly larger against a smaller canvas (found by
// comparing the corner menu button, then the quick menu it opens, against
// real TyranoScript's own rendering, which sizes this chrome relative to
// the screen rather than as a fixed pixel count). At 1920x1080
// this scale is exactly 1, so the たそがれ図書室 example's own look is
// completely unchanged.
const systemChromeReferenceScreenWidth = 1920

// systemChromeScale is the factor every "system chrome" screen (the corner
// menu button, the quick menu it opens, the save/load slot picker and its
// shared BACK button) scales its bundled resources/system/images/* assets
// and their pixel-tuned layout constants by — see
// systemChromeReferenceScreenWidth's doc comment for why this exists.
func systemChromeScale(r *Renderer) float64 {
	return float64(r.manager.Config.ScreenWidth) / systemChromeReferenceScreenWidth
}
