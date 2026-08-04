package ebitengine

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/ebinovel/kag3"
)

func TestOpBarConfigTogglesVisibilityAndQuickSaveGroup(t *testing.T) {
	defer func() { opRowVisible, opRowShowQuickSave = true, true }()
	opRowVisible, opRowShowQuickSave = true, true

	tag := kag3.TagObject{Name: "opbar_config", Pm: map[string]string{"visible": "false", "showquicksave": "false"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if opRowVisible {
		t.Error("expected opRowVisible = false")
	}
	if opRowShowQuickSave {
		t.Error("expected opRowShowQuickSave = false")
	}
}

// TestOpRowGroupsIncludesTitleButton covers the button added to replace
// the deprecated corner menu button/quick menu (tags_sysdesign.go/
// tags_save.go) as the only entry point back to title once scene1.ks
// stopped calling @showmenubutton — it must reuse buttonRoles["title"]
// (confirmGoToTitle), not a new implementation.
func TestOpRowGroupsIncludesTitleButton(t *testing.T) {
	var found *opRowButton
	for _, g := range opRowGroups() {
		for i, b := range g {
			if b.Label == "Title" {
				found = &g[i]
			}
		}
	}
	if found == nil {
		t.Fatal("expected a \"Title\" button in opRowGroups()")
	}
	if found.Role != "title" {
		t.Errorf("Title button Role = %q, want %q (buttonRoles[\"title\"] == confirmGoToTitle)", found.Role, "title")
	}
	if _, ok := buttonRoles["title"]; !ok {
		t.Fatal("buttonRoles[\"title\"] does not exist — the Title button's Role would silently no-op")
	}
}

func TestOpRowGroupsOmitsQuickSaveGroupWhenDisabled(t *testing.T) {
	defer func() { opRowShowQuickSave = true }()

	opRowShowQuickSave = true
	withQS := opRowGroups()
	foundQS := false
	for _, g := range withQS {
		for _, b := range g {
			if b.Label == "Q.SAVE" {
				foundQS = true
			}
		}
	}
	if !foundQS {
		t.Error("expected Q.SAVE present when opRowShowQuickSave = true")
	}

	opRowShowQuickSave = false
	without := opRowGroups()
	for _, g := range without {
		for _, b := range g {
			if b.Label == "Q.SAVE" || b.Label == "Q.LOAD" {
				t.Errorf("expected Q.SAVE/Q.LOAD absent when opRowShowQuickSave = false, got %+v", without)
			}
			if b.Label == "LOG" {
				// LOG must survive — only the quicksave/quickload group is
				// gated by opRowShowQuickSave, not LOG's own group.
			}
		}
	}
}

func TestDispatchOperationRowClickAtRunsKnownRole(t *testing.T) {
	// See TestDrawOperationRowNoPanicWhenActiveAndInactive's comment: restore
	// rather than null out, since other tests rely on textPosition being
	// left non-nil by whatever ran before them.
	savedTextPosition, savedGLinks, savedIsJump, savedIsSkip := textPosition, glinks, isJump, isSkip
	defer func() {
		textPosition, glinks, isJump, isSkip = savedTextPosition, savedGLinks, savedIsJump, savedIsSkip
	}()

	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginRight: 56}
	glinks, isJump = nil, false
	isSkip = false
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)

	items, total := opRowLayout(r)
	startX, y := opRowOrigin(total)
	var skipItem *opRowItem
	for idx := range items {
		if items[idx].Btn.Label == "SKIP" {
			skipItem = &items[idx]
			break
		}
	}
	if skipItem == nil {
		t.Fatal("expected a SKIP button in the layout")
	}

	r.dispatchOperationRowClickAt(int(startX+skipItem.X+1), int(y+1))
	if !isSkip {
		t.Error("expected clicking SKIP to dispatch buttonRoles[\"skip\"] and set isSkip = true")
	}
}

func TestDispatchOperationRowClickAtOpensConfigViaSleepgamePath(t *testing.T) {
	// See TestDrawOperationRowNoPanicWhenActiveAndInactive's comment: restore
	// rather than null out.
	savedTextPosition, savedGLinks, savedIsJump, savedJumpIndex := textPosition, glinks, isJump, jumpIndex
	savedBg, savedBg2 := bg, bg2
	defer func() {
		textPosition, glinks, isJump, jumpIndex = savedTextPosition, savedGLinks, savedIsJump, savedJumpIndex
		bg, bg2 = savedBg, savedBg2
	}()

	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginRight: 56}
	textPosition.BackImage = newTestImage(1728, 300)
	glinks, isJump, jumpIndex = nil, false, 0
	bg, bg2 = &kag3.Background{}, &kag3.Background{}
	r := newTestRendererWithImageFS(t, map[string][]byte{})
	r.fontFace = newTestFontFace(t)
	r.manager.Labels = map[string]kag3.LabelInfo{}
	r.manager.Senario = kag3.Senario{}
	r.manager.FSes = map[string]fs.FS{"senarios": fstest.MapFS{}}
	r.currentStorage = "scene1.ks"
	beforeSleepStackLen := len(r.sleepStack)

	items, total := opRowLayout(r)
	startX, y := opRowOrigin(total)
	var configItem *opRowItem
	for idx := range items {
		if items[idx].Btn.Label == "設定" {
			configItem = &items[idx]
			break
		}
	}
	if configItem == nil {
		t.Fatal("expected a 設定 button in the layout")
	}

	// loadScript("config.ks") will fail (no such file in this empty test
	// FS) — that's fine, openConfigScreen's sleepStack push and
	// isJump/jumpIndex bookkeeping happen unconditionally before the
	// (unchecked, same as hitButtons' own call) loadScript call.
	r.dispatchOperationRowClickAt(int(startX+configItem.X+1), int(y+1))

	if len(r.sleepStack) != beforeSleepStackLen+1 {
		t.Fatalf("sleepStack len = %d, want %d (one frame pushed)", len(r.sleepStack), beforeSleepStackLen+1)
	}
	if got := r.sleepStack[len(r.sleepStack)-1].Storage; got != "scene1.ks" {
		t.Errorf("pushed sleepFrame.Storage = %q, want %q", got, "scene1.ks")
	}
	if !isJump || jumpIndex != 0 {
		t.Errorf("isJump/jumpIndex = %v/%d, want true/0", isJump, jumpIndex)
	}
}

// TestHandleOperationRowClickNoopWithoutRealClick mirrors
// TestMenuButtonClickOpensQuickMenu's approach (tags_sysdesign_test.go):
// ebiten's real mouse-press state can't be synthesized in a headless unit
// test, so this only verifies handleOperationRowClick doesn't panic or act
// without one.
func TestHandleOperationRowClickNoopWithoutRealClick(t *testing.T) {
	// See TestDrawOperationRowNoPanicWhenActiveAndInactive's comment: restore
	// rather than null out.
	savedTextPosition, savedGLinks, savedIsJump := textPosition, glinks, isJump
	defer func() { textPosition, glinks, isJump = savedTextPosition, savedGLinks, savedIsJump }()

	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginRight: 56}
	glinks, isJump = nil, false
	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)

	r.handleOperationRowClick() // must not panic
}

func TestDrawOperationRowNoPanicWhenActiveAndInactive(t *testing.T) {
	// Restores the pre-test textPosition rather than forcing nil: other
	// tests in this package (e.g. tags_save_test.go's TestSaveSlotRoundTrip)
	// rely on whatever an earlier test left textPosition as, a known
	// cross-test-ordering fragility documented in this repo's CLAUDE.md —
	// leaving it nil here would break any test that runs after this one.
	savedTextPosition, savedGLinks, savedIsJump, savedOpRowVisible := textPosition, glinks, isJump, opRowVisible
	defer func() {
		textPosition, glinks, isJump, opRowVisible = savedTextPosition, savedGLinks, savedIsJump, savedOpRowVisible
	}()

	r := newTestRenderer()
	r.fontFace = newTestFontFace(t)
	buf := newTestImage(1920, 1080)

	// Inactive: no textPosition.
	textPosition = nil
	drawOperationRow(r, buf)

	// Active.
	textPosition = &kag3.TextPosition{Visible: true, Left: 96, Top: 736, Width: 1728, Height: 300, MarginRight: 56}
	glinks, isJump, opRowVisible = nil, false, true
	drawOperationRow(r, buf)

	// Hidden behind an active choice overlay.
	glinks, isJump = []*kag3.GLink{{Target: "somewhere"}}, false
	drawOperationRow(r, buf)
}
