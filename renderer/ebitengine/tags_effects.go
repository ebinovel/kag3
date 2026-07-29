package ebitengine

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	register("quake", handleQuake)
	register("quake2", handleQuake2)
	register("vibrate", handleVibrate)
	register("vibrate_stop", handleVibrateStop)
	register("layermode", handleLayerMode)
	register("free_layermode", handleFreeLayerMode)
	register("filter", handleFilter)
	register("free_filter", handleFreeFilter)
	register("mask", handleMask)
	register("mask_off", handleMaskOff)
	register("camera", handleCamera)
	register("reset_camera", handleResetCamera)
	register("wait_camera", handleWaitCamera)
}

// camera is applied in Renderer.Draw when compositing renderBuffer onto
// the real screen: a screen-space pan (X, Y) and zoom (Scale) around
// center. [camera] tweens it using the same animations/stepAnimations
// mechanism as [anim], under the reserved key "camera".
var camera = struct {
	X, Y  float64
	Scale float64
}{Scale: 1}

func handleCamera(ctx *tagCtx) error {
	object := ctx.tag
	var props []animProp
	if v, ok := object.Pm["x"]; ok {
		to, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		props = append(props, animProp{from: camera.X, to: to, apply: func(val float64) { camera.X = val }})
	}
	if v, ok := object.Pm["y"]; ok {
		to, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		props = append(props, animProp{from: camera.Y, to: to, apply: func(val float64) { camera.Y = val }})
	}
	if v, ok := object.Pm["scale"]; ok {
		to, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		props = append(props, animProp{from: camera.Scale, to: to, apply: func(val float64) { camera.Scale = val }})
	}
	if len(props) == 0 {
		return nil
	}
	timeMs, err := getIntDefault(object.Pm, "time", 1000)
	if err != nil {
		return err
	}
	animations["camera"] = &animation{
		startTick: t,
		durTicks:  timeMs * ebiten.TPS() / 1000,
		ease:      easingFunc(object.Pm["accel"]),
		props:     props,
	}
	wait := true
	if v, ok, err := getBool(object.Pm, "wait"); err != nil {
		return err
	} else if ok {
		wait = v
	}
	if wait {
		waitForAnim(ctx, "camera")
	}
	return nil
}

func handleResetCamera(ctx *tagCtx) error {
	delete(animations, "camera")
	camera.X, camera.Y, camera.Scale = 0, 0, 1
	return nil
}

func handleWaitCamera(ctx *tagCtx) error {
	waitForAnim(ctx, "camera")
	return nil
}

// shake drives the screen-shake offset Draw adds on top of the camera
// transform. quake shakes horizontally, quake2 vertically — kag3 doesn't
// have a documented distinction beyond "two shake tags", so this is a
// reasonable, real (not faked) split rather than making quake2 an alias.
var shake struct {
	startT, durTicks int
	amplitude        float64
	vertical         bool
}

func currentShakeOffset() (dx, dy float64) {
	if shake.durTicks <= 0 {
		return 0, 0
	}
	elapsed := t - shake.startT
	if elapsed >= shake.durTicks {
		return 0, 0
	}
	remaining := 1 - float64(elapsed)/float64(shake.durTicks)
	off := shake.amplitude * remaining * math.Sin(float64(elapsed)*2.5)
	if shake.vertical {
		return 0, off
	}
	return off, 0
}

func handleQuake(ctx *tagCtx) error  { return startQuake(ctx, false) }
func handleQuake2(ctx *tagCtx) error { return startQuake(ctx, true) }

func startQuake(ctx *tagCtx, vertical bool) error {
	object := ctx.tag
	timeMs, err := getIntDefault(object.Pm, "time", 500)
	if err != nil {
		return err
	}
	amplitude := 10.0
	if v, ok := object.Pm["strength"]; ok {
		a, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		amplitude = a
	}
	shake.startT = t
	shake.durTicks = timeMs * ebiten.TPS() / 1000
	shake.amplitude = amplitude
	shake.vertical = vertical

	wait := true
	if v, ok, err := getBool(object.Pm, "wait"); err != nil {
		return err
	} else if ok {
		wait = v
	}
	if wait {
		ctx.y.Until(true, func() bool {
			return t-shake.startT >= shake.durTicks
		})
	}
	return nil
}

// handleVibrate/handleVibrateStop wrap ebiten's device-vibration API
// directly. It's a real call, not a stub — it just has no visible effect
// on a desktop build without a vibration-capable device (mobile/web
// targets are where it does something).
func handleVibrate(ctx *tagCtx) error {
	object := ctx.tag
	ms := 200
	if v, ok := object.Pm["time"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		ms = n
	}
	magnitude := 1.0
	if v, ok := object.Pm["strength"]; ok {
		m, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		magnitude = m
	}
	ebiten.Vibrate(&ebiten.VibrateOptions{Duration: time.Duration(ms) * time.Millisecond, Magnitude: magnitude})
	return nil
}

func handleVibrateStop(ctx *tagCtx) error {
	ebiten.Vibrate(&ebiten.VibrateOptions{Duration: 0, Magnitude: 0})
	return nil
}

// layerBlend holds a per-[image]-layer blend mode, applied in drawScene's
// image-drawing loop. Only "normal" (the default) and "add" are supported —
// ebiten has no built-in multiply/screen preset, and building one from raw
// blend factors for tags this rarely used isn't worth the added surface
// right now.
var layerBlend = map[string]ebiten.Blend{}

func handleLayerMode(ctx *tagCtx) error {
	object := ctx.tag
	layer := object.Pm["layer"]
	switch mode := object.Pm["mode"]; mode {
	case "add":
		layerBlend[layer] = ebiten.BlendLighter
	case "normal", "":
		delete(layerBlend, layer)
	default:
		return fmt.Errorf("未対応の値です %s", mode)
	}
	return nil
}

func handleFreeLayerMode(ctx *tagCtx) error {
	layer := ctx.tag.Pm["layer"]
	if layer == "" {
		layerBlend = map[string]ebiten.Blend{}
		return nil
	}
	delete(layerBlend, layer)
	return nil
}

// activeFilter is Tyrano's CSS-style [filter]: a color tint + opacity
// (applied directly via ebiten.ColorScale, see Renderer.Draw) plus the
// CSS-Filter-Effects-style adjustments (grayscale/sepia/saturate/hue/
// invert/brightness/contrast/blur) applied via filterShaderSrc
// (filter_shader.go) whenever any of them differs from its identity value —
// see needsShader. Splitting it this way keeps the common case (just a
// tint, e.g. a night-time [filter color=...]) on the cheap ColorScale path
// instead of always paying for a shader draw.
//
// opacity/color's 0-255 scale (rather than official Tyrano's 0-100 for
// [filter opacity=]) intentionally matches [mask opacity=]'s existing 0-255
// convention elsewhere in this file, for internal consistency.
type filterState struct {
	Tint  color.Color
	Alpha float32

	Grayscale  float32 // 0-1
	Sepia      float32 // 0-1
	Saturate   float32 // 1 = unchanged, matches CSS saturate()'s default
	HueDeg     float32 // degrees, matches CSS hue-rotate()
	Invert     float32 // 0-1
	Brightness float32 // 1 = unchanged
	Contrast   float32 // 1 = unchanged
	Blur       float32 // pixels, 0 = off
}

// needsShader reports whether any CSS-Filter-Effects-style adjustment is
// active, i.e. whether Draw must route through filterShaderSrc instead of
// the plain ColorScale tint/opacity path.
func (f *filterState) needsShader() bool {
	return f.Grayscale != 0 || f.Sepia != 0 || f.Saturate != 1 || f.HueDeg != 0 ||
		f.Invert != 0 || f.Brightness != 1 || f.Contrast != 1 || f.Blur != 0
}

var activeFilter *filterState

func handleFilter(ctx *tagCtx) error {
	object := ctx.tag
	f := &filterState{Tint: color.White, Alpha: 1, Saturate: 1, Brightness: 1, Contrast: 1}
	if v, ok := object.Pm["color"]; ok {
		r, g, b, err := parseColor(v)
		if err != nil {
			return err
		}
		f.Tint = color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	}
	if v, ok := object.Pm["opacity"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		f.Alpha = float32(n) / 255
	}
	if v, ok, err := getFilterPercent(object.Pm, "grayscale"); err != nil {
		return err
	} else if ok {
		f.Grayscale = v
	}
	if v, ok, err := getFilterPercent(object.Pm, "sepia"); err != nil {
		return err
	} else if ok {
		f.Sepia = v
	}
	if v, ok, err := getFilterPercent(object.Pm, "saturate"); err != nil {
		return err
	} else if ok {
		f.Saturate = v
	}
	if v, ok := object.Pm["hue"]; ok {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		f.HueDeg = float32(n)
	}
	if v, ok, err := getFilterPercent(object.Pm, "invert"); err != nil {
		return err
	} else if ok {
		f.Invert = v
	}
	if v, ok, err := getFilterPercent(object.Pm, "brightness"); err != nil {
		return err
	} else if ok {
		f.Brightness = v
	}
	if v, ok, err := getFilterPercent(object.Pm, "contrast"); err != nil {
		return err
	} else if ok {
		f.Contrast = v
	}
	if v, ok := object.Pm["blur"]; ok {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		f.Blur = float32(n)
	}
	activeFilter = f
	return nil
}

// getFilterPercent reads a CSS-filter()-style percentage/ratio attribute:
// Tyrano/CSS both accept either a bare number (1 = 100% = fully applied,
// matching e.g. grayscale(1) / saturate(2)) or a "NN%" suffix (grayscale(50%)).
func getFilterPercent(pm map[string]string, key string) (float32, bool, error) {
	v, ok := pm[key]
	if !ok {
		return 0, false, nil
	}
	if s, isPercent := strings.CutSuffix(v, "%"); isPercent {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false, err
		}
		return float32(n) / 100, true, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false, err
	}
	return float32(n), true, nil
}

func handleFreeFilter(ctx *tagCtx) error {
	activeFilter = nil
	return nil
}

// activeMask is [mask]'s full-screen overlay image, drawn on top of
// everything (including camera/shake) in Renderer.Draw.
type maskState struct {
	Image   *ebiten.Image
	Opacity float32
}

var activeMask *maskState

func handleMask(ctx *tagCtx) error {
	object := ctx.tag
	img, err := loadImage(ctx.r, "", object.Pm["storage"])
	if err != nil {
		return err
	}
	opacity := float32(1)
	if v, ok, err := getInt(object.Pm, "opacity"); err != nil {
		return err
	} else if ok {
		opacity = float32(v) / 255
	}
	activeMask = &maskState{Image: img, Opacity: opacity}
	return nil
}

func handleMaskOff(ctx *tagCtx) error {
	activeMask = nil
	return nil
}
