package ebitengine

import (
	"fmt"
	"image/color"
	_ "image/png"
	"io/fs"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
	"github.com/hajimehoshi/ebiten/v2"

	//"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Text struct {
	Text string
	TextStyle *kag3.TextStyle
}

type Renderer struct {
	scripts []interface{}
	fontFace *text.GoTextFace
	nameFontFace *text.GoTextFace
	fses map[string]fs.FS
	texts map[int][]Text
	line int
}

var (
	isFirst bool
	isWait bool
	c *coro.Coro
	loop func(y coro.Yield)
	isClicked func() bool
	t, tick, oldTick int
	charas map[string]*kag3.Character
	viewCharas map[string]string
	bgImage *ebiten.Image
	textPosition *kag3.TextPosition
	textStyle *kag3.TextStyle
	beforeTextSize float64
	textGlyphs []text.Glyph
	charaName string
)

func init() {
	charas = make(map[string]*kag3.Character)
	viewCharas = make(map[string]string)
	textPosition = &kag3.TextPosition{}
	isClicked = func () bool  {
		return oldTick + 5 >= tick
	}
}

func NewRenderer(scripts []interface{}, fontFace *text.GoTextFace, fses map[string]fs.FS) (r *Renderer, err error) {
	r = &Renderer{
		scripts: scripts,
		fontFace: fontFace,
		nameFontFace: &text.GoTextFace{
			Source: fontFace.Source,
			Size: fontFace.Size,
			Language: fontFace.Language,
		},
		fses: fses,
	}
	r.initScript()
	beforeTextSize = fontFace.Size
	r.texts = make(map[int][]Text)
	return
}

func (r *Renderer) Update() {
	t++
	if !isFirst {
		go func() {
			co := coro.New(loop)
			for co.Next() {
				tick++
			}
		}()
		isFirst = true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			isWait = false
			oldTick = tick
	}
}

func (r *Renderer) initScript() {
	loop = func(y coro.Yield) {
		var err error
		for _, s := range r.scripts {
			fmt.Printf("%+v\n", s)
			switch s.(type) {
			case kag3.CharacterInfo:
			case kag3.TextObject:
				textObject := s.(kag3.TextObject)
				r.line = textObject.Line
				if textObject.Chara != nil {
					fmt.Println(textObject.Chara)
					charaName = textObject.Chara.Name
				}
				if len(r.texts[textObject.Line]) == 0 {
					r.texts[textObject.Line] = append(
						r.texts[textObject.Line],
						Text{Text: textObject.Val},
					)
				} else {
					if r.texts[textObject.Line][len(r.texts[textObject.Line])-1].Text == "" {
						r.texts[textObject.Line][len(r.texts[textObject.Line])-1].Text = textObject.Val
					} else {
						r.texts[textObject.Line] = append(
							r.texts[textObject.Line],
							Text{Text: textObject.Val},
						)
					}
				}
				y()
			case kag3.TagObject:
				tagObject := s.(kag3.TagObject)
				r.line = tagObject.Line
				switch tagObject.Name {
				case "bg":
					bgImage, _, err = ebitenutil.NewImageFromFileSystem(
						r.fses["images"],
						tagObject.Pm["storage"],
					)
					if err != nil {
						panic(err)
					}
				case "chara_new":
					charaImage, _, err := ebitenutil.NewImageFromFileSystem(
						r.fses["images"],
						tagObject.Pm["storage"],
					)
					if err != nil {
						panic(err)
					}
					charas[tagObject.Pm["name"]] = &kag3.Character{
						Name: tagObject.Pm["name"],
						Image: charaImage,
						JName: tagObject.Pm["jname"],
						Faces: make(map[string]string),
					}
					charas[tagObject.Pm["name"]].Faces["default"] = tagObject.Pm["storage"]
					fmt.Printf("chara:%+v\n", charas)
				case "chara_hide":
					delete(viewCharas, tagObject.Pm["name"])
				case "chara_face":
					charas[tagObject.Pm["name"]].Faces[tagObject.Pm["face"]] = tagObject.Pm["storage"]
					fmt.Printf("chara:%+v\n", charas)
				case "chara_mod":
					charaImage, _, err := ebitenutil.NewImageFromFileSystem(
						r.fses["images"],
						charas[tagObject.Pm["name"]].Faces[tagObject.Pm["face"]],
					)
					if err != nil {
						panic(err)
					}
					charas[tagObject.Pm["name"]].Image = charaImage
				case "chara_show":
					viewCharas[tagObject.Pm["name"]] = tagObject.Pm["name"]
				case "font":
					err = r.textStyle(tagObject)
					if err != nil {
						panic(err)
					}
				case "p":
					y.Until(true, isClicked)
				case "ptext":

				case "position":
					err = r.position(tagObject)
					if err != nil {
						panic(err)
					}
				case "resetfont":
					textStyle = nil
				}
			}
			y()
		}
	}
}

func (r *Renderer) position(tagObject kag3.TagObject) (err error){
	for key, value := range tagObject.Pm {
		switch key {
		case "layer":
			textPosition.Layer = value
		case "page":
			textPosition.Page = value
		case "left":
			textPosition.Left, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "top":
			textPosition.Top, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "width":
			textPosition.Width, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "height":
			textPosition.Height, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "frame":
			textPosition.FrameImage, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses["images"],
				tagObject.Pm["frame"],
			)
			if err != nil {
				return err
			}
		case "color", "border_color":
			var r, g, b int
			r, g, b, err = parseColor(value)
			if key == "color" {
				textPosition.Color = color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			} else {
				textPosition.BorderColor = color.RGBA{uint8(r), uint8(g), uint8(b), 0}
			}
		case "border_size":
			textPosition.BorderSize, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "opacity":
			textPosition.Opacity, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "marginl":
			textPosition.MarginLeft, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "margint":
			textPosition.MarginTop, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "marginr":
			textPosition.MarginRight, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "marginb":
			textPosition.MarginBottom, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "marginn":
			textPosition.MarginN, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "radius":
			textPosition.Radius, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		case "vertial":
			textPosition.Vertical = (value == "true")
		case "visible":
			textPosition.Visible = (value == "true")
		}
	}
	if textPosition.Width != 0 && textPosition.Height != 0 {
		textPosition.BackImage = ebiten.NewImage(textPosition.Width, textPosition.Height)
	}
	return nil
}

func (r *Renderer) textStyle(tagObject kag3.TagObject) (err error) {
	if textStyle == nil {
		textStyle = &kag3.TextStyle{}
	}
	for key, value := range tagObject.Pm {
		switch key {
		case "size":
			textStyle.Size, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "color", "edge", "shadow":
			var r, g, b int
			r, g, b, err = parseColor(value)
			if key == "color" {
				textStyle.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			} else if key == "edge" {
				textStyle.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			} else {
				textStyle.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			}
		case "bold":
			textStyle.IsBold = true
		case "itaric":
			textStyle.IsItaric = true
		}
	}
	r.texts[tagObject.Line] = append(r.texts[tagObject.Line], Text{TextStyle: textStyle})
	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	screenWidth, screenHeight := screen.Bounds().Dx(), screen.Bounds().Dy()
	//_, h := text.Measure(r.text, fontFace, 0)
	if bgImage != nil {
		screen.DrawImage(bgImage, &ebiten.DrawImageOptions{})
	}
	{
		for _, chara := range viewCharas {
			charaOp := &ebiten.DrawImageOptions{}
			charaLeft := (screenWidth - charas[chara].Image.Bounds().Dx()) / 2
			charaTop := (screenHeight - charas[chara].Image.Bounds().Dy())
			charaOp.GeoM.Translate(float64(charaLeft), float64(charaTop))
			screen.DrawImage(charas[chara].Image, charaOp)
		}
	}
	if textPosition != nil && textPosition.Visible {
		op := &ebiten.DrawImageOptions{}
		x, y := float64(textPosition.Left), float64(textPosition.Top)
		op.GeoM.Translate(x, y)
		if textPosition.FrameImage != nil {
			op.ColorScale.SetA(float32(textPosition.Opacity))
			screen.DrawImage(textPosition.FrameImage, op)
		} else {
			textPosition.BackImage.Fill(color.RGBA{0, 0, 0, 128})
			screen.DrawImage(textPosition.BackImage, op)
		}
		cPTextOp := &text.DrawOptions{}
		cPTextOp.GeoM.Translate(180, 510)
		cPTextOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, charaName, r.nameFontFace, cPTextOp)

		marginLeft := x + float64(textPosition.MarginLeft)
		marginTop := y + float64(textPosition.MarginTop)
		count := t / 5
		multiCount := 0
		multiWidth := 0.0
		if len(r.texts[r.line]) != 0 {
			for _, v := range r.texts[r.line] {
				if len(r.texts[r.line]) > 1 {
					multiCount++
				}
				if len(v.Text) == 0 {
					continue
				}
				tOp := &text.DrawOptions{}
				tOp.LineSpacing = r.fontFace.Size
				w, _ := text.Measure(v.Text, r.fontFace, tOp.LineSpacing)
				if multiCount > 1 {
					fmt.Printf("text:%+v, width:%+v\n", v.Text, w)
					multiWidth += w
				}
				maxWidth := float64(textPosition.Width - textPosition.MarginRight)
				if w > maxWidth {
					rn := []rune(v.Text)
					key := len(rn) - 1
					for v := key; v >= 0; v-- {
						ww, _ := text.Measure(string(rn[:v]), r.fontFace, tOp.LineSpacing)
						if ww <= maxWidth {
							rn = append(rn[:v+1], rn[v:]...)
							rn[v] = []rune("\n")[0]
							break
						}
					}
					v.Text = string(rn)
				}
				glyph := text.AppendGlyphs(
					textGlyphs,
					v.Text,
					r.fontFace,
					&tOp.LayoutOptions,
				)
				if textStyle != nil {
					if textStyle.Size != 0 && float64(textStyle.Size) != beforeTextSize {
						r.fontFace.Size = float64(textStyle.Size)
					}
					if textStyle.Color != nil {
						tOp.ColorScale.ScaleWithColor(textStyle.Color)
					}
				} else {
					if v.TextStyle != nil {
						if v.TextStyle.Size != 0 && float64(v.TextStyle.Size) != beforeTextSize {
							r.fontFace.Size = float64(v.TextStyle.Size)
						}
						if v.TextStyle.Color != nil {
							tOp.ColorScale.ScaleWithColor(v.TextStyle.Color)
						}
					} else {
						r.fontFace.Size = beforeTextSize
						tOp.ColorScale.ScaleWithColor(color.White)
					}
				}
				if !isWait {
					count = count % len(glyph)
				}
				if count >= len(glyph) - 1 {
					isWait = true
					tOp.GeoM.Reset()
					tOp.GeoM.Translate(marginLeft + multiWidth, marginTop)
					text.Draw(screen, v.Text, r.fontFace, tOp)
				} else {
					for _, g := range glyph[:count] {
						tOp.GeoM.Reset()
						tOp.GeoM.Translate(marginLeft, marginTop)
						tOp.GeoM.Translate(g.X, g.Y)
						screen.DrawImage(g.Image, &tOp.DrawImageOptions)
					}
				}
				textGlyphs = []text.Glyph{}
				ebitenutil.DebugPrint(screen, fmt.Sprintf("w:%+v, maxWidth:%+v\n", w, maxWidth))
			}
		}
	}
}

func parseColor(value string) (r, g, b int, err error) {
	if value == "red" {
		return 255, 0, 0, nil
	}
	runes := []rune(value)
	rStr, gStr, bStr := string(runes[2:3]), string(runes[4:5]), string(runes[6:7])
	r, err = strconv.Atoi(rStr)
	if err != nil {
		return
	}
	g, err = strconv.Atoi(gStr)
	if err != nil {
		return
	}
	b, err = strconv.Atoi(bStr)
	if err != nil {
		return
	}
	return
}
