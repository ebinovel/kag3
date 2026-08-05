package ebitengine

import (
	"image/color"
	"strconv"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("opbar_config", handleOpBarConfig)
}

// opRowButton is one label in the persistent operation row above the
// message box. Role, when set, is looked up in buttonRoles (role_dispatch.go)
// — the same map [button role=...] already dispatches through, so this row
// reuses the exact same actions rather than re-implementing them. onClick
// covers the two that don't map onto an existing role: 設定 (no "config"
// role exists — see openConfigScreen) and 既読SKIP (no unread-skip logic
// exists anywhere in the engine yet, see unreadSkipEnabled).
type opRowButton struct {
	Label   string
	Role    string
	onClick func(r *Renderer)
}

var (
	// opRowVisible/opRowShowQuickSave are plain package vars in the same
	// spirit as menuButtonVisible/sysViewVisible (tags_sysdesign.go) —
	// simple on/off toggles, not script-configurable beyond [opbar_config].
	opRowVisible       = true
	opRowShowQuickSave = true
	// unreadSkipEnabled is an honest state-only stub: kag3.Config's own
	// UnReadTextSkip field (config.go) is likewise never consulted by any
	// skip logic today, so toggling this changes the label's color but not
	// actual skip behavior — matching tags_sysdesign.go's existing
	// honest-stub convention (e.g. handleSetResizeCall) rather than
	// pretending to implement unread-tracking here.
	unreadSkipEnabled bool
)

func handleOpBarConfig(ctx *tagCtx) error {
	pm := ctx.tag.Pm
	if v, ok := pm["visible"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		opRowVisible = b
	}
	if v, ok := pm["showquicksave"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		opRowShowQuickSave = b
	}
	return nil
}

// opRowGroups returns the row's button groups left-to-right, exactly the
// spec's grouping: [AUTO,SKIP,既読SKIP] | [LOG] | (optional) [Q.SAVE,Q.LOAD]
// | [SAVE,LOAD,設定,Title]. LOG is its own group (always shown) separate
// from the quicksave/quickload group so opRowShowQuickSave=false only
// removes the latter, matching the source design's own sc-if placement.
// "Title" is not part of the source design's own mockup — added when the
// corner menu button/quick menu (tags_sysdesign.go/tags_save.go, the
// pre-redesign way to reach title) was removed, so returning to title
// still has an entry point. Reuses buttonRoles["title"] (confirmGoToTitle,
// same confirm-dialog flow the old quick menu's BACK TO TITLE row used) —
// not a new implementation.
func opRowGroups() [][]opRowButton {
	groups := [][]opRowButton{
		{
			{Label: "AUTO", Role: "auto"},
			{Label: "SKIP", Role: "skip"},
			{Label: "既読SKIP", onClick: func(r *Renderer) { unreadSkipEnabled = !unreadSkipEnabled }},
		},
		{
			{Label: "LOG", Role: "backlog"},
		},
	}
	if opRowShowQuickSave {
		groups = append(groups, []opRowButton{
			{Label: "Q.SAVE", Role: "quicksave"},
			{Label: "Q.LOAD", Role: "quickload"},
		})
	}
	groups = append(groups, []opRowButton{
		{Label: "SAVE", Role: "save"},
		{Label: "LOAD", Role: "load"},
		{Label: "設定", onClick: openConfigScreen},
		{Label: "Title", Role: "title"},
	})
	return groups
}

// openConfigScreen is 設定's onClick: title.ks's own config button uses
// [button role="sleepgame" storage="config.ks"], whose return-address
// bookkeeping happens inline in hitButtons (input_hit.go) rather than
// through buttonRoles — there's no "config" role to look up, so this
// replicates that exact sequence instead of inventing a new one.
func openConfigScreen(r *Renderer) {
	r.sleepStack = append(r.sleepStack, sleepFrame{
		Storage:      r.currentStorage,
		Index:        currentScriptIndex,
		Buttons:      append([]*kag3.Button(nil), buttons...),
		Bg:           *bg,
		TextPosition: *textPosition,
	})
	screenChanged = true
	r.loadScript("config.ks")
	jumpIndex = 0
	isJump = true
}

const (
	opRowFontSize = 22
	opRowGap      = 22.0
	opRowGroupGap = 26.0
	opRowAboveBox = 16.0
	opRowDividerW = 1.0
	opRowDividerH = 22.0
)

var (
	opRowTextColor    = color.RGBA{0xd5, 0xdd, 0xe4, 0xff}
	opRowDimColor     = color.RGBA{0x8a, 0x94, 0x9e, 0xff}
	opRowDividerColor = color.RGBA{0x6f, 0x7a, 0x85, 0xff}
	opRowActiveColor  = color.RGBA{0x8f, 0xc0, 0xd8, 0xff}
)

func opRowFace(r *Renderer) *text.GoTextFace {
	return &text.GoTextFace{Source: r.fontFace.Source, Size: opRowFontSize, Language: r.fontFace.Language}
}

// opRowItem is either a button (Btn set, IsDivider false) or a group
// divider bar (IsDivider true) — X/W are both relative to the row's own
// left edge (startX in drawOperationRow/handleOperationRowClick), not
// screen-absolute, so the two can share this one layout function and stay
// in sync by construction.
type opRowItem struct {
	Btn       opRowButton
	IsDivider bool
	X, W      float64
}

// opRowLayout lays out opRowGroups() left-to-right and reports the row's
// total width — shared by drawOperationRow and handleOperationRowClick so
// hit-testing always matches what's actually drawn.
func opRowLayout(r *Renderer) (items []opRowItem, totalWidth float64) {
	face := opRowFace(r)
	groups := opRowGroups()
	x := 0.0
	for gi, group := range groups {
		if gi > 0 {
			x += opRowGroupGap
			items = append(items, opRowItem{IsDivider: true, X: x, W: opRowDividerW})
			x += opRowDividerW + opRowGroupGap
		}
		for bi, btn := range group {
			w, _ := text.Measure(btn.Label, face, 0)
			items = append(items, opRowItem{Btn: btn, X: x, W: w})
			x += w
			if bi < len(group)-1 {
				x += opRowGap
			}
		}
	}
	return items, x
}

// opRowOrigin computes the row's top-left (startX, y): right-aligned to the
// message box's own right edge (Left+Width-MarginRight), sitting
// opRowAboveBox above the box's top edge. Shared by draw and hit-test.
func opRowOrigin(totalWidth float64) (startX, y float64) {
	rightEdge := float64(textPosition.Left) + float64(textPosition.Width) - float64(textPosition.MarginRight)
	startX = rightEdge - totalWidth
	y = float64(textPosition.Top) - opRowAboveBox - opRowDividerH
	return
}

// opRowActive reports whether the row is currently something the player can
// see/click at all: the redesign is opted into (Config.MessageBoxStyle —
// renderer/ebitengine is a shared package, so this row must stay invisible
// for any other project importing kag3 unless it explicitly asks for the
// メッセージ欄 redesign), box visible, not suppressed, and not hidden
// behind the [glink] choice dim overlay (drawChoiceDimOverlay/draw_link.go
// covers it visually; this guard keeps clicks from landing on it too while
// hidden).
func opRowActive(r *Renderer) bool {
	return r.manager.Config.MessageBoxStyle == "redesigned" &&
		opRowVisible && textPosition != nil && textPosition.Visible && !(len(glinks) > 0 && !isJump)
}

func drawOperationRow(r *Renderer, buf *ebiten.Image) {
	if !opRowActive(r) {
		return
	}
	face := opRowFace(r)
	items, total := opRowLayout(r)
	startX, y := opRowOrigin(total)
	for _, item := range items {
		if item.IsDivider {
			fillRect(buf, startX+item.X, y, item.W, opRowDividerH, opRowDividerColor)
			continue
		}
		col := opRowTextColor
		switch {
		case item.Btn.Label == "既読SKIP":
			if unreadSkipEnabled {
				col = opRowActiveColor
			} else {
				col = opRowDimColor
			}
		case item.Btn.Label == "AUTO" && isAuto:
			col = opRowActiveColor
		case item.Btn.Label == "SKIP" && isSkip:
			col = opRowActiveColor
		}
		op := &text.DrawOptions{}
		op.GeoM.Translate(startX+item.X, y)
		op.ColorScale.ScaleWithColor(col)
		text.Draw(buf, item.Btn.Label, face, op)
	}
}

func (r *Renderer) handleOperationRowClick() {
	if !opRowActive(r) {
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mX, mY := ebiten.CursorPosition()
	r.dispatchOperationRowClickAt(mX, mY)
}

// dispatchOperationRowClickAt is handleOperationRowClick's testable core —
// split out so tests can drive a click at a known position without needing
// to fake ebiten's real mouse-press state (no precedent for that exists in
// this package; see tags_sysdesign_test.go's TestMenuButtonClickOpensQuickMenu
// for the same constraint on the pre-existing menu button).
func (r *Renderer) dispatchOperationRowClickAt(mX, mY int) {
	items, total := opRowLayout(r)
	startX, y := opRowOrigin(total)
	for _, item := range items {
		if item.IsDivider {
			continue
		}
		if isColision(mX, mY, int(startX+item.X), int(y), int(item.W), int(opRowDividerH)) {
			if item.Btn.Role != "" {
				if fn, ok := buttonRoles[item.Btn.Role]; ok {
					fn(r)
				}
			} else if item.Btn.onClick != nil {
				item.Btn.onClick(r)
			}
			return
		}
	}
}
