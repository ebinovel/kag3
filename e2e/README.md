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
   から `.msi` を取得してインストール(デフォルトパス:
   `C:\Program Files (x86)\Windows Application Driver\WinAppDriver.exe`)。
   別パスに入れた場合は環境変数 `WINAPPDRIVER_PATH` でそのパスを指定する
4. Go 1.25 以上

`TestMain` の自動起動が失敗する環境がある — 対話的なターミナルからなら
通常問題ないが、詳細と回避策は「既知のこの環境固有の制約」を参照。

## 実行方法

```powershell
# 1. example をビルド(kag3リポジトリルートで実行)
go build -o e2e/testdata/kag3example.exe ./example

# 2. E2E テスト実行
cd e2e
go test ./... -v
```

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
├── main_test.go   TestMain(WinAppDriver自動起動)
└── flows_test.go  実際のE2Eフロー
```

## kag3本体への追加フック

`e2e/` から外部プロセスとして起動する都合上、kag3本体には最小限の環境変数
フックが2つ入っている(いずれも未設定時は既存の挙動を完全維持):

- `KAG3_SAVE_DIR`: セーブ先ディレクトリを絶対パスで上書き
  (`renderer/ebitengine/tags_save.go` の `saveDir()`)
- `KAG3_E2E_FAST`: 起動時に `textNoWait` を強制 `true` にする
  (`renderer/ebitengine/tags_message.go`) — 文字送りアニメーションを
  スキップし、E2Eの待ち時間を大幅に削減する

## 座標・行数がハードコードされている理由と注意点

`nav.go`/`flows_test.go` には `example/resources/senarios/title.ks`・
`scene1.ks` のボタン座標やEnterキーを送る回数がハードコードされている。
ebitengineのウィンドウには要素検索の手段がなく、座標クリックとEnterキー
連打で操作するしかないため。**シナリオファイル(`title.ks`/`scene1.ks`)や
ボタン画像を変更した場合、これらの値は再計算が必要**:

- ボタン座標は `[button x= y=]` のタグ属性 + 実際の画像ファイルサイズ
  (`file example/resources/images/**/*.png` で確認可能)の中心座標
- Enter回数は多くの箇所で「多めに見積もって、想定より先に進んでも
  安全な位置で止まる」設計にしてあるが、`openingLinesBeforeGlink`
  (glink選択肢直前)のようにピンポイントで数える必要がある箇所もある

いずれも、狙った操作が失敗した場合は後続のアサーション(ファイルが
存在しない・画面内容が変化していない等)で明確にテストが失敗するように
してあるので、「間違った位置を操作して誤ってテストが通ってしまう」
リスクは低いはず。

## 実機確認済みの内容

3フローとも実機(Windows 11)で `go test ./... -v` を通してPASS確認済み
(各テスト60〜75秒程度、フルスイートで3〜4分程度)。

- `TestQuickSaveWritesCurrentPtextsPosition`
- `TestTitleReturnDoesNotLeakPreviousPlaythroughStyle`
- `TestQuickSaveThenLoadRestoresSceneText`

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
- **`TestMain` によるWinAppDriverの自動起動が非対話シェルから失敗する
  ことがある**: WinAppDriver.exeは「Press ENTER to exit.」という標準入力
  待ちを行うため、標準入力がまともなコンソールに繋がっていない状態
  (このセッションを含む、ツール経由でコマンドを実行する非対話シェル)
  から `exec.Command` で直接起動すると、リッスンを開始した直後に標準
  入力のEOFを読んで自分で終了してしまう。この場合は`go test`を実行する
  前に、リダイレクトなしで(自分のコンソールを持たせて)WinAppDriverを
  事前に起動しておく——例えば PowerShell で
  `Start-Process "C:\Program Files (x86)\Windows Application Driver\WinAppDriver.exe" -WindowStyle Hidden`
  としてから `go test ./... -v` を実行する。通常の対話的な PowerShell/
  cmd ターミナルから直接 `go test` を実行する場合はこの問題は起きない
  (`TestMain` の自動起動で問題なく動く)。
