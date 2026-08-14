package ebitengine

import (
	"fmt"
	"net/url"
)

// launchURL actually opens rawURL in the OS's default handler — the
// platform-specific half, one function per GOOS (openurl_windows.go,
// openurl_darwin.go, openurl_linux.go, openurl_js.go, and
// openurl_unsupported.go for everything else, e.g. Android/iOS). A package
// var rather than a direct call to openURLPlatform so tests can swap it out
// to verify openURL's validation/wiring without actually spawning a browser
// process.
var launchURL = openURLPlatform

// openURL is the shared entry point behind both [web url=] (handleWeb,
// tags_web.go) and window.open(url) (the VM's browser shim — see newVM,
// vm.go). Real Tyrano documents [web] as simply "opens the given URL in a
// browser" — nothing HTML/CSS-rendering about it, unlike [html]/[loadcss],
// so this doesn't need any DOM this engine doesn't have.
//
// Only http/https is ever handed to the OS's URL-open mechanism: a bare
// path, a file:// URL, or some other scheme could otherwise let a .ks file
// pass an arbitrary argument straight into whatever
// rundll32/open/xdg-open's own scheme dispatch does with it.
func openURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%q は不正なURLです: %w", rawURL, err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("%q はhttp/https以外のスキームなので開けません", rawURL)
	}
	return launchURL(rawURL)
}
