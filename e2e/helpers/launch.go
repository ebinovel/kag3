//go:build windows

// Package helpers wraps e2e/driver with kag3-specific conveniences: launching
// the game with a sandboxed save dir, converting kag3's 1280x720 logical tag
// coordinates (button x=/y=) to driver.Session click coordinates, waiting for
// animation to settle, and reading save slot JSON directly.
package helpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebinovel/kag3/e2e/driver"
)

// ExamplePath is where the built example binary is expected. Build it with:
//
//	go build -o e2e/testdata/kag3example.exe ./example
//
// before running e2e tests — see e2e/README.md.
const ExamplePath = "testdata/kag3example.exe"

// Game bundles a driver.Session with the sandboxed save directory
// LaunchGame pointed KAG3_SAVE_DIR at, so tests can read save slot files
// back out (see ReadSaveSlot) without re-deriving the path.
type Game struct {
	*driver.Session
	SaveDir string
}

// LaunchGame builds nothing itself (see ExamplePath) — it starts the
// already-built example binary with KAG3_SAVE_DIR pointed at a fresh
// t.TempDir() (so this test's saves never touch the real player's
// %AppData% profile and never collide with another test's) and
// KAG3_E2E_FAST=1 (see tags_message.go's textNoWait hook — skips
// glyph-by-glyph text reveal, which otherwise dominates E2E wall time).
// The session and process are registered with t.Cleanup.
func LaunchGame(t *testing.T) *Game {
	t.Helper()

	exePath, err := filepath.Abs(ExamplePath)
	if err != nil {
		t.Fatalf("resolving %s: %v", ExamplePath, err)
	}
	if _, err := os.Stat(exePath); err != nil {
		t.Fatalf("%s not found — build it first: go build -o %s ./example (%v)", exePath, exePath, err)
	}

	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(),
		"KAG3_SAVE_DIR="+saveDir,
		"KAG3_E2E_FAST=1",
	)

	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() {
		if err := sess.Close(); err != nil {
			t.Logf("closing session: %v", err)
		}
	})

	g := &Game{Session: sess, SaveDir: saveDir}

	// The window can exist (per Win32) a moment before ebitengine has
	// actually drawn a first real frame (embedded FS decode, font load,
	// first.ks -> tyrano.ks -> title.ks jump chain all run before the
	// title screen settles) — wait for the screenshot to stabilize
	// before handing the session to the test.
	if err := WaitStable(g.Session, 10*time.Second); err != nil {
		t.Fatalf("waiting for title screen to settle: %v", err)
	}
	return g
}
