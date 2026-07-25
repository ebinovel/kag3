package ebitengine

import "github.com/ebinovel/kag3"

// goToTitle resets session state and jumps to title.ks, shared by button
// role="title" and the quick-menu's title item (see tags_save.go).
func (r *Renderer) goToTitle() {
	screenChanged = true
	r.loadScript("title.ks")
	r.texts = make(map[int][]Text)
	r.callStack = nil
	viewCharas = nil
	charaName = ""
	pendingRuby = ""
	// title.ks itself never touches these — real Tyrano's own title screen
	// only ever gets shown once, right after the boot iscript that hides
	// them initially. Since goToTitle can now re-enter title.ks at any
	// point in the middle of a playthrough (role="title", the quick menu's
	// "BACK TO TITLE"), gameplay UI left over from wherever we were has to
	// be reset explicitly here, or it bleeds through — not just onto the
	// title screen itself, but into whatever scenario starts next. [position]
	// (r.position in this file) only ever *merges* the attributes a given
	// tag call specifies, so a field a later [position] call never touches
	// again (most notably frame=, but also color/margins/vertical/...) stays
	// whatever the *previous* playthrough last set it to: reported as a
	// custom end-of-story message-window frame (scene1.ks's
	// [position frame="frame.png" ...] near the end) still showing behind
	// scene1.ks's very first line after choosing "はじめから" a second time,
	// since that early [position] call never specifies frame= to clear it.
	// A fresh struct matches exactly what NewRenderer starts a brand-new
	// process with. textStyle/defaultTextStyle ([font]/[deffont]) are the
	// same kind of never-explicitly-cleared global and get the same
	// treatment, for the same reason (scene1.ks's end-of-story
	// [deffont color="0x454D51"] otherwise recolors the next playthrough's
	// very first lines too).
	textPosition = &kag3.TextPosition{}
	textStyle = nil
	defaultTextStyle = nil
	menuButtonVisible = false
	closeAllModals()
	// true, not false: the tag coroutine may currently be blocked inside a
	// TextObject's y.Until(false, func() bool { return isWait }) — see
	// execItem in macro.go — waiting on this exact flag, which is
	// completely independent of isJump/[s]'s isJumped. Leaving it false
	// here (the old behavior) meant that if the source screen happened to
	// be mid-dialogue rather than resting at [s], the coroutine stayed
	// stuck there forever: isJump never gets a chance to be read until
	// whatever currently-blocked handler's own predicate resolves, and
	// isWait=false never does on its own. Setting it true releases that
	// wait immediately (harmlessly — the destination's own first text line
	// resets isWait=false again the moment it actually starts revealing).
	isWait = true
	// oldTick reset alongside isWait, for the same reason applySaveData
	// does (tags_save.go): if the destination's first tag happens to be a
	// [p] (unusual for a title screen, but not impossible depending on
	// the scenario), its own y.Until(true, isTextEnded) could otherwise
	// spuriously already be satisfied — oldTick+3>=tick can be true purely
	// by coincidence (whatever oldTick last was, from a real click on the
	// previous screen, ending up within 3 ticks of the current one) even
	// though isWait being true here is forced, not a sign the player
	// actually clicked past this line.
	oldTick = tick - 4
	isSkip = false
	isAuto = false
	jumpIndex = 0
	isJump = true
}

// confirmGoToTitle opens real Tyrano's own "タイトルに戻ります。よろしい
// ですか？" confirmation before actually calling goToTitle — role="title"
// and the quick-menu's "BACK TO TITLE" item both go through this instead of
// calling goToTitle directly, so a stray click can't discard the player's
// place in the story.
func confirmGoToTitle(r *Renderer) {
	activeDialog = &dialogState{
		Text:      "タイトルに戻ります。よろしいですか？",
		OKLabel:   dialogOKLabel,
		NGLabel:   dialogNGLabel,
		OnConfirm: func(r *Renderer) { r.goToTitle() },
	}
}

// resolveButtonDialog finishes a button-triggered confirm dialog (one with
// OnConfirm set — see confirmGoToTitle) once the user has picked OK or NG:
// runs OnConfirm on OK, then clears activeDialog either way. A [dialog]
// *tag*'s dialog never sets OnConfirm and resolves itself inside
// handleDialog via its own y.Until, so this is a no-op for those and safe
// to call unconditionally every frame activeDialog is non-nil.
func resolveButtonDialog(r *Renderer) {
	if activeDialog == nil || activeDialog.Result == 0 || activeDialog.OnConfirm == nil {
		return
	}
	if activeDialog.Result == 1 {
		activeDialog.OnConfirm(r)
	}
	activeDialog = nil
}
