package main

import (
	"bytes"
	"io/fs"

	"github.com/ebinovel/kag3"
	"github.com/ebinovel/kag3/renderer/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"
)

const (
	screenWidth  = 1280
	screenHeight = 720
)

type Game struct {}

var (
	renderer *ebitengine.Renderer
	fontFace *text.GoTextFace
)

func init () {
	resourcesInit()
	script, err := fs.ReadFile(Senarios, "scene1.ks")
	if err != nil {
		panic(err)
	}

	b, err := fs.ReadFile(Fonts, "NotoSansJP-Regular.ttf")
	if err != nil {
		panic(err)
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	fontFace = &text.GoTextFace{
		Source: s,
		Size: 28,
		Language: language.Japanese,
	}

	ks := &kag3.KS{}
	r, _, err := ks.ParseScenario(string(script))
	if err != nil {
		panic(err)
	}
	renderer, err = ebitengine.NewRenderer(r, fontFace, map[string]fs.FS{
		"bgms": Bgms,
		"fonts": Fonts,
		"images": Images,
		"senarios": Senarios,
		"ses": Ses,
	})
	if err != nil {
		panic(err)
	}
}

func (g *Game) Layout(width, height int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) Update() error {
	renderer.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	renderer.Draw(screen)
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	g := &Game{}
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}
