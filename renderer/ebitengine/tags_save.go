package ebitengine

import (
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
		ViewCharas:        append([]*kag3.CharaShow(nil), viewCharas...),
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
				if img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["images"], want); err != nil {
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
		img, _, err := ebitenutil.NewImageFromFileSystem(r.fses["images"], storage)
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
	viewCharas = reconcileViewCharas(r, append([]*kag3.CharaShow(nil), d.ViewCharas...), d.CharaStorage, d.CharaFaces)
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

// saveDir resolves the directory save/load slots live in. KAG3_SAVE_DIR, if
// set, wins outright and is used as-is (no kag3/<title>/saves suffix) — an
// external-process E2E harness (see e2e/) has no way to set the in-package
// saveBaseDirOverride var, so this is the only way it can sandbox a real
// build's save files away from the player's actual %AppData% profile.
// saveBaseDirOverride (for in-package Go tests) and the OS config dir are
// still checked, in that order, when KAG3_SAVE_DIR is unset.
func saveDir(r *Renderer) (string, error) {
	if v := os.Getenv("KAG3_SAVE_DIR"); v != "" {
		if err := os.MkdirAll(v, 0o755); err != nil {
			return "", err
		}
		return v, nil
	}
	base := saveBaseDirOverride
	if base == "" {
		b, err := os.UserConfigDir()
		if err != nil {
			return "", err
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
func captureSnapshot(buf *ebiten.Image) {
	if buf == nil {
		return
	}
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	snap := ebiten.NewImage(w, h)
	snap.DrawImage(buf, &ebiten.DrawImageOptions{})
	lastSnapshot = snap
}

func (r *Renderer) saveSlot(slot int) error {
	dir, err := saveDir(r)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(r.buildSaveData(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(slotPath(dir, slot, "json"), b, 0o644); err != nil {
		return err
	}
	// The slot picker's thumbnail cache (tags_uiscreens.go) may be holding a
	// stale (or absent) decode of slot_<N>.png from before this save.
	delete(slotThumbnailCache, slot)
	if lastSnapshot != nil {
		if f, err := os.Create(slotPath(dir, slot, "png")); err == nil {
			_ = png.Encode(f, lastSnapshot)
			f.Close()
		}
	}
	return nil
}

func (r *Renderer) loadSlot(slot int) error {
	dir, err := saveDir(r)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(slotPath(dir, slot, "json"))
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
	box := ebiten.NewImage(m.W, m.H)
	box.Fill(color.RGBA{255, 255, 255, 230})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(m.X), float64(m.Y))
	buf.DrawImage(box, op)
	tw, th := text.Measure(m.Label, face, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.Black)
	top.GeoM.Translate(float64(m.X)+(float64(m.W)-tw)/2, float64(m.Y)+(float64(m.H)-th)/2)
	text.Draw(buf, m.Label, face, top)
}

func dialogButtonRects(screenW, screenH int) (ok, ng modalRect) {
	w, h := 160, 50
	y := screenH/2 + 40
	ok = modalRect{Label: dialogOKLabelOr(), X: screenW/2 - w - 20, Y: y, W: w, H: h}
	ng = modalRect{Label: dialogNGLabelOr(), X: screenW/2 + 20, Y: y, W: w, H: h}
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
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mX, mY := ebiten.CursorPosition()
	ok, ng := dialogButtonRects(screenW, screenH)
	switch {
	case isColision(mX, mY, ok.X, ok.Y, ok.W, ok.H):
		activeDialog.Result = 1
	case isColision(mX, mY, ng.X, ng.Y, ng.W, ng.H):
		activeDialog.Result = 2
	}
}

func drawDialog(r *Renderer, buf *ebiten.Image) {
	d := activeDialog
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	dim := ebiten.NewImage(w, h)
	dim.Fill(dialogFilterColor)
	buf.DrawImage(dim, &ebiten.DrawImageOptions{})
	tw, th := text.Measure(d.Text, r.fontFace, 0)
	top := &text.DrawOptions{}
	top.ColorScale.ScaleWithColor(color.White)
	top.GeoM.Translate(float64(w)/2-tw/2, float64(h)/2-40-th/2)
	text.Draw(buf, d.Text, r.fontFace, top)
	ok, ng := dialogButtonRects(w, h)
	drawModalRect(buf, r.fontFace, ok)
	drawModalRect(buf, r.fontFace, ng)
}

// --- backlog (button role="backlog") ---

var (
	backlogViewing     bool
	backlogOpenedFrame int
)

func drawBacklog(r *Renderer, buf *ebiten.Image) {
	w, h := buf.Bounds().Dx(), buf.Bounds().Dy()
	dim := ebiten.NewImage(w, h)
	dim.Fill(color.RGBA{0, 0, 0, 200})
	buf.DrawImage(dim, &ebiten.DrawImageOptions{})

	margin := 40.0
	lineHeight := r.fontFace.Size + 8
	// Most recent entries at the bottom, like a chat transcript — show as
	// many as fit, walking backward from the end of backlog.
	maxLines := int((float64(h) - 2*margin) / lineHeight)
	start := len(backlog) - maxLines
	if start < 0 {
		start = 0
	}
	y := margin
	for _, line := range backlog[start:] {
		op := &text.DrawOptions{}
		op.ColorScale.ScaleWithColor(color.White)
		op.GeoM.Translate(margin, y)
		text.Draw(buf, line, r.fontFace, op)
		y += lineHeight
	}
}

// --- quick menu (button role="menu") ---
//
// Laid out to match real Tyrano's own system menu screen (built from the
// same bundled resources/system/images assets: bg_base.png, label_menu.png,
// menu_button_close.png for the "BACK" button top-right — same as the slot
// picker's — and the five menu_button_*/menu_message_close.png pill
// buttons), at a 1280x720 canvas. scene1.ks already places individual
// save/load/skip/auto/backlog buttons directly on screen too, so this
// doesn't need a "閉じる" item of its own — the top-right BACK button
// covers that, consistently with the slot picker.
const (
	quickMenuLabelX, quickMenuLabelY    = 10, 10
	quickMenuButtonX, quickMenuButtonY0 = 380, 190
	quickMenuButtonW, quickMenuButtonH  = 520, 70
	quickMenuButtonGap                  = 25
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
func quickMenuButtonImageName(btn quickMenuButtonSpec, mX, mY int) string {
	if isColision(mX, mY, btn.X, btn.Y, btn.W, btn.H) {
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
		dim := ebiten.NewImage(w, h)
		dim.Fill(color.RGBA{0, 0, 0, 160})
		buf.DrawImage(dim, &ebiten.DrawImageOptions{})
	}

	if labelImg := loadSystemImage(r, "label_menu.png"); labelImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(quickMenuLabelX, quickMenuLabelY)
		buf.DrawImage(labelImg, op)
	}

	back := backButtonRect(r)
	mX, mY := ebiten.CursorPosition()
	if backImg := loadSystemImage(r, backButtonImageName(back, mX, mY)); backImg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(back.X), float64(back.Y))
		buf.DrawImage(backImg, op)
	}

	for _, btn := range quickMenuButtons() {
		if img := loadSystemImage(r, quickMenuButtonImageName(btn, mX, mY)); img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(btn.X), float64(btn.Y))
			buf.DrawImage(img, op)
		}
	}
}

func (r *Renderer) handleQuickMenuClick(screenW, screenH int) {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	if t == menuOpenedFrame {
		return
	}
	mX, mY := ebiten.CursorPosition()

	back := backButtonRect(r)
	if isColision(mX, mY, back.X, back.Y, back.W, back.H) {
		menuOpen = false
		return
	}

	for idx, btn := range quickMenuButtons() {
		if !isColision(mX, mY, btn.X, btn.Y, btn.W, btn.H) {
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
