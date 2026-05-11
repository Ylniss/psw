# Phase 2 detail — `psw rollback` CLI command (with Phase 3 tail)

_Parent plan: `plans/multi-remove-rollback.md` (HEAD `932f1ce`)._
_Drift check: parent's `Last updated` commit equals current HEAD (after the just-applied stamp refresh). All cited files verified — `internal/storage/{merge.go, git_local.go, git.go, picker.go, storage.go, pull_merge.go, encryption.go, loader.go}` and `internal/cli/{remove.go, log.go, root.go, helpers.go}` match the parent plan's descriptions. One bonus that the parent plan didn't surface: `internal/storage/pull_merge.go` already has a private helper `decryptBlobToRecords(ref, password)` that does `gitShowBlob → decryptBytes → json.Unmarshal` — Phase 2 will reuse it._

## Scope confirmation

In:
- New CLI subcommand `psw rollback` (no positional args, no flags) that opens a single-select picker over past commits, lets the user pick one, asks y/n confirmation, then replaces current records with that commit's snapshot, mtime-stamps every record to now, and creates a new commit on top of HEAD.
- Early refusal when not a git repo (red message, exit 1).
- Early refusal when the chosen commit's `storage.psw` can't be decrypted under the current main password (red message naming the cause, exit 1) — pre-confirm.
- Existing pull→merge→commit→push pipeline reused via `GetOrCreateForMutate` + `GitCommit`. Best-effort push (same posture as add/change/remove).
- Extension of `colorizeLogMessage` in `internal/cli/log.go` so `psw log` renders rollback commits in a distinct color.
- Integration tests covering happy path, change-main guard, no-git-repo, no-prior-commits, offline mode, mtime stamping, and divergent-peer convergence.

Out:
- Menu exposure. `psw rollback` is CLI-only; no menu action, no menu hotkey. Documented in the parent plan and reaffirmed here.
- Positional arg `psw rollback <short-sha>`. Defer to a future phase; v1 is picker-only to keep the surface small and force a confirmation step.
- `--yes`/`--no-confirm` flag. Rollback is destructive enough to always confirm.
- Force-push / hard-reset / detached HEAD. Rollback is revert-by-commit — history grows, never rewinds.
- Rolling back across a `change main` boundary. Refused with a banner; user is told to roll back from the device that owns the new password (or to roll back to a target encrypted under the *current* password).

## Key design decisions

### D1 — Core logic in `internal/storage/rollback.go`; CLI is a thin TUI wrapper
CLAUDE.md: "thin wrappers, real logic under `internal/`." Rollback reads a git blob, decrypts it, mutates a `Storage`, calls `Save()` and `GitCommit()` — all `internal/storage` concerns. The CLI orchestrates I/O: pick, confirm, print. Mirrors the layering of `add`/`remove`/`change`/`log`.

### D2 — Two-function core split: load-then-apply
```go
// internal/storage/rollback.go

// LoadCommitRecords reads <ref>:storage.psw and returns the decrypted record
// slice. Returns ErrForkUndecryptable when the blob can't be decrypted under
// the current main password (e.g. across a 'change main' boundary).
func LoadCommitRecords(ref, password string) ([]Record, error)

// ApplyRollback replaces store.Records with the supplied records (each MTime
// stamped to time.Now().UnixMilli()), persists, and commits via GitCommit.
// target is used only to format the commit message.
func ApplyRollback(store *Storage, target LogEntry, records []Record) error
```
The split lets the CLI do the decrypt-check *before* the y/n confirm. Combined call would force the user to confirm before knowing the rollback can succeed.

### D3 — Decrypt-check happens before the y/n confirm
Flow in the CLI:

1. Pick target via picker.
2. `records, err := LoadCommitRecords(target.ShortSHA, store.MainPassword)`.
3. If `err == ErrForkUndecryptable` → red banner + `errSilentExit`. **No confirm prompt.**
4. y/n confirm.
5. On `y` → `ApplyRollback(store, target, records)`.
6. On `n` (or Esc on the y/n) → return silently.

Banner text: `"This commit was encrypted under a previous main password; cannot roll back across that."`

### D4 — Stamp every record's MTime to `time.Now().UnixMilli()` on apply
This is the LWW correctness fix flagged in the parent plan. Without it, after the rollback push, a divergent peer's `mergeRecords` would see `F1 L1 R1` and prefer the peer's (newer-mtime) records — partially reverting the rollback. Stamping ensures the rollback's records win the LWW comparison.

Caveat to surface in the verification plan: this only governs records *present* in the rollback snapshot. Records added between target and HEAD are "removed" by the rollback; their `F1 L1 R0` merge case compares L.mtime vs F.mtime, and if a peer modified one of those records offline (L.mtime > F.mtime), the peer's modification survives the merge. This is the existing "modification beats removal" semantic — same as a regular `psw remove` competing with a peer's `psw change`. Document in the integration test as expected behavior.

### D5 — Filter HEAD from the candidate list; bail when empty
`storage.GitLog()` returns every commit, including HEAD. Rolling back to HEAD is a no-op. Filter it before passing to the picker. If the filtered list is empty (vault has only the initial commit, or the user already rolled back to the only other available state), print:

```
No previous commits to roll back to.
```

and return nil (exit 0). No picker launch.

HEAD detection: add a tiny exported helper `storage.HeadShortSHA() (string, error)` that wraps `revParse("HEAD")` and slices the first 7 hex chars. Reuses `revParse` already in `git_local.go`. Alternative: compare against full SHA — but `LogEntry.ShortSHA` is already 7 chars, so comparing on the short form is consistent.

### D6 — Reuse single-select `PickerModel` with composed labels; map labels back to entries
The picker accepts `[]string`; rollback needs to display sha + date + message and resolve back to a `LogEntry`. Build a `map[string]LogEntry` keyed by the composed label, pass labels to `NewPickerModel(labels, nil)`, look up the LogEntry by the chosen label.

Label format: `<short-sha>  <yyyy-mm-dd hh:mm>  <message>` (two spaces between fields, matching `psw log`'s output). Names are guaranteed unique by the SHA prefix.

Picker's single-item fast-path is acceptable here: if only one rollback target exists, the picker returns it without TUI, and the subsequent y/n still asks the user to confirm with the SHA + message inline — so the user is never silently committed to a rollback.

### D7 — Commit message: `rollback to <short-sha>: <msg>`
Single source of truth: build the message inside `ApplyRollback` from `target.ShortSHA` and `target.Message`. No `<short-sha>` rewriting; we trust `LogEntry.ShortSHA` as produced by `gitLogEntries`.

Rationale for including the original commit's `Message`: makes `psw log` self-describing after a rollback — readers see what state the vault was rolled back *to* without needing to look up the SHA.

### D8 — Colorize "rollback" in `psw log`
`colorizeLogMessage` in `internal/cli/log.go` switches on keywords (`add` → green, `update`/`change` → yellow, `remove` → red). Rollback fits none — and the message starts with `"rollback to ..."`. Add a `strings.Contains(low, "rollback")` case before the `remove` case (otherwise `"rollback to"` could be partially-matched by some other keyword in the future).

Color choice: `color.InPurple` — distinct from add/update/remove, and `go-color` exposes it (`InPurple` is the standard purple/magenta). Semantically rollback is a bulk replace, not a pure add/update/remove; a fourth color reads well.

Order the cases so `rollback` matches before `remove` (defensive — current commit messages don't contain both, but the explicit ordering documents intent).

### D9 — `GetOrCreateForMutate` does the pull; rollback runs on the post-pull view
Same as add/change/remove. If a peer pushed since our last sync, `GetOrCreateForMutate` will fetch+merge first, *then* return the store. Rollback then writes a new commit on top of that merged HEAD. The user picks from a commit list that already reflects the merge.

One subtle implication: the `LogEntry.ShortSHA`s in the picker correspond to commits in *our* local repo after the pull-merge. Some of those may be merge commits authored by us during the pull. Rolling back to a merge commit is fine — its `storage.psw` blob is well-formed (it's the merged state).

### D10 — Non-TTY behavior
`psw rollback` requires a TTY for the picker and the y/n. If stdin is non-TTY:
- The picker (`storage.GetRecordNameInteractive`) does NOT have a non-TTY check today (it just launches bubbletea, which would fail or hang).
- `prompt.YesOrNo` returns `false` on non-TTY (scripting-safe).

For consistency with `psw menu`'s non-TTY-stdin policy, the rollback CLI should detect non-TTY stdin up front and return a red error + `errSilentExit`. Implementation: gate on `term.IsTerminal(int(os.Stdin.Fd()))`. There's no existing exported helper — `prompt.isTTY()` is private. Add `prompt.IsTTY()` (capitalize the existing helper) and reuse, or duplicate the one-liner in rollback.go. Decision: capitalize once — `prompt.IsTTY()` — and reuse. The same helper might be useful for future commands.

### D11 — Push failure during confirm window: existing best-effort posture handles it
If a peer pushes between our `GetOrCreateForMutate` (which pulled) and `ApplyRollback` (which commits + pushes), our push will fail non-fast-forward. `GitCommit` already warns yellow on push failure and continues. Local commit lands; the next mutation on this device auto-pulls, smart-merges, and pushes. Same posture as a contended `psw add`. No special handling in rollback.

## File-by-file changes

### `internal/storage/rollback.go` (new)
```go
package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// LoadCommitRecords reads <ref>:storage.psw and returns the decoded record
// slice. Maps any decrypt failure to ErrForkUndecryptable (same posture as
// pull_merge.go's fastForward / divergentMerge).
func LoadCommitRecords(ref, password string) ([]Record, error) {
	blob, err := gitShowBlob(ref, Paths.storageFileName)
	if err != nil {
		return nil, fmt.Errorf("read commit blob: %w", err)
	}
	plain, err := decryptBytes(blob, password)
	if err != nil {
		// decryptBytes' only failure modes (bad magic, wrong password) all
		// imply the snapshot can't be read under the current password.
		return nil, ErrForkUndecryptable
	}
	var records []Record
	if err := json.Unmarshal([]byte(plain), &records); err != nil {
		return nil, fmt.Errorf("parse commit records: %w", err)
	}
	return records, nil
}

// ApplyRollback replaces store.Records with the supplied snapshot (each
// MTime stamped to now), saves, and commits. Push happens inside GitCommit
// (best-effort).
func ApplyRollback(store *Storage, target LogEntry, records []Record) error {
	now := time.Now().UnixMilli()
	stamped := make([]Record, len(records))
	for i, r := range records {
		r.MTime = now
		stamped[i] = r
	}
	store.Records = stamped
	if err := store.Save(); err != nil {
		return err
	}
	GitCommit(fmt.Sprintf("rollback to %s: %s", target.ShortSHA, target.Message))
	return nil
}

// HeadShortSHA returns the 7-char short SHA of HEAD. Used by CLI rollback
// to filter HEAD out of the candidate list. Returns wrapped revParse error
// when HEAD can't be resolved (e.g. unborn branch).
func HeadShortSHA() (string, error) {
	sha, err := revParse("HEAD")
	if err != nil {
		return "", err
	}
	if len(sha) < 7 {
		return "", fmt.Errorf("HEAD sha too short: %q", sha)
	}
	return sha[:7], nil
}
```

Notes:
- `LoadCommitRecords` is intentionally a separate function from the existing private `decryptBlobToRecords` to make the wrong-password→ErrForkUndecryptable mapping explicit. We could alternatively make `decryptBlobToRecords` itself map and remove the inline mapping in `pull_merge.go`, but that's a refactor beyond Phase 2's scope.
- `ApplyRollback` builds a fresh slice rather than mutating the caller's slice so a CLI caller doesn't see surprising aliasing if it kept a handle to the pre-stamp records.

### `internal/cli/rollback.go` (new)
```go
package cli

import (
	"errors"
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back records to a past commit's snapshot",
	Long: `Pick a past commit; rollback creates a new commit replacing the
current records with that snapshot. History is preserved.

Rolling back across a 'change main' boundary is refused — the target commit
would be encrypted under a different main password.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !prompt.IsTTY() {
			fmt.Println(color.InRed("rollback requires an interactive terminal"))
			return errSilentExit
		}

		ok, err := storage.IsGitRepo()
		if err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		if !ok {
			fmt.Println(color.InRed("Storage is not a git repository; nothing to roll back to."))
			return errSilentExit
		}

		store, err := storage.GetOrCreateForMutate()
		if done, ret := handleCmdErr(err); done {
			return ret
		}

		entries, err := storage.GitLog()
		if done, ret := handleCmdErr(err); done {
			return ret
		}
		headShort, err := storage.HeadShortSHA()
		if err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}

		picks := make([]storage.LogEntry, 0, len(entries))
		for _, e := range entries {
			if e.ShortSHA != headShort {
				picks = append(picks, e)
			}
		}
		if len(picks) == 0 {
			fmt.Println("No previous commits to roll back to.")
			return nil
		}

		byLabel := make(map[string]storage.LogEntry, len(picks))
		labels := make([]string, len(picks))
		for i, e := range picks {
			label := fmt.Sprintf("%s  %s  %s",
				e.ShortSHA,
				e.Time.Format("2006-01-02 15:04"),
				e.Message,
			)
			labels[i] = label
			byLabel[label] = e
		}

		chosen, err := storage.GetRecordNameInteractive(labels, nil)
		if errors.Is(err, storage.ErrPickerCancelled) {
			return nil
		}
		if done, ret := handleCmdErr(err); done {
			return ret
		}
		target, ok := byLabel[chosen]
		if !ok {
			fmt.Println(color.InRed("internal: picker returned unrecognized label"))
			return errSilentExit
		}

		records, err := storage.LoadCommitRecords(target.ShortSHA, store.MainPassword)
		if errors.Is(err, storage.ErrForkUndecryptable) {
			fmt.Println(color.InRed("This commit was encrypted under a previous main password; cannot roll back across that."))
			return errSilentExit
		}
		if done, ret := handleCmdErr(err); done {
			return ret
		}

		if !prompt.YesOrNo(fmt.Sprintf("Replace records with snapshot from %s (%s)?", target.ShortSHA, target.Message)) {
			return nil
		}

		if done, ret := handleCmdErr(storage.ApplyRollback(store, target, records)); done {
			return ret
		}
		fmt.Printf("Rolled back to %s: %s\n", color.InCyan(target.ShortSHA), target.Message)
		return nil
	},
}
```

### `internal/cli/root.go` (edit)
Single-line change to the `AddCommand` call:
```go
rootCmd.AddCommand(getCmd, addCmd, changeCmd, removeCmd, menuCmd, logCmd, rollbackCmd, versionCmd)
```
Insert `rollbackCmd` between `logCmd` and `versionCmd` (mirrors the natural order — destructive/mutating commands grouped, version last).

### `internal/cli/log.go` (edit)
Extend `colorizeLogMessage` with a rollback case, ordered before `remove` to avoid any partial-match drift in the future:
```go
func colorizeLogMessage(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "rollback"):
		return color.InPurple(msg)
	case strings.Contains(low, "add"):
		return color.InGreen(msg)
	case strings.Contains(low, "update"), strings.Contains(low, "change"):
		return color.InYellow(msg)
	case strings.Contains(low, "remove"):
		return color.InRed(msg)
	}
	return msg
}
```

### `internal/prompt/prompts.go` (edit)
Promote the existing private `isTTY()` helper to exported `IsTTY()`. Single rename, no other change. Callers inside the package switch to `IsTTY()`.

Alternative: keep `isTTY()` private and duplicate the one-liner in rollback.go. Rejected — clearer intent to have one shared helper.

### `tests/rollback_test.go` (new)
Test plan in next section.

## Test plan (`tests/rollback_test.go`)

All tests use `newGitVault` (PSW_GIT enabled, PSW_GIT_REMOTE=0) unless they need multi-device. Sleeps between mutations where needed to ensure mtime separation (use the existing `mtimeSeparation = 20 * time.Millisecond` from `sync_test.go`; promote to a test-helpers file if more than one rollback test needs it).

### TestRollback_RestoresSnapshot
```
vault, env := newGitVault(t)
psw add foo -u u --password=v1
psw add bar -u u --password=v2
psw add baz -u u --password=v3
// Snapshot to roll back to: after adding `bar`, before `baz`. We capture the
// short-sha of that commit by running `psw log` and grepping for "bar".
... resolve targetSHA from log output ...

// Drive the interactive picker via expect-style stdin... actually no — psw rollback
// requires a TTY. Integration test path: implement a non-TTY env var bypass for
// tests? Better: add an opt-in env var like PSW_ROLLBACK_TARGET=<sha> that, when set
// AND PSW_ROLLBACK_YES=1 are both present, skips both the picker and the y/n.
```

**Open subdecision:** the integration tests need a non-TTY path. Three options:

**(a) Add scripting env vars** `PSW_ROLLBACK_TARGET=<sha>` and `PSW_ROLLBACK_YES=1`. The rollback CLI, when both are set, bypasses the picker and the y/n. Mirrors `PSW_MAIN_PASSWORD` semantics. Lowest test friction; ships a small but real surface for scripting.

**(b) Driver test using a pty** (`creack/pty` or similar). Heavyweight; no precedent in current `tests/`.

**(c) Unit-test the core in `internal/storage`** via `LoadCommitRecords` + `ApplyRollback` directly, and rely on the CLI integration tests only for the non-happy paths (no-git-repo, change-main-refused, no-prior-commits) which don't need TTY interaction.

Decision: **(c) for the happy paths + (a) for the multi-device convergence test**, because that test specifically validates the *commit* lands and the *push* propagates — which needs the full CLI path.

Concretely:
- Storage-level unit tests (`internal/storage/rollback_test.go`) for `LoadCommitRecords` (good password / wrong password) and `ApplyRollback` (records replaced, mtimes stamped, commit lands).
- CLI integration tests for failure modes that don't need TTY interaction (no-git-repo, no-prior-commits, change-main-refused, non-TTY-error).
- CLI integration tests gated on env vars `PSW_ROLLBACK_TARGET` + `PSW_ROLLBACK_YES` for the happy path and the multi-device convergence test.

Adds a small but justifiable scripting surface (mirrors the existing `PSW_MAIN_PASSWORD` / `PSW_GIT` / `PSW_GIT_REMOTE` conventions).

### Concrete test cases

**Unit tests in `internal/storage/rollback_test.go` (new):**

1. `TestLoadCommitRecords_OK`: write a vault, add 2 records, snapshot commit. `LoadCommitRecords(snapshotSHA, password)` returns those 2 records.
2. `TestLoadCommitRecords_WrongPassword`: same setup; call with a wrong password → returns `ErrForkUndecryptable`.
3. `TestLoadCommitRecords_MissingCommit`: nonexistent SHA → returns wrapped `gitShowBlob` error (not ErrForkUndecryptable).
4. `TestApplyRollback_StampsAllMTimes`: build a store, call ApplyRollback with records that have stale MTimes, assert every record's MTime ≥ pre-call `time.Now().UnixMilli()`.
5. `TestApplyRollback_CommitMessageFormat`: verify the latest commit's message equals `fmt.Sprintf("rollback to %s: %s", target.ShortSHA, target.Message)`.

These run in-process via the storage package; no `psw` binary needed, no TTY.

**Integration tests in `tests/rollback_test.go` (new):**

6. `TestRollback_NoGitRepo`: `newVault(t)` (PSW_GIT=0). `runPsw(t, vault, "rollback")` → exit 1, stdout contains `Storage is not a git repository`.
7. `TestRollback_NoPriorCommits`: `newGitVault(t)`, no adds (just the initial-main-password commit which becomes HEAD after the first-run banner). `runPswEnv(t, vault, env, "rollback")` → exit 0, stdout contains `No previous commits to roll back to.`. **Note:** rollback requires TTY but this path exits before launching any picker. Hmm — need to allow exit before the TTY check, OR move the TTY check after this branch. Move the TTY check to immediately before the picker launch (after the empty-picks-list short-circuit). Update the implementation accordingly.
8. `TestRollback_NoTTY_HappyPath`: with `PSW_ROLLBACK_TARGET` and `PSW_ROLLBACK_YES` UNSET, a vault with multiple commits → exit 1, stderr or stdout mentions "interactive terminal". Confirms the TTY guard fires only when the picker would actually run.
9. `TestRollback_ChangeMainRefused`: `newGitVault`, add `alpha`, change main, add `beta`. Resolve the alpha-add commit SHA. Run rollback against it (via scripting env vars) → exit 1, stdout contains `encrypted under a previous main password`.
10. `TestRollback_Happy_RestoresSnapshot` (scripting): newGitVault; add `alpha`, `beta`, `gamma`; resolve the alpha-add commit SHA; rollback to it. Verify post-rollback list contains only `alpha`, not `beta` or `gamma`. Verify `psw log` shows the rollback commit with message `rollback to <sha>: added new record alpha`.
11. `TestRollback_Happy_Offline` (scripting): same as above but with `PSW_GIT_REMOTE=0`. Verify the local commit lands (bareCommitCount unchanged if multi-device; or via `git log` against the local repo).
12. `TestRollback_DivergentPeerConverges` (scripting + multi-device): two peers A and B, both initially synced with records `alpha, beta, gamma`. A rolls back to a commit with only `alpha`. B (still online) makes another mutation; B's pull-merge sees A's rollback, mtime LWW + L1 R0 logic produces a final state. Assert: `alpha` survives on B with A's mtime; `beta`/`gamma` are dropped from B's view (assuming B didn't modify them post-fork). Confirms the LWW mtime stamp does its job.
13. `TestRollback_PostForkModificationSurvives` (scripting + multi-device): A and B synced with `alpha, beta`. Both go offline. A rolls back to a commit with only `alpha`. B modifies `beta` (`change` flow). Both come online. Final state on B: `alpha` from A (LWW), `beta` from B (modification beats removal). Confirms the documented edge case from D4.

### Scripting env-var additions (D-bonus)
- `PSW_ROLLBACK_TARGET=<short-sha>` — skip picker, use this SHA.
- `PSW_ROLLBACK_YES=1` — skip y/n, assume yes.
- Both required together. If only one is set: warn yellow + fall back to interactive (or refuse + exit 1 — to be decided). Recommend: refuse + exit 1 with a clear error, so test failures don't silently hang on a TUI.

Document in CLAUDE.md alongside the existing `PSW_MAIN_PASSWORD` / `PSW_NEW_MAIN_PASSWORD` listing.

## Risk surface

| Risk | Mitigation |
|---|---|
| Picker label parsing brittleness (parsing SHA out of the label) | Use a `map[label]LogEntry` lookup — label is the *key*, no parsing. |
| Wrong-password mapping leaks non-decrypt errors | `LoadCommitRecords` only maps the decrypt-step error; `gitShowBlob` errors propagate wrapped. |
| Stamping mtimes mutates caller's records slice | `ApplyRollback` allocates a fresh slice. |
| Push race during confirm window | Existing `GitCommit` best-effort push posture handles this; next mutation auto-pulls and converges. |
| TTY guard fires before "no prior commits" message | Reorder: empty-list branch before TTY check (otherwise users without TTY get a misleading error). |
| Rolling back to a merge commit | No special handling; merge commits have valid `storage.psw` blobs. Picker shows them with their `merge: ...` subject. |
| Records added post-fork survive on divergent peers (modification-beats-removal) | Documented in D4 + covered by `TestRollback_PostForkModificationSurvives`. Matches existing remove semantics; not a regression. |
| Storage with very long commit history overwhelms picker | Bubbles/list paginates; cosmetic only. No special handling. |
| `colorizeLogMessage` falsely matches "rollback" inside an unrelated future message | Low risk; the keyword is specific. Order before `remove` is just defensive. |

## Verification steps (post-implementation)

1. `make test` — all green, including the new rollback unit + integration tests.
2. `make build` — clean build of both binaries.
3. `psw version` prints `psw v0.29` pre-Phase-3, `psw v0.30` post.
4. Manual CLI smoke-test against a local vault:
   - `psw add alpha -u u --password=p1`
   - `psw add beta -u u --password=p2`
   - `psw add gamma -u u --password=p3`
   - `psw log` — confirm three add commits + initial-main-password commit; copy alpha's short-sha.
   - `psw rollback` — picker opens listing all four commits *except* HEAD; pick alpha's commit; y/n shows `Replace records with snapshot from <sha> (added new record alpha)?`; answer `y`; expect "Rolled back to <sha>: added new record alpha".
   - `psw` (list) — only `alpha` remains.
   - `psw log` — new top entry "rollback to <sha>: added new record alpha", rendered purple.
5. Manual error-path smoke-test:
   - Vault with only the initial commit → `psw rollback` prints "No previous commits to roll back to." and exits 0.
   - PSW_GIT=0 → `psw rollback` prints the red "not a git repository" message and exits 1.
   - Add a record, run `psw change main` (set new password), run `psw rollback`, pick a commit from before the change-main — expect red "encrypted under a previous main password" + exit 1.
   - Non-TTY: `echo q | psw rollback` (in a real terminal but with non-TTY stdin) → red "requires an interactive terminal" + exit 1.
6. Multi-device manual smoke (optional, parallels `TestRollback_DivergentPeerConverges`): set up two peers against a bare remote (mirrors `newGitVaultWithRemote`). A rolls back. B does a mutation. Verify B's resulting state matches the test's expected convergence.

---

# Phase 3 detail — Version bump to 0.30

Trivial phase; included here so a fresh-context Claude can ship Phase 2 + 3 together if desired.

## Scope confirmation

In:
- Edit `VERSION` from `0.29` to `0.30`.
- Verify `psw version` reflects the bump after `make build`.
- One commit.

Out: anything else.

## Steps

1. `Read VERSION` → confirm contents are `0.29\n` (or just `0.29`).
2. `Edit VERSION` → replace `0.29` with `0.30`.
3. `make build` — verify the ldflag picks up the new version (`./bin/psw version` prints `psw v0.30`).
4. Commit message: match repo convention. Recent commits use `feat:` and `refactor:` prefixes; no `chore:` precedent in the recent history shown by `git log --oneline`. Suggested message: `chore: bump VERSION to 0.30` if the user accepts introducing `chore:`, or simply `bump VERSION to 0.30` if they prefer not to introduce a new prefix. Confirm with user before committing.

## Verification

- `./bin/psw version` prints `psw v0.30`.
- `git log -1` shows the bump commit.
- `make test` still green (the version string isn't asserted anywhere in `tests/`; verified separately).

## Risk surface

| Risk | Mitigation |
|---|---|
| Forgetting to rebuild after the VERSION change | `make build` is part of verification. |
| Stale `bin/pswcfg.toml` from a prior build leaks into the new binary's data dir | Pre-existing; not a Phase 3 concern. |
| `gomod2nix.toml` / `vendorHash` drift if any dep was added in Phase 2 | None added — Phase 2 introduces no new third-party deps; everything reuses existing imports (`bubbletea`, `bubbles`, `lipgloss`, `go-color`, `go-git`, `argon2`, `cobra`). Sanity-check `go mod tidy` produces no diff. |

## Out-of-scope follow-ups (after Phase 3 ships)

- Adding a `psw rollback <short-sha>` positional arg (skip picker for scripting / copy-paste from `psw log`).
- Adding `psw rollback --yes` to skip the confirm (only useful with the positional arg).
- Exposing rollback in the menu — explicitly excluded from this initiative; revisit only if user demand surfaces.
- Refactor: collapse `LoadCommitRecords` and `decryptBlobToRecords` into one helper, with the wrong-password→ErrForkUndecryptable mapping centralized. Out-of-scope here to keep the diff small; worth a refactor pass when next touching `pull_merge.go`.
