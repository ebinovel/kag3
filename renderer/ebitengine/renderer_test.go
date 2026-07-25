package ebitengine

import (
	"image/color"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
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

// TestVerticalFontFaceSubstitutesVerticalGlyphs guards the vertical-text
// glyph substitution added to fix [position vertical=true]: without the
// OpenType "vert" feature enabled, kag3's manual vertical layout was just
// rotating horizontal-form glyphs into a column, so punctuation like "。"
// stayed in its horizontal (lower-left) position instead of moving to the
// vertical form's upper-right, and "ー" stayed a horizontal bar instead of
// a vertical one. NotoSansJP-Regular.ttf (the bundled font) does define
// substitute glyphs for these — this pins that down via GID, since the
// rendered pixels can't be read back outside a real ebiten game loop.
func TestVerticalFontFaceSubstitutesVerticalGlyphs(t *testing.T) {
	horiz := newTestFontFace(t)
	vert := newTestVerticalFontFace(t)
	for _, ch := range []string{"。", "ー", "ゃ"} {
		hg := text.AppendGlyphs(nil, ch, horiz, nil)
		vg := text.AppendGlyphs(nil, ch, vert, nil)
		if len(hg) == 0 || len(vg) == 0 {
			t.Fatalf("%q: expected at least one glyph from both faces, got horiz=%d vert=%d", ch, len(hg), len(vg))
		}
		if hg[0].GID == vg[0].GID {
			t.Errorf("%q: horizontal and vertical GID both = %d, want the vert feature to substitute a different glyph", ch, hg[0].GID)
		}
	}
}

// TestDrawSceneVerticalTextDoesNotPanic covers [position vertical=true]'s
// draw path (renderer.go's textPosition.Vertical branch) end to end: it
// must use r.verticalFontFace (wired from Manager.VerticalFontFace in
// NewRenderer), not silently fall back to a nil face.
func TestDrawSceneVerticalTextDoesNotPanic(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.verticalFontFace = newTestVerticalFontFace(t)
	r.texts = map[int][]Text{0: {{Text: "このように縦書きで記述することもできます。"}}}
	r.line = 0
	textPosition = &kag3.TextPosition{
		Visible: true, Vertical: true,
		Left: 20, Top: 40, Width: 1200, Height: 660,
		MarginTop: 45, MarginRight: 70, MarginBottom: 60,
	}
	textPosition.BackImage = newTestImage(textPosition.Width, textPosition.Height)
	bg = &kag3.Background{}
	bg2 = &kag3.Background{}
	viewCharas, links, glinks, imgs, buttons = nil, nil, nil, nil, nil
	isWait = true

	buf := newTestImage(1280, 720)
	r.drawScene(buf) // must not panic
}

// TestDrawSceneCapturesSnapshotBeforeModalOverlay reproduces a real report:
// save slot thumbnails were showing the quick menu / save screen itself
// instead of the game scene underneath. The earlier fix captured
// lastSnapshot when openSlotPicker/saveSlot ran, but by then renderBuffer
// could already have another modal (e.g. the quick menu, reached first to
// click its own SAVE item) baked into it from prior frames. drawScene now
// captures unconditionally, every call, right before drawModal draws
// whatever overlay is active into buf — so lastSnapshot always reflects
// the scene as of *this* frame with no modal on it yet, regardless of
// what was on screen before. This only proves the "always re-captured,
// fresh object" contract (pixel content can't be asserted headless — see
// captureSnapshot's own doc comment on why): combined with capture being
// the last thing before drawModal in source order, that's the observable
// behavior a test here can pin down.
func TestDrawSceneCapturesSnapshotBeforeModalOverlay(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	r.fses = map[string]fs.FS{"system/images": fstest.MapFS{}}
	textPosition = &kag3.TextPosition{Visible: false}
	bg = &kag3.Background{}
	bg2 = &kag3.Background{}
	viewCharas = nil
	links, glinks, imgs, buttons = nil, nil, nil, nil
	lastSnapshot = nil
	menuOpen = false
	defer func() { menuOpen = false; lastSnapshot = nil }()

	buf := newTestImage(1280, 720)
	r.drawScene(buf)
	firstCapture := lastSnapshot
	if firstCapture == nil {
		t.Fatal("expected drawScene to populate lastSnapshot even with no modal active")
	}

	menuOpen = true
	r.drawScene(buf)
	if lastSnapshot == nil {
		t.Fatal("expected drawScene to populate lastSnapshot with the quick menu open")
	}
	if lastSnapshot == firstCapture {
		t.Error("expected a fresh capture on this call, not the previous frame's")
	}
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
	// goToTitle sets oldTick = tick - 4; this test never drives tick forward
	// afterward, so that offset would otherwise stick around and break later
	// tests' isTextEnded checks (oldTick+3>=tick, permanently false once
	// oldTick is stuck behind tick) — see the matching comment in
	// confirm_title_flow_test.go.
	defer func() { tick, oldTick = 0, 0 }()

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

// TestResolveFolderImage is the regression test for the other half of the
// 回想モード (recollection mode) bug: tyrano.ks's replay_image_button/
// cg_image_button macros pass folder="bgimage" (a subdirectory of images,
// not a separate top-level FS root kag3 never registered) together with a
// real-Tyrano-specific relative escape for their shared "no image"
// placeholder ("../../tyrano/images/system/noimage.png") that plain
// r.fses["bgimage"] resolution turned into a nil FS, panicking
// ebitenutil.NewImageFromFileSystem the moment a locked replay/CG slot
// tried to draw its fallback graphic.
func TestResolveFolderImage(t *testing.T) {
	r := newTestRenderer()
	// fstest.MapFS is a map, which can't be compared with == (interface
	// comparison panics on uncomparable dynamic types) — so identity is
	// checked behaviorally instead, via a marker file unique to each FS.
	imagesFS := fstest.MapFS{"images-marker": &fstest.MapFile{}}
	systemFS := fstest.MapFS{"system-marker": &fstest.MapFile{}}
	r.fses = map[string]fs.FS{"images": imagesFS, "system/images": systemFS}
	isImagesFS := func(f fs.FS) bool { _, err := fs.Stat(f, "images-marker"); return err == nil }
	isSystemFS := func(f fs.FS) bool { _, err := fs.Stat(f, "system-marker"); return err == nil }

	t.Run("plain images root", func(t *testing.T) {
		gotFS, gotName := resolveFolderImage(r, "", "room.jpg")
		if !isImagesFS(gotFS) || gotName != "room.jpg" {
			t.Errorf("resolveFolderImage(\"\", \"room.jpg\") name=%q, want images FS, \"room.jpg\"", gotName)
		}
	})

	t.Run("bgimage subfolder", func(t *testing.T) {
		gotFS, gotName := resolveFolderImage(r, "bgimage", "cat.jpg")
		if !isImagesFS(gotFS) || gotName != "bgimage/cat.jpg" {
			t.Errorf(`resolveFolderImage("bgimage", "cat.jpg") name=%q, want images FS, "bgimage/cat.jpg"`, gotName)
		}
	})

	t.Run("relative escape to the shared system asset folder", func(t *testing.T) {
		gotFS, gotName := resolveFolderImage(r, "bgimage", "../../tyrano/images/system/noimage.png")
		if !isSystemFS(gotFS) || gotName != "noimage.png" {
			t.Errorf(`resolveFolderImage("bgimage", "../../tyrano/images/system/noimage.png") name=%q, want system/images FS, "noimage.png"`, gotName)
		}
	})
}

// TestParseColor is the regression test for a real bug found while fixing
// black<->white: the "0xRRGGBB" hex branch sliced out only the first digit
// of each byte pair (e.g. "0x454D51" -> "4","4","5") and fed it to Atoi as
// *decimal*, and "black" itself returned white. "white"/"pink" — also used
// by the bundled scripts — weren't recognized as named colors at all and
// fell into that same broken hex path. Both color="0x454D51"/"0xFAFAFA"
// (scene1.ks's custom message window) and color="pink"/"white" are real,
// reachable attribute values in the bundled example scripts.
func TestParseColor(t *testing.T) {
	cases := []struct {
		name                string
		in                  string
		wantR, wantG, wantB int
		wantErr             bool
	}{
		{"black", "black", 0, 0, 0, false},
		{"white", "white", 255, 255, 255, false},
		{"red", "red", 255, 0, 0, false},
		{"blue", "blue", 0, 0, 255, false},
		{"pink", "pink", 255, 192, 203, false},
		{"hex used by scene1.ks's message window", "0x454D51", 0x45, 0x4D, 0x51, false},
		{"hex used by scene1.ks's name plate", "0xFAFAFA", 0xFA, 0xFA, 0xFA, false},
		{"hex with a letter in the first digit of a pair", "0xD45D51", 0xD4, 0x5D, 0x51, false},
		{"malformed", "not-a-color", 0, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, g, b, err := parseColor(c.in)
			if c.wantErr {
				if err == nil {
					t.Errorf("parseColor(%q) = %d,%d,%d,nil, want an error", c.in, r, g, b)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseColor(%q) unexpected error: %v", c.in, err)
			}
			if r != c.wantR || g != c.wantG || b != c.wantB {
				t.Errorf("parseColor(%q) = %d,%d,%d, want %d,%d,%d", c.in, r, g, b, c.wantR, c.wantG, c.wantB)
			}
		})
	}
}

// TestApplyTextStyleKeepsCurrentFontColorForInactiveLine is the regression
// test for「こんな風に。簡単です。」disappearing: drawScene's "already
// fully-revealed line" branch used to fall straight to v.TextStyle (always
// nil — Text segments never set it at creation) and default to plain white,
// ignoring the package-level textStyle entirely. On scene1.ks's custom
// message window ([deffont color="0x454D51"], a near-white box), that made
// every earlier line on the page revert to invisible white text the moment
// it stopped being the currently-revealing line. applyTextStyle must resolve
// the *same* color regardless of which line is calling it.
func TestApplyTextStyleKeepsCurrentFontColorForInactiveLine(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	beforeTextSize = r.fontFace.Size
	saved := textStyle
	defer func() { textStyle = saved }()
	textStyle = &kag3.TextStyle{Color: &color.RGBA{0x45, 0x4D, 0x51, 0xff}}

	tOp := &text.DrawOptions{}
	applyTextStyle(r, tOp, Text{Text: "こんな風に。簡単です。"})

	gotR, gotG, gotB := tOp.ColorScale.R(), tOp.ColorScale.G(), tOp.ColorScale.B()
	wantR, wantG, wantB := float32(0x45)/0xff, float32(0x4D)/0xff, float32(0x51)/0xff
	const tol = 0.01
	if abs32(gotR-wantR) > tol || abs32(gotG-wantG) > tol || abs32(gotB-wantB) > tol {
		t.Errorf("ColorScale = %v/%v/%v, want ~%v/%v/%v (0x454D51, the active [deffont] color)", gotR, gotG, gotB, wantR, wantG, wantB)
	}
	if gotR >= 0.99 && gotG >= 0.99 && gotB >= 0.99 {
		t.Errorf("ColorScale = %v/%v/%v, resolved to plain white instead of the active [deffont] color", gotR, gotG, gotB)
	}
}

// TestApplyTextStyleDoesNotLeakSizeAcrossSegments is the regression test
// for a real reported bug: after a "[font size=40]...[resetfont][font
// color=pink]" sequence, the pink text kept rendering (and, worse,
// measuring — see drawMessageHorizontal) at size 40, because a v.TextStyle
// whose own Size was left unspecified (0, e.g. a [font] call that only
// changed color) used to leave r.fontFace.Size completely untouched rather
// than falling back to beforeTextSize — so it silently kept whatever the
// *previous* segment's applyTextStyle call last set it to.
func TestApplyTextStyleDoesNotLeakSizeAcrossSegments(t *testing.T) {
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	beforeTextSize = 24
	r.fontFace.Size = beforeTextSize
	saved := textStyle
	defer func() { textStyle = saved }()
	textStyle = nil

	applyTextStyle(r, &text.DrawOptions{}, Text{TextStyle: &kag3.TextStyle{Size: 40}})
	if r.fontFace.Size != 40 {
		t.Fatalf("fontFace.Size after size=40 segment = %v, want 40", r.fontFace.Size)
	}

	// A later segment whose own TextStyle only changes color (Size left at
	// its zero value) must fall back to beforeTextSize, not keep 40.
	applyTextStyle(r, &text.DrawOptions{}, Text{TextStyle: &kag3.TextStyle{Color: &color.RGBA{0xff, 0xc0, 0xcb, 0xff}}})
	if r.fontFace.Size != beforeTextSize {
		t.Errorf("fontFace.Size after color-only segment = %v, want beforeTextSize %v (leaked from the earlier size=40 segment instead of resetting)", r.fontFace.Size, beforeTextSize)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestClearNonFixButtonsKeepsOnlyFixButtons(t *testing.T) {
	buttons = []*kag3.Button{{Name: "a", Fix: true}, {Name: "b", Fix: false}, {Name: "c", Fix: true}}
	defer func() { buttons = nil }()

	clearNonFixButtons()

	if len(buttons) != 2 || buttons[0].Name != "a" || buttons[1].Name != "c" {
		t.Errorf("buttons = %+v, want only the fix buttons \"a\" and \"c\" remaining", buttons)
	}
}

// TestClearLinksOnJumpKeepsButtonsForSameStorageJump is the regression test
// for the reported bug: clicking a [link] to a label in the *same* scenario
// (e.g. scene1.ks's [link target=*playmusic]) made the role_button set
// (registered earlier via [button name="role_button" ...], no fix="true" —
// matching the real bundled sample script exactly) disappear. That jump sets
// isJump but must NOT set screenChanged, so clearLinksOnJump must leave
// non-fix buttons alone.
func TestClearLinksOnJumpKeepsButtonsForSameStorageJump(t *testing.T) {
	buttons = []*kag3.Button{{Name: "role_button", Fix: false}}
	links = []*kag3.Link{{Target: "*playmusic"}}
	glinks = []*kag3.GLink{{Target: "somewhere"}}
	isJump = true
	screenChanged = false
	defer func() { buttons, links, glinks, isJump, screenChanged = nil, nil, nil, false, false }()

	clearLinksOnJump()

	if len(buttons) != 1 || buttons[0].Name != "role_button" {
		t.Errorf("buttons = %+v, want the role_button to survive a same-storage label jump", buttons)
	}
	if links != nil || glinks != nil {
		t.Errorf("links/glinks = %+v/%+v, want both cleared regardless of screenChanged", links, glinks)
	}
}

// TestClearLinksOnJumpClearsNonFixButtonsForScreenChange is the counterpart:
// a real storage change (a [link storage=...] to a different .ks file,
// goToTitle, applySaveData) must still sweep non-fix buttons, same as
// before this fix.
func TestClearLinksOnJumpClearsNonFixButtonsForScreenChange(t *testing.T) {
	buttons = []*kag3.Button{{Name: "a", Fix: true}, {Name: "b", Fix: false}}
	isJump = true
	screenChanged = true
	defer func() { buttons, isJump, screenChanged = nil, false, false }()

	clearLinksOnJump()

	if len(buttons) != 1 || buttons[0].Name != "a" {
		t.Errorf("buttons = %+v, want only the fix button \"a\" remaining after a screen change", buttons)
	}
	if screenChanged {
		t.Error("expected screenChanged to be consumed (cleared) by clearLinksOnJump")
	}
}
