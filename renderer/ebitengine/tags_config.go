package ebitengine

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
)

func init() {
	register("configsave", handleConfigSave)
	register("configload", handleConfigLoad)
	register("msgbox_opacity", handleMsgBoxOpacity)
	register("unreadskip_config", handleUnreadSkipConfig)
	register("config_record_label", handleConfigRecordLabel)
}

// settingsPath resolves where [configsave]/[configload] persist the
// scenario's own settings variables — alongside save slots (saveDir,
// tags_save.go), not a new directory-resolution scheme, so KAG3_SAVE_DIR/
// saveBaseDirOverride sandbox this exactly like any other on-disk state a
// test or an external E2E harness needs to isolate. Only consulted when
// ConfigStorage isn't set — see its own doc comment.
func settingsPath(r *Renderer) (string, error) {
	dir, err := saveDir(r)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// ConfigStorage lets an embedding app override how [configsave]/
// [configload] persist settings, instead of the default raw
// os.WriteFile/os.ReadFile against settingsPath(). Both fields are
// optional; nil means "use the default file-based behavior". For a
// platform where a raw file isn't the natural fit (e.g. Android, where
// Preferences DataStore is — see example/mobile's own android-only storage
// bridge for a concrete implementation), an embedding app can set these
// instead; renderer/ebitengine itself stays storage-mechanism-agnostic
// since it's a shared package used by non-Android projects too.
var ConfigStorage struct {
	Save func(data []byte) error
	Load func() ([]byte, error)
}

// settingsStore is the in-package counterpart to ConfigStorage, consulted
// only when the exported hook above is unset (an embedding app's explicit
// choice always wins). It exists because ConfigStorage's signature carries
// no *Renderer, so an implementation can't namespace what it writes by
// Config.Title — fine for Android, whose bridge is per-app anyway, but not
// for storage_js.go, where every game served from one origin shares a
// single localStorage. Same rationale and lifecycle as slotStore
// (tags_save.go); nil means "use settingsPath and a real file".
var settingsStore struct {
	Save func(r *Renderer, data []byte) error
	Load func(r *Renderer) ([]byte, error)
}

// handleConfigSave persists every variable currently in the "tf" namespace
// (goja's tf object — see vm.go's ExportTF) as JSON. Deliberately generic:
// this tag has no idea what config.ks chose to name its own settings
// variables (tf.current_bgm_vol, tf.current_ch_speed, ...) — the script
// owns that vocabulary entirely, matching the rest of kag3's f/sf/tf model
// where Go never inspects variable names, only round-trips whatever's
// there.
//
// Every failure here is non-fatal (matches handleConfigLoad's own tolerant
// handling below) rather than returned as an error: initScript's tag loop
// (renderer.go) logs and skips past any non-nil tag error rather than
// panicking, but a config screen visit failing to persist should still just
// be logged as what it is — a missed preference write — rather than
// silently skipping whatever tag happens to come after [configsave] in the
// same script.
func handleConfigSave(ctx *tagCtx) error {
	r := ctx.r
	b, err := json.MarshalIndent(r.vm.ExportTF(), "", "  ")
	if err != nil {
		return nil
	}
	if ConfigStorage.Save != nil {
		if err := ConfigStorage.Save(b); err != nil {
			fmt.Printf("configsave failed: %v\n", err)
		}
		return nil
	}
	if settingsStore.Save != nil {
		if err := settingsStore.Save(r, b); err != nil {
			fmt.Printf("configsave failed: %v\n", err)
		}
		return nil
	}
	path, err := settingsPath(r)
	if err != nil {
		return nil
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fmt.Printf("configsave failed: %v\n", err)
	}
	return nil
}

// handleConfigLoad reads back what [configsave] wrote and merges it onto
// the current "tf" (VM.MergeTF, vm.go) — not a wholesale replace, since
// config.ks's own bootstrap [iscript] has already populated tf with
// unrelated working variables by the time this typically runs. A missing
// or corrupt settings blob is not an error (matches saveSlot/loadSlot's own
// best-effort tolerance elsewhere in this package): a fresh install simply
// has nothing to merge, and script-set TG.config-derived defaults stand.
func handleConfigLoad(ctx *tagCtx) error {
	r := ctx.r
	var b []byte
	if ConfigStorage.Load != nil {
		loaded, err := ConfigStorage.Load()
		if err != nil {
			return nil
		}
		b = loaded
	} else if settingsStore.Load != nil {
		loaded, err := settingsStore.Load(r)
		if err != nil {
			return nil
		}
		b = loaded
	} else {
		path, err := settingsPath(r)
		if err != nil {
			return nil
		}
		loaded, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		b = loaded
	}
	var vars map[string]interface{}
	if err := json.Unmarshal(b, &vars); err != nil {
		return nil
	}
	r.vm.MergeTF(vars)
	return nil
}

// handleMsgBoxOpacity implements [msgbox_opacity value="0-100"] — there is
// no existing tag reaching messageBoxFillColorNormal (draw_messagebox.go),
// the message box's own background fill alpha, as distinct from
// [position opacity=]'s whole-window fade alpha (kag3.TextPosition.Opacity)
// — so the settings screen needs this one small dedicated tag to expose it
// to script.
func handleMsgBoxOpacity(ctx *tagCtx) error {
	v, ok, err := getInt(ctx.tag.Pm, "value")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("[msgbox_opacity] requires value=")
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	messageBoxFillColorNormal.A = uint8(v * 255 / 100)
	return nil
}

// handleUnreadSkipConfig implements [unreadskip_config mode="read_only"|"all"]
// — the settings screen's "スキップ対象" row. Drives the existing
// unreadSkipEnabled (tags_oprow.go), the same var the operation row's own
// 既読SKIP label already reads for its highlight color, rather than
// introducing a second piece of state for the same concept.
func handleUnreadSkipConfig(ctx *tagCtx) error {
	mode, ok := getString(ctx.tag.Pm, "mode")
	if !ok {
		return fmt.Errorf("[unreadskip_config] requires mode=")
	}
	switch mode {
	case "read_only":
		unreadSkipEnabled = true
	case "all":
		unreadSkipEnabled = false
	default:
		return fmt.Errorf("[unreadskip_config] 未対応の値です mode=%q", mode)
	}
	return nil
}

// handleConfigRecordLabel implements TyranoScript's own
// [config_record_label skip="true"|"false" color="0xRRGGBB"] — real Tyrano's
// stock tag for this (tyrano.jp/tag), kept alongside kag3's own
// [unreadskip_config] rather than replacing it, since existing scripts (see
// test/config.ks) already call this one directly. skip is unreadSkipEnabled
// spelled the opposite way round: "未読スキップ有効" (skip=true) means skip
// is unrestricted, i.e. unreadSkipEnabled=false. color drives
// alreadyReadTextColor (tags_message.go), applied per-line by
// snapshotTextStyle (macro.go) — an empty/absent color leaves whatever tint
// was already set untouched, matching "blank means unset" in the tag
// reference rather than clearing it back to no tint.
func handleConfigRecordLabel(ctx *tagCtx) error {
	if skip, ok, err := getBool(ctx.tag.Pm, "skip"); err != nil {
		return err
	} else if ok {
		unreadSkipEnabled = !skip
	}
	if v, ok := getString(ctx.tag.Pm, "color"); ok && v != "" {
		r, g, b, err := parseColor(v)
		if err != nil {
			return err
		}
		alreadyReadTextColor = &color.RGBA{uint8(r), uint8(g), uint8(b), 0xff}
	}
	return nil
}
