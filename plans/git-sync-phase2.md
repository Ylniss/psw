# Phase 2 detail: Pull/push spinner

_Last updated: 2026-05-08 — commit `54366f3` (pre-Phase 2). Sibling: `git-sync.md` (parent), `git-sync-phase1.md` (prior phase, archived)._

## Status: **Delivered** (uncommitted)

1 new ui unit test + 48 integration tests + 16 merge unit subtests green. `go vet` and `gofmt` clean. Two cleanup passes applied after first pass: modern-bubbletea-idiom review (`prog.Send` + closed-channel signaling, dropped `WithoutSignalHandler`, helper extraction) and naming/comment review (three doc tightens, `prog` → `program` rename).

### Deviations from this plan

- **`runGitNetworkSpinner` helper added.** Original plan had inline closures at both wrap sites (`GitFetch`, `GitPush`) — ~6 lines each, identical shape. Deduplicated into a single helper `runGitNetworkSpinner(label string, args ...string) (string, error)` in `internal/storage/git_sync.go`. Both call sites collapse to one line each.
- **Modern bubbletea pattern: `program.Send(doneMsg{})` + closed-channel signal.** Original plan used a `tea.Cmd` (`waitForResult`) that blocked on a result channel inside `Init()`. Refactored to: a bridge goroutine that calls `program.Send(doneMsg{})` after `<-opDone`. `doneMsg` is now a flag-only struct; the captured `opErr` carries the error. Eliminates a `<-resultCh` double-read seam when `tea.Run()` errors. Matches v1 idiom for external events.
- **Dropped `tea.WithoutSignalHandler()`.** Original plan listed it as a key decision. On review, the rationale was thin — bubbletea's default SIGINT handler doesn't block delivery to child processes (git still receives SIGINT), it just adds a deferred cursor-restore that runs on `Run()` return. Without it, Ctrl-C leaves the terminal with a hidden cursor (smoke 4 risk). Default handler is preferable.
- **Pre-existing `remove.go` newline bug fixed.** During smoke testing, the spinner overwrote the "Record successfully removed" message because `internal/cli/remove.go:66` used `Printf` without trailing `\n`. Pre-Phase 2 bug; spinner just exposed it. Fixed in the same diff.
- **Unit test added** (`internal/ui/spinner_test.go`). Plan marked it "optional, not blocking." Added a 17-line test covering the non-TTY pass-through path — the dominant code path during `make test`.
- **Documentation polish.** Three doc comments tightened post-implementation (`spinnerThreshold` 3 → 1 line, `WithSpinner` 3 → 2 lines, test comment 2 → 1 line); `prog` → `program` (3 sites in `spinner.go`).

### File layout (delivered)

| File | Status |
|---|---|
| `internal/ui/spinner.go` | **New** — `WithSpinner(label, op) error`, 250ms threshold, stderr TTY gate, `bubbles/spinner` MiniDot in cyan. |
| `internal/ui/spinner_test.go` | **New** — non-TTY pass-through unit test. |
| `internal/storage/git_sync.go` | Modified — `runGitNetworkSpinner` helper + import `internal/ui`; `GitFetch`/`GitPush` wrap sites collapsed to one line each. |
| `internal/cli/remove.go` | Modified — trailing `\n` on success message (pre-existing bug, exposed by spinner). |
| `go.mod` / `go.sum` / `nix/gomod2nix.toml` | Unchanged — `bubbles/spinner` reaches via existing `bubbles v1.0.0` direct dep. |
| All other files | Unchanged. |

### What Phase 3 needs to know

- `internal/ui/spinner.go` is package-isolated: `storage` imports `ui`; `ui` imports nothing from this project. Phase 3's go-git swap doesn't touch it.
- Single network wrap point: `runGitNetworkSpinner(label, args...)` in `internal/storage/git_sync.go`. If go-git replaces `runGitNetwork`, replace its body and the helper still works — call sites in `GitFetch`/`GitPush` stay one-liners.
- Spinner is TTY-gated; never paints during `make test` (tests pipe stderr → not a TTY → early-return). Phase 3 tests inherit this for free.
- `tea.Run()` error handling in `WithSpinner` falls through to a final `<-opDone` that ensures the goroutine has finished before returning the captured `opErr`. Don't introduce a path that returns before the channel close — would race with the `op()` goroutine.
- Open question still live: smoke 4 (Ctrl-C cursor-restore) was made safer by dropping `WithoutSignalHandler`, but no automated coverage. If Phase 3 changes how subprocesses receive SIGINT (e.g. go-git uses a separate goroutine pool), re-test.

---

## Pre-flight verification (no longer needed — Phase 2 is delivered)

Kept for archive:

Before implementing, confirm the assumptions this plan rests on. If any check fails, **stop and update the plan**, do not improvise.

```sh
# 1. Phase 1 is committed and the helpers Phase 2 wraps still exist.
git log --oneline -1
# expect: 54366f3 (or a later commit) — phase 1 git sync; mtime-LWW merge; pull/push around mutations
grep -n "runGitNetwork\b" internal/storage/git_sync.go
# expect: definition + two call sites (one in GitFetch, one in GitPush)

# 2. bubbles/spinner is reachable without a go.mod bump.
go list -deps ./... | grep -E "charmbracelet/bubbles(/spinner)?$"
# expect: github.com/charmbracelet/bubbles  (and likely bubbles/spinner if anything imports it)
# bubbles v1.0.0 is already a direct dep — its spinner subpackage imports without modifying go.mod.
# If the line is missing and `go build` later complains, run `go mod tidy` and update gomod2nix.toml.

# 3. Tests still pass on a clean tree (Phase 2 must not regress anything).
make test

# 4. Spinner output target is correct: stderr is where warnings go.
grep -n "Fprintln(os.Stderr" internal/storage/git_sync.go
# expect: printWarn writes to stderr — spinner shares the stream.
```

If `runGitNetwork` was renamed, or `GitFetch`/`GitPush` no longer exist, re-read `internal/storage/git_sync.go` and adjust the wrap targets in **§ Wrap targets** below before proceeding.

---

## Goal

While `psw add` / `change` / `remove` / `change main` are blocked on a slow `git fetch` or `git push`, render an unobtrusive spinner on stderr telling the user what is happening. When the network call completes in under a threshold, render nothing (no flicker on local-bare-remote tests, no spurious paint on a fast LAN remote).

## Constraints / non-goals

- **No behavioral change.** Same exit codes, same stdout, same warnings. Only an additional ANSI sequence on stderr.
- **No new dependency.** `bubbles/spinner` is already reachable through `bubbles v1.0.0`.
- **No cancellation logic.** Ctrl-C continues to behave as it does today (SIGINT propagates to the child `git` process, which exits, which unblocks `runGitNetwork`). We do not add a cancellation context, retry button, or progress bar.
- **No spinner around local ops.** `git init`, `git add`, `git commit`, `git log`, `git merge-base`, `git rev-parse`, `git show` stay un-wrapped — they finish in milliseconds.
- **Tests stay green and silent.** Tests pipe stderr → `term.IsTerminal(os.Stderr)` is false → spinner is skipped → no ANSI in captured stderr. No test changes.
- **No config knobs.** Threshold is hardcoded. Spinner style is hardcoded. Disabling the spinner happens implicitly via non-TTY stderr (e.g. `psw add foo 2>/tmp/log`).

## Key decisions

### New `internal/ui` package, single exported function

Decision: create `internal/ui/spinner.go` with:

```go
// WithSpinner runs op. If op takes longer than spinnerThreshold and stderr is a
// TTY, a spinner labeled <label> is rendered to stderr until op returns.
// Returns whatever op returns. Never modifies stdout.
func WithSpinner(label string, op func() error) error
```

Why a separate package: `internal/storage` calls into the spinner; the spinner imports nothing from `storage`. Keeps the dependency direction clean and makes Phase 3 unaffected by the swap to go-git.

Rejected: putting `WithSpinner` in `internal/prompt`. `prompt` is interactive-input-only; mixing animation in there muddies the package's purpose. A `psw ui` package can later host other passive UI helpers (banners, summaries) without growing `prompt`.

### Threshold: 250ms before paint

Decision: do not paint the spinner until 250ms elapses. If `op` returns first, no UI is emitted at all.

Why: against a local bare repo (the test setup) fetch+push complete in <50ms; painting and clearing within that window produces visible flicker. 250ms is comfortably above local ops and well below human "this is stuck" perception (~1s).

Rejected: painting immediately (causes flicker on fast remotes); 1s threshold (uncomfortably long stare on a 700ms remote).

### Output: stderr; TTY gate via `term.IsTerminal(os.Stderr.Fd())`

Decision: render to stderr only when stderr is a TTY. Tests redirect stderr to a buffer → not a TTY → no spinner. Same posture as `printWarn`, which already writes warnings to stderr.

Why: keeps stdout machine-parseable (someone running `psw get foo | xclip` sees no ANSI on stdout); piping stderr disables the spinner without any `--quiet` flag; matches `prompt.YesOrNo`'s scripting-safe non-TTY contract.

Rejected: stdout (would corrupt parseable output); always paint and rely on terminal width detection (still corrupts piped stderr); a `--quiet` flag (config surface for what redirection already solves).

### Concurrency pattern: goroutine + channel + bubbletea

Decision: launch `op` on a goroutine, deliver its result on a buffered channel of size 1, run `tea.NewProgram` on the calling goroutine, drive a `tea.Tick` ticker for spinner frames, and exit when a custom `doneMsg{err}` (sourced from the result channel via `tea.Cmd`) reaches `Update`.

Why: matches the codebase's existing bubbletea idiom (input prompts, picker). bubbletea handles cursor save/restore, line-clearing, and resize correctly. Hand-rolling a `\r<frame>\033[K` loop is shorter but reproduces those edge cases poorly and tangles the lifecycle.

Rejected: hand-rolled `time.Ticker` + `\r` overwrite (re-implements terminal-state handling); `bubbles/progress` (we don't have a progress fraction to show).

### bubbletea program options: `WithOutput(os.Stderr)`, `WithoutSignalHandler()`

Decision: bubbletea program is constructed with `tea.WithOutput(os.Stderr)` (default is stdout) and `tea.WithoutSignalHandler()` (we don't want bubbletea to swallow SIGINT — we want git's child process to receive it and exit, which then resolves the goroutine).

Why: `WithOutput` is the canonical knob to retarget bubbletea's renderer; `WithoutSignalHandler` lets the existing Ctrl-C-kills-git behavior keep working.

Rejected: `WithAltScreen` (overkill — we want an inline status line, not a takeover); leaving the default signal handler on (would race with git's own SIGINT handling and could leave the cursor hidden).

### Wrap targets: `runGitNetwork` calls inside `GitFetch` and `GitPush`

Decision: wrap the **single line** that invokes `runGitNetwork(...)` inside each of `GitFetch` and `GitPush`. Do not wrap the public function bodies (which also do `ensureGitRemote` and `detectBranch` — local, fast).

Why: focuses the spinner on the actual network blocking call. `ensureGitRemote` shells out to `git remote get-url`, which is local; including it under the spinner would briefly flash before the real fetch starts.

Rejected: wrapping the public function (includes fast local prelude → flicker); wrapping at the cobra layer (fork has not started yet at that level → would have to plumb through too many boundaries).

### Spinner style: `spinner.MiniDot`, foreground cyan

Decision: use `spinner.MiniDot` (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, 12 FPS) with `lipgloss.NewStyle().Foreground(lipgloss.Color("6"))` — same cyan family as the existing `color.InCyan` used for hints/commands.

Why: MiniDot is the most commonly-used neutral spinner and renders in a single cell (won't push the label past 80 cols on small terminals); cyan keeps colour semantics consistent (cyan = "in progress, informational"; yellow stays for warnings; red stays for errors).

Rejected: `spinner.Dot` (two-cell — wider; not a real win); reusing the global `color.InCyan` (it returns a string, not a `lipgloss.Style` — bubbletea wants the latter so it can compose).

---

## File-by-file changes

### New file: `internal/ui/spinner.go`

```
package ui

// WithSpinner runs op and renders a labeled spinner on stderr while op is in
// flight, suppressed if op finishes within ~250ms or stderr is not a TTY.
//
// Errors from op are returned unchanged; the spinner is decorative.
//
// Implementation:
//   - resultCh := make(chan error, 1); go func(){ resultCh <- op() }()
//   - !term.IsTerminal(stderr): wait on resultCh and return; no UI.
//   - timer := time.NewTimer(spinnerThreshold).
//     select { case err := <-resultCh: return err  // fast path, no paint
//              case <-timer.C: } // slow path: launch tea program
//   - model: spinner.Model + label + completed (bool) + opErr (error).
//   - Init: returns tea.Batch(spinner.Tick, waitForResult(resultCh)).
//   - waitForResult: tea.Cmd that blocks on resultCh and emits doneMsg{err}.
//   - Update on doneMsg: m.completed=true, m.opErr=msg.err, return m, tea.Quit.
//   - Update on spinner.TickMsg: pass through to spinner.Update; rerender.
//   - View: " <spinner.View()> <label>" while !completed; "" after (clears).
//   - tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithoutSignalHandler()).Run().
//   - Final: return m.opErr.
//
// Threshold tuning: spinnerThreshold = 250 * time.Millisecond.
//
// Edge cases:
//   - op returns nil error: spinner clears, return nil.
//   - op returns non-nil: spinner clears, error bubbles unchanged.
//   - stderr non-TTY: no goroutine timing, no tea program; just `return op()`.
//   - tea.Run() returns its own error (extremely rare): log via slog.Debug and
//     still return m.opErr (the op result is what the caller cares about).
```

Imports:

```
"log/slog"
"os"
"time"

"github.com/charmbracelet/bubbles/spinner"
tea "github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"
"golang.org/x/term"
```

Constants/helpers (file-private):

- `const spinnerThreshold = 250 * time.Millisecond`
- `var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))`
- `func isStderrTTY() bool { return term.IsTerminal(int(os.Stderr.Fd())) }`
- `type doneMsg struct{ err error }`
- `func waitForResult(ch <-chan error) tea.Cmd { return func() tea.Msg { return doneMsg{err: <-ch} } }`

### Modified: `internal/storage/git_sync.go`

Two call-site wraps. The function signatures of `GitFetch` and `GitPush` stay identical; only their internals change.

In `GitFetch` (currently line 150):

```go
// before
if out, err := runGitNetwork("fetch", "origin", branch); err != nil {
    return fmt.Errorf("git fetch %s: %s", redactURL(AppConfig.Remote), strings.TrimSpace(out))
}
```

```go
// after
var fetchOut string
err = ui.WithSpinner("Pulling from remote", func() error {
    var runErr error
    fetchOut, runErr = runGitNetwork("fetch", "origin", branch)
    return runErr
})
if err != nil {
    return fmt.Errorf("git fetch %s: %s", redactURL(AppConfig.Remote), strings.TrimSpace(fetchOut))
}
```

In `GitPush` (currently line 172):

```go
// before
out, err := runGitNetwork("push", "origin", branch)
```

```go
// after
var out string
err = ui.WithSpinner("Pushing to remote", func() error {
    var runErr error
    out, runErr = runGitNetwork("push", "origin", branch)
    return runErr
})
```

The label strings are user-visible; keep them in the present-continuous, lowercase except for the first word, no trailing dots ("Pulling from remote", "Pushing to remote"). They match the existing tone used in `initGitRepoIfNotExists` ("Initializing git repository in …").

Add to imports: `"github.com/ylniss/psw/internal/ui"`.

### No changes elsewhere

- `internal/storage/git.go` — all functions there are local-only.
- `internal/cli/*` — sees no spinner; the wrap is below the cobra layer.
- `pswcfg-template.toml` — no new config.
- `tests/*` — no test changes (TTY gate handles them).
- `go.mod` — `bubbles/spinner` reaches through the existing `bubbles` direct dep. **Run `go mod tidy` after the import is added; if it pulls anything new, update `nix/gomod2nix.toml` and `vendorHash` per `CLAUDE.md`.**

---

## Test plan

### Automated: regression-only

No new automated tests are required. The TTY gate routes all integration tests through the no-op path, so `make test` should be green with zero diff in captured stderr. Things to verify by running `make test`:

1. `tests/sync_test.go` (8 cases) all pass and contain no spinner glyphs (`⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏`) in their captured stderr.
2. `tests/log_test.go` still passes (this one had `PSW_GIT_REMOTE=0` added in Phase 1; spinner is irrelevant when remote is disabled).
3. No flake — run `make test` twice. The 250ms threshold is well above local-bare-repo latency, so the spinner never paints in tests; if a CI machine ever blows past 250ms on a local bare op, that would be the only flake source — re-run to confirm.

(Optional, not blocking: add a non-TTY unit test in `internal/ui/spinner_test.go` that calls `WithSpinner` with a fast op and asserts the returned error matches the op's. Keep it tiny — one or two cases; the heavy lifting is the contract `op() error` round-trip.)

### Manual smoke: TTY path

Spinner only renders interactively, so it must be smoke-tested by hand. Run from a real terminal (not piped, not in CI):

#### Smoke 1: fast remote → no spinner

```
make build
PSW_HOME=/tmp/sm1 ./bin/psw
# [first-run prompts; set a main pwd]
# Quit, then add `remote = "/tmp/sm1-bare"` to /tmp/sm1/pswcfg.toml.
git init --bare --initial-branch=main /tmp/sm1-bare
PSW_HOME=/tmp/sm1 ./bin/psw add foo -u u --password=p
```

Expected: command finishes in <500ms, no spinner glyph visible. (Acceptable if a glyph briefly flashes on a slow box; the threshold is 250ms.)

#### Smoke 2: slow remote → spinner appears, then clears

```
# Slow remote: an SSH URL that will hang on connect for ~30s.
sed -i 's|^remote = .*|remote = "ssh://nonexistent.example.invalid:22/repo"|' /tmp/sm1/pswcfg.toml
PSW_HOME=/tmp/sm1 ./bin/psw add bar -u u --password=p
```

Expected:
- After ~250ms a cyan spinner with label `Pulling from remote` appears on stderr.
- The line clears when `runGitNetwork` returns its 30s timeout error.
- A yellow `git fetch ssh://nonexistent.example.invalid:22/repo failed: …` warning prints (existing behavior).
- Mutation completes (record `bar` is added locally).
- Then a second spinner labeled `Pushing to remote` appears, runs the 30s push timeout, clears, prints another yellow warning.
- Exit code 0.

#### Smoke 3: non-TTY → no spinner, no ANSI

```
PSW_HOME=/tmp/sm1 ./bin/psw add baz -u u --password=p 2>/tmp/stderr.log
cat -v /tmp/stderr.log
```

Expected: `cat -v` shows no `^[[` (ESC) sequences, no spinner frames. Only the existing yellow-warning ANSI from `printWarn`, which is fine (those write to stderr regardless; that is unchanged).

#### Smoke 4: Ctrl-C during fetch

Repeat Smoke 2; while the spinner is rendering, press Ctrl-C.

Expected: process exits promptly (git child receives SIGINT, exits, `runGitNetwork` returns, the goroutine signals tea via the channel, tea quits, `WithSpinner` returns). Exit code is non-zero (cobra error path). No leftover hidden cursor or stuck terminal — `bubbletea` cleans up on `tea.Quit`. If the cursor is left hidden after Ctrl-C, that is a regression from `WithoutSignalHandler` and needs investigation (see Risks).

---

## Risks & open questions

- **Cursor state after Ctrl-C.** With `WithoutSignalHandler()`, bubbletea does not catch SIGINT; the program may be torn down before tea's deferred cleanup runs (cursor restore, line clear). Mitigation: bubbletea's renderer hides/shows the cursor via terminal escapes, but those are sent at program start/end. If Ctrl-C aborts mid-program, the terminal may stay with hidden cursor. If smoke 4 reveals this, fall back to letting bubbletea install its handler (drop `WithoutSignalHandler()`); the goroutine will leak briefly until git's 30s timeout, but the leak is harmless because the process is exiting anyway.
- **Goroutine leak on tea.Run() error.** If `tea.Run()` itself returns a non-nil error (rare — happens on bad terminal init), the goroutine running `op` may still be in flight. We don't `wait` on it before returning. Acceptable: the goroutine has no shared state with us, and the process exits soon after. Worth a `slog.Debug("tea.Run failed", "err", teaErr)` so a future debugger sees it.
- **Slow-but-stable remote pushes <250ms.** A user with a fast remote that consistently completes in 200ms never sees the spinner. By design (no flicker > occasional feedback). If user feedback says "I'd like to see something", lower the threshold to 100ms — single-line code change.
- **Spinner double-paint when fetch is followed quickly by push.** A typical mutation runs `gitPullAndMerge` (fetch) → mutate → `GitCommit` → `GitPush`. Two separate `WithSpinner` calls. Against a slow remote, the user sees `Pulling from remote …`, then a brief gap (mutation + commit, <50ms locally), then `Pushing to remote …`. That is the intended UX; bundling them under one label would lie about which phase is in flight. Document this expectation in the implementation comment.
- **stderr is a TTY but stdin is piped.** `prompt.YesOrNo` already returns `false` on non-TTY stdin; the spinner's TTY check uses stderr only, which is correct (a script could pipe stdin but watch stderr live). No conflict.
- **`go mod tidy` may pull a new transitive.** `bubbles/spinner` is part of `bubbles v1.0.0` so it should not. If it does (or a Go-version idiosyncrasy adds an indirect), update `nix/gomod2nix.toml` and `vendorHash` per `CLAUDE.md`'s build section. Regenerate gomod2nix only if `gomod2nix` is on the dev's `PATH`; otherwise note in PR description that the nix flake will need a follow-up.

---

## Done when

- `make build` succeeds; `./bin/psw --version` works.
- `make test` is green with zero changes to existing test files.
- Smoke 1 (fast remote): no spinner.
- Smoke 2 (slow remote): cyan spinner appears within ~250ms, label is `Pulling from remote` then `Pushing to remote`, line clears on completion, existing yellow warning still prints.
- Smoke 3 (piped stderr): no ANSI escape sequences, no spinner glyphs in the captured file.
- Smoke 4 (Ctrl-C): process exits promptly with no terminal-state corruption.
- `go vet ./...` and `gofmt -l .` clean.
- `CLAUDE.md` reads true after the change (no doc updates expected — spinner is internal; the user-visible UX is "you might see a cyan dot now," which is below the threshold of doc-worthy).

---

## Decisions log (during implementation)

_Append-only. Format: `YYYY-MM-DD — decision — why`._

- 2026-05-08 — extract `runGitNetworkSpinner(label, args...)` helper — both wrap sites had identical 6-line closures around `runGitNetwork`; the helper deduplicates and restores the original one-line shape at the call sites in `GitFetch`/`GitPush`.
- 2026-05-08 — refactor spinner from `tea.Cmd`-blocking-on-channel to `program.Send(doneMsg{})` from a bridge goroutine — modern v1 idiom; `doneMsg` becomes a flag-only struct; captured `opErr` carries the error so `tea.Run()` errors don't race a second `<-resultCh` read.
- 2026-05-08 — drop `tea.WithoutSignalHandler()` — bubbletea's default handler doesn't block SIGINT delivery to child processes; without it Ctrl-C may leave the cursor hidden. Default handler runs deferred cursor-restore.
- 2026-05-08 — fix `internal/cli/remove.go:66` trailing `\n` — pre-Phase 2 bug; spinner exposed it by rendering on top of the success message line. Patched in the same diff for a clean Phase 2 boundary.
- 2026-05-08 — add `internal/ui/spinner_test.go` non-TTY pass-through test — the early-return path is the only path `make test` exercises (tests pipe stderr); 17-line test materially raises confidence.
- 2026-05-08 — three doc comments tightened (`spinnerThreshold`, `WithSpinner`, test comment) — original plan's first-pass comments contained restatements and one test-impl-detail leak; trimmed to one line each except `WithSpinner` which stays at two.
- 2026-05-08 — rename `prog` → `program` in `spinner.go` — tiny mental-leap reduction; bubbletea README abbreviates in toy examples but full name reads cleanly in production code.
