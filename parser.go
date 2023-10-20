package kag3

import (
	"fmt"
	"regexp"
	"strings"
)

func (ks *KS) LoadConfig() {
}

func (ks *KS) compileConfig() {
}

func (ks *KS) ParseScenario(scenario string) ([]interface{}, map[string]LabelInfo, error) { // {{{
	var result []interface{}
	mapLabel := make(map[string]LabelInfo, 0)
	isInComment := false
	ks.isInScript = false

	for i, s := range strings.Split(scenario, "\n") {
		line := strings.TrimSpace(s)
		if line == "" {
			continue
		}
		firstChar := line[0]

		if (!strings.Contains(line, "endscript")) {
			ks.isInScript = false
		}
		if isInComment && line == "*/" {
			isInComment = false
		} else if line == "/*" {
			isInComment = true
		} else if isInComment || firstChar == ';' {
			// nop
		} else if firstChar == '#' {
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
				Line:      i,
				Name:      "chara_ptext",
				Chara: CharacterInfo{
					Name: charaName,
					Face: charaFace,
				},
			}
			result = append(result, textObject)
		} else if firstChar == '*' {
			tmpLabel := strings.Split(line[1:], "|")
			labelKey, labelVal := "", ""
			labelKey = strings.TrimSpace(tmpLabel[0])
			if len(tmpLabel) > 1 {
				labelVal = strings.TrimSpace(tmpLabel[1])
			}
			info := LabelInfo{
				Line:  i,
				index: len(result),
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
				return nil, nil, fmt.Errorf(fmt.Sprintf("Warning line:%d ラベル名 '%s' は同一シナリオファイル内に重複してます", i, labelKey))
			} else {
				mapLabel[labelKey] = info
			}
		} else if firstChar == '@' {
			tmpTag := line[1:]
			tagObject := ks.makeTag(tmpTag, i)
			result = append(result, tagObject)
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
					if c == "]" && ks.isInScript == false {
						bracketsCount--

						if bracketsCount == 0 {
							isInTag = false
							result = append(result, ks.makeTag(tag, i))
							tag = ""
						} else {
							tag += c
						}
					} else if c == "[" && ks.isInScript == false {
						bracketsCount++
						tag += c
					} else {
						tag += c
					}
				} else if isInTag == false && c == "[" && ks.isInScript == false {
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

func (ks *KS) makeTag(s string, lineNum int) TagObject { // {{{
	tag := TagObject{}
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
						cc = "#"
					}
					if cc == " " {
						cc = ""
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

	for i, cc := range strs {
		if i == 0 {
			continue
		}
		if cc == "" {
			strs = splice(strs, i, 1)
			i--
		} else if cc == "=" {
			if len(strs) > i {
				if len(strs) > i + 2 {
					strs[i-1] = strs[i-1] + "=" + strs[i+1]
					strs = splice(strs, i, 2)
					i--
				}
			}
		} else if strs[:1][0] == "=" {
			if len(strs) > i {
				if len(strs) > i + 1 {
					strs[i-1] = strs[i-1] + "=" + strs[i+1]
					strs = splice(strs, i, 1)
				}
			}
		} else if strs[len(strs)-1:][0] == "=" {
			if len(strs) > i + 2{
				if len(strs) > i {
					strs[i-1] = strs[i] + "=" + strs[i+1]
					strs = splice(strs, i+1, 1)
				}
			}
		}
	}

	for _, cc := range strs {
		tag.Pm = make(map[string]string)
		tmp := strings.Split(cc, "=")
		key := strings.TrimSpace(tmp[0])
		val := ""

		if len(tmp) > 1 {
			val = strings.TrimSpace(tmp[1])
		}

		if key == "*" {
			tag.Pm["*"] = ""
		}
		if val != "" {
			tag.Pm[key] = strings.ReplaceAll(val, "#", "=")
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
	tag.ifCount = 0

	switch tag.Name {
	case "if":
		ks.ifCount++
	case "elsif", "else":
		tag.ifCount = ks.ifCount
	case "endif":
		tag.ifCount = ks.ifCount
		ks.ifCount--
	}
	return tag
} // }}}

func splice(a []string, start, deleteCount int) []string {
	var result []string
	result = append(result, a[0:start]...)
	result = append(result, a[start+deleteCount:]...)
	return result
}
