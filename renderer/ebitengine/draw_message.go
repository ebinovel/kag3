package ebitengine

import (
	"image/color"
	"math"
	"slices"

	"github.com/ebinovel/kag3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

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
	if textPosition.FrameImage != nil {
		op.ColorScale.SetA(float32(textPosition.Opacity))
		buf.DrawImage(textPosition.FrameImage, op)
	} else {
		if textPosition.FilterColor != nil {
			textPosition.BackImage.Fill(*textPosition.FilterColor)
		} else {
			textPosition.BackImage.Fill(color.RGBA{0, 0, 0, 128})
		}
		buf.DrawImage(textPosition.BackImage, op)
	}
	drawPTexts(buf, r.nameFontFace)

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
	drawGlyph(buf)
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

// drawMessageHorizontal lays out r.texts as normal left-to-right rows: the
// active line (r.line) reveals glyph-by-glyph up to count, every earlier
// line on the same page draws fully revealed (see the applyTextStyle
// comment below for why that still needs the current [font]/[deffont]
// color rather than a bare white default).
func drawMessageHorizontal(r *Renderer, buf *ebiten.Image, marginLeft, marginTop float64, lineNums []int, count int) {
	rowY := 0.0
	for _, lineNum := range lineNums {
		segs := r.texts[lineNum]
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
			totalGlyphs := 0
			for _, v := range segs {
				if len(v.Text) == 0 {
					continue
				}
				tOp2 := &text.DrawOptions{}
				tOp2.LineSpacing = r.fontFace.Size
				g := text.AppendGlyphs(nil, v.Text, r.fontFace, &tOp2.LayoutOptions)
				totalGlyphs += len(g)
			}
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
				for _, v := range segs {
					if len(v.Text) == 0 {
						continue
					}
					tOp := &text.DrawOptions{}
					tOp.LineSpacing = r.fontFace.Size
					vText := v.Text
					w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
					maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
					if w > maxWidth {
						rn := []rune(vText)
						key := len(rn) - 1
						for idx := key; idx >= 0; idx-- {
							ww, _ := text.Measure(string(rn[:idx]), r.fontFace, tOp.LineSpacing)
							if ww <= maxWidth {
								rn = append(rn[:idx+1], rn[idx:]...)
								rn[idx] = []rune("\n")[0]
								break
							}
						}
						vText = string(rn)
						w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
					}
					glyph := text.AppendGlyphs(nil, vText, r.fontFace, &tOp.LayoutOptions)
					segLen := len(glyph)
					applyTextStyle(r, tOp, v)
					segShowCount := charsToShow - segOffset
					if segShowCount >= segLen {
						tOp.GeoM.Reset()
						tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
						text.Draw(buf, vText, r.fontFace, tOp)
						if v.Ruby != "" {
							rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: beforeTextSize * 0.5, Language: r.fontFace.Language}
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
					textEndX = marginLeft + xOffset
					// marginTop+rowY+rubyLineHeight is this row's *top*
					// (where text.Draw's own Y coordinate anchors, per
					// text/v2's default top-left origin) — add beforeTextSize
					// (this row's height, the same value rowY itself is
					// incremented by below) to land on the row's bottom
					// instead, so the wait-mark's rest position sits right
					// under the actual characters rather than floating
					// somewhere above them.
					textEndY = marginTop + rowY + rubyLineHeight + beforeTextSize
				}
			}
		} else {
			xOffset := 0.0
			for _, v := range segs {
				if len(v.Text) == 0 {
					continue
				}
				tOp := &text.DrawOptions{}
				tOp.LineSpacing = r.fontFace.Size
				vText := v.Text
				w, _ := text.Measure(vText, r.fontFace, tOp.LineSpacing)
				maxWidth := float64(textPosition.Width - textPosition.MarginLeft - textPosition.MarginRight)
				if w > maxWidth {
					rn := []rune(vText)
					key := len(rn) - 1
					for idx := key; idx >= 0; idx-- {
						ww, _ := text.Measure(string(rn[:idx]), r.fontFace, tOp.LineSpacing)
						if ww <= maxWidth {
							rn = append(rn[:idx+1], rn[idx:]...)
							rn[idx] = []rune("\n")[0]
							break
						}
					}
					vText = string(rn)
					w, _ = text.Measure(vText, r.fontFace, tOp.LineSpacing)
				}
				// applyTextStyle, not a bare color.White default: an
				// already fully-revealed line must keep using
				// whatever [font]/[deffont] color is *currently* in
				// effect, the same as the active line resolves it —
				// before this fix it fell straight to plain white
				// the instant it stopped being the active line,
				// which on a light/white message-box design (e.g.
				// scene1.ks's [deffont color="0x454D51"] custom
				// window) made every earlier line on the same page
				// effectively invisible against the background as
				// soon as the next line started revealing.
				applyTextStyle(r, tOp, v)
				tOp.GeoM.Translate(marginLeft+xOffset, marginTop+rowY+rubyLineHeight)
				text.Draw(buf, vText, r.fontFace, tOp)
				if v.Ruby != "" {
					rubyFace := &text.GoTextFace{Source: r.fontFace.Source, Size: beforeTextSize * 0.5, Language: r.fontFace.Language}
					rubyW, _ := text.Measure(v.Ruby, rubyFace, 0)
					rubyOp := &text.DrawOptions{}
					rubyOp.GeoM.Translate(marginLeft+xOffset+(w-rubyW)/2, marginTop+rowY)
					rubyOp.ColorScale.ScaleWithColor(color.White)
					text.Draw(buf, v.Ruby, rubyFace, rubyOp)
				}
				xOffset += w
			}
		}
		rowY += beforeTextSize + rubyLineHeight
	}
}
