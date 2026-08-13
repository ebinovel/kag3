// Browser-persistent save/settings storage for the wasm build.
//
// Without this, nothing survives a page reload: on GOOS=js Go's os package
// writes to an in-memory filesystem that dies with the tab, and saveDir
// (save_slots.go) can't even get that far — os.UserConfigDir and
// os.UserHomeDir both fail with no $HOME, so every save errors out.
//
// # Why localStorage rather than OPFS
//
// OPFS is the more modern answer and holds far more data, but its
// *synchronous* API (createSyncAccessHandle) is only available inside a Web
// Worker. Ebitengine drives its wasm game loop on the main thread via
// requestAnimationFrame, where OPFS is Promise-only — and Go/wasm shares
// that single thread, so blocking a goroutine until a Promise resolves
// deadlocks: the JS event loop that would resolve it can't run. Every call
// site here is synchronous and shaped like os.ReadFile/os.Stat, and one of
// them (slotPickerRows, tags_uiscreens.go) re-reads every slot on *every
// frame* the save/load screen is open, so an async backend would need a
// whole cache-and-invalidate layer on top.
//
// Capacity works out: ~3KB of JSON plus a ~60KB PNG thumbnail per slot,
// over 7 slots (1..5 plus quicksave 0 and autosave -1) is ~450KB, ~600KB
// once the thumbnails are base64'd — comfortably inside localStorage's
// typical 5MB-per-origin budget.
//
// Revisit OPFS if that budget ever stops being enough, or if the game loop
// moves into a Worker (where the sync API becomes available and this whole
// tradeoff flips).
//
//go:build js && wasm

package ebitengine

import (
	"encoding/base64"
	"errors"
	"fmt"
	"syscall/js"
	"time"
)

// storageKeyPrefix namespaces every key by game title, mirroring the
// kag3/<title>/saves directory layout saveDir builds on other platforms
// (save_slots.go) — one origin can serve several kag3 games, and they must
// not share each other's slots.
func storageKeyPrefix(r *Renderer) string {
	name := "kag3"
	if r != nil && r.manager != nil && r.manager.Config != nil && r.manager.Config.Title != "" {
		name = r.manager.Config.Title
	}
	return "kag3:" + name + ":"
}

func slotKey(r *Renderer, slot int, ext string) string {
	return fmt.Sprintf("%sslot_%d.%s", storageKeyPrefix(r), slot, ext)
}

// savedAtKey holds an RFC3339 timestamp written alongside every slot_N.json.
// localStorage has no mtime, and the save picker's row text *is* the mtime
// (saveSlotInfo, tags_uiscreens.go), so it has to be recorded explicitly.
func savedAtKey(r *Renderer, slot int) string {
	return fmt.Sprintf("%sslot_%d.savedAt", storageKeyPrefix(r), slot)
}

func settingsKey(r *Renderer) string {
	return storageKeyPrefix(r) + "settings"
}

var errNoLocalStorage = errors.New("kag3: localStorage is unavailable")

// localStorage returns the Storage object, or an error where it isn't
// reachable at all — some browsers throw on access (rather than returning
// undefined) when cookies/site data are blocked, so the property read
// itself goes through jsTry.
func localStorage() (ls js.Value, err error) {
	err = jsTry(func() {
		ls = js.Global().Get("localStorage")
	})
	if err != nil {
		return js.Value{}, err
	}
	if !ls.Truthy() {
		return js.Value{}, errNoLocalStorage
	}
	return ls, nil
}

// jsTry runs fn, converting a thrown JS exception (which syscall/js
// surfaces as a panic carrying js.Error) into a returned error. Needed
// above all for setItem, which throws QuotaExceededError once the origin's
// storage budget is full — see storageSaveSlot's handling of that.
func jsTry(fn func()) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			if e, ok := rec.(js.Error); ok {
				err = e
				return
			}
			panic(rec)
		}
	}()
	fn()
	return nil
}

func storageGet(key string) (string, bool) {
	ls, err := localStorage()
	if err != nil {
		return "", false
	}
	var v js.Value
	if err := jsTry(func() { v = ls.Call("getItem", key) }); err != nil {
		return "", false
	}
	if !v.Truthy() {
		// Absent keys come back as null; "" is equally useless to every
		// caller here, so both are "no data".
		return "", false
	}
	return v.String(), true
}

func storageSet(key, value string) error {
	ls, err := localStorage()
	if err != nil {
		return err
	}
	return jsTry(func() { ls.Call("setItem", key, value) })
}

// storageSaveSlot implements slotStore.Save. JSON is stored as-is (it's
// valid UTF-8 by construction, and staying readable makes the DevTools
// Application panel actually useful); PNG thumbnails are base64'd, since
// localStorage only holds strings.
func storageSaveSlot(r *Renderer, slot int, ext string, data []byte) error {
	if ext == "png" {
		// A thumbnail is the one thing worth dropping when the origin's
		// storage budget runs out — the game state itself must still make
		// it in. saveSlot already treats thumbnail writes as best-effort
		// (it discards os.Create's error on the file path), so swallowing
		// the failure here matches, rather than weakening, existing
		// behavior.
		if err := storageSet(slotKey(r, slot, ext), base64.StdEncoding.EncodeToString(data)); err != nil {
			fmt.Printf("save slot %d thumbnail not stored: %v\n", slot, err)
		}
		return nil
	}
	if err := storageSet(slotKey(r, slot, ext), string(data)); err != nil {
		return err
	}
	if ext == "json" {
		// Best-effort: a slot whose timestamp didn't make it still loads
		// fine, it just shows no date in the picker.
		_ = storageSet(savedAtKey(r, slot), time.Now().Format(time.RFC3339))
	}
	return nil
}

// storageLoadSlot implements slotStore.Load. A missing key is an error, so
// callers distinguish "no save here" exactly as they do from os.ReadFile.
func storageLoadSlot(r *Renderer, slot int, ext string) ([]byte, error) {
	v, ok := storageGet(slotKey(r, slot, ext))
	if !ok {
		return nil, fmt.Errorf("kag3: no data for slot %d (%s)", slot, ext)
	}
	if ext == "png" {
		return base64.StdEncoding.DecodeString(v)
	}
	return []byte(v), nil
}

// storageSlotInfo implements slotStore.Info. Existence keys off the JSON
// payload (not the timestamp) so a slot written before its savedAt entry
// failed still reads as present; an unparseable/absent timestamp just
// yields the zero time, which the picker renders the same as any other
// missing metadata.
func storageSlotInfo(r *Renderer, slot int) (bool, time.Time) {
	if _, ok := storageGet(slotKey(r, slot, "json")); !ok {
		return false, time.Time{}
	}
	v, ok := storageGet(savedAtKey(r, slot))
	if !ok {
		return true, time.Time{}
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return true, time.Time{}
	}
	return true, t
}

func storageSaveSettings(r *Renderer, data []byte) error {
	return storageSet(settingsKey(r), string(data))
}

func storageLoadSettings(r *Renderer) ([]byte, error) {
	v, ok := storageGet(settingsKey(r))
	if !ok {
		return nil, errors.New("kag3: no settings stored yet")
	}
	return []byte(v), nil
}

func init() {
	slotStore.Save = storageSaveSlot
	slotStore.Load = storageLoadSlot
	slotStore.Info = storageSlotInfo
	settingsStore.Save = storageSaveSettings
	settingsStore.Load = storageLoadSettings
}
