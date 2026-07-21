package ebitengine

import (
	"fmt"
	"io/fs"
)

func init() {
	register("iscript", handleIScript)
	register("endscript", handleNoop)
	register("eval", handleEval)
	register("emb", handleEmb)
	register("trace", handleTrace)
	register("clearvar", handleClearVar)
	register("clearsysvar", handleClearSysVar)
	register("erasemacro", handleEraseMacro)
	register("loadjs", handleLoadJS)
	register("plugin", handlePlugin)
}

// handleIScript runs the raw JavaScript body the parser collected between
// [iscript] and its matching [endscript] (kag3.TagObject.Body).
func handleIScript(ctx *tagCtx) error {
	_, err := ctx.r.vm.Eval(ctx.tag.Body)
	return err
}

// handleEval runs a single JS expression/statement given via exp=, for its
// side effects (e.g. [eval exp="f.hoge = 5"]).
func handleEval(ctx *tagCtx) error {
	_, err := ctx.r.vm.Eval(ctx.tag.Pm["exp"])
	return err
}

// handleEmb evaluates exp= and inserts the stringified result inline into
// the current line's text, the same way [ruby]/[font] append supplementary
// segments for the surrounding TextObjects to pick up.
func handleEmb(ctx *tagCtx) error {
	r := ctx.r
	val := r.vm.EvalString(ctx.tag.Pm["exp"])
	r.texts[ctx.tag.Line] = append(r.texts[ctx.tag.Line], Text{Text: val})
	return nil
}

func handleTrace(ctx *tagCtx) error {
	val := ctx.r.vm.EvalString(ctx.tag.Pm["exp"])
	fmt.Println("[trace]", val)
	return nil
}

// handleClearVar clears a single f.<name> if name= is given, otherwise
// resets the whole f namespace.
func handleClearVar(ctx *tagCtx) error {
	v := ctx.r.vm
	if name := ctx.tag.Pm["name"]; name != "" {
		v.f.Delete(name)
		return nil
	}
	v.ClearF()
	return nil
}

func handleClearSysVar(ctx *tagCtx) error {
	ctx.r.vm.ClearSF()
	return nil
}

func handleEraseMacro(ctx *tagCtx) error {
	delete(ctx.r.manager.Macros, ctx.tag.Pm["name"])
	return nil
}

// loadJSFile reads storage (default folder "senarios", the one fs key
// external script files naturally live alongside) and evals it as one JS
// source, for [loadjs] and [plugin storage=...].
func (r *Renderer) loadJSFile(folder, storage string) error {
	if folder == "" {
		folder = "senarios"
	}
	b, err := fs.ReadFile(r.fses[folder], storage)
	if err != nil {
		return err
	}
	_, err = r.vm.Eval(string(b))
	return err
}

func handleLoadJS(ctx *tagCtx) error {
	storage, ok := ctx.tag.Pm["storage"]
	if !ok {
		return nil
	}
	return ctx.r.loadJSFile(ctx.tag.Pm["folder"], storage)
}

// loadedPlugins tracks plugin names [plugin name=...] has registered.
// There's no real plugin/extension-point system for them to hook into yet,
// so this is honest bookkeeping rather than a functioning plugin loader —
// except when storage= is also given, which real Tyrano plugins commonly
// are (a JS file), in which case it's really just [loadjs] under another
// name.
var loadedPlugins = map[string]bool{}

func handlePlugin(ctx *tagCtx) error {
	if name := ctx.tag.Pm["name"]; name != "" {
		loadedPlugins[name] = true
	}
	if storage, ok := ctx.tag.Pm["storage"]; ok {
		return ctx.r.loadJSFile(ctx.tag.Pm["folder"], storage)
	}
	return nil
}
