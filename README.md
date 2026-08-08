# kag3

Go言語 + [ebitengine](https://ebitengine.org/) で実装された、ティラノスクリプト互換のノベルゲームエンジンです。
既存の `.ks` シナリオ資産をほぼそのまま動かしながら、Windows・macOS・Linux へシングルバイナリで展開できます。

## 特徴

- **Go製・シングルバイナリ** — `go build` だけで外部ランタイム依存のない実行ファイルを生成できます。配布もクロスコンパイルもシンプルです
- **ティラノスクリプト互換** — おなじみの `.ks` タグ構文をそのまま解釈します。既存シナリオの資産を活かしながらGoの実行環境に移行できます
- **マルチプラットフォーム** — ebitengineを描画エンジンに採用し、Windows・macOS・Linuxへ同じコードベースから展開できます
- **タグレジストリで拡張しやすい** — タグはレジストリ方式で登録され、未実装のタグがあってもエンジンは止まりません。プロジェクト独自のタグも追加しやすい設計です

## Requirements

Go 1.25 以上

## Installation

```sh
go get github.com/ebinovel/kag3
```

## Usage

kag3は `fs.FS` 経由でシナリオ・素材を読み込みます。最小構成:

```go
package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/ebinovel/kag3"
	"github.com/ebinovel/kag3/renderer/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed resources
var embedded embed.FS

func sub(dir string) fs.FS {
	f, err := fs.Sub(embedded, "resources/"+dir)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

type Game struct {
	manager  *kag3.Manager
	renderer *ebitengine.Renderer
}

func (g *Game) Layout(w, h int) (int, int) {
	return g.manager.Config.ScreenWidth, g.manager.Config.ScreenHeight
}

func (g *Game) Update() error {
	g.renderer.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.renderer.Draw(screen)
}

func main() {
	root, err := fs.Sub(embedded, "resources")
	if err != nil {
		log.Fatal(err)
	}

	manager := &kag3.Manager{}
	manager.Init(map[string]fs.FS{
		"resources": root,
		"senarios":  sub("senarios"),
		"images":    sub("images"),
		"bgms":      sub("bgms"),
		"ses":       sub("ses"),
		// system/images はkag3が同梱する既定のUI素材(メニューボタン等)をそのまま使う
		"system/images": kag3.Images,
	})
	if err := manager.LoadFirstScript(); err != nil {
		log.Fatal(err)
	}

	renderer, err := ebitengine.NewRenderer(manager)
	if err != nil {
		log.Fatal(err)
	}

	game := &Game{manager: manager, renderer: renderer}
	ebiten.SetWindowSize(manager.Config.ScreenWidth, manager.Config.ScreenHeight)
	ebiten.SetWindowTitle(manager.Config.Title)
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
```

`resources/senarios/first.ks` が最初に読み込まれるシナリオです。`images` / `bgms` / `ses`
は、シナリオ側で `[image]` や `[playbgm]` などのタグを使ったときに初めて参照されるため、
使わないうちはディレクトリが空でも構いません。`resources/config.toml` は省略でき、存在しない
場合は既定のウィンドウサイズ・タイトルなどが使われます。

より詳しい手順は [ebinovel.github.io/getting-started.html](https://ebinovel.github.io/getting-started.html) を参照してください。ブラウザで動く[プレイページ](https://ebinovel.github.io/play.html)もあります。

## エディタ

kag3向けのネイティブGUIエディタ [ebinovel-editor](https://github.com/ebinovel/editor) を
別プロジェクトとして開発中です。シンタックスハイライト付きの `.ks` 編集・リアルタイム
プレビュー・ワンクリックビルドに対応予定です。

## License

Apache License 2.0

## Author

raa0121 <raa0121@gmail.com>
