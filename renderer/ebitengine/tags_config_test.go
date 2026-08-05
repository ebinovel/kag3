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
