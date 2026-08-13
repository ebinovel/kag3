package ebitengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("savesnap", handleSaveSnap)
	register("autosave", handleAutoSave)
	register("autoload", handleAutoLoad)
	register("checkpoint", handleCheckpoint)
	register("rollback", handleRollback)
	register("clear_checkpoint", handleClearCheckpoint)
	register("screen_full", handleScreenFull)
	register("dialog", handleDialog)
	register("dialog_config", handleDialogConfig)
	register("dialog_config_ok", handleDialogConfigOK)
	register("dialog_config_ng", handleDialogConfigNG)
	register("dialog_config_filter", handleDialogConfigFilter)
	register("start_keyconfig", handleStartKeyConfig)
	register("stop_keyconfig", handleStopKeyConfig)
	register("closeconfirm_on", handleCloseConfirmOn)
	register("closeconfirm_off", handleCloseConfirmOff)
}

// Reserved slot numbers for the button roles that don't carry an explicit
// slot attribute: real Tyrano's save/load roles open a slot-picker screen
// (that's Phase 9's showsave/showload), so until that exists role="save"/
// "load" target one fixed slot and role="quicksave"/"quickload" another.
const (
	manualSaveSlot = 1
	quickSaveSlot  = 0
	autoSaveSlot   = -1
)

// bgSaveState is the subset of *kag3.Background that's actually
// serializable (Image/NextImage are GPU textures, not JSON data) and worth
// keeping — see saveData's doc comment for what's deliberately left out.
type bgSaveState struct {
	Time     int
	IsWait   bool
	IsCross  bool
	Position string
	Method   string
	IsSystem bool
	// Storage is bg.Storage (tags_background.go) — the file path
	// applySaveData reloads Image from in a fresh process.
	Storage string
}

// textPositionSaveState is the subset of *kag3.TextPosition that's actually
// serializable (BackImage/FrameImage are GPU textures, not JSON data).
// applySaveData resumes execution via jumpIndex straight into the middle of
// a script (see its doc comment), which skips whatever one-time [position]/
// [layopt] setup ran earlier in the file to configure the message window —
// without this, a load either leaves textPosition at its zero value (fresh
// process: Visible=false, Width/Height=0, no BackImage) or at whatever
// goToTitle/[close] last left it at (same process), so the message box and
// its text silently never reappear.
type textPositionSaveState struct {
	Layer        string
	Page         string
	Left         int
	Top          int
	Width        int
	Height       int
	Color        color.RGBA
	BorderColor  color.RGBA
	BorderSize   int
	Opacity      int
	MarginLeft   int
	MarginTop    int
	MarginRight  int
	MarginBottom int
	MarginN      int
	Radius       int
	Vertical     bool
	Visible      bool
	Gradient     color.RGBA
	FilterColor  *color.RGBA
	// FrameStorage is textPosition.FrameStorage — the file path
	// applySaveData reloads FrameImage from, same idea as Bg.Storage.
	FrameStorage string
}

// sleepStackToCallFrames/callFramesToSleepStack convert between
// Renderer.sleepStack's live []sleepFrame (which carries a Buttons
// snapshot) and saveData's serializable []callFrame (Storage/Index only —
// see SleepStack's doc comment for why Buttons is dropped).
func sleepStackToCallFrames(stack []sleepFrame) []callFrame {
	out := make([]callFrame, len(stack))
	for i, f := range stack {
		out[i] = callFrame{Storage: f.Storage, Index: f.Index}
	}
	return out
}

func callFramesToSleepStack(frames []callFrame) []sleepFrame {
	out := make([]sleepFrame, len(frames))
	for i, f := range frames {
		out[i] = sleepFrame{Storage: f.Storage, Index: f.Index}
	}
	return out
}

// saveData is the plan's minimal in-memory-first save format: enough to
// resume script execution and restore variables faithfully, not a pixel-
// perfect visual snapshot. bg2 (the secondary background layer) and
// characters' differential parts ([chara_layer]) are still out of scope —
// bg/viewCharas' base appearance is reconstructed via CharaStorage/
// Bg.Storage below (see reconcileViewCharas and applySaveData), everything
// else restores only within the same process (where `charas` is already
// populated).
type saveData struct {
	Storage   string
	Index     int
	CallStack []callFrame
	// SleepStack is Renderer.sleepStack — role="sleepgame"/[sleepgame]'s own
	// return-address stack, kept separate from CallStack (see the
	// Renderer.sleepStack doc comment in renderer.go). Storage/Index only:
	// sleepFrame.Buttons/Bg hold *ebiten.Image fields that can't round-trip
	// through JSON, same class of gap as bg2/imgs (see the save/load gaps
	// this repo already accepts) — a save made mid-sleepgame won't restore
	// the caller's buttons/background after a fresh-process load, but
	// that's an unreachable edge case today (config.ks, the only sleepgame
	// user, disables the normal save UI while open).
	SleepStack []callFrame
	SFVars     map[string]interface{}
	FVars      map[string]interface{}
	ViewCharas []*kag3.CharaShow
	// CharaStorage records, for every name appearing in ViewCharas, the
	// image path charas[name].Storage held at save time — what
	// reconcileViewCharas uses to re-register a character that a fresh
	// process never ran [chara_new] for.
	CharaStorage map[string]string
	// CharaFaces records, for every name appearing in ViewCharas, the full
	// charas[name].Faces map (face name -> image path) held at save time.
	// A save resumed mid-script (jumpIndex straight to the saved position)
	// skips whatever [chara_face] tags ran earlier in the file, so without
	// this a [chara_mod ... face="happy"] reached after loading would find
	// nothing but the "default" face reconcileViewCharas used to seed on its
	// own — see the save/load gaps note above.
	CharaFaces   map[string]map[string]string
	Bg           bgSaveState
	TextPosition textPositionSaveState
	// TextStyle/DefaultTextStyle are the package-level textStyle/
	// defaultTextStyle vars ([font]/[deffont]/[resetfont], tags_text.go) at
	// save time. kag3.TextStyle has no *ebiten.Image fields, so unlike
	// TextPosition it round-trips through JSON as-is — no separate
	// "SaveState" struct needed. Without this, loading a save taken while
	// [font color=...] was in effect showed the *current* session's color
	// instead (e.g. black after a later [deffont] call), because nothing
	// reset/restored textStyle on load — applySaveData resumes via
	// jumpIndex straight into the saved position, skipping whatever [font]
	// tag was in scope there. nil (nothing set, or an old save predating
	// this field) is itself a valid, safe value — the renderer's built-in
	// default look.
	TextStyle        *kag3.TextStyle
	DefaultTextStyle *kag3.TextStyle
	// MenuButtonVisible is menuButtonVisible (tags_sysdesign.go's
	// @showmenubutton/@hidemenubutton corner icon) at save time. Without
	// this, loading a save taken while the button was still visible left it
	// hidden if the *current* session had since called [hidemenubutton] —
	// applySaveData resumes via jumpIndex straight into the saved position,
	// never re-running whatever @showmenubutton call put it there.
	MenuButtonVisible bool
	// Ptexts is the package-level ptexts map ([ptext]/[chara_config]/[mtext]/
	// [graph]'s underlying storage, tags_text.go) at save time. kag3.PText
	// has no *ebiten.Image fields, so it round-trips through JSON as-is —
	// no separate "SaveState" type needed, same reasoning as TextStyle.
	// Without this, loading a save resumed via jumpIndex (skipping whatever
	// [ptext]/[chara_config] calls ran earlier in the file) showed the
	// *current* session's ptext layout instead — most visibly, a character
	// name-plate repositioned partway through a playthrough stayed at its
	// *new* position even when loading a save from before that change.
	Ptexts map[string]*kag3.PText
	// CharaNamePText is charaNamePText (tags_character.go's [chara_config
	// ptext=...], tracking which ptexts entry doubles as the name-plate) at
	// save time — same gap as Ptexts above; without it a load could resume
	// pointing at a name-plate ptext area that either didn't exist yet or
	// has since moved.
	CharaNamePText string
	// CharaName is charaName (tags_character.go's current speaker, set by
	// the most recent "#name" line — see its doc comment for why it
	// persists across [p]/[cm]) at save time. Without it, the name-plate
	// ptext (driven by charaName, see ptextContent in renderer.go) came
	// back blank after a load even once Texts below restored the dialogue
	// itself, since charaName is tracked separately from r.texts and
	// nothing else on the load path touches it.
	CharaName string
	// LastMessage is whatever text was in the message window at save time
	// (see currentMessageText in tags_message.go) — the DATA SAVE/LOAD
	// screen's preview line, alongside the thumbnail (see captureSnapshot).
	LastMessage string
	// Texts is r.texts (the current page's revealed message-window content,
	// renderer.go) at save time. applySaveData resumes execution via
	// jumpIndex straight at the saved position — almost always a [p]/[s]/
	// [l] tag itself, not the TextObject(s) before it that actually put
	// text on screen — so without this, loading (or reloading a
	// [checkpoint]) always resumed with a *blank* message window: the tag
	// that was blocking re-runs and blocks again, but the dialogue line(s)
	// it was blocking *for* are never replayed. A real reported bug: the
	// player's own save landed on an [s] right after a [link] choice, so
	// the message text vanished *and* the choice itself did (see Links/
	// GLinks below) — with nothing left on screen to click, loading looked
	// like it silently did nothing at all.
	Texts map[int][]Text
	// Links/GLinks are the links/glinks package-level slices (tags_link.go)
	// at save time — the actual [link]/[glink] choices visible in the
	// message window, as opposed to Texts above (the plain dialogue text
	// around them). Same gap as Texts: resuming via jumpIndex at a [s]
	// sitting right after a choice's [link]/[endlink] pair never re-runs
	// handleLink, so the choice itself would otherwise never reappear.
	Links  []*kag3.Link
	GLinks []*kag3.GLink
}

func (r *Renderer) buildSaveData() *saveData {
	charaStorage := make(map[string]string, len(viewCharas))
	charaFaces := make(map[string]map[string]string, len(viewCharas))
	for _, c := range viewCharas {
		if _, ok := charaStorage[c.Name]; ok {
			continue
		}
		if ch, ok := charas[c.Name]; ok {
			charaStorage[c.Name] = ch.Storage
			faces := make(map[string]string, len(ch.Faces))
			for face, storage := range ch.Faces {
				faces[face] = storage
			}
			charaFaces[c.Name] = faces
		}
	}
	return &saveData{
		Storage:           r.currentStorage,
		Index:             currentScriptIndex,
		CallStack:         append([]callFrame(nil), r.callStack...),
		SleepStack:        sleepStackToCallFrames(r.sleepStack),
		SFVars:            r.vm.ExportSF(),
		FVars:             r.vm.ExportF(),
		ViewCharas:        copyCharaShows(viewCharas),
		CharaStorage:      charaStorage,
		CharaFaces:        charaFaces,
		Bg:                snapshotBg(bg),
		TextPosition:      snapshotTextPosition(textPosition),
		TextStyle:         copyTextStyle(textStyle),
		DefaultTextStyle:  copyTextStyle(defaultTextStyle),
		MenuButtonVisible: menuButtonVisible,
		Ptexts:            snapshotPtexts(ptexts),
		CharaNamePText:    charaNamePText,
		CharaName:         charaName,
		LastMessage:       currentMessageText(r),
		Texts:             copyTexts(r.texts),
		Links:             append([]*kag3.Link(nil), links...),
		GLinks:            append([]*kag3.GLink(nil), glinks...),
	}
}

// copyCharaShows deep-copies viewCharas: stepAnimations (tags_animation.go)
// mutates a *kag3.CharaShow in place via SetLeft/SetTop/SetOpacity/
// SetScaleX/SetScaleY/SetRotation while [anim]/[kanim] is running, so a
// shallow append (sharing the original *CharaShow pointers) let an [anim]
// running after [checkpoint] silently corrupt the snapshot [rollback] later
// restores from. CharaShow's fields are all scalars (see its own
// declaration in kag3.go), so a top-level struct copy is a full deep copy,
// same reasoning as copyTextStyle.
func copyCharaShows(src []*kag3.CharaShow) []*kag3.CharaShow {
	out := make([]*kag3.CharaShow, len(src))
	for i, c := range src {
		cp := *c
		out[i] = &cp
	}
	return out
}

// copyTexts deep-copies texts, including each segment's TextStyle pointer
// — same reasoning as copyTextStyle: [checkpoint] keeps its *saveData in
// memory and can be applied via [rollback] more than once, so sharing a
// live *kag3.TextStyle a later [font] call might mutate in place (see
// handleFont, tags_text.go) would silently corrupt an already-taken
// checkpoint's text.
func copyTexts(src map[int][]Text) map[int][]Text {
	out := make(map[int][]Text, len(src))
	for line, segs := range src {
		cp := make([]Text, len(segs))
		for i, seg := range segs {
			cp[i] = seg
			cp[i].TextStyle = copyTextStyle(seg.TextStyle)
		}
		out[line] = cp
	}
	return out
}

// snapshotPtexts shallow-copies the ptexts map: [ptext]/[chara_config]/
// [mtext]/[graph] always register a brand new *kag3.PText on a given name
// rather than mutating an existing one in place (see handlePText,
// tags_text.go), so sharing the *kag3.PText pointers themselves across a
// [checkpoint]-then-later-[ptext] sequence is safe — only the map itself
// needs its own identity, same idea as buildSaveData's CharaFaces copy.
func snapshotPtexts(src map[string]*kag3.PText) map[string]*kag3.PText {
	out := make(map[string]*kag3.PText, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// copyTextStyle shallow-copies a *kag3.TextStyle (or returns nil for nil).
// [checkpoint] keeps its *saveData in memory until [rollback] uses it (see
// checkpointData below) rather than serializing it right away, so storing
// the live textStyle/defaultTextStyle pointers directly would let a later
// [font]/[deffont] call — which mutates the shared struct's fields in place
// — silently corrupt an already-taken checkpoint. Copying just the top-level
// struct is enough: nothing ever mutates an existing *color.RGBA in place
// (Color/Edge/Shadow are always reassigned to a brand new pointer), so
// sharing those is safe.
func copyTextStyle(ts *kag3.TextStyle) *kag3.TextStyle {
	if ts == nil {
		return nil
	}
	cp := *ts
	return &cp
}

// snapshotBg captures the serializable subset of *kag3.Background into a
// bgSaveState — the read half of buildSaveData's Bg field, split out so
// [checkpoint]-style snapshots don't need to duplicate the field list.
func snapshotBg(bg *kag3.Background) bgSaveState {
	return bgSaveState{
		Time:     bg.Time,
		IsWait:   bg.IsWait,
		IsCross:  bg.IsCross,
		Position: bg.Position,
		Method:   bg.Method,
		IsSystem: bg.IsSystem,
		Storage:  bg.Storage,
	}
}

// snapshotTextPosition captures the serializable subset of
// *kag3.TextPosition into a textPositionSaveState — the read half of
// buildSaveData's TextPosition field.
func snapshotTextPosition(tp *kag3.TextPosition) textPositionSaveState {
	return textPositionSaveState{
		Layer:        tp.Layer,
		Page:         tp.Page,
		Left:         tp.Left,
		Top:          tp.Top,
		Width:        tp.Width,
		Height:       tp.Height,
		Color:        tp.Color,
		BorderColor:  tp.BorderColor,
		BorderSize:   tp.BorderSize,
		Opacity:      tp.Opacity,
		MarginLeft:   tp.MarginLeft,
		MarginTop:    tp.MarginTop,
		MarginRight:  tp.MarginRight,
		MarginBottom: tp.MarginBottom,
		MarginN:      tp.MarginN,
		Radius:       tp.Radius,
		Vertical:     tp.Vertical,
		Visible:      tp.Visible,
		Gradient:     tp.Gradient,
		FilterColor:  tp.FilterColor,
		FrameStorage: tp.FrameStorage,
	}
}

// applyBgFromSnapshot writes s's non-image fields onto bg, then reloads
// Image from s.Storage (the GPU texture a bgSaveState can't carry through
// JSON — see bgSaveState's doc comment). A no-op on the image reload when
// s.Storage is empty (nothing was ever captured, e.g. a save predating this
// field). The caller decides how to react to a reload failure (applySaveData
// logs and continues rather than treating it as fatal, matching every other
// best-effort asset reload on the load path).
func applyBgFromSnapshot(r *Renderer, bg *kag3.Background, s bgSaveState) error {
	bg.Time = s.Time
	bg.IsWait = s.IsWait
	bg.IsCross = s.IsCross
	bg.Position = s.Position
	bg.Method = s.Method
	bg.IsSystem = s.IsSystem
	if s.Storage == "" {
		return nil
	}
	images := "images"
	if s.IsSystem {
		images = "system/images"
	}
	img, _, err := ebitenutil.NewImageFromFileSystem(r.fses[images], s.Storage)
	if err != nil {
		return err
	}
	bg.Image = img
	bg.NextImage = nil
	bg.IsEnd = true
	bg.Storage = s.Storage
	return nil
}

// applyTextPositionFromSnapshot writes s's non-image fields onto tp, then
// rebuilds BackImage and reloads FrameImage from s.FrameStorage (the GPU
// textures a textPositionSaveState can't carry through JSON — see its doc
// comment). A no-op when s.Width and s.Height are both zero (nothing was
// ever captured). Same caller-decides error contract as applyBgFromSnapshot.
func applyTextPositionFromSnapshot(r *Renderer, tp *kag3.TextPosition, s textPositionSaveState) error {
	if s.Width == 0 && s.Height == 0 {
		return nil
	}
	tp.Layer = s.Layer
	tp.Page = s.Page
	tp.Left = s.Left
	tp.Top = s.Top
	tp.Width = s.Width
	tp.Height = s.Height
	tp.Color = s.Color
	tp.BorderColor = s.BorderColor
	tp.BorderSize = s.BorderSize
	tp.Opacity = s.Opacity
	tp.MarginLeft = s.MarginLeft
	tp.MarginTop = s.MarginTop
	tp.MarginRight = s.MarginRight
	tp.MarginBottom = s.MarginBottom
	tp.MarginN = s.MarginN
	tp.Radius = s.Radius
	tp.Vertical = s.Vertical
	tp.Visible = s.Visible
	tp.Gradient = s.Gradient
	tp.FilterColor = s.FilterColor
	tp.BackImage = ebiten.NewImage(tp.Width, tp.Height)
	tp.FrameImage = nil
	tp.FrameStorage = ""
	if s.FrameStorage == "" {
		return nil
	}
	img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["images"], s.FrameStorage)
	if err != nil {
		return err
	}
	tp.FrameImage = img
	tp.FrameStorage = s.FrameStorage
	return nil
}

// reconcileViewCharas ensures every entry in restored has a matching charas
// registration before it's allowed back into the live viewCharas — the
// invariant the rest of the renderer (drawScene, charaShow) already assumes
// [chara_show] enforces. Anything already registered (a same-process load)
// has its Image/Storage reloaded to match c.Storage (see the loop body's
// comment for why); anything missing is re-registered from charaStorage if
// possible, or dropped (logged, not panicked) if its path is missing or the
// file can't be read.
//
// charaFaces restores the character's full face registry (see saveData's
// CharaFaces doc comment) — without it, [chara_mod ... face=...] for any
// face beyond "default" would find nothing and, before that was guarded
// (see handleCharaMod), crashed the whole game. A save written before this
// field existed (or a name reconcileViewCharas had to fall back to
// charaStorage for some other reason) still gets a working "default" entry.
func reconcileViewCharas(r *Renderer, restored []*kag3.CharaShow, charaStorage map[string]string, charaFaces map[string]map[string]string) []*kag3.CharaShow {
	kept := restored[:0]
	for _, c := range restored {
		if reg, ok := charas[c.Name]; ok {
			// drawCharacters (draw_chara.go) draws charas[name].Image, not
			// anything on c (CharaShow) itself — a separate "currently
			// showing" pointer [chara_mod] etc. update directly
			// (handleCharaMod), which never touches the matching
			// viewCharas/CharaShow entry's own Face/Storage fields (they
			// stay whatever [chara_show] originally set, typically empty —
			// see charaStorage's doc comment for where the *real* current
			// value lives instead). Left alone, a same-process load would
			// keep whatever face the *current* session had showing at load
			// time instead of the one active at save time — reload it here
			// from charaStorage[c.Name] (buildSaveData's charas[name].Storage
			// snapshot) to match, same as handleCharaMod's own face switch.
			if want := charaStorage[c.Name]; want != "" && want != reg.Storage {
				// loadImage, not ebitenutil.NewImageFromFileSystem directly:
				// want may be a [chara_new_psd]-generated storage descriptor
				// (tags_chara_psd.go), which isn't a real file path at all —
				// loadImage is the one place that knows how to regenerate
				// one of those from scratch, exactly what a fresh process
				// (that never ran [chara_new_psd] itself) needs here.
				if img, err := loadImage(r, "", want); err != nil {
					fmt.Printf("save/load: %s の画像 %s の読み込みに失敗したため表情を復元できません: %v\n", c.Name, want, err)
				} else {
					reg.Image = img
					reg.Storage = want
				}
			}
			kept = append(kept, c)
			continue
		}
		storage := charaStorage[c.Name]
		if storage == "" {
			fmt.Printf("save/load: %s の画像パスが無いため復元できません(スキップします)\n", c.Name)
			continue
		}
		img, err := loadImage(r, "", storage)
		if err != nil {
			fmt.Printf("save/load: %s の画像読み込みに失敗したためスキップします: %v\n", c.Name, err)
			continue
		}
		faces := charaFaces[c.Name]
		if len(faces) == 0 {
			faces = map[string]string{"default": storage}
		}
		charas[c.Name] = &kag3.Character{
			Name:    c.Name,
			Image:   img,
			Storage: storage,
			Faces:   faces,
		}
		kept = append(kept, c)
	}
	return kept
}

// applySaveData restores everything saveData captured, then defers to the
// existing jump mechanism (jumpIndex/isJump) to actually move execution
// there — same as [jump]/button clicks, so it takes effect at the next
// script-loop iteration boundary rather than tearing r.scripts out from
// under a suspended coroutine.
func (r *Renderer) applySaveData(d *saveData) error {
	// Save data doesn't capture button state (see the save/load gaps note in
	// tags_save.go's own doc comments), so whatever's in `buttons` right now
	// belongs to wherever the player was when they opened the load screen,
	// not the loaded position — always treat a load as a screen change so
	// Update()'s isJump handling clears stale non-fix buttons.
	screenChanged = true
	if d.Storage != "" && d.Storage != r.currentStorage {
		if err := r.loadScript(d.Storage); err != nil {
			return err
		}
	}
	r.callStack = append([]callFrame(nil), d.CallStack...)
	r.sleepStack = callFramesToSleepStack(d.SleepStack)
	r.vm.RestoreF(d.FVars)
	r.vm.RestoreSF(d.SFVars)
	// Copied again (not aliased directly), same reasoning as textStyle a few
	// lines below: [rollback] can apply the same *saveData (checkpointData)
	// more than once, and reconcileViewCharas hands its result straight to
	// the live viewCharas — which stepAnimations (tags_animation.go) mutates
	// in place while [anim]/[kanim] runs. Without this copy, a second
	// rollback to the same checkpoint would see whatever the first
	// rollback's aftermath did to it, not the state actually captured.
	viewCharas = reconcileViewCharas(r, copyCharaShows(d.ViewCharas), d.CharaStorage, d.CharaFaces)
	if err := applyBgFromSnapshot(r, bg, d.Bg); err != nil {
		fmt.Printf("save/load: 背景 %s の読み込みに失敗しました: %v\n", d.Bg.Storage, err)
	}
	// Width/Height!=0 as the "was this actually captured" signal — same idea
	// as Bg.Storage!="" inside applyBgFromSnapshot — so loading a save
	// written before this field existed doesn't stomp a same-process
	// textPosition that's already correctly configured with zeroed-out
	// layout (see applyTextPositionFromSnapshot's own no-op guard).
	if err := applyTextPositionFromSnapshot(r, textPosition, d.TextPosition); err != nil {
		fmt.Printf("save/load: メッセージ枠 %s の読み込みに失敗しました: %v\n", d.TextPosition.FrameStorage, err)
	}
	// Copied again (not assigned directly), same reasoning as buildSaveData's
	// copyTextStyle: [rollback] can apply the same *saveData (checkpointData)
	// more than once, so textStyle must get its own struct instance rather
	// than aliasing d's — otherwise a [font] call after this rollback would
	// mutate the checkpoint itself, corrupting any later rollback to it.
	textStyle = copyTextStyle(d.TextStyle)
	defaultTextStyle = copyTextStyle(d.DefaultTextStyle)
	menuButtonVisible = d.MenuButtonVisible
	// nil check (not just "always assign"): a save written before Ptexts
	// existed decodes it as nil, and assigning that would wipe out whatever
	// ptext layout the *current* session already has — leave it alone in
	// that case rather than making an old save regress further than "same
	// behavior as before this fix".
	if d.Ptexts != nil {
		ptexts = d.Ptexts
		charaNamePText = d.CharaNamePText
		// BgImage is excluded from JSON (kag3.PText's json:"-" tag) — reload
		// it from BgStorage for every restored ptext area that has one, same
		// pattern as TextPosition.FrameStorage/FrameImage just above. Needed
		// even for a same-process load: d.Ptexts came from json.Unmarshal
		// (buildSaveData/saveSlot round-trip through the file on disk), so
		// BgImage is always nil/zero-value here regardless of process
		// lifetime, never the live *ebiten.Image the current session loaded.
		for _, pt := range ptexts {
			if pt.BgStorage == "" {
				continue
			}
			img, err := loadImage(r, "", pt.BgStorage)
			if err != nil {
				fmt.Printf("save/load: %s の背景画像の読み込みに失敗しました: %v\n", pt.BgStorage, err)
				continue
			}
			pt.BgImage = img
		}
	}
	// menuButtonImg is loaded lazily (see handleShowMenuButton,
	// tags_sysdesign.go) and never reset by a load — a fresh process that
	// jumps straight to a saved position via jumpIndex never runs the
	// @showmenubutton call that would normally load it, so drawMenuButton's
	// own "menuButtonImg == nil" guard would otherwise keep the button
	// hidden even with MenuButtonVisible restored to true.
	if menuButtonVisible && menuButtonImg == nil {
		if img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["system/images"], "button_menu.png"); err != nil {
			fmt.Printf("save/load: メニューボタンの読み込みに失敗しました: %v\n", err)
		} else {
			menuButtonImg = img
		}
	}
	// nil check, same reasoning as Ptexts above: a save written before
	// Texts existed decodes it as nil, and this is the *only* thing that
	// puts the loaded position's actual dialogue back on screen — without
	// it, jumpIndex resumes execution at the [p]/[s]/[l] tag itself, which
	// blocks again but never replays whatever TextObject(s) before it in
	// the script originally revealed the text it was blocking for, so the
	// message window came back completely blank after every load.
	if d.Texts != nil {
		r.texts = copyTexts(d.Texts)
	} else {
		r.texts = make(map[int][]Text)
	}
	// Same gap, for the [link]/[glink] choices actually visible in the
	// message window rather than the plain text around them — a save
	// landing on an [s] right after a [link]/[endlink] pair (a real
	// reported case) never re-runs handleLink on load, so without this the
	// choice itself silently never reappeared either, leaving nothing on
	// screen the player could click at all.
	if len(d.Links) > 0 || len(d.GLinks) > 0 {
		links = append([]*kag3.Link(nil), d.Links...)
		glinks = append([]*kag3.GLink(nil), d.GLinks...)
		preserveLinksOnJump = true
	} else {
		links = nil
		glinks = nil
	}
	charaName = d.CharaName
	pendingRuby = ""
	// true, not false — see the matching comment in goToTitle
	// (renderer.go): a save/load/rollback triggered from the slot picker
	// or [rollback] can just as easily land while the tag coroutine is
	// blocked on isWait (mid-dialogue) rather than [s]'s isJumped, and
	// isWait=false would leave it stuck there forever, never reaching the
	// pending jump at all.
	isWait = true
	// oldTick reset alongside isWait — see the matching comment in
	// goToTitle for why this is necessary too, not just isWait: without
	// it, [p]'s own y.Until(true, isTextEnded) (isTextEnded =
	// oldTick+3>=tick && isWait) can spuriously already be satisfied the
	// instant the jump lands on a [p] tag, since oldTick is whatever it
	// last was set to (the most recent real click) and isWait is now
	// forced true — if those happen to be within 3 ticks of the current
	// tick (confirmed empirically: a same-process load taken shortly
	// after a real click routinely lands in exactly this window), the
	// [p]tag at the loaded position resolves immediately instead of
	// waiting for the player to actually click, silently skipping to the
	// next line.
	oldTick = tick - 4
	isSkip = false
	isAuto = false
	jumpIndex = d.Index
	isJump = true
	return nil
}

// saveBaseDirOverride lets tests point saveDir at a temp directory instead
// of the real OS config dir.
var saveBaseDirOverride string

// SaveDirFunc, if set, is consulted before falling back to os.UserConfigDir
// (but after KAG3_SAVE_DIR/saveBaseDirOverride) — an embedding app can
// register this to supply its own platform-appropriate writable directory
// obtained some other way than the standard os package APIs (e.g.
// Android's Context.getFilesDir(), reached over JNI — see example/mobile's
// own android-only storage bridge for a concrete implementation;
// renderer/ebitengine is a shared package used by non-Android projects too,
// so it has no business knowing about JNI itself).
var SaveDirFunc func() (string, error)

// saveDir resolves the directory save/load slots live in. KAG3_SAVE_DIR, if
// set, wins outright and is used as-is (no kag3/<title>/saves suffix) — an
// external-process E2E harness (see e2e/) has no way to set the in-package
// saveBaseDirOverride var, so this is the only way it can sandbox a real
// build's save files away from the player's actual %AppData% profile.
// saveBaseDirOverride (for in-package Go tests), SaveDirFunc (for an
// embedding app), and the OS config dir are still checked, in that order,
// when KAG3_SAVE_DIR is unset.
func saveDir(r *Renderer) (string, error) {
	if v := os.Getenv("KAG3_SAVE_DIR"); v != "" {
		if err := os.MkdirAll(v, 0o755); err != nil {
			return "", err
		}
		return v, nil
	}
	base := saveBaseDirOverride
	if base == "" && SaveDirFunc != nil {
		b, err := SaveDirFunc()
		if err != nil {
			return "", err
		}
		base = b
	}
	if base == "" {
		b, err := os.UserConfigDir()
		if err != nil {
			// os.UserConfigDir() has no android case (unlike os.UserHomeDir,
			// which explicitly returns "/sdcard" there) and always errors on
			// Android — there's no $HOME. This is a last-resort fallback for
			// an embedding app that hasn't registered SaveDirFunc above; not
			// guaranteed writable without storage permissions this package
			// doesn't declare, but every caller of saveDir already tolerates
			// a failure gracefully either way (see handleConfigSave/
			// handleConfigLoad, tags_config.go).
			b, err = os.UserHomeDir()
			if err != nil {
				return "", err
			}
		}
		base = b
	}
	name := "kag3"
	if r.manager != nil && r.manager.Config != nil && r.manager.Config.Title != "" {
		name = r.manager.Config.Title
	}
	dir := filepath.Join(base, "kag3", name, "saves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func slotPath(dir string, slot int, ext string) string {
	return filepath.Join(dir, fmt.Sprintf("slot_%d.%s", slot, ext))
}

// slotStore, if set, replaces the raw os.WriteFile/os.ReadFile/os.Stat calls
// every save slot goes through (saveSlot/loadSlot here, plus
// loadSlotThumbnail/saveSlotInfo/saveSlotLastMessage in tags_uiscreens.go —
// those five are the entire persistent-slot I/O surface). All three fields
// nil, the default, keeps the on-disk behavior every non-wasm platform uses.
//
// Deliberately unexported, unlike SaveDirFunc/ConfigStorage: the only
// implementation is storage_js.go's, which lives in this same package
// because localStorage needs nothing but syscall/js (whereas Android's
// bridge has to live in the embedding app — it calls that app's own Java
// class over JNI). Nothing outside this package needs to swap slot storage
// yet; promote it to exported API if and when something does.
//
// Info is separate from Load rather than derived from it because the save
// picker's row text is the file's *mtime* (saveSlotInfo, tags_uiscreens.go)
// — a backend with no filesystem behind it has to record that timestamp
// itself.
//
// Every hook takes *Renderer so an implementation can namespace by
// Config.Title the same way saveDir's own directory layout does.
var slotStore struct {
	Save func(r *Renderer, slot int, ext string, data []byte) error
	Load func(r *Renderer, slot int, ext string) ([]byte, error)
	Info func(r *Renderer, slot int) (exists bool, modTime time.Time)
}

// lastSnapshot is the save slot thumbnail, written alongside the next
// save*Slot call. Kept fresh automatically every frame (see drawScene in
// renderer.go, which calls captureSnapshot right before drawModal — after
// the full scene is drawn but before any modal overlay is), so by the time
// any save action runs, whatever's here is always "the scene, with no
// modal on top", regardless of how it was reached (direct button, the
// quick menu's own SAVE item, [showsave], TG.menu.doSave, ...).
var lastSnapshot *ebiten.Image

// captureSnapshot copies buf into lastSnapshot. buf is passed explicitly
// (rather than reading the renderBuffer global directly) so drawScene can
// call this with the scene-so-far *before* it draws in the current frame's
// modal overlay (see drawScene's own comment) — same image, different
// point in its own draw sequence, not necessarily renderBuffer's final
// state for this frame.
//
// Reuses the existing lastSnapshot image (Clear + redraw) instead of
// allocating a new one every call: drawScene calls this unconditionally
// every frame, so a fresh ebiten.NewImage(1920, 1080) each time was ~8MB of
// GPU texture churn per frame (~250MB/s at 30 TPS) — fine on desktop/
// simulator GPUs and their fast GC, but enough sustained memory pressure on
// an old/RAM-constrained real iOS device (confirmed: a 2017 iPad Pro 10.5",
// 4GB RAM) to trigger system-wide memory-warning stalls within the first
// idle minute on the title screen, before any scenario-specific rendering.
// [save_img] (tags_sysdesign.go's handleSaveImg) still assigns lastSnapshot
// directly to a differently-sized loaded image; the size check below
// reallocates in that case rather than corrupting it via Clear.
func captureSnapshot(buf *ebiten.Image) {
	if buf == nil {
		return
	}
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	if lastSnapshot == nil || lastSnapshot.Bounds().Dx() != w || lastSnapshot.Bounds().Dy() != h {
		lastSnapshot = ebiten.NewImage(w, h)
	} else {
		lastSnapshot.Clear()
	}
	lastSnapshot.DrawImage(buf, &ebiten.DrawImageOptions{})
	snapshotCaptureCount++
}

// snapshotCaptureCount counts captureSnapshot calls — test-only hook to
// prove drawScene actually re-captured this frame now that lastSnapshot is
// reused in place (see captureSnapshot's own comment) rather than replaced
// with a fresh, identifiably-different object every time.
var snapshotCaptureCount int

func (r *Renderer) saveSlot(slot int) error {
	b, err := json.MarshalIndent(r.buildSaveData(), "", "  ")
	if err != nil {
		return err
	}

	// Both branches below do the same three things — write the JSON, drop the
	// slot picker's now-stale thumbnail cache entry (tags_uiscreens.go), then
	// write the new thumbnail best-effort — against their respective backend.
	if slotStore.Save != nil {
		if err := slotStore.Save(r, slot, "json", b); err != nil {
			return err
		}
		delete(slotThumbnailCache, slot)
		if lastSnapshot != nil {
			// Thumbnail failures stay best-effort here exactly as they are
			// in the file branch below (the os.Create error is deliberately
			// dropped there) — a missing preview image must never turn into
			// a failed save.
			var buf bytes.Buffer
			if err := png.Encode(&buf, lastSnapshot); err == nil {
				_ = slotStore.Save(r, slot, "png", buf.Bytes())
			}
		}
		return nil
	}

	dir, err := saveDir(r)
	if err != nil {
		return err
	}
	if err := os.WriteFile(slotPath(dir, slot, "json"), b, 0o644); err != nil {
		return err
	}
	delete(slotThumbnailCache, slot)
	if lastSnapshot != nil {
		if f, err := os.Create(slotPath(dir, slot, "png")); err == nil {
			_ = png.Encode(f, lastSnapshot)
			f.Close()
		}
	}
	return nil
}

// readSlotFile reads one save slot's <ext> payload, through slotStore when
// an implementation is registered (storage_js.go) and off disk otherwise.
// Shared by loadSlot here and by the picker's own readers in
// tags_uiscreens.go so the "which backend?" decision lives in exactly one
// place. Note it never calls saveDir in the hook branch — on GOOS=js that
// would fail outright (no $HOME for os.UserConfigDir/os.UserHomeDir).
func readSlotFile(r *Renderer, slot int, ext string) ([]byte, error) {
	if slotStore.Load != nil {
		return slotStore.Load(r, slot, ext)
	}
	dir, err := saveDir(r)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(slotPath(dir, slot, ext))
}

func (r *Renderer) loadSlot(slot int) error {
	b, err := readSlotFile(r, slot, "json")
	if err != nil {
		return err
	}
	var data saveData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	return r.applySaveData(&data)
}

// handleSaveSnap captures the current frame as a thumbnail for the *next*
// save*Slot call — redundant with drawScene's own automatic per-frame
// capture in the common case, but harmless to keep: real Tyrano scripts
// may still call [savesnap] explicitly, and this keeps that working.
func handleSaveSnap(ctx *tagCtx) error {
	captureSnapshot(renderBuffer)
	return nil
}

func handleAutoSave(ctx *tagCtx) error { return ctx.r.saveSlot(autoSaveSlot) }
func handleAutoLoad(ctx *tagCtx) error { return ctx.r.loadSlot(autoSaveSlot) }

// checkpointData is [checkpoint]'s single in-memory snapshot — not
// persisted to disk, and not a full history stack (real Tyrano's rollback
// can step back through many prior points; this remembers only the most
// recent [checkpoint]).
var checkpointData *saveData

func handleCheckpoint(ctx *tagCtx) error {
	checkpointData = ctx.r.buildSaveData()
	return nil
}

func handleRollback(ctx *tagCtx) error {
	if checkpointData == nil {
		return nil
	}
	return ctx.r.applySaveData(checkpointData)
}

func handleClearCheckpoint(ctx *tagCtx) error {
	checkpointData = nil
	return nil
}

func handleScreenFull(ctx *tagCtx) error {
	ebiten.SetFullscreen(!ebiten.IsFullscreen())
	return nil
}

// --- start_keyconfig / stop_keyconfig / closeconfirm_on / closeconfirm_off ---
//
// kag3 has no rebindable-key system and no window-close interception yet,
// so these are honest state flags rather than faked behavior — tracked in
// case a future input layer wants to consult them, same spirit as
// [current] in tags_message.go.
var (
	keyConfigEnabled    = true
	closeConfirmEnabled bool
)

func handleStartKeyConfig(ctx *tagCtx) error { keyConfigEnabled = true; return nil }
func handleStopKeyConfig(ctx *tagCtx) error  { keyConfigEnabled = false; return nil }
func handleCloseConfirmOn(ctx *tagCtx) error { closeConfirmEnabled = true; return nil }
func handleCloseConfirmOff(ctx *tagCtx) error {
	closeConfirmEnabled = false
	return nil
}

// --- [dialog]: a minimal built-in OK/Cancel modal ---
//
// Real Tyrano's [dialog] shows a native/system dialog; kag3 draws its own
// tiny modal instead (dim overlay + two text buttons), since there's no
// cross-platform native dialog wired into ebiten here. It blocks the
// calling tag via the same y.Until pattern [wait]/[wse] already use, so no
// special-case coroutine freezing is needed — that's only required for
// backlog/menu below, which are triggered from button clicks outside the
// tag coroutine entirely.
type dialogState struct {
	Text        string
	OKLabel     string
	NGLabel     string
	Target      string
	FalseTarget string
	// Result: 0 pending, 1 OK clicked, 2 NG clicked.
	Result int
	// OnConfirm, if set, marks this as a dialog opened from a button click
	// (role="title" — see confirmGoToTitle) rather than a [dialog] tag:
	// there's no coroutine y.Until to block on outside a tag handler, so
	// Update() polls Result itself and calls OnConfirm once the user picks
	// OK, then clears activeDialog — see anyModalActive (tags_uiscreens.go)
	// and the resolution check in Update() (renderer.go).
	OnConfirm func(*Renderer)
}

var (
	activeDialog      *dialogState
	dialogOKLabel     = "OK"
	dialogNGLabel     = "キャンセル"
	dialogFilterColor = color.RGBA{0, 0, 0, 160}
)

func handleDialog(ctx *tagCtx) error {
	r := ctx.r
	d := &dialogState{
		Text:        ctx.tag.Pm["text"],
		OKLabel:     dialogOKLabel,
		NGLabel:     dialogNGLabel,
		Target:      strings.TrimPrefix(ctx.tag.Pm["target"], "*"),
		FalseTarget: strings.TrimPrefix(ctx.tag.Pm["false_target"], "*"),
	}
	activeDialog = d
	ctx.y.Until(true, func() bool {
		return d.Result != 0
	})
	activeDialog = nil
	label := d.Target
	if d.Result == 2 && d.FalseTarget != "" {
		label = d.FalseTarget
	}
	if label == "" {
		return nil
	}
	if v, ok := r.labels[label]; ok {
		*ctx.i = v.Index
	}
	return nil
}

func handleDialogConfig(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["ok"]; ok {
		dialogOKLabel = v
	}
	if v, ok := ctx.tag.Pm["ng"]; ok {
		dialogNGLabel = v
	}
	return handleDialogConfigFilter(ctx)
}

func handleDialogConfigOK(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["text"]; ok {
		dialogOKLabel = v
	}
	return nil
}

func handleDialogConfigNG(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["text"]; ok {
		dialogNGLabel = v
	}
	return nil
}

func handleDialogConfigFilter(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["color"]
	if !ok {
		return nil
	}
	rr, g, b, err := parseColor(v)
	if err != nil {
		return err
	}
	alpha := 160
	if a, ok := ctx.tag.Pm["opacity"]; ok {
		n, err := strconv.Atoi(a)
		if err != nil {
			return err
		}
		alpha = n
	}
	dialogFilterColor = color.RGBA{uint8(rr), uint8(g), uint8(b), uint8(alpha)}
	return nil
}

// --- shared modal plumbing: dialog / backlog / quick menu ---

// modalRect is a simple hit-testable label button, used by all three
// overlays below.
type modalRect struct {
	Label      string
	X, Y, W, H int
}

func drawModalRect(buf *ebiten.Image, face *text.GoTextFace, m modalRect) {
	fillRect(buf, float64(m.X), float64(m.Y), float64(m.W), float64(m.H), color.RGBA{255, 255, 255, 230})
	tw, th := text.Measure(m.Label, face, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.Black)
	top.GeoM.Translate(float64(m.X)+(float64(m.W)-tw)/2, float64(m.Y)+(float64(m.H)-th)/2)
	text.Draw(buf, m.Label, face, top)
}

// dialogButtonRects computes the OK/NG buttons centered below the dialog
// text. w/h/the button gap and the +60 vertical offset are ×1.5 of the
// original 1280x720-tuned values (X/Y positioning itself was already
// screenW/screenH-relative and needed no change).
func dialogButtonRects(screenW, screenH int) (ok, ng modalRect) {
	w, h := 240, 75
	y := screenH/2 + 60
	ok = modalRect{Label: dialogOKLabelOr(), X: screenW/2 - w - 30, Y: y, W: w, H: h}
	ng = modalRect{Label: dialogNGLabelOr(), X: screenW/2 + 30, Y: y, W: w, H: h}
	return
}

func dialogOKLabelOr() string {
	if activeDialog != nil {
		return activeDialog.OKLabel
	}
	return dialogOKLabel
}

func dialogNGLabelOr() string {
	if activeDialog != nil {
		return activeDialog.NGLabel
	}
	return dialogNGLabel
}

func handleDialogClick(screenW, screenH int) {
	if activeDialog == nil || activeDialog.Result != 0 {
		return
	}
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	ok, ng := dialogButtonRects(screenW, screenH)
	switch {
	case isColisionTouch(mX, mY, ok.X, ok.Y, ok.W, ok.H, touch):
		activeDialog.Result = 1
	case isColisionTouch(mX, mY, ng.X, ng.Y, ng.W, ng.H, touch):
		activeDialog.Result = 2
	}
}

func drawDialog(r *Renderer, buf *ebiten.Image) {
	d := activeDialog
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	fillRect(buf, 0, 0, float64(w), float64(h), dialogFilterColor)
	tw, th := text.Measure(d.Text, r.fontFace, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.White)
	top.GeoM.Translate(float64(w)/2-tw/2, float64(h)/2-40-th/2)
	text.Draw(buf, d.Text, r.fontFace, top)
	ok, ng := dialogButtonRects(w, h)
	drawModalRect(buf, r.fontFace, ok)
	drawModalRect(buf, r.fontFace, ng)
}

// --- backlog (button role="backlog"/LOG in the operation row) ---
//
// Redesigned per the "3a バックログ" mockup: full-screen dim, a header
// ("BACKLOG"/履歴 + 閉じる✕), a fixed-width name column distinct from the
// text column (recordBacklog, tags_message.go, now keeps them separate),
// recency-fade opacity on the oldest few visible rows, a proportional
// scrollbar, and a footer with scroll/navigation hints. Coordinates are
// the source design's own 1920x1080 pixel values, unscaled — see the
// design plan's "画面解像度" note (example/ now runs at 1920x1080).

var (
	backlogViewing     bool
	backlogOpenedFrame int
	// backlogScrollY is how far scrolled *up* from the newest entry (0 =
	// showing the most recent entries at the bottom, the default/rest
	// state) — the opposite sense from slotPickerScrollY (which measures
	// down from the top), because backlog reads newest-at-bottom like a
	// chat transcript.
	backlogScrollY   float64
	backlogDragging  bool
	backlogDragLastY int
	// backlogDidDrag is "has the pointer moved since it was first pressed"
	// — deliberately a separate flag from backlogDragging ("is the pointer
	// currently held down at all"). See updateBacklogDrag's doc comment for
	// why collapsing these into one flag doesn't work.
	backlogDidDrag bool
)

const (
	backlogScrollStep = 40.0

	backlogPaddingTop       = 56.0
	backlogPaddingLeftRight = 96.0
	backlogPaddingBottom    = 44.0
	backlogHeaderTitleSize  = 34.0
	backlogHeaderSubSize    = 20.0
	backlogHeaderPaddingGap = 20.0
	backlogHeaderPaddingBtm = 20.0
	backlogCloseSize        = 22.0
	backlogBodyPaddingTop   = 34.0
	backlogNameColW         = 220.0
	backlogColGap           = 40.0
	backlogNameFontSize     = 26.0
	backlogTextFontSize     = 30.0
	backlogTextLineHeight   = 1.7
	backlogScrollbarColGap  = 28.0
	backlogScrollbarW       = 6.0
	backlogFooterPaddingTop = 20.0
	backlogFooterMarginTop  = 14.0
	backlogFooterFontSize   = 20.0
	backlogFooterItemGap    = 26.0
)

var (
	backlogDimColor          = color.RGBA{0x0a, 0x0c, 0x10, 0xe6} // rgba(10,12,16,0.9)
	backlogHeaderBorderColor = color.RGBA{0x8f, 0xc0, 0xd8, 0x66} // rgba(143,192,216,0.4)
	backlogTitleColor        = color.RGBA{0xf2, 0xf5, 0xf8, 0xff}
	backlogSubColor          = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogCloseColor        = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	backlogNameColor         = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	backlogNarrationColor    = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogTextColor         = color.RGBA{0xe6, 0xeb, 0xf0, 0xff}
	backlogScrollTrackColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0x2e} // rgba(143,192,216,0.18)
	backlogScrollThumbColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
	backlogFooterBorderColor = color.RGBA{0x8f, 0xc0, 0xd8, 0x40} // rgba(143,192,216,0.25)
	backlogFooterDimColor    = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	backlogFooterTextColor   = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	// backlogFadeSteps are the opacities applied to the oldest few visible
	// rows (top of the viewport), newest-first fading in from there —
	// matches the mockup's 0.45/0.6/0.8/1.0 sample. Rows beyond this are
	// fully opaque.
	backlogFadeSteps = []float32{0.45, 0.6, 0.8, 1.0}
)

// backlogEntryHeight is a fixed per-row height (text column's line-height
// at its font size) — entries aren't word-wrapped (matching this package's
// existing "no clipping, no wrap" simplicity elsewhere for secondary UI),
// so this is exact, not an estimate.
const backlogEntryHeight = backlogTextFontSize * backlogTextLineHeight

func backlogContentHeight() float64 {
	if len(backlog) == 0 {
		return 0
	}
	return float64(len(backlog)) * backlogEntryHeight
}

func backlogMaxScroll(viewportH float64) float64 {
	max := backlogContentHeight() - viewportH
	if max < 0 {
		max = 0
	}
	return max
}

func clampBacklogScroll(viewportH float64) {
	max := backlogMaxScroll(viewportH)
	if backlogScrollY > max {
		backlogScrollY = max
	}
	if backlogScrollY < 0 {
		backlogScrollY = 0
	}
}

func backlogFace(r *Renderer, size float64) *text.GoTextFace {
	return &text.GoTextFace{Source: r.fontFace.Source, Size: size, Language: r.fontFace.Language}
}

// backlogNameDisplay is the name column's text for entry — "──" (dimmed)
// for a nameless/narration entry ([pushlog] or a bare "#" line), matching
// the source design's placeholder-narration style.
func backlogNameDisplay(name string) (string, color.RGBA) {
	if name == "" {
		return "──", backlogNarrationColor
	}
	return name, backlogNameColor
}

func drawBacklog(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	fillRect(buf, 0, 0, float64(w), float64(h), backlogDimColor)

	contentX := backlogPaddingLeftRight
	contentRight := float64(w) - backlogPaddingLeftRight
	contentW := contentRight - contentX

	// --- header ---
	titleFace := backlogFace(r, backlogHeaderTitleSize)
	titleW, titleH := text.Measure("BACKLOG", titleFace, 0)
	titleOp := &text.DrawOptions{}
	titleOp.GeoM.Translate(contentX, backlogPaddingTop)
	titleOp.ColorScale.ScaleWithColor(backlogTitleColor)
	text.Draw(buf, "BACKLOG", titleFace, titleOp)

	subFace := backlogFace(r, backlogHeaderSubSize)
	subOp := &text.DrawOptions{}
	subOp.GeoM.Translate(contentX+titleW+backlogHeaderPaddingGap, backlogPaddingTop+(titleH-backlogHeaderSubSize))
	subOp.ColorScale.ScaleWithColor(backlogSubColor)
	text.Draw(buf, "履歴", subFace, subOp)

	const closeLabel = "閉じる ✕"
	closeFace := backlogFace(r, backlogCloseSize)
	closeW, _ := text.Measure(closeLabel, closeFace, 0)
	closeOp := &text.DrawOptions{}
	closeOp.GeoM.Translate(contentRight-closeW, backlogPaddingTop+(titleH-backlogCloseSize))
	closeOp.ColorScale.ScaleWithColor(backlogCloseColor)
	text.Draw(buf, closeLabel, closeFace, closeOp)

	headerBottom := backlogPaddingTop + titleH + backlogHeaderPaddingBtm
	fillRect(buf, contentX, headerBottom, contentW, 1, backlogHeaderBorderColor)

	// --- footer ---
	footerFace := backlogFace(r, backlogFooterFontSize)
	const scrollHint = "ホイール／ドラッグでスクロール"
	_, footerH := text.Measure(scrollHint, footerFace, 0)
	footerBorderY := float64(h) - backlogPaddingBottom - footerH - backlogFooterPaddingTop
	fillRect(buf, contentX, footerBorderY, contentW, 1, backlogFooterBorderColor)
	footerTextY := footerBorderY + backlogFooterPaddingTop
	hintOp := &text.DrawOptions{}
	hintOp.GeoM.Translate(contentX, footerTextY)
	hintOp.ColorScale.ScaleWithColor(backlogFooterDimColor)
	text.Draw(buf, scrollHint, footerFace, hintOp)

	const backHint = "右クリックで戻る"
	const latestHint = "最新へ ▼"
	backW, _ := text.Measure(backHint, footerFace, 0)
	latestW, _ := text.Measure(latestHint, footerFace, 0)
	backOp := &text.DrawOptions{}
	backOp.GeoM.Translate(contentRight-backW, footerTextY)
	backOp.ColorScale.ScaleWithColor(backlogFooterTextColor)
	text.Draw(buf, backHint, footerFace, backOp)
	latestOp := &text.DrawOptions{}
	latestOp.GeoM.Translate(contentRight-backW-backlogFooterItemGap-latestW, footerTextY)
	latestOp.ColorScale.ScaleWithColor(backlogFooterTextColor)
	text.Draw(buf, latestHint, footerFace, latestOp)

	// --- body: scrollable name/text columns + scrollbar ---
	bodyTop := headerBottom + backlogBodyPaddingTop
	bodyBottom := footerBorderY - backlogFooterMarginTop
	viewportH := bodyBottom - bodyTop
	if viewportH < 0 {
		viewportH = 0
	}
	clampBacklogScroll(viewportH)

	textColW := contentW - backlogNameColW - backlogColGap - backlogScrollbarColGap - backlogScrollbarW
	nameFace := backlogFace(r, backlogNameFontSize)
	textFace := backlogFace(r, backlogTextFontSize)

	contentH := backlogContentHeight()
	// scrollTop is the content-space Y of the viewport's own top edge:
	// content is bottom-anchored (newest entry's bottom sits at
	// bodyBottom) when backlogScrollY == 0, and moves up as it increases.
	scrollTop := contentH - viewportH - backlogScrollY

	for i, entry := range backlog {
		rowTop := float64(i) * backlogEntryHeight
		y := bodyTop + (rowTop - scrollTop)
		if y+backlogEntryHeight < bodyTop || y > bodyBottom {
			continue
		}
		// Fade the first few rows *from the top of the viewport*, not by
		// absolute recency — matches the mockup's "fades in as you scroll
		// up toward older lines" read (the bottom-most/newest rows are
		// always fully opaque regardless of scroll position).
		rowIndexFromViewportTop := int((y - bodyTop) / backlogEntryHeight)
		alpha := float32(1.0)
		if rowIndexFromViewportTop >= 0 && rowIndexFromViewportTop < len(backlogFadeSteps) {
			alpha = backlogFadeSteps[rowIndexFromViewportTop]
		}

		name, nameColor := backlogNameDisplay(entry.Name)
		nameColor.A = uint8(float32(nameColor.A) * alpha)
		nameOp := &text.DrawOptions{}
		nameOp.GeoM.Translate(contentX, y)
		nameOp.ColorScale.ScaleWithColor(nameColor)
		text.Draw(buf, name, nameFace, nameOp)

		txtColor := backlogTextColor
		txtColor.A = uint8(float32(txtColor.A) * alpha)
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(contentX+backlogNameColW+backlogColGap, y)
		textOp.ColorScale.ScaleWithColor(txtColor)
		txt := entry.Text
		if w, _ := text.Measure(txt, textFace, 0); w > textColW {
			// No word-wrap for backlog text (matches the rest of this
			// package's secondary-UI simplicity) — truncate instead of
			// overflowing into the scrollbar column.
			for len([]rune(txt)) > 0 {
				rn := []rune(txt)
				txt = string(rn[:len(rn)-1])
				if ww, _ := text.Measure(txt+"…", textFace, 0); ww <= textColW {
					txt += "…"
					break
				}
			}
		}
		text.Draw(buf, txt, textFace, textOp)
	}

	if max := backlogMaxScroll(viewportH); max > 0 {
		trackX := contentRight - backlogScrollbarW
		fillRect(buf, trackX, bodyTop, backlogScrollbarW, viewportH, backlogScrollTrackColor)
		thumbH := viewportH * viewportH / contentH
		if thumbH < 20 {
			thumbH = 20
		}
		if thumbH > viewportH {
			thumbH = viewportH
		}
		// backlogScrollY == 0 anchors the thumb to the bottom (newest
		// visible) — the inverse of scrollY's own top-anchored sense.
		thumbY := bodyTop + (viewportH-thumbH)*(1-backlogScrollY/max)
		fillRect(buf, trackX, thumbY, backlogScrollbarW, thumbH, backlogScrollThumbColor)
	}
}

// updateBacklogDrag advances the backlog's drag-to-scroll state by one
// frame given this frame's raw pointer state, and reports how much to add
// to backlogScrollY (0 if nothing changed). Split out from
// handleBacklogClick so the drag/click distinction is testable without
// faking ebiten's real input state — this package has no way to do that
// directly (see skipShouldAdvance's own doc comment, renderer.go, for the
// same constraint on a different feature).
//
// backlogDidDrag exists as a separate flag from backlogDragging
// specifically because collapsing them doesn't work: a plain click's very
// first frame already has pressed=true with backlogDragging previously
// false, so the "not currently dragging -> start dragging" branch below
// sets backlogDragging = true on that exact frame — before
// handleBacklogClick's own justPressed check ever runs. Using
// backlogDragging there to mean "was this a click or a drag" meant every
// click, including one landing outside the backlog specifically to dismiss
// it, was misclassified as a drag before dismissal could ever be
// evaluated: the "click outside closes the backlog" behavior documented on
// handleBacklogClick below was unreachable. backlogDidDrag instead only
// ever becomes true once the pointer actually moves while held down, and
// is reset the moment a fresh press begins — so a click that never moves
// still reports backlogDidDrag == false on its own justPressed frame.
func updateBacklogDrag(pressed bool, mY int) (scrollDelta float64) {
	if pressed {
		if !backlogDragging {
			backlogDragging = true
			backlogDidDrag = false
			backlogDragLastY = mY
		} else if mY != backlogDragLastY {
			// Dragging down reveals older entries (content moves down with
			// the pointer), so backlogScrollY — which measures up from the
			// newest entry — increases.
			scrollDelta = float64(backlogDragLastY - mY)
			backlogDragLastY = mY
			backlogDidDrag = true
		}
	} else {
		backlogDragging = false
	}
	return scrollDelta
}

// handleBacklogClick drives the backlog screen's input: wheel/drag-to-scroll,
// the header's 閉じる✕ button, and any-other-click/right-click to close
// (preserving the pre-redesign "any click dismisses" behavior for clicks
// that land outside the close button, except on the very frame the screen
// opened — that click is the button press that opened it).
func (r *Renderer) handleBacklogClick() {
	w, h := r.manager.Config.ScreenWidth, r.manager.Config.ScreenHeight
	footerFace := backlogFace(r, backlogFooterFontSize)
	_, footerH := text.Measure("ホイール／ドラッグでスクロール", footerFace, 0)
	footerBorderY := float64(h) - backlogPaddingBottom - footerH - backlogFooterPaddingTop
	titleFace := backlogFace(r, backlogHeaderTitleSize)
	_, titleH := text.Measure("BACKLOG", titleFace, 0)
	headerBottom := backlogPaddingTop + titleH + backlogHeaderPaddingBtm
	bodyTop := headerBottom + backlogBodyPaddingTop
	viewportH := footerBorderY - backlogFooterMarginTop - bodyTop
	if viewportH < 0 {
		viewportH = 0
	}

	if _, wheelY := ebiten.Wheel(); wheelY != 0 {
		backlogScrollY += wheelY * backlogScrollStep
		clampBacklogScroll(viewportH)
	}

	mX, mY, justPressed, pressed, touch := pointerState()
	if delta := updateBacklogDrag(pressed, mY); delta != 0 {
		backlogScrollY += delta
		clampBacklogScroll(viewportH)
	}

	if t == backlogOpenedFrame {
		return
	}
	// Right-click-to-close is a desktop-only convenience — touch has no
	// equivalent gesture here, but the explicit close button and
	// tap-outside-to-dismiss below already cover it.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		backlogViewing = false
		return
	}
	if justPressed {
		contentRight := float64(w) - backlogPaddingLeftRight
		closeFace := backlogFace(r, backlogCloseSize)
		closeW, _ := text.Measure("閉じる ✕", closeFace, 0)
		closeX := contentRight - closeW
		closeY := backlogPaddingTop + (titleH - backlogCloseSize)
		if isColisionTouch(mX, mY, int(closeX), int(closeY), int(closeW), int(backlogCloseSize), touch) {
			backlogViewing = false
			return
		}
		// backlogDidDrag, not backlogDragging — see updateBacklogDrag's doc
		// comment for why the latter is always true by this point on a
		// plain click's own justPressed frame, and would make this branch
		// unreachable.
		if !backlogDidDrag {
			backlogViewing = false
		}
	}
}

// --- quick menu (button role="menu") ---
//
// Deprecated: reachable only via role="menu" or the (also deprecated)
// [showmenubutton] corner icon (tags_sysdesign.go) — the redesigned message
// window's own operation row (tags_oprow.go) now covers everything this
// popup offered (SAVE/LOAD/SKIP/BACK TO TITLE) except HIDE MESSAGE
// (case 2 below; still just buttonRoles["window"]'s toggle, reachable via a
// script-placed [button role="window"] if needed). Left implemented, not
// deleted, for any script that still opens it directly.
//
// Laid out to match real Tyrano's own system menu screen (built from the
// same bundled resources/system/images assets: bg_base.png, label_menu.png,
// menu_button_close.png for the "BACK" button top-right — same as the slot
// picker's — and the five menu_button_*/menu_message_close.png pill
// buttons), at a 1920x1080 canvas (×1.5 from the original 1280x720 layout —
// both these position/size constants and the underlying
// resources/system/images/*.png assets were scaled together, see the
// upscale note in the project history). scene1.ks already places individual
// save/load/skip/auto/backlog buttons directly on screen too, so this
// doesn't need a "閉じる" item of its own — the top-right BACK button
// covers that, consistently with the slot picker.
const (
	quickMenuLabelX, quickMenuLabelY    = 15, 15
	quickMenuButtonX, quickMenuButtonY0 = 570, 285
	quickMenuButtonW, quickMenuButtonH  = 780, 105
	quickMenuButtonGap                  = 38
)

var (
	menuOpen        bool
	menuOpenedFrame int
)

// quickMenuButtonSpec is one pill button: its normal/hover image pair and
// hit-test rect. The order here is the on-screen top-to-bottom order and is
// what quickMenuButtons()'s index maps to in handleQuickMenuClick.
type quickMenuButtonSpec struct {
	Normal, Hover string
	X, Y, W, H    int
}

func quickMenuButtons() []quickMenuButtonSpec {
	names := [...][2]string{
		{"menu_button_save.png", "menu_button_save2.png"},
		{"menu_button_load.png", "menu_button_load2.png"},
		{"menu_message_close.png", "menu_message_close2.png"},
		{"menu_button_skip.png", "menu_button_skip2.png"},
		{"menu_button_title.png", "menu_button_title2.png"},
	}
	items := make([]quickMenuButtonSpec, len(names))
	for i, n := range names {
		items[i] = quickMenuButtonSpec{
			Normal: n[0], Hover: n[1],
			X: quickMenuButtonX, Y: quickMenuButtonY0 + i*(quickMenuButtonH+quickMenuButtonGap),
			W: quickMenuButtonW, H: quickMenuButtonH,
		}
	}
	return items
}

// quickMenuButtonImageName picks btn.Hover/Normal depending on whether
// (mX, mY) is currently over it — split out from drawQuickMenu so it's
// testable without a real ebiten.CursorPosition(), same idea as
// backButtonImageName in tags_uiscreens.go.
func quickMenuButtonImageName(btn quickMenuButtonSpec, mX, mY int, touch bool) string {
	if isColisionTouch(mX, mY, btn.X, btn.Y, btn.W, btn.H, touch) {
		return btn.Hover
	}
	return btn.Normal
}

func drawQuickMenu(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	if bgImg := loadSystemImage(r, "bg_base.png"); bgImg != nil {
		op := &ebiten.DrawImageOptions{}
		bw, bh := bgImg.Bounds().Dx(), bgImg.Bounds().Dy()
		op.GeoM.Scale(float64(w)/float64(bw), float64(h)/float64(bh))
		buf.DrawImage(bgImg, op)
	} else {
		fillRect(buf, 0, 0, float64(w), float64(h), color.RGBA{0, 0, 0, 160})
	}

	if labelImg := loadSystemImage(r, "label_menu.png"); labelImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(quickMenuLabelX, quickMenuLabelY)
		buf.DrawImage(labelImg, op)
	}

	back := backButtonRect(r)
	mX, mY, _, _, touch := pointerState()
	if backImg := loadSystemImage(r, backButtonImageName(back, mX, mY, touch)); backImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(back.X), float64(back.Y))
		buf.DrawImage(backImg, op)
	}

	for _, btn := range quickMenuButtons() {
		if img := loadSystemImage(r, quickMenuButtonImageName(btn, mX, mY, touch)); img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(btn.X), float64(btn.Y))
			buf.DrawImage(img, op)
		}
	}
}

func (r *Renderer) handleQuickMenuClick(screenW, screenH int) {
	mX, mY, justPressed, _, touch := pointerState()
	if !justPressed {
		return
	}
	if t == menuOpenedFrame {
		return
	}

	back := backButtonRect(r)
	if isColisionTouch(mX, mY, back.X, back.Y, back.W, back.H, touch) {
		menuOpen = false
		return
	}

	for idx, btn := range quickMenuButtons() {
		if !isColisionTouch(mX, mY, btn.X, btn.Y, btn.W, btn.H, touch) {
			continue
		}
		switch idx {
		case 0: // SAVE — Phase 9's slot picker (tags_uiscreens.go)
			openSlotPicker(slotPickerSave)
		case 1: // LOAD
			openSlotPicker(slotPickerLoad)
		case 2: // HIDE MESSAGE — same toggle as button role="window"
			menuOpen = false
			textPosition.Visible = !textPosition.Visible
		case 3: // MESSAGE SKIP — same toggle as button role="skip"
			menuOpen = false
			isSkip = !isSkip
			if isSkip {
				isAuto = false
			}
		case 4: // BACK TO TITLE
			menuOpen = false
			confirmGoToTitle(r)
		}
		return
	}
}

// drawModal renders whichever overlay (if any) is currently active, on top
// of the normal scene. Returning bool isn't needed by drawScene (Update
// tracks activity itself via anyModalActive/activeDialog — tags_uiscreens.go),
// so this only draws.
func drawModal(r *Renderer, buf *ebiten.Image) {
	switch {
	case activeDialog != nil:
		drawDialog(r, buf)
	case slotPickerActive != slotPickerNone:
		drawSlotPicker(r, buf)
	case backlogViewing:
		drawBacklog(r, buf)
	case menuOpen:
		drawQuickMenu(r, buf)
	}
}
