package kag3

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

type KS struct {
	isInScript bool
	ifCount int
}

type TextObject struct {
	Name string
	Line int
	Chara *CharacterInfo
	Val string
}

type LabelObject struct {
	Name string
	Val string
	Info LabelInfo
}

type TagObject struct {
	Name string
	Line int
	Pm map[string]string
	Val string
	ifCount int
}

type LabelInfo struct {
	Line int
	index int
	Name string
	Val string
}

type CharacterInfo struct {
	Name string
	Face string
}

type Background struct {
	Image     *ebiten.Image
	NextImage *ebiten.Image
	Time      int
	IsWait    bool
	IsCross   bool
	Position  string
	Method    string
}

type Character struct {
	Name  string
	JName string
	Faces map[string]string
	Image *ebiten.Image
}

type TextPosition struct {
	Layer string
	Page string
	Left int
	Top int
	Width int
	Height int
	BackImage *ebiten.Image
	FrameImage *ebiten.Image
	Color color.RGBA
	BorderColor color.RGBA
	BorderSize int
	Opacity int
	MarginLeft int
	MarginTop int
	MarginRight int
	MarginBottom int
	MarginN int
	Radius int
	Vertical bool
	Visible bool
	Gradient color.RGBA
}

type TextStyle struct {
	Size int
	Color *color.RGBA
	IsBold bool
	IsItaric bool
	Edge *color.RGBA
	Shadow *color.RGBA
}

var BackgroundMethod = []string{
	"crossfade",
	"explode",
	"slide",
	"blind",
	"bounce",
	"clip",
	"drop",
	"fold",
	"puff",
	"scale",
	"shake",
	"size",
	"fadeIn",
	"fadeInDown",
	"fadeInLeft",
	"fadeInRight",
	"fadeInUp",
	"lightSpeedIn",
	"rotateIn",
	"rotateInDownLeft",
	"rotateInDownRight",
	"rotateInUpLeft",
	"rotateInUpRight",
	"zoomIn",
	"zoomInDown",
	"zoomInLeft",
	"zoomInRight",
	"zoomInUp",
	"slideInDown",
	"slideInLeft",
	"slideInRight",
	"slideInUp",
	"bounceIn ",
	"bounceInDown",
	"bounceInLeft",
	"bounceInRight",
	"bounceInUp",
	"rollIn",
	"vanishIn",
	"puffIn",
}
