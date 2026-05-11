# Multi-remove + rollback

_Last updated: 2026-05-11 — commit `932f1ce` (Phase 1 committed; Phases 2-3 pending implementation. Detail: `plans/multi-remove-rollback-phase2.md`.)_

## Goal
Add multi-select removal (menu + CLI) and a `psw rollback` CLI command that reverts records to a past commit's snapshot via a new commit (not a hard reset). Bump version after both features land.

## Constraints / non-goals
- Rollback is CLI-only (not exposed in the menu).
- Rollback does not force-push, hard-reset, or rewrite history — it commits the old snapshot as a new commit.
- Picker behavior for `get`/`change` is unchanged (still single-select).
- No new dependency on `git` being on PATH for rollback.

## Key decisions (pre-implementation)
- **Rollback = revert-commit, not hard reset.** Reuses existing pull→merge→push pipeline. No force-push, no `--force-with-lease`, no detached HEAD. Rejected hard-reset+force-push because it silently regresses on other devices (their pull-merge sees them as "already ahead" via `isAncestor(remoteSHA, localSHA)` and pushes the discarded commits back).
- **Rollback stamps every record's MTime to `now()`.** Required so the rollback's records win LWW on other devices' divergent merges. Traced through `mergeRecords`: without this, on a divergent peer that modified an unrelated record, the `F1 L1 R1` case for records the rollback reverted keeps the local (newer-mtime) version → rollback partially reverts. `F1 L1 R0` removals drop correctly without mtime work because that branch compares local against fork, not remote.
- **`change main` guard.** Decrypt target blob with cached password before writing; refuse with an `ErrForkUndecryptable`-style banner if it fails (same root cause: blob encrypted under a different password).
- **Multi-select via opt-in `PickerModel.WithMulti()`.** Single-select default preserves get/change. Space toggles, Enter confirms; Enter with no toggles falls back to highlighted-as-selection (today's fast path).
- **Commit message for multi-remove: count only** (`removed N records`).
- **`--exact` with multiple args:** apply exact match to each.

## Repo context a fresh Claude needs
- **MTime LWW lives in `internal/storage/merge.go` (`pickByMTime`).** The cases that matter for rollback: `F1 L1 R1` and `F0 L1 R1` use mtime to pick. Without bumping mtime, rollback's old-mtime records lose. `F1 L1 R0` (removal) compares local-mtime > fork-mtime — removed records drop correctly without mtime work.
- **`storage.gitShowBlob(ref, path)`** already exists (`internal/storage/git_local.go`); used by merge to fetch fork/remote storage.psw. Reuse for rollback target.
- **`PickerModel` is shared** (`internal/storage/picker.go`). Today: single-select, fuzzy filter, `Help()` returns one fixed const. Multi-mode needs: toggled-set, render marker per item, mode-aware `Help()`, distinct `Selections() []string`.
- **`GetOrCreateForMutate`** (`internal/storage/loader.go`) auto-pulls/merges before returning; `GitCommit` auto-pushes (`internal/storage/git.go`). Rollback should use it like any other mutation.
- **CLI errors:** `errSilentExit` for exit-1-without-usage-dump; print via `color.InRed`. See `internal/cli/remove.go` for the pattern.
- **Menu remove footer** uses `picker.Help()` via `RemoveAction.FooterHelp()`. Mode-aware `Help()` propagates automatically.
- **PSW_GIT=0** disables git entirely; rollback should refuse early when `storage.IsGitRepo()` is false.
- **`storage.GitLog()`** already returns commits newest-first after `slices.Reverse` (`internal/storage/git_local.go:gitLogEntries`).
- **Record mtime field:** `internal/storage/storage.go` — `Record.MTime` is `int64` UTC ms; stamped centrally in `AddRecord`/`UpdateRecord`. Rollback writes records directly, so it must stamp explicitly.

## Phases

### [x] Phase 1: Multi-select removal (menu + CLI)
- **Goal:** Multi-select in remove flow, both interactive picker and variadic CLI args.
- **Scope:**
  - `internal/storage/picker.go`: add `WithMulti()`, `Selections()`, toggled set, space keybind, render marker (e.g. `[x]` / `[ ]`), mode-aware `Help()`. Enter with no toggles → fall through as single selection (matches current UX).
  - `internal/menu/remove.go`: switch picker to multi mode; after picking, run a y/n confirm listing the names with a one-line hint that `psw rollback` can undo; save loop + single commit.
  - `internal/cli/remove.go`: accept variadic args; zero args opens multi-picker; `--exact` applies per-arg; update `Use`/`Short`/`Long` to document `psw remove [name...]`.
- **Done when:** menu and CLI both support N-record removal in one operation, single commit `removed N records`, integration tests cover multi-arg and picker multi-select paths; get/change unaffected.
- **Risk:** medium (picker is shared; regression risk on get/change if multi-mode flag leaks)
- **Depends on:** none

### [ ] Phase 2: `psw rollback` CLI command
- **Goal:** New `psw rollback` command that creates a revert-style commit from a chosen past commit's `storage.psw`.
- **Scope:**
  - `internal/cli/rollback.go` (new), registered in `internal/cli/root.go`'s `init()`.
  - Refuse early when `IsGitRepo()` is false (red error, exit 1).
  - List commits via `storage.GitLog()`; filter out HEAD (no-op); format each as `<short-sha>  <date>  <msg>` single line; pass to single-select `PickerModel`.
  - On selection: `gitShowBlob(targetSHA, Paths.storageFileName)` → `decryptBytes(blob, password)`. On decrypt failure: red message ("This commit was encrypted under a previous main password; cannot roll back across that.") + exit 1.
  - Short confirm via `prompt.YesOrNo`: `"Replace records with snapshot from <short-sha> (<msg>)? (y/n)"`.
  - Parse decrypted blob → `[]Record`; stamp `MTime = time.Now().UnixMilli()` on every record; marshal → encrypt under current password → `storage.Save()` → `storage.GitCommit("rollback to <short-sha>: <msg>")`.
  - Integration tests: rollback to N-back commit verifies records match snapshot; rollback across `change main` refused; rollback in offline mode (`PSW_GIT_REMOTE=0`) commits locally; rollback with `PSW_GIT=0` errors out.
- **Done when:** `psw rollback` works end-to-end against a multi-commit vault; the resulting commit, when pulled on a second device with divergent local changes, correctly wins LWW on modified records and drops added-since records via the existing merge.
- **Risk:** medium (touches encryption/decryption + git plumbing; MTime semantics are subtle)
- **Depends on:** none (independent of Phase 1)

### [ ] Phase 3: Version bump
- **Goal:** Bump `VERSION` to `0.30`.
- **Scope:** Single-line edit. Build verification (`make build`).
- **Done when:** committed.
- **Risk:** low
- **Depends on:** phases 1 and 2 merged

## Decisions log (during implementation)
- 2026-05-11 — bubbletea v2 `KeyPressMsg.String()` for spacebar returns `"space"` (not `" "` as in v1). Picker uses `case "space":` accordingly. Caught during picker unit tests; documented inline.
- 2026-05-11 — `RemoveCommitMessage(n)` lives in `internal/storage/git.go` next to `GitCommit` (single source of truth for menu + CLI). Avoided duplicating the singular/plural switch.
- 2026-05-11 — Picker's "fall through to cursor item on empty toggle set" logic centralized as `(PickerModel).ResolvedSelections()`; both `stepPickerMulti` and `GetRecordNamesInteractive` use it.
- 2026-05-11 — Menu remove confirm: y/n decline (n or esc on the y/n) bounces back to picker via `picker.Reopen()`; toggled set preserved so the user can adjust selections. Esc on the picker still aborts the whole flow.
- 2026-05-11 — Picker constructor split: `NewPickerModelMulti(names)` is a separate top-level constructor (not a chained `WithMulti()`). Avoids carrying an `extras` field on `PickerModel` solely for delayed delegate reconstruction.

## Open questions
(none)

## Hand-off
To detail a phase, start a fresh context and ask:
> Prepare a detailed plan for phase N from `plans/multi-remove-rollback.md`.

**Before writing phase detail, verify the plan is not stale.** Compare the **Last updated** commit to current `HEAD`; read the files cited in **Repo context** to confirm they still exist and behave as described. If anything has drifted, surface the drift and update the plan before producing detail.
