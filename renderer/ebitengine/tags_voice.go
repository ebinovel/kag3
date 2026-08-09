package ebitengine

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/ebinovel/kag3"
)

func init() {
	register("voconfig", handleVoConfig)
	register("vostart", handleVoStart)
	register("vostop", handleVoStop)
}

// voiceConfig is one [voconfig] registration: which SE buffer a speaker's
// auto-voice plays through, the vostorage= filename template, and the
// counter substituted into that template's {number} placeholder.
type voiceConfig struct {
	SeBuf   string // sebuf=
	Storage string // vostorage=, e.g. "akane_{number}.ogg"
	Number  int    // the number to use for the *next* line, then incremented
}

// voiceConfigs is the process-lifetime [voconfig] registry, keyed by the
// speaker name a "#name" line declares (charaName, tags_character.go).
// Deliberately not part of SaveData (tags_save.go), matching how no audio
// state is: currentBGM, ses and defaultSeVolume (tags_audio.go) have never
// been saved either, so persisting only this one piece would be the odd one
// out. Same "transient, in-memory" treatment as charas
// (tags_character.go) and readLines (tags_message.go) besides — a scenario
// re-declares its own [voconfig]/[vostart] near the top of each scene, so
// replaying from a load restores it naturally.
//
// goToTitle (title_flow.go) does reset it, unlike charas: this registry
// carries a *counter*, so leaving it alone would have a second playthrough
// start its voices partway through the numbering instead of at the top.
var voiceConfigs = map[string]*voiceConfig{}

// voiceAutoPlay is [vostart]/[vostop]. Off by default: real Tyrano's
// [voconfig] registrations do nothing at all until [vostart] arms them.
var voiceAutoPlay bool

// defaultVoiceSeBuf is the ses[] key used when [voconfig] omits sebuf=.
// One shared buffer across every character on purpose: dialogue voices
// should cut each other off as the speaker changes, which is exactly what
// swapSEPlayer already does to whatever is in a buffer.
const defaultVoiceSeBuf = "voice"

// voiceNumberPlaceholder is [voconfig vostorage=]'s counter slot.
const voiceNumberPlaceholder = "{number}"

// handleVoConfig implements [voconfig name= sebuf= vostorage= number=],
// registering (or updating) one speaker's auto-voice settings.
//
// Merges into any existing registration rather than replacing it, so the
// common Tyrano idiom of re-running [voconfig name="akane" number="1"] to
// rewind just the counter keeps that character's sebuf/vostorage intact.
func handleVoConfig(ctx *tagCtx) error {
	name, ok := getString(ctx.tag.Pm, "name")
	if !ok || name == "" {
		return fmt.Errorf("[voconfig] requires name=")
	}
	cfg, found := voiceConfigs[name]
	if !found {
		// Number defaults to 1, not 0: Tyrano's own vostorage examples are
		// 1-based ("akane_1.ogg"), and [voconfig] without number= is meant
		// to start from the beginning.
		cfg = &voiceConfig{Number: 1}
		voiceConfigs[name] = cfg
	}
	if v, ok := getString(ctx.tag.Pm, "sebuf"); ok {
		cfg.SeBuf = v
	}
	if v, ok := getString(ctx.tag.Pm, "vostorage"); ok {
		cfg.Storage = v
	}
	// A malformed number= is a script typo rather than a missing asset, so
	// this one does return an error — same split handleSEOpt/handleChangeVol
	// already draw between "attribute won't parse" (error) and "file isn't
	// there" (log and carry on, see playCharaVoice below).
	if n, ok, err := getInt(ctx.tag.Pm, "number"); err != nil {
		return err
	} else if ok {
		cfg.Number = n
	}
	return nil
}

func handleVoStart(ctx *tagCtx) error {
	voiceAutoPlay = true
	return nil
}

func handleVoStop(ctx *tagCtx) error {
	voiceAutoPlay = false
	return nil
}

// voiceStorageName substitutes number into a vostorage= template. A
// template with no {number} in it is returned unchanged rather than having
// the counter appended or the extension rewritten: Tyrano accepts a fixed
// filename there (a character with a single stock line), and that has to
// keep resolving to the same file on every speaker line.
//
// Zero-padded numbering ("akane_001.ogg") is outside the {number} spec and
// not supported; if it's ever wanted, this function is the only place that
// would need to change.
func voiceStorageName(template string, number int) string {
	if !strings.Contains(template, voiceNumberPlaceholder) {
		return template
	}
	return strings.ReplaceAll(template, voiceNumberPlaceholder, strconv.Itoa(number))
}

// voicePlaybackFor resolves which SE buffer and which file the speaker
// "name" should play next, advancing that speaker's counter. ok is false
// when nothing should play at all: auto-play not armed ([vostart] never
// called), no speaker (a bare "#" line clears charaName), or a speaker with
// no [voconfig] of their own. Touches neither the filesystem nor the audio
// system, so the whole decision is unit-testable on its own.
//
// The counter advances even when the resulting file later turns out to be
// missing (playCharaVoice logs and skips it): only advancing on a
// successful load would shift every subsequent line of that character's
// dialogue onto the wrong file for the rest of the scenario, which is a far
// worse failure than one silent line. Revisiting the same "#name" line
// (a loop, or a backward [jump]) likewise advances it — real Tyrano
// increments unconditionally too.
func voicePlaybackFor(name string) (buf, storage string, ok bool) {
	if !voiceAutoPlay || name == "" {
		return "", "", false
	}
	cfg, found := voiceConfigs[name]
	if !found || cfg.Storage == "" {
		return "", "", false
	}
	storage = voiceStorageName(cfg.Storage, cfg.Number)
	cfg.Number++
	buf = cfg.SeBuf
	if buf == "" {
		buf = defaultVoiceSeBuf
	}
	return buf, storage, true
}

// voiceFS returns the fs.FS voice files are read from: a dedicated
// "voices" key when the host project provides one, falling back to "ses"
// otherwise. May return nil (a Renderer built without an fses map at all,
// as newTestRenderer does) — callers must nil-check before handing it to
// fs.ReadFile, which panics rather than errors on a nil fs.FS.
//
// The fallback isn't a hedge, it's the faithful default: real Tyrano keeps
// voices in the same data/sound folder as sound effects, and [voconfig]'s
// own sebuf= says outright that a voice *is* an SE playing through an SE
// buffer. Requiring a new "voices" key instead would have meant editing
// example/game/resources.go's DefaultFSes — which lives under the
// git-excluded example/ (see CLAUDE.md's "Repository layout gotcha"), so
// the change would never show up in a commit — while silently breaking
// every other host project that builds its own fses map and doesn't know
// about the new key.
func (r *Renderer) voiceFS() fs.FS {
	if fsys, ok := r.fses["voices"]; ok && fsys != nil {
		return fsys
	}
	return r.fses["ses"]
}

// playCharaVoice plays the next auto-voice file for the speaker a "#name"
// line just declared, if [voconfig] registered one for them and [vostart]
// has armed auto-play. Called from execItem (macro.go) at the single point
// charaName is assigned.
//
// Deliberately returns no error: initScript's loop panics on anything a tag
// handler returns (see its own comment), and a missing voice asset is
// nowhere near worth taking the whole game down for — handlePopopo and
// handleCharaMod (tags_audio.go/tags_character.go) set the same precedent.
//
// Playback goes through swapSEPlayer into the shared ses map rather than a
// registry of its own, which is what makes [stopse], [wse buf=...],
// [fadeoutse], [pausese]/[resumese], [seopt] and [changevol buf=...] all
// work on voices with no extra code — and matches Tyrano, where sebuf=
// says a voice is an SE. One consequence worth knowing: swapSEPlayer only
// replaces the buffer on a *successful* load, so a missing file leaves the
// previous line's voice in place instead of silencing it. Same behavior
// [playse] already has.
func (r *Renderer) playCharaVoice(name string) {
	buf, storage, ok := voicePlaybackFor(name)
	if !ok {
		return
	}
	fsys := r.voiceFS()
	if fsys == nil {
		// No "voices" and no "ses" — a host project that never wired either
		// up. Silent rather than logged: this would otherwise print on
		// every single speaker line, and stdout is genuinely slow on iOS
		// (see traceTags in macro.go for the same lesson).
		return
	}
	se := &kag3.BGM{Storage: storage, Volume: defaultSeVolume}
	if err := swapSEPlayer(fsys, buf, storage, se); err != nil {
		fmt.Printf("[voice] %s を読み込めないためスキップします: %v\n", storage, err)
		return
	}
	delete(activeFades, "se:"+buf)
	se.Player.SetVolume(volFraction(se.Volume))
	se.Player.Play()
}
