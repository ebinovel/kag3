// genplaceholders.go generates placeholder art for the new original
// "たそがれ図書室" example scenario, so it's actually runnable/testable
// before real character art and backgrounds are supplied. Deliberately
// avoids importing hajimehoshi/ebiten (its init() connects to GLFW/the
// windowing system unconditionally, which panics in a headless context —
// see this repo's CI workflow discussion) — pure image/x/image only.
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const root = "example/game/resources"

// sysRoot is kag3 engine's own bundled default UI assets (repo root, not
// per-example) — the quick-menu/save-load-slot-picker chrome every kag3
// game gets unless it overrides these itself. Also TyranoScript sample art
// originally; see the "menu buttons/backgrounds" section in main below.
const sysRoot = "resources/system/images"

func main() {
	must(os.MkdirAll(filepath.Join(root, "images", "chara", "nagi"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "images", "title"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "bgms"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "ses"), 0o755))

	// Backgrounds (1280x720 JPEG) — solid color + a big centered label so
	// each is visually distinct at a glance during manual/E2E checks.
	bgs := []struct {
		name  string
		label string
		c     color.RGBA
	}{
		{"title.jpg", "TITLE", color.RGBA{0x4a, 0x35, 0x6b, 0xff}},
		{"library.jpg", "LIBRARY", color.RGBA{0x6b, 0x54, 0x3a, 0xff}},
		{"corridor.jpg", "CORRIDOR", color.RGBA{0x77, 0x7c, 0x82, 0xff}},
		{"rooftop_sunset.jpg", "ROOFTOP SUNSET", color.RGBA{0xd9, 0x6a, 0x2e, 0xff}},
		{"classroom.jpg", "CLASSROOM", color.RGBA{0x3d, 0x7a, 0x8c, 0xff}},
	}
	for _, b := range bgs {
		img := solidLabeled(1280, 720, b.c, color.White, b.label, 3)
		saveJPEG(filepath.Join(root, "images", b.name), img)
	}

	// Character standing art (400x600 transparent PNG) — one per expression,
	// a translucent silhouette + name/expression label.
	faces := []struct {
		file  string
		label string
		c     color.RGBA
	}{
		{"normal.png", "NAGI\nnormal", color.RGBA{0xb0, 0x8a, 0xc8, 0xd0}},
		{"smile.png", "NAGI\nsmile", color.RGBA{0xe8, 0xa9, 0xc0, 0xd0}},
		{"sad.png", "NAGI\nsad", color.RGBA{0x7a, 0x8a, 0xb0, 0xd0}},
		{"surprised.png", "NAGI\nsurprised", color.RGBA{0xe8, 0xd0, 0x7a, 0xd0}},
		{"shy.png", "NAGI\nshy", color.RGBA{0xf0, 0x9a, 0x9a, 0xd0}},
	}
	for _, f := range faces {
		img := charaSilhouette(400, 600, f.c, f.label)
		savePNG(filepath.Join(root, "images", "chara", "nagi", f.file), img)
	}

	// Title buttons (360x74 PNG, normal + hover each) — all four of
	// title.ks's buttons, replacing TyranoScript's bundled sample art.
	// basicfont is ASCII-only (see drawLabelCentered's doc comment), so
	// labels are romaji, not the in-script Japanese button text.
	titleButtons := []struct {
		file   string
		label  string
		normal color.RGBA
		hover  color.RGBA
	}{
		{"button_start", "START", color.RGBA{0xb0, 0x3a, 0x5a, 0xff}, color.RGBA{0xd8, 0x4e, 0x76, 0xff}},
		{"button_load", "LOAD", color.RGBA{0x3a, 0x8a, 0x5c, 0xff}, color.RGBA{0x4e, 0xae, 0x78, 0xff}},
		{"button_demo", "DEMO", color.RGBA{0x2e, 0x5c, 0x8a, 0xff}, color.RGBA{0x3d, 0x78, 0xb0, 0xff}},
		{"button_config", "CONFIG", color.RGBA{0x8a, 0x6a, 0x2e, 0xff}, color.RGBA{0xb0, 0x8a, 0x3d, 0xff}},
	}
	for _, b := range titleButtons {
		savePNG(filepath.Join(root, "images", "title", b.file+".png"), buttonImage(360, 74, b.normal, b.label))
		savePNG(filepath.Join(root, "images", "title", b.file+"2.png"), buttonImage(360, 74, b.hover, b.label))
	}

	// --- kag3 engine's built-in quick-menu / save-load slot picker chrome
	// (resources/system/images, repo root — see drawQuickMenu/drawSlotPicker
	// in renderer/ebitengine/tags_save.go and tags_uiscreens.go for exactly
	// which filenames are actually loaded; anything not listed there was
	// confirmed unreferenced and deleted rather than replaced). Sizes must
	// match the originals exactly — several are drawn unscaled.
	must(os.MkdirAll(sysRoot, 0o755))

	savePNG(filepath.Join(sysRoot, "bg_base.png"), solidOpaque(1280, 720, color.RGBA{0x24, 0x1f, 0x33, 0xff}))
	savePNG(filepath.Join(sysRoot, "button_menu.png"), hamburgerIcon(64, color.RGBA{0x3a, 0x35, 0x50, 0xf0}))
	savePNG(filepath.Join(sysRoot, "label_menu.png"), circleButton(285, color.RGBA{0x4a, 0x35, 0x6b, 0xff}, "MENU"))
	savePNG(filepath.Join(sysRoot, "label_load.png"), panelLabel(625, 120, color.RGBA{0x3a, 0x8a, 0x5c, 0xff}, "LOAD"))
	savePNG(filepath.Join(sysRoot, "label_save.png"), panelLabel(580, 120, color.RGBA{0xb0, 0x3a, 0x5a, 0xff}, "SAVE"))
	savePNG(filepath.Join(sysRoot, "saveslot.png"), panel(1000, 120, color.RGBA{0x3a, 0x35, 0x50, 0xc0}))
	// noimage.png is resolveFolderImage's (image_load.go) fallback graphic
	// for a [button folder="bgimage"] pointed at a CG/replay slot with no
	// image yet — real Tyrano's own bundled equivalent lives at
	// "../../tyrano/images/system/noimage.png"; this is kag3's placeholder
	// replacement for it, not a copy of that art.
	savePNG(filepath.Join(sysRoot, "noimage.png"), panelLabel(160, 110, color.RGBA{0x50, 0x48, 0x60, 0xff}, "NO IMAGE"))

	closeC := color.RGBA{0x6a, 0x2e, 0x2e, 0xff}
	closeCHover := color.RGBA{0x8a, 0x3d, 0x3d, 0xff}
	savePNG(filepath.Join(sysRoot, "menu_button_close.png"), circleButton(100, closeC, "X"))
	savePNG(filepath.Join(sysRoot, "menu_button_close2.png"), circleButton(100, closeCHover, "X"))

	quickMenuButtons := []struct {
		file   string
		label  string
		normal color.RGBA
		hover  color.RGBA
	}{
		{"menu_button_save", "SAVE", color.RGBA{0xb0, 0x3a, 0x5a, 0xff}, color.RGBA{0xd8, 0x4e, 0x76, 0xff}},
		{"menu_button_load", "LOAD", color.RGBA{0x3a, 0x8a, 0x5c, 0xff}, color.RGBA{0x4e, 0xae, 0x78, 0xff}},
		{"menu_message_close", "HIDE MESSAGE", color.RGBA{0x2e, 0x5c, 0x8a, 0xff}, color.RGBA{0x3d, 0x78, 0xb0, 0xff}},
		{"menu_button_skip", "SKIP", color.RGBA{0x8a, 0x6a, 0x2e, 0xff}, color.RGBA{0xb0, 0x8a, 0x3d, 0xff}},
		{"menu_button_title", "BACK TO TITLE", color.RGBA{0x6a, 0x2e, 0x2e, 0xff}, color.RGBA{0x8a, 0x3d, 0x3d, 0xff}},
	}
	for _, b := range quickMenuButtons {
		savePNG(filepath.Join(sysRoot, b.file+".png"), buttonImage(520, 70, b.normal, b.label))
		savePNG(filepath.Join(sysRoot, b.file+"2.png"), buttonImage(520, 70, b.hover, b.label))
	}

	// --- config.ks's own background/button images (example/game/resources/images/
	// config) — only the filenames config.ks actually references; the rest
	// of the original TyranoScript config/ art (arrows, unread-skip toggle
	// button art never actually wired to a [button], CG/recollection labels
	// left over from the removed cg.ks/replay.ks, a duplicate
	// menu_button_close shadowed by the system one above) was unreferenced
	// and deleted rather than replaced.
	must(os.MkdirAll(filepath.Join(root, "images", "config"), 0o755))

	savePNG(filepath.Join(root, "images", "config", "bg_config.png"),
		solidLabeled(1280, 720, color.RGBA{0x24, 0x1f, 0x33, 0xff}, color.White, "CONFIG", 3))
	savePNG(filepath.Join(root, "images", "config", "c_btn_back.png"), circleButton(100, color.RGBA{0x6a, 0x2e, 0x2e, 0xff}, "X"))
	savePNG(filepath.Join(root, "images", "config", "c_btn_back2.png"), circleButton(100, color.RGBA{0x8a, 0x3d, 0x3d, 0xff}, "X"))
	// c_btn.png/.gif is real Tyrano's "off" state for every volume/speed
	// number button (tf.btn_path_off) — tiny (4x4) because the actual
	// numbers/highlight are meant to come from the set1.png/set2.png
	// overlay [image]s drawn on top, not this base icon.
	savePNG(filepath.Join(root, "images", "config", "c_btn.png"), panel(4, 4, color.RGBA{0x50, 0x48, 0x60, 0xff}))
	saveGIF(filepath.Join(root, "images", "config", "c_btn.gif"), panel(4, 4, color.RGBA{0x50, 0x48, 0x60, 0xff}))
	savePNG(filepath.Join(root, "images", "config", "c_set.png"), panel(46, 46, color.RGBA{0x3a, 0x8a, 0x5c, 0xff}))
	savePNG(filepath.Join(root, "images", "config", "c_skipoff.png"), buttonImage(170, 45, color.RGBA{0x6a, 0x2e, 0x2e, 0xff}, "OFF"))
	savePNG(filepath.Join(root, "images", "config", "c_skipon.png"), buttonImage(170, 45, color.RGBA{0x3a, 0x8a, 0x5c, 0xff}, "ON"))
	// set1.png/set2.png (46x46 overlay icons for the BGM/SE/text-speed/auto-
	// speed number rows — see config.ks's *load_img) were referenced by 21
	// [image] tags each but never actually shipped in this project at all
	// (not a TyranoScript-copyright file to replace — just missing).
	// Generating them here fixes a latent "file not found" on first entry
	// to the config screen, not just a licensing concern.
	savePNG(filepath.Join(root, "images", "config", "set1.png"), panel(46, 46, color.RGBA{0xd8, 0xb0, 0x3d, 0xff}))
	savePNG(filepath.Join(root, "images", "config", "set2.png"), panel(46, 46, color.RGBA{0x4e, 0xae, 0x78, 0xff}))

	log.Println("placeholder images generated")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func solidLabeled(w, h int, bg, fg color.Color, label string, scale int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	drawLabelCentered(img, label, fg, scale)
	return img
}

// solidOpaque is a plain flat-color fill with no label — for full-screen
// backdrop images (bg_base.png) that sit behind other chrome, where a
// centered label would just get covered up.
func solidOpaque(w, h int, bg color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	return img
}

// panel is a plain rounded-rect filled block, no label/border — for chrome
// that's a pure background (saveslot.png row background, the tiny
// off-state number-button icons in config/).
func panel(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := min(w, h) / 4
	if r < 1 {
		r = 0
	}
	fillRoundedRect(img, 0, 0, w-1, h-1, r, c)
	return img
}

// panelLabel is a rounded-rect panel with a centered label — for the
// slot-picker's title banners (label_load.png/label_save.png), which are
// wide bars rather than clickable buttons (no border, unlike buttonImage).
func panelLabel(w, h int, bg color.Color, label string) *image.RGBA {
	img := panel(w, h, bg)
	drawLabelCentered(img, label, color.White, 2)
	return img
}

// circleButton is a filled circle with a centered label — for round icon
// buttons (the quick-menu/slot-picker close button, config's back button)
// and label_menu.png (285x285, square canvas but round art — real Tyrano's
// own menu label is a circular badge, not a horizontal banner like
// label_save/label_load).
func circleButton(d int, bg color.Color, label string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, d, d))
	fillCircle(img, d/2, d/2, d/2-1, bg)
	drawLabelCentered(img, label, color.White, 2)
	return img
}

// hamburgerIcon is the quick-menu toggle button (button_menu.png): a filled
// circle with three horizontal bars, standing in for real Tyrano's icon
// art without needing basicfont to render anything legible at 64px.
func hamburgerIcon(d int, bg color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, d, d))
	fillCircle(img, d/2, d/2, d/2-1, bg)
	barW, barH := d/2, d/16
	if barH < 2 {
		barH = 2
	}
	x0 := (d - barW) / 2
	for i, cy := range []int{d * 3 / 8, d / 2, d * 5 / 8} {
		_ = i
		for y := cy - barH/2; y < cy+barH/2; y++ {
			for x := x0; x < x0+barW; x++ {
				img.Set(x, y, color.White)
			}
		}
	}
	return img
}

func charaSilhouette(w, h int, c color.RGBA, label string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// transparent background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{}}, image.Point{}, draw.Src)
	// simple rounded silhouette: head circle + body rounded-rect, close
	// enough to read as "a standing character" at a glance.
	cx := w / 2
	headR := w / 5
	headCY := h/6 + headR
	fillCircle(img, cx, headCY, headR, c)
	bodyTop := headCY + headR - 10
	fillRoundedRect(img, cx-w/3, bodyTop, cx+w/3, h-20, 40, c)
	drawLabelCentered(img, label, color.White, 2)
	return img
}

func buttonImage(w, h int, bg color.Color, label string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	drawBorder(img, color.White, 3)
	drawLabelCentered(img, label, color.White, 1)
	return img
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}

func fillRoundedRect(img *image.RGBA, x0, y0, x1, y1, radius int, c color.Color) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			// corner rounding via distance check near each corner
			inCornerCut := false
			corners := [][2]int{{x0 + radius, y0 + radius}, {x1 - radius, y0 + radius}, {x0 + radius, y1 - radius}, {x1 - radius, y1 - radius}}
			if x < x0+radius && y < y0+radius {
				dx, dy := x-corners[0][0], y-corners[0][1]
				inCornerCut = dx*dx+dy*dy > radius*radius
			} else if x > x1-radius && y < y0+radius {
				dx, dy := x-corners[1][0], y-corners[1][1]
				inCornerCut = dx*dx+dy*dy > radius*radius
			} else if x < x0+radius && y > y1-radius {
				dx, dy := x-corners[2][0], y-corners[2][1]
				inCornerCut = dx*dx+dy*dy > radius*radius
			} else if x > x1-radius && y > y1-radius {
				dx, dy := x-corners[3][0], y-corners[3][1]
				inCornerCut = dx*dx+dy*dy > radius*radius
			}
			if !inCornerCut {
				img.Set(x, y, c)
			}
		}
	}
}

func drawBorder(img *image.RGBA, c color.Color, thickness int) {
	b := img.Bounds()
	for t := 0; t < thickness; t++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.Set(x, b.Min.Y+t, c)
			img.Set(x, b.Max.Y-1-t, c)
		}
		for y := b.Min.Y; y < b.Max.Y; y++ {
			img.Set(b.Min.X+t, y, c)
			img.Set(b.Max.X-1-t, y, c)
		}
	}
}

// drawLabelCentered draws label (basicfont, ASCII/JP-unsupported —
// placeholder labels are deliberately romaji-only) centered horizontally,
// scaled up scale× via nearest-neighbor for legibility at these image sizes.
func drawLabelCentered(img *image.RGBA, label string, c color.Color, scale int) {
	lines := splitLines(label)
	face := basicfont.Face7x13
	lineH := face.Metrics().Height.Ceil() * scale
	totalH := lineH * len(lines)
	b := img.Bounds()
	startY := b.Min.Y + (b.Dy()-totalH)/2 + lineH
	for i, line := range lines {
		w := font.MeasureString(face, line).Round() * scale
		x := b.Min.X + (b.Dx()-w)/2
		y := startY + i*lineH
		drawStringScaled(img, face, line, x, y, c, scale)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

// drawStringScaled renders at 1x onto a small buffer then nearest-neighbor
// upscales into img at (x,y) — basicfont only draws at native size.
func drawStringScaled(img *image.RGBA, face font.Face, s string, x, y int, c color.Color, scale int) {
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	w := font.MeasureString(face, s).Round()
	if w <= 0 {
		return
	}
	tmp := image.NewRGBA(image.Rect(0, 0, w, ascent+descent))
	d := &font.Drawer{
		Dst:  tmp,
		Src:  &image.Uniform{c},
		Face: face,
		Dot:  fixed.P(0, ascent),
	}
	d.DrawString(s)
	for ty := 0; ty < tmp.Bounds().Dy(); ty++ {
		for tx := 0; tx < tmp.Bounds().Dx(); tx++ {
			_, _, _, a := tmp.At(tx, ty).RGBA()
			if a == 0 {
				continue
			}
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					img.Set(x+tx*scale+sx, y-ascent*scale+ty*scale+sy, tmp.At(tx, ty))
				}
			}
		}
	}
}

func saveJPEG(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(jpeg.Encode(f, img, &jpeg.Options{Quality: 90}))
}

func savePNG(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(png.Encode(f, img))
}

func saveGIF(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(gif.Encode(f, img, nil))
}
