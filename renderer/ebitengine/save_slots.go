package ebitengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// saveBaseDirOverride lets tests point saveDir at a temp directory instead
// of the real OS config dir.
var saveBaseDirOverride string

// SaveDirFunc, if set, is consulted before falling back to os.UserConfigDir
// (but after KAG3_SAVE_DIR/saveBaseDirOverride) — an embedding app can
// register this to supply its own platform-appropriate writable directory
// obtained some other way than the standard os package APIs (e.g.
// Android's Context.getFilesDir(), reached over JNI — see example/mobile's
// own android-only storage bridge for a concrete implementation;
// renderer/ebitengine is a shared package used by non-Android projects too,
// so it has no business knowing about JNI itself).
var SaveDirFunc func() (string, error)

// saveDir resolves the directory save/load slots live in. KAG3_SAVE_DIR, if
// set, wins outright and is used as-is (no kag3/<title>/saves suffix) — an
// external-process E2E harness (see e2e/) has no way to set the in-package
// saveBaseDirOverride var, so this is the only way it can sandbox a real
// build's save files away from the player's actual %AppData% profile.
// saveBaseDirOverride (for in-package Go tests), SaveDirFunc (for an
// embedding app), and the OS config dir are still checked, in that order,
// when KAG3_SAVE_DIR is unset.
func saveDir(r *Renderer) (string, error) {
	if v := os.Getenv("KAG3_SAVE_DIR"); v != "" {
		if err := os.MkdirAll(v, 0o755); err != nil {
			return "", err
		}
		return v, nil
	}
	base := saveBaseDirOverride
	if base == "" && SaveDirFunc != nil {
		b, err := SaveDirFunc()
		if err != nil {
			return "", err
		}
		base = b
	}
	if base == "" {
		b, err := os.UserConfigDir()
		if err != nil {
			// os.UserConfigDir() has no android case (unlike os.UserHomeDir,
			// which explicitly returns "/sdcard" there) and always errors on
			// Android — there's no $HOME. This is a last-resort fallback for
			// an embedding app that hasn't registered SaveDirFunc above; not
			// guaranteed writable without storage permissions this package
			// doesn't declare, but every caller of saveDir already tolerates
			// a failure gracefully either way (see handleConfigSave/
			// handleConfigLoad, tags_config.go).
			b, err = os.UserHomeDir()
			if err != nil {
				return "", err
			}
		}
		base = b
	}
	name := "kag3"
	if r.manager != nil && r.manager.Config != nil && r.manager.Config.Title != "" {
		name = r.manager.Config.Title
	}
	dir := filepath.Join(base, "kag3", name, "saves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func slotPath(dir string, slot int, ext string) string {
	return filepath.Join(dir, fmt.Sprintf("slot_%d.%s", slot, ext))
}

// slotStore, if set, replaces the raw os.WriteFile/os.ReadFile/os.Stat calls
// every save slot goes through (saveSlot/loadSlot here, plus
// loadSlotThumbnail/saveSlotInfo/saveSlotLastMessage in tags_uiscreens.go —
// those five are the entire persistent-slot I/O surface). All three fields
// nil, the default, keeps the on-disk behavior every non-wasm platform uses.
//
// Deliberately unexported, unlike SaveDirFunc/ConfigStorage: the only
// implementation is storage_js.go's, which lives in this same package
// because localStorage needs nothing but syscall/js (whereas Android's
// bridge has to live in the embedding app — it calls that app's own Java
// class over JNI). Nothing outside this package needs to swap slot storage
// yet; promote it to exported API if and when something does.
//
// Info is separate from Load rather than derived from it because the save
// picker's row text is the file's *mtime* (saveSlotInfo, tags_uiscreens.go)
// — a backend with no filesystem behind it has to record that timestamp
// itself.
//
// Every hook takes *Renderer so an implementation can namespace by
// Config.Title the same way saveDir's own directory layout does.
var slotStore struct {
	Save func(r *Renderer, slot int, ext string, data []byte) error
	Load func(r *Renderer, slot int, ext string) ([]byte, error)
	Info func(r *Renderer, slot int) (exists bool, modTime time.Time)
}

func (r *Renderer) saveSlot(slot int) error {
	b, err := json.MarshalIndent(r.buildSaveData(), "", "  ")
	if err != nil {
		return err
	}

	// Both branches below do the same three things — write the JSON, drop the
	// slot picker's now-stale thumbnail cache entry (tags_uiscreens.go), then
	// write the new thumbnail best-effort — against their respective backend.
	if slotStore.Save != nil {
		if err := slotStore.Save(r, slot, "json", b); err != nil {
			return err
		}
		delete(slotThumbnailCache, slot)
		if lastSnapshot != nil {
			// Thumbnail failures stay best-effort here exactly as they are
			// in the file branch below (the os.Create error is deliberately
			// dropped there) — a missing preview image must never turn into
			// a failed save.
			var buf bytes.Buffer
			if err := png.Encode(&buf, lastSnapshot); err == nil {
				_ = slotStore.Save(r, slot, "png", buf.Bytes())
			}
		}
		return nil
	}

	dir, err := saveDir(r)
	if err != nil {
		return err
	}
	if err := os.WriteFile(slotPath(dir, slot, "json"), b, 0o644); err != nil {
		return err
	}
	delete(slotThumbnailCache, slot)
	if lastSnapshot != nil {
		if f, err := os.Create(slotPath(dir, slot, "png")); err == nil {
			_ = png.Encode(f, lastSnapshot)
			f.Close()
		}
	}
	return nil
}

// readSlotFile reads one save slot's <ext> payload, through slotStore when
// an implementation is registered (storage_js.go) and off disk otherwise.
// Shared by loadSlot here and by the picker's own readers in
// tags_uiscreens.go so the "which backend?" decision lives in exactly one
// place. Note it never calls saveDir in the hook branch — on GOOS=js that
// would fail outright (no $HOME for os.UserConfigDir/os.UserHomeDir).
func readSlotFile(r *Renderer, slot int, ext string) ([]byte, error) {
	if slotStore.Load != nil {
		return slotStore.Load(r, slot, ext)
	}
	dir, err := saveDir(r)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(slotPath(dir, slot, ext))
}

func (r *Renderer) loadSlot(slot int) error {
	b, err := readSlotFile(r, slot, "json")
	if err != nil {
		return err
	}
	var data saveData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	return r.applySaveData(&data)
}
