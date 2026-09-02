package ebitengine

import (
	"image/color"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// activeLineGlyphCount reports how many glyphs (horizontal layout, counted
// the same way drawMessageHorizontal does via text.AppendGlyphs — font
// metrics affect glyph count, so a plain rune count wouldn't match what's
// actually drawn) or runes (vertical layout, which lays out one rune per
// cell with no font-dependent shaping) the currently-revealing line
// (r.line) has in total. This is the same total drawMessageHorizontal/
// drawMessageVertical compare their reveal counter against to decide a
// line has finished — factored out so revealActiveLine (renderer.go) can
// make that same decision from Update(), not just from Draw(). Returns 0
// for a line with no text yet (r.texts[r.line] absent or empty), matching
// both draw functions' own "nothing to compare against, don't touch
// isWait" behavior for that case.
func activeLineGlyphCount(r *Renderer, vertical bool) int {
	segs := r.texts[r.line]
	if vertical {
		n := 0
		for _, v := range segs {
			n += utf8.RuneCountInString(v.Text)
		}
		return n
	}
	total := 0
	for _, v := range segs {
		if len(v.Text) == 0 {
			continue
		}
		tOp := &text.DrawOptions{}
		tOp.LineSpacing = r.fontFace.Size
		g := text.AppendGlyphs(nil, v.Text, r.fontFace, &tOp.LayoutOptions)
		total += len(g)
	}
	return total
}

// drawMessageWindow draws the message box frame (or configured FrameImage),
// every [ptext] area (including the character name-plate), and the current
// page's revealed text — delegating the actual glyph layout to
// drawMessageVertical/drawMessageHorizontal depending on [position
// vertical=true].
func drawMessageWindow(r *Renderer, buf *ebiten.Image) {
	if textPosition == nil || !textPosition.Visible {
		return
	}
	op := &ebiten.DrawImageOptions{}
	x, y := float64(textPosition.Left), float64(textPosition.Top)
	op.GeoM.Translate(x, y)
	switch {
	case textPosition.FrameImage != nil:
		op.ColorScale.SetA(float32(textPosition.Opacity))
		buf.DrawImage(textPosition.FrameImage, op)
	case textPosition.BackImage != nil:
		if textPosition.FilterColor != nil {
			textPosition.BackImage.Fill(*textPosition.FilterColor)
		} else {
			textPosition.BackImage.Fill(messageBoxFillColor(r.manager.Config.MessageBoxStyle))
		}
		buf.DrawImage(textPosition.BackImage, op)
	default:
		// Neither image exists yet — Visible alone (the guard above) isn't
		// proof the box is actually ready to draw. [ct] (handleCT,
		// tags_message.go) is the one place that produces this exact
		// combination on purpose: it replaces textPosition wholesale with a
		// fresh struct that keeps Visible but zeroes everything else,
		// including BackImage, as its documented "revert layout to a blank
		// slate" behavior — Width/Height land at 0 too, so even
		// handlePosition's own "only allocate once both are nonzero" guard
		// (tags_layer.go) doesn't reallocate one until whatever [position]
		// call follows [ct] actually sets a size. Any frame drawn in that
		// gap would otherwise call BackImage.Fill on a nil *ebiten.Image and
		// crash the whole renderer over a single tag with nothing else wrong
		// in the script. Skipping just the box here is enough: nothing below
		// this switch reads BackImage/FrameImage again, so the rest of the
		// window (text, ptexts, the operation row) still draws normally,
		// just without a background box for however many frames it takes
		// the script to give it one.
	}

	marginLeft := x + float64(textPosition.MarginLeft)
	marginTop := y + float64(textPosition.MarginTop)
	count := math.MaxInt32
	if !textNoWait {
		count = (t - textStartT) / ticksPerChar()
	}
	isTextEnd = false
	lineNums := make([]int, 0, len(r.texts))
	for k := range r.texts {
		lineNums = append(lineNums, k)
	}
	slices.Sort(lineNums)
	if textPosition.Vertical {
		drawMessageVertical(r, buf, x, marginTop, lineNums, count)
	} else {
		drawMessageHorizontal(r, buf, marginLeft, marginTop, lineNums, count)
	}
	textGlyphs = []text.Glyph{}
	if r.manager.Config.ContinueMarkStyle == "fixed" {
		drawContinueMark(r, buf)
	} else {
		drawGlyph(buf)
	}
	drawTextSpeedIndicator(r, buf)
}

// drawMessageVertical lays out r.texts top-to-bottom within each column,
// columns filling right-to-left — [position vertical=true]'s draw path.
func drawMessageVertical(r *Renderer, buf *ebiten.Image, x, marginTop float64, lineNums []int, count int) {
	type runeEntry struct {
		ch    rune
		style *kag3.TextStyle
	}
	charSize := r.verticalFontFace.Size
	rightEdge := x + float64(textPosition.Width) - float64(textPosition.MarginRight)
	availableHeight := float64(textPosition.Height) - float64(textPosition.MarginTop) - float64(textPosition.MarginBottom)
	charsPerCol := int(availableHeight / charSize)
	if charsPerCol <= 0 {
		charsPerCol = 1
	}
	colIndex := 0
	for _, lineNum := range lineNums {
		segs := r.texts[lineNum]
		var runes []runeEntry
		for _, v := range segs {
			if len(v.Text) == 0 {
				continue
			}
			for _, ch := range v.Text {
				runes = append(runes, runeEntry{ch, v.TextStyle})
			}
		}
		if len(runes) == 0 {
			continue
		}
		showCount := len(runes)
		if lineNum == r.line {
			if !isWait {
				if count >= len(runes) {
					isWait = true
					isTextEnd = true
				} else {
					showCount = count
				}
			} else {
				isTextEnd = true
			}
		}
		for i, rs := range runes[:showCount] {
			col := colIndex + i/charsPerCol
			row := i % charsPerCol
			colX := rightEdge - float64(col+1)*charSize
			tOp := &text.DrawOptions{}
			tOp.GeoM.Translate(colX, marginTop+float64(row)*charSize)
			applyTextStyle(r, tOp, Text{TextStyle: rs.style})
			text.Draw(buf, string(rs.ch), r.verticalFontFace, tOp)
			if lineNum == r.line && isTextEnd && i == showCount-1 {
				textEndX = colX
				textEndY = marginTop + float64(row+1)*charSize
			}
		}
		colIndex += (len(runes) + charsPerCol - 1) / charsPerCol
	}
}

// wrapText hard-wraps s into as many "\n"-separated rows as it takes for
// every row to measure within maxWidth, using the same pixel-measured,
// rune-granularity fitting check as the rest of this file (no word-boundary
// awareness — matches how CJK text is conventionally wrapped, and this
// engine has never distinguished words for this purpose). It loops until
// every row fits rather than breaking once at the first overflow point:
// text several times maxWidth wide (e.g. demo_movie.ks's credit line) needs
// as many breaks as it takes, not one.
func wrapText(s string, face *text.GoTextFace, lineSpacing, maxWidth float64) string {
	remaining := []rune(s)
	var lines []string
	for {
		w, _ := text.Measure(string(remaining), face, lineSpacing)
		if w <= maxWidth {
			lines = append(lines, string(remaining))
			break
		}
		split := len(remaining) - 1
		for split > 0 {
			ww, _ := text.Measure(string(remaining[:split]), face, lineSpacing)
			if ww <= maxWidth {
				break
			}
			split--
		}
		if split == 0 {
			// Not even a single rune fits maxWidth (a pathologically narrow
			// message box) — take one anyway so the loop always makes
			// forward progress instead of spinning forever.
			split = 1
		}
		lines = append(lines, string(remaining[:split]))
		remaining = remaining[split:]
	}
	return strings.Join(lines, "\n")
}

// drawMessageHorizontal lays out r.texts as normal left-to-right rows: the
// active line (r.line) reveals glyph-by-glyph up to count, every earlier
// line on the same page draws fully revealed (see the applyTextStyle
// comment below for why that still needs the current [font]/[deffont]
// color rather than a bare white default).
func drawMessageHorizontal(r *Renderer, buf *ebiten.Image, marginLeft, marginTop float64, lineNums []int, count int) {
	rowY := 0.0
	for _, lineNum := range lineNums {
		segs := r.texts[lineNum]
		// rowHeight tracks the tallest size any segment on this line
		// actually resolves to (updated alongside each applyTextStyle
		// call below) — advancing rowY by a flat beforeTextSize
		// regardless leaves a [font size=40] line's descender overlapping
		// whatever line comes right after it, since the next row would
		// start at the *default* line height instead of the enlarged one.
		rowHeight := beforeTextSize
		hasText := false
		for _, v := range segs {
			if len(v.Text) > 0 {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}
		rubyLineHeight := 0.0
		for _, v := range segs {
			if v.Ruby != "" {
				rubyLineHeight = beforeTextSize * 0.5
				break
			}
		}
		if lineNum == r.line {
			// Same total activeLineGlyphCount computes (it reads
			// r.texts[r.line], which is exactly segs here since lineNum ==
			// r.line) — shared so Update()'s revealActiveLine and this
			// function can never disagree about what "fully revealed"
			// means for the active line.
			totalGlyphs := activeLineGlyphCount(r, false)
			if totalGlyphs > 0 {
				charsToShow := totalGlyphs
				if !isWait {
					if count >= totalGlyphs {
						isWait = true
						isTextEnd = true
					} else {
						charsToShow = count
					}
				} else {
					isTextEnd = true
				}
				segOffset := 0
				xOffset := 0.0
				// wrapRows/lastLineX track whichever segment was most
				// recently processed: how many extra physical rows its own
				// auto-wrap (the maxWidth check below) added, and the pixel
				// width of just its *last* wrapped row. xOffset keeps
				// accumulating the segment's full measured width (its
				// widest wrapped row, from text.Measure) regardless — it's
				// only ever used to position a *later* segment on the same
				// logical line immediately after this one, which text/v2
				// draws as a plain horizontal continuation rather than
				// respecting an embedded "\n" itself, so it has to stay in
				// that same coordinate space. textEnd below is the one
				// place true wrapped position matters (see its comment).
				wrapRows := 0
				lastLineX := 0.0
				for _, v := range segs {
					if len(v.Text) == 0 {
						continue
					}
					tOp := &text.DrawOptions{}
					// applyTextStyle first — it's what sets r.fontFace.Size
					// for *this* segment, and text.Measure/AppendGlyphs
					// below read r.fontFace directly. Measuring before
					// calling this uses the previous segment's leftover
					// size instead of this one's, both under- and
					// over-shooting xOffset (wrong spacing/overlap right
					// after a size change) and mis-centering this segment's
					// ruby text over the wrong width.
					applyTextStyle(r, tOp, v)
					if r.fontFace.Size > rowHeight {
						rowHeight = r.fontFace.Size
					}
					tOp.LineSpacing = r.fontFace.Size
					vText := v.Text
					w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
					maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
					wrapRows = 0
					lastLineX = xOffset + w
					if w > maxWidth {
						vText = wrapText(vText, r.fontFace, tOp.LineSpacing, maxWidth)
						w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
						wrapLines := strings.Split(vText, "\n")
						wrapRows = len(wrapLines) - 1
						lastLineX, _ = text.Measure(wrapLines[len(wrapLines)-1], r.fontFace, tOp.LineSpacing)
					}
					glyph := text.AppendGlyphs(nil, vText, r.fontFace, &tOp.LayoutOptions)
					segLen := len(glyph)
					segShowCount := charsToShow - segOffset
					if segShowCount >= segLen {
						tOp.GeoM.Reset()
						tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
						text.Draw(buf, vText, r.fontFace, tOp)
						if v.Ruby != "" {
							rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: r.fontFace.Size * 0.5, Language: r.fontFace.Language}
							rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
							rubyOp := &text.DrawOptions{}
							rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
							rubyOp.ColorScale.ScaleWithColor(color.White)
							text.Draw(buf, v.Ruby, rubyFace, rubyOp)
						}
					} else if segShowCount > 0 {
						for _, g := range glyph[:segShowCount] {
							if g.Image == nil {
								continue
							}
							tOp.GeoM.Reset()
							tOp.GeoM.Translate(marginLeft+xOffset+g.X, marginTop+rowY+rubyLineHeight+g.Y)
							buf.DrawImage(g.Image, &tOp.DrawImageOptions)
						}
					}
					xOffset += w
					segOffset += segLen
				}
				if isTextEnd {
					// marginTop+rowY+rubyLineHeight is this row's *top*
					// (where text.Draw's own Y coordinate anchors, per
					// text/v2's default top-left origin) — add rowHeight
					// (this row's actual height, the same value rowY
					// itself is incremented by below) to land on the
					// row's bottom instead, so the wait-mark's rest
					// position sits right under the actual characters
					// rather than floating somewhere above them.
					//
					// If the last segment processed auto-wrapped (wrapRows
					// > 0), that "bottom" is wrapRows rows further down
					// than a single-line segment's, and the horizontal
					// position is lastLineX (this segment's own last
					// wrapped row's width) — not marginLeft+xOffset, which
					// is the *widest* row's width and would land the mark
					// after line 1 instead of the actual last line, since
					// xOffset accumulates via text.Measure's whole-block
					// (max-line) width, not "how far the cursor ended up."
					textEndX = marginLeft + lastLineX
					textEndY = marginTop + rowY + rubyLineHeight + rowHeight + float64(wrapRows)*rowHeight
				}
			}
		} else {
			xOffset := 0.0
			for _, v := range segs {
				if len(v.Text) == 0 {
					continue
				}
				tOp := &text.DrawOptions{}
				// applyTextStyle, not a bare color.White default: an
				// already fully-revealed line must keep using
				// whatever [font]/[deffont] color is *currently* in
				// effect, the same as the active line resolves it —
				// falling back to plain white the instant a
				// line stops being the active one makes every
				// earlier line on the same page effectively
				// invisible against a light/white message-box
				// design (e.g. scene1.ks's [deffont
				// color="0x454D51"] custom window).
				//
				// Called first, before text.Measure/AppendGlyphs below —
				// same reasoning as the active-line branch above:
				// r.fontFace.Size has to be resolved for *this* segment
				// before anything measures against r.fontFace.
				applyTextStyle(r, tOp, v)
				if r.fontFace.Size > rowHeight {
					rowHeight = r.fontFace.Size
				}
				tOp.LineSpacing = r.fontFace.Size
				vText := v.Text
				w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
				maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
				if w > maxWidth {
					vText = wrapText(vText, r.fontFace, tOp.LineSpacing, maxWidth)
					w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
				}
				tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
				text.Draw(buf, vText, r.fontFace, tOp)
				if v.Ruby != "" {
					rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: r.fontFace.Size * 0.5, Language: r.fontFace.Language}
					rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
					rubyOp := &text.DrawOptions{}
					rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
					rubyOp.ColorScale.ScaleWithColor(color.White)
					text.Draw(buf, v.Ruby, rubyFace, rubyOp)
				}
				xOffset += w
			}
		}
		lineHeightRatio := 1.0
		if r.manager.Config.MessageBoxStyle == "redesigned" {
			lineHeightRatio = bodyLineHeightRatio
		}
		rowY += rowHeight*lineHeightRatio + rubyLineHeight
	}
}
