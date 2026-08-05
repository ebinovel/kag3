package ebitengine

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/ebinovel/kag3"
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
// __jqSetImageSrc, if SetJQueryHooks has wired it in, is the one $(...)
// method actually implemented rather than stubbed: config.ks's own visual
// feedback for its volume/speed/skip buttons — "which one is currently
// selected" — works entirely through $(".someClass").attr("src", path),
// swapping a button's displayed graphic. Everything else $ can do stays a
// harmless no-op (see the browserShimJS doc comment above).
const browserShimJS = `
var $ = (function() {
	var methods = [
		"css", "empty", "remove", "html", "text", "val", "show", "hide",
		"addClass", "removeClass", "toggleClass", "hasClass", "on", "off",
		"trigger", "click", "bind", "unbind", "each", "find", "children",
		"parent", "closest", "append", "prepend", "before", "after", "width",
		"height", "offset", "position", "data", "stop", "delay", "animate",
		"fadeIn", "fadeOut", "fadeTo", "toggle", "is"
	];
	function stub(selector) {
		var api = { length: 0 };
		for (var i = 0; i < methods.length; i++) {
			(function(name) { api[name] = function() { return api; }; })(methods[i]);
		}
		api.attr = function(name, value) {
			if (arguments.length >= 2 && name === "src" && typeof __jqSetImageSrc === "function") {
				__jqSetImageSrc(selector, value);
			}
			return api;
		};
		return api;
	}
	return function(selector) { return stub(selector); };
})();
var window = { open: function() {} };
var TG = { config: {}, menu: {} };
`

// SetConfig populates TG.config with the actual values from resources/
// config.toml (via kag3.Config) rather than leaving TG.config an empty
// object. This matters beyond just avoiding "reading a property of
// undefined": config.ks's bootstrap [iscript] does e.g.
// `tf.current_bgm_vol = parseInt(TG.config.defaultBgmVolume);` and later
// feeds tf.current_bgm_vol into tag attributes like
// [bgmopt volume="&tf.current_bgm_vol"] — an empty TG.config would make
// that parseInt(undefined) => NaN, and a numeric tag attribute that fails
// strconv.Atoi is a *second*, later crash (dispatchTag's error return
// panics the whole coroutine — see execItem in macro.go). Booleans are
// exposed as the strings "true"/"false", matching how these bundled
// scripts compare them (e.g. `TG.config.unReadTextSkip != "true"`).
func (v *VM) SetConfig(cfg *kag3.Config) {
	config := v.rt.Get("TG").ToObject(v.rt).Get("config").ToObject(v.rt)
	config.Set("defaultBgmVolume", cfg.DefaultBgmVolume)
	config.Set("defaultSeVolume", cfg.DefaultSeVolume)
	config.Set("chSpeed", cfg.ChSpeed)
	config.Set("autoSpeed", cfg.AutoSpeed)
	config.Set("unReadTextSkip", boolToJSString(cfg.UnReadTextSkip))
	config.Set("alreadyReadTextColor", cfg.AlreadyReadTextColor)
	config.Set("autoRecordLabel", boolToJSString(cfg.AutoRecordLabel))
}

func boolToJSString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// SetMenuHooks wires TG.menu.doSave/loadGame/getSaveData — real Tyrano's
// own save/load API, called from the bundled (but in this example project,
// never actually invoked) [setsave]/[loading]/[saveinfo] macros in
// tyrano.ks — to this engine's actual save system (saveSlot/loadSlot/
// saveSlotInfo in tags_save.go/tags_uiscreens.go), so that if a script ever
// does call one of those macros it does something real instead of
// throwing "TG.menu.doSave is not a function". index is 0-based, matching
// real Tyrano's array-index convention (tf.array_save[mp.index]); this
// engine's slots are 1-based (see manualSaveSlot etc.), hence the +1.
func (v *VM) SetMenuHooks(r *Renderer) {
	menu := v.rt.Get("TG").ToObject(v.rt).Get("menu").ToObject(v.rt)
	menu.Set("doSave", func(call goja.FunctionCall) goja.Value {
		slot := int(call.Argument(0).ToInteger()) + 1
		if err := r.saveSlot(slot); err != nil {
			fmt.Printf("TG.menu.doSave(%d): %v\n", slot-1, err)
		}
		return goja.Undefined()
	})
	menu.Set("loadGame", func(call goja.FunctionCall) goja.Value {
		slot := int(call.Argument(0).ToInteger()) + 1
		if err := r.loadSlot(slot); err != nil {
			fmt.Printf("TG.menu.loadGame(%d): %v\n", slot-1, err)
		}
		return goja.Undefined()
	})
	menu.Set("getSaveData", func(call goja.FunctionCall) goja.Value {
		n := 5
		if r.manager != nil && r.manager.Config != nil && r.manager.Config.ConfigSaveSlotNum > 0 {
			n = r.manager.Config.ConfigSaveSlotNum
		}
		data := make([]map[string]interface{}, n)
		for i := 0; i < n; i++ {
			slot := i + 1
			exists, modTime := saveSlotInfo(r, slot)
			entry := map[string]interface{}{"title": "", "save_date": ""}
			if exists {
				entry["title"] = fmt.Sprintf("スロット%d", slot)
				entry["save_date"] = modTime.Format("2006/01/02 15:04:05")
			}
			data[i] = entry
		}
		result := v.rt.NewObject()
		result.Set("data", data)
		return result
	})
}

// SetJQueryHooks wires __jqSetImageSrc, the one $(...) call the
// browserShimJS stub actually forwards to Go — see its doc comment.
// selector is a bare class selector (".bgmvol_10"); real Tyrano renders
// [button name="a,b"] as an HTML element with class="a b", so the match
// here is against kag3.Button.Name's comma-separated tokens, the closest
// equivalent this engine has to an HTML class list.
func (v *VM) SetJQueryHooks(r *Renderer) {
	v.rt.Set("__jqSetImageSrc", func(selector, path string) {
		r.setButtonImageByClass(selector, path)
	})
}

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

// EvalButtonExp runs a clicked [button]'s preexp/exp, real Tyrano's pattern
// for running JS as a side effect of a click before Target/Role take effect
// (see config.ks's volume buttons and tyrano.ks's CG-gallery buttons).
// preexp, if set, runs first and its result is bound to a "preexp" variable
// exp can reference. Errors are swallowed, matching handleEval's tolerance
// for malformed expressions elsewhere in this file.
func (v *VM) EvalButtonExp(preexp, exp string) {
	if preexp != "" {
		if val, err := v.Eval(preexp); err == nil {
			v.rt.Set("preexp", val)
		}
	}
	if exp != "" {
		v.Eval(exp)
	}
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

// ExportTF snapshots "tf" (the per-scenario-screen variable namespace) the
// same way ExportF/ExportSF do, for [configsave] (tags_config.go) to
// persist a script's own settings variables (e.g. config.ks's
// tf.current_bgm_vol) to disk.
func (v *VM) ExportTF() map[string]interface{} {
	m, _ := v.tf.Export().(map[string]interface{})
	return m
}

// MergeTF sets vars onto the *existing* "tf" object instead of replacing it
// wholesale like RestoreF/RestoreSF do — [configload] (tags_config.go) runs
// after config.ks's own bootstrap [iscript] has already populated tf with
// unrelated working variables (tf.img_path, tf.btn_w, ...), and discarding
// those would break the rest of the script. Missing/malformed settings.json
// means an empty or partial vars map, which is a no-op merge, not an error.
func (v *VM) MergeTF(vars map[string]interface{}) {
	for k, val := range vars {
		v.tf.Set(k, val)
	}
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
