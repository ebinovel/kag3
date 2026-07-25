package ebitengine

import (
	"fmt"

	"github.com/ebinovel/kag3"
	"github.com/eihigh/coro"
)

// execItem runs exactly one parsed scenario item (TextObject/TagObject/
// CharacterInfo) against the given owning slice. scripts/i are parameterized
// rather than always r.scripts/the outer loop index so the same logic can
// run against a macro's own body (see expandMacro) without mutating the
// top-level script stream — that matters because r.labels' Index values are
// positions within r.scripts, and splicing macro bodies into r.scripts
// would invalidate every label after the splice point.
//
// depth is the current macro-expansion nesting level (0 at the top level),
// threaded through to dispatchTag so it can enforce a recursion limit when
// a tag turns out to be a macro call rather than a built-in.
//
// Known limitation: tags that jump execution elsewhere in the *top-level*
// script (jump/link/call) read/write r.scripts and r.labels directly, so
// they behave correctly when scripts==r.scripts but not if used inside a
// macro body — Tyrano macros aren't expected to contain them.
func (r *Renderer) execItem(y coro.Yield, scripts []any, i *int, depth int) error {
	s := scripts[*i]
	fmt.Printf("Index:%d, %+v\n", *i, s)
	switch object := s.(type) {
	case kag3.CharacterInfo:
	case kag3.TextObject:
		isNewLine := r.line != object.Line
		r.line = object.Line
		if isNewLine && len(object.Val) > 0 {
			isWait = false
			textStartT = t
		}
		if object.Chara != nil {
			fmt.Println(object.Chara)
			charaName = object.Chara.Name
		}
		if len(r.texts[object.Line]) == 0 {
			r.texts[object.Line] = appendRubyText(r.texts[object.Line], object.Val, pendingRuby)
			pendingRuby = ""
		} else {
			last := len(r.texts[object.Line]) - 1
			if r.texts[object.Line][last].Text == "" {
				// Filling in a [font]/[deffont]-created empty marker
				// segment in place (see applyTextStyle's doc comment) —
				// same ruby-splitting as appendRubyText below, just
				// written into the existing slot instead of appending.
				runes := []rune(object.Val)
				if pendingRuby != "" && len(runes) > 1 {
					r.texts[object.Line][last].Text = string(runes[0])
					r.texts[object.Line][last].Ruby = pendingRuby
					r.texts[object.Line] = append(r.texts[object.Line], Text{Text: string(runes[1:])})
				} else {
					r.texts[object.Line][last].Text = object.Val
					r.texts[object.Line][last].Ruby = pendingRuby
				}
				pendingRuby = ""
			} else {
				r.texts[object.Line] = appendRubyText(r.texts[object.Line], object.Val, pendingRuby)
				pendingRuby = ""
			}
		}
		// タグを挟まない連続するテキスト行を1つに連結
		if len(object.Val) > 0 {
			for *i+1 < len(scripts) {
				next, ok := scripts[*i+1].(kag3.TextObject)
				if !ok || len(next.Val) == 0 {
					break
				}
				(*i)++
				if next.Chara != nil {
					charaName = next.Chara.Name
				}
				last := len(r.texts[object.Line]) - 1
				r.texts[object.Line][last].Text += next.Val
			}
		}
		if isNewLine && len(object.Val) > 0 {
			y()
			y.Until(false, func() bool { return isWait })
		}
	case kag3.TagObject:
		r.line = object.Line
		// currentScripts records whichever slice this specific execItem call
		// is iterating — r.scripts at the top level, or a macro's Body while
		// expandMacro is running one (see expandMacro below) — so handlers
		// that scan forward/backward for a matching tag ([if]/[elsif]/
		// [else]/[endif]'s skipIfChain, [ignore], [keyframe], all in
		// tags_flow.go/tags_animation.go) search the right array instead of
		// always r.scripts. Unlike jump/link/call (documented as an accepted
		// macro limitation, since real Tyrano macros essentially never
		// contain them), [if] inside a macro body is common and expected —
		// tyrano.ks's own cg_image_button/replay_image_button macros use it
		// — so this one has to work.
		r.currentScripts = scripts
		if err := dispatchTag(r, y, object, i, depth); err != nil {
			return err
		}
	}
	return nil
}

// appendRubyText appends a new Text segment for val, splitting off a
// non-empty ruby onto just its first rune. Real Tyrano's [ruby text=...]
// annotates exactly the single character immediately after it — parsing
// only sees tag/text boundaries, though, so "[ruby text=たん]単にできます"
// (no further tag until the line ends) parses as one six-rune TextObject,
// and attaching the ruby to the whole thing would center "たん" over all
// six characters instead of just "単". Splitting here keeps drawMessage*'s
// per-segment ruby centering (draw_message.go) correct without it having
// to know anything about this.
func appendRubyText(line []Text, val, ruby string) []Text {
	if ruby != "" {
		if runes := []rune(val); len(runes) > 1 {
			return append(line,
				Text{Text: string(runes[0]), Ruby: ruby},
				Text{Text: string(runes[1:])},
			)
		}
	}
	return append(line, Text{Text: val, Ruby: ruby})
}

// maxMacroDepth guards against runaway/self-recursive macro expansion.
const maxMacroDepth = 32

// expandMacro runs a macro's body in place of an unrecognized tag. pm
// becomes the macro-call's "mp" namespace (readable from JS as mp.name,
// and via "%name" tag-argument expansion) for the duration of the call.
func (r *Renderer) expandMacro(y coro.Yield, m *kag3.Macro, pm map[string]string, depth int) error {
	if depth > maxMacroDepth {
		return fmt.Errorf("マクロの再帰が深すぎます: %s", m.Name)
	}
	pop := r.vm.PushMPFrame(pm)
	defer pop()
	for j := 0; j < len(m.Body); j++ {
		if err := r.execItem(y, m.Body, &j, depth); err != nil {
			return err
		}
		y()
	}
	return nil
}
