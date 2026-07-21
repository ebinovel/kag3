package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

// newSaveTestRendererWithVars is newTestRenderer() plus a couple of f/sf
// variables set through the VM, mirroring what a real [iscript] would leave
// behind before a save.
func newSaveTestRendererWithVars(t *testing.T) *Renderer {
	r := newTestRenderer()
	r.currentStorage = "scene1.ks"
	if _, err := r.vm.Eval("f.hoge = 5; sf.seen = true;"); err != nil {
		t.Fatalf("seeding f/sf failed: %v", err)
	}
	return r
}

func TestSaveSlotRoundTrip(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	defer delete(charas, "akane")

	r := newSaveTestRendererWithVars(t)
	r.callStack = []callFrame{{Storage: "sub.ks", Index: 3}}
	// Registered in charas (same-process load), like a real [chara_new]
	// would leave it — reconcileViewCharas's character-reconstruction path
	// (fresh-process load, no prior registration) is covered separately by
	// TestApplySaveDataReconstructsCharaAfterFreshProcess.
	charas["akane"] = &kag3.Character{Name: "akane"}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 10, Top: 20}}
	bg.Time, bg.Method, bg.Position = 1234, "slide", "left"
	currentScriptIndex = 42

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Clobber everything the save should restore, so the round trip is
	// actually observed rather than coincidentally already correct.
	r.callStack = nil
	viewCharas = nil
	bg.Time, bg.Method, bg.Position = 0, "", ""
	r.vm.ClearF()
	r.vm.ClearSF()
	jumpIndex, isJump = 0, false

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	if len(r.callStack) != 1 || r.callStack[0] != (callFrame{Storage: "sub.ks", Index: 3}) {
		t.Errorf("callStack after load = %+v, want [{sub.ks 3}]", r.callStack)
	}
	if len(viewCharas) != 1 || viewCharas[0].Name != "akane" || viewCharas[0].Left != 10 {
		t.Errorf("viewCharas after load = %+v", viewCharas)
	}
	if bg.Time != 1234 || bg.Method != "slide" || bg.Position != "left" {
		t.Errorf("bg after load = %+v, want Time=1234 Method=slide Position=left", bg)
	}
	if got := r.vm.EvalString("f.hoge"); got != "5" {
		t.Errorf("f.hoge after load = %q, want %q", got, "5")
	}
	if got := r.vm.EvalString("sf.seen"); got != "true" {
		t.Errorf("sf.seen after load = %q, want %q", got, "true")
	}
	if !isJump || jumpIndex != 42 {
		t.Errorf("isJump/jumpIndex after load = %v/%d, want true/42", isJump, jumpIndex)
	}
}

func TestLoadSlotMissingFileReturnsError(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	if err := r.loadSlot(99); err == nil {
		t.Fatal("expected an error loading a slot that was never saved")
	}
}

func TestCheckpointRollback(t *testing.T) {
	checkpointData = nil
	r := newSaveTestRendererWithVars(t)
	r.callStack = []callFrame{{Storage: "a.ks", Index: 1}}

	tag := kag3.TagObject{Name: "checkpoint"}
	// i (not a separately-set currentScriptIndex) is what dispatchTag uses
	// to update currentScriptIndex for a depth-0 dispatch — see dispatch.go.
	i := 7
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("checkpoint dispatch error: %v", err)
	}
	if checkpointData == nil {
		t.Fatal("expected checkpointData to be set")
	}

	// Diverge state after the checkpoint...
	r.callStack = nil
	jumpIndex, isJump = 0, false

	rollback := kag3.TagObject{Name: "rollback"}
	if err := dispatchTag(r, fakeYield(), rollback, &i, 0); err != nil {
		t.Fatalf("rollback dispatch error: %v", err)
	}
	if len(r.callStack) != 1 || r.callStack[0].Index != 1 {
		t.Errorf("callStack after rollback = %+v, want restored from checkpoint", r.callStack)
	}
	if !isJump || jumpIndex != 7 {
		t.Errorf("isJump/jumpIndex after rollback = %v/%d, want true/7", isJump, jumpIndex)
	}

	clear := kag3.TagObject{Name: "clear_checkpoint"}
	if err := dispatchTag(r, fakeYield(), clear, &i, 0); err != nil {
		t.Fatalf("clear_checkpoint dispatch error: %v", err)
	}
	if checkpointData != nil {
		t.Error("expected clear_checkpoint to clear checkpointData")
	}
}

func TestRollbackWithNoCheckpointIsNoop(t *testing.T) {
	checkpointData = nil
	r := newTestRenderer()
	r.callStack = []callFrame{{Storage: "x.ks", Index: 9}}
	tag := kag3.TagObject{Name: "rollback"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("rollback dispatch error: %v", err)
	}
	if len(r.callStack) != 1 || r.callStack[0].Index != 9 {
		t.Errorf("callStack changed by a no-checkpoint rollback: %+v", r.callStack)
	}
}

func TestHandleDialogJumpsToTargetOnOK(t *testing.T) {
	activeDialog = nil
	r := newTestRenderer()
	r.labels = map[string]kag3.LabelInfo{
		"yes": {Index: 10},
		"no":  {Index: 20},
	}
	tag := kag3.TagObject{Name: "dialog", Pm: map[string]string{
		"text": "本当に終了しますか？", "target": "*yes", "false_target": "*no",
	}}
	i := 5
	y := func() bool {
		if activeDialog != nil && activeDialog.Result == 0 {
			activeDialog.Result = 1 // simulate an OK click
		}
		return true
	}
	if err := dispatchTag(r, y, tag, &i, 0); err != nil {
		t.Fatalf("dialog dispatch error: %v", err)
	}
	if i != 10 {
		t.Errorf("i after OK = %d, want 10 (target)", i)
	}
	if activeDialog != nil {
		t.Error("expected activeDialog to be cleared after the dialog resolves")
	}
}

func TestHandleDialogJumpsToFalseTargetOnNG(t *testing.T) {
	activeDialog = nil
	r := newTestRenderer()
	r.labels = map[string]kag3.LabelInfo{
		"yes": {Index: 10},
		"no":  {Index: 20},
	}
	tag := kag3.TagObject{Name: "dialog", Pm: map[string]string{
		"text": "本当に終了しますか？", "target": "*yes", "false_target": "*no",
	}}
	i := 5
	y := func() bool {
		if activeDialog != nil && activeDialog.Result == 0 {
			activeDialog.Result = 2 // simulate a Cancel click
		}
		return true
	}
	if err := dispatchTag(r, y, tag, &i, 0); err != nil {
		t.Fatalf("dialog dispatch error: %v", err)
	}
	if i != 20 {
		t.Errorf("i after NG = %d, want 20 (false_target)", i)
	}
}

func TestDialogConfigTagsUpdateLabels(t *testing.T) {
	dialogOKLabel, dialogNGLabel = "OK", "キャンセル"
	ok := kag3.TagObject{Name: "dialog_config_ok", Pm: map[string]string{"text": "はい"}}
	ng := kag3.TagObject{Name: "dialog_config_ng", Pm: map[string]string{"text": "いいえ"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), ok, &i, 0); err != nil {
		t.Fatalf("dialog_config_ok dispatch error: %v", err)
	}
	if err := dispatchTag(newTestRenderer(), fakeYield(), ng, &i, 0); err != nil {
		t.Fatalf("dialog_config_ng dispatch error: %v", err)
	}
	if dialogOKLabel != "はい" || dialogNGLabel != "いいえ" {
		t.Errorf("labels = %q/%q, want はい/いいえ", dialogOKLabel, dialogNGLabel)
	}
}

func TestKeyConfigAndCloseConfirmFlags(t *testing.T) {
	i := 0
	r := newTestRenderer()
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "stop_keyconfig"}, &i, 0); err != nil {
		t.Fatalf("stop_keyconfig error: %v", err)
	}
	if keyConfigEnabled {
		t.Error("expected keyConfigEnabled = false after stop_keyconfig")
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "start_keyconfig"}, &i, 0); err != nil {
		t.Fatalf("start_keyconfig error: %v", err)
	}
	if !keyConfigEnabled {
		t.Error("expected keyConfigEnabled = true after start_keyconfig")
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "closeconfirm_on"}, &i, 0); err != nil {
		t.Fatalf("closeconfirm_on error: %v", err)
	}
	if !closeConfirmEnabled {
		t.Error("expected closeConfirmEnabled = true after closeconfirm_on")
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "closeconfirm_off"}, &i, 0); err != nil {
		t.Fatalf("closeconfirm_off error: %v", err)
	}
	if closeConfirmEnabled {
		t.Error("expected closeConfirmEnabled = false after closeconfirm_off")
	}
}

func TestHandleSaveSnapNoopWithoutRenderBuffer(t *testing.T) {
	renderBuffer = nil
	lastSnapshot = nil
	tag := kag3.TagObject{Name: "savesnap"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("savesnap dispatch error: %v", err)
	}
	if lastSnapshot != nil {
		t.Error("expected lastSnapshot to stay nil when renderBuffer hasn't been drawn yet")
	}
}

func TestAutoSaveAutoLoadUseReservedSlot(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	tag := kag3.TagObject{Name: "autosave"}
	i := 3
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("autosave dispatch error: %v", err)
	}

	jumpIndex, isJump = 0, false
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "autoload"}, &i, 0); err != nil {
		t.Fatalf("autoload dispatch error: %v", err)
	}
	if !isJump || jumpIndex != 3 {
		t.Errorf("isJump/jumpIndex after autoload = %v/%d, want true/3", isJump, jumpIndex)
	}
	// Confirm it didn't collide with the manual slot.
	if err := r.loadSlot(manualSaveSlot); err == nil {
		t.Error("expected no manual-slot save to exist yet")
	}
}

// TestApplySaveDataReconstructsCharaAfterFreshProcess is the crash this
// whole fix is for: loading a save right after starting a brand-new process
// (charas empty, none of the scenario's [chara_new] calls have run yet)
// must re-register the character from CharaStorage instead of leaving a
// dangling viewCharas entry for drawScene to crash on.
func TestApplySaveDataReconstructsCharaAfterFreshProcess(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()
	defer delete(charas, "akane")
	defer func() { viewCharas = nil }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{"akane.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	charas["akane"] = &kag3.Character{Name: "akane", Storage: "akane.png", Image: newTestImage(1, 1)}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 10, Top: 20}}

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate a fresh process: charas starts empty, nothing has run
	// [chara_new] yet.
	delete(charas, "akane")
	viewCharas = nil

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}

	got, ok := charas["akane"]
	if !ok || got.Image == nil {
		t.Fatalf("charas[akane] after load = %+v, want reconstructed with a non-nil Image", got)
	}
	if got.Storage != "akane.png" {
		t.Errorf("charas[akane].Storage = %q, want %q", got.Storage, "akane.png")
	}
	if len(viewCharas) != 1 || viewCharas[0].Name != "akane" {
		t.Errorf("viewCharas after load = %+v, want akane kept (not dropped)", viewCharas)
	}
}

// TestReconcileViewCharasDropsUnrestorableCharaWithoutPanicking covers the
// fallback when a character can't be reconstructed at all (no recorded
// path, or the file no longer exists) — it must be silently dropped from
// viewCharas, never left dangling for drawScene to panic on.
func TestReconcileViewCharasDropsUnrestorableCharaWithoutPanicking(t *testing.T) {
	delete(charas, "ghost")
	r := newTestRenderer()
	r.fses = map[string]fs.FS{"images": fstest.MapFS{}}
	restored := []*kag3.CharaShow{{Name: "ghost", Left: 5}}

	got := reconcileViewCharas(r, restored, map[string]string{}) // no CharaStorage entry at all
	if len(got) != 0 {
		t.Errorf("reconcileViewCharas with no storage path = %+v, want dropped (empty)", got)
	}

	got = reconcileViewCharas(r, restored, map[string]string{"ghost": "missing.png"}) // path given but file absent
	if len(got) != 0 {
		t.Errorf("reconcileViewCharas with an unreadable path = %+v, want dropped (empty)", got)
	}
	if _, ok := charas["ghost"]; ok {
		t.Error("expected charas[ghost] to stay unregistered after a failed reconstruction")
	}
}

// TestApplySaveDataReconstructsBackgroundAfterFreshProcess mirrors the
// character fix for bg: a fresh process's bg.Image starts as NewRenderer's
// solid-black placeholder, and load should replace it with the actual saved
// background rather than leaving that placeholder in place.
func TestApplySaveDataReconstructsBackgroundAfterFreshProcess(t *testing.T) {
	saveBaseDirOverride = t.TempDir()
	defer func() { saveBaseDirOverride = "" }()

	r := newSaveTestRendererWithVars(t)
	r.fses = map[string]fs.FS{"images": fstest.MapFS{"bg.png": &fstest.MapFile{Data: tinyPNG(t)}}}
	bg = &kag3.Background{Time: 3000, Method: "crossfade", Storage: "bg.png", Image: newTestImage(1, 1)}

	if err := r.saveSlot(manualSaveSlot); err != nil {
		t.Fatalf("saveSlot error: %v", err)
	}

	// Simulate a fresh process: NewRenderer's default solid-black fill, no
	// Storage tracked yet.
	placeholder := newTestImage(1, 1)
	bg = &kag3.Background{Image: placeholder}

	if err := r.loadSlot(manualSaveSlot); err != nil {
		t.Fatalf("loadSlot error: %v", err)
	}
	if bg.Storage != "bg.png" {
		t.Errorf("bg.Storage after load = %q, want %q", bg.Storage, "bg.png")
	}
	if bg.Image == nil || bg.Image == placeholder {
		t.Error("expected bg.Image to be replaced with the reloaded background, not left as the placeholder")
	}
	if bg.Method != "crossfade" {
		t.Errorf("bg.Method after load = %q, want %q", bg.Method, "crossfade")
	}
}
