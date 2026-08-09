package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestConfigSaveLoadRoundTrip is the deliverable for config.ks's new
// persistence: [configsave] writes tf's current contents to settings.json
// (under saveDir), and [configload] on a fresh tf brings them back.
func TestConfigSaveLoadRoundTrip(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "iscript", Body: "tf.set_speed_idx = 3; tf.set_skip_mode = 'all';"}, &i, 0); err != nil {
		t.Fatalf("iscript: %v", err)
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "configsave"}, &i, 0); err != nil {
		t.Fatalf("configsave: %v", err)
	}

	r2 := newTestRenderer()
	if err := dispatchTag(r2, fakeYield(), kag3.TagObject{Name: "configload"}, &i, 0); err != nil {
		t.Fatalf("configload: %v", err)
	}
	if got := r2.vm.EvalString("tf.set_speed_idx"); got != "3" {
		t.Errorf("tf.set_speed_idx after configload = %q, want %q", got, "3")
	}
	if got := r2.vm.EvalString("tf.set_skip_mode"); got != "all" {
		t.Errorf("tf.set_skip_mode after configload = %q, want %q", got, "all")
	}
}

// TestConfigLoadMissingFileIsNoop covers a fresh install: no settings.json
// exists yet, so [configload] must leave tf's already-set values alone
// rather than erroring or clobbering them (see saveSlot/loadSlot's own
// best-effort tolerance elsewhere in this package).
func TestConfigLoadMissingFileIsNoop(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "iscript", Body: "tf.set_speed_idx = 2;"}, &i, 0); err != nil {
		t.Fatalf("iscript: %v", err)
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "configload"}, &i, 0); err != nil {
		t.Fatalf("configload on missing file returned an error: %v", err)
	}
	if got := r.vm.EvalString("tf.set_speed_idx"); got != "2" {
		t.Errorf("tf.set_speed_idx after configload(missing) = %q, want unchanged %q", got, "2")
	}
}

// TestConfigStorageOverrideUsedInsteadOfFile covers the embedding-app
// override hook (e.g. example/mobile's android-only DataStore bridge — see
// ConfigStorage's own doc comment): when both fields are set, [configsave]/
// [configload] must route through them instead of settingsPath's raw
// os.WriteFile/os.ReadFile, and must not touch the filesystem at all.
func TestConfigStorageOverrideUsedInsteadOfFile(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	var stored []byte
	saveCalled, loadCalled := false, false
	ConfigStorage.Save = func(data []byte) error {
		saveCalled = true
		stored = append([]byte(nil), data...)
		return nil
	}
	ConfigStorage.Load = func() ([]byte, error) {
		loadCalled = true
		return stored, nil
	}
	defer func() { ConfigStorage.Save, ConfigStorage.Load = nil, nil }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "iscript", Body: "tf.set_speed_idx = 3;"}, &i, 0); err != nil {
		t.Fatalf("iscript: %v", err)
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "configsave"}, &i, 0); err != nil {
		t.Fatalf("configsave: %v", err)
	}
	if !saveCalled {
		t.Error("expected [configsave] to call ConfigStorage.Save instead of writing a file")
	}
	if len(stored) == 0 {
		t.Fatal("ConfigStorage.Save was never given any data")
	}

	r2 := newTestRenderer()
	if err := dispatchTag(r2, fakeYield(), kag3.TagObject{Name: "configload"}, &i, 0); err != nil {
		t.Fatalf("configload: %v", err)
	}
	if !loadCalled {
		t.Error("expected [configload] to call ConfigStorage.Load instead of reading a file")
	}
	if got := r2.vm.EvalString("tf.set_speed_idx"); got != "3" {
		t.Errorf("tf.set_speed_idx after configload = %q, want %q", got, "3")
	}
}

// TestMergeTFPreservesExistingKeys is config.ks's own reason for needing a
// merge rather than ExportF/RestoreF's replace-wholesale semantics: by the
// time [configload] runs, tf already holds unrelated working variables
// (here standing in for things like config.ks's own tf.img_path) that a
// settings-only merge must not discard.
func TestMergeTFPreservesExistingKeys(t *testing.T) {
	v := newVM()
	v.tf.Set("unrelated_var", "keep-me")
	v.MergeTF(map[string]interface{}{"set_speed_idx": float64(4)})

	if got := v.EvalString("tf.unrelated_var"); got != "keep-me" {
		t.Errorf("tf.unrelated_var after MergeTF = %q, want %q (MergeTF must not replace tf wholesale)", got, "keep-me")
	}
	if got := v.EvalString("tf.set_speed_idx"); got != "4" {
		t.Errorf("tf.set_speed_idx after MergeTF = %q, want %q", got, "4")
	}
}

// TestMsgBoxOpacityUpdatesFillAlpha is the regression test for
// [msgbox_opacity]: the settings screen's "メッセージ欄の不透明度" slider has
// no existing tag to reach messageBoxFillColorNormal (draw_messagebox.go)
// with, so this is a new dedicated one.
func TestMsgBoxOpacityUpdatesFillAlpha(t *testing.T) {
	orig := messageBoxFillColorNormal
	defer func() { messageBoxFillColorNormal = orig }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "msgbox_opacity", Pm: map[string]string{"value": "50"}}, &i, 0); err != nil {
		t.Fatalf("msgbox_opacity: %v", err)
	}
	if got, want := messageBoxFillColorNormal.A, uint8(50*255/100); got != want {
		t.Errorf("messageBoxFillColorNormal.A = %d, want %d", got, want)
	}
}

// TestUnreadSkipConfigSetsVar covers [unreadskip_config mode=], which drives
// the same unreadSkipEnabled var the operation row's own 既読SKIP label
// already reads (tags_oprow.go) — this test is the settings-screen side of
// that shared state.
func TestUnreadSkipConfigSetsVar(t *testing.T) {
	orig := unreadSkipEnabled
	defer func() { unreadSkipEnabled = orig }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "unreadskip_config", Pm: map[string]string{"mode": "read_only"}}, &i, 0); err != nil {
		t.Fatalf("unreadskip_config(read_only): %v", err)
	}
	if !unreadSkipEnabled {
		t.Error("unreadSkipEnabled = false after mode=read_only, want true")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "unreadskip_config", Pm: map[string]string{"mode": "all"}}, &i, 0); err != nil {
		t.Fatalf("unreadskip_config(all): %v", err)
	}
	if unreadSkipEnabled {
		t.Error("unreadSkipEnabled = true after mode=all, want false")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "unreadskip_config", Pm: map[string]string{"mode": "bogus"}}, &i, 0); err == nil {
		t.Error("unreadskip_config(mode=bogus) returned nil error, want an error for an unsupported value")
	}
}

// TestConfigRecordLabelSkipTogglesUnreadSkipEnabled covers real
// TyranoScript's own [config_record_label skip=] spelling — the inverse of
// [unreadskip_config]'s mode=: skip="true" means unread text CAN be skipped
// (unrestricted), so unreadSkipEnabled must go false; skip="false" restricts
// skip to already-read text, so unreadSkipEnabled must go true.
func TestConfigRecordLabelSkipTogglesUnreadSkipEnabled(t *testing.T) {
	orig := unreadSkipEnabled
	defer func() { unreadSkipEnabled = orig }()

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "config_record_label", Pm: map[string]string{"skip": "false"}}, &i, 0); err != nil {
		t.Fatalf("config_record_label(skip=false): %v", err)
	}
	if !unreadSkipEnabled {
		t.Error("unreadSkipEnabled = false after skip=false, want true (unread skip disabled = restricted to read_only)")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "config_record_label", Pm: map[string]string{"skip": "true"}}, &i, 0); err != nil {
		t.Fatalf("config_record_label(skip=true): %v", err)
	}
	if unreadSkipEnabled {
		t.Error("unreadSkipEnabled = true after skip=true, want false (unread skip enabled = unrestricted)")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "config_record_label", Pm: map[string]string{"skip": "bogus"}}, &i, 0); err == nil {
		t.Error("config_record_label(skip=bogus) returned nil error, want an error for a non-bool value")
	}
}

// TestConfigRecordLabelColorSetsAlreadyReadTextColor covers the color=
// attribute, and that omitting it (a later [config_record_label] call that
// only touches skip=) leaves a previously-set color untouched rather than
// clearing it back to no tint — matching the tag reference's "blank means
// unset" default, not "blank means clear".
func TestConfigRecordLabelColorSetsAlreadyReadTextColor(t *testing.T) {
	orig := alreadyReadTextColor
	defer func() { alreadyReadTextColor = orig }()
	alreadyReadTextColor = nil

	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "config_record_label", Pm: map[string]string{"color": "0xFF00FF"}}, &i, 0); err != nil {
		t.Fatalf("config_record_label(color=0xFF00FF): %v", err)
	}
	if alreadyReadTextColor == nil || alreadyReadTextColor.R != 0xFF || alreadyReadTextColor.G != 0x00 || alreadyReadTextColor.B != 0xFF {
		t.Fatalf("alreadyReadTextColor = %+v, want R=0xFF G=0x00 B=0xFF", alreadyReadTextColor)
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "config_record_label", Pm: map[string]string{"skip": "true"}}, &i, 0); err != nil {
		t.Fatalf("config_record_label(skip=true, no color): %v", err)
	}
	if alreadyReadTextColor == nil || alreadyReadTextColor.R != 0xFF {
		t.Errorf("alreadyReadTextColor changed after a color-less call, want it left untouched: %+v", alreadyReadTextColor)
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "config_record_label", Pm: map[string]string{"color": "not-a-color"}}, &i, 0); err == nil {
		t.Error("config_record_label(color=not-a-color) returned nil error, want an error for a malformed color code")
	}
}
