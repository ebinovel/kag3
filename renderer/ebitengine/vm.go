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
// macro-call frame via mpStack, pushed/popped through PushMPFrame.
type VM struct {
	rt        *goja.Runtime
	f, sf, tf *goja.Object
	mpStack   []*goja.Object
}

func newVM() *VM {
	v := &VM{rt: goja.New()}
	v.f = v.rt.NewObject()
	v.sf = v.rt.NewObject()
	v.tf = v.rt.NewObject()
	initSystemNamespace(v.rt, v.sf)
	initSystemNamespace(v.rt, v.tf)
	v.rt.Set("f", v.f)
	v.rt.Set("sf", v.sf)
	v.rt.Set("tf", v.tf)
	v.rt.Set("mp", v.rt.NewObject())
	if _, err := v.rt.RunString(browserShimJS); err != nil {
		// This is a fixed, hand-written shim, not user script — a failure
		// here is a bug in this engine, not something to hide from a
		// script author, but it also shouldn't be possible to hit in
		// practice, so there's nothing more useful to do than note it.
		panic("kag3: browser shim failed to install: " + err.Error())
	}
	return v
}

// browserShimJS defines $ and window as harmless no-ops. The bundled
// TyranoScript template (config.ks, scene1.ks, tyrano.ks — this project's
// own example resources) is the browser/jQuery-based official Tyrano
// distribution: its [iscript] blocks call things like
// $(".layer_camera").empty() to manipulate an HTML page that simply
// doesn't exist in this canvas-based engine, and window.open(url) to open
// a link in a new browser tab. Without these, the first such call throws
// "ReferenceError: $/window is not defined" and kills the whole renderer —
// the same category of problem tf.system/sf.system (see
// initSystemNamespace) solves for engine-reserved variables, just for
// browser globals instead. $(...) returns a chainable stub whose methods
// are no-ops and report an empty result (.length = 0), which is a safe
// default: this engine's own system screens (menu/save/load — see
// tags_uiscreens.go/tags_save.go) are drawn independently of whatever a
// legacy script's jQuery calls would have done to a DOM that was never
// there to begin with.
const browserShimJS = `
var $ = (function() {
	var methods = [
		"attr", "css", "empty", "remove", "html", "text", "val", "show", "hide",
		"addClass", "removeClass", "toggleClass", "hasClass", "on", "off",
		"trigger", "click", "bind", "unbind", "each", "find", "children",
		"parent", "closest", "append", "prepend", "before", "after", "width",
		"height", "offset", "position", "data", "stop", "delay", "animate",
		"fadeIn", "fadeOut", "fadeTo", "toggle", "is"
	];
	function stub() {
		var api = { length: 0 };
		for (var i = 0; i < methods.length; i++) {
			(function(name) { api[name] = function() { return api; }; })(methods[i]);
		}
		return api;
	}
	return function() { return stub(); };
})();
var window = { open: function() {} };
`

// initSystemNamespace sets ns.system to a fresh object with a "backlog"
// array, matching real Tyrano's own tf.system/sf.system — scripts (e.g. the
// bundled replay.ks/config.ks templates) read/write tf.system.flag_replay
// and call tf.system.backlog.pop() unconditionally, assuming the engine
// already created these before any script ran; without this they'd hit a
// "Cannot convert undefined or null to object" TypeError the first time
// they touch tf.system.
func initSystemNamespace(rt *goja.Runtime, ns *goja.Object) {
	system := rt.NewObject()
	system.Set("backlog", rt.NewArray())
	ns.Set("system", system)
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

// PushMPFrame creates a new "mp" object populated from pm and makes it the
// active mp for subsequent evaluations, for the duration of a macro call.
// The returned pop function restores the previous mp frame (or a fresh
// empty one if none) and must be called once the macro body finishes.
func (v *VM) PushMPFrame(pm map[string]string) (pop func()) {
	frame := v.rt.NewObject()
	for k, val := range pm {
		frame.Set(k, val)
	}
	v.mpStack = append(v.mpStack, frame)
	v.rt.Set("mp", frame)
	return func() {
		v.mpStack = v.mpStack[:len(v.mpStack)-1]
		if n := len(v.mpStack); n > 0 {
			v.rt.Set("mp", v.mpStack[n-1])
		} else {
			v.rt.Set("mp", v.rt.NewObject())
		}
	}
}

// ClearF replaces "f" (the game-variable namespace) with a fresh empty
// object, for [clearvar] with no name= attribute.
func (v *VM) ClearF() {
	v.f = v.rt.NewObject()
	v.rt.Set("f", v.f)
}

// ClearSF replaces "sf" (the system-variable namespace) with a fresh empty
// object, for [clearsysvar]. sf.system is reinitialized too (see
// initSystemNamespace) so a script relying on it doesn't crash just because
// it happened to run after a [clearsysvar].
func (v *VM) ClearSF() {
	v.sf = v.rt.NewObject()
	initSystemNamespace(v.rt, v.sf)
	v.rt.Set("sf", v.sf)
}

// ExportF/ExportSF snapshot "f"/"sf" as plain Go maps so save/load (see
// tags_save.go) can round-trip them through JSON. goja.Object.Export()
// walks the whole object graph, so this only works for JSON-shaped data
// (strings/numbers/bools/nested maps/slices) — a script that stashes a
// function or other exotic value in f/sf will lose it on save, same
// limitation as any other KAG engine's save file.
func (v *VM) ExportF() map[string]interface{} {
	m, _ := v.f.Export().(map[string]interface{})
	return m
}

func (v *VM) ExportSF() map[string]interface{} {
	m, _ := v.sf.Export().(map[string]interface{})
	return m
}

// RestoreF/RestoreSF replace "f"/"sf" with a fresh object populated from a
// previously-exported map, for [load]/[rollback].
func (v *VM) RestoreF(vars map[string]interface{}) {
	obj := v.rt.NewObject()
	for k, val := range vars {
		obj.Set(k, val)
	}
	v.f = obj
	v.rt.Set("f", obj)
}

func (v *VM) RestoreSF(vars map[string]interface{}) {
	obj := v.rt.NewObject()
	for k, val := range vars {
		obj.Set(k, val)
	}
	// A save made before sf.system existed (or one where a script deleted
	// it) shouldn't reintroduce the same crash this namespace exists to
	// avoid — see initSystemNamespace. A missing property comes back as Go
	// nil here (goja.Object.Get doesn't return the _undefined sentinel for
	// keys that were never set at all), so check for both.
	if system := obj.Get("system"); system == nil || goja.IsUndefined(system) {
		initSystemNamespace(v.rt, obj)
	}
	v.sf = obj
	v.rt.Set("sf", obj)
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
