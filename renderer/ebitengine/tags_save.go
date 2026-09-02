package ebitengine

func init() {
	register("savesnap", handleSaveSnap)
	register("autosave", handleAutoSave)
	register("autoload", handleAutoLoad)
	register("checkpoint", handleCheckpoint)
	register("rollback", handleRollback)
	register("clear_checkpoint", handleClearCheckpoint)
}

// Reserved slot numbers for the save paths that carry no explicit slot
// attribute: role="quicksave"/"quickload" (role_dispatch.go) and
// [autosave]/[autoload] below. role="save"/"load" instead open the slot
// picker (showsave/showload, tags_uiscreens.go), matching real Tyrano;
// manualSaveSlot is the first of the slots that picker offers.
const (
	manualSaveSlot = 1
	quickSaveSlot  = 0
	autoSaveSlot   = -1
)

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
