package ebitengine

import "fmt"

func init() {
	register("web", handleWeb)
}

// handleWeb implements [web url=]: opens url in the OS's default browser
// via the shared openURL (openurl.go) — the same path window.open(url)
// inside an [iscript] block goes through (see newVM, vm.go). Real Tyrano's
// own docs note this needs to run as a direct result of user interaction
// (a button's exp=) or the browser may block it, the same caveat autoplay
// audio has elsewhere in this engine; kag3 doesn't special-case that here,
// it just tries to open the URL either way.
//
// url= missing entirely is a script-authoring mistake (nothing to open) and
// returns a hard error, matching [movie]'s own storage= requirement. A
// rejected scheme or a failed launch, by contrast, doesn't affect story
// state at all — logged and swallowed rather than crashing the coroutine.
func handleWeb(ctx *tagCtx) error {
	rawURL, ok := getString(ctx.tag.Pm, "url")
	if !ok || rawURL == "" {
		return fmt.Errorf("[web] には url= が必要です")
	}
	if err := openURL(rawURL); err != nil {
		fmt.Println("[web]:", err)
	}
	return nil
}
