package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

// TestDrawSceneButtonWithoutEnterImgDoesNotPanicOnHover reproduces a real
// crash: config.ks's volume/speed slider buttons (e.g.
// [button graphic="&tf.btn_path_off" ... ] with no enterimg= at all) have
// button.EnterImg == nil while button.Graphic is set. drawScene's old hover
// check only skipped drawing when *both* were nil, so as soon as the mouse
// hovered one of these buttons it tried buf.DrawImage(nil, ...) and
// panicked.
func TestDrawSceneButtonWithoutEnterImgDoesNotPanicOnHover(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	textPosition = &kag3.TextPosition{Visible: false}
	bg = &kag3.Background{}
	bg2 = &kag3.Background{}
	viewCharas = nil
	links = nil
	glinks = nil
	imgs = nil
	// A huge rect starting at the origin so it covers whatever position a
	// headless test environment reports for ebiten.CursorPosition() — the
	// point is exercising the hover branch, not any specific coordinate.
	buttons = []*kag3.Button{
		{Graphic: newTestImage(10, 10), EnterImg: nil, X: 0, Y: 0, Width: 100000, Height: 100000},
	}
	defer func() { buttons = nil }()

	buf := newTestImage(1280, 720)
	r.drawScene(buf) // must not panic
}

func resetConfirmDialogState() {
	activeDialog = nil
	viewCharas = nil
	backlog = nil
}

// TestConfirmGoToTitleOpensDialogInsteadOfActingImmediately covers real
// Tyrano's "タイトルに戻ります。よろしいですか？" confirmation: role="title"
// (and the quick-menu's "BACK TO TITLE") must not call goToTitle directly
// anymore — clicking it should only open a dialog.
func TestConfirmGoToTitleOpensDialogInsteadOfActingImmediately(t *testing.T) {
	resetConfirmDialogState()
	r := newTestRenderer()
	r.currentStorage = "scene1.ks"
	viewCharas = []*kag3.CharaShow{{Name: "akane"}} // something goToTitle would clear

	confirmGoToTitle(r)

	if activeDialog == nil {
		t.Fatal("expected confirmGoToTitle to open a dialog")
	}
	if activeDialog.OnConfirm == nil {
		t.Error("expected activeDialog.OnConfirm to be set (marks it as button-triggered — see anyModalActive)")
	}
	if r.currentStorage != "scene1.ks" || len(viewCharas) != 1 {
		t.Error("expected confirmGoToTitle to not touch renderer state yet — only OK should")
	}
}

// TestResolveButtonDialogRunsOnConfirmOnlyOnOK covers both outcomes of the
// confirm dialog: OK actually goes to the title, NG (cancel) leaves
// everything alone. Either way activeDialog must be cleared afterward.
func TestResolveButtonDialogRunsOnConfirmOnlyOnOK(t *testing.T) {
	resetConfirmDialogState()
	called := false
	activeDialog = &dialogState{Text: "test", OnConfirm: func(r *Renderer) { called = true }}
	r := newTestRenderer()

	resolveButtonDialog(r) // pending (Result == 0): must not resolve yet
	if activeDialog == nil {
		t.Fatal("expected a still-pending dialog to remain open")
	}
	if called {
		t.Error("expected OnConfirm not to run before the user picks anything")
	}

	activeDialog.Result = 2 // NG / cancel
	resolveButtonDialog(r)
	if activeDialog != nil {
		t.Error("expected resolveButtonDialog to clear activeDialog after NG")
	}
	if called {
		t.Error("expected OnConfirm not to run on NG")
	}

	resetConfirmDialogState()
	called = false
	activeDialog = &dialogState{Text: "test", OnConfirm: func(r *Renderer) { called = true }}
	activeDialog.Result = 1 // OK
	resolveButtonDialog(r)
	if activeDialog != nil {
		t.Error("expected resolveButtonDialog to clear activeDialog after OK")
	}
	if !called {
		t.Error("expected OnConfirm to run on OK")
	}
}

// TestResolveButtonDialogIgnoresTagDialogs covers the other kind of
// activeDialog — a [dialog] *tag*'s (no OnConfirm) — which resolves itself
// inside handleDialog's own y.Until and must be left alone here even once
// Result is set, so Update() calling resolveButtonDialog unconditionally
// every frame doesn't double-resolve it.
func TestResolveButtonDialogIgnoresTagDialogs(t *testing.T) {
	resetConfirmDialogState()
	activeDialog = &dialogState{Text: "test", Target: "somewhere", Result: 1} // no OnConfirm
	r := newTestRenderer()
	resolveButtonDialog(r)
	if activeDialog == nil {
		t.Error("expected a tag-triggered dialog (no OnConfirm) to be left untouched")
	}
	resetConfirmDialogState()
}

// TestAnyModalActiveDistinguishesDialogKinds: a button-triggered confirm
// dialog must freeze story advancement (anyModalActive() == true); a
// [dialog] tag's dialog must not, since it already blocks the coroutine
// itself via y.Until — see anyModalActive's doc comment.
func TestAnyModalActiveDistinguishesDialogKinds(t *testing.T) {
	resetConfirmDialogState()
	activeDialog = &dialogState{Text: "tag dialog"} // no OnConfirm
	if anyModalActive() {
		t.Error("expected a [dialog]-tag dialog to not count as a freezing modal")
	}
	activeDialog = &dialogState{Text: "button dialog", OnConfirm: func(r *Renderer) {}}
	if !anyModalActive() {
		t.Error("expected a button-triggered confirm dialog to count as a freezing modal")
	}
	resetConfirmDialogState()
}

// TestGoToTitleHidesLeftoverGameplayChrome reproduces a real report: after
// confirmGoToTitle's OK took the player back to title.ks, the bottom-right
// menu button (left showing from scene1.ks's @showmenubutton) and the
// message window were still visible on top of the title screen. title.ks
// itself never hides these — real Tyrano only ever shows the title screen
// once, right after boot's own hidemenubutton — so goToTitle has to do it
// itself now that it can be re-entered mid-playthrough.
func TestGoToTitleHidesLeftoverGameplayChrome(t *testing.T) {
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	// goToTitle's r.loadScript("title.ks") will fail (no such file in this
	// empty fs), but that error is deliberately ignored by goToTitle — what
	// this test cares about is that the UI-reset side effects below still
	// happen regardless. A nil FSes map would panic (fs.ReadFile on a nil
	// fs.FS), not just error, so it needs to be a real, if empty, fs.FS.
	r.manager.FSes = map[string]fs.FS{"senarios": fstest.MapFS{}}
	textPosition = &kag3.TextPosition{Visible: true}
	menuButtonVisible = true
	backlogViewing = true
	menuOpen = true
	slotPickerActive = slotPickerSave

	r.goToTitle()

	if textPosition.Visible {
		t.Error("expected goToTitle to hide the message window")
	}
	if menuButtonVisible {
		t.Error("expected goToTitle to hide the leftover @showmenubutton corner icon")
	}
	if backlogViewing || menuOpen || slotPickerActive != slotPickerNone {
		t.Errorf("expected goToTitle to close any open overlay; backlogViewing=%v menuOpen=%v slotPickerActive=%v",
			backlogViewing, menuOpen, slotPickerActive)
	}
}

// TestButtonTargetJumpPushesCallFrameForReturnToResume reproduces the
// reported bug: clicking a config.ks-style [button target=... fix="true"]
// (e.g. a BGM volume button, target="*vol_bgm_change") did a bare jump, so
// the [return] at the end of *vol_bgm_change had no call frame to pop and
// silently fell through into the next label's body instead of resuming the
// config screen. buttonTargetJump must push a call frame — currentScriptIndex,
// not +1, since this runs outside the coroutine (see its doc comment) — so
// [return] resumes exactly where the button was clicked.
func TestButtonTargetJumpPushesCallFrameForReturnToResume(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "config.ks"
	r.labels = map[string]kag3.LabelInfo{"vol_bgm_change": {Name: "vol_bgm_change", Index: 10}}
	currentScriptIndex = 3
	jumpIndex, isJump = 0, false
	r.callStack = nil

	r.buttonTargetJump("*vol_bgm_change")

	if !isJump || jumpIndex != 10 {
		t.Fatalf("isJump/jumpIndex = %v/%d, want true/10", isJump, jumpIndex)
	}
	if len(r.callStack) != 1 || r.callStack[0] != (callFrame{Storage: "config.ks", Index: 3}) {
		t.Fatalf("callStack = %+v, want [{config.ks 3}]", r.callStack)
	}

	// Simulate reaching *vol_bgm_change's [return].
	i := 10
	tag := kag3.TagObject{Name: "return"}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("return dispatch error: %v", err)
	}
	if len(r.callStack) != 0 {
		t.Errorf("callStack after return = %+v, want empty", r.callStack)
	}
	// frame.Index (3, currentScriptIndex at click time) minus 1: the
	// enclosing loop's own i++ afterward brings i back to 3, re-entering
	// whatever tag was blocking there (e.g. [s]).
	if i != 2 {
		t.Errorf("i after return = %d, want 2 (so the enclosing loop's i++ lands back on index 3)", i)
	}
}

// TestHandleButtonParsesExpAndPreExp covers the other half of the exp=
// crash fix: the [button] tag itself must capture exp=/preexp= onto the
// registered kag3.Button so EvalButtonExp has something to run on click.
func TestHandleButtonParsesExpAndPreExp(t *testing.T) {
	buttons = nil
	defer func() { buttons = nil }()
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "button", Pm: map[string]string{
		"target": "*ch_speed_change",
		"exp":    "tf.set_ch_speed = 100",
		"preexp": "mp.graphic",
		"width":  "10",
		"height": "10",
	}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(buttons) != 1 {
		t.Fatalf("buttons = %+v, want 1 registered button", buttons)
	}
	if buttons[0].Exp != "tf.set_ch_speed = 100" || buttons[0].PreExp != "mp.graphic" {
		t.Errorf("buttons[0].Exp/PreExp = %q/%q, want the tag's exp=/preexp=", buttons[0].Exp, buttons[0].PreExp)
	}
}

func TestClearNonFixButtonsKeepsOnlyFixButtons(t *testing.T) {
	buttons = []*kag3.Button{{Name: "a", Fix: true}, {Name: "b", Fix: false}, {Name: "c", Fix: true}}
	defer func() { buttons = nil }()

	clearNonFixButtons()

	if len(buttons) != 2 || buttons[0].Name != "a" || buttons[1].Name != "c" {
		t.Errorf("buttons = %+v, want only the fix buttons \"a\" and \"c\" remaining", buttons)
	}
}
