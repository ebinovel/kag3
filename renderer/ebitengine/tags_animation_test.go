package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

// TestHandleAnimMovesCharaOverTime uses "tt" instead of "t" for the
// *testing.T parameter because this test advances the package-level tick
// counter "t" that stepAnimations compares against (same reason as
// TestHandleWTCompletesWhenElapsed).
func TestHandleAnimMovesCharaOverTime(tt *testing.T) {
	animations = map[string]*animation{}
	c := &kag3.CharaShow{Name: "akane", Left: 0, Top: 0, Opacity: 255, ScaleX: 1, ScaleY: 1}
	viewCharas = []*kag3.CharaShow{c}
	t = 0

	y := coro.Yield(func() bool {
		t++
		stepAnimations()
		return true
	})
	tag := kag3.TagObject{Name: "anim", Pm: map[string]string{
		"name": "akane", "left": "100", "opacity": "128", "time": "100", "wait": "true",
	}}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if c.Left != 100 {
		tt.Errorf("Left = %d, want 100 (animation should have reached its target)", c.Left)
	}
	if c.Opacity != 128 {
		tt.Errorf("Opacity = %v, want 128", c.Opacity)
	}
	if _, ok := animations["akane"]; ok {
		tt.Error("expected the animation to be removed from animations once complete")
	}
}

func TestHandleAnimUnknownTargetErrors(t *testing.T) {
	viewCharas = nil
	imgs = nil
	tag := kag3.TagObject{Name: "anim", Pm: map[string]string{"name": "nobody", "left": "10"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for an unknown anim target")
	}
}

func TestHandleAnimTargetsImageToo(tt *testing.T) {
	animations = map[string]*animation{}
	img := &kag3.Image{Name: "cg1", X: 0, Y: 0, Opacity: 255, ScaleX: 1, ScaleY: 1}
	imgs = []*kag3.Image{img}
	t = 0

	y := coro.Yield(func() bool {
		t++
		stepAnimations()
		return true
	})
	// "left"/"top" map onto Image.X/Y (see kag3.Image.SetLeft/SetTop),
	// since that's what drawScene actually renders from.
	tag := kag3.TagObject{Name: "anim", Pm: map[string]string{"name": "cg1", "left": "50", "time": "50", "wait": "true"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if img.X != 50 {
		tt.Errorf("X = %d, want 50", img.X)
	}
}

func TestHandleAnimNonBlockingLeavesAnimationRunning(t *testing.T) {
	animations = map[string]*animation{}
	c := &kag3.CharaShow{Name: "akane", Opacity: 255, ScaleX: 1, ScaleY: 1}
	viewCharas = []*kag3.CharaShow{c}

	tag := kag3.TagObject{Name: "anim", Pm: map[string]string{"name": "akane", "left": "999", "time": "999999", "wait": "false"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if _, ok := animations["akane"]; !ok {
		t.Error("expected the animation to still be running after a non-blocking anim call")
	}
}

func TestHandleStopAnimFreezesInPlace(t *testing.T) {
	animations = map[string]*animation{"akane": {startTick: 0, durTicks: 100}}
	tag := kag3.TagObject{Name: "stopanim", Pm: map[string]string{"name": "akane"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if _, ok := animations["akane"]; ok {
		t.Error("expected the animation to be removed by stopanim")
	}
}

func TestHandleStopAnimAllWithNoName(t *testing.T) {
	animations = map[string]*animation{
		"a": {startTick: 0, durTicks: 100},
		"b": {startTick: 0, durTicks: 100},
	}
	tag := kag3.TagObject{Name: "stop_xanim", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(animations) != 0 {
		t.Errorf("animations = %+v, want empty", animations)
	}
}

func TestHandleWAWaitsForNamedAnimation(tt *testing.T) {
	animations = map[string]*animation{}
	c := &kag3.CharaShow{Name: "akane", Opacity: 255, ScaleX: 1, ScaleY: 1}
	viewCharas = []*kag3.CharaShow{c}
	t = 0

	y := coro.Yield(func() bool {
		t++
		stepAnimations()
		return true
	})
	animTag := kag3.TagObject{Name: "anim", Pm: map[string]string{"name": "akane", "left": "10", "time": "50", "wait": "false"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, animTag, &i, 0); err != nil {
		tt.Fatalf("anim dispatch error: %v", err)
	}
	waTag := kag3.TagObject{Name: "wa", Pm: map[string]string{"name": "akane"}}
	if err := dispatchTag(newTestRenderer(), y, waTag, &i, 0); err != nil {
		tt.Fatalf("wa dispatch error: %v", err)
	}
	if _, ok := animations["akane"]; ok {
		tt.Error("expected wa to block until the animation finished")
	}
}

func TestKeyframeScanCollectsFramesAndSkipsThem(t *testing.T) {
	keyframes = map[string][]frameSpec{}
	scripts := []any{
		kag3.TagObject{Name: "keyframe", Line: 0, Pm: map[string]string{"name": "seq1"}},
		kag3.TagObject{Name: "frame", Line: 1, Pm: map[string]string{"time": "0", "left": "0"}},
		kag3.TagObject{Name: "frame", Line: 2, Pm: map[string]string{"time": "100", "left": "50"}},
		kag3.TagObject{Name: "endkeyframe", Line: 3},
		kag3.TextObject{Line: 4, Name: "text", Val: "after"},
	}
	r := newTestRenderer()
	r.scripts = scripts
	for i := 0; i < len(scripts); i++ {
		if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
			t.Fatalf("execItem error: %v", err)
		}
	}
	frames, ok := keyframes["seq1"]
	if !ok || len(frames) != 2 {
		t.Fatalf("keyframes[seq1] = %+v, want 2 frames", frames)
	}
	if frames[1].timeMs != 100 || frames[1].props["left"] != "50" {
		t.Errorf("frames[1] = %+v, want time=100 left=50", frames[1])
	}
	if got := r.texts[4]; len(got) != 1 || got[0].Text != "after" {
		t.Errorf("r.texts[4] = %+v, want the text after the keyframe block to still render", got)
	}
}

func TestHandleKanimPlaysSequence(tt *testing.T) {
	keyframes = map[string][]frameSpec{
		"seq1": {
			{timeMs: 0, props: map[string]string{"left": "0"}},
			{timeMs: 50, props: map[string]string{"left": "100"}},
		},
	}
	c := &kag3.CharaShow{Name: "akane", Opacity: 255, ScaleX: 1, ScaleY: 1}
	viewCharas = []*kag3.CharaShow{c}
	t = 0

	y := coro.Yield(func() bool {
		t++
		stepAnimations()
		return true
	})
	tag := kag3.TagObject{Name: "kanim", Pm: map[string]string{"name": "akane", "keyframe": "seq1"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if c.Left != 100 {
		tt.Errorf("Left = %d, want 100 after playing the keyframe sequence", c.Left)
	}
}

func TestHandleKanimUnknownKeyframeErrors(t *testing.T) {
	keyframes = map[string][]frameSpec{}
	c := &kag3.CharaShow{Name: "akane"}
	viewCharas = []*kag3.CharaShow{c}
	tag := kag3.TagObject{Name: "kanim", Pm: map[string]string{"name": "akane", "keyframe": "missing"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for an undefined keyframe sequence")
	}
}

func TestEasingFuncLinearByDefault(t *testing.T) {
	if easingFunc("") != nil {
		t.Error("expected easingFunc(\"\") to be nil (linear)")
	}
	if easingFunc("unknown-name") != nil {
		t.Error("expected an unrecognized accel name to fall back to linear (nil)")
	}
	if f := easingFunc("accelerate"); f == nil || f(0.5) == 0.5 {
		t.Error("expected accelerate to be a real non-linear curve")
	}
}
