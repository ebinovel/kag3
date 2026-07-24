package ebitengine

import (
	"image/color"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func init() {
	register("current", handleCurrent)
	register("er", handleCM)
	register("ct", handleCT)
	register("deffont", handleDefFont)
	register("message_config", handleMessageConfig)
	register("delay", handleDelay)
	register("resetdelay", handleResetDelay)
	register("configdelay", handleConfigDelay)
	register("nowait", handleNowait)
	register("endnowait", handleEndNowait)
	register("skipstart", handleSkipStart)
	register("skipstop", handleSkipStop)
	register("cancelskip", handleSkipStop)
	register("autostart", handleAutoStart)
	register("autostop", handleAutoStop)
	register("autoconfig", handleAutoConfig)
	register("position_filter", handlePositionFilter)
	register("nolog", handleNoLog)
	register("endnolog", handleEndNoLog)
	register("pushlog", handlePushLog)
	register("fuki_start", handleFukiStart)
	register("fuki_stop", handleFukiStop)
	register("fuki_chara", handleFukiChara)
	register("mtext", handleMText)
	register("mark", handleFont)
	register("endmark", handleResetFont)
	register("graph", handleGraph)
}

// current is a simple state tracker for [current layer=...] — kag3 only
// ever had a single message box (textPosition/r.texts), not real Tyrano's
// multiple named message layers, so this doesn't yet gate anything; it's
// tracked honestly rather than silently dropped, in case later tags want
// to branch on it.
var currentMessageLayer string

func handleCurrent(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["layer"]; ok {
		currentMessageLayer = v
	}
	return nil
}

// handleCT implements [ct]: a fuller reset than [cm]/[er] — clears text
// *and* reverts textPosition's layout (margins, frame, colors, size) to a
// blank slate, not just the displayed characters.
func handleCT(ctx *tagCtx) error {
	r := ctx.r
	recordBacklog(r)
	r.texts = make(map[int][]Text)
	isWait = false
	charaName = ""
	pendingRuby = ""
	visible := textPosition.Visible
	textPosition = &kag3.TextPosition{Visible: visible}
	return nil
}

// defaultTextStyle is [deffont]'s configured baseline, restored by
// [resetfont] (tags_text.go) instead of always clearing to nil.
var defaultTextStyle *kag3.TextStyle

var (
	textPosition   *kag3.TextPosition
	textStyle      *kag3.TextStyle
	beforeTextSize float64
	textGlyphs     []text.Glyph
	isSkip, isAuto bool
	autoStartT     int
)

func init() {
	textPosition = &kag3.TextPosition{}
}

func handleDefFont(ctx *tagCtx) error {
	saved := textStyle
	textStyle = nil
	if err := applyFontAttrs(ctx); err != nil {
		textStyle = saved
		return err
	}
	defaultTextStyle = textStyle
	textStyle = saved
	return nil
}

// handleMessageConfig forwards the subset of [message_config]'s attributes
// that overlap with textPosition's existing fields (visible/opacity);
// Tyrano's real message_config bundles more window-chrome options that
// kag3's simplified single message box doesn't have equivalents for.
func handleMessageConfig(ctx *tagCtx) error {
	if v, ok := ctx.tag.Pm["visible"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		textPosition.Visible = b
	}
	if v, ok := ctx.tag.Pm["opacity"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		textPosition.Opacity = n
	}
	return nil
}

// textSpeedMs/defaultTextSpeedMs drive Draw()'s character-reveal rate via
// ticksPerChar. 83ms preserves the previous hardcoded "5 ticks at 60 TPS".
var (
	textSpeedMs        = 83
	defaultTextSpeedMs = 83
	textNoWait         bool
)

// KAG3_E2E_FAST forces textNoWait on at startup — an external-process E2E
// harness (see e2e/) has no way to send a [nowait] tag or flip the
// in-package textNoWait var directly, and glyph-by-glyph text reveal is by
// far the slowest thing such a harness waits on. [nowait]/[endnowait] still
// work normally afterward; this only changes the starting value.
func init() {
	if os.Getenv("KAG3_E2E_FAST") != "" {
		textNoWait = true
	}
}

func ticksPerChar() int {
	tpc := textSpeedMs * ebiten.TPS() / 1000
	if tpc < 1 {
		tpc = 1
	}
	return tpc
}

func handleDelay(ctx *tagCtx) error {
	v, err := strconv.Atoi(ctx.tag.Pm["speed"])
	if err != nil {
		return err
	}
	textSpeedMs = v
	return nil
}

func handleResetDelay(ctx *tagCtx) error {
	textSpeedMs = defaultTextSpeedMs
	return nil
}

func handleConfigDelay(ctx *tagCtx) error {
	v, err := strconv.Atoi(ctx.tag.Pm["speed"])
	if err != nil {
		return err
	}
	defaultTextSpeedMs = v
	textSpeedMs = v
	return nil
}

func handleNowait(ctx *tagCtx) error {
	textNoWait = true
	return nil
}

func handleEndNowait(ctx *tagCtx) error {
	textNoWait = false
	return nil
}

// handleSkipStart/handleSkipStop mirror the existing button role="skip"
// toggle (renderer.go's Update()), just triggerable from a script tag
// instead of only a click.
func handleSkipStart(ctx *tagCtx) error {
	isSkip = true
	isAuto = false
	return nil
}

func handleSkipStop(ctx *tagCtx) error {
	isSkip = false
	return nil
}

func handleAutoStart(ctx *tagCtx) error {
	isAuto = true
	isSkip = false
	autoStartT = t
	return nil
}

func handleAutoStop(ctx *tagCtx) error {
	isAuto = false
	return nil
}

// autoWaitMs is how long auto-mode waits after a line finishes revealing
// before advancing — Update() reads it instead of a hardcoded 3000ms.
var autoWaitMs = 3000

func handleAutoConfig(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["speed"]
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	autoWaitMs = n
	return nil
}

// handlePositionFilter sets textPosition.FilterColor, which Draw uses in
// place of the hardcoded black/50%-alpha fill behind the message window
// when set.
func handlePositionFilter(ctx *tagCtx) error {
	v, ok := ctx.tag.Pm["color"]
	if !ok {
		textPosition.FilterColor = nil
		return nil
	}
	r, g, b, err := parseColor(v)
	if err != nil {
		return err
	}
	alpha := 128
	if a, ok := ctx.tag.Pm["opacity"]; ok {
		n, err := strconv.Atoi(a)
		if err != nil {
			return err
		}
		alpha = n
	}
	textPosition.FilterColor = &color.RGBA{uint8(r), uint8(g), uint8(b), uint8(alpha)}
	return nil
}

// backlog is a plain append-only transcript of dialogue chunks, recorded
// by recordBacklog whenever [p]/[cm]/[er]/[ct] clears the text buffer.
var (
	backlog       []string
	backlogPaused bool
)

func recordBacklog(r *Renderer) {
	if backlogPaused {
		return
	}
	if s := currentMessageText(r); s != "" {
		backlog = append(backlog, s)
	}
}

// currentMessageText concatenates every segment of r.texts, in line order —
// whatever's currently on screen in the message window, revealed or not.
// Shared by recordBacklog and buildSaveData (tags_save.go), which captures
// it as the save slot's preview text (see the DATA SAVE/LOAD screen).
func currentMessageText(r *Renderer) string {
	lineNums := make([]int, 0, len(r.texts))
	for k := range r.texts {
		lineNums = append(lineNums, k)
	}
	sort.Ints(lineNums)
	var sb strings.Builder
	for _, ln := range lineNums {
		for _, seg := range r.texts[ln] {
			sb.WriteString(seg.Text)
		}
	}
	return sb.String()
}

func handleNoLog(ctx *tagCtx) error {
	backlogPaused = true
	return nil
}

func handleEndNoLog(ctx *tagCtx) error {
	backlogPaused = false
	return nil
}

// handlePushLog always appends regardless of backlogPaused — it's an
// explicit, deliberate log entry, not automatic dialogue recording.
func handlePushLog(ctx *tagCtx) error {
	text := ctx.tag.Pm["text"]
	if text == "" {
		return nil
	}
	backlog = append(backlog, text)
	return nil
}

// fukiEnabled/fukiOffsets/applyFukiPosition are a simplified stand-in for
// Tyrano's speech-bubble mode: no bubble-shaped graphic or pointer tail,
// but the message window genuinely repositions to hover near whichever
// character is currently speaking (per charaName), offset by whatever
// [fuki_chara] registered for them.
var (
	fukiEnabled bool
	fukiOffsets = map[string]struct{ Left, Top int }{}
)

func handleFukiStart(ctx *tagCtx) error {
	fukiEnabled = true
	return nil
}

func handleFukiStop(ctx *tagCtx) error {
	fukiEnabled = false
	return nil
}

func handleFukiChara(ctx *tagCtx) error {
	object := ctx.tag
	name := object.Pm["name"]
	left, err := strconv.Atoi(object.Pm["left"])
	if err != nil {
		return err
	}
	top, err := strconv.Atoi(object.Pm["top"])
	if err != nil {
		return err
	}
	fukiOffsets[name] = struct{ Left, Top int }{left, top}
	return nil
}

func applyFukiPosition() {
	if !fukiEnabled || charaName == "" {
		return
	}
	c := findViewChara(charaName)
	if c == nil {
		return
	}
	off, ok := fukiOffsets[charaName]
	if !ok {
		return
	}
	textPosition.Left = c.Left + off.Left
	textPosition.Top = c.Top + off.Top
}

// handleMText implements [mtext name= text= x= y= ...]: kag3 doesn't have
// a separate "effect text" rendering path, so this reuses [ptext]'s
// mechanism directly — a real, visible, named text placement, just
// without an animated entrance (real Tyrano's mtext has one; that would
// need extending PText with its own opacity tween).
func handleMText(ctx *tagCtx) error {
	return handlePText(ctx)
}

// handleGraph implements [graph storage= x= y= ...] (inline image display):
// kag3's text layout doesn't support true inline image glyphs mixed into
// character-by-character reveal, so this places a real image via the same
// mechanism as [image] instead of literally embedding it in the text flow.
func handleGraph(ctx *tagCtx) error {
	return handleImage(ctx)
}
