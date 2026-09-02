package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// applyBgFromSnapshot writes s's non-image fields onto bg, then reloads
// Image from s.Storage (the GPU texture a bgSaveState can't carry through
// JSON — see bgSaveState's doc comment). A no-op on the image reload when
// s.Storage is empty (nothing was ever captured, e.g. a save file carrying
// no such field). The caller decides how to react to a reload failure (applySaveData
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
// face beyond "default" finds nothing (handleCharaMod guards that rather
// than crashing). A save file carrying no such field, or a name
// reconcileViewCharas had to fall back to charaStorage for some other
// reason, still gets a working "default" entry.
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
	// save_data.go's own doc comments), so whatever's in `buttons` right now
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
	// as Bg.Storage!="" inside applyBgFromSnapshot — so loading a save file
	// that carries no such field doesn't stomp a same-process textPosition
	// that's already correctly configured with zeroed-out layout (see
	// applyTextPositionFromSnapshot's own no-op guard).
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
	// nil check (not just "always assign"): a save file carrying no Ptexts
	// decodes it as nil, and assigning that would wipe out whatever ptext
	// layout the *current* session already has.
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
	// nil check, same reasoning as Ptexts above: a save file carrying no
	// Texts decodes it as nil, and this is the *only* thing that puts the
	// loaded position's actual dialogue back on screen — without it,
	// jumpIndex resumes execution at the [p]/[s]/[l] tag itself, which
	// blocks again but never replays whatever TextObject(s) before it in
	// the script originally revealed the text it was blocking for, leaving
	// the message window completely blank after every load.
	if d.Texts != nil {
		r.texts = copyTexts(d.Texts)
	} else {
		r.texts = make(map[int][]Text)
	}
	// Same gap, for the [link]/[glink] choices actually visible in the
	// message window rather than the plain text around them — a save
	// landing on an [s] right after a [link]/[endlink] pair never re-runs
	// handleLink on load, so without this the choice itself never reappears
	// either, leaving nothing on screen the player could click at all.
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
