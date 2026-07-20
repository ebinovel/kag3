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
	Text      string
	TextStyle *kag3.TextStyle
	Ruby      string
}

type Renderer struct {
	manager      *kag3.Manager
	scripts      []any
	labels       map[string]kag3.LabelInfo
	fontFace     *text.GoTextFace
	nameFontFace *text.GoTextFace
	fses         map[string]fs.FS
	texts        map[int][]Text
	line         int
	Done         bool
}

var (
	isFirst                                      bool
	isWait                                       bool
	loop                                         func(y coro.Yield)
	isClicked, isTextEnded                       func() bool
	t, tick, oldTick, bgTick, charaTick, bgmTick int
	charas                                       map[string]*kag3.Character
	viewCharas                                   []*kag3.CharaShow
	bg                                           *kag3.Background
	textPosition                                 *kag3.TextPosition
	textStyle                                    *kag3.TextStyle
	beforeTextSize                               float64
	pText                                        *kag3.PText
	textGlyphs                                   []text.Glyph
	charaName                                    string
	buttons                                      []*kag3.Button
	imgs                                         []*kag3.Image
	glinks                                       []*kag3.GLink
	links                                        []*kag3.Link
	isJump                                       bool
	isJumped                                     func() bool
	jumpIndex                                    int
	audioContext                                 *audio.Context
	layopt                                       *kag3.LayOpt
	isTextEnd                                    bool
	textStartT                                   int
	prevLine                                     int
	pendingRuby                                  string
	isSkip                                       bool
	isAuto                                       bool
	autoStartT                                   int
	co                                           *coro.Coro
)

func init() {
	charas = make(map[string]*kag3.Character)
	textPosition = &kag3.TextPosition{}
	audioContext = audio.NewContext(44100)
	layopt = &kag3.LayOpt{}
	bg = &kag3.Background{
		Time:   3000,
		IsWait: true,
		Method: "crossfade",
	}
	isClicked = func() bool {
		return oldTick+3 >= tick
	}
	isTextEnded = func() bool {
		return oldTick+3 >= tick && isWait
	}
	isJumped = func() bool {
		return isJump
	}
}

func NewRenderer(manager *kag3.Manager) (r *Renderer, err error) {
	r = &Renderer{
		manager:  manager,
		scripts:  manager.Senario,
		labels:   manager.Labels,
		fontFace: manager.FontFace,
		nameFontFace: &text.GoTextFace{
			Source:   manager.FontFace.Source,
			Size:     manager.FontFace.Size,
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
		co = coro.New(loop)
		isFirst = true
	}
	if doNext() {
		if !isWait {
			isWait = true
		} else {
			oldTick = tick
		}
	}
	if isSkip {
		isWait = true
		oldTick = tick
	}
	if !isWait {
		autoStartT = t
	}
	if isAuto && isWait && t-autoStartT >= 3*ebiten.TPS() {
		oldTick = tick
	}
	for i, link := range links {
		mX, mY := ebiten.CursorPosition()
		for j, t := range link.Texts {
			x, y := textPosition.Left, textPosition.Top
			w, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + textPosition.MarginLeft
			marginTop := y + textPosition.MarginTop + int(h)*(i+j)
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
				fmt.Printf("click button:%+v\n", button)
				fmt.Printf("labels:%+v\n", r.labels)
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
					case "quicksave":
					case "quickload":
					case "backlog":
					case "menu":
					case "fullscreen":
						fmt.Printf("button.Role:%s\n", button.Role)
						ebiten.SetFullscreen(!ebiten.IsFullscreen())
					case "title":
						r.manager.LoadScript("title.ks")
						r.labels = r.manager.Labels
						r.scripts = r.manager.Senario
						r.texts = make(map[int][]Text)
						viewCharas = nil
						charaName = ""
						pendingRuby = ""
						isWait = false
						isSkip = false
						isAuto = false
						jumpIndex = 0
						isJump = true
					case "skip":
						isSkip = !isSkip
						if isSkip {
							isAuto = false
						}
					case "auto":
						isAuto = !isAuto
						if isAuto {
							isSkip = false
							autoStartT = t
						}
					case "window":
						textPosition.Visible = !textPosition.Visible
					case "sleepgame":
						// storage loading already handled above
					}
				}
				if button.Target != "" {
					if v, ok := r.labels[button.Target]; ok {
						fmt.Printf("label:%+v\n", v)
						jumpIndex = v.Index
						isJump = true
					}
					if v, ok := r.labels[button.Target[1:]]; ok {
						fmt.Printf("label:%+v\n", v)
						jumpIndex = v.Index
						isJump = true
					}

				}
			}
		}
	}
	if isJump {
		glinks = nil
		buttons = nil
		links = nil
	}
	for i := 0; i < 1000; i++ {
		if !co.Next() {
			break
		}
		tick++
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
				isNewLine := r.line != object.Line
				r.line = object.Line
				if isNewLine && len(object.Val) > 0 {
					isWait = false
					textStartT = t
				}
				if object.Chara != nil {
					fmt.Println(object.Chara)
					charaName = object.Chara.Name
				}
				if len(r.texts[object.Line]) == 0 {
					r.texts[object.Line] = append(
						r.texts[object.Line],
						Text{Text: object.Val, Ruby: pendingRuby},
					)
					pendingRuby = ""
				} else {
					if r.texts[object.Line][len(r.texts[object.Line])-1].Text == "" {
						r.texts[object.Line][len(r.texts[object.Line])-1].Text = object.Val
						if pendingRuby != "" {
							r.texts[object.Line][len(r.texts[object.Line])-1].Ruby = pendingRuby
							pendingRuby = ""
						}
					} else {
						r.texts[object.Line] = append(
							r.texts[object.Line],
							Text{Text: object.Val, Ruby: pendingRuby},
						)
						pendingRuby = ""
					}
				}
				// タグを挟まない連続するテキスト行を1つに連結
				if len(object.Val) > 0 {
					for i+1 < len(r.scripts) {
						next, ok := r.scripts[i+1].(kag3.TextObject)
						if !ok || len(next.Val) == 0 {
							break
						}
						i++
						if next.Chara != nil {
							charaName = next.Chara.Name
						}
						last := len(r.texts[object.Line]) - 1
						r.texts[object.Line][last].Text += next.Val
					}
				}
				if isNewLine && len(object.Val) > 0 {
					y()
					y.Until(false, func() bool { return isWait })
				}
			case kag3.TagObject:
				r.line = object.Line
				switch object.Name {
				case "bg":
					fmt.Printf("bg:%+v\n", bg)
					bgTick = t
					bg.IsEnd = false
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
						Name:  name,
						Image: charaImage,
						JName: object.Pm["jname"],
						Faces: make(map[string]string),
					}
					charas[name].Faces["default"] = object.Pm["storage"]
				case "chara_hide":
					for _, chara := range viewCharas {
						chara.Remove(object.Pm["name"])
						fmt.Printf("chara_hide:%+v\n", chara)
					}
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
					chara, err := r.charaShow(object)
					if err != nil {
						panic(err)
					}
					if chara.Wait {
						y.Until(true, func() bool {
							return float64(t-charaTick)/float64(chara.Time*ebiten.TPS()/1000) >= 1
						})
					}
				case "cm":
					r.texts = make(map[int][]Text)
					isWait = false
					charaName = ""
					pendingRuby = ""
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
				case "layopt":
					for key, value := range object.Pm {
						switch key {
						case "layer":
							layopt.Layer = value
						case "page":
							layopt.Page = value
						case "visible":
							switch value {
							case "true":
								layopt.Visible = true
							case "false":
								layopt.Visible = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "left":
							layopt.Left, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "top":
							layopt.Top, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						case "opacity":
							layopt.Opacity, err = strconv.Atoi(value)
							if err != nil {
								panic(err)
							}
						}
					}
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
					y.Until(true, isTextEnded)
					r.texts = make(map[int][]Text)
					isWait = false
					charaName = ""
					pendingRuby = ""
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
					pText = &kag3.PText{}
					r, g, b, a := color.White.RGBA()
					pText.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}
					for key, value := range object.Pm {
						switch key {
						case "name":
							pText.Name = value
						case "layer":
							pText.Layer = value
						case "page":
							pText.Page = value
						case "text":
							pText.Text = value
						case "x":
							pText.X, err = strconv.Atoi(value)
						case "y":
							pText.Y, err = strconv.Atoi(value)
						case "vertical":
							pText.Vertical, err = strconv.ParseBool(value)
						case "size":
							pText.Size, err = strconv.Atoi(value)
						case "face":
							pText.Face = value
						case "color", "edge", "shadow":
							var r, g, b int
							r, g, b, err = parseColor(value)
							switch key {
							case "color":
								pText.Color = &color.RGBA{uint8(r), uint8(g), uint8(b), 0}
							case "edge":
								pText.Edge = &color.RGBA{uint8(r), uint8(g), uint8(b), 0}
							case "shadow":
								pText.Shadow = &color.RGBA{uint8(r), uint8(g), uint8(b), 0}
							}
						case "bold":
							switch value {
							case "true":
								pText.Bold = true
							case "false":
								pText.Bold = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "width":
							pText.Width, err = strconv.Atoi(value)
						case "align":
							switch value {
							case "left", "center", "right":
								pText.Align = value
							}
						case "time":
							pText.Time, err = strconv.Atoi(value)
						case "overwrite":
							switch value {
							case "true":
								pText.Overwrite = true
							case "false":
								pText.Overwrite = false
							default:
								panic(fmt.Errorf("未対応の値です %s", value))
							}
						case "gradient":
							pText.Gradient = value
						}
					}
				case "position":
					err = r.position(object)
					if err != nil {
						panic(err)
					}
				case "r":
				case "resetfont":
					textStyle = nil
				case "ruby":
					pendingRuby = object.Pm["text"]
				case "s":
					y.Until(true, isJumped)
				case "title":
					ebiten.SetWindowTitle(object.Pm["name"])
				}
			}
			y()
		}
		r.Done = true
	}
}

func (r *Renderer) charaShow(object kag3.TagObject) (chara *kag3.CharaShow, err error) {
	chara = &kag3.CharaShow{}
	charaTick = t
	charaNew := true
	name := object.Pm["name"]
	if _, ok := charas[name]; !ok {
		return nil, fmt.Errorf("そのキャラクターは登録されてません name=%s", name)
	}
	for _, c := range viewCharas {
		if c.Name == name && c.IsRemove {
			c.IsRemove = false
			charaNew = false
			chara = c
		}
	}
	if charaNew {
		chara.Wait = true
		chara.Time = 1000
		for key, value := range object.Pm {
			switch key {
			case "name":
				chara.Name = value
			case "time":
				chara.Time, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "zindex":
				chara.Zindex, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "depth":
				chara.Depth = value
			case "page":
				chara.Page = value
			case "wait":
				chara.Wait, err = strconv.ParseBool(value)
				if err != nil {
					return
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
					return nil, err
				}
				charas[name].Image = charaImage
			case "refrect":
				chara.Reflect, err = strconv.ParseBool(value)
				if err != nil {
					return
				}
			case "width":
				chara.Width, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "height":
				chara.Height, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "left":
				chara.Left, err = strconv.Atoi(value)
				if err != nil {
					return
				}
			case "top":
				chara.Top, err = strconv.Atoi(value)
				if err != nil {
					return
				}
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
		if charaNew {
			c.IsSlide = true
		}

	}
	if charaNew {
		viewCharas = append(viewCharas, chara)
	}
	fmt.Printf("viewCharas:%+v\n", chara)
	fmt.Printf("viewCharas:%+v\n", viewCharas)
	return
}

func (r *Renderer) position(tagObject kag3.TagObject) (err error) {
	for key, value := range tagObject.Pm {
		switch key {
		case "layer":
			textPosition.Layer = value
		case "page":
			textPosition.Page = value
		case "left":
			textPosition.Left, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "top":
			textPosition.Top, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "width":
			textPosition.Width, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "height":
			textPosition.Height, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "frame":
			textPosition.FrameImage, _, err = ebitenutil.NewImageFromFileSystem(
				r.fses["images"],
				tagObject.Pm["frame"],
			)
			if err != nil {
				return
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
				return
			}
		case "opacity":
			textPosition.Opacity, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginl":
			textPosition.MarginLeft, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "margint":
			textPosition.MarginTop, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginr":
			textPosition.MarginRight, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginb":
			textPosition.MarginBottom, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "marginn":
			textPosition.MarginN, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "radius":
			textPosition.Radius, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "vertial", "vertical":
			textPosition.Vertical, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "visible":
			textPosition.Visible, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
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
			button.Fix, err = strconv.ParseBool(value)
			if err != nil {
				return
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
			button.Visible, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "auto_next":
			button.AutoNext, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "savesnap":
			button.SaveSnap, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "keyforcus":
			button.KeyForcus, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		}
	}
	if button.Width == 0 && button.Height == 0 {
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
			img.IsWait, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "zindex":
			img.ZIndex, err = strconv.Atoi(value)
			if err != nil {
				return
			}
		case "depth":
			img.Depth = value
		case "refrect":
			img.Reflect, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		case "pos":
			img.Depth = value
		case "animimg":
			img.AnimImg, err = strconv.ParseBool(value)
			if err != nil {
				return
			}
		}
	}
	folder := "images"
	if object.Pm["folder"] != "" {
		folder = object.Pm["folder"]
	}
	img.Image, _, err = ebitenutil.NewImageFromFileSystem(
		r.fses[folder],
		object.Pm["storage"],
	)
	if err != nil {
		return
	}
	imgs = append(imgs, img)
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
			if chara.IsRemove {
				e := &effects.FadeOut{}
				e.Draw(screen, charas[chara.Name].Image, chara.Left, chara.Top, charaTick, t, chara.Time)
			} else {
				e := &effects.FadeIn{}
				e.Draw(screen, charas[chara.Name].Image, chara.Left, chara.Top, charaTick, t, chara.Time)
			}
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
		if pText != nil {
			cPTextOp := &text.DrawOptions{}
			cPTextOp.GeoM.Translate(float64(pText.X), float64(pText.Y))
			cPTextOp.ColorScale.ScaleWithColor(color.White)
			text.Draw(screen, charaName, r.nameFontFace, cPTextOp)
		}

		marginLeft := x + float64(textPosition.MarginLeft)
		marginTop := y + float64(textPosition.MarginTop)
		count := (t - textStartT) / 5
		isTextEnd = false
		lineNums := make([]int, 0, len(r.texts))
		for k := range r.texts {
			lineNums = append(lineNums, k)
		}
		slices.Sort(lineNums)
		if textPosition.Vertical {
			type runeEntry struct {
				ch    rune
				style *kag3.TextStyle
			}
			charSize := r.fontFace.Size
			rightEdge := x + float64(textPosition.Width) - float64(textPosition.MarginRight)
			availableHeight := float64(textPosition.Height) - float64(textPosition.MarginTop) - float64(textPosition.MarginBottom)
			charsPerCol := int(availableHeight / charSize)
			if charsPerCol <= 0 {
				charsPerCol = 1
			}
			colIndex := 0
			for _, lineNum := range lineNums {
				segs := r.texts[lineNum]
				var runes []runeEntry
				for _, v := range segs {
					if len(v.Text) == 0 {
						continue
					}
					for _, ch := range v.Text {
						runes = append(runes, runeEntry{ch, v.TextStyle})
					}
				}
				if len(runes) == 0 {
					continue
				}
				showCount := len(runes)
				if lineNum == r.line {
					if !isWait {
						if count >= len(runes) {
							isWait = true
							isTextEnd = true
						} else {
							showCount = count
						}
					} else {
						isTextEnd = true
					}
				}
				for i, rs := range runes[:showCount] {
					col := colIndex + i/charsPerCol
					row := i % charsPerCol
					colX := rightEdge - float64(col+1)*charSize
					tOp := &text.DrawOptions{}
					tOp.GeoM.Translate(colX, marginTop+float64(row)*charSize)
					if textStyle != nil && textStyle.Color != nil {
						tOp.ColorScale.ScaleWithColor(textStyle.Color)
					} else if rs.style != nil && rs.style.Color != nil {
						tOp.ColorScale.ScaleWithColor(rs.style.Color)
					} else {
						tOp.ColorScale.ScaleWithColor(color.White)
					}
					text.Draw(screen, string(rs.ch), r.fontFace, tOp)
				}
				colIndex += (len(runes) + charsPerCol - 1) / charsPerCol
			}
		} else {
			rowY := 0.0
			for _, lineNum := range lineNums {
				segs := r.texts[lineNum]
				hasText := false
				for _, v := range segs {
					if len(v.Text) > 0 {
						hasText = true
						break
					}
				}
				if !hasText {
					continue
				}
				rubyLineHeight := 0.0
				for _, v := range segs {
					if v.Ruby != "" {
						rubyLineHeight = beforeTextSize * 0.5
						break
					}
				}
				if lineNum == r.line {
					totalGlyphs := 0
					for _, v := range segs {
						if len(v.Text) == 0 {
							continue
						}
						tOp2 := &text.DrawOptions{}
						tOp2.LineSpacing = r.fontFace.Size
						g := text.AppendGlyphs(nil, v.Text, r.fontFace, &tOp2.LayoutOptions)
						totalGlyphs += len(g)
					}
					if totalGlyphs > 0 {
						charsToShow := totalGlyphs
						if !isWait {
							if count >= totalGlyphs {
								isWait = true
								isTextEnd = true
							} else {
								charsToShow = count
							}
						} else {
							isTextEnd = true
						}
						segOffset := 0
						xOffset := 0.0
						for _, v := range segs {
							if len(v.Text) == 0 {
								continue
							}
							tOp := &text.DrawOptions{}
							tOp.LineSpacing = r.fontFace.Size
							vText := v.Text
							w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
							maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
							if w > maxWidth {
								rn := []rune(vText)
								key := len(rn) - 1
								for idx := key; idx >= 0; idx-- {
									ww, _ := text.Measure(string(rn[:idx]), r.fontFace, tOp.LineSpacing)
									if ww <= maxWidth {
										rn = append(rn[:idx+1], rn[idx:]...)
										rn[idx] = []rune("\n")[0]
										break
									}
								}
								vText = string(rn)
								w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
							}
							glyph := text.AppendGlyphs(nil, vText, r.fontFace, &tOp.LayoutOptions)
							segLen := len(glyph)
							if textStyle != nil {
								if textStyle.Size != 0 && float64(textStyle.Size) != beforeTextSize {
									r.fontFace.Size = float64(textStyle.Size)
								}
								if textStyle.Color != nil {
									tOp.ColorScale.ScaleWithColor(textStyle.Color)
								} else {
									tOp.ColorScale.ScaleWithColor(color.White)
								}
							} else if v.TextStyle != nil {
								if v.TextStyle.Size != 0 && float64(v.TextStyle.Size) != beforeTextSize {
									r.fontFace.Size = float64(v.TextStyle.Size)
								}
								if v.TextStyle.Color != nil {
									tOp.ColorScale.ScaleWithColor(v.TextStyle.Color)
								} else {
									tOp.ColorScale.ScaleWithColor(color.White)
								}
							} else {
								r.fontFace.Size = beforeTextSize
								tOp.ColorScale.ScaleWithColor(color.White)
							}
							segShowCount := charsToShow - segOffset
							if segShowCount >= segLen {
								tOp.GeoM.Reset()
								tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
								text.Draw(screen, vText, r.fontFace, tOp)
								if v.Ruby != "" {
									rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: beforeTextSize * 0.5, Language: r.fontFace.Language}
									rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
									rubyOp := &text.DrawOptions{}
									rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
									rubyOp.ColorScale.ScaleWithColor(color.White)
									text.Draw(screen, v.Ruby, rubyFace, rubyOp)
								}
							} else if segShowCount > 0 {
								for _, g := range glyph[:segShowCount] {
									if g.Image == nil {
										continue
									}
									tOp.GeoM.Reset()
									tOp.GeoM.Translate(marginLeft+xOffset+g.X, marginTop+rowY+rubyLineHeight+g.Y)
									screen.DrawImage(g.Image, &tOp.DrawImageOptions)
								}
							}
							xOffset += w
							segOffset += segLen
						}
					}
				} else {
					xOffset := 0.0
					for _, v := range segs {
						if len(v.Text) == 0 {
							continue
						}
						tOp := &text.DrawOptions{}
						tOp.LineSpacing = r.fontFace.Size
						vText := v.Text
						w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
						maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
						if w > maxWidth {
							rn := []rune(vText)
							key := len(rn) - 1
							for idx := key; idx >= 0; idx-- {
								ww, _ := text.Measure(string(rn[:idx]), r.fontFace, tOp.LineSpacing)
								if ww <= maxWidth {
									rn = append(rn[:idx+1], rn[idx:]...)
									rn[idx] = []rune("\n")[0]
									break
								}
							}
							vText = string(rn)
							w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
						}
						if v.TextStyle != nil {
							if v.TextStyle.Color != nil {
								tOp.ColorScale.ScaleWithColor(v.TextStyle.Color)
							} else {
								tOp.ColorScale.ScaleWithColor(color.White)
							}
						} else {
							tOp.ColorScale.ScaleWithColor(color.White)
						}
						tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
						text.Draw(screen, vText, r.fontFace, tOp)
						if v.Ruby != "" {
							rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: beforeTextSize * 0.5, Language: r.fontFace.Language}
							rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
							rubyOp := &text.DrawOptions{}
							rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
							rubyOp.ColorScale.ScaleWithColor(color.White)
							text.Draw(screen, v.Ruby, rubyFace, rubyOp)
						}
						xOffset += w
					}
				}
				rowY += beforeTextSize + rubyLineHeight
			}
		}
		textGlyphs = []text.Glyph{}
	}
	for i, link := range links {
		for j, t := range link.Texts {
			linkOp := &text.DrawOptions{}
			x, y := float64(textPosition.Left), float64(textPosition.Top)
			_, h := text.Measure(t.Val, r.fontFace, 0)
			marginLeft := x + float64(textPosition.MarginLeft)
			marginTop := y + float64(textPosition.MarginTop) + h*float64(i+j)
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
		glinkOp.GeoM.Translate(float64(glink.Width/2+glink.X)-(w/2), float64(glink.Height/2+glink.Y)-(h/2))
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
	return mX >= x && mX <= x+width && mY >= y && mY <= y+height
}
