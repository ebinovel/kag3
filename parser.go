package kag3

import (
	"fmt"
	"regexp"
	"strings"
)

func (ks *KS) LoadConfig(callback func()) {

}

func (ks *KS) compileConfig() {
}

func (ks *KS) ParseScenario(scenario string) (result []interface{}, mapLabel map[string]LabelInfo, err error) { // {{{
	mapLabel = make(map[string]LabelInfo, 0)
	isInComment := false
	ks.isInScript = false
	scriptTagIndex := -1
	var scriptBuf []string

	for i, s := range strings.Split(scenario, "\n") {
		line := strings.TrimSpace(s)
		if line == "" {
			continue
		}

		// [iscript]...[endscript] bodies are raw JavaScript: buffer every
		// line verbatim until the matching [endscript]/@endscript instead
		// of running it through comment/tag/text parsing.
		if ks.isInScript {
			if name, body, ok := splitTagLine(line); ok && name == "endscript" {
				endTag := ks.makeTag(body, i)
				if scriptTagIndex >= 0 {
					if pending, ok := result[scriptTagIndex].(TagObject); ok {
						pending.Body = strings.Join(scriptBuf, "\n")
						result[scriptTagIndex] = pending
					}
					scriptTagIndex = -1
				}
				result = append(result, endTag)
				scriptBuf = nil
			} else {
				scriptBuf = append(scriptBuf, line)
			}
			continue
		}

		firstChar := line[0]

		if isInComment && line == "*/" {
			isInComment = false
		} else if line == "/*" {
			isInComment = true
		} else if isInComment || firstChar == ';' {
			// nop
		} else if firstChar == '#' {
			result = append(result, characterPText(line, i))
		} else if firstChar == '*' {
			tmpLabel := strings.Split(line[1:], "|")
			labelKey, labelVal := "", ""
			labelKey = strings.TrimSpace(tmpLabel[0])
			if len(tmpLabel) > 1 {
				labelVal = strings.TrimSpace(tmpLabel[1])
			}
			info := LabelInfo{
				Line:  i,
				Index: len(result),
				Name:  labelKey,
				Val:   labelVal,
			}
			labelObject := LabelObject{
				Name: "label",
				Val:  labelVal,
				Info: info,
			}
			result = append(result, labelObject)

			if _, ok := mapLabel[labelKey]; ok {
				return nil, nil, fmt.Errorf("Warning line:%d ラベル名 '%s' は同一シナリオファイル内に重複してます", i, labelKey)
			} else {
				mapLabel[labelKey] = info
			}
		} else if firstChar == '@' {
			tmpTag := line[1:]
			tagObject := ks.makeTag(tmpTag, i)
			result = append(result, tagObject)
			if tagObject.Name == "iscript" {
				scriptTagIndex = len(result) - 1
				scriptBuf = nil
			}
		} else {
			if firstChar == '_' {
				line = line[1:]
			}
			chars := strings.Split(line, "")
			text := ""
			tag := ""
			isInTag := false
			bracketsCount := 0
			for _, c := range chars {
				if isInTag {
					if c == "]" {
						bracketsCount--

						if bracketsCount == 0 {
							isInTag = false
							newTag := ks.makeTag(tag, i)
							result = append(result, newTag)
							if newTag.Name == "iscript" {
								scriptTagIndex = len(result) - 1
								scriptBuf = nil
							}
							tag = ""
						} else {
							tag += c
						}
					} else if c == "[" {
						bracketsCount++
						tag += c
					} else {
						tag += c
					}
				} else if isInTag == false && c == "[" {
					bracketsCount++
					if text != "" {
						textObject := TextObject{
							Line: i,
							Name: "text",
							Val:  text,
						}
						result = append(result, textObject)
						text = ""
					}
					isInTag = true
				} else {
					text += c
				}
			}
			if text != "" {
				textObject := TextObject{
					Line: i,
					Name: "text",
					Val:  text,
				}
				result = append(result, textObject)
			}
		}
	}

	return result, mapLabel, nil
} //}}}

// quotedSpace/quotedEquals stand in for a space or an "=" that appeared
// *inside* a quoted attribute value, for exactly as long as it takes
// makeTag to finish tokenizing. Both characters are structural to the
// tokenizer below — attributes are separated by splitting on runs of
// spaces, and a key is separated from its value by splitting on "=" — so
// occurrences that came from inside quotes have to be hidden from those
// two splits and put back afterwards.
//
// Control characters, deliberately: this used to substitute the *space*
// with nothing at all (destroying it outright — `[ptext text="Hello World"]`
// arrived as "HelloWorld") and the "=" with "#", which was then mapped back
// with a blanket ReplaceAll("#", "="). That second half quietly corrupted
// any value legitimately containing a "#": `color="#FF0000"` parsed as
// "=FF0000". A stand-in only works if it cannot occur in real source text,
// which "#" plainly can and NUL/SOH cannot — and makeTag strips them from
// its input up front (see below) so even a pathological .ks file can't
// smuggle one in to forge an attribute boundary.
const (
	quotedSpace  = "\x00"
	quotedEquals = "\x01"
)

// unescapeQuoted restores the characters quotedSpace/quotedEquals stood in
// for. Applied per key and per value, after tokenizing and after
// TrimSpace — so ordinary whitespace around an attribute is still trimmed
// while whitespace the author actually quoted survives verbatim.
func unescapeQuoted(s string) string {
	s = strings.ReplaceAll(s, quotedEquals, "=")
	s = strings.ReplaceAll(s, quotedSpace, " ")
	return s
}

func (ks *KS) makeTag(s string, lineNum int) TagObject { // {{{
	tag := TagObject{}
	// Strip the stand-ins from the input so the only ones present later are
	// the ones this function put there itself (see quotedSpace's comment).
	s = strings.NewReplacer(quotedSpace, "", quotedEquals, "").Replace(s)
	c := strings.Split(s, "")
	quoteStr := ""
	tmpStr := ""
	quoteCount := 0

	for _, cc := range c {
		if quoteStr == "" && (cc == `"` || cc == `'`) {
			quoteStr = cc
			quoteCount = 0
		} else {
			if quoteStr != "" {
				if quoteStr == cc {
					quoteStr = ""

					if quoteCount == 0 {
						tmpStr += "undefined"
					}

					quoteCount = 0
				} else {
					if cc == "=" {
						cc = quotedEquals
					}
					if cc == " " {
						cc = quotedSpace
					}

					tmpStr += cc
					quoteCount++
				}
			} else {
				tmpStr += cc
			}
		}
	}

	str := tmpStr
	strs := regexp.MustCompile(` +`).Split(str, -1)

	tag.Name = strings.TrimSpace(strs[0])
	tag.Line = lineNum

	strs = joinSpacedEquals(strs)

	tag.Pm = make(map[string]string)
	for _, cc := range strs {
		tmp := strings.Split(cc, "=")
		key := unescapeQuoted(strings.TrimSpace(tmp[0]))
		val := ""

		if len(tmp) > 1 {
			val = strings.TrimSpace(tmp[1])
		}

		if key == "*" {
			tag.Pm["*"] = ""
		}
		if val != "" {
			tag.Pm[key] = unescapeQuoted(val)
		}
		if val == "undefined" {
			tag.Pm[key] = ""
		}
	}

	if tag.Name == "iscript" {
		ks.isInScript = true
	}
	if tag.Name == "endscript" {
		ks.isInScript = false
	}
	tag.IfCount = 0

	switch tag.Name {
	case "if":
		ks.ifCount++
		tag.IfCount = ks.ifCount
	case "elsif", "else":
		tag.IfCount = ks.ifCount
	case "endif":
		tag.IfCount = ks.ifCount
		ks.ifCount--
	}
	return tag
} // }}}

// splitTagLine extracts a tag's name and raw body ("name arg=val ...") from
// a line written as either "@name ..." or "[name ...]". ok is false if the
// line isn't a bare tag line in one of those two forms.
func splitTagLine(line string) (name, body string, ok bool) {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "@"):
		body = trimmed[1:]
	case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
		body = trimmed[1 : len(trimmed)-1]
	default:
		return "", "", false
	}
	name = body
	if idx := strings.IndexAny(body, " \t"); idx != -1 {
		name = body[:idx]
	}
	return name, body, true
}

func characterPText(line string, lineCount int) TextObject {
	tmpLine := strings.TrimSpace(strings.Replace(line, "#", "", 1))
	charaName := ""
	charaFace := ""
	if len(strings.Split(tmpLine, ":")) > 1 {
		lines := strings.Split(tmpLine, ":")
		charaName, charaFace = lines[0], lines[1]
	} else {
		charaName = tmpLine
	}
	textObject := TextObject{
		Line:      lineCount,
		Name:      "chara_ptext",
		Chara: &CharacterInfo{
			Name: charaName,
			Face: charaFace,
		},
	}
	return textObject
}

// joinSpacedEquals rejoins attribute tokens that regexp.Split(` +`) broke
// apart around an "=" that had whitespace on one or both sides: "key",
// "=", "value" (space on both sides), "key=", "value" (space only after),
// and "key", "=value" (space only before) all become a single
// "key=value" token, matching how a plain "key=value" (no surrounding
// space) is already a single token straight out of the split. strs[0]
// (the tag name) is never a merge candidate; empty tokens (a tag body
// ending in trailing whitespace collapses to one via the split) are
// dropped rather than treated as a key.
//
// This replaces an earlier version of this rejoining step that tried to
// mutate strs in place while ranging over it — range captures the slice
// header once at loop start, so reassigning strs mid-loop never affected
// what the loop actually walked, and the surviving index arithmetic
// (guarding merges with `len(strs) > i+2` etc.) was tuned to that broken
// control flow rather than to the merge actually succeeding. In practice
// it meant any single "key = value" attribute (the exact number of
// tokens produced was the case this off-by-one landed on) was silently
// dropped from Pm — confirmed to happen on real content, not just a
// theoretical edge case: kag3's own example/game/resources/senarios/title.ks
// has "@wait time = 200" and tyrano.ks has "[freeimage layer = %layer]",
// both of which previously vanished without any error.
func joinSpacedEquals(strs []string) []string {
	if len(strs) == 0 {
		return strs
	}
	out := make([]string, 0, len(strs))
	out = append(out, strs[0])
	for i := 1; i < len(strs); i++ {
		tok := strs[i]
		switch {
		case tok == "":
			// drop
		case tok == "=":
			if len(out) > 0 && i+1 < len(strs) {
				out[len(out)-1] = out[len(out)-1] + "=" + strs[i+1]
				i++
				continue
			}
			out = append(out, tok)
		case strings.HasSuffix(tok, "=") && i+1 < len(strs) && strs[i+1] != "":
			out = append(out, tok+strs[i+1])
			i++
		case strings.HasPrefix(tok, "=") && len(out) > 0:
			out[len(out)-1] = out[len(out)-1] + tok
		default:
			out = append(out, tok)
		}
	}
	return out
}
