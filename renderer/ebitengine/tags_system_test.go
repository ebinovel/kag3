package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

func TestHandleWaitZeroDurationCompletesImmediately(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "wait", Pm: map[string]string{"time": "0"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
}

func TestHandleWaitCancelUnblocksWait(t *testing.T) {
	r := newTestRenderer()
	calls := 0
	y := coro.Yield(func() bool {
		calls++
		waitCancelled = true
		return true
	})
	tag := kag3.TagObject{Name: "wait", Pm: map[string]string{"time": "999999"}}
	i := 0
	if err := dispatchTag(r, y, tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if calls == 0 {
		t.Error("expected the yield to be invoked at least once")
	}
}

// TestHandleWTCompletesWhenElapsed uses "tt" instead of "t" for the
// *testing.T parameter because this test needs to read/reset the
// package-level tick counter "t" that handleWT compares bgTick against.
func TestHandleWTCompletesWhenElapsed(tt *testing.T) {
	r := newTestRenderer()
	bg.Time = 100
	bgTick = 0
	t = 0
	y := coro.Yield(func() bool {
		t++
		return true
	})
	tag := kag3.TagObject{Name: "wt"}
	i := 0
	if err := dispatchTag(r, y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if t <= 0 {
		tt.Errorf("expected t to have advanced past 0, got %d", t)
	}
}

func TestHandleClose(t *testing.T) {
	r := newTestRenderer()
	textPosition.Visible = true
	tag := kag3.TagObject{Name: "close"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if textPosition.Visible {
		t.Error("expected textPosition.Visible = false after [close]")
	}
}

func TestSleepgameAwakegameRoundTrip(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "test.ks"
	r.labels = map[string]kag3.LabelInfo{"sub": {Name: "sub", Index: 5}}

	sleepTag := kag3.TagObject{Name: "sleepgame", Line: 2, Pm: map[string]string{"target": "*sub"}}
	i := 2
	if err := dispatchTag(r, fakeYield(), sleepTag, &i, 0); err != nil {
		t.Fatalf("sleepgame dispatch error: %v", err)
	}
	if len(r.sleepStack) != 1 {
		t.Fatalf("sleepStack = %+v, want 1 frame after sleepgame", r.sleepStack)
	}
	if i != 5 {
		t.Errorf("i after sleepgame = %d, want target index 5", i)
	}

	awakeTag := kag3.TagObject{Name: "awakegame"}
	if err := dispatchTag(r, fakeYield(), awakeTag, &i, 0); err != nil {
		t.Fatalf("awakegame dispatch error: %v", err)
	}
	if len(r.sleepStack) != 0 {
		t.Errorf("sleepStack = %+v, want empty after awakegame", r.sleepStack)
	}
	if i != 2 {
		t.Errorf("i after awakegame = %d, want to resume right after sleepgame (2)", i)
	}
}

// TestAwakeGameRestoresCallersButtonsBackgroundAndTextPosition reproduces
// three real reports from the same flow: from title.ks, opening config.ks
// (role="sleepgame") then clicking Back correctly returned execution to
// title.ks, but (1) title.ks's own buttons (New Game/Load/CG/Replay/Config)
// never came back, (2) the background stayed config's (bg_config.png)
// instead of reverting to title.ks's (title.jpg), and (3) after visiting
// config's text-speed sample (*ch_speed_change, which repoints textPosition
// to a tiny "message1" preview box and relies on [layopt visible=false] —
// a no-op, since kag3 has no per-layer visibility system — to hide it
// again), that leftover preview box stayed on screen back on title.ks
// instead of disappearing. config.ks's *backtitle calls [clearfix] before
// [awakegame] (correctly clearing config's own buttons) and config.ks's
// own boot/subroutines re-point the single package-level bg/textPosition —
// without a snapshot to restore, all three bleed into title.ks even though
// execution correctly resumes there.
func TestAwakeGameRestoresCallersButtonsBackgroundAndTextPosition(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "title.ks"
	r.labels = map[string]kag3.LabelInfo{"config_page": {Name: "config_page", Index: 5}}
	buttons = []*kag3.Button{{Name: "start"}, {Name: "config"}}
	bg = &kag3.Background{Storage: "title.jpg", Method: "crossfade"}
	textPosition = &kag3.TextPosition{Visible: false}
	defer func() { buttons = nil }()

	sleepTag := kag3.TagObject{Name: "sleepgame", Pm: map[string]string{"target": "*config_page"}}
	i := 2
	if err := dispatchTag(r, fakeYield(), sleepTag, &i, 0); err != nil {
		t.Fatalf("sleepgame dispatch error: %v", err)
	}

	// Simulate config.ks: it registers its own buttons, swaps in its own
	// background, and (via *ch_speed_change) repoints textPosition to the
	// tiny sample-preview box — then clears the buttons via [clearfix]
	// (all Fix=true) right before returning. The background and
	// textPosition stay swapped; [layopt visible=false] doesn't undo the
	// textPosition change (it's a no-op — see handleLayopt).
	buttons = []*kag3.Button{{Name: "vol_bgm_10", Fix: true}}
	bg = &kag3.Background{Storage: "bg_config.png", Method: "crossfade"}
	textPosition = &kag3.TextPosition{Layer: "message1", Left: 90, Top: 580, Width: 1100, Height: 100, Visible: true}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "clearfix"}, &i, 0); err != nil {
		t.Fatalf("clearfix dispatch error: %v", err)
	}
	if len(buttons) != 0 {
		t.Fatalf("buttons after clearfix = %+v, want empty", buttons)
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "awakegame"}, &i, 0); err != nil {
		t.Fatalf("awakegame dispatch error: %v", err)
	}
	if len(buttons) != 2 || buttons[0].Name != "start" || buttons[1].Name != "config" {
		t.Errorf("buttons after awakegame = %+v, want title.ks's original [start config]", buttons)
	}
	if bg.Storage != "title.jpg" {
		t.Errorf("bg.Storage after awakegame = %q, want title.ks's original %q", bg.Storage, "title.jpg")
	}
	if textPosition.Visible || textPosition.Layer == "message1" {
		t.Errorf("textPosition after awakegame = %+v, want title.ks's original (Visible=false)", textPosition)
	}
}

// TestAwakeGameDoesNotResetTextSpeed guards the opposite of the
// TextPosition-restore fix above: unlike textPosition/buttons/bg, textSpeedMs
// (set via [configdelay], which config.ks's *ch_speed_change calls with the
// player's chosen speed) is a deliberate, persistent player setting — a
// player who picks a faster/slower text speed in config and backs out
// expects it to carry into the story, not revert to whatever it was before
// they opened config. sleepFrame never captured it, so this should already
// hold; asserted explicitly so a future change doesn't accidentally start
// snapshotting/restoring it.
func TestAwakeGameDoesNotResetTextSpeed(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "title.ks"
	r.labels = map[string]kag3.LabelInfo{"config_page": {Name: "config_page", Index: 5}}
	buttons = []*kag3.Button{}
	bg = &kag3.Background{}
	textPosition = &kag3.TextPosition{}
	textSpeedMs, defaultTextSpeedMs = 83, 83
	defer func() { buttons = nil }()

	i := 2
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "sleepgame", Pm: map[string]string{"target": "*config_page"}}, &i, 0); err != nil {
		t.Fatalf("sleepgame dispatch error: %v", err)
	}

	// Simulate config.ks's *ch_speed_change: the player picked the "ch_50"
	// speed, applied via [configdelay speed=50].
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "configdelay", Pm: map[string]string{"speed": "50"}}, &i, 0); err != nil {
		t.Fatalf("configdelay dispatch error: %v", err)
	}

	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "awakegame"}, &i, 0); err != nil {
		t.Fatalf("awakegame dispatch error: %v", err)
	}
	if textSpeedMs != 50 {
		t.Errorf("textSpeedMs after awakegame = %d, want the player's chosen 50 to persist", textSpeedMs)
	}
}

// TestSleepgameDoesNotShareCallStack guards the reason sleepStack exists at
// all: config.ks (the bundled sample) calls [clearstack] right before
// [awakegame] to discard [button target=...]-click call frames — that must
// not also discard the pending sleepgame frame [awakegame] needs.
func TestSleepgameDoesNotShareCallStack(t *testing.T) {
	r := newTestRenderer()
	r.currentStorage = "scene1.ks"
	r.labels = map[string]kag3.LabelInfo{"sub": {Name: "sub", Index: 5}}

	sleepTag := kag3.TagObject{Name: "sleepgame", Pm: map[string]string{"target": "*sub"}}
	i := 2
	if err := dispatchTag(r, fakeYield(), sleepTag, &i, 0); err != nil {
		t.Fatalf("sleepgame dispatch error: %v", err)
	}

	clearTag := kag3.TagObject{Name: "clearstack"}
	if err := dispatchTag(r, fakeYield(), clearTag, &i, 0); err != nil {
		t.Fatalf("clearstack dispatch error: %v", err)
	}
	if len(r.callStack) != 0 {
		t.Errorf("callStack = %+v, want empty after clearstack", r.callStack)
	}
	if len(r.sleepStack) != 1 {
		t.Fatalf("sleepStack = %+v, want the sleepgame frame to survive clearstack", r.sleepStack)
	}

	awakeTag := kag3.TagObject{Name: "awakegame"}
	if err := dispatchTag(r, fakeYield(), awakeTag, &i, 0); err != nil {
		t.Fatalf("awakegame dispatch error: %v", err)
	}
	if i != 2 {
		t.Errorf("i after awakegame = %d, want to resume right after sleepgame (2)", i)
	}
}

func TestBreakGameDiscardsFrameWithoutResuming(t *testing.T) {
	r := newTestRenderer()
	r.sleepStack = []sleepFrame{{Storage: "test.ks", Index: 3}}

	tag := kag3.TagObject{Name: "breakgame"}
	i := 10
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(r.sleepStack) != 0 {
		t.Errorf("sleepStack = %+v, want empty after breakgame", r.sleepStack)
	}
	if i != 10 {
		t.Errorf("i = %d, want unchanged 10 (breakgame must not resume like return)", i)
	}
}
