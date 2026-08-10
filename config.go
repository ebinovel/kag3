package kag3

import (
	"errors"
	"io/fs"
	"runtime"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Title string `toml:"Title"`
	ScreenRatio string `toml:"ScreenRatio"`
	ScreenCentering bool `toml:"ScreenCentering"`
	UseCamera bool `toml:"UseCamera"`
	Use3D bool `toml:"Use3D"`
	ScreenWidth int `toml:"ScreenWidth"`
	ScreenHeight int `toml:"ScreenHeight"`
	ChSpeed int `toml:"ChSpeed"`
	ChSpeeds Speeds `toml:"ChSpeeds"`
	SkipSpeed int `toml:"SkipSpeed"`
	SkipEffectIgnore bool `toml:"SkipEffectIgnore"`
	AutoSpeed int `toml:"AutoSpeed"`
	AutoClickStop bool `toml:"AutoClickStop"`
	AutoSpeedWithText int `toml:"AutoSpeedWithText"`
	CursorDefault string `toml:"CursorDefault"`
	MediaFormatDefault string `toml:"MediaFormatDefault"`
	DefaultBgmVolume int `toml:"DefaultBgmVolume"`
	DefaultSeVolume int `toml:"DefaultSeVolume"`
	DefaultMovieVolume int `toml:"DefaultMovieVolume"`
	DefaultBgmSlotNum int `toml:"DefaultBgmSlotNum"`
	DefaultSoundSlotNum int `toml:"DefaultSoundSlotNum"`
	ConfigVisible bool `toml:"ConfigVisible"`
	ConfigLeft int `toml:"ConfigLeft"`
	ConfigTop int `toml:"ConfigTop"`
	ConfigSave string `toml:"ConfigSave"`
	ConfigThumbnail bool `toml:"ConfigThumbnail"`
	ConfigThumbnailQuality string `toml:"ConfigThumbnailQuality"`
	ConfigThumbnailScale float64 `toml:"ConfigThumbnailScale"`
	ConfigSaveSlotNum int `toml:"ConfigSaveSlotNum"`
	ConfigSaveDataFormat string `toml:"ConfigSaveDataFormat"`
	ConfigSaveOverWrite bool `toml:"ConfigSaveOverWrite"`
	MaxBackLogNum int `toml:"MaxBackLogNum"`
	AutoRecordLabel bool `toml:"AutoRecordLabel"`
	UnReadTextSkip bool `toml:"UnReadTextSkip"`
	AlreadyReadTextColor string `toml:"AlreadyReadTextColor"`
	CharacterLayerNum int `toml:"CharacterLayerNum"`
	ScreenPositionX ScreenPosition `toml:"ScreenPositionX"`
	MessageLayerNum int `toml:"MessageLayerNum"`
	InitialMessageLayerVisible bool `toml:"InitialMessageLayerVisible"`
	MarginL int `toml:"MarginL"`
	MarginT int `toml:"MarginT"`
	MarginR int `toml:"MarginR"`
	MarginB int `toml:"MarginB"`
	ML int `toml:"ML"`
	MT int `toml:"MT"`
	MW int `toml:"MW"`
	MH int `toml:"MH"`
	DebugMenu DebugMenu `toml:"DebugMenu"`
	FrameColor string `toml:"FrameColor"`
	FrameOpacity int `toml:"FrameOpacity"`
	DefaultAutoReturn bool `toml:"DefaultAutoReturn"`
	MarginRCH int `toml:"MarginRCH"`
	DefaultFontSize int `toml:"DefaultFontSize"`
	DefaultLineSpacing int `toml:"DefaultLineSpacing"`
	DefaultPitch int `toml:"DefaultPitch"`
	UseFace string `toml:"UseFace"`
	DefaultCharColor string `toml:"DefaultCharColor"`
	DefaultBold bool `toml:"DefaultBold"`
	DefaultRubySize int `toml:"DefaultRubySize"`
	DefaultRubyOffset int `toml:"DefaultRubyOffset"`
	DefaultAntiAliaced int `toml:"DefaultAntiAliaced"`
	DefaultShadow bool `toml:"DefaultShadow"`
	DefaultShadowColor string `toml:"DefaultShadowColor"`
	DefaultEdge bool `toml:"DefaultEdge"`
	DefaultEdgeColor string `toml:"DefaultEdgeColor"`
	DefaultLinkColor string `toml:"DefaultLinkColor"`
	DefaultLinkOpacity int `toml:"DefaultLinkOpacity"`
	Vertial bool `toml:"Vertial"`
	UseKeyFocus bool `toml:"UseKeyFocus"`
	FirstKeyFocusType string `toml:"FirstKeyFocusType"`
	KeyFocusWithMouseCursor bool `toml:"KeyFocusWithMouseCursor"`
	KeyFocusWithHoverStyle bool `toml:"KeyFocusWithHoverStyle"`
	KeyFocusOutlineWidth int `toml:"KeyFocusOutlineWidth"`
	KeyFocusOutlineStyle string `toml:"KeyFocusOutlineStyle"`
	KeyFocusOutlineColor string `toml:"KeyFocusOutlineColor"`
	KeyFocusOutlineAnim string `toml:"KeyFocusOutlineAnim"`
	KeyFocusOutlineAnimDuration int `toml:"KeyFocusOutlineAnimDuration"`
	UseGamepad bool `toml:"UseGamepad"`
	UseCloseConfirm bool `toml:"UseCloseConfirm"`
	OffscreenClickable bool `toml:"OffscreenClickable"`
	// ContinueMarkStyle picks the message window's "waiting for a click"
	// indicator: "follow" (default) keeps the existing text-following
	// glyphNormal mark (tags_sysdesign.go) untouched; "fixed" switches to a
	// fixed bottom-right "▽" (drawContinueMark, draw_messagebox.go).
	ContinueMarkStyle string `toml:"ContinueMarkStyle"`
	// MessageBoxStyle gates the whole メッセージ欄 "2a" redesign —
	// renderer/ebitengine is a shared package, not example-specific code, so
	// every project importing kag3 (this repo's example, but also sibling
	// projects like tsf-action) picks up any unconditional change here.
	// "legacy" (default) is the original look this package always had
	// before that redesign: flat rgba(0,0,0,0.5) box fill, tight (1.0x)
	// line spacing, no persistent operation row, no in-box text-speed
	// indicator. "redesigned" turns all four on — only example/game/resources/
	// config.toml sets this; a project that never heard of the redesign
	// keeps rendering exactly as it always did, with no code changes on
	// its side. (The [ptext] visibility/color fixes made alongside this
	// redesign are real bugfixes, not part of this style switch, and stay
	// unconditional — see tags_text.go/renderer.go.)
	MessageBoxStyle string `toml:"MessageBoxStyle"`
	// VoicevoxCorePath/VoicevoxOpenJtalkDictPath/VoicevoxModelsPath configure
	// [speak_on]/[speak_off] (tags_speech.go) — kag3's own TTS extension,
	// with no equivalent in real TyranoScript. All three default to "" (see
	// docs/VOICEVOX.md): empty means the feature is simply unavailable,
	// [speak_on] becomes a silent no-op, nothing else changes. renderer/
	// ebitengine never reads these paths itself (it stays voicevox_core-
	// oblivious, same as ConfigStorage/SaveDirFunc) — an entrypoint-side
	// file (e.g. example/game/speech_desktop.go, example/mobile/speech_android.go)
	// calls VoicevoxPaths() (below) to construct the actual synthesizer and
	// wire ebitengine.SpeechSynth, rather than reading these three fields
	// directly. These three act as the fallback VoicevoxPaths() returns
	// when VoicevoxPlatform has no entry for the current runtime.GOOS — see
	// that field's own doc comment for why the same three strings mean a
	// different *kind* of path on every platform (desktop: a real
	// filesystem path; Android: a bare .so filename/assets subdirectory
	// name; iOS: an app-bundle-relative path fragment), which is exactly
	// why a single flat triple can't serve every build target from one
	// config.toml. See docs/VOICEVOX.md's "対応プラットフォーム" section for
	// concrete examples.
	VoicevoxCorePath          string `toml:"VoicevoxCorePath"`
	VoicevoxOpenJtalkDictPath string `toml:"VoicevoxOpenJtalkDictPath"`
	VoicevoxModelsPath        string `toml:"VoicevoxModelsPath"`
	// VoicevoxPlatform lets one config.toml carry a *different* Voicevox*
	// path triple per build target, keyed by runtime.GOOS ("windows",
	// "linux", "darwin", "android", "ios") — without it, a project that
	// wants working [speak_on] on more than one platform would have to
	// hand-edit config.toml between builds, since e.g. Android's bare
	// ".so"-filename-plus-assets-subdirectory values are meaningless as a
	// literal filesystem path on Windows, and even Windows/Linux/macOS
	// alone already need different core-library filenames
	// (voicevox_core.dll vs. libvoicevox_core.so vs. libvoicevox_core.dylib).
	// A GOOS key with no entry (or entry omitted from config.toml entirely)
	// falls back to the flat VoicevoxCorePath/VoicevoxOpenJtalkDictPath/
	// VoicevoxModelsPath fields above wholesale — see VoicevoxPaths().
	// Example:
	//
	//	[VoicevoxPlatform.windows]
	//	CorePath = "voicevox_core.dll"
	//	OpenJtalkDictPath = "dict/open_jtalk_dic_utf_8-1.11"
	//	ModelsPath = "models"
	//
	//	[VoicevoxPlatform.android]
	//	CorePath = "libvoicevox_core.so"
	//	OpenJtalkDictPath = "voicevox/dict/open_jtalk_dic_utf_8-1.11"
	//	ModelsPath = "voicevox/models"
	VoicevoxPlatform map[string]VoicevoxPlatformPaths `toml:"VoicevoxPlatform"`
}

// VoicevoxPlatformPaths is one runtime.GOOS entry in Config.VoicevoxPlatform
// — see that field's own doc comment.
type VoicevoxPlatformPaths struct {
	CorePath          string `toml:"CorePath"`
	OpenJtalkDictPath string `toml:"OpenJtalkDictPath"`
	ModelsPath        string `toml:"ModelsPath"`
}

// VoicevoxPaths resolves the (corePath, openJtalkDictPath, modelsPath)
// triple to actually use for the current runtime.GOOS build: c's
// VoicevoxPlatform[runtime.GOOS] entry if config.toml set one, otherwise
// the flat VoicevoxCorePath/VoicevoxOpenJtalkDictPath/VoicevoxModelsPath
// fields. Entrypoint-side TTS wiring (speech_desktop.go,
// speech_android.go, voicevoxpaths.Resolve/ResolveIOS) should always go
// through this rather than reading the Voicevox* fields directly, so a
// single config.toml transparently serves every platform it has an entry
// (or a working fallback) for. Whichever triple is returned keeps the
// existing "" == disabled contract: callers already check all three for
// emptiness before treating the feature as configured.
func (c *Config) VoicevoxPaths() (corePath, openJtalkDictPath, modelsPath string) {
	if p, ok := c.VoicevoxPlatform[runtime.GOOS]; ok {
		return p.CorePath, p.OpenJtalkDictPath, p.ModelsPath
	}
	return c.VoicevoxCorePath, c.VoicevoxOpenJtalkDictPath, c.VoicevoxModelsPath
}

type Speeds struct {
	Fast int `toml:"Fast"`
	Normal int `toml:"Normal"`
	Slow int `toml:"Slow"`
}

type ScreenPosition struct {
	Left int `toml:"Left"`
	LeftCenter int `toml:"LeftCenter"`
	Center int `toml:"Center"`
	RightCenter int `toml:"RightCenter"`
	Right int `toml:"Right"`
	L int `toml:"L"`
	LC int `toml:"LC"`
	C int `toml:"C"`
	RC int `toml:"RC"`
	R int `toml:"R"`
}

type DebugMenu struct {
	Visible bool `toml:"Visible"`
}

func (c *Config) LoadDefault() {
	left, leftCenter, center, rightCenter, right := 160, 240, 320, 400, 480
	c.Title = "エビノベル"
	c.ScreenRatio = "fix"
	c.ScreenCentering = true
	c.UseCamera = true
	c.Use3D = false
	c.ScreenWidth = 1280
	c.ScreenHeight = 720
	c.ChSpeed = 30
	c.ChSpeeds = Speeds{
		Fast: 10,
		Normal: 30,
		Slow: 50,
	}
	c.SkipSpeed = 30
	c.SkipEffectIgnore = true
	c.AutoSpeed = 1300
	c.AutoClickStop = true
	c.AutoSpeedWithText = 60
	c.CursorDefault = "default"
	c.MediaFormatDefault = "ogg"
	c.DefaultBgmVolume = 100
	c.DefaultSeVolume = 100
	c.DefaultMovieVolume = 100
	c.DefaultBgmSlotNum = 1
	c.DefaultSoundSlotNum = 3

	c.ConfigVisible = true
	c.ConfigLeft = -1
	c.ConfigTop = -1
	c.ConfigSave = "webstorage"
	c.ConfigThumbnail = true
	c.ConfigThumbnailQuality = "meddle"
	c.ConfigThumbnailScale = 0.3
	c.ConfigSaveSlotNum = 5
	c.ConfigSaveDataFormat = "yyyy/MM/dd hh:mm:ss"
	c.ConfigSaveOverWrite = false
	c.MaxBackLogNum = 50
	c.AutoRecordLabel = false
	c.UnReadTextSkip = false
	c.AlreadyReadTextColor = "0x87cefa"
	c.CharacterLayerNum = 3
	c.ScreenPositionX= ScreenPosition{
		Left: left,
		LeftCenter: leftCenter,
		Center: center,
		RightCenter: rightCenter,
		Right: right,
		L: left,
		LC: leftCenter,
		C: center,
		RC: rightCenter,
		R: right,
	}
	c.MessageLayerNum = 2
	c.InitialMessageLayerVisible = true
	c.MarginL = 8
	c.MarginT = 8
	c.MarginR = 8
	c.MarginB = 8
	c.ML = 16
	c.MT = 16
	c.MW = 960-32
	c.MH = 640-32
	c.DebugMenu = DebugMenu{
		Visible: true,
	}
	c.FrameColor = "0x0000000"
	c.FrameOpacity = 128
	c.DefaultAutoReturn = true
	c.MarginRCH = 2
	c.DefaultFontSize = 28
	c.ContinueMarkStyle = "follow"
	c.MessageBoxStyle = "legacy"
	// Left "" (Go's zero value) deliberately, not just left unset: this is
	// the documented off-switch for [speak_on]/[speak_off], not merely an
	// omitted field. See the struct field's own doc comment.
	c.VoicevoxCorePath = ""
	c.VoicevoxOpenJtalkDictPath = ""
	c.VoicevoxModelsPath = ""
}

func (c *Config) Load(dir fs.FS, file string) {
	_, err := fs.Stat(dir, file)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	b, err := fs.ReadFile(dir, file)
	if err != nil {
		panic(err)
	}
	_, err = toml.Decode(string(b), c)
	if err != nil {
		panic(err)
	}

}
