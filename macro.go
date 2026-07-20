package kag3

// extractMacros scans senario for [macro name="X"]...[endmacro] blocks,
// records each one's body into macros, and returns senario with those
// blocks (markers and body alike) removed. Definitions accumulate across
// LoadScript calls (via the caller-supplied macros map) so macros defined
// in one .ks file remain callable after later scripts are loaded.
func extractMacros(senario Senario, macros map[string]*Macro) Senario {
	result := make(Senario, 0, len(senario))
	i := 0
	for i < len(senario) {
		tag, ok := senario[i].(TagObject)
		if !ok || tag.Name != "macro" {
			result = append(result, senario[i])
			i++
			continue
		}
		name := tag.Pm["name"]
		var body []any
		i++
		for i < len(senario) {
			if end, ok := senario[i].(TagObject); ok && end.Name == "endmacro" {
				i++
				break
			}
			body = append(body, senario[i])
			i++
		}
		if name != "" {
			macros[name] = &Macro{Name: name, Body: body}
		}
	}
	return result
}

// reindexLabels rebuilds a label table from a Senario's LabelObject
// entries. Needed after extractMacros removes items, since a LabelInfo's
// Index computed during parsing (len(result) at the time) no longer
// matches the position of anything after a removed macro block.
func reindexLabels(senario Senario) map[string]LabelInfo {
	labels := make(map[string]LabelInfo)
	for idx, item := range senario {
		if lbl, ok := item.(LabelObject); ok {
			info := lbl.Info
			info.Index = idx
			labels[info.Name] = info
		}
	}
	return labels
}
