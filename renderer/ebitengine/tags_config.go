package ebitengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func init() {
	register("configsave", handleConfigSave)
	register("configload", handleConfigLoad)
	register("msgbox_opacity", handleMsgBoxOpacity)
	register("unreadskip_config", handleUnreadSkipConfig)
}

// settingsPath resolves where [configsave]/[configload] persist the
// scenario's own settings variables — alongside save slots (saveDir,
// tags_save.go), not a new directory-resolution scheme, so KAG3_SAVE_DIR/
// saveBaseDirOverride sandbox this exactly like any other on-disk state a
// test or an external E2E harness needs to isolate.
func settingsPath(r *Renderer) (string, error) {
	dir, err := saveDir(r)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// handleConfigSave persists every variable currently in the "tf" namespace
// (goja's tf object — see vm.go's ExportTF) as JSON. Deliberately generic:
// this tag has no idea what config.ks chose to name its own settings
// variables (tf.current_bgm_vol, tf.current_ch_speed, ...) — the script
// owns that vocabulary entirely, matching the rest of kag3's f/sf/tf model
// where Go never inspects variable names, only round-trips whatever's
// there.
func handleConfigSave(ctx *tagCtx) error {
	r := ctx.r
	path, err := settingsPath(r)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(r.vm.ExportTF(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// handleConfigLoad reads back what [configsave] wrote and merges it onto
// the current "tf" (VM.MergeTF, vm.go) — not a wholesale replace, since
// config.ks's own bootstrap [iscript] has already populated tf with
// unrelated working variables by the time this typically runs. A missing
// or corrupt settings.json is not an error (matches saveSlot/loadSlot's own
// best-effort tolerance elsewhere in this package): a fresh install simply
// has nothing to merge, and script-set TG.config-derived defaults stand.
func handleConfigLoad(ctx *tagCtx) error {
	r := ctx.r
	path, err := settingsPath(r)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
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
