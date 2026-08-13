# 🎬 動画再生機能

`[movie]`は、[github.com/liqmix/govid](https://github.com/liqMix/govid)(**純Go・cgo不使用・
ffmpeg不要**の動画デコードライブラリ)によるフルスクリーン動画再生機能です。

`movie`というタグ名自体はtyrano.jp/tagのタグリファレンスにも載っていますが、kag3の実装は本家とは
別物です。cgo/ffmpegに依存しないため、kag3の「`go build`だけでシングルバイナリ」という設計方針を
崩さずに動画再生を追加できます。

## 対応コーデック・コンテナ

| コンテナ | コーデック |
| --- | --- |
| MP4 | H.264 |
| WebM | VP8 |
| MPEG-PS/ES(`.mpg`/`.mpeg`) | MPEG-1 |

**AV1は対応していません。** govid自身のAV1デコーダは「構造は解析できるが出力が壊れている」状態
(govidのREADME自身がそう明記しています)。`[movie]`はAV1エンコードのMP4/WebMを検出すると、
壊れた映像を表示する代わりにエラーを返します(ログに出て、その`[movie]`タグ自体は静かに
スキップされます)。

**音声トラックは一切デコードされません。** govidは映像のみのライブラリです。動画に音声を
付けたい場合は、別の音声ファイルを`se=`属性で指定して同時再生してください(下記参照)。

## 導入

### 1. `"videos"`というfs.FSキーを登録する

`[movie]`は`Manager.Init`に渡す`fs.FS`マップの`"videos"`キーから動画ファイルを読み込みます。
既存の`"images"`/`"bgms"`/`"ses"`と同じ要領で、エントリーポイント側(`example/game/resources.go`
など)で登録してください。

```go
func DefaultFSes() map[string]fs.FS {
	return map[string]fs.FS{
		// ...既存のキー...
		"videos": Videos, // fs.Sub(Embed, "videos") 等
	}
}
```

このキーを登録しないプロジェクトでは、`[movie]`は静かなno-op(エラーにならず、何も再生され
ない)になります — `renderer/ebitengine`は他プロジェクトとも共有しているため、動画再生を
使わないプロジェクトを壊さないための設計です。

### 2. 動画ファイルをエンコードする

ffmpegで以下のように用意してください。

```sh
# MP4 (H.264) — 標準的な選択肢
ffmpeg -i input.mov -c:v libx264 -pix_fmt yuv420p -bf 0 out.mp4

# WebM (VP8)
ffmpeg -i input.mov -c:v libvpx -crf 20 -b:v 4M -pix_fmt yuv420p out.webm
```

`-bf 0`(Bフレーム無効化)はgovidの必須要件ではありませんが、指定するとゼロレイテンシで
デコードできます。

**govidが対応していないもの**(いずれもエラーを返すか、`Codec`が対応チェック済みの範囲):
インターレース(フィールド/MBAFF)、マルチスライスグループ(FMO)、POC type 1、4:2:2/4:4:4
クロマ、ロスレス変換バイパス。特にOBS/NVENCの「ロスレス」画面キャプチャが吐く
High 4:4:4 Predictiveプロファイル(profile 244)は、エラーにならず**壊れた映像になる既知の
欠陥**があるので、`ffmpeg -c:v libx264 -profile:v high -pix_fmt yuv420p`で変換してから
使ってください。

## タグの使い方

```
[movie storage="op.mp4" se="op_bgm.ogg" skip=true]
```

- `storage=`(必須): 動画ファイル名。`"videos"`のfs.FSから読み込まれる
- `se=`: 同時再生する音声ファイル。`"ses"`のfs.FSから読み込まれ、`[playse]`と同じ`ses[]`経由で
  再生される(`"movie"`という専用バッファ名を使うので、`[stopse buf="movie"]`
  `[changevol buf="movie"]`がそのまま効く)
- `skip=`(既定`true`): クリックで再生を中断できるか
- `loop=`(既定`false`): ループ再生。`wait=false`と組み合わせて使う
  (`wait=true`のままループさせると、コルーチンが永久に戻ってこない)
- `wait=`(既定`true`): 再生が終わる(自然終了、またはスキップ)までコルーチンをブロックするか

```
; 背景ループ動画の例(音声無し・スキップ不要)
[movie storage="bg_loop.webm" loop=true wait=false skip=false]
```

## デコード性能

govid自身のベンチマーク(720p, 1フレームあたり):

| コーデック | ms/frame |
| --- | --- |
| H.264(Bフレーム有) | 15.2 |
| H.264(High, Bフレーム無) | 11.5 |
| MPEG-1 | 0.9 |

H.264は60TPSの1フレーム予算(約16.6ms)をほぼ使い切るため、`[movie]`の内部実装は常に
バックグラウンドgoroutineでデコードします(govidの`NewAsyncPlayer`)。デコードが再生に
追いつかない場合は、コマ落ちせず現在のフレームを保持したまま待つ(音声・映像が壊れることは
ない)動作になります。MPEG-1はH.264の15倍程度軽量なので、性能が厳しい環境ではMPEG-1も
選択肢に入ります。

## プラットフォーム対応

govidは純Go実装でcgoに依存しないため、`renderer/ebitengine`が対応しているプラットフォーム
(Windows/Linux/macOS/wasm/Android/iOS)すべてでビルドタグ無しにコンパイルが通ります。ただし
実機・実ブラウザでの動作確認は今のところデスクトップ(Windows)のみです — モバイル/wasmでの
デコード性能は未検証です。

## ライセンス

govid本体はMITライセンスです。同梱デモ(`example/game/resources/videos/`)のサンプル動画は
govidリポジトリの`examples/videos/`から借用しています(同じくMIT)。詳細は
`example/CREDITS.md`を参照してください。
