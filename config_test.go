package kag3

import (
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestLoadDefaultContinueMarkStyleDefaultsToFollow(t *testing.T) {
	c := &Config{}
	c.LoadDefault()
	if c.ContinueMarkStyle != "follow" {
		t.Errorf("ContinueMarkStyle = %q, want %q (backward-compatible default)", c.ContinueMarkStyle, "follow")
	}
}

func TestVoicevoxPathsFallsBackToFlatFieldsWhenNoPlatformEntry(t *testing.T) {
	c := &Config{
		VoicevoxCorePath:          "voicevox_core.dll",
		VoicevoxOpenJtalkDictPath: "dict",
		VoicevoxModelsPath:        "models",
	}
	core, dict, models := c.VoicevoxPaths()
	if core != "voicevox_core.dll" || dict != "dict" || models != "models" {
		t.Errorf("VoicevoxPaths() = (%q, %q, %q), want the flat fields unchanged", core, dict, models)
	}
}

func TestVoicevoxPathsUsesPlatformEntryForCurrentGOOS(t *testing.T) {
	c := &Config{
		VoicevoxCorePath:          "voicevox_core.dll",
		VoicevoxOpenJtalkDictPath: "dict",
		VoicevoxModelsPath:        "models",
		VoicevoxPlatform: map[string]VoicevoxPlatformPaths{
			runtime.GOOS: {
				CorePath:          "override-core",
				OpenJtalkDictPath: "override-dict",
				ModelsPath:        "override-models",
			},
		},
	}
	core, dict, models := c.VoicevoxPaths()
	if core != "override-core" || dict != "override-dict" || models != "override-models" {
		t.Errorf("VoicevoxPaths() = (%q, %q, %q), want the current-GOOS VoicevoxPlatform entry", core, dict, models)
	}
}

func TestVoicevoxPathsIgnoresEntryForOtherGOOS(t *testing.T) {
	c := &Config{
		VoicevoxCorePath:          "voicevox_core.dll",
		VoicevoxOpenJtalkDictPath: "dict",
		VoicevoxModelsPath:        "models",
		VoicevoxPlatform: map[string]VoicevoxPlatformPaths{
			"not-" + runtime.GOOS: {
				CorePath:          "wrong-platform-core",
				OpenJtalkDictPath: "wrong-platform-dict",
				ModelsPath:        "wrong-platform-models",
			},
		},
	}
	core, dict, models := c.VoicevoxPaths()
	if core != "voicevox_core.dll" || dict != "dict" || models != "models" {
		t.Errorf("VoicevoxPaths() = (%q, %q, %q), want the flat fields since no entry matches runtime.GOOS %q", core, dict, models, runtime.GOOS)
	}
}

func TestLoadDecodesVoicevoxPlatformTable(t *testing.T) {
	src := `
VoicevoxCorePath = "voicevox_core.dll"
VoicevoxOpenJtalkDictPath = "dict"
VoicevoxModelsPath = "models"

[VoicevoxPlatform.android]
CorePath = "libvoicevox_core.so"
OpenJtalkDictPath = "voicevox/dict"
ModelsPath = "voicevox/models"

[VoicevoxPlatform.ios]
CorePath = "Frameworks/voicevox_core.framework/voicevox_core"
OpenJtalkDictPath = "dict"
ModelsPath = "models"
`
	var c Config
	if _, err := toml.Decode(src, &c); err != nil {
		t.Fatalf("toml.Decode failed: %v", err)
	}
	android, ok := c.VoicevoxPlatform["android"]
	if !ok {
		t.Fatal("VoicevoxPlatform[\"android\"] missing after decode")
	}
	if android.CorePath != "libvoicevox_core.so" || android.OpenJtalkDictPath != "voicevox/dict" || android.ModelsPath != "voicevox/models" {
		t.Errorf("VoicevoxPlatform[\"android\"] = %+v, want the android table's values", android)
	}
	ios, ok := c.VoicevoxPlatform["ios"]
	if !ok {
		t.Fatal("VoicevoxPlatform[\"ios\"] missing after decode")
	}
	if ios.CorePath != "Frameworks/voicevox_core.framework/voicevox_core" {
		t.Errorf("VoicevoxPlatform[\"ios\"].CorePath = %q, want the ios table's value", ios.CorePath)
	}
}
