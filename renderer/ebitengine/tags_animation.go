package ebitengine

import (
	"fmt"
	"math"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	register("anim", handleAnim)
	register("wa", handleWA)
	register("stopanim", handleStopAnim)
	register("xanim", handleAnim)
	register("stop_xanim", handleStopAnim)
	register("keyframe", handleKeyframe)
	register("endkeyframe", handleNoop)
	register("kanim", handleKanim)
	register("stop_kanim", handleStopAnim)
}

// animProp is one tweened attribute: apply is called every frame with the
// interpolated value between from and to.
type animProp struct {
	from, to float64
	apply    func(float64)
}

// animation drives a set of animProps in lockstep over one duration/easing
// curve — one [anim] call's worth of motion. Keyed by target object name in
// animations, advanced once per frame by stepAnimations (called from
// Update(), same pattern as audio's activeFades).
type animation struct {
	startTick, durTicks int
	ease                func(float64) float64
	props               []animProp
}

var animations = map[string]*animation{}

func stepAnimations() {
	for name, a := range animations {
		elapsed := t - a.startTick
		p := 1.0
		if a.durTicks > 0 {
			p = float64(elapsed) / float64(a.durTicks)
			if p > 1 {
				p = 1
			}
			if p < 0 {
				p = 0
			}
		}
		eased := p
		if a.ease != nil {
			eased = a.ease(p)
		}
		for _, prop := range a.props {
			prop.apply(prop.from + (prop.to-prop.from)*eased)
		}
		if p >= 1 {
			delete(animations, name)
		}
	}
}

// easingFunc returns nil for linear (accel="" or unrecognized).
func easingFunc(accel string) func(float64) float64 {
	switch accel {
	case "accelerate", "easein":
		return func(p float64) float64 { return p * p }
	case "decelerate", "easeout":
		return func(p float64) float64 { return 1 - (1-p)*(1-p) }
	case "easeinout":
		return func(p float64) float64 {
			if p < 0.5 {
				return 2 * p * p
			}
			return 1 - math.Pow(-2*p+2, 2)/2
		}
	default:
		return nil
	}
}

func findViewChara(name string) *kag3.CharaShow {
	for _, c := range viewCharas {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func findImageByName(name string) *kag3.Image {
	for _, img := range imgs {
		if img.Name == name {
			return img
		}
	}
	return nil
}

func findAnimatable(name string) kag3.Animatable {
	if c := findViewChara(name); c != nil {
		return c
	}
	if img := findImageByName(name); img != nil {
		return img
	}
	return nil
}

// buildAnimProps reads the subset of left/top/opacity/scalex/scaley/scale/
// rotation present in pm and turns each into an animProp reading/writing
// target through the shared kag3.Animatable interface — one code path for
// both characters and images.
func buildAnimProps(pm map[string]string, target kag3.Animatable) ([]animProp, error) {
	var props []animProp
	if v, ok := pm["left"]; ok {
		to, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: float64(target.GetLeft()), to: float64(to), apply: func(val float64) { target.SetLeft(int(val)) }})
	}
	if v, ok := pm["top"]; ok {
		to, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: float64(target.GetTop()), to: float64(to), apply: func(val float64) { target.SetTop(int(val)) }})
	}
	if v, ok := pm["opacity"]; ok {
		to, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: target.GetOpacity(), to: float64(to), apply: target.SetOpacity})
	}
	if v, ok := pm["scale"]; ok {
		to, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: target.GetScaleX(), to: to, apply: target.SetScaleX})
		props = append(props, animProp{from: target.GetScaleY(), to: to, apply: target.SetScaleY})
	}
	if v, ok := pm["scalex"]; ok {
		to, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: target.GetScaleX(), to: to, apply: target.SetScaleX})
	}
	if v, ok := pm["scaley"]; ok {
		to, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: target.GetScaleY(), to: to, apply: target.SetScaleY})
	}
	if v, ok := pm["rotation"]; ok {
		deg, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		props = append(props, animProp{from: target.GetRotation(), to: deg * math.Pi / 180, apply: target.SetRotation})
	}
	return props, nil
}

// handleAnim implements [anim name=... left= top= opacity= scale[xy]=
// rotation= time= accel= wait=], and doubles as [xanim] — Tyrano documents
// them as the same kind of generic tween, just [xanim] being the more
// "advanced" entry point; kag3 doesn't have a meaningful distinction to
// draw between the two given [anim] already accepts every property.
func handleAnim(ctx *tagCtx) error {
	object := ctx.tag
	name := object.Pm["name"]
	target := findAnimatable(name)
	if target == nil {
		return fmt.Errorf("そのオブジェクトが見つかりません name=%s", name)
	}

	timeMs := 1000
	if v, ok := object.Pm["time"]; ok {
		ms, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		timeMs = ms
	}
	props, err := buildAnimProps(object.Pm, target)
	if err != nil {
		return err
	}
	if len(props) == 0 {
		return nil
	}

	animations[name] = &animation{
		startTick: t,
		durTicks:  timeMs * ebiten.TPS() / 1000,
		ease:      easingFunc(object.Pm["accel"]),
		props:     props,
	}

	wait := true
	if v, ok := object.Pm["wait"]; ok {
		w, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		wait = w
	}
	if wait {
		waitForAnim(ctx, name)
	}
	return nil
}

func waitForAnim(ctx *tagCtx, name string) {
	ctx.y.Until(true, func() bool {
		_, ok := animations[name]
		return !ok
	})
}

// handleWA implements [wa name=...] (wait for a running [anim]/[kanim] to
// finish) and, with name omitted, waits for every running one.
func handleWA(ctx *tagCtx) error {
	name := ctx.tag.Pm["name"]
	if name != "" {
		waitForAnim(ctx, name)
		return nil
	}
	ctx.y.Until(true, func() bool {
		return len(animations) == 0
	})
	return nil
}

// handleStopAnim implements [stopanim]/[stop_xanim]/[stop_kanim]: freezes
// the named animation (or every one, if name is omitted) wherever it
// currently is, rather than jumping to its target.
func handleStopAnim(ctx *tagCtx) error {
	name := ctx.tag.Pm["name"]
	if name == "" {
		animations = map[string]*animation{}
		return nil
	}
	delete(animations, name)
	return nil
}

// frameSpec is one [frame] waypoint inside a [keyframe]...[endkeyframe]
// definition: the same attributes [anim] takes, plus its own time= (an
// absolute offset from the keyframe's start, not a duration).
type frameSpec struct {
	timeMs int
	props  map[string]string
}

var keyframes = map[string][]frameSpec{}

// handleKeyframe scans forward from [keyframe name=...] to the matching
// [endkeyframe], collecting each [frame]'s attributes, the same way
// [ignore] scans forward for [endignore] — no parser changes needed since
// a keyframe block only ever contains [frame] tags.
func handleKeyframe(ctx *tagCtx) error {
	r := ctx.r
	name := ctx.tag.Pm["name"]
	var frames []frameSpec
	idx := *ctx.i
	for idx+1 < len(r.scripts) {
		idx++
		tag, ok := r.scripts[idx].(kag3.TagObject)
		if !ok {
			continue
		}
		if tag.Name == "endkeyframe" {
			break
		}
		if tag.Name == "frame" {
			ms, err := strconv.Atoi(tag.Pm["time"])
			if err != nil {
				return err
			}
			frames = append(frames, frameSpec{timeMs: ms, props: tag.Pm})
		}
	}
	*ctx.i = idx - 1 // land on [endkeyframe] (or end of script) next iteration
	keyframes[name] = frames
	return nil
}

// handleKanim implements [kanim name=... keyframe=...]: plays a defined
// keyframe sequence on a target by chaining one animation per waypoint,
// each blocking until done before the next starts. Because the coroutine
// only ever runs one thing at a time, there's no way to also support a
// non-blocking kanim without a background goroutine, so unlike [anim] this
// is always blocking — a documented simplification.
func handleKanim(ctx *tagCtx) error {
	object := ctx.tag
	name := object.Pm["name"]
	target := findAnimatable(name)
	if target == nil {
		return fmt.Errorf("そのオブジェクトが見つかりません name=%s", name)
	}
	kfName := object.Pm["keyframe"]
	frames, ok := keyframes[kfName]
	if !ok {
		return fmt.Errorf("そのキーフレームは定義されてません keyframe=%s", kfName)
	}

	prevMs := 0
	for _, f := range frames {
		dur := f.timeMs - prevMs
		prevMs = f.timeMs
		props, err := buildAnimProps(f.props, target)
		if err != nil {
			return err
		}
		if len(props) == 0 {
			continue
		}
		animations[name] = &animation{
			startTick: t,
			durTicks:  dur * ebiten.TPS() / 1000,
			ease:      easingFunc(f.props["accel"]),
			props:     props,
		}
		waitForAnim(ctx, name)
	}
	return nil
}
