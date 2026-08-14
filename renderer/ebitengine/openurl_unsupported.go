//go:build !windows && !darwin && !linux && !(js && wasm)

package ebitengine

import (
	"fmt"
	"os"
)

// openURLPlatform is the fallback for platforms with no known "open a URL
// in the default handler" mechanism from here — Android and iOS, chiefly.
// Logs and no-ops rather than erroring, the same "optional feature just
// isn't wired up on this platform" contract errVideosFSUnset ([movie],
// tags_movie.go) follows for a project that never registered "videos".
func openURLPlatform(rawURL string) error {
	fmt.Fprintln(os.Stderr, "[web]: このプラットフォームではURLを開けません:", rawURL)
	return nil
}
