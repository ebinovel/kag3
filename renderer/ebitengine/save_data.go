package ebitengine

import (
	"image/color"

	"github.com/ebinovel/kag3"
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

// saveData is a minimal in-memory-first save format: enough to resume
// script execution and restore variables faithfully, not a pixel-perfect
// visual snapshot. bg2 (the secondary background layer) and
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
	// nothing but the "default" face reconcileViewCharas seeds on its own —
	// see the save/load gaps note above.
	CharaFaces   map[string]map[string]string
	Bg           bgSaveState
	TextPosition textPositionSaveState
	// TextStyle/DefaultTextStyle are the package-level textStyle/
	// defaultTextStyle vars ([font]/[deffont]/[resetfont], tags_text.go) at
	// save time. kag3.TextStyle has no *ebiten.Image fields, so unlike
	// TextPosition it round-trips through JSON as-is — no separate
	// "SaveState" struct needed. Without this, loading a save taken while
	// [font color=...] was in effect shows the *current* session's color
	// instead (e.g. black after a later [deffont] call): applySaveData
	// resumes via jumpIndex straight into the saved position, skipping
	// whatever [font] tag was in scope there. nil (nothing set, or a save
	// file carrying no such field) is itself a valid, safe value — the
	// renderer's built-in default look.
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
	// [checkpoint]) resumes with a *blank* message window: the blocking tag
	// re-runs and blocks again, but the dialogue line(s) it was blocking
	// *for* are never replayed. A save landing on an [s] right after a
	// [link] choice loses the message text *and* the choice itself (see
	// Links/GLinks below), leaving nothing on screen to click — the load
	// looks like it silently did nothing at all.
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
// shallow append (sharing the original *CharaShow pointers) lets an [anim]
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
