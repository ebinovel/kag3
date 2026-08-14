//go:build darwin

package ebitengine

import "os/exec"

// openURLPlatform opens rawURL via macOS's `open` command. GOOS=ios is a
// distinct build target from GOOS=darwin (since Go 1.16), so this file
// covers desktop macOS only — iOS falls through to openurl_unsupported.go.
func openURLPlatform(rawURL string) error {
	return exec.Command("open", rawURL).Start()
}
