//go:build windows

package e2e

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebinovel/kag3/e2e/driver"
	"github.com/ebinovel/kag3/e2e/helpers"
)

// TestManualNewGameWalkthrough is a throwaway manual-verification script
// for the new original "たそがれ図書室" example: launches the game and
// walks all the way through the opening, the choice branch (rooftop
// route), the ending, back to title, and into the demo hub, screenshotting
// each stop.
func TestManualNewGameWalkthrough(t *testing.T) {
	outDir := os.Getenv("KAG3_MANUAL_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("KAG3_MANUAL_SCREENSHOT_DIR not set")
	}

	exePath, err := filepath.Abs(helpers.ExamplePath)
	if err != nil {
		t.Fatalf("resolving example path: %v", err)
	}
	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(), "KAG3_SAVE_DIR="+saveDir, "KAG3_E2E_FAST=1")
	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	shot := func(name string) {
		time.Sleep(500 * time.Millisecond)
		img, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}
	press := func(n int) {
		for i := 0; i < n; i++ {
			sess.KeyPress("Enter")
			time.Sleep(600 * time.Millisecond)
		}
	}

	time.Sleep(5 * time.Second)

	if err := helpers.ClickTitleStart(sess); err != nil {
		t.Fatalf("click start: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// scene1.ks's 20 [p]s + scene2.ks's own 4, plus one more press past the
	// last [p] to actually reveal the [glink] choice (each press only
	// reveals the *next* wait's text — moving past the final one to a
	// non-blocking [glink] takes an extra press).
	press(25)
	shot("04_choice.png")

	// glink "行く" (scene2.ks: x=540 width=750 y=300 at 1920x1080 scale),
	// center (915,390) — ×1.5 from the original 1280x720-scale measured
	// center (610,260). Retry a few times — clicks in this environment
	// occasionally don't register (same class of flakiness as advanceOne's
	// Enter retries elsewhere in this package).
	clicked := false
	for attempt := 0; attempt < 5 && !clicked; attempt++ {
		before, _ := sess.Screenshot()
		if err := helpers.ClickLogical(sess, 915, 390); err != nil {
			t.Fatalf("click glink go (attempt %d): %v", attempt, err)
		}
		time.Sleep(800 * time.Millisecond)
		after, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot after click (attempt %d): %v", attempt, err)
		}
		if !helpers.RegionsEqual(before, after) {
			clicked = true
		}
	}
	if !clicked {
		t.Fatal("glink click did not change the screen after 5 attempts")
	}
	shot("05_after_choice.png")

	// scene3a.ks has 12 [p]s; one more press past the 12th is needed to
	// actually fire its @jump into ending_a.ks (each press only reveals
	// the *next* wait's text — moving past the final one takes an extra
	// press, same off-by-one as the glink choice above).
	press(13)
	shot("06_rooftop_progress.png")

	// ending_a.ks has 5 [p]s of its own; likewise one more press past the
	// 5th is needed to move past "- おしまい -" and reveal its glink.
	press(6)
	shot("07_ending.png")

	// ending_a's "タイトルへ戻る" glink (x=540 width=750 y=750 at 1920x1080 scale)
	clicked = false
	for attempt := 0; attempt < 5 && !clicked; attempt++ {
		before, _ := sess.Screenshot()
		if err := helpers.ClickLogical(sess, 540+750/2, 750+45); err != nil {
			t.Fatalf("click ending totitle (attempt %d): %v", attempt, err)
		}
		time.Sleep(800 * time.Millisecond)
		after, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot after click (attempt %d): %v", attempt, err)
		}
		if !helpers.RegionsEqual(before, after) {
			clicked = true
		}
	}
	if !clicked {
		t.Fatal("ending totitle glink click did not change the screen after 5 attempts")
	}
	time.Sleep(1500 * time.Millisecond)
	shot("08_back_to_title.png")

	// title.ks's DEMO button (x=135 y=450, matches button_start's size 360x74)
	if err := helpers.ClickLogical(sess, 203+180, 675+37); err != nil {
		t.Fatalf("click demo button: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	shot("09_hub.png")
}

// clickGlinkRetry clicks a glink at logical (lx,ly) and retries up to 5
// times if the screen doesn't visibly change — same flakiness class as
// advanceOne's Enter retries elsewhere in this package.
func clickGlinkRetry(t *testing.T, sess *driver.Session, lx, ly int, what string) {
	t.Helper()
	for attempt := 0; attempt < 5; attempt++ {
		before, _ := sess.Screenshot()
		if err := helpers.ClickLogical(sess, lx, ly); err != nil {
			t.Fatalf("click %s (attempt %d): %v", what, attempt, err)
		}
		time.Sleep(800 * time.Millisecond)
		after, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot after clicking %s (attempt %d): %v", what, attempt, err)
		}
		if !helpers.RegionsEqual(before, after) {
			return
		}
	}
	t.Fatalf("click %s did not change the screen after 5 attempts", what)
}

// TestManualRouteB walks the "今日はやめておく" (go_home) branch: scene3b
// (alone route) into ending_b, then back to title. Companion to
// TestManualNewGameWalkthrough, which only covers the go_rooftop branch.
func TestManualRouteB(t *testing.T) {
	outDir := os.Getenv("KAG3_MANUAL_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("KAG3_MANUAL_SCREENSHOT_DIR not set")
	}
	exePath, err := filepath.Abs(helpers.ExamplePath)
	if err != nil {
		t.Fatalf("resolving example path: %v", err)
	}
	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(), "KAG3_SAVE_DIR="+saveDir, "KAG3_E2E_FAST=1")
	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	shot := func(name string) {
		time.Sleep(500 * time.Millisecond)
		img, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}
	press := func(n int) {
		for i := 0; i < n; i++ {
			sess.KeyPress("Enter")
			time.Sleep(600 * time.Millisecond)
		}
	}

	time.Sleep(5 * time.Second)

	if err := helpers.ClickTitleStart(sess); err != nil {
		t.Fatalf("click start: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	press(25)
	// glink "今日はやめておく" (x=540 y=450 width=750 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 915, 495, "glink go_home")
	shot("b01_after_choice.png")

	// scene3b.ks has 8 [p]s; one more past the 8th fires its @jump into
	// ending_b.ks.
	press(9)
	shot("b02_corridor.png")

	// ending_b.ks has 5 [p]s; one more past the 5th reveals its glink.
	press(6)
	shot("b03_ending.png")

	clickGlinkRetry(t, sess, 540+750/2, 750+45, "ending_b totitle")
	time.Sleep(1500 * time.Millisecond)
	shot("b04_back_to_title.png")
}

// TestManualHubDemos walks two of the demo hub's six items (text decoration
// and choice/link) round-trip to hub and back to title, as a spot check
// that hub navigation and the demo scenarios themselves work — not
// exhaustive over all six items.
func TestManualHubDemos(t *testing.T) {
	outDir := os.Getenv("KAG3_MANUAL_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("KAG3_MANUAL_SCREENSHOT_DIR not set")
	}
	exePath, err := filepath.Abs(helpers.ExamplePath)
	if err != nil {
		t.Fatalf("resolving example path: %v", err)
	}
	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(), "KAG3_SAVE_DIR="+saveDir, "KAG3_E2E_FAST=1")
	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	shot := func(name string) {
		time.Sleep(500 * time.Millisecond)
		img, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}
	press := func(n int) {
		for i := 0; i < n; i++ {
			sess.KeyPress("Enter")
			time.Sleep(600 * time.Millisecond)
		}
	}

	time.Sleep(5 * time.Second)

	// title.ks's DEMO button -> hub.ks
	if err := helpers.ClickLogical(sess, 203+180, 675+37); err != nil {
		t.Fatalf("click demo button: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// hub.ks has 1 [p] before its 7 glinks.
	press(2)
	shot("h01_hub.png")

	// "① テキスト装飾" glink (x=240 y=180 width=690 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 240+345, 180+30, "hub to_text")
	shot("h02z_text_demo_first_line.png")
	press(1)
	shot("h02y_text_demo_line2.png")
	press(1)
	shot("h02x_text_demo_line3_size40.png")
	press(1)
	shot("h02w_text_demo_line4_pink.png")
	press(1)
	shot("h02a_text_demo_last_line.png")
	press(1)
	shot("h02_text_demo.png")
	// "ハブへ戻る" glink (x=660 y=840 width=600 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 660+300, 840+30, "demo_text to_hub")
	time.Sleep(1000 * time.Millisecond)

	// back at hub — reveal glinks again.
	press(2)
	shot("h03_hub_again.png")

	// "⑤ 選択肢分岐" glink (x=990 y=180 width=690 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 990+345, 180+30, "hub to_choice")
	// demo_choice.ks has 2 [p]s before its "犬派"/"猫派" glinks.
	press(3)
	shot("h04_choice_demo.png")
	// "犬派" glink (x=540 y=225 width=750 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 540+375, 225+30, "demo_choice dog")
	shot("h05_dog_chosen.png")
	// One more press to move past "犬派を選びました。" and reveal the
	// [link] demo's first line ("［link］は…") — just confirms this line
	// (previously an unescaped-tag crash risk, now fixed) renders fine.
	press(1)
	shot("h06_choice_after_dog.png")
}

// TestManualHubRemainingDemos covers the other four hub items (bg, chara,
// audio, save) not visited by TestManualHubDemos — mainly a regression
// check for the "tag name mentioned in prose gets parsed as a real,
// attribute-less tag call" bug found via TestManualHubDemos (e.g.
// [chara_show]/[playbgm]/[bg method=...] written directly in dialogue text
// crashed the renderer with "ebiten: width at NewImage must be positive but
// 0" — fixed by switching those mentions to fullwidth ［ ］ brackets so
// they're plain text instead of real tag invocations).
func TestManualHubRemainingDemos(t *testing.T) {
	outDir := os.Getenv("KAG3_MANUAL_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("KAG3_MANUAL_SCREENSHOT_DIR not set")
	}
	exePath, err := filepath.Abs(helpers.ExamplePath)
	if err != nil {
		t.Fatalf("resolving example path: %v", err)
	}
	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(), "KAG3_SAVE_DIR="+saveDir, "KAG3_E2E_FAST=1")
	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	shot := func(name string) {
		time.Sleep(500 * time.Millisecond)
		img, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}
	press := func(n int) {
		for i := 0; i < n; i++ {
			sess.KeyPress("Enter")
			// 1000ms, not the usual 600-700ms: demo_chara.ks has
			// chara_move transitions with time=800, and a press sent
			// before a transition finishes doesn't advance anything (the
			// coroutine isn't at a wait point yet), silently wasting that
			// press and desyncing the count.
			time.Sleep(1000 * time.Millisecond)
		}
	}

	time.Sleep(5 * time.Second)

	// title.ks's DEMO button -> hub.ks
	if err := helpers.ClickLogical(sess, 203+180, 675+37); err != nil {
		t.Fatalf("click demo button: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	press(2)

	// "② キャラクター表示・表情・移動" glink (x=240 y=285 width=690 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 240+345, 285+30, "hub to_chara")
	// demo_chara.ks has 12 [p]s before its "ハブへ戻る" glink.
	press(13)
	shot("r01_chara_demo.png")
	// "ハブへ戻る" glink (x=660 y=825 width=600 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 660+300, 825+30, "demo_chara to_hub")
	time.Sleep(1000 * time.Millisecond)
	press(2)

	// "③ 背景・画面転換" glink (x=240 y=390 width=690 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 240+345, 390+30, "hub to_bg")
	// demo_bg.ks has 7 [p]s before its "ハブへ戻る" glink.
	press(8)
	shot("r02_bg_demo.png")
	// "ハブへ戻る" glink (x=660 y=915 width=600 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 660+300, 915+30, "demo_bg to_hub")
	time.Sleep(1000 * time.Millisecond)
	press(2)

	// "④ 音声(BGM/効果音)" glink (x=240 y=495 width=690 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 240+345, 495+30, "hub to_audio")
	// demo_audio.ks has 7 [p]s before its "ハブへ戻る" glink.
	press(8)
	shot("r03_audio_demo.png")
	// "ハブへ戻る" glink (x=660 y=915 width=600 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 660+300, 915+30, "demo_audio to_hub")
	time.Sleep(1000 * time.Millisecond)
	press(2)

	// "⑥ セーブ・ロード" glink (x=990 y=285 width=690 at 1920x1080 scale)
	clickGlinkRetry(t, sess, 990+345, 285+30, "hub to_save")
	// demo_save.ks: exact press count to its glink is uncertain (it has a
	// real mid-file [s] wait, unlike the other demos) — press generously
	// and screenshot to see exactly where we land.
	press(10)
	shot("r04_save_demo.png")
}

// TestManualConfigAndMenu is a regression check for the config screen,
// whose TyranoScript-sourced art (example/resources/images/config/,
// resources/system/images/) was replaced with original placeholders. Also
// exercises config.ks's *load_img (set1.png/set2.png), which referenced
// files that never actually existed in this project before today. The
// quick menu / save-load slot picker this test used to also cover was
// dropped when the corner menu button (@showmenubutton) was removed from
// demo_save.ks — see the comment at the end of this test.
func TestManualConfigAndMenu(t *testing.T) {
	outDir := os.Getenv("KAG3_MANUAL_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("KAG3_MANUAL_SCREENSHOT_DIR not set")
	}
	exePath, err := filepath.Abs(helpers.ExamplePath)
	if err != nil {
		t.Fatalf("resolving example path: %v", err)
	}
	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(), "KAG3_SAVE_DIR="+saveDir, "KAG3_E2E_FAST=1")
	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	shot := func(name string) {
		time.Sleep(500 * time.Millisecond)
		img, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}
	press := func(n int) {
		for i := 0; i < n; i++ {
			sess.KeyPress("Enter")
			time.Sleep(700 * time.Millisecond)
		}
	}

	time.Sleep(5 * time.Second)

	// title.ks's CONFIG button (role="sleepgame" storage="config.ks",
	// x=203 y=840 width=360 height=74 at 1920x1080 scale).
	if err := helpers.ClickLogical(sess, 203+180, 840+37); err != nil {
		t.Fatalf("click config button: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	shot("c01_config_screen.png")

	// config.ks's own "Back" button (c_btn_back.png, x=1160 y=20, 100x100)
	// — config.ks itself was not touched by the 1920x1080 redesign (out of
	// scope, see the design plan), so this literal coordinate is unchanged.
	clickGlinkRetry(t, sess, 1160+50, 20+50, "config back")
	time.Sleep(1500 * time.Millisecond)
	shot("c02_back_to_title.png")

	// title.ks's DEMO button -> hub.ks -> "⑥ セーブ・ロード" -> demo_save.ks,
	// which calls @showmenubutton right away.
	if err := helpers.ClickLogical(sess, 203+180, 675+37); err != nil {
		t.Fatalf("click demo button: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	press(2)
	clickGlinkRetry(t, sess, 990+345, 285+30, "hub to_save")
	time.Sleep(1500 * time.Millisecond)
	shot("c03_demo_save.png")

	// The corner quick-menu button (button_menu.png / @showmenubutton) no
	// longer appears here — demo_save.ks dropped its @showmenubutton call
	// once the redesigned message window's own operation row covered
	// SAVE/LOAD/Title (see scene1.ks/demo_save.ks's comments and
	// helpers.OpRowSaveX/Y in nav.go). demo_save.ks's own [position] wasn't
	// updated to the 1920x1080 redesign (out of scope), so the operation
	// row's exact hit-box here differs from scene1.ks's OpRowSaveX/Y —
	// re-derive against demo_save.ks's own [position] if this flow needs
	// covering again; skipped for now since TestQuickSaveThenLoadRestoresSceneText
	// (flows_test.go) already exercises the same save/load-slot-picker path
	// end to end against scene1.ks.
}

// TestManualTitleLoadButton spot-checks title.ks's LOAD button (role="load"),
// a separate entry point into the slot picker from the quick menu's SAVE
// button already covered by TestManualConfigAndMenu.
func TestManualTitleLoadButton(t *testing.T) {
	outDir := os.Getenv("KAG3_MANUAL_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("KAG3_MANUAL_SCREENSHOT_DIR not set")
	}
	exePath, err := filepath.Abs(helpers.ExamplePath)
	if err != nil {
		t.Fatalf("resolving example path: %v", err)
	}
	saveDir := filepath.Join(t.TempDir(), "saves")
	env := append(os.Environ(), "KAG3_SAVE_DIR="+saveDir, "KAG3_E2E_FAST=1")
	sess, err := driver.NewSession(exePath, env)
	if err != nil {
		t.Fatalf("launching game: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	shot := func(name string) {
		time.Sleep(500 * time.Millisecond)
		img, err := sess.Screenshot()
		if err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}

	time.Sleep(5 * time.Second)

	if err := helpers.ClickTitleLoad(sess); err != nil {
		t.Fatalf("click title load: %v", err)
	}
	shot("d01_title_load_screen.png")
}
