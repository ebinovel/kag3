package kag3

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

type KS struct {
	isInScript bool
	ifCount    int
}

type TextObject struct {
	Name  string
	Line  int
	Chara *CharacterInfo
	Val   string
}

type LabelObject struct {
	Name string
	Val  string
	Info LabelInfo
}

type TagObject struct {
	Name string
	Line int
	Pm   map[string]string
	Val  string
	// Body holds the raw JavaScript source for an [iscript] tag, captured
	// between it and its matching [endscript].
	Body    string
	IfCount int
}

type LabelInfo struct {
	Line  int
	Index int
	Name  string
	Val   string
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
	IsSystem  bool
}

type BGM struct {
	Storage    string
	Loop       bool
	SpriteTime string
	Volume     int
	Pause      bool
	Seek       int
	Restart    bool
	Time       int
	Player     *audio.Player
}

type Button struct {
	Graphic             *ebiten.Image
	Storage             string
	Target              string
	Name                string
	X, Y, Width, Height int
	Fix                 bool
	Role                string
	Hint                string
	ClickSE             string
	EnterSE             string
	LeaveSE             string
	ActiveImg           string
	ClickImg            string
	EnterImg            *ebiten.Image
	AutoImg             string
	SkipImg             string
	Visible             bool
	AutoNext            bool
	SaveSnap            bool
	KeyForcus           int
}

type Character struct {
	Name  string
	JName string
	Faces map[string]string
	Image *ebiten.Image
	// Parts holds differential-part image variants, keyed by layer slot
	// (e.g. "face", "accessory") then part name (see [chara_layer]).
	Parts map[string]map[string]*ebiten.Image
	// ActivePart is which part name is currently showing for each layer
	// slot (see [chara_part]/[chara_part_reset]).
	ActivePart map[string]string
}

type CharaShow struct {
	Name     string
	Time     int
	Layer    int
	Zindex   int
	Depth    string
	Page     string
	Wait     bool
	Face     string
	Storage  string
	Reflect  bool
	Width    int
	Height   int
	Left     int
	Top      int
	IsSlide  bool
	NewLeft  int
	IsRemove bool
	// Opacity/ScaleX/ScaleY/Rotation are animatable via [anim]/[kanim] (see
	// tags_animation.go). Opacity is 0-255 like Tyrano's convention;
	// ScaleX/ScaleY default to 1, Rotation is radians.
	Opacity  float64
	ScaleX   float64
	ScaleY   float64
	Rotation float64
}

type Image struct {
	Image   *ebiten.Image
	Storage string
	Layer   string
	Page    string
	Visible bool
	Left    int
	Top     int
	X       int
	Y       int
	Width   int
	Height  int
	Folder  string
	Name    string
	Time    int
	IsWait  bool
	ZIndex  int
	Depth   string
	Reflect bool
	Pos     string
	AnimImg bool
	// Opacity/ScaleX/ScaleY/Rotation are animatable via [anim]/[kanim] (see
	// tags_animation.go). Same conventions as CharaShow's.
	Opacity  float64
	ScaleX   float64
	ScaleY   float64
	Rotation float64
}

type Jump struct {
	Storage string
	Target  string
}

type Macro struct {
	Name string
	Body []any
}

type Link struct {
	Storage   string
	Target    string
	KeyForcus int
	Texts     []TextObject
}

type LayOpt struct {
	Layer   string
	Page    string
	Visible bool
	Left    int
	Top     int
	Opacity int
}

type PText struct {
	Name      string
	Layer     string
	Page      string
	Text      string
	X         int
	Y         int
	Vertical  bool
	Size      int
	Face      string
	Color     *color.RGBA
	Bold      bool
	Edge      *color.RGBA
	Shadow    *color.RGBA
	Width     int
	Align     string
	Time      int
	Overwrite bool
	Gradient  string
}

type TextPosition struct {
	Layer        string
	Page         string
	Left         int
	Top          int
	Width        int
	Height       int
	BackImage    *ebiten.Image
	FrameImage   *ebiten.Image
	Color        color.RGBA
	BorderColor  color.RGBA
	BorderSize   int
	Opacity      int
	MarginLeft   int
	MarginTop    int
	MarginRight  int
	MarginBottom int
	MarginN      int
	Radius       int
	Vertical     bool
	Visible      bool
	Gradient     color.RGBA
}

type TextStyle struct {
	Size     int
	Color    *color.RGBA
	IsBold   bool
	IsItaric bool
	Edge     *color.RGBA
	Shadow   *color.RGBA
}

type GLink struct {
	Color                     *color.RGBA
	FontColor                 string
	Storage                   string
	Target                    string
	Name                      string
	Text                      string
	X, Y, Width, Height, Size int
	Face                      string
	Graphic                   string
	EnterImg                  string
	ClickSE                   string
	EnterSE                   string
	LeaveSE                   string
	ClearMessage              bool
	Bold                      bool
	Opacity                   uint8
	Shadow                    string
	AutoPos                   bool
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
	"bounceIn",
	"bounceInDown",
	"bounceInLeft",
	"bounceInRight",
	"bounceInUp",
	"rollIn",
	"vanishIn",
	"puffIn",
}

func (c *CharaShow) Remove(deleteTarget string) {
	if c.Name == deleteTarget {
		c.IsRemove = true
	}
}

// Animatable is implemented by CharaShow and Image so [anim]/[kanim] (see
// renderer/ebitengine/tags_animation.go) can tween either kind of target
// through one shared code path instead of duplicating attribute parsing
// per type.
type Animatable interface {
	GetLeft() int
	SetLeft(int)
	GetTop() int
	SetTop(int)
	GetOpacity() float64
	SetOpacity(float64)
	GetScaleX() float64
	SetScaleX(float64)
	GetScaleY() float64
	SetScaleY(float64)
	GetRotation() float64
	SetRotation(float64)
}

func (c *CharaShow) GetLeft() int          { return c.Left }
func (c *CharaShow) SetLeft(v int)         { c.Left = v }
func (c *CharaShow) GetTop() int           { return c.Top }
func (c *CharaShow) SetTop(v int)          { c.Top = v }
func (c *CharaShow) GetOpacity() float64   { return c.Opacity }
func (c *CharaShow) SetOpacity(v float64)  { c.Opacity = v }
func (c *CharaShow) GetScaleX() float64    { return c.ScaleX }
func (c *CharaShow) SetScaleX(v float64)   { c.ScaleX = v }
func (c *CharaShow) GetScaleY() float64    { return c.ScaleY }
func (c *CharaShow) SetScaleY(v float64)   { c.ScaleY = v }
func (c *CharaShow) GetRotation() float64  { return c.Rotation }
func (c *CharaShow) SetRotation(v float64) { c.Rotation = v }

// GetLeft/SetLeft/GetTop/SetTop map to X/Y, not the same-named Left/Top
// fields: drawScene's [image] rendering positions from X/Y, so those are
// the fields that actually need to move for [anim] to have any visible
// effect.
func (img *Image) GetLeft() int          { return img.X }
func (img *Image) SetLeft(v int)         { img.X = v }
func (img *Image) GetTop() int           { return img.Y }
func (img *Image) SetTop(v int)          { img.Y = v }
func (img *Image) GetOpacity() float64   { return img.Opacity }
func (img *Image) SetOpacity(v float64)  { img.Opacity = v }
func (img *Image) GetScaleX() float64    { return img.ScaleX }
func (img *Image) SetScaleX(v float64)   { img.ScaleX = v }
func (img *Image) GetScaleY() float64    { return img.ScaleY }
func (img *Image) SetScaleY(v float64)   { img.ScaleY = v }
func (img *Image) GetRotation() float64  { return img.Rotation }
func (img *Image) SetRotation(v float64) { img.Rotation = v }
