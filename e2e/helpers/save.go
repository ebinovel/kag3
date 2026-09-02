//go:build windows

package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Reserved slot numbers, mirroring renderer/ebitengine/tags_save.go's
// manualSaveSlot/quickSaveSlot/autoSaveSlot constants. ManualSaveSlot is
// what the slot picker writes to via its first row (see
// nav.go's ClickSlotPickerRow1) — the current example has no
// role="save"/"quicksave" [button] to target QuickSaveSlot/AutoSaveSlot
// directly, they're listed here only to mirror the renderer-side
// constants.
const (
	ManualSaveSlot = 1
	QuickSaveSlot  = 0
	AutoSaveSlot   = -1
)

// PText mirrors kag3.PText's JSON-relevant fields (renderer/ebitengine's
// ptexts entries) — only what e2e tests currently assert on. Add fields
// here as needed; unknown JSON fields are silently ignored, so this never
// needs to track kag3.PText exhaustively.
type PText struct {
	Name string `json:"Name"`
	X    int    `json:"X"`
	Y    int    `json:"Y"`
	Text string `json:"Text"`
}

// SaveData mirrors the subset of renderer/ebitengine/tags_save.go's
// unexported saveData struct that e2e flow tests assert on. It's a
// separate type (not shared code) deliberately: the two packages don't
// import each other, and JSON's "unknown fields are ignored" behavior
// means kag3 can add saveData fields freely without breaking this — only
// a field *rename* or *removal* on the kag3 side would need a matching
// update here.
type SaveData struct {
	Storage        string            `json:"Storage"`
	Index          int               `json:"Index"`
	Ptexts         map[string]*PText `json:"Ptexts"`
	CharaNamePText string            `json:"CharaNamePText"`
}

// ReadSaveSlot reads and decodes slot_<slot>.json from saveDir — the same
// directory Game.SaveDir (from LaunchGame) points KAG3_SAVE_DIR at. Returns
// an error (not a zero value) if the slot doesn't exist yet, so tests fail
// loudly on a save that never happened rather than silently comparing
// against zero values.
func ReadSaveSlot(saveDir string, slot int) (*SaveData, error) {
	path := filepath.Join(saveDir, fmt.Sprintf("slot_%d.json", slot))
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading save slot %d: %w", slot, err)
	}
	var data SaveData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("decoding save slot %d (%s): %w", slot, path, err)
	}
	return &data, nil
}
