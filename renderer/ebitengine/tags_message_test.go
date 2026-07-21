package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

func TestHandleCurrentTracksLayer(t *testing.T) {
	currentMessageLayer = ""
	tag := kag3.TagObject{Name: "current", Pm: map[string]string{"layer": "message1"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if currentMessageLayer != "message1" {
		t.Errorf("currentMessageLayer = %q, want %q", currentMessageLayer, "message1")
	}
}

func TestHandleCTResetsTextPositionButKeepsVisibility(t *testing.T) {
	r := newTestRenderer()
	r.texts = map[int][]Text{0: {{Text: "hi"}}}
	textPosition = &kag3.TextPosition{Visible: true, MarginLeft: 99, Left: 42}
	charaName = "akane"

	tag := kag3.TagObject{Name: "ct"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(r.texts) != 0 {
		t.Errorf("r.texts = %+v, want empty", r.texts)
	}
	if !textPosition.Visible {
		t.Error("expected Visible to survive the reset")
	}
	if textPosition.MarginLeft != 0 || textPosition.Left != 0 {
		t.Errorf("textPosition = %+v, want layout fields reset to zero", textPosition)
	}
	if charaName != "" {
		t.Errorf("charaName = %q, want cleared", charaName)
	}
}

func TestHandleErSameAsCM(t *testing.T) {
	r := newTestRenderer()
	r.texts = map[int][]Text{0: {{Text: "hi"}}}
	tag := kag3.TagObject{Name: "er"}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(r.texts) != 0 {
		t.Errorf("r.texts = %+v, want empty after er", r.texts)
	}
}

func TestDeffontThenResetfontRestoresConfiguredDefault(t *testing.T) {
	defaultTextStyle = nil
	textStyle = nil

	deffont := kag3.TagObject{Name: "deffont", Pm: map[string]string{"color": "0xFF0000", "size": "20"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), deffont, &i, 0); err != nil {
		t.Fatalf("deffont dispatch error: %v", err)
	}
	if defaultTextStyle == nil || defaultTextStyle.Size != 20 {
		t.Fatalf("defaultTextStyle = %+v, want Size 20", defaultTextStyle)
	}
	// deffont must not itself change the *current* style.
	if textStyle != nil {
		t.Errorf("textStyle = %+v, want unchanged (nil) right after deffont", textStyle)
	}

	// font changes the current style away from the default...
	font := kag3.TagObject{Name: "font", Pm: map[string]string{"size": "40"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), font, &i, 0); err != nil {
		t.Fatalf("font dispatch error: %v", err)
	}
	if textStyle.Size != 40 {
		t.Fatalf("textStyle.Size = %d, want 40", textStyle.Size)
	}

	// ...and resetfont must restore deffont's configured default, not nil.
	resetfont := kag3.TagObject{Name: "resetfont"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), resetfont, &i, 0); err != nil {
		t.Fatalf("resetfont dispatch error: %v", err)
	}
	if textStyle == nil || textStyle.Size != 20 {
		t.Errorf("textStyle after resetfont = %+v, want the deffont default (Size 20)", textStyle)
	}
}

func TestDelayResetDelayConfigDelay(t *testing.T) {
	textSpeedMs, defaultTextSpeedMs = 83, 83

	delay := kag3.TagObject{Name: "delay", Pm: map[string]string{"speed": "10"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), delay, &i, 0); err != nil {
		t.Fatalf("delay dispatch error: %v", err)
	}
	if textSpeedMs != 10 {
		t.Errorf("textSpeedMs = %d, want 10", textSpeedMs)
	}
	if defaultTextSpeedMs != 83 {
		t.Errorf("defaultTextSpeedMs = %d, want unchanged 83", defaultTextSpeedMs)
	}

	resetdelay := kag3.TagObject{Name: "resetdelay"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), resetdelay, &i, 0); err != nil {
		t.Fatalf("resetdelay dispatch error: %v", err)
	}
	if textSpeedMs != 83 {
		t.Errorf("textSpeedMs after resetdelay = %d, want back to default 83", textSpeedMs)
	}

	configdelay := kag3.TagObject{Name: "configdelay", Pm: map[string]string{"speed": "30"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), configdelay, &i, 0); err != nil {
		t.Fatalf("configdelay dispatch error: %v", err)
	}
	if textSpeedMs != 30 || defaultTextSpeedMs != 30 {
		t.Errorf("textSpeedMs=%d defaultTextSpeedMs=%d, want both 30 after configdelay", textSpeedMs, defaultTextSpeedMs)
	}
}

func TestTicksPerCharNeverZero(t *testing.T) {
	textSpeedMs = 0
	if got := ticksPerChar(); got < 1 {
		t.Errorf("ticksPerChar() = %d, want >= 1 even for speed=0", got)
	}
}

func TestNowaitEndNowaitToggle(t *testing.T) {
	textNoWait = false
	tag := kag3.TagObject{Name: "nowait"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("nowait dispatch error: %v", err)
	}
	if !textNoWait {
		t.Error("expected textNoWait = true after nowait")
	}
	tag = kag3.TagObject{Name: "endnowait"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("endnowait dispatch error: %v", err)
	}
	if textNoWait {
		t.Error("expected textNoWait = false after endnowait")
	}
}

func TestSkipStartStopCancel(t *testing.T) {
	isSkip, isAuto = false, true
	tag := kag3.TagObject{Name: "skipstart"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("skipstart dispatch error: %v", err)
	}
	if !isSkip || isAuto {
		t.Errorf("isSkip=%v isAuto=%v, want isSkip=true isAuto=false", isSkip, isAuto)
	}
	tag = kag3.TagObject{Name: "cancelskip"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("cancelskip dispatch error: %v", err)
	}
	if isSkip {
		t.Error("expected isSkip = false after cancelskip")
	}
}

func TestAutoStartStopConfig(t *testing.T) {
	isAuto, isSkip = false, true
	autoWaitMs = 3000
	tag := kag3.TagObject{Name: "autostart"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("autostart dispatch error: %v", err)
	}
	if !isAuto || isSkip {
		t.Errorf("isAuto=%v isSkip=%v, want isAuto=true isSkip=false", isAuto, isSkip)
	}

	cfg := kag3.TagObject{Name: "autoconfig", Pm: map[string]string{"speed": "1500"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), cfg, &i, 0); err != nil {
		t.Fatalf("autoconfig dispatch error: %v", err)
	}
	if autoWaitMs != 1500 {
		t.Errorf("autoWaitMs = %d, want 1500", autoWaitMs)
	}

	tag = kag3.TagObject{Name: "autostop"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("autostop dispatch error: %v", err)
	}
	if isAuto {
		t.Error("expected isAuto = false after autostop")
	}
}

func TestPositionFilterSetsAndClearsColor(t *testing.T) {
	textPosition = &kag3.TextPosition{}
	tag := kag3.TagObject{Name: "position_filter", Pm: map[string]string{"color": "0x123456", "opacity": "64"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if textPosition.FilterColor == nil {
		t.Fatal("expected FilterColor to be set")
	}
	if textPosition.FilterColor.A != 64 {
		t.Errorf("FilterColor.A = %d, want 64", textPosition.FilterColor.A)
	}

	tag = kag3.TagObject{Name: "position_filter", Pm: map[string]string{}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if textPosition.FilterColor != nil {
		t.Error("expected FilterColor to be cleared when color= is omitted")
	}
}

func TestBacklogRecordingAndPause(t *testing.T) {
	backlog = nil
	backlogPaused = false
	r := newTestRenderer()
	r.texts = map[int][]Text{0: {{Text: "hello "}}, 1: {{Text: "world"}}}

	pTag := kag3.TagObject{Name: "p"}
	i := 0
	if err := dispatchTag(r, fakeYield(), pTag, &i, 0); err != nil {
		t.Fatalf("p dispatch error: %v", err)
	}
	if len(backlog) != 1 || backlog[0] != "hello world" {
		t.Fatalf("backlog = %+v, want [\"hello world\"]", backlog)
	}

	nolog := kag3.TagObject{Name: "nolog"}
	if err := dispatchTag(r, fakeYield(), nolog, &i, 0); err != nil {
		t.Fatalf("nolog dispatch error: %v", err)
	}
	r.texts = map[int][]Text{0: {{Text: "should not be logged"}}}
	if err := dispatchTag(r, fakeYield(), pTag, &i, 0); err != nil {
		t.Fatalf("p dispatch error: %v", err)
	}
	if len(backlog) != 1 {
		t.Errorf("backlog = %+v, want still just 1 entry while paused", backlog)
	}

	endnolog := kag3.TagObject{Name: "endnolog"}
	if err := dispatchTag(r, fakeYield(), endnolog, &i, 0); err != nil {
		t.Fatalf("endnolog dispatch error: %v", err)
	}

	push := kag3.TagObject{Name: "pushlog", Pm: map[string]string{"text": "manual entry"}}
	backlogPaused = true // pushlog must ignore this
	if err := dispatchTag(r, fakeYield(), push, &i, 0); err != nil {
		t.Fatalf("pushlog dispatch error: %v", err)
	}
	if backlog[len(backlog)-1] != "manual entry" {
		t.Errorf("last backlog entry = %q, want %q (pushlog should ignore nolog)", backlog[len(backlog)-1], "manual entry")
	}
}

func TestFukiRepositionsMessageWindow(t *testing.T) {
	fukiEnabled = false
	fukiOffsets = map[string]struct{ Left, Top int }{}
	viewCharas = []*kag3.CharaShow{{Name: "akane", Left: 200, Top: 300}}
	charaName = "akane"
	textPosition = &kag3.TextPosition{}

	startTag := kag3.TagObject{Name: "fuki_start"}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), startTag, &i, 0); err != nil {
		t.Fatalf("fuki_start dispatch error: %v", err)
	}
	charaTag := kag3.TagObject{Name: "fuki_chara", Pm: map[string]string{"name": "akane", "left": "10", "top": "-50"}}
	if err := dispatchTag(newTestRenderer(), fakeYield(), charaTag, &i, 0); err != nil {
		t.Fatalf("fuki_chara dispatch error: %v", err)
	}

	applyFukiPosition()
	if textPosition.Left != 210 || textPosition.Top != 250 {
		t.Errorf("textPosition = {Left:%d Top:%d}, want {210 250}", textPosition.Left, textPosition.Top)
	}

	stopTag := kag3.TagObject{Name: "fuki_stop"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), stopTag, &i, 0); err != nil {
		t.Fatalf("fuki_stop dispatch error: %v", err)
	}
	textPosition.Left, textPosition.Top = 999, 999
	applyFukiPosition()
	if textPosition.Left != 999 || textPosition.Top != 999 {
		t.Error("expected applyFukiPosition to do nothing once fuki mode is stopped")
	}
}

func TestMTextCreatesPText(t *testing.T) {
	ptexts = map[string]*kag3.PText{}
	tag := kag3.TagObject{Name: "mtext", Pm: map[string]string{"name": "title", "text": "Chapter 1", "x": "10", "y": "20"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	pt, ok := ptexts["title"]
	if !ok || pt.Text != "Chapter 1" {
		t.Errorf("ptexts[title] = %+v, want Text \"Chapter 1\"", pt)
	}
}

func TestMarkEndmarkAliasFontResetfont(t *testing.T) {
	defaultTextStyle = nil
	textStyle = nil
	tag := kag3.TagObject{Name: "mark", Pm: map[string]string{"color": "0xFF0000"}}
	i := 0
	if err := dispatchTag(newTestRenderer(), fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("mark dispatch error: %v", err)
	}
	if textStyle == nil || textStyle.Color == nil {
		t.Fatal("expected mark to set a text color via the shared textStyle mechanism")
	}
	endTag := kag3.TagObject{Name: "endmark"}
	if err := dispatchTag(newTestRenderer(), fakeYield(), endTag, &i, 0); err != nil {
		t.Fatalf("endmark dispatch error: %v", err)
	}
	if textStyle != nil {
		t.Errorf("textStyle after endmark = %+v, want nil (no deffont configured)", textStyle)
	}
}

func TestGraphCreatesImage(t *testing.T) {
	imgs = nil
	r := newTestRendererWithImageFS(t, map[string][]byte{"icon.png": tinyPNG(t)})
	tag := kag3.TagObject{Name: "graph", Pm: map[string]string{"storage": "icon.png", "x": "5", "y": "6"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("dispatchTag error: %v", err)
	}
	if len(imgs) != 1 || imgs[0].X != 5 {
		t.Errorf("imgs = %+v, want one image at X=5", imgs)
	}
}
