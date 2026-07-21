package effects

import (
	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
)

// BackgroundTransition is implemented by every kag3.BackgroundMethod's
// effect type (fade.go, slide.go, transitions.go).
type BackgroundTransition interface {
	DrawBackground(screen *ebiten.Image, bg *kag3.Background, baseTick, tick, time int)
}

// Transitions maps every kag3.BackgroundMethod name to its implementation,
// so callers do one map lookup instead of a long name switch.
var Transitions = map[string]BackgroundTransition{
	"fadeIn":            &FadeIn{},
	"crossfade":         &CrossFade{},
	"slide":             &SlideInRight{},
	"slideInRight":      &SlideInRight{},
	"explode":           &Explode{},
	"blind":             &Blind{},
	"bounce":            &Bounce{},
	"clip":              &Clip{},
	"drop":              &Drop{},
	"fold":              &Fold{},
	"puff":              &Puff{},
	"scale":             &Scale{},
	"shake":             &Shake{},
	"size":              &Size{},
	"fadeInDown":        &FadeInDown{},
	"fadeInLeft":        &FadeInLeft{},
	"fadeInRight":       &FadeInRight{},
	"fadeInUp":          &FadeInUp{},
	"lightSpeedIn":      &LightSpeedIn{},
	"rotateIn":          &RotateIn{},
	"rotateInDownLeft":  &RotateInDownLeft{},
	"rotateInDownRight": &RotateInDownRight{},
	"rotateInUpLeft":    &RotateInUpLeft{},
	"rotateInUpRight":   &RotateInUpRight{},
	"zoomIn":            &ZoomIn{},
	"zoomInDown":        &ZoomInDown{},
	"zoomInLeft":        &ZoomInLeft{},
	"zoomInRight":       &ZoomInRight{},
	"zoomInUp":          &ZoomInUp{},
	"slideInDown":       &SlideInDown{},
	"slideInLeft":       &SlideInLeft{},
	"slideInUp":         &SlideInUp{},
	"bounceIn":          &BounceIn{},
	"bounceInDown":      &BounceInDown{},
	"bounceInLeft":      &BounceInLeft{},
	"bounceInRight":     &BounceInRight{},
	"bounceInUp":        &BounceInUp{},
	"rollIn":            &RollIn{},
	"vanishIn":          &VanishIn{},
	"puffIn":            &PuffIn{},
}
