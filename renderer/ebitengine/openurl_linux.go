//go:build linux

package ebitengine

import "os/exec"

// openURLPlatform opens rawURL via xdg-open, the freedesktop.org standard
// for "hand this to whatever the desktop environment considers the default
// handler" — matches example/linux/build-linux.sh's own .desktop-based
// packaging, which targets the same freedesktop.org conventions.
func openURLPlatform(rawURL string) error {
	return exec.Command("xdg-open", rawURL).Start()
}
