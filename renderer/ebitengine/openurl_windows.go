//go:build windows

package ebitengine

import "os/exec"

// openURLPlatform opens rawURL in the OS's default browser via the same
// rundll32 FileProtocolHandler trick every other Go "open a URL" helper on
// Windows uses — there's no ShellExecute equivalent in the standard
// library.
func openURLPlatform(rawURL string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
}
