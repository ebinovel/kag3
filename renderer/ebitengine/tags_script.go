package ebitengine

import "fmt"

func init() {
	register("iscript", handleIScript)
	register("endscript", handleNoop)
	register("eval", handleEval)
	register("emb", handleEmb)
	register("trace", handleTrace)
	register("clearvar", handleClearVar)
	register("clearsysvar", handleClearSysVar)
	register("erasemacro", handleEraseMacro)
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
