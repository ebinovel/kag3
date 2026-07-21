package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
	"github.com/hajimehoshi/ebiten/v2"
)

// TestHandleCameraTweensAndWaits uses "tt" instead of "t" for the same
// reason as TestHandleAnimMovesCharaOverTime: it advances the package-level
// tick counter "t".
func TestHandleCameraTweensAndWaits(tt *testing.T) {
	animations = map[string]*animation{}
	camera.X, camera.Y, camera.Scale = 0, 0, 1
	t = 0

	y := coro.Yield(func() bool {
		t++
		stepAnimations()
		return true
	})
	tag := kag3.TagObject{Name: "camera", Pm: map[string]string{"x": "50", "scale": "1.5", "time": "50", "wait": "true"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if camera.X != 50 {
		tt.Errorf("camera.X = %v, want 50", camera.X)
	}
	if camera.Scale != 1.5 {
		tt.Errorf("camera.Scale = %v, want 1.5", camera.Scale)
	}
	if camera.Y != 0 {
		tt.Errorf("camera.Y = %v, want unchanged 0 (no y= given)", camera.Y)
	}
}

func TestHandleResetCameraSnapsBackAndCancelsTween(t *testing.T) {
	animations = map[string]*animation{"camera": {startTick: 0, durTicks: 100}}
	camera.X, camera.Y, camera.Scale = 50, 30, 2
	tag := kag3.TagObject{Name: "reset_camera"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if camera.X != 0 || camera.Y != 0 || camera.Scale != 1 {
		t.Errorf("camera = %+v, want reset to {0 0 1}", camera)
	}
	if _, ok := animations["camera"]; ok {
		t.Error("expected reset_camera to cancel any in-flight camera tween")
	}
}

func TestHandleWaitCameraBlocksUntilTweenDone(tt *testing.T) {
	animations = map[string]*animation{"camera": {startTick: 0, durTicks: 3}}
	t = 0
	calls := 0
	y := coro.Yield(func() bool {
		calls++
		t++
		stepAnimations()
		return true
	})
	tag := kag3.TagObject{Name: "wait_camera"}
	i := 0
	if err := dispatchTag(newTestRenderer(), y, tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if calls == 0 {
		tt.Error("expected wait_camera to actually block on the yield")
	}
	if _, ok := animations["camera"]; ok {
		tt.Error("expected the camera animation to be finished")
	}
}

func TestQuakeSetsShakeStateAndOffsetDecays(tt *testing.T) {
	t = 0
	tag := kag3.TagObject{Name: "quake", Pm: map[string]string{"time": "100", "strength": "20", "wait": "false"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		tt.Fatalf("dispatchTag error: %v", err)
	}
	if shake.vertical {
		tt.Error("expected quake to shake horizontally")
	}
	// currentShakeOffset is amplitude*sin(elapsed*2.5): exactly 0 at
	// elapsed==0 by construction (sin(0)==0), so check a tick in rather
	// than right at the start.
	t++
	dx, dy := currentShakeOffset()
	if dx == 0 && dy == 0 {
		tt.Error("expected a non-zero shake offset one tick after quake starts")
	}
	if dy != 0 {
		tt.Errorf("dy = %v, want 0 for horizontal quake", dy)
	}

	t += 1000 // well past the shake duration
	dx, dy = currentShakeOffset()
	if dx != 0 || dy != 0 {
		tt.Errorf("offset after the shake ends = (%v, %v), want (0, 0)", dx, dy)
	}
}

func TestQuake2ShakesVertically(t *testing.T) {
	tag := kag3.TagObject{Name: "quake2", Pm: map[string]string{"time": "100", "wait": "false"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if !shake.vertical {
		t.Error("expected quake2 to shake vertically")
	}
}

func TestHandleVibrateAndStopDoNotError(t *testing.T) {
	tag := kag3.TagObject{Name: "vibrate", Pm: map[string]string{"time": "100", "strength": "0.5"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("vibrate dispatch error: %v", err)
	}
	stopTag := kag3.TagObject{Name: "vibrate_stop"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), stopTag, &i, 0); err != nil {
		t.Fatalf("vibrate_stop dispatch error: %v", err)
	}
}

func TestLayerModeAddAndFree(t *testing.T) {
	layerBlend = map[string]ebiten.Blend{}
	tag := kag3.TagObject{Name: "layermode", Pm: map[string]string{"layer": "1", "mode": "add"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("layermode dispatch error: %v", err)
	}
	if _, ok := layerBlend["1"]; !ok {
		t.Fatal("expected layer 1 to have a blend mode registered")
	}

	freeTag := kag3.TagObject{Name: "free_layermode", Pm: map[string]string{"layer": "1"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), freeTag, &i, 0); err != nil {
		t.Fatalf("free_layermode dispatch error: %v", err)
	}
	if _, ok := layerBlend["1"]; ok {
		t.Error("expected layer 1's blend mode to be cleared")
	}
}

func TestLayerModeUnsupportedModeErrors(t *testing.T) {
	tag := kag3.TagObject{Name: "layermode", Pm: map[string]string{"layer": "1", "mode": "multiply"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for an unsupported layermode mode")
	}
}

func TestFreeLayerModeAllWithNoLayer(t *testing.T) {
	layerBlend = map[string]ebiten.Blend{"1": ebiten.BlendLighter, "2": ebiten.BlendLighter}
	tag := kag3.TagObject{Name: "free_layermode", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(layerBlend) != 0 {
		t.Errorf("layerBlend = %+v, want empty", layerBlend)
	}
}

func TestFilterAndFreeFilter(t *testing.T) {
	activeFilter = nil
	tag := kag3.TagObject{Name: "filter", Pm: map[string]string{"color": "0x808080", "opacity": "128"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("filter dispatch error: %v", err)
	}
	if activeFilter == nil {
		t.Fatal("expected activeFilter to be set")
	}
	if activeFilter.Alpha == 0 {
		t.Error("expected a non-zero filter alpha")
	}

	freeTag := kag3.TagObject{Name: "free_filter"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), freeTag, &i, 0); err != nil {
		t.Fatalf("free_filter dispatch error: %v", err)
	}
	if activeFilter != nil {
		t.Error("expected activeFilter to be cleared")
	}
}

func TestMaskAndMaskOff(t *testing.T) {
	activeMask = nil
	r := newTestRendererWithImageFS(t, map[string][]byte{"mask.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "mask", Pm: map[string]string{"storage": "mask.png", "opacity": "200"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("mask dispatch error: %v", err)
	}
	if activeMask == nil || activeMask.Image == nil {
		t.Fatal("expected activeMask to be set with an image")
	}

	offTag := kag3.TagObject{Name: "mask_off"}
	if err := dispatchTag(r, fakeYield(), offTag, &i, 0); err != nil {
		t.Fatalf("mask_off dispatch error: %v", err)
	}
	if activeMask != nil {
		t.Error("expected activeMask to be cleared")
	}
}

func TestMaskMissingFileReturnsError(t *testing.T) {
	r := newTestRendererWithImageFS(t, map[string][]byte{})
	tag := kag3.TagObject{Name: "mask", Pm: map[string]string{"storage": "missing.png"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Error("expected an error for a missing mask file")
	}
}
