# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

kag3 is a Go reimplementation of a TyranoScript-compatible visual novel engine, rendered with
[ebitengine](https://ebitengine.org/). It parses `.ks` scenario files (TyranoScript's own tag syntax)
and executes them against an ebitengine `Game`. The goal is for real-world TyranoScript projects (the
bundled `example`/`example2` are full copies of the official TyranoScript sample game) to run mostly
unmodified.

## Commands

```sh
go build ./...          # build everything
go vet ./...             # static checks
go test ./...             # run the full test suite
go test ./... -count=1    # same, bypassing the test cache (do this after touching package-level state — see below)
go test ./renderer/ebitengine/... -run TestName -v   # run a single test
gofmt -l <file>...        # check formatting; renderer.go has long-standing unrelated
                           # drift in one var block — don't "fix" it as a side effect of
                           # unrelated edits, gofmt only the lines you actually touched
```

To smoke-test a change end-to-end, build and run `./example` (it opens a real window):

```sh
go build -o /tmp/kag3example.exe ./example
```

There is no way to script mouse/keyboard input against a running ebitengine window in *this*
sandboxed dev environment — interactive verification of UI changes has to be done by the user, or
by the `e2e/` WinAppDriver suite (see "E2E tests" below), which needs a real Windows desktop and
can't run here either. Prefer writing a unit test that drives the same code path directly (see the
package-level globals note under "Execution model" below) over claiming a UI fix works without
having verified it either way.

## Repository layout gotcha

`example/`, `example2/`, and `resources/system/images/` are excluded via `.git/info/exclude` (not
`.gitignore`), so they exist on disk but are **not tracked by git** — `git status`/`git diff` will
never show edits made there. This is intentional (keeps the large bundled sample game and system
art out of history), not an oversight. When a fix requires editing a `.ks` file under `example/`,
say so explicitly — it won't show up in a commit.

## Architecture

### Two packages

- **`kag3` (root package)** — script-format concerns: `parser.go` (the `.ks` tokenizer, a single
  `*KS` instance per `Manager`, tracks `ifCount`/`isInScript` across the whole parse), `manager.go`
  (`Manager.LoadScript` reads from an `fs.FS` map keyed by resource kind — `"senarios"`, `"images"`,
  `"bgms"`, `"ses"`, `"system/images"`, `"fonts"`, `"resources"` — and re-parses on every load;
  `Macros` persist across `LoadScript` calls so macros defined in one file stay callable from later
  ones), `config.go` (`Config`, loaded from `resources/config.toml`, falling back to
  `LoadDefault()`), `kag3.go` (shared data types: `TagObject`, `TextObject`, `Background`,
  `Character`, `CharaShow`, `Image`, etc.), `macro.go` (`extractMacros`/`reindexLabels`).
- **`renderer/ebitengine`** — the actual engine: implements every tag against ebitengine and drives
  the `ebiten.Game` loop (`Renderer.Update`/`Draw` in `renderer.go`).

### Tag dispatch: a registry, not a switch

`dispatch.go` defines `tagHandler func(*tagCtx) error` and a global `handlers map[string]tagHandler`.
Each `tags_<category>.go` file (`tags_audio.go`, `tags_character.go`, `tags_background.go`,
`tags_save.go`, `tags_uiscreens.go`, ...) registers its own tags in an `init()`. `dispatchTag` looks
up the handler; if none exists it tries a user-defined `[macro]` of the same name; if neither
matches, it **logs** (`未実装のタグです: ...`) and continues rather than crashing — an unimplemented
tag must never take down the whole renderer. Follow this pattern for anything new: add a
`tags_*.go` file (or extend an existing one) with an `init()` `register()` call, not a new dispatch
mechanism.

### Execution model: one coroutine, plus a pile of package-level globals

The scenario runs on a single `coro.Coro` (`github.com/eihigh/coro`), stepped once per `Update()`
via `co.Next()`. Blocking tags (`[s]`, `[wait]`, `[playbgm ... wait]`, etc.) call
`ctx.y.Until(preYield, predicate)`, which repeatedly yields until `predicate()` is true — meaning a
tag can only be unblocked by whatever flag its own predicate reads becoming true, checked fresh on
every subsequent `Next()` call. Two flags matter almost everywhere:

- `isJump`/`jumpIndex` — the *only* way to redirect execution from outside the coroutine (button
  clicks, save/load, "return to title" all set these instead of touching `r.scripts` position
  directly). `initScript`'s outer loop is a `for { if isJump {...}; if i >= len(r.scripts) { break }; ... }`
  (deliberately *not* a bounds-checked `for i := 0; i < len(r.scripts); i++` — that shape checks the
  bound before the body's `isJump` check, so a jump into a *shorter* script than wherever `i`
  currently was can silently end the loop before the jump ever gets read).
  See `dispatchTag`'s `currentScriptIndex` tracking for how code outside the coroutine learns "where
  are we right now" to build a resumable position.
- `isWait` — "the current line has finished revealing, waiting for a click to advance." A bare
  `TextObject` blocks on this directly (`y.Until(false, func() bool { return isWait })` in
  `execItem`, macro.go). `[p]` (`handleP`, tags_text.go) blocks on something finer-grained instead —
  `isTextEndedOrJumped` (state.go) — built from `isTextEnded = oldTick+3>=tick && isWait`: a real
  click resets `oldTick = tick` (Update()'s `doNext()` branch), giving a 3-`tick` grace window
  measured from *right now* so a short burst of already-queued tags right after the click doesn't
  re-block immediately. `tick` is not "frames elapsed" — it's "successful `co.Next()` calls", and a
  blocked `y.Until` calling `y()` on every failed predicate check counts too (Update()'s
  `for i:=0;i<1000;...` loop drives those exactly like real tag advancement), so `tick` keeps
  climbing by up to 1000/frame even while genuinely stuck. Any code that force-jumps elsewhere (see
  `goToTitle`, `applySaveData`) must still set `isWait = true` (not `false`) — a bare `TextObject`
  wait only reads that — but for a screen resting on a blocked `[p]`, two more things follow from
  the tick mechanics above, both the subject of real regressions:
  - The jump target's *own* first `[p]`, if it has one, must not spuriously satisfy `isTextEnded`
    from `oldTick` simply having been "recent enough" at jump time — `goToTitle`/`applySaveData`
    additionally set `oldTick = tick - 4` so the destination always starts genuinely blocked,
    requiring a real click rather than skipping a line unclicked.
  - `isJump` alone does **not** unblock a coroutine currently nested inside `handleP`'s `y.Until` —
    the outer `initScript` loop that actually reads `isJump` (see below) is further up the *same*
    call stack, blocked until this `y.Until` call returns on its own. Setting `isJump = true` while
    `[p]` is blocking (e.g. clicking role="title" mid-dialogue) does nothing by itself; without also
    satisfying `isTextEnded`, it just sits there until some *unrelated* later click accidentally
    lands on it. `isTextEndedOrJumped` closes this by also releasing on a pending `isJump` — safe
    for the destination's own `[p]`, since the outer loop consumes (resets false) `isJump` before
    the destination's first tag ever runs.

Almost all renderer state — `bg`, `viewCharas`, `charas`, `textPosition`, `buttons`, `links`,
`glinks`, `imgs`, `backlog`, `activeDialog`, and dozens more — lives in **package-level `var`s in
`renderer/ebitengine`**, not on the `Renderer` struct. This is why tests must explicitly reset the
globals they touch (and why a test can fail from a *different* test's leftover state, e.g. a stale
`lastSnapshot` or `bg.Storage` tripping a later save/load test) — grep existing `_test.go` files for
the reset pattern before adding a new one, and run the specific new test alone as well as the full
suite to catch cross-test pollution.

### goja (JS) embedding

`vm.go` wraps a `goja.Runtime` per `Renderer`, exposing Tyrano's `f`/`sf`/`tf`/`mp` variable
namespaces and the `&expr`/`%name` attribute-expansion syntax. `tf.system`/`sf.system` and a
`browserShimJS` (`$` as a no-op chainable jQuery stub, `window.open` as a no-op, `TG.config`/
`TG.menu` wired to real engine config/save state) exist specifically because the bundled sample
project is the genuine browser/jQuery-based TyranoScript distribution — its `[iscript]` blocks
assume a browser DOM and an engine-provided `TG` global that plain goja doesn't have. If a bundled
script throws `ReferenceError: X is not defined`, the fix is almost always adding `X` to this shim
layer, not editing the script.

### Modal UI overlays

Save/load slot picker, quick menu, backlog viewer, `[edit]` text input, and button-triggered confirm
dialogs (`confirmGoToTitle`) are all package-level state machines that must **freeze** the coroutine
while open — `anyModalActive()` (in `tags_uiscreens.go`) is the single source of truth `Update()`
checks before stepping `co.Next()`, so any new modal needs to be added there too. A `[dialog]` *tag*
is the odd one out: it blocks the coroutine directly via its own `y.Until`, so it's deliberately
excluded from `anyModalActive()` — only dialogs opened from a button click (`OnConfirm` set) need
the external freeze.

## E2E tests (`e2e/`)

`e2e/` is a **separate Go module** (its own `go.mod`) — not part of `go test ./...` from the repo
root, and not runnable from this sandboxed dev environment. It builds `example/` into a real `.exe`
and drives the actual window via [WinAppDriver](https://github.com/microsoft/WinAppDriver) (window
attach + screenshots only — its own click/key endpoints don't work reliably here, so those go
through direct Win32 calls instead, `e2e/driver/window_windows.go`) to catch bugs headless unit
tests can't: save/load round-trips, title-return state leaks, same-process quickload. Needs a real
Windows desktop with WinAppDriver installed — hand off to the user, or say explicitly that an `e2e/`
change needs manual confirmation. Full setup/known caveats are in `e2e/README.md`.

kag3 itself has exactly two env var hooks for it, both no-ops unless set: `KAG3_SAVE_DIR` (absolute
override for `saveDir()`, `tags_save.go` — lets an external test process sandbox saves the way
`saveBaseDirOverride` lets an in-package Go test do) and `KAG3_E2E_FAST` (forces `textNoWait = true`
at startup via an `init()`, `tags_message.go` — skips glyph-by-glyph text reveal, the biggest E2E
wall-clock cost).
