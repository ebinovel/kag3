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
	// Storage is the file path of whatever's currently requested into
	// Image/NextImage (see applyBGTag in tags_background.go). Neither
	// *ebiten.Image field can round-trip through a JSON save file, so this
	// is what save/load uses to reconstruct the background after a fresh
	// process start.
	Storage string
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
	// Exp/PreExp are raw JS run on click, before Target/Role take effect —
	// e.g. config.ks's volume buttons use exp to set tf.current_bgm_vol
	// ahead of jumping to *vol_bgm_change, which reads it. PreExp runs
	// first, if set, with its result bound to a "preexp" variable Exp can
	// reference (see tyrano.ks's CG-gallery buttons).
	Exp    string
	PreExp string
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
	// Storage is the file path of whatever's currently in Image (set by
	// [chara_new], kept in sync by [chara_mod]). *ebiten.Image can't
	// round-trip through a JSON save file, so save/load uses this to
	// re-register the character in a fresh process that never ran
	// [chara_new] for it.
	Storage string
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
	// BgStorage/BgImage back a [ptext bg="..."] background image, drawn
	// behind the text at (X, Y) — see drawPTexts (renderer.go). Nil unless
	// bg= was given; ordinary ptext areas keep drawing text-only exactly as
	// before. BgImage is excluded from JSON (save/load, tags_save.go) —
	// same reasoning as TextPosition.BackImage/FrameImage below: an
	// *ebiten.Image can't round-trip through a JSON save file, and letting
	// encoding/json serialize/deserialize it anyway produced a zero-value
	// Image indistinguishable from a disposed one, crashing drawPTexts's
	// DrawImage on the very first load. applySaveData reloads it from
	// BgStorage after restoring Ptexts, the same pattern Background.Storage/
	// TextPosition.FrameStorage already use.
	BgStorage string
	BgImage   *ebiten.Image `json:"-"`
}

type TextPosition struct {
	Layer      string
	Page       string
	Left       int
	Top        int
	Width      int
	Height     int
	BackImage  *ebiten.Image
	FrameImage *ebiten.Image
	// FrameStorage is the file path FrameImage was loaded from (see
	// [position frame=...] in renderer.go). *ebiten.Image can't round-trip
	// through a JSON save file, so save/load uses this to reconstruct
	// FrameImage after a fresh process start, same idea as Background.Storage.
	FrameStorage string
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
	// FilterColor is [position_filter]'s override for the message window's
	// backing fill (nil means the renderer's built-in default).
	FilterColor *color.RGBA
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
