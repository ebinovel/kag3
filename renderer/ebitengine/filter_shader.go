package ebitengine

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// filterShaderSrc implements Tyrano's [filter]'s CSS-Filter-Effects-style
// adjustments (grayscale/sepia/saturate/hue-rotate/invert/brightness/
// contrast/blur) as a Kage fragment shader. The grayscale/sepia/hue-rotate
// coefficients are the standard matrices from the W3C Filter Effects Module
// Level 1 spec (https://www.w3.org/TR/filter-effects-1/#grayscaleEquivalent
// et al.) — the same ones browsers use for the equivalent CSS filter()
// functions — not an invented approximation. blur is a fixed 9x9 tap box
// blur (BlurRadius scales the tap spacing); a true separable Gaussian would
// need a second pass, which isn't worth the complexity for a visual-novel
// screen filter.
const filterShaderSrc = `
//go:build ignore

//kage:unit pixels

package main

var Grayscale float
var Sepia float
var Saturate float
var HueSin float
var HueCos float
var Invert float
var Brightness float
var Contrast float
var BlurRadius float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	var c vec4
	if BlurRadius > 0 {
		var sum vec4
		tap := BlurRadius / 4
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				sum += imageSrc0UnsafeAt(srcPos + vec2(float(dx), float(dy))*tap)
			}
		}
		c = sum / 81
	} else {
		c = imageSrc0UnsafeAt(srcPos)
	}

	// c is alpha-premultiplied (ebitengine's internal convention); the CSS
	// matrices below assume straight RGB, so un-premultiply, apply, then
	// re-premultiply.
	a := c.a
	if a <= 0 {
		return c
	}
	rgb := c.rgb / a

	if Grayscale > 0 {
		gs := vec3(
			dot(rgb, vec3(0.2126, 0.7152, 0.0722)),
			dot(rgb, vec3(0.2126, 0.7152, 0.0722)),
			dot(rgb, vec3(0.2126, 0.7152, 0.0722)),
		)
		rgb = mix(rgb, gs, Grayscale)
	}
	if Sepia > 0 {
		sp := vec3(
			dot(rgb, vec3(0.393, 0.769, 0.189)),
			dot(rgb, vec3(0.349, 0.686, 0.168)),
			dot(rgb, vec3(0.272, 0.534, 0.131)),
		)
		rgb = mix(rgb, sp, Sepia)
	}
	if Saturate != 1 {
		gray := dot(rgb, vec3(0.2126, 0.7152, 0.0722))
		rgb = mix(vec3(gray, gray, gray), rgb, Saturate)
	}
	if HueSin != 0 || HueCos != 1 {
		rgb = vec3(
			dot(rgb, vec3(0.213+HueCos*0.787-HueSin*0.213, 0.715-HueCos*0.715-HueSin*0.715, 0.072-HueCos*0.072+HueSin*0.928)),
			dot(rgb, vec3(0.213-HueCos*0.213+HueSin*0.143, 0.715+HueCos*0.285+HueSin*0.140, 0.072-HueCos*0.072-HueSin*0.283)),
			dot(rgb, vec3(0.213-HueCos*0.213-HueSin*0.787, 0.715-HueCos*0.715+HueSin*0.715, 0.072+HueCos*0.928+HueSin*0.072)),
		)
	}
	if Invert > 0 {
		rgb = mix(rgb, vec3(1, 1, 1)-rgb, Invert)
	}
	rgb *= Brightness
	rgb = (rgb-0.5)*Contrast + 0.5
	rgb = clamp(rgb, 0, 1)

	return vec4(rgb*a, a)
}
`

var filterShader *ebiten.Shader

func compiledFilterShader() *ebiten.Shader {
	if filterShader == nil {
		s, err := ebiten.NewShader([]byte(filterShaderSrc))
		if err != nil {
			panic(fmt.Sprintf("filter_shader.go: filterShaderSrc failed to compile: %v", err))
		}
		filterShader = s
	}
	return filterShader
}
