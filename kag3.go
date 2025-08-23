package kag3

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
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
	Index int
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
	IsEnd     bool
}

type BGM struct {
	Storage string
	Loop bool
	SpriteTime string
	Volume int
	Pause bool
	Seek int
	Restart bool
	Time int
	Player *audio.Player
}

type Button struct {
	Graphic *ebiten.Image
	Storage string
	Target string
	Name string
	X, Y, Width, Height int
	Fix bool
	Role string
	Hint string
	ClickSE string
	EnterSE string
	LeaveSE string
	ActiveImg string
	ClickImg string
	EnterImg *ebiten.Image
	AutoImg string
	SkipImg string
	Visible bool
	AutoNext bool
	SaveSnap bool
	KeyForcus int
}

type Character struct {
	Name  string
	JName string
	Faces map[string]string
	Image *ebiten.Image
}

type CharaShow struct {
	Name string
	Time int
	Layer int
	Zindex int
	Depth string
	Page string
	Wait bool
	Face string
	Storage string
	Reflect bool
	Width int
	Height int
	Left int
	Top int
	IsSlide bool
	NewLeft int
}


type Jump struct {
	Storage string
	Target string
}

type Macro struct {
	Name string
	Macro []any
}

type Link struct {
	Storage string
	Target string
	KeyForcus int
	Texts []TextObject
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

type GLink struct {
	Color *color.RGBA
	FontColor string
	Storage string
	Target string
	Name string
	Text string
	X, Y, Width, Height, Size int
	Face string
	Graphic string
	EnterImg string
	ClickSE string
	EnterSE string
	LeaveSE string
	ClearMessage bool
	Bold bool
	Opacity uint8
	Shadow string
	AutoPos bool
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
