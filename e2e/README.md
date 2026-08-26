# kag3 WinAppDriver E2E テスト

`example/` のサンプルゲームを実際にビルド・起動し、[WinAppDriver](https://github.com/microsoft/WinAppDriver)
(Microsoft公式のWindows Desktopアプリ操作ツール)でウィンドウを検知・
アタッチしながら検証するE2Eテスト。WinAppDriver自身が話すのはW3C
WebDriverではなく、レガシーなSelenium/Appium JSON Wire Protocol
(`{"desiredCapabilities":{...}}`リクエスト、`sessionId`がトップレベルの
レスポンス)——実機で確認済み。また、WinAppDriver自身の座標クリック・
キー入力エンドポイント(`/actions`, `/moveto`, `/keys`)はこの環境では
実際には機能しなかった(pen/touchポインタのみ対応、`/moveto`は相対移動で
Y軸が反映されない等)ため、クリック・キー入力はWin32 API
(`SetCursorPos`+`mouse_event`、`keybd_event`)を直接呼んでいる
(`driver/window_windows.go`)。スクリーンショット取得のみWinAppDriverの
`/screenshot`を使う(これは正常に機能する)。

このディレクトリは kag3 本体のモジュールとは別の**独立した Go モジュール**
(`go.mod` が別)。`go test ./...` をリポジトリルートで実行してもここは含まれない。

## なぜ別モジュール/別リポジトリ運用にしないのか

- 実ウィンドウの GUI 操作が必要なため、このリポジトリのメインの CI や
  Claude Code のようなサンドボックス環境からは実行できない。開発者の
  ローカル Windows 環境で手動実行する前提
- renderer 側の変更と同じ PR でレビューできるよう、リポジトリは分けず
  ディレクトリのみ分離した

## 前提条件

1. Windows 10 (1809+) または Windows 11
2. **Developer Mode を有効化**: 設定 → プライバシーとセキュリティ → 開発者向け
3. **WinAppDriver をインストール**: [Releases](https://github.com/microsoft/WinAppDriver/releases)
   から `.msi` を取得してインストール。`TestMain` は 32bit 側・64bit 側の
   両方の Program Files を探索する(`%ProgramFiles(x86)%` →
   `%ProgramFiles%` の順。インストーラの公式なデフォルトは前者だが、
   実際に後者に入るケースを確認済み)。どちらでもない場所に入れた場合のみ、
   環境変数 `WINAPPDRIVER_PATH` でそのパスを指定する
4. Go 1.25 以上

Developer Mode が無効なままだと WinAppDriver は
`Developer mode is not enabled ... Failed to initialize: 0x80004005` を
出して初期化に失敗する。この場合 `TestMain` からは
「起動したがリッスンしなかった」というエラーに見えるので、まずここを疑うこと。

## 実行方法

```powershell
# 1. example をビルド(kag3リポジトリルートで実行)
#    ここでは GOWORK=off にしないこと(理由は下記)
go build -o e2e/testdata/kag3example.exe ./example

# 2. E2E テスト実行
cd e2e
go test ./... -v
```

### `GOWORK=off` は不要になった

`e2e/` は独立モジュールですが、リポジトリルートにあった `go.work` は nanoda の 0.16 対応が
main に入り直接 `require` するようになったため削除されました(CLAUDE.md の「nanoda」節を参照)。
`go.work` が無い今、手順1・2 とも素の(非ワークスペース)モジュールモードでそのまま動きます。

`TestMain` が WinAppDriver の起動状態を自動確認し、起動していなければ
`WinAppDriver.exe` を自動起動する(自動起動した場合のみ、テスト終了後に
自動で終了させる。既に起動していたインスタンスはそのまま残す)。

各テストは `example.exe` を実際に起動し、数秒間ウィンドウが表示されて
自動操作される様子が見える。テストごとに `t.TempDir()` 配下の一時ディレクトリを
`KAG3_SAVE_DIR` として渡すため、実際の `%AppData%\kag3\...\saves` は一切
汚さない。

## 構成

```
e2e/
├── driver/    WinAppDriver への最小限のHTTPクライアント(要素検索は非対応 — ebitengineの
│              ウィンドウはUI Automationツリーがほぼ空の単一canvasのため)
├── helpers/   kag3固有のヘルパー(座標変換・スクショ安定待ち・セーブJSON読み取り等)
├── testdata/  ビルド済みexample.exe(gitignore対象)
├── main_test.go           TestMain(WinAppDriver自動起動)
├── flows_test.go          既定で実行されるE2Eフロー(アサーションあり)
└── manual_newgame_test.go 目視確認用のスクリーンショット撮影ウォークスルー
                           (既定でSKIP — KAG3_MANUAL_SCREENSHOT_DIR参照)
```

## kag3本体への追加フック

`e2e/` から外部プロセスとして起動する都合上、kag3本体には最小限の環境変数
フックが2つ入っている(いずれも未設定時は既存の挙動を完全維持):

- `KAG3_SAVE_DIR`: セーブ先ディレクトリを絶対パスで上書き
  (`renderer/ebitengine/save_slots.go` の `saveDir()`)
- `KAG3_E2E_FAST`: 起動時に `textNoWait` を強制 `true` にする
  (`renderer/ebitengine/tags_message.go`) — 文字送りアニメーションを
  スキップし、E2Eの待ち時間を大幅に削減する

いずれも `helpers.LaunchGame` が自動で設定するため、手で指定する必要はない。

## `KAG3_MANUAL_SCREENSHOT_DIR`(目視確認用)

これはkag3本体ではなく `e2e/` 側だけで読む環境変数。`manual_newgame_test.go`
の `TestManual*` 6件は、ゲームを実際に進めながら各所でスクリーンショットを
撮って指定ディレクトリにPNGとして書き出す「目視確認用のウォークスルー」。
操作が実行できなければ `t.Fatalf` で止まるが、**描画結果が正しいかどうかの
アサーションは持たない**(`t.Error` は1つもなく、正しさの判断は撮れた画像を
見る人間側に委ねられている) — その点が `flows_test.go` との違い。
未設定だと全件SKIPされる:

```powershell
cd e2e
$env:GOWORK = "off"
$env:KAG3_MANUAL_SCREENSHOT_DIR = "$env:TEMP\kag3shots"
go test ./... -run TestManual -v
```

出力先ディレクトリは存在しなければ自動で作られる
(`manualScreenshotDir`)。事前に用意しておく必要はない。

既定のフローテストより時間がかかり、撮影結果を人間が見て初めて意味がある
ため、通常のリグレッション確認では設定しないでよい。

## 座標・行数がハードコードされている理由と注意点

`nav.go`/`flows_test.go` には `example/game/resources/senarios/title.ks`・
`scene1.ks` のボタン座標やEnterキーを送る回数がハードコードされている。
ebitengineのウィンドウには要素検索の手段がなく、座標クリックとEnterキー
連打で操作するしかないため。**シナリオファイル(`title.ks`/`scene1.ks`)や
ボタン画像を変更した場合、これらの値は再計算が必要**:

- ボタン座標は `[button x= y=]` のタグ属性 + 実際の画像ファイルサイズ
  (`file example/game/resources/images/**/*.png` で確認可能)の中心座標
- Enter回数は多くの箇所で「多めに見積もって、想定より先に進んでも
  安全な位置で止まる」設計にしてあるが、`openingLinesBeforeGlink`
  (glink選択肢直前)のようにピンポイントで数える必要がある箇所もある

いずれも、狙った操作が失敗した場合は後続のアサーション(ファイルが
存在しない・画面内容が変化していない等)で明確にテストが失敗するように
してあるので、「間違った位置を操作して誤ってテストが通ってしまう」
リスクは低いはず。

## 実機確認済みの内容

既定で実行されるフローテストは以下の2件。実機(Windows 11)で
`GOWORK=off go test ./... -v` を通してPASS確認済み(それぞれ17秒・20秒、
フルスイートで40秒弱)。

- `TestTitleReturnDoesNotLeakPreviousPlaythroughStyle`
- `TestQuickSaveThenLoadRestoresSceneText`

`manual_newgame_test.go` の `TestManual*` 6件は既定でSKIPされる(下記の
`KAG3_MANUAL_SCREENSHOT_DIR` を参照)。

この過程で実際に発見・修正したkag3本体側のバグ:

- `[p]` タグでブロック中(まだクリックで確定させていない行)にセーブし、
  ロードすると1行先に進んでしまうバグ(`applySaveData`/`goToTitle`が
  `isWait`だけ強制`true`にして`oldTick`を更新していなかったため、ロード先
  の`[p]`が偶然`isTextEnded`を満たしてしまっていた)
- 上記の修正の副作用で、role="title"のようにダイアログ確認を経由して
  `goToTitle`が呼ばれるケースが退行し、確認ダイアログのOKを押した直後の
  次のクリックが本来向けたかったUI(タイトル画面のボタン等)ではなく、
  まだ画面遷移が完了していない古い画面の`[p]`待ちの解除に浪費されてしまう
  問題(`handleP`が`isJump`を一切見ておらず、コルーチンのコールスタック
  の奥深くでブロックしていたため、外側のジャンプ処理に`isJump`が
  読まれる機会がなかった)

いずれも `renderer/ebitengine` 側にユニットテストの回帰テストを追加済み
(`confirm_title_flow_test.go`)。

## 既知のこの環境固有の制約

- **キー入力の取りこぼし**: `keybd_event` で送ったEnterキーがkag3の
  `Update()` ループのタイミングと競合し、まれに反映されないことがある
  (この環境固有のタイミング競合)。`flows_test.go` の `advanceOne` は
  画面のスクリーンショットが実際に変化したかを確認し、変化なければ
  最大5回まで再送するリトライ方式で吸収している。
- **WinAppDriverの標準入力EOF問題(修正済み — 手動の回避策はもう不要)**:
  WinAppDriver.exeは「Press ENTER to exit.」という標準入力待ちを行うため、
  標準入力がまともなコンソールに繋がっていない状態から `exec.Command` で
  起動すると、リッスンを開始した直後に標準入力のEOFを読んで自分で終了して
  しまう。`os/exec` は Stdin が nil の子プロセスに null デバイスを渡すので、
  対話的ターミナルから実行した場合でもこれは起きうる(症状は
  「起動したがリッスンしなかった」というタイムアウト)。
  現在は `TestMain` が誰も書き込まない `os.Pipe` の読み側を標準入力として
  渡し、その書き込み側をテストバイナリの寿命ぶん開いたままにすることで
  この読み取りをブロックさせ続けている。事前にWinAppDriverを手動起動して
  おく必要はない(既に起動しているインスタンスがあればそれを使い、
  テスト終了後もそのまま残す挙動は従来どおり)。
