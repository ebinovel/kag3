# TyranoScript 互換性

kag3 が [TyranoScript](https://tyrano.jp/) のどのタグ・機能に対応しているかをまとめたものです。
タグの実体は `renderer/ebitengine/tags_*.go` の `init()` で `register("タグ名", handler)` してあるものが
一覧の基準になります(未登録のタグは「未実装のタグです」というログを出したうえで処理を継続し、
ゲームを止めません — `renderer/ebitengine/dispatch.go` の `dispatchTag` を参照)。

対応状況は https://tyrano.jp/tag のタグリファレンス(V6)を基準に照合しています。

## 対応方針

3D・AR・動画・HTML/CSS・音声合成(読み上げ)・パッチ配信は対象外です。これらを除いた実用スコープでは、
ほぼ全てのタグに対応しています。

## カテゴリ別対応状況

| カテゴリ | 対応状況 | 備考 |
| --- | --- | --- |
| メッセージ・テキスト | 全対応 | `l` `p` `graph` `r` `er` `cm` `ct` `current` `fuki_start` `fuki_stop` `fuki_chara` `ptext` `mtext` `ruby` `mark` `endmark` |
| メッセージ関連の設定 | 全対応 | 既読テキストの追跡・既読のみスキップ(`unreadskip_config`、`config_record_label`)・既読テキストの色分け(`config_record_label color=`)に対応 — 既読判定の粒度については下記「既知の制約」参照 |
| ラベル・ジャンプ操作 | 全対応 | `jump` `link`/`endlink` `button` `glink` `glink_config` `clickable` |
| キャラクター操作 | 全対応 | `chara_show` `chara_mod` `chara_move` `chara_layer` 等、パーツ制御まで含め対応 |
| 画像・背景・レイヤ操作 | 全対応 | `bg` `bg2` `image` `trans` `locate` `layopt` 等 |
| 演出・効果・動画 | 動画関連のみ未対応 | `quake` `filter` `mask` などの演出は対応。`movie` `bgmovie` `wait_bgmovie` `stop_bgmovie` `layermode_movie` は対象外(Non-goal) |
| アニメーション | 全対応 | `anim` `keyframe` `kanim` `xanim` 系すべて |
| カメラ操作 | 全対応 | `camera` `reset_camera` `wait_camera` |
| システム操作 | パッチ配信のみ未対応 | `save`/`load` 系・`rollback`・`dialog` 等は対応。`apply_local_patch` `check_web_patch` は対象外(Non-goal) |
| システムデザイン変更 | 全対応 | `glyph` 系・`showmenubutton` `sysview` `dialog_config` 系など |
| メニュー・HTML表示 | HTML関連のみ未対応 | `showsave` `showload` `showmenu` `showlog` は対応。`html` `endhtml` `web` は対象外(Non-goal) |
| マクロ・分岐・サブルーチン | 全対応 | `if`/`elsif`/`else`/`endif`、`macro`/`endmacro`(パーサー側で処理)、`call`/`return` 等 |
| 変数・JS操作・ファイル読込 | `loadcss` のみ未対応 | `iscript`/`endscript`・`eval`・`emb` は [goja](https://github.com/dop251/goja) 埋め込みで評価。`loadcss` はHTML/CSS非対応につき対象外(Non-goal) |
| オーディオ | 全対応 | BGM/SE の再生・フェード・音量調整まですべて |
| ボイス・読み上げ | 未対応(Non-goal) | `voconfig` `vostart` `vostop` `speak_on` `speak_off` |
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
- **`[edit]` はASCIIのみ**: IME(日本語入力)と連動したテキストボックスの実装は未対応です。

## kag3 独自の拡張タグ

メッセージウィンドウの再デザイン(操作ボタン行・文字送りゲージ等)向けに、TyranoScript本家には
無い以下のタグを追加しています。プロジェクト側で `MessageBoxStyle = "redesigned"` を
`config.toml` に設定した場合のみ意味を持ちます。

- `[opbar_config]` — 操作ボタン行の表示/非表示・クイックセーブ行の有無を設定
- `[msgbox_opacity]` — メッセージ欄の不透明度を設定
- `[unreadskip_config]` — 既読SKIPボタンの状態(`mode="read_only"`/`"all"`)を設定
- `[configsave]` / `[configload]` — `config.ks` が使う `tf` 名前空間の変数をJSONファイルへ保存/復元

## 詳細な実装状況を自分で確認する

タグの実装漏れが疑わしい場合は、以下で実際に登録されているタグ名の一覧を得られます。

```sh
grep -rhoE 'register\("[a-zA-Z_0-9]+"' renderer/ebitengine/*.go | sed -E 's/register\("([^"]+)"/\1/' | sort -u
```

未実装のタグに遭遇した場合、ゲームはクラッシュせず継続し、標準出力に
`未実装のタグです: <タグ名> (line <行番号>)` というログを出します。
