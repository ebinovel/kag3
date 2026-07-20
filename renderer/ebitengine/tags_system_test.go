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
	if len(r.callStack) != 1 {
		t.Fatalf("callStack = %+v, want 1 frame after sleepgame", r.callStack)
	}
	if i != 5 {
		t.Errorf("i after sleepgame = %d, want target index 5", i)
	}

	awakeTag := kag3.TagObject{Name: "awakegame"}
	if err := dispatchTag(r, fakeYield(), awakeTag, &i, 0); err != nil {
		t.Fatalf("awakegame dispatch error: %v", err)
	}
	if len(r.callStack) != 0 {
		t.Errorf("callStack = %+v, want empty after awakegame", r.callStack)
	}
	if i != 2 {
		t.Errorf("i after awakegame = %d, want to resume right after sleepgame (2)", i)
	}
}

func TestBreakGameDiscardsFrameWithoutResuming(t *testing.T) {
	r := newTestRenderer()
	r.callStack = []callFrame{{Storage: "test.ks", Index: 3}}

	tag := kag3.TagObject{Name: "breakgame"}
	i := 10
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(r.callStack) != 0 {
		t.Errorf("callStack = %+v, want empty after breakgame", r.callStack)
	}
	if i != 10 {
		t.Errorf("i = %d, want unchanged 10 (breakgame must not resume like return)", i)
	}
}
