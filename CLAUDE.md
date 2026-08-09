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
go build -o /tmp/kag3example ./example   # drop the .exe suffix outside Windows
```

**The rest of this section (WinAppDriver, `e2e/`) is Windows-only** — both depend on real Win32 APIs
(`e2e/driver/window_windows.go`'s direct click/key calls) and the Windows Application Driver itself;
neither exists on macOS/Linux, there's no cross-platform equivalent to fall back to, and no amount of
retrying will make them reachable there. On a non-Windows checkout, verify UI changes with a unit
test that drives the same code path directly (see the package-level globals note under "Execution
model" below) and say explicitly that end-to-end/visual confirmation needs a Windows machine, rather
than attempting `e2e/` or claiming it passed.

On the Windows dev machine this was originally written on, a real desktop and a WinAppDriver instance
are already running (reachable at `http://127.0.0.1:4723/status` — check with the PowerShell tool if
in doubt) — the built binary can actually be launched and its window driven end-to-end, including
through the `e2e/` suite (see "E2E tests" below). Prefer writing a unit test first for fast,
deterministic coverage, but don't claim a UI fix works without actually verifying it there — run the
built binary and/or the relevant `e2e/` test rather than assuming.

## Repository layout gotcha

`example/`, `example2/`, and `resources/system/images/` are excluded via `.git/info/exclude` (not
`.gitignore`), so they exist on disk but are **not tracked by git** — `git status`/`git diff` will
never show edits made there. This is intentional (keeps the large bundled sample game and system
art out of history), not an oversight. When a fix requires editing a `.ks` file under `example/`,
say so explicitly — it won't show up in a commit. `example/` is committed as a whole once its
placeholder/original assets are ready, not gradually — see `.github/workflows/build-android.yml`'s
own comment for how that's reflected on the CI side in the meantime.

`tmp_genplaceholders/` (a tracked Go program at the repo root, `package main`) generates the
placeholder art `example/game/resources/` and `resources/system/images/` currently use in place of
TyranoScript's own copyrighted sample assets — run it (`go run ./tmp_genplaceholders`) to regenerate
that art from scratch on a fresh checkout; it deliberately avoids importing ebitengine (whose `init()`
touches the windowing system unconditionally) so it also runs headless in CI. `example_tyrano_official_backup/`
(untracked, ~37MB, includes a prebuilt Windows `.exe`) is a backup of the pre-refactor `example/`
layout with the real official TyranoScript assets — not referenced by any build, safe to leave behind
entirely when copying this checkout elsewhere (the `.exe` inside it won't run on macOS/Linux anyway).

## Mobile/wasm/macOS/Linux/Windows builds (`example/mobile`, `example/wasm`, `example/macos`, `example/linux`, `example/windows`)

`example/game` (the `Game` struct implementing `ebiten.Game`) is deliberately its own package, not
`package main`, specifically so it can be shared across every entrypoint below — `ebitenmobile bind`
refuses to bind a `package main` target, so this split was a prerequisite for Android/iOS at all, not
just a style choice. All of these live under the git-excluded `example/` (see "Repository
layout gotcha" above) and are themselves platform-gated, so treat everything in this section as
unverified until it's actually been run on the platform it targets — each subsection below says
plainly what has and hasn't been confirmed working:

- **Android** (`example/mobile/androidapp/build-android.ps1`, PowerShell) — `ebitenmobile bind` then
  `gradlew assembleDebug`, optionally `-Install` to `adb install` it. **Confirmed working end-to-end**
  on a real device, including a from-scratch NDK/toolchain setup (see the script's own comments for
  the Windows-specific NDK command-line-length workaround it applies — likely unnecessary, but
  unverified, on macOS/Linux, where the NDK's wrapper scripts don't hit the same Windows batch-file
  limit). `example/mobile/storage_android.go` (JNI-backed save/settings bridge) and
  `renderer/ebitengine/vm.go`'s `MobilePlatform`/`TG.config.isMobile` (lets `config.ks` hide UI that
  only makes sense on desktop, e.g. the window/fullscreen toggle) are the Android-specific pieces to
  know about if save/load or the config screen misbehave only on Android.
- **iOS** (`example/mobile/build-ios.sh`, `example/mobile/ios/README.md`) — needs a Mac with a full
  Xcode install; this repository has never had one available, so **nothing here has actually been
  run** — the script and guide are a best-effort starting point written by reasoning from the Android
  side and ebitengine's public docs, not a verified recipe. Expect the first real attempt to surface
  issues (wrong generated class/API names, storage persistence that may or may not need its own
  bridge the way Android's does — see the README's own note on trying the default `os.UserHomeDir()`
  fallback first) and to need iteration, the same way Android did.
- **wasm** (`example/wasm/build-wasm.ps1`, PowerShell) — plain `GOOS=js GOARCH=wasm go build`, no
  special toolchain. `-Publish` also builds a stripped copy into `../ebinovel.github.io/play/` (a
  sibling repo — see that repo's own README) for the public browser demo. `renderer/ebitengine/storage_js.go`
  (`//go:build js && wasm`, a `localStorage`-backed save/settings bridge — `syscall/js` only, no
  external deps) is confirmed to compile (`GOOS=js GOARCH=wasm go build ./renderer/...`) but has never
  actually been run in a browser from this environment (no browser/JS runtime available here) — treat
  its localStorage read/write behavior as unverified until confirmed in a real browser.
- **macOS** (`example/macos/build-macos.sh`, bash) — wraps the plain desktop `go build ./example`
  binary (the same one Windows/Linux run directly, no bundle) in a real `.app`: a universal
  (Intel + Apple Silicon) binary via `lipo`, plus a generated `Info.plist`. **Confirmed working
  end-to-end** on a real Mac (Apple Silicon host): both architecture slices build, `lipo` produces a
  genuine universal binary, the `.app` launches (ad-hoc `codesign --force --deep --sign -` first,
  per the script's own trailing output — otherwise Gatekeeper blocks even a same-machine launch) and
  runs its render loop without crashing. The one real bug this surfaced, now fixed in the script:
  building the **amd64** slice from an Apple Silicon host without `CGO_ENABLED=1` explicit on that
  `go build` invocation silently compiles against GLFW's non-cgo stub instead of erroring — Go drops
  cgo by default for any `GOARCH` that doesn't match the host, and ebitengine's macOS backend needs
  its GLFW cgo bindings, so the symptom was `undefined: glfw.Window`/`undefined: glfw.WindowHint`
  failing to *link*, not the "dies looking for `clang`" failure `GOOS=darwin` cross-compilation from
  a non-Mac host hits (still accurate below — a different failure mode, same underlying cgo
  requirement). Both `go build` lines in the script now set `CGO_ENABLED=1` explicitly. No app icon
  is wired up yet (`CFBundleIconFile` is commented out in the generated plist); running the bundle on
  any *other* Mac still needs at least its own ad-hoc `codesign`, or for real distribution, a paid
  Apple Developer ID and notarization. What's *not* confirmed from here: `screencapture`/window
  screenshots aren't available in this sandbox (no Screen Recording permission grantable
  non-interactively) — visual correctness (does the title screen actually render right) was verified
  only by process health (stays alive, accumulates CPU/render-loop time, no crash log), not a
  screenshot.
  Separately, from a non-Mac host: `GOOS=darwin go build` cannot cross-compile at all — ebitengine's
  macOS backend is cgo (GLFW/Cocoa bindings), so it fails outright without a macOS C toolchain on
  `PATH` (dies looking for `clang`; forcing `CGO_ENABLED=0` just trades that for a *different* set of
  undefined GLFW symbols instead of a working binary) — so this script must run on an actual Mac,
  unlike the two `.ps1` scripts below.
- **Linux** (`example/linux/build-linux.sh`, bash) — wraps the same plain `go build ./example`
  binary in a freedesktop.org `.desktop` entry (Desktop Entry spec), plus an `install.sh` that
  copies both into `~/.local/bin`/`~/.local/share/applications` (no sudo, nothing outside `$HOME`
  touched) so the game shows up in a normal desktop environment's app launcher instead of only being
  runnable from a terminal. **Confirmed working end-to-end** via WSL2 + WSLg on a Windows host (no
  native Linux desktop available in this project's usual dev environment — same caveat as the macOS
  bullet above, but here a real screenshot *was* obtainable, via the Windows host's own screen
  capture against WSLg's RDP-backed window): `go build` succeeds, the binary launches and renders
  the title screen correctly (Japanese font included), `desktop-file-validate` reports zero
  errors/warnings on the generated `.desktop`, and `install.sh` correctly rewrites the placeholder
  `Exec=__EXEC_PATH__` to the real installed path before copying both files into place. Requires the
  same native dev libraries ebitengine's Linux backend needs at build time (X11/GLFW + ALSA headers)
  — not present by default even on a fresh Ubuntu WSL image: `sudo apt-get install libgl1-mesa-dev
  libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev pkg-config`.
  No app icon is set — `Icon=applications-games` falls back to a generic icon from whatever theme is
  active rather than a bundled file, the same "not done yet" state as macOS's commented-out
  `CFBundleIconFile`.
- **Windows** (`example/windows/build-windows-installer.ps1` + `installer.iss`, PowerShell + Inno
  Setup 6) — wraps the same `go build ./example` binary, built here with `-ldflags -H=windowsgui`
  (this machine's own global convention for Go GUI apps — see `~/.claude/CLAUDE.md`; without it a
  console window pops up alongside the game window) in a proper installer: Start Menu shortcut,
  optional desktop icon, Add/Remove Programs entry with a working uninstaller. **Confirmed working
  end-to-end** on this dev machine: build → `ISCC` compile → silent test install (`/VERYSILENT
  /DIR=...` into a scratch temp dir) → launch (window title and responsiveness confirmed) → silent
  uninstall — all succeeded, and the install dir/Start Menu folder/registry entry were all
  confirmed gone afterward. Requires Inno Setup 6 (`ISCC.exe`) — already installed on this machine
  at its default location; `$env:ISCC_PATH` overrides the lookup if it's somewhere else, and the
  script throws with a download link if it's missing entirely. Two real issues this surfaced, now
  fixed in `installer.iss`:
  - `ArchitecturesInstallIn64BitMode=x64compatible` (the more correct modern value, also covering
    ARM64 via x64 emulation) isn't recognized by this machine's Inno Setup 6.2.2 — reverted to the
    older `x64`; worth revisiting if a newer Inno Setup is ever installed here.
  - The default `PrivilegesRequired=admin` install failed outright in this environment (exit code
    2, "Access to the path ...\msdtadmin is denied") — this looks like a sandboxed/non-interactive
    session that can't satisfy a UAC elevation prompt. Switched to `PrivilegesRequired=lowest`
    (per-user install under `{localappdata}\Programs`, no admin/UAC needed) — arguably the more
    correct choice anyway for a single-player game with no reason to touch shared system
    directories.

Both `.ps1` scripts need PowerShell (`pwsh`) to run as-is; on macOS that means either installing
`pwsh` or translating the handful of commands inside them manually — they're short and mostly
inline comments explaining *why* each step exists, worth reading even if not run verbatim.

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

## E2E tests (`e2e/`) — Windows-only

`e2e/` is a **separate Go module** (its own `go.mod`) — not part of `go test ./...` from the repo
root. It builds `example/` into a real `.exe` and drives the actual window via
[WinAppDriver](https://github.com/microsoft/WinAppDriver) (window attach + screenshots only — its
own click/key endpoints don't work reliably here, so those go through direct Win32 calls instead,
`e2e/driver/window_windows.go`) to catch bugs headless unit tests can't: save/load round-trips,
title-return state leaks, same-process quickload. **This entire suite is Windows-only** — WinAppDriver
and the direct Win32 calls it's built on have no macOS/Linux equivalent, so on those platforms `e2e/`
cannot run at all (not "might be flaky," genuinely inapplicable); rely on unit tests plus manual
confirmation on an actual Windows machine for anything this would have covered.

On the Windows dev machine this was written on, there's a real Windows desktop and WinAppDriver is
already running — `e2e/` is runnable there (via the Bash tool, same as any other `go test`). Before
running it, rebuild the test binary whenever `example/` or anything under its embedded `resources/`
changes —
`example/game/resources.go`'s `//go:embed resources` bakes the whole resource tree into the binary
at build time, so editing files under `example/game/resources/` (including the git-excluded ones,
see "Repository layout gotcha" above) has no effect on `e2e/` until rebuilt:

```sh
go build -o e2e/testdata/kag3example.exe ./example
cd e2e && go test ./... -v
```

If a given run reports WinAppDriver unreachable, or the environment genuinely lacks a real desktop,
fall back to handing off to the user or saying explicitly that the change needs manual confirmation.
Full setup/known caveats are in `e2e/README.md`.

kag3 itself has exactly two env var hooks for it, both no-ops unless set: `KAG3_SAVE_DIR` (absolute
override for `saveDir()`, `tags_save.go` — lets an external test process sandbox saves the way
`saveBaseDirOverride` lets an in-package Go test do) and `KAG3_E2E_FAST` (forces `textNoWait = true`
at startup via an `init()`, `tags_message.go` — skips glyph-by-glyph text reveal, the biggest E2E
wall-clock cost).
