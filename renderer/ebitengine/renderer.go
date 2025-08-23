package ebitengine

import (
	"bytes"
	"fmt"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"slices"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/ebinovel/kag3/renderer/ebitengine/effects"
	"github.com/eihigh/coro"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Text struct {
	Text string
	TextStyle *kag3.TextStyle
}

type Renderer struct {
	manager *kag3.Manager
	scripts []any
	labels map[string]kag3.LabelInfo
	fontFace *text.GoTextFace
	nameFontFace *text.GoTextFace
	fses map[string]fs.FS
	texts map[int][]Text
	line int
}

var (
	isFirst bool
	isWait bool
	loop func(y coro.Yield)
	isClicked func() bool
	t, tick, oldTick, bgTick, charaTick, bgmTick int
	charas map[string]*kag3.Character
	viewCharas []*kag3.CharaShow
	bg *kag3.Background
	textPosition *kag3.TextPosition
	textStyle *kag3.TextStyle
	beforeTextSize float64
	textGlyphs []text.Glyph
	charaName string
	buttons []*kag3.Button
	imgs []*kag3.Image
	glinks []*kag3.GLink
	links []*kag3.Link
	isJump bool
	isJumped func() bool
	jumpIndex int
	audioContext *audio.Context
)

func init() {
	charas = make(map[string]*kag3.Character)
	textPosition = &kag3.TextPosition{}
	audioContext = audio.NewContext(44100)
	bg = &kag3.Background{
		Time: 3000,
		IsWait: true,
		Method: "crossfade",
	}
	isClicked = func () bool  {
		return oldTick + 5 >= tick
	}
	isJumped = func() bool {
		return isJump
	}
}

func NewRenderer(manager *kag3.Manager) (r *Renderer, err error) {
	r = &Renderer{
		manager: manager,
		scripts: manager.Senario,
		labels: manager.Labels,
		fontFace: manager.FontFace,
		nameFontFace: &text.GoTextFace{
			Source: manager.FontFace.Source,
			Size: manager.FontFace.Size,
			Language: manager.FontFace.Language,
		},
		fses: manager.FSes,
	}
	r.initScript()
	img := ebiten.NewImage(manager.Config.ScreenWidth, manager.Config.ScreenHeight)
	img.Fill(color.Black)
	bg.Image = img
	beforeTextSize = manager.FontFace.Size
	r.texts = make(map[int][]Text)
	return
}

func doNext() bool {
	return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsKeyJustPressed(ebiten.KeyEnter)
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
	if doNext() {
		isWait = false
		oldTick = tick
	}
	for i, link := range links {
		mX, mY := ebiten.CursorPosition()
		for j, t := range link.Texts {
			x, y := textPosition.Left, textPosition.Top
			w, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + textPosition.MarginLeft
			marginTop := y + textPosition.MarginTop + int(h) * (i + j)
			if isColision(mX, mY, marginLeft, marginTop, int(w), int(h)) {
				//fmt.Println("isCollsion", mX, mY)
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					if link.Storage != "" {
						r.manager.LoadScript(link.Storage)
						r.labels = r.manager.Labels
						r.scripts = r.manager.Senario
					}
					if v, ok := r.labels[link.Target[1:]]; ok {
						fmt.Printf("click label:%+v\n", v)
						jumpIndex = v.Index
						isJump = true
					}
				}
			}
		}
	}
	for _, glink := range glinks {
		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, glink.X, glink.Y, glink.Width, glink.Height) {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if glink.Storage != "" {
					r.manager.LoadScript(glink.Storage)
					r.labels = r.manager.Labels
					r.scripts = r.manager.Senario
				}
				if v, ok := r.labels[glink.Target]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
				if v, ok := r.labels[glink.Target[1:]]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
			}
		}
	}
	for _, button := range buttons {
		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, button.X, button.Y, button.Width, button.Height) {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				fmt.Println("click")
				if button.Storage != "" {
					fmt.Printf("button.Storage:%+v\n", button.Storage)
					r.manager.LoadScript(button.Storage)
					r.labels = r.manager.Labels
					r.scripts = r.manager.Senario
					if button.Target == "" {
						jumpIndex = 0
						isJump = true
					}
				}
				if button.Role != "" {
					switch button.Role {
					case "save":
					case "load":
					case "title":
					}
				}
				if v, ok := r.labels[button.Target]; ok {
					fmt.Printf("label:%+v\n", v)
					jumpIndex = v.Index
					isJump = true
				}
			}
		}
	}
	if isJump {
		glinks = nil
		buttons = nil
		links = nil
	}
}

func (r *Renderer) initScript() {
	loop = func(y coro.Yield) {
		var err error
		for i := 0; i < len(r.scripts); i++ {
			if isJump {
				i = jumpIndex
				isJump = false
			}
			s := r.scripts[i]
			fmt.Printf("Index:%d, %+v\n", i, s)
			switch object := s.(type) {
			case kag3.CharacterInfo:
			case kag3.TextObject:
				r.line = object.Line
				if object.Chara != nil {
					fmt.Println(object.Chara)
					charaName = object.Chara.Name
				}
				if len(r.texts[object.Line]) == 0 {
					r.texts[object.Line] = append(
						r.texts[object.Line],
						Text{Text: object.Val},
					)
				} else {
					if r.texts[object.Line][len(r.texts[object.Line])-1].Text == "" {
						r.texts[object.Line][len(r.texts[object.Line])-1].Text = object.Val
					} else {
						r.texts[object.Line] = append(
							r.texts[object.Line],
							Text{Text: object.Val},
						)
					}
				}
				y()
			case kag3.TagObject:
				r.line = object.Line
				switch object.Name {
				case "bg":
					fmt.Printf("bg:%+v\n", bg)
					bgTick = t
					images := "images"
					for key, value := range object.Pm {
						switch key {
						case "time":
							bg.Time, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "wait":
							switch value {
							case "true":
								bg.IsWait = true
							case "false":
								bg.IsWait = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "cross":
							switch value {
							case "true":
								bg.IsCross = true
							case "false":
								bg.IsCross = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "position":
							switch value {
							case "left", "center", "right", "top", "bottom":
								bg.Position = value
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "method":
							if slices.Contains(kag3.BackgroundMethod, value) {
								bg.Method = value
							} else {
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "system":
							switch value {
							case "true":
								bg.IsSystem = true
								images = "system/images"
							case "false":
								bg.IsSystem = false
								images = "images"
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						}
					}
					bg.NextImage, _, err = ebitenutil.NewImageFromFileSystem(
						r.fses[images],
						object.Pm["storage"],
					)
					if err != nil {
						panic(err)
					}
					if bg.IsWait {
						y.Until(true, func() bool {
							return bg.IsEnd
						})
					}
				case "button":
					err = r.button(object)
					if err != nil {
						panic(err)
					}
				case "chara_new":
					name := object.Pm["name"]
					charaImage, _, err := ebitenutil.NewImageFromFileSystem(
						r.fses["images"],
						object.Pm["storage"],
					)
					if err != nil {
						panic(err)
					}
					charas[name] = &kag3.Character{
						Name: name,
						Image: charaImage,
						JName: object.Pm["jname"],
						Faces: make(map[string]string),
					}
					charas[name].Faces["default"] = object.Pm["storage"]
				case "chara_hide":
					viewCharas = removeCharaShow(viewCharas, object.Pm["name"])
				case "chara_face":
					charas[object.Pm["name"]].Faces[object.Pm["face"]] = object.Pm["storage"]
				case "chara_mod":
					charaImage, _, err := ebitenutil.NewImageFromFileSystem(
						r.fses["images"],
						charas[object.Pm["name"]].Faces[object.Pm["face"]],
					)
					if err != nil {
						panic(err)
					}
					charas[object.Pm["name"]].Image = charaImage
				case "chara_show":
					charaTick = t
					name := object.Pm["name"]
					if _, ok := charas[name]; !ok {
						panic(fmt.Errorf("そのキャラクターは登録されてません name=%s", name))
					}
					chara := &kag3.CharaShow{}
					chara.Wait = true
					chara.Time = 1000
					for key, value := range object.Pm {
						switch key {
						case "name":
							chara.Name = value
						case "time":
							chara.Time, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "zindex":
							chara.Zindex, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "depth":
							chara.Depth = value
						case "page":
							chara.Page = value
						case "wait":
							switch value {
							case "true":
								chara.Wait = true
							case "false":
								chara.Wait = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "face":
							if v, ok := charas[name].Faces[value]; ok {
								chara.Face = v
							}
						case "storage":
							charaImage, _, err := ebitenutil.NewImageFromFileSystem(
								r.fses["images"],
								object.Pm["storage"],
							)
							if err != nil {
								panic(err)
							}
							charas[name].Image = charaImage
						case "refrect":
							switch value {
							case "true":
								chara.Reflect = true
							case "false":
								chara.Reflect = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "width":
							chara.Width, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "height":
							chara.Height, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "left":
							chara.Left, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "top":
							chara.Top, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						}
					}
					herfWidth := charas[name].Image.Bounds().Dx() / 2
					charaSpace := r.manager.Config.ScreenWidth / (len(viewCharas) + 2)
					currentLeft := charaSpace
					if chara.Left == 0 && chara.Top == 0 {
						chara.Left = charaSpace - herfWidth
						chara.Top = r.manager.Config.ScreenHeight - charas[name].Image.Bounds().Dy()
					}
					for _, c := range viewCharas {
						currentLeft += charaSpace
						left := currentLeft - (charas[c.Name].Image.Bounds().Dx() / 2)
						c.NewLeft = left
						c.IsSlide = true

					}
					viewCharas = append(viewCharas, chara)
					fmt.Printf("viewCharas:%+v\n", chara)
					fmt.Printf("viewCharas:%+v\n", viewCharas)
					if chara.Wait {
						y.Until(true, func() bool {
							return float64(t - charaTick) / float64(chara.Time * ebiten.TPS() / 1000) >= 1
						})
					}
				case "cm":
				case "font":
					err = r.textStyle(object)
					if err != nil {
						panic(err)
					}
				case "glink":
					glink := &kag3.GLink{}
					for key, value := range object.Pm {
						switch key {
						case "color":
							var r, g, b int
							r, g, b, err = parseColor(value)
							if err != nil {
								panic(err)
							}
							glink.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
						case "font_color":
							glink.FontColor = value
						case "storage":
							glink.Storage = value
						case "target":
							glink.Target = value
						case "name":
							glink.Name = value
						case "text":
							glink.Text = value
						case "x":
							glink.X, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "y":
							glink.Y, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "width":
							glink.Width, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "height":
							glink.Height, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "size":
							glink.Size, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "face":
							glink.Face = value
						case "graphic":
							glink.Graphic = value
						case "enterimg":
							glink.EnterImg = value
						case "clickse":
							glink.ClickSE = value
						case "enterse":
							glink.EnterSE = value
						case "leavese":
							glink.LeaveSE = value
						}
					}
					if glink.Height == 0 {
						_, h := text.Measure(glink.Text, r.fontFace, 0)
						glink.Height = int(h) + 20
					}
					glinks = append(glinks, glink)
				case "image":
					err = r.image(object)
					if err != nil {
						panic(err)
					}
				case "jump":
					jump := &kag3.Jump{}
					for key, value := range object.Pm {
						switch key {
						case "storage":
							jump.Storage = value
						case "target":
							jump.Target = value
						}
					}
					fmt.Printf("jump:%+v\n", jump)
					if jump.Storage != "" {
						r.manager.LoadScript(jump.Storage)
						r.labels = r.manager.Labels
						r.scripts = r.manager.Senario
						if jump.Target == "" {
							i = 0
						}
					} else {
						if v, ok := r.labels[jump.Target]; ok {
							fmt.Printf("label:%+v\n", v)
							jumpIndex = v.Index
							i = v.Index
						}
						if v, ok := r.labels[jump.Target[1:]]; ok {
							fmt.Printf("label:%+v\n", v)
							jumpIndex = v.Index
							i = v.Index
						}
					}
				case "l":
					y.Until(true, isClicked)
				case "link":
					link := &kag3.Link{}
					for key, value := range object.Pm {
						switch key {
						case "storage":
							link.Storage = value
						case "target":
							link.Target = value
						case "keyforcus":
							link.KeyForcus, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						}
					}
					for {
						if v, ok := r.scripts[i].(kag3.TagObject); ok {
							if v.Name == "endlink" {
								break
							}
						}
						if v, ok := r.scripts[i].(kag3.TextObject); ok {
							link.Texts = append(link.Texts, v)
						}
						i++
					}
					links = append(links, link)
					fmt.Printf("links:%+v\n", links)
				case "p":
					y.Until(true, isClicked)
				case "playbgm":
					bgmTick = t
					fmt.Println("playbgm")
					bgm := &kag3.BGM{}
					bgm.Volume = 100
					var b []byte
					var v *vorbis.Stream
					b, err = fs.ReadFile(r.fses["bgms"], object.Pm["storage"])
					if err != nil {
						panic(err)
					}
					v, err = vorbis.DecodeF32(bytes.NewReader(b))
					if err != nil {
						panic(err)
					}
					bgm.Player, err = audioContext.NewPlayerF32(v)
					if err != nil {
						panic(err)
					}
					for key, value := range object.Pm {
						switch key {
						case "loop":
							switch value {
							case "true":
								bgm.Loop = true
							case "false":
								bgm.Loop = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "sprite_time":
							bgm.SpriteTime = value
						case "volume":
							bgm.Volume, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "pause":
							switch value {
							case "true":
								bgm.Pause = true
							case "false":
								bgm.Pause = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "seek":
							bgm.Seek, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "restart":
							switch value {
							case "true":
								bgm.Restart = true
							case "false":
								bgm.Restart = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "time":
							bgm.Time, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						}
					}
					bgm.Player.SetVolume(float64(bgm.Volume / 100))
					if bgm.Restart {
						bgm.Player.Rewind()
					}
					if !bgm.Player.IsPlaying() {
						bgm.Player.Rewind()
						bgm.Player.Play()
					}
				case "ptext":

				case "position":
					err = r.position(object)
					if err != nil {
						panic(err)
					}
				case "r":
				case "resetfont":
					textStyle = nil
				case "s":
					y.Until(true, isJumped)
				case "title":
					ebiten.SetWindowTitle(object.Pm["name"])
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
			switch key {
			case "color":
				textStyle.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "edge":
				textStyle.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "shadow":
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

func (r *Renderer) button(object kag3.TagObject) (err error) {
	button := &kag3.Button{}
	for key, value := range object.Pm {
		switch key {
		case "graphic":
			folder := "images"
			if object.Pm["folder"] != "" {
				folder = object.Pm["folder"]
			}
			button.Graphic, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses[folder],
				object.Pm["graphic"],
			)
			if err != nil {
				return
			}
		case "storage":
			button.Storage = value
		case "target":
			button.Target = value
		case "name":
			button.Name = value
		case "x":
			button.X, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "y":
			button.Y, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "width":
			button.Width, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "height":
			button.Height, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "fix":
			switch value {
			case "true":
				button.Fix = true
			case "false":
				button.Fix = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "role":
			button.Role = value
		case "hint":
			button.Hint = value
		case "clickse":
			button.ClickSE = value
		case "enterse":
			button.EnterSE = value
		case "leavese":
			button.LeaveSE = value
		case "activeimg":
			button.ActiveImg = value
		case "clickimg":
			button.ClickImg = value
		case "enterimg":
			folder := "images"
			if object.Pm["folder"] != "" {
				folder = object.Pm["folder"]
			}
			button.EnterImg, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses[folder],
				object.Pm["enterimg"],
			)
			if err != nil {
				return
			}
		case "autoimg":
			button.AutoImg = value
		case "skipimg":
			button.SkipImg = value
		case "visible":
			switch value {
			case "true":
				button.Visible = true
			case "false":
				button.Visible = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "auto_next":
			switch value {
			case "true":
				button.AutoNext = true
			case "false":
				button.AutoNext = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "savesnap":
			switch value {
			case "true":
				button.SaveSnap = true
			case "false":
				button.SaveSnap = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "keyforcus":
			button.KeyForcus, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		}
	}
	if button.Width ==0 && button.Height == 0 {
		button.Width, button.Height = button.Graphic.Bounds().Dx(), button.Graphic.Bounds().Dy()
	}
	fmt.Printf("button: %+v\n", button)
	buttons = append(buttons, button)
	return nil
}

func (r *Renderer) image(object kag3.TagObject) (err error) {
	img := &kag3.Image{}
	for key, value := range object.Pm {
		switch key {
		case "layer":
			img.Layer = value
		case "page":
			img.Page = value
		case "left":
			img.Left, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "top":
			img.Top, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "x":
			img.X, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "y":
			img.Y, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "width":
			img.Width, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "height":
			img.Height, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "folder":
			img.Folder = value
		case "name":
			img.Name = value
		case "time":
			img.Time, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "wait":
			switch value {
			case "true":
				img.IsWait = true
			case "false":
				img.IsWait = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "zindex":
			img.ZIndex, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "depth":
			img.Depth = value
		case "refrect":
			switch value {
			case "true":
				img.Reflect = true
			case "false":
				img.Reflect = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		case "pos":
			img.Depth = value
		case "animimg":
			switch value {
			case "true":
				img.AnimImg = true
			case "false":
				img.AnimImg = false
			default:
				return fmt.Errorf("未対応の値です %s", value)
			}
		}
	}
	img.Image, _, err = ebitenutil.NewImageFromFileSystem(
		r.fses["images"],
		object.Pm["storage"],
	)
	if err != nil {
		return
	}
	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	if bg.Image != nil {
		screen.DrawImage(bg.Image, &ebiten.DrawImageOptions{})
		if bg.NextImage != nil {
			switch bg.Method {
			case "fadeIn":
				e := effects.FadeIn{}
				e.DrawBackground(screen, bg, bgTick, t, bg.Time)
			case "crossfade":
				e := effects.CrossFade{}
				e.DrawBackground(screen, bg, bgTick, t, bg.Time)
			case "slide", "slideInRight":
				e := effects.SlideInRight{}
				e.DrawBackground(screen, bg, bgTick, t, bg.Time)
			}
		}
	}
	for _, chara := range viewCharas {
		if chara.IsSlide {
			e := &effects.SlideInLeft{}
			e.Draw(screen, charas[chara.Name].Image, chara, charaTick, t, chara.Time)
		} else {
			e := &effects.FadeIn{}
			e.Draw(screen, charas[chara.Name].Image, chara.Left, chara.Top, charaTick, t, chara.Time)
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
		//count := t / (r.manager.Config.ChSpeed * ebiten.TPS() / 1000)
		count := t / 5
		multiCount := 0
		multiWidth := 0.0
		lastGlyphWidth := 0.0
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
				multiWidth += w
				maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
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
				//fmt.Println(v.Text)
				glyph := text.AppendGlyphs(
					textGlyphs,
					v.Text,
					r.fontFace,
					&tOp.LayoutOptions,
				)
				lastGlyph := glyph[len(glyph)-1]
				lastGlyphWidth += lastGlyph.X + float64(lastGlyph.Image.Bounds().Dx())
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
				if (count >= len(glyph) - 1){
					isWait = true
					tOp.GeoM.Reset()
					tOp.GeoM.Translate(marginLeft + multiWidth - w, marginTop)
					text.Draw(screen, v.Text, r.fontFace, tOp)
				} else {
					if multiCount > 1 {
						marginLeft += lastGlyphWidth - glyph[len(glyph) - 1].X + float64(glyph[len(glyph) - 1].Image.Bounds().Dx())
					}
					for _, g := range glyph[:count] {
						tOp.GeoM.Reset()
						tOp.GeoM.Translate(marginLeft, marginTop)
						tOp.GeoM.Translate(g.X, g.Y)
						screen.DrawImage(g.Image, &tOp.DrawImageOptions)
					}
				}
				textGlyphs = []text.Glyph{}
			}
		}
	}
	for i, link := range links {
		for j, t := range link.Texts {
			linkOp := &text.DrawOptions{}
			x, y := float64(textPosition.Left), float64(textPosition.Top)
			_, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + float64(textPosition.MarginLeft)
			marginTop := y + float64(textPosition.MarginTop) + h * float64(i + j)
			linkOp.GeoM.Translate(marginLeft, marginTop)
			if textStyle != nil {
				if textStyle.Size != 0 && float64(textStyle.Size) != beforeTextSize {
					r.fontFace.Size = float64(textStyle.Size)
				}
				if textStyle.Color != nil {
					linkOp.ColorScale.ScaleWithColor(textStyle.Color)
				}
			}
			if !isJump {
				text.Draw(screen, t.Val, r.fontFace, linkOp)
			}
		}
	}
	for _, glink := range glinks {
		backgroundOp := &ebiten.DrawImageOptions{}
		backgroundOp.GeoM.Translate(float64(glink.X), float64(glink.Y))
		img := ebiten.NewImage(glink.Width, glink.Height)
		img.Fill(glink.Color)
		screen.DrawImage(img, backgroundOp)
		glinkOp := &text.DrawOptions{}
		w, h := text.Measure(glink.Text, r.fontFace, 0)
		glinkOp.GeoM.Translate(float64(glink.Width / 2 + glink.X) - (w / 2), float64(glink.Height / 2 + glink.Y) - (h / 2))
		text.Draw(screen, glink.Text, r.fontFace, glinkOp)
	}
	for _, button := range buttons {
		buttonOp := &ebiten.DrawImageOptions{}
		buttonOp.GeoM.Translate(float64(button.X), float64(button.Y))

		mX, mY := ebiten.CursorPosition()
		if isColision(mX, mY, button.X, button.Y, button.Width, button.Height) {
			screen.DrawImage(button.EnterImg, buttonOp)
		} else {
			screen.DrawImage(button.Graphic, buttonOp)
		}
	}
	for _, img := range imgs {
		imgOp := &ebiten.DrawImageOptions{}
		imgOp.GeoM.Translate(float64(img.X), float64(img.Y))
		screen.DrawImage(img.Image, imgOp)
	}
	//mx, my := ebiten.CursorPosition()
	//ebitenutil.DebugPrint(screen, fmt.Sprintf("t:%+v bgTick:%+v mouseX:%+v mouseY:%+v", t, bgTick, mx, my))
}

func parseColor(value string) (r, g, b int, err error) {
	switch value {
	case "black":
		return 255, 255, 255, nil
	case "red":
		return 255, 0, 0, nil
	case "blue":
		return 0, 0, 255, nil
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

func isColision(mX, mY, x, y, width, height int) bool {
	return mX >= x && mX <= x + width && mY >= y && mY <= y + height
}

func removeCharaShow(slice []*kag3.CharaShow, deleteTarget string) (result []*kag3.CharaShow) {
	if len(slice) == 1 {
		if slice[0].Name != deleteTarget {
			panic(fmt.Errorf("その名前のキャラクターはいません name=%s", deleteTarget))
		} else {
			return
		}
	}
	for i, v := range slice {
		if v.Name == deleteTarget {
			result = append(result[:i], result[i+1:]...)
		}
	}
	return
}
