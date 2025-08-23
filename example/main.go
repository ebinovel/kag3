package main

import (
	"io/fs"

	"github.com/ebinovel/kag3"
	"github.com/ebinovel/kag3/renderer/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	manager *kag3.Manager
}

var (
	renderer *ebitengine.Renderer
)

func init () {
	resourcesInit()
}

func (g *Game) Layout(width, height int) (int, int) {
	return g.manager.Config.ScreenWidth, g.manager.Config.ScreenHeight
}

func (g *Game) Update() error {
	renderer.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	renderer.Draw(screen)
}

func main() {
	g := &Game{manager: &kag3.Manager{}}
	g.manager.Init(map[string]fs.FS{
		"resources": Embed,
		"bgms": Bgms,
		"fonts": Fonts,
		"images": Images,
		"senarios": Senarios,
		"ses": Ses,
		"system/images": kag3.Images,
	})
	

	var err error
	err = g.manager.LoadFirstScript()
	if err != nil {
		panic(err)
	}
	renderer, err = ebitengine.NewRenderer(g.manager)
	if err != nil {
		panic(err)
	}
	ebiten.SetWindowSize(g.manager.Config.ScreenWidth, g.manager.Config.ScreenHeight)
	ebiten.SetWindowTitle(g.manager.Config.Title)
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}
