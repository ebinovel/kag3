package ebitengine

import (
	"strings"

	"github.com/dop251/goja"
)

// VM wraps a goja.Runtime and exposes the Tyrano-style global variable
// namespaces (f/sf/tf/mp) used by [iscript]/[eval]/[emb] and by the
// "&expr" / "%name" attribute-expansion syntax inside tag arguments.
//
// f/sf/tf live for the Renderer's lifetime (sf is the save-file namespace,
// persisted across script loads once save/load exists). mp is swapped per
// macro-call frame; until macro expansion exists it stays a single empty
// object.
type VM struct {
	rt        *goja.Runtime
	f, sf, tf *goja.Object
}

func newVM() *VM {
	v := &VM{rt: goja.New()}
	v.f = v.rt.NewObject()
	v.sf = v.rt.NewObject()
	v.tf = v.rt.NewObject()
	v.rt.Set("f", v.f)
	v.rt.Set("sf", v.sf)
	v.rt.Set("tf", v.tf)
	v.rt.Set("mp", v.rt.NewObject())
	return v
}

// Eval runs a JavaScript expression or statement and returns its value.
func (v *VM) Eval(src string) (goja.Value, error) {
	return v.rt.RunString(src)
}

// EvalBool evaluates src as a boolean condition, e.g. [if exp="f.hoge > 3"].
// An evaluation error is treated as false rather than propagated, matching
// Tyrano's tolerant handling of malformed expressions.
func (v *VM) EvalBool(src string) bool {
	val, err := v.Eval(src)
	if err != nil {
		return false
	}
	return val.ToBoolean()
}

// EvalString evaluates src and stringifies the result, e.g.
// [emb exp="f.name"]. An evaluation error yields an empty string.
func (v *VM) EvalString(src string) string {
	val, err := v.Eval(src)
	if err != nil {
		return ""
	}
	return val.String()
}

// expandParams resolves Tyrano's "&expression" and "%name" / "%name|default"
// syntax inside tag argument values before a handler sees them. Values
// without either prefix pass through unchanged.
func (v *VM) expandParams(pm map[string]string) map[string]string {
	if pm == nil {
		return pm
	}
	out := make(map[string]string, len(pm))
	for k, s := range pm {
		switch {
		case strings.HasPrefix(s, "&"):
			out[k] = v.EvalString(s[1:])
		case strings.HasPrefix(s, "%"):
			name, def, hasDefault := strings.Cut(s[1:], "|")
			got := v.EvalString("mp." + name)
			if (got == "" || got == "undefined") && hasDefault {
				got = def
			}
			out[k] = got
		default:
			out[k] = s
		}
	}
	return out
}
