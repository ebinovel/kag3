package effects

import (
	"testing"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestTransitionsCoversEveryBackgroundMethod(t *testing.T) {
	for _, method := range kag3.BackgroundMethod {
		if _, ok := Transitions[method]; !ok {
			t.Errorf("BackgroundMethod %q has no Transitions entry", method)
		}
	}
	if len(Transitions) != len(kag3.BackgroundMethod) {
		t.Errorf("Transitions has %d entries, BackgroundMethod has %d — check for stale/duplicate keys",
			len(Transitions), len(kag3.BackgroundMethod))
	}
}

func newTestBG() *kag3.Background {
	return &kag3.Background{
		Image:     ebiten.NewImage(64, 64),
		NextImage: ebiten.NewImage(64, 64),
		Time:      100,
	}
}

// TestEveryTransitionCompletesAndSwaps drives each registered transition
// well past its duration and checks the two invariants every
// DrawBackground implementation must uphold: bg.IsEnd becomes true, and
// bg.Image/NextImage get swapped (via finishBackground). This is what
// [bg wait=true]/[wt] depend on to ever unblock.
func TestEveryTransitionCompletesAndSwaps(t *testing.T) {
	screen := ebiten.NewImage(64, 64)
	for method, transition := range Transitions {
		bg := newTestBG()
		next := bg.NextImage
		transition.DrawBackground(screen, bg, 0, 100000, bg.Time)
		if !bg.IsEnd {
			t.Errorf("%s: IsEnd = false after well past duration", method)
		}
		if bg.NextImage != nil {
			t.Errorf("%s: NextImage = %v, want nil after completion", method, bg.NextImage)
		}
		if bg.Image != next {
			t.Errorf("%s: Image was not swapped to the former NextImage on completion", method)
		}
	}
}

// TestEveryTransitionMidwayDoesNotCompleteEarly checks the other half of
// the same invariant: nothing finishes before its time is up.
func TestEveryTransitionMidwayDoesNotCompleteEarly(t *testing.T) {
	screen := ebiten.NewImage(64, 64)
	for method, transition := range Transitions {
		bg := newTestBG()
		transition.DrawBackground(screen, bg, 0, 1, bg.Time) // 1 tick into a 100ms (6-tick) transition
		if bg.IsEnd {
			t.Errorf("%s: IsEnd = true after only 1 tick, want still in progress", method)
		}
		if bg.NextImage == nil {
			t.Errorf("%s: NextImage was cleared before the transition finished", method)
		}
	}
}

func TestEasingHelpers(t *testing.T) {
	if got := progress(0, 0, 100); got != 0 {
		t.Errorf("progress at start = %v, want 0", got)
	}
	if got := progress(0, 100000, 100); got != 1 {
		t.Errorf("progress far past end = %v, want 1", got)
	}
	if got := progress(0, 100, 0); got != 1 {
		t.Errorf("progress with timeMs<=0 = %v, want 1 (treat as already done)", got)
	}
}
