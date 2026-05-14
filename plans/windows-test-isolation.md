# Windows test isolation — quarantine integration tests from developer git config

_Last updated: 2026-05-14 — phase 2 landed (pending commit on branch `develop`)_

## Goal

`go test ./...` is green on both Linux and Windows regardless of the developer's `~/.gitconfig`, `/etc/gitconfig`, shell, or local environment. Currently three tests fail on Windows after the renameio→writeFileAtomic switch made the build work: `TestSync_GPGSignFallback_GitNotOnPath`, `TestSync_GPGSignFallback_GitOnPath`, `TestRollback_LogColoring`. None are production-code bugs — all are test-harness portability gaps now reachable that were previously hidden by the broken build.

## Constraints / non-goals

- Do not modify production code under `internal/` or `cmd/`. This is a pure test-harness fix.
- Do not break Linux test runs.
- Tests that need a real `git` binary stay gated on `exec.LookPath("git")` (already done).
- Do not introduce a new third-party dependency; stdlib + existing deps only.

## Key decisions (pre-implementation)

### Approach: A+B hybrid (clean HOME + git env overrides), not pure B

**Decision:** Override `HOME`, `USERPROFILE`, and `XDG_CONFIG_HOME` to an empty `t.TempDir()` AND set `GIT_CONFIG_NOSYSTEM=1` in the spawned `psw` subprocess env.

**Why:** Verified against `go-git/v5/config/config.go:191-214` — go-git's `Paths(GlobalScope)` reads `XDG_CONFIG_HOME` env var and then `os.UserHomeDir() + .gitconfig`. go-git **does not honor** `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, or `GIT_CONFIG_NOSYSTEM`. Pure-B (just setting those env vars) would only isolate shell-out git, leaving go-git's pure-Go config reader still loading the user's `~/.gitconfig`. The hybrid covers both code paths.

**Rejected:**
- **Pure A (clean HOME only):** would leave shell-out git reading `/etc/gitconfig` (or Windows `C:\ProgramData\Git\config`); `GIT_CONFIG_NOSYSTEM=1` plugs that small gap for shell-git.
- **Pure B (`GIT_CONFIG_GLOBAL=NUL`):** verified broken for go-git, as above.
- **Skip the offending tests on Windows:** leaves real coverage gaps and was the reason the harness wasn't already isolated.

### Env scope safety: `cmd.Env` on the child only, never `os.Setenv` in the test process

**Decision:** Set the isolation vars via `cmd.Env = flattenEnv(...)` on the spawned `psw` subprocess only. Do NOT touch the test process's environment.

**Why:** `cmd.Env` is local to a single subprocess and freed when the child exits — even on panic, kill -9, or test crash. Zero risk of leaking into the shell, OS, or persistent Windows registry env. (`HKCU\Environment` / `HKLM\...\Session Manager\Environment` are the only persistent locations and we never touch them.)

### Fake-gpg portability: Go-built helper binary, not shebang script

**Decision:** Replace the `#!/bin/sh` script in `tests/sync_gogit_test.go:writeFakeGpg` with a tiny Go-built helper compiled once in `TestMain` (same pattern as the existing `psw` build).

**Why:** Windows can't execute a `#!/bin/sh` file as a binary regardless of extension. A `.bat` alternative would work but requires OS-split scripting. A Go-built helper is cross-platform, has no shell dependency, and follows the existing `TestMain` pattern.

**Rejected:**
- `.bat` on Windows / `.sh` on Linux (build-tag split): two artifacts to maintain.
- Skipping `TestSync_GPGSignFallback_GitOnPath` on Windows: leaves the test green-for-the-wrong-reason.

### Dedupe `flatEnvForVault` ↔ `runPswEnv`

**Decision:** Extract a `flattenEnv(vault, extra) []string` helper. `runPswEnv` uses it inline (`cmd.Env = flattenEnv(...)`); the existing call site in `rollback_test.go` (the one that bypasses ANSI stripping) uses it directly. Delete `flatEnvForVault`.

**Why:** They already diverged once (only `runPswEnv` got `USERPROFILE` forwarded). Two parallel env builders is a future-bug magnet. Small helper, not over-engineered.

## Repo context a fresh Claude needs

- **go-git ignores git's standard env overrides.** `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, `GIT_CONFIG_NOSYSTEM` only affect the `git` binary (shell-out path). go-git's config loader is at `go-git/v5/config/config.go:Paths`. It uses `XDG_CONFIG_HOME` env var + `os.UserHomeDir()`, with `/etc/gitconfig` hard-coded for system scope. This is non-obvious and load-bearing for any future test-isolation work.
- **psw has a hybrid git backend.** Local ops are pure go-git. Network ops (`fetch`/`push`) and signed commits fall back to shell `git` via `internal/storage/git_shell.go` when go-git can't handle them. Therefore test isolation must cover both code paths.
- **Two env-flattening helpers exist today:** `tests/helpers_test.go:runPswEnv` and `tests/rollback_test.go:flatEnvForVault`. They diverged — only `runPswEnv` forwards `USERPROFILE`. Phase 1 consolidates.
- **`tests/main_test.go`** uses `TestMain` to build `psw` once into a temp dir. The fake-gpg helper in Phase 2 follows the same pattern.
- **The `.exe` suffix on Windows:** `tests/main_test.go` now appends `.exe` to `pswBinary` on Windows so `exec.Command(absolutePath)` resolves correctly. Same convention applies to the new fake-gpg helper.
- **`writeFakeGpg`'s wire protocol with git:** writes shebang script that drains stdin, prints `[GNUPG:] SIG_CREATED B 1 10 00 1234567890 0000000000000000` to stderr, and a dummy `-----BEGIN PGP SIGNATURE-----` block to stdout. The Go reimplementation must match this exactly — git parses the SIG_CREATED line and won't accept the commit without it.
- **PSW_GIT env var meanings:** `PSW_GIT=0` disables git entirely (default in tests). `PSW_GIT=""` (empty) enables. `PSW_GIT_REMOTE=0` enables local commits but disables network. See CLAUDE.md "Testing / scripting mode" for the full env-var contract.
- **HOME on Linux vs USERPROFILE on Windows:** Go's `os.UserHomeDir()` reads `$HOME` on Unix and `%USERPROFILE%` on Windows. Both must be overridden for cross-platform isolation. `XDG_CONFIG_HOME` is read regardless of OS (go-git path) — override to a known empty dir.

## Phases

### [x] Phase 1: Env isolation in test harness
- **Goal:** Quarantine spawned `psw` subprocesses from developer's git config; consolidate the two env-flattening helpers into one.
- **Scope:** `tests/helpers_test.go`, `tests/rollback_test.go`, `tests/sync_gogit_test.go` (the last added mid-phase — see decisions log). Add `HOME`, `USERPROFILE`, `XDG_CONFIG_HOME` overrides pointing at an empty `t.TempDir()`. Set `GIT_CONFIG_NOSYSTEM=1`. Extract `flattenEnv(t, vault, extra) []string`; delete `flatEnvForVault`; update the one direct caller in `rollback_test.go` to use the new helper. Also fix `pathWithoutGit` to strip `git.exe` (Windows ships git only as `.exe`).
- **Done when:** `TestSync_GPGSignFallback_GitNotOnPath` passes on Windows. `TestSync_GPGSignFallback_GitOnPath` no longer fails at gitconfig-read time (it will still fail at fake-gpg exec / `gpg.program` path encoding, addressed in Phase 2). Linux tests still pass. Helper consolidation visible in diff.
- **Outcome (2026-05-14):** done. `TestSync_GPGSignFallback_GitNotOnPath` green on Windows; `TestSync_GitNotOnPath_LocalOnly` still green (no regression from the `pathWithoutGit` change). `TestSync_GPGSignFallback_GitOnPath` now fails at `9:15: unknown escape sequence` in go-git's INI parser (raw `C:\…` path in `gpg.program`) — explicitly a Phase 2 blocker now.
- **Risk:** low.
- **Depends on:** none.

### [x] Phase 2: Cross-platform fake-gpg helper
- **Goal:** Replace `#!/bin/sh` fake-gpg with a Go-built helper executable so `TestSync_GPGSignFallback_GitOnPath` works on Windows.
- **Scope:** New `tests/cmd/fakegpg/main.go`. Build it in `TestMain` next to `psw`, store path in package-level `fakeGpgBinary`. Drop `writeFakeGpg` (inline at the single caller). Wire protocol: drain stdin, emit `[GNUPG:] SIG_CREATED ...` to stderr, emit dummy `-----BEGIN PGP SIGNATURE-----`/`-----END PGP SIGNATURE-----` block to stdout, exit 0. `.exe` suffix on Windows like `psw`. Fix the gcfg INI escape bug by writing the `gpg.program` value through `filepath.ToSlash` (forward slashes — no escapes needed; gcfg's value scanner only accepts `\\ \" \n n t b` after a backslash).
- **Done when:** `TestSync_GPGSignFallback_GitOnPath` passes on Windows and Linux. Old shebang script removed.
- **Outcome (2026-05-14):** done. Both Windows gpgsign tests green; full suite has only `TestRollback_LogColoring` (Phase 3) still red. `tests/cmd/fakegpg` shows `[no test files]` under `go test ./tests/...` — expected, no failure.
- **Risk:** low–medium (wire protocol must match git's expectations exactly; INI escape rules must match go-git's parser, not git's).
- **Depends on:** none (independent of Phase 1, but Phase 1 should ship first since it's smaller and unblocks the GitNotOnPath test).

### [ ] Phase 3: `TestRollback_LogColoring` ANSI on Windows
- **Goal:** Make the test pass on Windows. Likely a 1-line test fix once root cause is known.
- **Scope:** Start with investigation, not implementation. Hypotheses to falsify:
  - `TwiN/go-color` v1.4.1's `enabled` is `true` by default and no code calls `Toggle(false)`, so `InPurple` should emit `\x1b[35m`. Verify by running `psw log` directly on Windows and inspecting raw bytes (e.g., `go run ./cmd/psw log | xxd | head`).
  - If bytes ARE in stdout but `strings.Contains(raw, "\x1b[35m")` fails: encoding/decoding issue at the test's `cmd.Output()` boundary on Windows.
  - If bytes are NOT in stdout: something at the `fmt.Printf` boundary in `internal/cli/log.go` is filtering. Could be a Go-on-Windows console handle quirk when stdout is a pipe (vs terminal).
  - If a Cobra or stdlib hook disables colors when stdout isn't a TTY, that would explain it — but neither does so by default. Worth confirming.
- **Done when:** root cause identified, fix applied (in test or production), `TestRollback_LogColoring` passes on Windows.
- **Risk:** unknown until investigated. Could be 1 line or could surface a real production code change (e.g., color emission needs an explicit unconditional flag).
- **Depends on:** none.

## Decisions log

(append-only; entries: `YYYY-MM-DD — decision — why`)

- 2026-05-14 — extended phase-1 scope to include 1-line `pathWithoutGit` fix in `tests/sync_gogit_test.go` — without it the test's "git not on PATH" branch is unreachable on Windows since git ships as `git.exe` only. Confirmed empirically: 4 PATH entries contain `git.exe`, 0 contain `git`. The original phase-1 scope (`helpers_test.go`/`rollback_test.go` only) was insufficient to meet the "done when" criterion for `TestSync_GPGSignFallback_GitNotOnPath`.
- 2026-05-14 — `flattenEnv` takes `*testing.T` (was `(vault, extra)` in the original plan) — needed to call `t.TempDir()` for the isolation dir. Caller (`TestRollback_LogColoring`) already has `t` in scope; minor signature change, no churn elsewhere.
- 2026-05-14 — surfaced INI-escape bug in `addToGitConfig` writing the `gpg.program` Windows path → folded into Phase 2 scope since it's the same test (`TestSync_GPGSignFallback_GitOnPath`) and same file (`tests/sync_gogit_test.go`).
- 2026-05-14 — phase 2 chose `filepath.ToSlash` over quoted-double-escape for `gpg.program` — gcfg's scanner only escapes `\\ \" \n n t b`; forward-slash paths need no escaping and `CreateProcess` on Windows accepts them. Quoted `"C:\\\\Users\\\\..."` would also work but needs four backslashes per separator in Go source.
- 2026-05-14 — phase 2 deleted `writeFakeGpg` rather than renaming it — single caller, no longer "writes" anything; inlining the package-level `fakeGpgBinary` + `filepath.ToSlash` at the call site is one line.

## Open questions

(none at save time)

## Hand-off

To detail a phase, start a fresh context and ask:
> Prepare a detailed plan for phase N from `plans/windows-test-isolation.md`.

**Before writing phase detail, verify the plan is not stale.** Compare the **Last updated** commit to current `HEAD`; read the files cited in **Repo context** to confirm they still exist and behave as described. The verified facts about go-git config loading and the fake-gpg wire protocol are particularly worth re-checking against the current dep versions. If anything has drifted, surface the drift and update the plan before producing detail.
