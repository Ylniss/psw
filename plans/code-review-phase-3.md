# Phase 3 detail: cqroot → bubbletea/bubbles

_Source: plans/code-review-improvements.md, Phase 3._
_Drafted: 2026-05-07 against HEAD `8ddc6e6`. Re-verify before implementation if HEAD has moved._

## Goal

Single prompt framework. Replace `cqroot/prompt` + `cqroot/multichoose` with `bubbles/textinput` for inputs and a tiny custom bubbletea model for y/n. After this phase:
- `internal/prmpt/prompts.go` imports only bubbletea + bubbles (and `golang.org/x/term` for the non-TTY pre-check).
- `go.mod` no longer mentions `cqroot/*`.
- All `os.Exit(1)` calls inside `internal/prmpt/` are gone (6 sites today; the parent plan said 5 — count drifted by one because Ctrl-C inside `YesOrNo`'s raw-mode loop also exits).
- API surface preserved (signatures unchanged); callers add a single `errors.Is(err, prmpt.ErrPromptCancelled)` translation where they want silent exit on cancel.

## Pre-flight verification (already done)

- `internal/prmpt/` contains exactly one file: `prompts.go`. ✓
- Public surface used by callers: `PromptForName`, `PromptForRecordPass`, `PromptForMainPass(bool)`, `PromptForMainPassChange()`, `YesOrNo(string)`. ✓ matches parent plan.
- Caller inventory (15 call sites total):
  - `internal/cli/add.go`: 4 sites (`PromptForName ×3`, `PromptForRecordPass ×1`).
  - `internal/cli/change.go`: 6 prompt sites + 4 `YesOrNo` sites.
  - `internal/strg/storage.go`: 2 sites (`PromptForMainPass`).
- Existing bubbletea pattern lives in `internal/strg/picker.go`. Mirror its conventions: cancellation sentinel error, custom delegate, no alt-screen for inline UI.
- bubbletea v1.3.10 / bubbles v1.0.0 / lipgloss v1.1.0 are already direct deps. No version change needed.
- `golang.org/x/term` stays a direct dep (used for the non-TTY pre-check; nothing else uses it).
- `cqroot/prompt v0.9.4` is direct; `cqroot/multichoose v0.1.1` is indirect. Both must drop out after `go mod tidy`.
- `panic()` count in non-test code is already zero (Phase 2 fixed `term.MakeRaw` panic). Phase 3 must not reintroduce any.

## Target architecture

Single file `internal/prmpt/prompts.go` (rewritten in place — no new files). Approx 180 LOC. Contains:

1. **Public API** — five functions, signatures unchanged.
2. **`inputModel`** — bubbletea model wrapping `bubbles/textinput.Model`. Configurable: prompt label, echo password mode, validator (required-non-empty).
3. **`yesNoModel`** — bubbletea model for single-keypress y/n.
4. **`runInput(...)` helper** — wraps `tea.NewProgram(...).Run()` for inputs; returns `(string, error)`. Centralizes non-TTY pre-check + error mapping.
5. **`runYesNo(...)` helper** — same idea for y/n.
6. **`ErrPromptCancelled`** — sentinel exported error, message `"prompt cancelled"`.

Why one file: matches `picker.go` (single-file ~140 LOC). Splitting into `inputmodel.go` + `yesnomodel.go` adds no value and creates two import-graph edges instead of one. Revisit only if file grows past ~250 LOC.

## Cancellation & non-TTY contract

### Cancellation (Esc / Ctrl-C)

| Function | Today | After Phase 3 |
|---|---|---|
| `PromptForName` | `os.Exit(1)` | returns `("", ErrPromptCancelled)` |
| `PromptForRecordPass` | `os.Exit(1)` | returns `("", ErrPromptCancelled)` |
| `PromptForMainPass` | `os.Exit(1)` | returns `("", ErrPromptCancelled)` |
| `PromptForMainPassChange` | `os.Exit(1)` | returns `("", ErrPromptCancelled)` |
| `YesOrNo` | `os.Exit(1)` | returns `false` (cancel = "no") |

`YesOrNo` keeps its `(string) bool` signature — no error channel. Treating cancel as "no" is semantically safe for every caller: each call site reads "do you want to change X?" and false short-circuits the change. The parent plan's Phase 2 decision log already pre-committed to this.

For input prompts, callers translate `ErrPromptCancelled` to silent exit. Cleanest pattern matches `helpers.go`'s picker handling:

```go
val, err := prmpt.PromptForName("...")
if errors.Is(err, prmpt.ErrPromptCancelled) {
    return nil // silent, exit 0
}
if err != nil {
    fmt.Println(err.Error())
    return nil
}
```

This is a tiny behavior change vs. today: cancel was exit 1, becomes exit 0. Matches `helpers.go` picker behavior already shipped. Document in CLAUDE.md (Phase 6).

### Non-TTY behavior

- Env-var bypass (`PSW_MAIN_PASSWORD`, `PSW_NEW_MAIN_PASSWORD`) stays at top of `promptForMainPass` — short-circuits before any bubbletea program runs.
- For prompts without env-var bypass (`PromptForName`, `PromptForRecordPass`, `YesOrNo`): pre-check `term.IsTerminal(int(os.Stdin.Fd()))`. If false:
  - Inputs return `("", errors.New("interactive prompt required: stdin is not a terminal"))`.
  - `YesOrNo` returns `false` (preserves Phase 2 scripting-safe default).
- Don't call `tea.NewProgram(...).Run()` on a non-TTY — it can hang or fail with a less-clear error depending on the platform.

## Step-by-step implementation

Order matters: each step leaves the tree green so partial progress can land.

1. **Rewrite `internal/prmpt/prompts.go` end-to-end.** All five public fns use new bubbletea-based helpers. Preserve env-var bypass exactly. Keep `validateRequired` semantics (empty input → re-prompt with red-ish "input required" message).
2. **Update callers' cancellation handling** — 12 input-prompt sites. For each: wrap the existing `if err != nil` with `if errors.Is(err, prmpt.ErrPromptCancelled) { return nil }` first. Mechanical. `YesOrNo` callers don't change.
3. **`go mod tidy`.** Verify `cqroot/prompt` and `cqroot/multichoose` disappear from both `go.mod` and `go.sum`. Verify `golang.org/x/term` stays.
4. **`make build && make test`.** Integration tests must pass — they all use env-var bypass for main password, so they exercise the new code paths only via the env-var short-circuit. That's coverage of the bypass, not the prompt UI.
5. **Manual smoke (TTY-required) — see Test plan.**
6. **Commit as one PR.** Title: `refactor: drop cqroot, prompts on bubbletea/bubbles`.

## Per-function specs

### `inputModel`

Fields:
```go
type inputModel struct {
    label     string         // e.g. "Main password", "Username"
    input     textinput.Model
    err       string         // displayed below input on validation fail
    submitted bool
    cancelled bool
}
```

- `Init` returns `textinput.Blink`.
- `Update`:
  - `tea.KeyMsg`:
    - `enter`: run validator; if it fails, set `m.err` and stay; otherwise `m.submitted = true; return m, tea.Quit`.
    - `ctrl+c`, `esc`: `m.cancelled = true; return m, tea.Quit`.
  - default: delegate to `m.input.Update(msg)`.
- `View`:
  - Line 1: `<label>: <input.View()>`.
  - Line 2 (only if `m.err != ""`): faint/yellow error line. Match style with picker's `helpStyle` (or define a small `errStyle`).

Configuration in `runInput`:
- `textinput.New()`, `Focus()`, set `EchoMode = textinput.EchoPassword` for password fields.
- `tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(os.Stderr))`. Render to stderr so prompts don't pollute pipeable stdout — minor improvement over cqroot which used stdout. **Sanity-check with one manual run** before committing; if it breaks the test harness's stdout assertions, revert to default (stdout). The harness only asserts on `psw` output for non-prompt commands, so stderr should be fine.
- **No `tea.WithAltScreen()`.** Prompts render inline so they appear in normal terminal flow — same UX as cqroot.

### `yesNoModel`

Fields:
```go
type yesNoModel struct {
    question string
    answer   bool
    decided  bool
}
```

- `Init` returns `nil`.
- `Update`:
  - `tea.KeyMsg.String()` ∈ {`"y"`, `"Y"`}: `answer = true; decided = true; return m, tea.Quit`.
  - `tea.KeyMsg.String()` ∈ {`"n"`, `"N"`}: `answer = false; decided = true; return m, tea.Quit`.
  - `"ctrl+c"`, `"esc"`: `decided = false; return m, tea.Quit` (caller treats as cancel → false).
  - Anything else: ignore (stay in model).
- `View`: `"<question> (y/n)"` on one line.

`runYesNo` returns `(bool, bool)` — `(answer, ok)`. `ok=false` means cancelled. `YesOrNo` translates `ok=false` → `false`. Internal helper, not exported.

### `PromptForMainPass(ensure bool)` / `PromptForMainPassChange()`

- Env-var bypass (verbatim from current code) at the very top.
- Loop: ask main password, if `!ensure` return; ask repeat; if mismatch, print yellow "Passwords don't match, try again" and re-loop.
- Each `runInput` launch is its own `tea.NewProgram` instance. The loop is in Go, not in the model. Simpler and matches the current cqroot loop structure.
- On any `runInput` returning `ErrPromptCancelled`, propagate immediately (don't keep looping).

### `PromptForRecordPass`

Same loop structure as `promptForMainPass(ensure=true, mainPassChange=false)` minus the env-var bypass and minus the configurable label.

### `PromptForName(promptText string)`

Single `runInput` call with validator = `validateRequired`. Echo mode = normal (visible).

## Caller updates

12 input-prompt sites. For each site that currently looks like:
```go
val, err := prmpt.PromptForX(...)
if err != nil {
    fmt.Println(err.Error())
    return nil
}
```
Insert before the existing `if err != nil`:
```go
if errors.Is(err, prmpt.ErrPromptCancelled) {
    return nil
}
```

Sites:
- `internal/cli/add.go:134` (`getRecordName`)
- `internal/cli/add.go:141` (`getOrPromptUsername`)
- `internal/cli/add.go:155` (`getOrPromptValue`)
- `internal/cli/add.go:162` (`getOrPromptRecordPass` via `getOrGenerateRecordPass`) — note: this fn doesn't have an `if err` block today; it `return`s the result directly. Caller is in `addCmd.RunE`. Translate at the caller site.
- `internal/cli/change.go:61` (`changeMainPass` first prompt)
- `internal/cli/change.go:73` (`changeMainPass` new pass)
- `internal/cli/change.go:177` (`applyOrPromptRename`)
- `internal/cli/change.go:201` (`applyOrPromptUsername`)
- `internal/cli/change.go:221` (`applyOrPromptPassword`)
- `internal/cli/change.go:241` (`applyOrPromptValue`)
- `internal/strg/storage.go:138` (`GetOrCreateIfNotExists` re-prompt)
- `internal/strg/storage.go:187` (`createEncryptedStorageIfNotExists`)

`storage.go` callers should not silently swallow — propagate the cancellation up (return the error). The CLI layer at the outermost caller decides to `return nil` on `errors.Is(err, prmpt.ErrPromptCancelled)`. This keeps storage-layer concerns clean.

YesOrNo sites unchanged (4 sites in `change.go`).

## Test plan

### Automated

`make test` covers:
- Every command runs with `PSW_MAIN_PASSWORD=testpass` set, so the bypass path is exercised heavily. **This does not exercise the bubbletea prompt UI itself.** That's a known gap, same as today.
- Add a single negative test: `tests/prompt_test.go` running `psw add foo` (no flags) without `PSW_MAIN_PASSWORD` and without a TTY. Expect non-zero exit and stderr containing "interactive prompt required". Confirms the non-TTY pre-check fires and doesn't hang. Use `runPswEnv(t, vault, map[string]string{"PSW_MAIN_PASSWORD": ""}, "add", "foo")` and assert `r.code != 0`.

### Manual smoke (required, TTY-only)

Run from a real terminal against a scratch vault (e.g., `PSW_HOME=/tmp/psw-smoke ./bin/psw`). Cover:
- [ ] First-run vault creation: prompts for main password twice, mismatch shows warning, match proceeds.
- [ ] `psw add foo` (no flags): prompts for username, password, password-repeat. Mismatch warning. Esc on any prompt exits silently (exit 0).
- [ ] `psw add bar -s`: prompts for value.
- [ ] `psw change foo`: y/n prompts work. `y`/`Y`/`n`/`N` accepted. Other keys ignored. Esc on a y/n returns "no" semantically (no field change).
- [ ] `psw change main`: prompts for current pass, then new pass + repeat. Re-encrypts and lists records under new pass.
- [ ] Ctrl-C during a password prompt: silent exit, no stack trace, no leftover terminal in raw mode.
- [ ] Pipe `echo "" | psw add foo`: clean error "interactive prompt required …", exit non-zero, no hang.
- [ ] Verify `make build` succeeds, `go vet ./...` clean.
- [ ] Verify `grep -r cqroot go.mod go.sum` returns nothing.

## Verification checklist

- [ ] `grep -rn "cqroot" .` returns zero matches outside `plans/`.
- [ ] `grep -rn "os.Exit" internal/prmpt/` returns zero matches.
- [ ] `grep -rn "panic(" internal/` returns zero matches (was already true before this phase).
- [ ] `make test` green.
- [ ] Manual smoke checklist above all green.
- [ ] `go.sum` no longer contains `cqroot/*` or `cqroot/multichoose` entries.
- [ ] CLAUDE.md left untouched (Phase 6 updates docs).

## Risks & gotchas

- **bubbletea on stderr.** Rendering prompts to `os.Stderr` (recommended) might surprise tests that capture stderr. Existing tests do capture stderr (`helpers_test.go:70`). Tests don't assert on prompt UI text (env-var bypass), so OK. Sanity-check by running `make test` after the rewrite.
- **`textinput.Model.EchoMode` default value.** Default is normal echo. Set explicitly for password fields. If we forget, passwords echo to terminal — security-relevant. Add a code review checklist note.
- **Bubble blink command.** `textinput.Blink` is a `tea.Cmd` returned from `Init`. Forgetting it just makes the cursor not blink — cosmetic.
- **`tea.KeyMsg.String()` vs `.Type`.** Using `.String()` for `"esc"`, `"ctrl+c"` is idiomatic per picker.go:71. Stick with that.
- **`textinput.Validate`.** bubbles/textinput supports a `Validate` field, but it runs on every keystroke. We want validation only on Enter (matches cqroot semantics). Don't use `Validate`; run the check manually in the Enter branch.
- **Repeat-password loop UX.** Cqroot kept the "Passwords don't match, try again" message between attempts. Mirror that: print the warning to stderr/stdout between bubbletea program runs (i.e., after the program quits, before the next one starts). Don't try to render it inside the model.
- **Window size events.** Single-line prompts don't need resize handling. No `tea.WindowSizeMsg` branch needed.
- **Test-binary cancellation.** Tests don't send Ctrl-C; they exercise the env-var path. Manual smoke is the only Ctrl-C coverage.
- **No `tea.WithAltScreen()`.** Critical for prompts: alt-screen would clear terminal history each time. Already noted but bears repeating.

## Rollback

Single git revert. The phase is one PR. If the smoke test fails, revert and investigate.

## Out of scope (deferred)

- Phase 4 will clean up the remaining `os.Exit(1)` sites in `internal/cli/helpers.go` and `internal/strg/config.go`.
- Phase 6 updates CLAUDE.md (drops cqroot mentions; notes the new YesOrNo cancel semantics).
- Test coverage for bubbletea prompt UI itself is still a gap. Adding a `expect`-style script harness would be a separate effort, not blocking Phase 3.
