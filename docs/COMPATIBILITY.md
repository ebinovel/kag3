# TyranoScript 互換性

kag3 が [TyranoScript](https://tyrano.jp/) のどのタグ・機能に対応しているかをまとめたものです。
タグの実体は `renderer/ebitengine/tags_*.go` の `init()` で `register("タグ名", handler)` してあるものが
一覧の基準になります(未登録のタグは「未実装のタグです」というログを出したうえで処理を継続し、
ゲームを止めません — `renderer/ebitengine/dispatch.go` の `dispatchTag` を参照)。

対応状況は https://tyrano.jp/tag のタグリファレンス(V6)を基準に照合しています。

## 対応方針

3D・AR・HTML/CSS・パッチ配信は対象外です。これらを除いた実用スコープでは、
ほぼ全てのタグに対応しています。動画は`[movie]`(全画面再生)・`[bgmovie]`系(背景としての
ループ再生)・`[layermode_movie]`(合成モード付きのレイヤー動画)すべてに対応しています。

## 🎤 読み上げ機能(VOICEVOX CORE) — kag3だけの機能です

`speak_on`/`speak_off`はtyrano.jp/tagにタグ名としては載っていますが、本家の実装がどう動くかは
未確認です(ブラウザのWeb Speech APIを想定していると思われます)。**kag3の実装はそれとは別物で、
[VOICEVOX CORE](https://github.com/VOICEVOX/voicevox_core)によるオフラインの高品質ニューラル音声合成**
です。ネットワーク接続不要で、シナリオのセリフをそのまま声優品質の音声で読み上げられる、
TyranoScript本家には無いkag3独自の目玉機能です。

導入方法・タグの使い方・**利用規約とクレジット表記(重要)**は[docs/VOICEVOX.md](VOICEVOX.md)を
必ず参照してください。

## 🎨 PSD立ち絵読み込み — kag3だけの機能です

`[chara_new_psd]`はTyranoScript本家に相当するタグの無い、kag3独自の拡張タグです。表情差分違いの
PNGを1枚ずつ用意する代わりに、レイヤー分けされた1枚のPSDファイルと
[PSDToolFavorites](https://github.com/oov/psd)形式の`.pfv`プリセット定義ファイルから、
キャラクターの立ち絵を直接読み込めます。[oov/psd](https://github.com/oov/psd)でPSDのレイヤー木を
解析し、[raa0121/pfv](https://github.com/raa0121/pfv)で`.pfv`のプリセット(表示するレイヤーの組み
合わせ)を読み、[raa0121/ppi](https://github.com/raa0121/ppi)で実際にレイヤーを合成します。
`.pfv`が定義するプリセットはそれぞれ1つの表情/ポーズとして`[chara_show face=]`/`[chara_mod face=]`
にそのまま渡せます(`[chara_face]`を1つずつ呼ぶのと同じ結果になります)。

```
[chara_new_psd name="akane" storage="chara/akane.psd" favorite="chara/akane.pfv" face="通常"]
[chara_show name="akane"]
[chara_mod name="akane" face="笑顔"]
```

セーブ/ロードにも対応しています(生成した画像そのものではなく、PSD/pfv/プリセット名を指す文字列を
`charas[name].Storage`/`Faces`に保存し、ロード時に読み込み直す設計 — 新規プロセスでロードしても
`[chara_new_psd]`を再実行する必要はありません)。

## 🎬 動画再生

`speak_on`/`speak_off`と同様、`movie`/`bgmovie`/`layermode_movie`はtyrano.jp/tagに載っている
タグ名ですが、kag3の実装は[github.com/liqmix/govid](https://github.com/liqMix/govid)(純Go・
cgo不使用・ffmpeg不要の動画デコードライブラリ)によるものです。MP4(H.264)・WebM(VP8)・
MPEG-1に対応しています。

```
[movie storage="op.mp4" se="op.ogg" skip=true]
[bgmovie storage="bg_loop.webm" loop=true]
[layermode_movie video="fire.webm" mode="screen"]
```

- `[movie]`: フルスクリーンの一時再生(既定でコルーチンをブロック、クリックでスキップ可能)
- `[bgmovie]`/`[wait_bgmovie]`/`[stop_bgmovie]`: `[bg]`と同じ「背景として敷きっぱなしにする」
  ループ再生。既定`loop=true`
- `[layermode_movie]`: 既存のシーンの上に合成モード(`mode=` — `normal`/`add`/`multiply`/
  `screen`、`[layermode]`と共通実装)付きで重ねるオーバーレイ動画

**govidは映像のみで、動画ファイル自体の音声トラックは一切デコードできません。** 音声が必要な場合は
`se=`属性で別の音声ファイルを指定し、`[playse]`と同じ`ses[]`経由で同時再生してください
(`[layermode_movie se=]`はkag3独自の拡張です — 本家の属性一覧には無く、同じ理由で追加しています)。
AV1・Theora(`.ogv`/`.ogg`)コーデックは対応していません(govid自身が「AV1デコーダは未完成」
としており、Theoraデコーダはそもそも存在しないため、いずれもエラーになります —
H.264/VP8/MPEG-1を使ってください)。詳細・エンコード方法は[docs/VIDEO.md](VIDEO.md)を
参照してください。

## カテゴリ別対応状況

| カテゴリ | 対応状況 | 備考 |
| --- | --- | --- |
| メッセージ・テキスト | 全対応 | `l` `p` `graph` `r` `er` `cm` `ct` `current` `fuki_start` `fuki_stop` `fuki_chara` `ptext` `mtext` `ruby` `mark` `endmark` |
| メッセージ関連の設定 | 全対応 | 既読テキストの追跡・既読のみスキップ(`unreadskip_config`、`config_record_label`)・既読テキストの色分け(`config_record_label color=`)に対応 — 既読判定の粒度については下記「既知の制約」参照 |
| ラベル・ジャンプ操作 | 全対応 | `jump` `link`/`endlink` `button` `glink` `glink_config` `clickable` |
| キャラクター操作 | 全対応 | `chara_show` `chara_mod` `chara_move` `chara_layer` `chara_ptext` 等、パーツ制御まで含め対応。kag3独自拡張の`chara_new_psd`でPSD立ち絵読み込みにも対応(上記「🎨 PSD立ち絵読み込み」参照) |
| 画像・背景・レイヤ操作 | 全対応 | `bg` `bg2` `image` `trans` `locate` `layopt` 等 |
| 演出・効果・動画 | 全対応 | `quake` `filter` `mask` などの演出、`movie`/`bgmovie`系/`layermode_movie`(govid経由の動画再生、`layermode`とmultiply/screen合成モードを共有)に対応。詳細は上記「🎬 動画再生」参照 |
| アニメーション | 全対応 | `anim` `keyframe` `kanim` `xanim` 系すべて |
| カメラ操作 | 全対応 | `camera` `reset_camera` `wait_camera` |
| システム操作 | パッチ配信のみ未対応 | `save`/`load` 系・`rollback`・`dialog` 等は対応。`apply_local_patch` `check_web_patch` は対象外(Non-goal) |
| システムデザイン変更 | 全対応 | `glyph` 系・`showmenubutton` `sysview` `dialog_config` 系など |
| メニュー・HTML表示 | HTML関連のみ未対応 | `showsave` `showload` `showmenu` `showlog` `web`(既定ブラウザでURLを開く)は対応。`html` `endhtml` はHTML/CSS非対応につき対象外(Non-goal) |
| マクロ・分岐・サブルーチン | 全対応 | `if`/`elsif`/`else`/`endif`、`macro`/`endmacro`(パーサー側で処理)、`call`/`return` 等 |
| 変数・JS操作・ファイル読込 | `loadcss` のみ未対応 | `iscript`/`endscript`・`eval`・`emb` は [goja](https://github.com/dop251/goja) 埋め込みで評価。`loadcss` はHTML/CSS非対応につき対象外(Non-goal) |
| オーディオ | 全対応 | BGM/SE の再生・フェード・音量調整まですべて |
| ボイス・読み上げ | 全対応 | `voconfig` `vostart` `vostop` によるボイスファイルの自動再生に加え、`speak_on` `speak_off` によるVOICEVOX CORE経由のオフライン音声合成に対応(kag3独自拡張の`speak_config`でキャラごとの声を設定)。詳細は上記「🎤 読み上げ機能」参照 |
| 入力フォーム | 全対応 | `edit`(ASCIIのみ、IME連動は未対応) `commit` |
| 3D関連 | 未対応(Non-goal) | `3d_*` 系タグ約36本すべて |
| AR関連 | 未対応(Non-goal) | `bgcamera` `qr_*` 系タグすべて |

## 既知の制約

- **既読テキストの追跡には対応、ただし判定の粒度が本家と異なる**: どのシナリオファイルのどの行を
  表示済みかをプロセス内メモリで記録し(`renderer/ebitengine/tags_message.go` の `readLines`/
  `markLineRead`)、操作行の「既読SKIP」ボタン / `[unreadskip_config mode="read_only"]` /
  `[config_record_label skip="false"]` を有効にすると、SKIPが未読の行では通常速度に落ちてクリック待ちに
  戻り、既読の行だけ高速に読み飛ばします。`[config_record_label color=]` で既読テキストの色分けにも
  対応しています。TyranoScript本家は既読判定が**ラベル単位**(あるラベルから次のラベルに到達した時点で、
  その間の文章がまとめて既読になる)ですが、kag3は**行単位・表示した瞬間に即記録**という簡略化した実装
  です。実用上の見た目はほぼ同じですが、ラベル到達前に中断した場合の既読/未読の境界が本家と厳密には
  一致しません。セーブデータには含めず、バックログと同じくプロセス起動中のみ保持する扱いです。
- **セーブデータの永続化は最小限**: 同一プロセス内でのセーブ/ロード・クイックセーブ/ロード・
  チェックポイント/ロールバックは動作しますが、TyranoScript本家のセーブファイル形式との互換は
  目指していません。
- **ボイス自動再生の設定・連番カウンタはセーブされない**: `[voconfig]` / `[vostart]` / `[vostop]` に対応し、
  `#キャラ名` の話者行が現れるたびに `vostorage` の `{number}` を差し替えながら自動再生します
  (`renderer/ebitengine/tags_voice.go`)。再生は `sebuf` の指すSEバッファ経由なので、音量はSE音量設定に従い、
  `[stopse]` `[wse buf=...]`(本家 `[wv]` 相当) `[fadeoutse]` `[changevol buf=...]` がそのまま効きます。
  ただし登録内容と連番カウンタは `charas` や既読情報と同じくプロセス内メモリ保持で、セーブデータには
  含めません — 途中セーブからロードすると、そのシナリオの `[voconfig]` を読み直すまで連番がロード前の
  続きから進みます。タイトルに戻ると設定ごとリセットされます。ファイル形式はエンジン全体と同じく
  **Ogg Vorbisのみ**(本家プロジェクトのボイスはmp3が多いので注意)で、ファイルが見つからない場合はログを
  出してその行のボイスだけスキップします(連番は本家同様そのまま進みます)。ボイスファイルは `voices` の
  `fs.FS` キーがあればそこから、無ければSEと同じ `ses` から読みます(本家もボイスとSEを同じ `data/sound`
  に置きます)。
- **`[edit]` はASCIIのみ**: IME(日本語入力)と連動したテキストボックスの実装は未対応です。
- **読み上げ機能(`speak_on`/`speak_off`)は`config.toml`未設定なら静かに無効**: VOICEVOX CORE
  本体・ONNX Runtime・OpenJTalk辞書・音声モデルはkag3に同梱されておらず、`config.toml` の
  `VoicevoxCorePath`/`VoicevoxOpenJtalkDictPath`/`VoicevoxModelsPath`(いずれも空文字がデフォルト)
  を設定しない限り機能そのものが無効です(エラーにはならず、`[speak_on]`が静かなno-opになるだけ)。
  導入手順・**利用規約とクレジット表記**は[docs/VOICEVOX.md](VOICEVOX.md)を参照してください。
  VOICEVOXのスタイルID一覧を取得するAPI(`voicevox_synthesizer_create_metas_json`等)はまだ
  Go側のラッパー([nanoda](https://github.com/aethiopicuschan/nanoda))に実装されていないため、
  スタイルIDはVOICEVOX公式サイトで確認する必要があります。合成は非同期(バックグラウンドの
  goroutine)で行われるため文字送りは止まりませんが、その分`[wse buf="speech"]`のような
  「読み上げ待ち」は合成中の間は正しく機能しません。Windows/Linux/macOSに加え、Android・iOSでも
  実機での音声合成・再生を確認済みです(`config.toml`の3キーの意味がプラットフォームごとに
  異なります)。wasmのみpuregoにjs/wasmターゲットが無いため非対応です。詳細は
  [docs/VOICEVOX.md](VOICEVOX.md)の「対応プラットフォーム」を参照してください。
- **`[movie]`/`[bgmovie]`/`[layermode_movie]`は動画ファイル自体の音声を再生しません**:
  使っているgovidが映像のみのデコードライブラリのため、音が必要な場合は`se=`属性で別の音声
  ファイルを用意してください(docs/VIDEO.md参照)。AV1・Theora(`.ogv`/`.ogg`)コーデックは
  いずれもエラーになります(AV1はgovidのデコーダが未完成、Theoraはデコーダ自体が存在しない
  ため)。また、これら動画系タグはすべてエントリーポイント側が`fs.FS`マップに`"videos"`キーを
  登録して初めて機能します — 未登録のプロジェクトでは静かなno-opになります
  (`renderer/ebitengine`は他プロジェクトとも共有しているため、この種の設定省略は全て
  「機能を使わないだけ」として扱われます)。
- **`[layermode_movie speed=]`は無視されます**: govidの`Player`型には`Play`/`Pause`/
  `SetLoop`/`State`しか無く、再生速度を変えるAPIがそもそも存在しないためです。
- **`[layermode_movie]`を停止する専用タグは本家に存在しないため、kag3独自の解釈で対応**:
  既定`loop=true`なので、放置すると再生され続けます。`[free_layermode]`を`layer=`省略の
  「全解除」形で呼ぶと、レイヤーの合成モードのリセットと合わせて`[layermode_movie]`も停止する
  ようにしています(`layer=`を指定した個別解除では止まりません)。詳細はdocs/VIDEO.mdの
  「レイヤー合成動画」参照。
- **`[web]`はhttp/https以外のスキームを拒否します**: `.ks`ファイルから`file://`などの
  スキームを渡してOSのURLハンドラに任意の引数を渡せてしまわないようにするためのセキュリティ
  上の制限です。`window.open(url)`(`[iscript]`内)も同じ経路・同じ制限を通ります。

## kag3 独自の拡張タグ

TyranoScript本家には無い、kag3独自のタグです。前半4つはメッセージウィンドウの再デザイン
(操作ボタン行・文字送りゲージ等)向けで、プロジェクト側で `MessageBoxStyle = "redesigned"` を
`config.toml` に設定した場合のみ意味を持ちます。残り2つ(読み上げ機能・PSD立ち絵読み込み)は
それとは独立しています。

- `[opbar_config]` — 操作ボタン行の表示/非表示・クイックセーブ行の有無を設定
- `[msgbox_opacity]` — メッセージ欄の不透明度を設定
- `[unreadskip_config]` — 既読SKIPボタンの状態(`mode="read_only"`/`"all"`)を設定
- `[configsave]` / `[configload]` — `config.ks` が使う `tf` 名前空間の変数をJSONファイルへ保存/復元
- `[speak_config name= style=]` — VOICEVOXのキャラクターごとの読み上げスタイル(声)を設定。
  `name`省略時はモノローグ用のグローバルデフォルト。詳細は[docs/VOICEVOX.md](VOICEVOX.md)
- `[chara_new_psd name= storage= favorite= face= jname= encoding=]` — PSD+`.pfv`からキャラクターの
  立ち絵を読み込む。詳細は上記「🎨 PSD立ち絵読み込み」参照

## 詳細な実装状況を自分で確認する

タグの実装漏れが疑わしい場合は、以下で実際に登録されているタグ名の一覧を得られます。

```sh
grep -rhoE 'register\("[a-zA-Z_0-9]+"' renderer/ebitengine/*.go | sed -E 's/register\("([^"]+)"/\1/' | sort -u
```

未実装のタグに遭遇した場合、ゲームはクラッシュせず継続し、標準出力に
`未実装のタグです: <タグ名> (line <行番号>)` というログを出します。
