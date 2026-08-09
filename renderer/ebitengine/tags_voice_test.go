package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

// resetVoiceState clears every package-level var these tests touch, and
// restores it afterward. tags_voice.go's registry is process-lifetime by
// design (see its own doc comment), so a [voconfig] left behind by one test
// would make a later one's "unregistered speaker" case pass or fail for
// entirely the wrong reason — the cross-test pollution CLAUDE.md warns
// about for this package generally.
func resetVoiceState(t *testing.T) {
	t.Helper()
	origConfigs, origAutoPlay, origName := voiceConfigs, voiceAutoPlay, charaName
	origSes := ses
	voiceConfigs = map[string]*voiceConfig{}
	voiceAutoPlay = false
	charaName = ""
	ses = map[string]*kag3.BGM{}
	t.Cleanup(func() {
		voiceConfigs, voiceAutoPlay, charaName = origConfigs, origAutoPlay, origName
		ses = origSes
	})
}

// --- A. pure functions (no audio, no filesystem) ---

func TestVoiceStorageNameSubstitutesNumber(t *testing.T) {
	cases := []struct {
		template string
		number   int
		want     string
	}{
		{"akane_{number}.ogg", 1, "akane_1.ogg"},
		{"akane_{number}.ogg", 12, "akane_12.ogg"},
		{"voice/akane_{number}.ogg", 3, "voice/akane_3.ogg"},
		// Tyrano's own docs only ever show one placeholder, but replacing
		// every occurrence is the least surprising reading of it.
		{"{number}/akane_{number}.ogg", 2, "2/akane_2.ogg"},
	}
	for _, c := range cases {
		if got := voiceStorageName(c.template, c.number); got != c.want {
			t.Errorf("voiceStorageName(%q, %d) = %q, want %q", c.template, c.number, got, c.want)
		}
	}
}

// TestVoiceStorageNameWithoutPlaceholderIsUnchanged covers the fixed-filename
// case: a character with a single stock line has no {number} to substitute,
// and must keep resolving to that same file on every speaker line rather
// than having a counter spliced into it.
func TestVoiceStorageNameWithoutPlaceholderIsUnchanged(t *testing.T) {
	for _, n := range []int{1, 2, 99} {
		if got := voiceStorageName("akane.ogg", n); got != "akane.ogg" {
			t.Errorf("voiceStorageName(%q, %d) = %q, want it unchanged", "akane.ogg", n, got)
		}
	}
}

func TestVoicePlaybackForAdvancesNumber(t *testing.T) {
	resetVoiceState(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{SeBuf: "vo", Storage: "akane_{number}.ogg", Number: 1}

	buf, storage, ok := voicePlaybackFor("akane")
	if !ok || buf != "vo" || storage != "akane_1.ogg" {
		t.Fatalf("first call = (%q, %q, %v), want (\"vo\", \"akane_1.ogg\", true)", buf, storage, ok)
	}
	_, storage, ok = voicePlaybackFor("akane")
	if !ok || storage != "akane_2.ogg" {
		t.Fatalf("second call storage = %q (ok=%v), want %q", storage, ok, "akane_2.ogg")
	}
	if got := voiceConfigs["akane"].Number; got != 3 {
		t.Errorf("Number after two lines = %d, want 3", got)
	}
}

func TestVoicePlaybackForDefaultsSeBuf(t *testing.T) {
	resetVoiceState(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	buf, _, ok := voicePlaybackFor("akane")
	if !ok || buf != defaultVoiceSeBuf {
		t.Errorf("buf with sebuf= omitted = %q (ok=%v), want %q", buf, ok, defaultVoiceSeBuf)
	}
}

// TestVoicePlaybackForSkipsWhenNotStarted is the [vostart]-is-mandatory
// rule: registrations made by [voconfig] do nothing until auto-play is
// armed, and crucially must not burn a number while disarmed either.
func TestVoicePlaybackForSkipsWhenNotStarted(t *testing.T) {
	resetVoiceState(t)
	voiceAutoPlay = false
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	if _, _, ok := voicePlaybackFor("akane"); ok {
		t.Error("voicePlaybackFor = ok before [vostart], want false")
	}
	if got := voiceConfigs["akane"].Number; got != 1 {
		t.Errorf("Number = %d while auto-play is off, want it left at 1", got)
	}
}

// TestVoicePlaybackForSkipsEmptyName covers a bare "#" line, which
// characterPText (parser.go) turns into a Chara with an empty Name to clear
// the speaker.
func TestVoicePlaybackForSkipsEmptyName(t *testing.T) {
	resetVoiceState(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	if _, _, ok := voicePlaybackFor(""); ok {
		t.Error("voicePlaybackFor(\"\") = ok, want false for a bare \"#\" line")
	}
	if got := voiceConfigs["akane"].Number; got != 1 {
		t.Errorf("Number = %d after a speaker-clearing line, want it left at 1", got)
	}
}

func TestVoicePlaybackForSkipsUnregisteredChara(t *testing.T) {
	resetVoiceState(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	if _, _, ok := voicePlaybackFor("kenta"); ok {
		t.Error("voicePlaybackFor(\"kenta\") = ok for a character with no [voconfig], want false")
	}
	if got := voiceConfigs["akane"].Number; got != 1 {
		t.Errorf("akane's Number = %d after an unrelated speaker line, want it left at 1", got)
	}
}

// TestVoicePlaybackForSkipsConfigWithoutStorage covers [voconfig name= sebuf=]
// with no vostorage= — registered, but with nothing to build a filename
// from, so there is no file it could sensibly play.
func TestVoicePlaybackForSkipsConfigWithoutStorage(t *testing.T) {
	resetVoiceState(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{SeBuf: "vo", Number: 1}

	if _, _, ok := voicePlaybackFor("akane"); ok {
		t.Error("voicePlaybackFor = ok for a [voconfig] with no vostorage=, want false")
	}
}

// --- B. tag handlers ---

func TestHandleVoConfigRegisters(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer()
	i := 0
	tag := kag3.TagObject{Name: "voconfig", Pm: map[string]string{
		"name": "akane", "sebuf": "vo", "vostorage": "akane_{number}.ogg", "number": "5",
	}}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("voconfig: %v", err)
	}
	cfg, ok := voiceConfigs["akane"]
	if !ok {
		t.Fatal("voiceConfigs[\"akane\"] missing after [voconfig]")
	}
	if cfg.SeBuf != "vo" || cfg.Storage != "akane_{number}.ogg" || cfg.Number != 5 {
		t.Errorf("cfg = %+v, want SeBuf=vo Storage=akane_{number}.ogg Number=5", *cfg)
	}
}

// TestHandleVoConfigDefaultsNumberToOne pins the 1-based default: Tyrano's
// own vostorage examples start at akane_1.ogg, so a [voconfig] with no
// number= has to begin there rather than at a zero-valued 0.
func TestHandleVoConfigDefaultsNumberToOne(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer()
	i := 0
	tag := kag3.TagObject{Name: "voconfig", Pm: map[string]string{
		"name": "akane", "vostorage": "akane_{number}.ogg",
	}}
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("voconfig: %v", err)
	}
	if got := voiceConfigs["akane"].Number; got != 1 {
		t.Errorf("Number with number= omitted = %d, want 1", got)
	}
}

// TestHandleVoConfigMergesIntoExisting is the reason [voconfig] merges
// rather than replaces: the common Tyrano idiom of re-running it with only
// number= to rewind a character's counter must not wipe out the sebuf/
// vostorage set by the original registration.
func TestHandleVoConfigMergesIntoExisting(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer()
	i := 0
	full := kag3.TagObject{Name: "voconfig", Pm: map[string]string{
		"name": "akane", "sebuf": "vo", "vostorage": "akane_{number}.ogg", "number": "7",
	}}
	if err := dispatchTag(r, fakeYield(), full, &i, 0); err != nil {
		t.Fatalf("voconfig (full): %v", err)
	}
	rewind := kag3.TagObject{Name: "voconfig", Pm: map[string]string{"name": "akane", "number": "1"}}
	if err := dispatchTag(r, fakeYield(), rewind, &i, 0); err != nil {
		t.Fatalf("voconfig (rewind): %v", err)
	}
	cfg := voiceConfigs["akane"]
	if cfg.Number != 1 {
		t.Errorf("Number = %d after a number-only [voconfig], want 1", cfg.Number)
	}
	if cfg.SeBuf != "vo" || cfg.Storage != "akane_{number}.ogg" {
		t.Errorf("cfg = %+v, want sebuf/vostorage preserved across the rewind", *cfg)
	}
}

func TestHandleVoConfigRejectsMissingNameAndBadNumber(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer()
	i := 0
	noName := kag3.TagObject{Name: "voconfig", Pm: map[string]string{"vostorage": "a_{number}.ogg"}}
	if err := dispatchTag(r, fakeYield(), noName, &i, 0); err == nil {
		t.Error("voconfig with no name= returned nil error, want an error")
	}
	badNum := kag3.TagObject{Name: "voconfig", Pm: map[string]string{"name": "akane", "number": "bogus"}}
	if err := dispatchTag(r, fakeYield(), badNum, &i, 0); err == nil {
		t.Error("voconfig with number=bogus returned nil error, want an error (a script typo, not a missing asset)")
	}
}

func TestHandleVoStartVoStopToggleAutoPlay(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer()
	i := 0
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "vostart"}, &i, 0); err != nil {
		t.Fatalf("vostart: %v", err)
	}
	if !voiceAutoPlay {
		t.Error("voiceAutoPlay = false after [vostart], want true")
	}
	if err := dispatchTag(r, fakeYield(), kag3.TagObject{Name: "vostop"}, &i, 0); err != nil {
		t.Fatalf("vostop: %v", err)
	}
	if voiceAutoPlay {
		t.Error("voiceAutoPlay = true after [vostop], want false")
	}
}

// --- C. playCharaVoice / voiceFS ---

// TestPlayCharaVoiceMissingFileIsGracefulAndStillAdvances guards both halves
// of the missing-asset policy at once: a voice file that isn't there must
// not return an error (initScript panics on those — see handlePopopo for
// the same treatment), and the counter must advance anyway, or every later
// line of that character's dialogue would play one file off for the rest of
// the scenario.
func TestPlayCharaVoiceMissingFileIsGracefulAndStillAdvances(t *testing.T) {
	resetVoiceState(t)
	r := newTestRendererWithFS(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	r.playCharaVoice("akane")

	if len(ses) != 0 {
		t.Errorf("ses = %+v, want nothing registered when the file is missing", ses)
	}
	if got := voiceConfigs["akane"].Number; got != 2 {
		t.Errorf("Number = %d after a missing file, want 2 (the counter must not stall)", got)
	}
}

// TestPlayCharaVoiceSurvivesNilFS is not hypothetical: newTestRenderer
// leaves fses nil, and fs.ReadFile on a nil fs.FS panics rather than
// erroring (see TestGoToTitleHidesLeftoverGameplayChrome's own note), so
// without voiceFS's nil check this would take down any test that runs a
// speaker line.
func TestPlayCharaVoiceSurvivesNilFS(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer() // fses is nil
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	r.playCharaVoice("akane") // must not panic
}

func TestVoiceFSPrefersVoicesKeyOverSes(t *testing.T) {
	voices := fstest.MapFS{"akane_1.ogg": &fstest.MapFile{}}
	sesFS := fstest.MapFS{}

	r := newTestRenderer()
	r.fses = map[string]fs.FS{"voices": voices, "ses": sesFS}
	if got := r.voiceFS(); got == nil {
		t.Fatal("voiceFS() = nil with a \"voices\" key present")
	} else if _, err := fs.Stat(got, "akane_1.ogg"); err != nil {
		t.Errorf("voiceFS() did not return the \"voices\" FS: %v", err)
	}

	r.fses = map[string]fs.FS{"ses": sesFS}
	if got := r.voiceFS(); got == nil {
		t.Error("voiceFS() = nil with only a \"ses\" key, want the fallback")
	}

	r.fses = map[string]fs.FS{}
	if got := r.voiceFS(); got != nil {
		t.Errorf("voiceFS() = %v with neither key, want nil so callers can skip", got)
	}
}

// --- D. execItem wiring ---

// TestExecItemPlaysVoiceOnSpeakerLine is the only test that proves the hook
// in macro.go is actually connected: everything above exercises the voice
// code directly.
func TestExecItemPlaysVoiceOnSpeakerLine(t *testing.T) {
	resetVoiceState(t)
	r := newTestRendererWithFS(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	scripts := []any{
		kag3.TextObject{Line: 1, Name: "chara_ptext", Chara: &kag3.CharacterInfo{Name: "akane"}},
	}
	i := 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem: %v", err)
	}
	if charaName != "akane" {
		t.Errorf("charaName = %q, want %q", charaName, "akane")
	}
	if got := voiceConfigs["akane"].Number; got != 2 {
		t.Errorf("Number = %d after a speaker line, want 2 (the execItem hook never fired)", got)
	}
}

// TestExecItemBareHashDoesNotPlayVoice covers a "#" line on its own, which
// clears the speaker rather than declaring one.
func TestExecItemBareHashDoesNotPlayVoice(t *testing.T) {
	resetVoiceState(t)
	r := newTestRendererWithFS(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	scripts := []any{
		kag3.TextObject{Line: 1, Name: "chara_ptext", Chara: &kag3.CharacterInfo{Name: ""}},
	}
	i := 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem: %v", err)
	}
	if got := voiceConfigs["akane"].Number; got != 1 {
		t.Errorf("Number = %d after a bare \"#\" line, want it left at 1", got)
	}
}

// TestExecItemPlainTextDoesNotPlayVoice is the regression guard for the
// text-concatenation loop deliberately left unhooked (see macro.go's
// comment): ordinary dialogue lines carry no Chara and must not advance a
// character's voice counter a second time.
func TestExecItemPlainTextDoesNotPlayVoice(t *testing.T) {
	resetVoiceState(t)
	r := newTestRendererWithFS(t)
	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 1}

	scripts := []any{
		kag3.TextObject{Line: 1, Name: "text", Val: "こんにちは"},
	}
	i := 0
	if err := r.execItem(fakeYield(), scripts, &i, 0); err != nil {
		t.Fatalf("execItem: %v", err)
	}
	if got := voiceConfigs["akane"].Number; got != 1 {
		t.Errorf("Number = %d after a plain text line, want it left at 1", got)
	}
}

// --- E. goToTitle ---

// TestGoToTitleResetsVoiceState is why voiceConfigs is cleared there but
// charas isn't: this registry carries a counter, so a second playthrough
// would otherwise start partway through a character's numbering.
func TestGoToTitleResetsVoiceState(t *testing.T) {
	resetVoiceState(t)
	r := newTestRenderer()
	r.manager.Config = &kag3.Config{ScreenWidth: 1280, ScreenHeight: 720}
	// A nil FSes map would panic rather than error inside loadScript — same
	// reason TestGoToTitleHidesLeftoverGameplayChrome wires an empty one.
	r.manager.FSes = map[string]fs.FS{"senarios": fstest.MapFS{}}
	// goToTitle sets oldTick = tick - 4, which would otherwise stick around
	// and break later tests' isTextEnded checks.
	defer func() { tick, oldTick = 0, 0 }()

	voiceAutoPlay = true
	voiceConfigs["akane"] = &voiceConfig{Storage: "akane_{number}.ogg", Number: 57}

	r.goToTitle()

	if voiceAutoPlay {
		t.Error("voiceAutoPlay = true after goToTitle, want false")
	}
	if len(voiceConfigs) != 0 {
		t.Errorf("voiceConfigs = %+v after goToTitle, want empty", voiceConfigs)
	}
}
