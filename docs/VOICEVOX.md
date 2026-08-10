# 🎤 読み上げ機能(VOICEVOX CORE)

`[speak_on]`/`[speak_off]`は、[VOICEVOX CORE](https://github.com/VOICEVOX/voicevox_core)による
**オフラインの高品質ニューラル音声合成**でシナリオのセリフを読み上げる、**TyranoScript本家には無い
kag3独自の機能**です。

`speak_on`/`speak_off`というタグ名自体はtyrano.jp/tagのタグリファレンスにも載っていますが、本家の
実装がどう動くかは未確認です(ブラウザのWeb Speech APIを想定していると思われます)。kag3の実装は
それとは別物で、ネットワーク接続不要・声優品質のずんだもん等の音声をそのままシナリオに使えます。

**このドキュメントの「利用規約とクレジット表記」の節は必ず読んでください。** キャラクターボイスの
利用には法的な制約があります。

## 導入手順

### 1. VOICEVOX CORE 0.16.x 一式を取得する

kag3が使う[nanoda](https://github.com/aethiopicuschan/nanoda)(Go向けラッパー)は
VOICEVOX CORE **0.16.x**系にのみ対応しています(0.15系はビルド済みライブラリのライセンスが
異なるため非対応)。

公式の[ダウンローダー](https://github.com/VOICEVOX/voicevox_core/releases)を使うのが確実です:

```sh
# Windows の例(download-windows-x64.exe をダウンロードしてリネームしたもの)
./download.exe --only onnxruntime dict --output ./runtime   # CPU版のONNX RuntimeとOpenJTalk辞書
./download.exe --only models --models-pattern "0.vvm" --output ./runtime  # 音声モデル(1体だけ指定した例)
```

`--only c-api`(または`--min`)で本体のコアライブラリ(`voicevox_core.dll`/`.so`/`.dylib`)も
別途取得してください。それぞれ実行時に利用規約への同意を求められます(対話プロンプト)。

### 2. ファイルを配置する — 落とし穴に注意

```
your-project/
  voicevox_core.dll          (または libvoicevox_core.so / .dylib)
  voicevox_onnxruntime.dll   (または .so / .dylib) ← 上と同じディレクトリに置くこと!
  dict/
    open_jtalk_dic_utf_8-1.11/
  models/
    0.vvm
    ...
```

**`voicevox_onnxruntime`はコアライブラリと同じディレクトリに置いてください。** コアライブラリは
これをファイル名だけでOSの共有ライブラリ検索パスを使ってロードするため、`VoicevoxCorePath`の
指すディレクトリの**サブディレクトリ**に置くと見つかりません(Windowsの既定のDLL検索順序は
サブディレクトリまでは辿りません)。kag3自身の実装時にも実際に踏んだ落とし穴です。

音声モデル(`.vvm`)は`VoicevoxModelsPath`で指定したディレクトリの**直下**に置いてください
(サブディレクトリは走査しません)。

### 3. `config.toml`に3つのパスを設定する

```toml
VoicevoxCorePath = "voicevox_core.dll"
VoicevoxOpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
VoicevoxModelsPath = "models"
```

パスは実行時のカレントディレクトリからの相対パス(または絶対パス)です。3つとも空文字が
デフォルトで、その場合`[speak_on]`は静かなno-op(エラーにならない)になります — 読み上げ機能を
使わないプロジェクトは何もする必要がありません。

#### プラットフォームごとに別の値を指定する(`VoicevoxPlatform`)

下の「対応プラットフォーム」で説明する通り、この3つの値は**プラットフォームによって意味その
ものが変わります**(デスクトップは実ファイルシステムパス、Androidは`.so`ファイル名+assetsサブ
ディレクトリ名、iOSはアプリバンドル相対のパス断片)。そのため、上記のトップレベルの3キーだけ
だと、Windows/Android/iOSなど複数プラットフォーム向けにビルドするたびに`config.toml`を
手で書き換える必要がありました。

`[VoicevoxPlatform.<goos>]`テーブル(`<goos>`は`windows`/`linux`/`darwin`/`android`/`ios` —
Goの`runtime.GOOS`の値そのもの)を使うと、1つの`config.toml`に全プラットフォーム分の値を
同時に書いておけます。実行時、そのビルドの`runtime.GOOS`に一致するテーブルがあればそちらが
優先され、無ければトップレベルの3キーにフォールバックします:

```toml
# 未設定時のフォールバック(例: Windows向けの値をここに書いておく)
VoicevoxCorePath = "voicevox_core.dll"
VoicevoxOpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
VoicevoxModelsPath = "models"

[VoicevoxPlatform.linux]
CorePath = "libvoicevox_core.so"
OpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
ModelsPath = "models"

[VoicevoxPlatform.darwin]
CorePath = "libvoicevox_core.dylib"
OpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
ModelsPath = "models"

[VoicevoxPlatform.android]
CorePath = "libvoicevox_core.so"
OpenJtalkDictPath = "voicevox/dict/open_jtalk_dic_utf_8-1.11"
ModelsPath = "voicevox/models"

[VoicevoxPlatform.ios]
CorePath = "Frameworks/voicevox_core.framework/voicevox_core"
OpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
ModelsPath = "models"
```

`example/game/resources/config.toml`(kag3リポジトリ自身のデモプロジェクト)も実際にこの形式で
Android/iOS両方の値を持っています。3キーとも空の場合(そのGOOSのテーブルが無く、トップレベルも
未設定)は従来通り`[speak_on]`が静かなno-opになります。

## タグの使い方

```
[speak_config style="3"]
[speak_on]
こんにちは、これはVOICEVOXによる読み上げです。[p]
[speak_off]
```

- `[speak_on]` / `[speak_off]` — これ以降(または以降でない)表示される行を自動的に読み上げるか
  どうかを切り替えます。デフォルトはoff。
- `[speak_config name= style=]` — キャラクターごとに読み上げの声(VOICEVOXのスタイルID)を
  設定する、kag3独自の拡張タグです。`name`を省略すると、個別設定の無いキャラクター・モノローグ用の
  グローバルデフォルトになります。

```
[speak_config name="ずんだもん" style="3"]
[speak_config name="四国めたん" style="2"]
[speak_config style="8"]
[speak_on]
#ずんだもん
ボイスの割り当てが行われました[p]
```

スタイルIDの一覧をkag3から取得するAPIはまだ実装されていません。VOICEVOX公式サイトや、
使用する`.vvm`ファイルに付属する`metas.json`を参照してください。

## ライセンス・クレジット表記について(必ずお読みください)

読み上げ機能を使う(=生成された音声をゲームに含める)場合、以下を必ず守ってください。
**kag3・nanodaはこれらの利用規約を代弁・保証するものではなく、最終的な責任は開発者にあります。**

1. **VOICEVOX ONNX Runtimeの利用規約**: 商用・非商用問わず利用可能、アプリへの組み込み・再配布も
   可能ですが、生成された音声を使う際はキャラクターごとの規約に従うこと、利用の際は
   VOICEVOXを利用したことがわかるクレジット表記が必要、という条件があります。
2. **音声モデル(VVMファイル)は、キャラクターごとに個別の利用規約があります。** 「ずんだもん」
   「四国めたん」等それぞれ規約の文言・条件(商用利用の可否、年齢制限のある表現の可否など)が
   異なります。**配布前に、実際に使用するキャラクターの公式利用規約を必ず確認してください。**
   VOICEVOX公式サイト([https://voicevox.hiroshiba.jp/](https://voicevox.hiroshiba.jp/))の
   キャラクターごとの利用規約ページを参照してください。
3. **クレジット表記が必要です。** 多くのキャラクターは「VOICEVOX:キャラクター名」(例:
   「VOICEVOX:ずんだもん」)という形式のクレジット表記を、生成音声を使った作品に含めることを
   求めています。具体的な表記文言はキャラクターごとの規約に従ってください。
4. **シナリオのテキストが動的に読み上げられる以上、開発者が書いたセリフはすべて「生成された音声」
   に該当します。** `[voconfig]`のような静的な音声ファイルと同様、利用規約・クレジット表記の
   対象になります — 「実行時に合成しているだけだから対象外」ということにはなりません。

## 対応プラットフォーム

- **Windows / Linux / macOS**: 対応。nanodaが[purego](https://github.com/ebitengine/purego)
  (cgo不使用)でVOICEVOX CORE(dll/so/dylib)を直接呼び出します。
- **Android**: 実装済み・**実機で音声合成・再生まで動作確認済み**(`example/mobile/speech_android.go`)。
  以前は「試していない」扱いでしたが、実際にはpurego自体がcgo経由のdlopenシムでAndroidに対応済みで、
  nanodaのプラットフォーム分岐(`darwin || freebsd || linux`)もGoのビルドタグ上GOOS=androidが暗黙に
  `linux`にもマッチするため無改修でコンパイルが通り、VOICEVOX CORE公式もAndroid arm64向けビルド済み
  バイナリを配布しています——「.soをネイティブライブラリとして、辞書/モデルをアセットとして同梱し、
  実行時に実ファイルパスへ解決する」結線部分は実装・実機ビルドとも完了しています。arm64のみ
  (`build-android.ps1`が元々arm64単一ABIビルドのため)。
  - **「`LoadAllModels`でハングする」は誤診断でした**: 当初、`libvoicevox_core.so`のdlopenと
    `NewSynthesizer`は成功するのに`LoadAllModels`から先でログが一切進まず、ハング(デッドロック)
    していると見えていました。実際の原因はgomobileの`internal/mobileinit`が使う`os.Stdout`用ログ
    パイプがAndroid実機上で信頼できないことで、`fmt.Println`によるログそのものが配信されていな
    かっただけでした。`fmt.Fprintln(os.Stderr, ...)`(`os.Stderr`用パイプは正常動作)に切り替えた
    ところ、`LoadAllModels`自体は普通に完走していることが判明しました。
  - **本当のバグ**: `LoadAllModels`の先、`voicevox_synthesizer_tts`が`code=0`(成功)を返しつつ
    `output_wav_length`が`0`のまま返る、というiOSで最初に発見したのと**同一のバグ**
    (新規spawnしたgoroutineが行う最初のネイティブFFI呼び出しに限って出力引数が破棄される現象、
    詳細は下記iOSの節)がAndroidでも独立に再現しました。iOS向けに導入した「1つの永続ワーカー
    goroutineに集約し、実ジョブ処理前に1回ウォームアップ呼び出しを行う」パターン
    (`speechWorker`/`SpeechWarmup`、`renderer/ebitengine/tags_speech.go`)をAndroidにも適用した
    ところ改善しましたが、実機テストでは**ウォームアップ1回だけでは不十分で、同一goroutine内でも
    不定期に複数回この破損が再発する**ことが判明したため、最終的に`mobileSpeechSynth`
    (`example/mobile/speech_common.go`)自体に「実際にWAV本文を読み取ってサイズを確認できるまで
    最大5回リトライする」ロジックを組み込みました。この対策後、実機での`demo_speech.ks`実行で
    3行のセリフすべてが読み上げエラーなく再生されることを確認済みです。
  - Androidだけ`config.toml`の3キーの**意味が変わります**(値の形式そのものが変わるため):
    `VoicevoxCorePath`はnativeLibraryDir配下に置いた`.so`のファイル名(例:
    `"libvoicevox_core.so"`、フルパスではない)、`VoicevoxOpenJtalkDictPath`/`VoicevoxModelsPath`は
    APKの`assets/`配下のサブディレクトリ名(例: `"voicevox/dict/open_jtalk_dic_utf_8-1.11"`
    `"voicevox/models"`)です。実行時に`Context.getFilesDir()`配下へ自動展開されます。
    デスクトップと違い、絶対/相対のファイルシステムパスをそのまま書いても機能しません。
- **iOS**: 実装済み・**シミュレータで音声合成・再生まで動作確認済み**
  (`example/mobile/speech_ios.go`)。以前は「VOICEVOX COREを静的xcframeworkとして配布しているため
  dlopenできない」としていましたが誤りで、`voicevox_core-ios-xcframework-cpu-*.zip`/
  `voicevox_onnxruntime-ios-*.tgz`いずれも中身はMach-O**動的**共有ライブラリ(`file`コマンドで
  `Mach-O 64-bit arm64 dynamically linked shared library`と確認済み)——xcframeworkという配布形式
  自体は静的・動的どちらも格納できる箱であって、静的専用ではありません。puregoのdlopen実装
  (`dlfcn_darwin.go`)もiOSでビルドされます(GoのビルドタグルールでGOOS=iosは暗黙に`darwin`にも
  マッチするため、Android同様cgo必須)。
  - Androidと違い、iOSには「JVM/Contextを実行時に登録する」という仕組み自体が存在せず
    (`github.com/ebitengine/gomobile/app`のiOS実装にAndroidの`RunOnJVM`に相当する公開APIが無い)、
    起動レース対策のリトライループも不要です。`os.Executable()`からアプリバンドルのルートを求め、
    そこからの相対パスを組み立てるだけの純Goコード(`example/mobile/voicevoxpaths.ResolveIOS`)で
    パス解決が完結します——cgo/Objective-Cのブリッジコードも書いていません。
  - iOSのバンドルリソース(Xcodeの「Bundle Resources」で追加したファイル)はビルド時点で`.app`内に
    実ファイルとして配置されるため、Androidのassetsのような実行時展開処理も不要です。フラットな
    バンドル構造(実行ファイルとリソースフォルダが同階層)であることも実機シミュレータで確認済み。
  - iOSも`config.toml`の3キーの意味がデスクトップと異なります(バンドル相対パスの断片):
    ```toml
    VoicevoxCorePath = "Frameworks/voicevox_core.framework/voicevox_core"
    VoicevoxOpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
    VoicevoxModelsPath = "models"
    ```
  - `example/mobile/voicevoxpaths.ResolveIOS`のパス組み立てロジック自体はWindows上でも
    ユニットテスト済み(`go test ./example/mobile/voicevoxpaths/...`)。
  - **見つかった実バグ・修正済み**: iOS版`voicevox_core`は`VOICEVOX_LINK_ONNXRUNTIME`(ONNX
    Runtimeをリンク時の通常の依存として解決——dyldが自動でロードする)のみをエクスポートしており、
    他の全プラットフォーム(Android含む)が使う`VOICEVOX_LOAD_ONNXRUNTIME`
    (`voicevox_get_onnxruntime_lib_versioned_filename`+`voicevox_onnxruntime_load_once`で
    手動dlopen)は存在しません。これは同梱の`voicevox_core.h`自身がプラットフォームごとの仕様として
    明記している仕様差です。nanoda(`../nanoda`、`go.work`/`go.mod`の`replace`で参照するローカル
    フォーク)の`internal/core/core_0_16_0`はLOADモードしか実装していなかったため、
    `voicevox_get_onnxruntime_lib_versioned_filename`のシンボル解決で
    `dlsym: symbol not found`パニックが発生していました。`core_0_16_0`をビルドタグで分岐する
    2ファイル(`onnxruntime_other.go` `//go:build !ios` / `onnxruntime_ios.go` `//go:build ios`)に
    分割し、iOS向けにLINKモードの経路(`voicevox_onnxruntime_init_once`)を実装して修正済み
    (kag3側の回避策ではなく、nanodaフォーク本体への正当な修正)。
  - **見つかった実バグ2・修正済み**: 上記修正後は`voicevox_core`の読み込み・初期化まで成功し、
    `voicevox_synthesizer_tts`も`code=0`(成功)と有効な`output_wav`ポインタを返すものの、
    もう一方の出力引数`output_wav_length`が`0`のまま返ってくる、という症状が発生していました。
    Cの単体テストプログラムで`voicevox_core`自体は問題ないと確認した上で、purego単体の
    Goテストプログラムで原因を特定: **新規生成した(spawnしたばかりの)goroutineが行う
    最初のネイティブ呼び出しに限って出力引数が破棄され、同じgoroutineでの2回目以降の呼び出しは
    正常に動作する**、というこのプラットフォーム固有の現象でした(purego側のgoroutineスタック
    成長とFFI呼び出しトランポリンの競合が有力な原因と見られますが未確定)。`tags_speech.go`が
    `[speak_on]`の行ごとに新規goroutineを生成していたため、実質すべての発話がこのバグに
    該当していました。修正: 行ごとに新規goroutineを立てるのをやめ、プロセス寿命で1つだけ
    永続するワーカーgoroutineに集約し、実際のジョブを処理する前に1回だけ使い捨ての
    ウォームアップ呼び出しを行うようにしました(`renderer/ebitengine/tags_speech.go`、
    kag3側の共有コードでの修正——iOS固有の回避策ではなく、他プラットフォームでも安全)。
    シミュレータで`[speak_on]`のエラーログが一切出ず、音声合成・再生まで動作することを確認済み。
    後日Android実機でも同一バグの再現が確認され(上記Androidの節)、そちらでは「ウォームアップ1回
    だけでは不十分で、同一goroutine内でも不定期に複数回再発しうる」ことも判明したため、
    `mobileSpeechSynth`(`example/mobile/speech_common.go`、Android/iOS共有コード)自体に
    最大5回のリトライを組み込む形に強化されています——iOS側もこの強化の恩恵を受けます。
- **wasm**: 非対応。puregoにはjs/wasmターゲットがありません。

wasmでは`config.toml`にパスを設定していても`[speak_on]`は静かなno-opになります
(ビルドが壊れることはありません — `renderer/ebitengine`自体はVOICEVOX CORE/nanodaに一切依存せず、
`example/game`・`example/mobile`の中のプラットフォーム別ファイルが対応プラットフォームでのみ実際の
結線を行う設計です)。
