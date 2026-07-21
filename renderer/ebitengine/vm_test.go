package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

func TestVMEvalBool(t *testing.T) {
	v := newVM()
	if _, err := v.Eval("f.hoge = 5"); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if !v.EvalBool("f.hoge > 3") {
		t.Error("expected f.hoge > 3 to be true")
	}
	if v.EvalBool("f.hoge > 10") {
		t.Error("expected f.hoge > 10 to be false")
	}
	if v.EvalBool("this is not valid js (") {
		t.Error("expected malformed expression to evaluate as false, not panic")
	}
}

func TestVMEvalString(t *testing.T) {
	v := newVM()
	if _, err := v.Eval(`f.name = "akane"`); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := v.EvalString("f.name"); got != "akane" {
		t.Errorf("EvalString(f.name) = %q, want %q", got, "akane")
	}
	if got := v.EvalString("f.missing"); got != "undefined" {
		t.Errorf("EvalString(f.missing) = %q, want %q", got, "undefined")
	}
}

func TestVMExpandParams(t *testing.T) {
	v := newVM()
	if _, err := v.Eval("f.hoge = 42"); err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	pm := map[string]string{
		"plain": "left",
		"expr":  "&f.hoge",
		"mp":    "%name|default_name",
	}
	out := v.expandParams(pm)
	if out["plain"] != "left" {
		t.Errorf("plain = %q, want unchanged %q", out["plain"], "left")
	}
	if out["expr"] != "42" {
		t.Errorf("expr = %q, want %q", out["expr"], "42")
	}
	if out["mp"] != "default_name" {
		t.Errorf("mp = %q, want default %q (no active mp frame)", out["mp"], "default_name")
	}
}

func TestVMPushMPFrame(t *testing.T) {
	v := newVM()
	if got := v.EvalString("mp.name"); got != "undefined" {
		t.Fatalf("mp.name before any frame = %q, want %q", got, "undefined")
	}

	popA := v.PushMPFrame(map[string]string{"name": "foo"})
	if got := v.EvalString("mp.name"); got != "foo" {
		t.Errorf("mp.name with frame A active = %q, want %q", got, "foo")
	}

	popB := v.PushMPFrame(map[string]string{"name": "bar"})
	if got := v.EvalString("mp.name"); got != "bar" {
		t.Errorf("mp.name with frame B active (nested) = %q, want %q", got, "bar")
	}

	popB()
	if got := v.EvalString("mp.name"); got != "foo" {
		t.Errorf("mp.name after popping frame B = %q, want restored %q", got, "foo")
	}

	popA()
	if got := v.EvalString("mp.name"); got != "undefined" {
		t.Errorf("mp.name after popping frame A = %q, want %q", got, "undefined")
	}
}

// TestTFSystemAndSFSystemPreinitialized reproduces the crash reported when
// pressing the back button in replay mode: example/resources/senarios/
// replay.ks unconditionally does `tf.system.flag_replay = false;` and
// config.ks does `tf.system.backlog.pop();`, both assuming tf.system (and
// its backlog array) already exist — real Tyrano creates them before any
// script runs. Without that, tf.system is undefined and the assignment
// throws "Cannot convert undefined or null to object".
func TestTFSystemAndSFSystemPreinitialized(t *testing.T) {
	v := newVM()
	if _, err := v.Eval("tf.system.flag_replay = false;"); err != nil {
		t.Errorf("tf.system.flag_replay assignment failed: %v", err)
	}
	if _, err := v.Eval("tf.system.backlog.pop();"); err != nil {
		t.Errorf("tf.system.backlog.pop() failed: %v", err)
	}
	if _, err := v.Eval("sf.system.foo = 1;"); err != nil {
		t.Errorf("sf.system.foo assignment failed: %v", err)
	}
}

// TestClearSFReinitializesSystemNamespace ensures [clearsysvar] doesn't
// reintroduce the same crash for any script that happens to touch
// sf.system afterward.
func TestClearSFReinitializesSystemNamespace(t *testing.T) {
	v := newVM()
	v.ClearSF()
	if _, err := v.Eval("sf.system.foo = 1;"); err != nil {
		t.Errorf("sf.system.foo assignment after ClearSF failed: %v", err)
	}
}

// TestRestoreSFReinitializesMissingSystemNamespace covers loading a save
// made before sf.system existed (or one where a script deleted it).
func TestRestoreSFReinitializesMissingSystemNamespace(t *testing.T) {
	v := newVM()
	v.RestoreSF(map[string]interface{}{"seen": true}) // no "system" key at all
	if _, err := v.Eval("sf.system.foo = 1;"); err != nil {
		t.Errorf("sf.system.foo assignment after RestoreSF (no system key) failed: %v", err)
	}
	if got := v.EvalString("sf.seen"); got != "true" {
		t.Errorf("sf.seen after RestoreSF = %q, want %q (existing keys must survive)", got, "true")
	}
}

// TestBrowserShimReproducesConfigKsCrash reproduces the crash reported when
// opening the config screen: config.ks's [iscript] does
// `$(".layer_camera").empty(); $("#bgmovie").remove();` — real Tyrano runs
// in a browser with jQuery loaded, kag3 doesn't have a DOM at all, so
// without a $ shim this throws "ReferenceError: $ is not defined" and kills
// the whole renderer.
func TestBrowserShimReproducesConfigKsCrash(t *testing.T) {
	v := newVM()
	if _, err := v.Eval(`$(".layer_camera").empty(); $("#bgmovie").remove();`); err != nil {
		t.Errorf("jQuery-style $(...) call failed: %v", err)
	}
	// scene1.ks's web-demo-link buttons call window.open(url) directly.
	if _, err := v.Eval(`window.open("http://tyrano.jp/home/example");`); err != nil {
		t.Errorf("window.open(...) call failed: %v", err)
	}
	// A longer jQuery chain, matching config.ks's volume-icon update code
	// (`$(".bgmvol_"+tf.current_bgm_vol).attr("src", "...")`), must also
	// not panic and must be chainable.
	if _, err := v.Eval(`$(".bgmvol_10").attr("src", "c_set.png").css("color", "red");`); err != nil {
		t.Errorf("chained jQuery-style call failed: %v", err)
	}
}

// TestSetConfigReproducesConfigKsCrash reproduces the crash reported right
// after the previous ($/window) one: config.ks's bootstrap [iscript] reads
// several TG.config.* fields — without SetConfig, TG.config is empty and
// the very first line (`TG.config.autoRecordLabel = "true";`) throws
// "ReferenceError: TG is not defined" if TG itself doesn't exist yet, or —
// once TG exists but is empty — the parseInt(TG.config.defaultBgmVolume)
// calls silently produce NaN, which would go on to crash a *different* tag
// (e.g. [bgmopt volume="&tf.current_bgm_vol"], since strconv.Atoi("NaN")
// errors and that error panics the whole coroutine — see execItem in
// macro.go). SetConfig must make both not happen.
func TestSetConfigReproducesConfigKsCrash(t *testing.T) {
	v := newVM()
	cfg := &kag3.Config{
		DefaultBgmVolume:     80,
		DefaultSeVolume:      70,
		ChSpeed:              30,
		AutoSpeed:            1300,
		UnReadTextSkip:       true,
		AlreadyReadTextColor: "0x87cefa",
		AutoRecordLabel:      false,
	}
	v.SetConfig(cfg)

	script := `
TG.config.autoRecordLabel = "true";
tf.current_bgm_vol = parseInt(TG.config.defaultBgmVolume);
tf.current_se_vol = parseInt(TG.config.defaultSeVolume);
tf.current_ch_speed = parseInt(TG.config.chSpeed);
tf.current_auto_speed = parseInt(TG.config.autoSpeed);
tf.text_skip = "ON";
if (TG.config.unReadTextSkip != "true") {
	tf.text_skip = "OFF";
}
tf.user_setting = TG.config.alreadyReadTextColor;
if (tf.user_setting != 'default') {
	TG.config.alreadyReadTextColor = 'default';
}
`
	if _, err := v.Eval(script); err != nil {
		t.Fatalf("config.ks-style bootstrap script failed: %v", err)
	}
	if got := v.EvalString("tf.current_bgm_vol"); got != "80" {
		t.Errorf("tf.current_bgm_vol = %q, want %q (not NaN)", got, "80")
	}
	if got := v.EvalString("tf.current_ch_speed"); got != "30" {
		t.Errorf("tf.current_ch_speed = %q, want %q (not NaN)", got, "30")
	}
	if got := v.EvalString("tf.text_skip"); got != "ON" {
		t.Errorf("tf.text_skip = %q, want %q (UnReadTextSkip=true)", got, "ON")
	}
}

// TestSetMenuHooksRoutesToRealSaveLoad covers TG.menu.doSave/loadGame/
// getSaveData, wired to this engine's actual saveSlot/loadSlot — real
// Tyrano's index (mp.index, 0-based) maps to this engine's 1-based slot
// numbers.
func TestSetMenuHooksRoutesToRealSaveLoad(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	lastSnapshot = nil // avoid the GPU-readback panic noted elsewhere in this suite
	// bg is a package-level var another test may have left with a Storage
	// path set; loadSlot's applySaveData would then try to reload it
	// through r.fses, which this test never sets up (nil fs.FS panics
	// rather than erroring), so start from a clean bg.
	bg = &kag3.Background{}

	r := newSaveTestRendererWithVars(t)
	r.manager.Config = &kag3.Config{ConfigSaveSlotNum: 3}
	r.vm.SetMenuHooks(r)

	if _, err := r.vm.Eval("TG.menu.doSave(1);"); err != nil { // index 1 -> slot 2
		t.Fatalf("TG.menu.doSave(1) failed: %v", err)
	}
	if !hasSaveSlot(r, 2) {
		t.Error("expected TG.menu.doSave(1) to have written slot 2 (0-based index + 1)")
	}

	got, err := r.vm.Eval("TG.menu.getSaveData().data.length;")
	if err != nil {
		t.Fatalf("TG.menu.getSaveData() failed: %v", err)
	}
	if got.ToInteger() != 3 {
		t.Errorf("getSaveData().data.length = %v, want 3 (ConfigSaveSlotNum)", got)
	}

	if _, err := r.vm.Eval("TG.menu.loadGame(1);"); err != nil {
		t.Fatalf("TG.menu.loadGame(1) failed: %v", err)
	}
	if !isJump {
		t.Error("expected TG.menu.loadGame(1) to have triggered a jump via loadSlot")
	}
}
