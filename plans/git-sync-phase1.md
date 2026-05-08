# Phase 1 detail: Optional remote sync with smart merge

_Last updated: 2026-05-08 — commit `6120ee3` (pre-Phase 1). Sibling: `git-sync.md` (parent plan)._

## Status: **Delivered** (uncommitted)

48 integration tests + 16 merge unit subtests green. `go vet` and `gofmt` clean. Two cleanup passes applied after first pass: stdlib-idiom review (slices, `url.URL.Redacted()`, `CombinedOutput`, network timeout, error message dedup) and naming/comment review (case-switch readability, function rename consistency, em-dash removal).

### Deviations from this plan

- **`change main` routes through `GetOrCreateForMutate`.** This plan had `change main` use `storage.Get` directly. That path doesn't call `initGitRepoIfNotExists` so `Paths.gitRepoExists` stayed false and `GitCommit` silently no-op'd in tests with git enabled. Routing through `GetOrCreateForMutate` fixes that and gives `change main` pull-before-mutate + cross-merge-bail semantics. Side-effect: dropped the ensure-double-confirm of the *current* password (only the new password still requires confirmation).
- **`git init --initial-branch=main`.** Original plan kept `git init` plain. Standardizing on `main` aligns new vaults with the bare-repo default in tests and avoids ambiguity when the user's global `init.defaultBranch` is unset. Existing vaults are untouched.
- **Three `runGit` variants** instead of the single `runGit` originally sketched:
  - `runGit(args...)` — combined stdout+stderr, no timeout (local ops).
  - `runGitNetwork(args...)` — combined stdout+stderr, **30s `gitNetworkTimeout` via `exec.CommandContext`** (fetch + push only). Closes the "network timeout" open question from `git-sync.md`.
  - `runGitStdout(args...)` — stdout only with stderr surfaced in error message (used by `gitShowBlob`).
- **Warning helper renamed `printWarn`** (was `warnYellow`). Semantic over stylistic; matches the convention that public helpers describe purpose.
- **Centralized `printForkUndecryptable()` in `internal/cli/helpers.go`** — used by `add`, `change`, and `remove` instead of three duplicated red-print blocks.
- **Names in merge logic:** indexes are `forkByName` / `localByName` / `remoteByName` (not `fIdx` / `lIdx` / `rIdx`); per-iteration locals are `forkRec` / `localRec` / `remoteRec`; presence booleans are `inFork` / `inLocal` / `inRemote`. Each case statement reads as the truth-table notation: `case inLocal && !inRemote && inFork:`.
- **`url.URL.Redacted()`** instead of a custom `redactURL` (drops manual user-info handling).
- **`(*Storage).ToJson()` reused in `divergentMerge`** instead of inline `json.MarshalIndent` — keeps encoding centralized.

### File layout (delivered)

| File | Status |
|---|---|
| `internal/storage/storage.go` | Modified — `Record.MTime`, `GetOrCreateForRead`/`ForMutate` split. |
| `internal/storage/config.go` | Modified — `Config.Remote`. |
| `internal/storage/encryption.go` | Modified — exposes `decryptBytes`. |
| `internal/storage/git.go` | Modified — `--initial-branch=main`, `GitPush` tail-call in `GitCommit`. |
| `internal/storage/git_sync.go` | **New** — fetch/push/merge driver, 3 runGit variants, `printWarn`, `redactURL`, `ErrForkUndecryptable`. |
| `internal/storage/merge.go` | **New** — pure `mergeRecords` + `mergeSummary`. |
| `internal/storage/merge_test.go` | **New** — 14 table-driven subtests + 2 invariant tests. |
| `internal/cli/helpers.go` | Modified — `printForkUndecryptable()`. |
| `internal/cli/{add,change,remove}.go` | Modified — `GetOrCreateForMutate` + fork-undecryptable handling. |
| `internal/cli/{root,get}.go` | Modified — `GetOrCreateForRead`. |
| `internal/cli/log.go` | Unchanged. |
| `pswcfg-template.toml` | Modified — commented `# remote = ...` line. |
| `tests/git_helpers_test.go` | **New** — `newBareRemote`, `newGitVaultWithRemote`, `addPeer`, `runGitInBare`, `bareCommitCount`. |
| `tests/sync_test.go` | **New** — 8 integration test cases. |
| `tests/log_test.go` | Modified — `PSW_GIT_REMOTE: "0"` added to env. |
| `CLAUDE.md` | Modified — added "Remote sync (optional)" subsection + `PSW_GIT_REMOTE` doc line. |

### What Phase 2 needs to know

- Wrap `runGitNetwork` calls inside `GitFetch` and `GitPush` with the spinner. The function signatures are stable.
- TTY-gating belongs in the spinner package; `runGitNetwork` returns immediately on success so the spinner threshold (250ms) handles fast remotes naturally.
- `printWarn` writes to stderr — same stream the spinner should use for ANSI output.

### What Phase 3 needs to know

- All shell-out is in `internal/storage/git_sync.go` and `internal/storage/git.go`. The split keeps Phase 1 sync semantics testable separately from any go-git auth complexity.
- Format-stable: `Record.MTime` uses `omitempty` (zero stays absent in JSON); legacy records deserialize as zero and lose any LWW comparison.
- The `runGit*` family wraps every git invocation. To swap to go-git, replace the bodies of those three helpers (and `gitShowBlob`, `gitMergeBase`, `revParse`, `isAncestor`); call sites unchanged.

---

## Pre-flight (no longer needed — Phase 1 is delivered)

Kept for archive:

- `git rev-parse HEAD` → must equal `6120ee3` or this doc may be stale; if drifted, re-read all files in **Files touched** and reconcile with what's below before coding.
- Confirm `internal/storage/git.go` is still ~115 lines and shells out via `os/exec`. The plan assumes this; if it has migrated to go-git, Phase 1 is moot.
- `make test` baseline must be green on a clean checkout. Each new file we add ships with its own test coverage; do not let baseline regress at any commit boundary.

## Goal recap

Pull-before-mutate / push-after-commit when `pswcfg.toml` has `remote = "..."`. 3-way smart merge using a per-record `MTime` field. Read paths never touch the network. No spinner yet (Phase 2). Still shelling out to `git` (Phase 3 swaps to go-git).

## Files touched

| File | Change |
|---|---|
| `internal/storage/storage.go` | Add `Record.MTime int64`. Stamp in `AddRecord`/`UpdateRecord`. Split `GetOrCreateIfNotExists` into `GetOrCreateForRead` (no pull) and `GetOrCreateForMutate` (pulls before decrypt). |
| `internal/storage/config.go` | Add `Config.Remote string` (`toml:"remote"`). |
| `internal/storage/git.go` | Keep `IsGitRepo`, `GitLog`, `initGitRepoIfNotExists`, `GitCommit`. `GitCommit` gains a tail-call to `GitPush` when remote configured. |
| `internal/storage/git_sync.go` | **New.** `GitFetch`, `GitPush`, `gitMergeBase`, `gitShowBlob`, `gitPullAndMerge`, `ensureGitRemote`, `detectBranch`, `shouldUseRemote`. Sentinel `ErrForkUndecryptable`. |
| `internal/storage/merge.go` | **New.** Pure-function `mergeRecords(fork, local, remote []Record) ([]Record, mergeSummary)`. No I/O. |
| `internal/storage/merge_test.go` | **New.** Table-driven unit tests for all 8 structural cases + tie-break + case-insensitive name matching. |
| `internal/cli/add.go`, `change.go`, `remove.go` | `GetOrCreateIfNotExists()` → `GetOrCreateForMutate()`. Handle `ErrForkUndecryptable` (red message, return `errExit`). |
| `internal/cli/root.go`, `get.go`, `log.go` | `GetOrCreateIfNotExists()` → `GetOrCreateForRead()`. |
| `pswcfg-template.toml` | Add commented `# remote = "git@github.com:user/psw-storage.git"  # uncomment to enable git sync`. |
| `tests/git_helpers_test.go` | **New.** `newGitVaultWithRemote` builds a bare repo + vault. |
| `tests/sync_test.go` | **New.** Eight integration test cases (see **Test plan** below). |
| `tests/log_test.go` | Add `PSW_GIT_REMOTE: "0"` to `newGitVault` env (belt-and-suspenders against future warnings). |

Net new: ~600 lines of code + ~400 lines of tests. `git_sync.go` is the largest single file (~250 lines); split out from `git.go` so Phase 3 can swap go-git in `git_sync.go` mostly self-contained.

## Implementation steps (sequenced, each commit-able)

### Step 1 — `Record.MTime` field + central stamping

`internal/storage/storage.go`:

```go
type Record struct {
    Name     string `json:"name"`
    Username string `json:"user"`
    Password string `json:"pass"`
    Value    string `json:"value"`
    MTime    int64  `json:"mtime,omitempty"`
}

func (s *Storage) AddRecord(r *Record) {
    r.MTime = time.Now().UnixMilli()
    s.Records = append(s.Records, *r)
    s.sortRecords()
}

func (s *Storage) UpdateRecord(name string, updatedRecord Record) {
    for i, r := range s.Records {
        if strings.EqualFold(r.Name, name) {
            updatedRecord.MTime = time.Now().UnixMilli()
            s.Records[i] = updatedRecord
            s.sortRecords()
            return
        }
    }
}
```

- `omitempty` keeps disk-format diff minimal for legacy records (zero stays absent in JSON).
- `change main` does **not** go through these; it only re-encrypts. So mtimes stay put across a password rotation — required for case-4/5 merge correctness.
- Existing tests pass unchanged: nothing observes mtime yet.
- Verify: `make test` after this step.

### Step 2 — `Config.Remote` and template comment

`internal/storage/config.go`:

```go
type Config struct {
    ClipboardTimeout int    `toml:"clipboard_timeout"`
    Remote           string `toml:"remote"`
}
```

`pswcfg-template.toml`:

```toml
clipboard_timeout = 20  # duration in seconds
# remote = "git@github.com:user/psw-storage.git"  # uncomment to enable git sync (pull before mutate, push after commit)
```

- TOML `Unmarshal` into a missing field leaves zero (`""`); presence-of-key is the toggle.
- Existing vaults already have `pswcfg.toml`; adding a key means new vaults get the comment, old vaults don't. That's fine — the line is informational; not having it in old vaults doesn't break anything.

### Step 3 — `git_sync.go` skeleton: helpers, env gating, branch detection, remote setup

`internal/storage/git_sync.go`:

```go
package storage

import (
    "bytes"
    "errors"
    "fmt"
    "log/slog"
    "net/url"
    "os"
    "os/exec"
    "strings"

    color "github.com/TwiN/go-color"
)

var ErrForkUndecryptable = errors.New("fork commit cannot be decrypted with current main password")

// shouldUseRemote: pull/push allowed?
//   PSW_GIT=0       → no git at all
//   PSW_GIT_REMOTE=0 → local commits OK, no network
//   no remote in config → no network
func shouldUseRemote() bool {
    if os.Getenv("PSW_GIT") == "0" {
        return false
    }
    if os.Getenv("PSW_GIT_REMOTE") == "0" {
        return false
    }
    if !Paths.gitRepoExists {
        return false
    }
    return AppConfig.Remote != ""
}

func detectBranch() (string, error) {
    out, err := runGit("symbolic-ref", "--short", "HEAD")
    if err != nil {
        return "", fmt.Errorf("detect branch: %w", err)
    }
    return strings.TrimSpace(out), nil
}

// runGit always sets cwd to Paths.storagePath; returns combined output for diagnostics.
func runGit(args ...string) (string, error) {
    cmd := exec.Command("git", args...)
    cmd.Dir = Paths.storagePath
    var buf bytes.Buffer
    cmd.Stdout = &buf
    cmd.Stderr = &buf
    err := cmd.Run()
    return buf.String(), err
}

// ensureGitRemote idempotently sets `origin` to AppConfig.Remote.
func ensureGitRemote() error {
    out, err := runGit("remote", "get-url", "origin")
    if err != nil {
        // remote doesn't exist yet
        if _, addErr := runGit("remote", "add", "origin", AppConfig.Remote); addErr != nil {
            return fmt.Errorf("git remote add: %w", addErr)
        }
        return nil
    }
    if strings.TrimSpace(out) == AppConfig.Remote {
        return nil
    }
    if _, err := runGit("remote", "set-url", "origin", AppConfig.Remote); err != nil {
        return fmt.Errorf("git remote set-url: %w", err)
    }
    return nil
}

func redactURL(u string) string {
    p, err := url.Parse(u)
    if err != nil || p.User == nil {
        return u
    }
    p.User = url.User(p.User.Username())
    return p.String()
}

func warnYellow(format string, args ...any) {
    fmt.Fprintln(os.Stderr, color.InYellow(fmt.Sprintf(format, args...)))
}
```

Notes:
- `runGit` returns combined stdout+stderr for warnings — git's failure messages live on stderr.
- `ensureGitRemote` runs every command — cheap (one fork+exec). If a user manually mutates `origin`, we'll silently overwrite it; document as a known consequence.
- `redactURL` strips the password component (`https://user:pass@host` → `https://user@host`). Username alone is not a secret.
- All warnings go to stderr (yellow) so stdout stays parseable for scripting (`psw get --stdout`).

### Step 4 — Push (`GitPush`) wired into `GitCommit`

`internal/storage/git_sync.go`:

```go
func GitPush() {
    if !shouldUseRemote() {
        return
    }
    if err := ensureGitRemote(); err != nil {
        warnYellow("git remote setup failed: %v", err)
        return
    }
    branch, err := detectBranch()
    if err != nil {
        warnYellow("git push: %v", err)
        return
    }
    out, err := runGit("push", "origin", branch)
    if err != nil {
        slog.Debug("git push failed", "remote", redactURL(AppConfig.Remote), "branch", branch, "output", out)
        warnYellow("git push to %s failed: %s", redactURL(AppConfig.Remote), strings.TrimSpace(out))
        return
    }
    slog.Debug("git push ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
}
```

Append to `GitCommit` in `internal/storage/git.go`:

```go
func GitCommit(message string) error {
    // ... existing add/commit ...
    GitPush()
    return nil
}
```

- Push errors are **never** propagated as Go errors. Per plan: warn yellow, commit stays local, next pull-before-mutate will reconcile.
- `GitPush` is a `func()` not `func() error` to make this contract loud at the call site.
- The `slog.Debug` line uses redacted URL — never the raw token-bearing URL.
- A pushed-but-rejected case (non-fast-forward because remote moved on) prints the git output and the user can guess what happened. The next mutation's pre-pull will then merge.

### Step 5 — Fetch + blob reading

`internal/storage/git_sync.go`:

```go
func GitFetch() error {
    if !shouldUseRemote() {
        return nil
    }
    if err := ensureGitRemote(); err != nil {
        return fmt.Errorf("ensure remote: %w", err)
    }
    branch, err := detectBranch()
    if err != nil {
        return err
    }
    if out, err := runGit("fetch", "origin", branch); err != nil {
        return fmt.Errorf("git fetch %s: %s", redactURL(AppConfig.Remote), strings.TrimSpace(out))
    }
    return nil
}

// gitShowBlob reads <ref>:<path> and returns raw bytes.
// Used for storage.psw at fork and remote tips.
func gitShowBlob(ref, path string) ([]byte, error) {
    cmd := exec.Command("git", "-C", Paths.storagePath, "show", ref+":"+path)
    return cmd.Output()
}

func gitMergeBase(a, b string) (string, error) {
    out, err := runGit("merge-base", a, b)
    if err != nil {
        return "", fmt.Errorf("merge-base: %s", strings.TrimSpace(out))
    }
    return strings.TrimSpace(out), nil
}
```

- `gitShowBlob` uses `cmd.Output()` (stdout-only) so we get raw bytes, not stderr-mixed.

### Step 6 — `mergeRecords` (pure function)

`internal/storage/merge.go`:

```go
package storage

import (
    "sort"
    "strings"
)

type mergeAction int

const (
    actionUnchanged mergeAction = iota
    actionAddedFromRemote
    actionReplacedFromRemote
    actionDroppedByRemote
    actionKeptLocalOverRemoval
    actionKeptLocalNewer
)

type mergeChange struct {
    name   string
    action mergeAction
}

type mergeSummary struct {
    changes []mergeChange
}

// mergeRecords applies a 3-way merge with per-record mtime as the LWW signal.
// Equal mtime on a content conflict → remote wins (deterministic tiebreaker).
//
// Cases (F=fork present, L=local present, R=remote present):
//   F0 L0 R0 → impossible
//   F0 L1 R0 → local-added since fork; keep
//   F0 L0 R1 → remote-added since fork; pull in
//   F0 L1 R1 → independent add of same name; mtime-LWW
//   F1 L1 R0 → remote removed; if L.mtime > F.mtime, modification beats removal; else drop
//   F1 L0 R1 → local removed; if R.mtime > F.mtime, modification beats removal; else drop
//   F1 L1 R1 → standard content reconcile; mtime-LWW (remote wins on tie)
//   F1 L0 R0 → both removed; drop
func mergeRecords(fork, local, remote []Record) ([]Record, mergeSummary) {
    fIdx := indexByName(fork)
    lIdx := indexByName(local)
    rIdx := indexByName(remote)

    keys := unionKeys(fIdx, lIdx, rIdx)
    sort.Strings(keys)

    var merged []Record
    var summary mergeSummary

    for _, k := range keys {
        f, fOK := fIdx[k]
        l, lOK := lIdx[k]
        r, rOK := rIdx[k]

        switch {
        case !lOK && !rOK:
            // both removed (or never existed) — drop
            continue
        case lOK && !fOK && !rOK:
            merged = append(merged, l)
        case !lOK && !fOK && rOK:
            merged = append(merged, r)
            summary.changes = append(summary.changes, mergeChange{r.Name, actionAddedFromRemote})
        case lOK && rOK && !fOK:
            chosen, action := pickByMTime(l, r)
            merged = append(merged, chosen)
            summary.changes = append(summary.changes, mergeChange{chosen.Name, action})
        case lOK && !rOK && fOK:
            if l.MTime > f.MTime {
                merged = append(merged, l)
                summary.changes = append(summary.changes, mergeChange{l.Name, actionKeptLocalOverRemoval})
            } else {
                summary.changes = append(summary.changes, mergeChange{l.Name, actionDroppedByRemote})
            }
        case !lOK && rOK && fOK:
            if r.MTime > f.MTime {
                merged = append(merged, r)
                summary.changes = append(summary.changes, mergeChange{r.Name, actionAddedFromRemote})
            }
        case lOK && rOK && fOK:
            chosen, action := pickByMTime(l, r)
            merged = append(merged, chosen)
            if action != actionUnchanged {
                summary.changes = append(summary.changes, mergeChange{chosen.Name, action})
            }
        }
    }

    sort.Slice(merged, func(i, j int) bool {
        return strings.Compare(merged[i].Name, merged[j].Name) < 0
    })
    return merged, summary
}

func indexByName(rs []Record) map[string]Record {
    m := make(map[string]Record, len(rs))
    for _, r := range rs {
        m[strings.ToLower(r.Name)] = r
    }
    return m
}

func unionKeys(maps ...map[string]Record) []string {
    seen := map[string]struct{}{}
    for _, m := range maps {
        for k := range m {
            seen[k] = struct{}{}
        }
    }
    out := make([]string, 0, len(seen))
    for k := range seen {
        out = append(out, k)
    }
    return out
}

// pickByMTime: higher mtime wins; tie → remote wins (deterministic).
// If the records are byte-equal, returns local with actionUnchanged.
func pickByMTime(l, r Record) (Record, mergeAction) {
    if recordsEqual(l, r) {
        return l, actionUnchanged
    }
    if l.MTime > r.MTime {
        return l, actionKeptLocalNewer
    }
    return r, actionReplacedFromRemote
}

func recordsEqual(a, b Record) bool {
    return a.Name == b.Name && a.Username == b.Username && a.Password == b.Password && a.Value == b.Value
}

// printSummary emits a yellow stderr line if anything changed.
func (m mergeSummary) printIfNonempty() {
    if len(m.changes) == 0 {
        return
    }
    var added, replaced, dropped, kept []string
    for _, c := range m.changes {
        switch c.action {
        case actionAddedFromRemote:
            added = append(added, c.name)
        case actionReplacedFromRemote:
            replaced = append(replaced, c.name)
        case actionDroppedByRemote:
            dropped = append(dropped, c.name)
        case actionKeptLocalOverRemoval, actionKeptLocalNewer:
            kept = append(kept, c.name)
        }
    }
    if len(added) > 0 {
        warnYellow("Pulled %d new records from remote: %s", len(added), strings.Join(added, ", "))
    }
    if len(replaced) > 0 {
        warnYellow("Replaced %d records with newer version from remote: %s", len(replaced), strings.Join(replaced, ", "))
    }
    if len(dropped) > 0 {
        warnYellow("Dropped %d records removed on remote: %s", len(dropped), strings.Join(dropped, ", "))
    }
    if len(kept) > 0 {
        warnYellow("Kept %d local records (newer than remote): %s", len(kept), strings.Join(kept, ", "))
    }
}
```

Notes:
- `indexByName` lowercases keys to match repository-wide case-insensitive lookup convention (`strings.EqualFold` in `Storage.GetRecord` etc.).
- `recordsEqual` ignores `MTime` so a no-op edit on the same machine doesn't show up as "replaced from remote" when fetched.
- The output `Record` retains the chosen side's `MTime` — so subsequent merges see the right age.
- We don't need to bump merged-record mtimes; the merge is structural, not a user edit.

### Step 7 — `gitPullAndMerge` driver

`internal/storage/git_sync.go`:

```go
const branchRefPrefix = "refs/remotes/origin/"

// gitPullAndMerge fetches the remote, decides relationship, and either fast-forwards
// or builds a merge commit with smart-merged storage.psw. mainPassword is used to
// decrypt fork/remote blobs for the merge.
//
// Returns:
//   nil                       → success or nothing to do (incl. no-remote, network down).
//   ErrForkUndecryptable      → password mismatch in fork or remote (caller must exit 1).
//   wrapped error             → unexpected git failure (caller prints and exits 1).
func gitPullAndMerge(mainPassword string) error {
    if !shouldUseRemote() {
        return nil
    }

    if err := GitFetch(); err != nil {
        warnYellow("%v (continuing with local state)", err)
        return nil
    }

    branch, err := detectBranch()
    if err != nil {
        return err
    }
    remoteRef := branchRefPrefix + branch

    // If remote ref doesn't exist yet (first push from this device), nothing to merge.
    if _, err := runGit("rev-parse", "--verify", remoteRef); err != nil {
        return nil
    }

    localSHA, err := revParse("HEAD")
    if err != nil {
        return err
    }
    remoteSHA, err := revParse(remoteRef)
    if err != nil {
        return err
    }
    if localSHA == remoteSHA {
        return nil // nothing to do
    }

    // Already ahead? local contains remote; nothing to pull.
    if isAncestor(remoteSHA, localSHA) {
        return nil
    }

    // Pure fast-forward: local is ancestor of remote, and remote decrypts under our password.
    if isAncestor(localSHA, remoteSHA) {
        return fastForward(remoteRef, mainPassword)
    }

    // Divergent: 3-way merge.
    return divergentMerge(localSHA, remoteSHA, branch, mainPassword)
}

func revParse(ref string) (string, error) {
    out, err := runGit("rev-parse", ref)
    if err != nil {
        return "", fmt.Errorf("rev-parse %s: %s", ref, strings.TrimSpace(out))
    }
    return strings.TrimSpace(out), nil
}

func isAncestor(a, b string) bool {
    _, err := runGit("merge-base", "--is-ancestor", a, b)
    return err == nil
}

// fastForward: validate remote decrypts (cross-merge-bail check), then ff.
func fastForward(remoteRef, mainPassword string) error {
    if err := assertBlobDecryptable(remoteRef, mainPassword); err != nil {
        return err
    }
    if out, err := runGit("merge", "--ff-only", remoteRef); err != nil {
        return fmt.Errorf("git merge --ff-only: %s", strings.TrimSpace(out))
    }
    slog.Debug("git fast-forwarded", "to", remoteRef)
    return nil
}

// divergentMerge: read three blobs, smart-merge, write a merge commit.
func divergentMerge(localSHA, remoteSHA, branch, mainPassword string) error {
    forkSHA, err := gitMergeBase(localSHA, remoteSHA)
    if err != nil {
        return err
    }

    forkRecords, err := decryptBlobToRecords(forkSHA+":"+Paths.storageFileName, mainPassword)
    if err != nil {
        return ErrForkUndecryptable
    }
    remoteRecords, err := decryptBlobToRecords(remoteSHA+":"+Paths.storageFileName, mainPassword)
    if err != nil {
        return ErrForkUndecryptable
    }
    // Local: read from disk (already on filesystem, no need to git-show).
    localPlain, err := DecryptStringFromStorage(mainPassword)
    if err != nil {
        return fmt.Errorf("decrypt local: %w", err)
    }
    var localRecords []Record
    if err := json.Unmarshal([]byte(localPlain), &localRecords); err != nil {
        return fmt.Errorf("parse local records: %w", err)
    }

    merged, summary := mergeRecords(forkRecords, localRecords, remoteRecords)

    // Re-encrypt merged result over storage.psw.
    mergedJSON, err := json.MarshalIndent(merged, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal merged: %w", err)
    }
    if err := EncryptStringToStorage(string(mergedJSON), mainPassword); err != nil {
        return fmt.Errorf("encrypt merged: %w", err)
    }

    // Build a real merge commit with two parents using -s ours --no-commit,
    // then overwrite the staged storage.psw with our smart-merged content and commit.
    if out, err := runGit("merge", "--no-commit", "--no-ff", "-s", "ours", remoteSHA); err != nil {
        return fmt.Errorf("git merge -s ours: %s", strings.TrimSpace(out))
    }
    if out, err := runGit("add", Paths.storageFileName); err != nil {
        return fmt.Errorf("git add merged: %s", strings.TrimSpace(out))
    }
    msg := buildMergeMessage(summary)
    if out, err := runGit("commit", "--message="+msg); err != nil {
        return fmt.Errorf("git commit merge: %s", strings.TrimSpace(out))
    }

    summary.printIfNonempty()
    return nil
}

func decryptBlobToRecords(ref, password string) ([]Record, error) {
    blob, err := gitShowBlob(strings.SplitN(ref, ":", 2)[0], strings.SplitN(ref, ":", 2)[1])
    if err != nil {
        return nil, fmt.Errorf("git show %s: %w", ref, err)
    }
    plain, err := decryptBytes(blob, password)
    if err != nil {
        return nil, err
    }
    var rs []Record
    if err := json.Unmarshal([]byte(plain), &rs); err != nil {
        return nil, fmt.Errorf("parse records: %w", err)
    }
    return rs, nil
}

// decryptBytes is a sibling to decryptStringFromFile that operates on in-memory base64.
func decryptBytes(encoded []byte, password string) (string, error) {
    payload, err := base64.StdEncoding.DecodeString(string(encoded))
    if err != nil {
        return "", fmt.Errorf("decode storage: %w", err)
    }
    if len(payload) < len(magicHeaderV1)+saltLength {
        return "", errors.New("storage blob is corrupted or unrecognized")
    }
    if string(payload[:len(magicHeaderV1)]) != magicHeaderV1 {
        return "", errors.New("unrecognized storage format; expected PSW1")
    }
    salt := payload[len(magicHeaderV1) : len(magicHeaderV1)+saltLength]
    sealed := payload[len(magicHeaderV1)+saltLength:]
    key := deriveKey(password, salt)
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCMWithRandomNonce(block)
    if err != nil {
        return "", err
    }
    plain, err := gcm.Open(nil, nil, sealed, nil)
    if err != nil {
        return "", errors.New("Wrong password.")
    }
    return string(plain), nil
}

func assertBlobDecryptable(ref, password string) error {
    if _, err := decryptBlobToRecords(ref+":"+Paths.storageFileName, password); err != nil {
        return ErrForkUndecryptable
    }
    return nil
}

func buildMergeMessage(s mergeSummary) string {
    var parts []string
    var added, replaced, dropped, kept int
    for _, c := range s.changes {
        switch c.action {
        case actionAddedFromRemote:
            added++
        case actionReplacedFromRemote:
            replaced++
        case actionDroppedByRemote:
            dropped++
        case actionKeptLocalOverRemoval, actionKeptLocalNewer:
            kept++
        }
    }
    if added > 0 {
        parts = append(parts, fmt.Sprintf("+%d", added))
    }
    if replaced > 0 {
        parts = append(parts, fmt.Sprintf("~%d", replaced))
    }
    if dropped > 0 {
        parts = append(parts, fmt.Sprintf("-%d", dropped))
    }
    if kept > 0 {
        parts = append(parts, fmt.Sprintf("kept-local %d", kept))
    }
    if len(parts) == 0 {
        return "merge: no record changes"
    }
    return "merge: " + strings.Join(parts, ", ")
}
```

Notes:
- We use `decryptBytes` (new sibling of `decryptStringFromFile`) so we can decrypt blobs that aren't on disk. Pulling the AES/GCM bits up into `internal/storage/encryption.go` to expose `decryptBytes` keeps the format sealed. **Refactor:** factor out the common path; `decryptStringFromFile` calls `decryptBytes` after `os.ReadFile`. Encryption format unchanged.
- Add `encoding/json`, `encoding/base64`, `crypto/aes`, `crypto/cipher` imports to `git_sync.go` (or, if you'd rather, keep that code in a new `encryption.go` helper and call it from here).
- The merge commit message is intentionally short; full per-record summary already printed yellow on stderr. Git log readers see `merge: +2, ~1, -1` etc.
- `assertBlobDecryptable` runs on **every** fast-forward. Cost is one Argon2id (~100-300ms). Acceptable.
- The check is sufficient: in divergent case we always decrypt fork+remote anyway, which is the same check.

### Step 8 — Split `GetOrCreateIfNotExists`

`internal/storage/storage.go`:

```go
func GetOrCreateForRead() (*Storage, error)   { return getOrCreate(false) }
func GetOrCreateForMutate() (*Storage, error) { return getOrCreate(true) }

func getOrCreate(pull bool) (*Storage, error) {
    mainPassword, created, err := createEncryptedStorageIfNotExists()
    if err != nil {
        return nil, err
    }
    if err := initGitRepoIfNotExists(); err != nil {
        return nil, err
    }
    if !created && mainPassword == "" {
        mainPassword, err = prompt.PromptForMainPassword(false)
        if err != nil {
            return nil, err
        }
    }
    if pull {
        if err := gitPullAndMerge(mainPassword); err != nil {
            return nil, err
        }
    }
    return Get(mainPassword)
}
```

Delete the old `GetOrCreateIfNotExists` once all call sites are updated. **Do not** keep an alias — backwards-compat shims are explicitly disallowed (CLAUDE.md guidance).

Order matters: prompt for password **before** pull, because pull needs the password to decrypt blobs. On a fresh vault (`created==true`), `mainPassword` is already known from the creation prompt; pull is a no-op anyway because remote ref won't exist yet on the first push.

### Step 9 — CLI call-site updates

Mechanical search/replace across:

- `internal/cli/add.go`, `change.go`, `remove.go`: `storage.GetOrCreateIfNotExists()` → `storage.GetOrCreateForMutate()`. Add error handling for `ErrForkUndecryptable`:

```go
store, err := storage.GetOrCreateForMutate()
if errors.Is(err, prompt.ErrPromptCancelled) {
    return nil
}
if errors.Is(err, storage.ErrForkUndecryptable) {
    fmt.Println(color.InRed("Storage was re-encrypted under a different main password since the last sync. Push from the device that ran 'change main' first, then retry on this device."))
    return errExit
}
if err != nil {
    fmt.Println(err.Error())
    return nil
}
```

- `internal/cli/root.go`, `get.go`, `log.go`: `storage.GetOrCreateIfNotExists()` → `storage.GetOrCreateForRead()`. No `ErrForkUndecryptable` handling needed (read path doesn't pull).

- `change main` is special: `changeMainPassword()` in `change.go` calls `storage.Get(mainPassword)` directly (not the shared loader). For `change main`, the *post-change* `GitCommit` triggers `GitPush` which is correct. We don't pre-pull on `change main` — it'd be silly to pull a remote we're about to overwrite under a new password. **However**, we do need to handle the case where there are unpulled remote commits at the time of `change main`: the user is about to push a re-encryption that will be unmergeable from any device with the old password. This is intentional; the plan accepts the cross-merge-bail UX. Add a one-line note in `change main` flow: "(reminder: ensure other devices are synced before changing main password)" — optional.

  Actually, simpler: leave `change main` as-is. If user runs `change main` while remote has unpulled commits, the post-change `git push` will fail (non-fast-forward). Yellow warning. The user investigates; either pulls (which would now fail with ErrForkUndecryptable since remote is under old password) or force-pushes manually. This is the documented edge case.

### Step 10 — Wire `GitCommit` to push

`internal/storage/git.go`:

```go
func GitCommit(message string) error {
    if os.Getenv("PSW_GIT") == "0" {
        return nil
    }
    if !Paths.gitRepoExists {
        return nil
    }

    cmd := exec.Command("git", "add", "storage.psw", "pswcfg.toml")
    cmd.Dir = Paths.storagePath
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("git add: %w", err)
    }

    cmd = exec.Command("git", "commit", "--message="+message)
    cmd.Dir = Paths.storagePath
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("git commit: %w", err)
    }

    GitPush()
    return nil
}
```

Single line addition. `GitPush` no-ops when no remote configured.

### Step 11 — Tests

#### Unit tests: `internal/storage/merge_test.go`

Table-driven over the 8 cases plus tie-break. Must be in `package storage` (white-box) to access unexported types.

Cases:
| Name | F | L | R | Mtimes | Expected |
|---|---|---|---|---|---|
| empty everywhere | — | — | — | — | empty result |
| local-added | — | alice@5 | — | — | {alice} |
| remote-added | — | — | alice@5 | — | {alice}, action=AddedFromRemote |
| both-added-same-name | — | alice@5 (pass=L) | alice@7 (pass=R) | — | {alice@7 with pass=R}, action=ReplacedFromRemote |
| both-added-tie | — | alice@5 (pass=L) | alice@5 (pass=R) | mtime tie | {alice with pass=R}, action=ReplacedFromRemote (remote-wins-tie) |
| remote-removed-no-local-change | alice@5 | alice@5 | — | L.mtime==F.mtime | empty, action=DroppedByRemote |
| remote-removed-local-modified | alice@5 | alice@9 | — | L.mtime>F.mtime | {alice@9}, action=KeptLocalOverRemoval |
| local-removed-no-remote-change | alice@5 | — | alice@5 | R.mtime==F.mtime | empty (no action — local intent honored) |
| local-removed-remote-modified | alice@5 | — | alice@9 | R.mtime>F.mtime | {alice@9}, action=AddedFromRemote |
| both-have-same-content | alice@5 | alice@5 | alice@5 | byte-equal | {alice@5}, action=Unchanged (no warning) |
| both-have-different-content | alice@5 | alice@7 (pass=L7) | alice@9 (pass=R9) | L<R | {alice@9 R9}, action=ReplacedFromRemote |
| both-have-different-content-local-newer | alice@5 | alice@9 (pass=L9) | alice@7 (pass=R7) | L>R | {alice@9 L9}, action=KeptLocalNewer |
| both-removed | alice@5 | — | — | — | empty, no action |
| case-insensitive-name | Alice@5 | alice@7 | aLiCe@9 | L<R | {aLiCe@9} (preserves remote's casing because remote wins) |

Each row → one `t.Run("name", func(t *testing.T) { ... })`. Approximately 14 rows.

Helper:
```go
func rec(name, user, pass string, mtime int64) Record {
    return Record{Name: name, Username: user, Password: pass, MTime: mtime}
}
```

#### Integration tests: `tests/sync_test.go`

Helper `tests/git_helpers_test.go`:

```go
func newGitVaultWithRemote(t *testing.T) (vault, bare string, env map[string]string) {
    t.Helper()
    if _, err := exec.LookPath("git"); err != nil {
        t.Skip("git not available")
    }

    bare = t.TempDir()
    if out, err := exec.Command("git", "init", "--bare", "--initial-branch=main", bare).CombinedOutput(); err != nil {
        t.Fatalf("git init --bare: %v\n%s", err, out)
    }

    vault = t.TempDir()
    cfg := fmt.Sprintf("clipboard_timeout = 20\nremote = %q\n", bare)
    if err := os.WriteFile(filepath.Join(vault, "pswcfg.toml"), []byte(cfg), 0644); err != nil {
        t.Fatalf("write pswcfg: %v", err)
    }

    env = map[string]string{
        "PSW_GIT":             "",
        "GIT_AUTHOR_NAME":     "psw-tests",
        "GIT_AUTHOR_EMAIL":    "psw@tests.local",
        "GIT_COMMITTER_NAME":  "psw-tests",
        "GIT_COMMITTER_EMAIL": "psw@tests.local",
    }
    runPswEnv(t, vault, env)  // first-run: prompt-bypass, create vault, init local, push initial
    return vault, bare, env
}

func cloneFromBare(t *testing.T, bare string) (vault string, env map[string]string) {
    t.Helper()
    vault = t.TempDir()
    if out, err := exec.Command("git", "clone", bare, vault).CombinedOutput(); err != nil {
        t.Fatalf("git clone: %v\n%s", err, out)
    }
    cfg := fmt.Sprintf("clipboard_timeout = 20\nremote = %q\n", bare)
    if err := os.WriteFile(filepath.Join(vault, "pswcfg.toml"), []byte(cfg), 0644); err != nil {
        t.Fatalf("write pswcfg: %v", err)
    }
    env = map[string]string{
        "PSW_GIT":             "",
        "GIT_AUTHOR_NAME":     "psw-tests",
        "GIT_AUTHOR_EMAIL":    "psw@tests.local",
        "GIT_COMMITTER_NAME":  "psw-tests",
        "GIT_COMMITTER_EMAIL": "psw@tests.local",
    }
    return vault, env
}
```

Eight integration tests:

1. **TestSync_AutoPushOnAdd**
   `newGitVaultWithRemote` → `add foo`. Assert: `git log --oneline` in bare contains commit "added new record" reachable from `main`.

2. **TestSync_AutoPullBeforeMutate** (disjoint adds across two devices)
   Device A and B clone same bare. A `add alpha`. B `add beta`. After B's add: B's `psw` listing must include alpha (pulled in). After A's next mutation pulls B's beta: A's listing contains both.

3. **TestSync_SmartMerge_DisjointAdds_NoConflict**
   Variant of #2 where both devices are simultaneously offline (PSW_GIT_REMOTE=0 toggled), then come back online. Verify merged result has both records on both sides after one round-trip per side.

4. **TestSync_SmartMerge_SameRecord_NewestWins**
   Both A and B `add shared` while offline (`PSW_GIT_REMOTE=0`). B sleeps then runs second so its mtime is later. A re-enables remote and pushes (`add otherA` to trigger push of pending commits). B re-enables remote and runs `add otherB` — pre-pull merges. Verify B's `shared` survived; A on next pull also gets B's `shared`.

5. **TestSync_SmartMerge_RemovedOnOneSide_NoModification**
   A and B sync alpha. A `remove alpha`. B's next mutation pulls and merges → alpha gone. Verify both ends.

6. **TestSync_SmartMerge_RemovedOnOneSide_ConcurrentModification**
   A and B sync alpha. A `remove alpha` while offline. B `change alpha --password=newp` while offline. A pushes (case 4 from algorithm: F1 L0 R-this-side-after-merge). On A's next pull-and-mutate from a fresh add, it sees B's modified alpha and keeps it (`L.mtime > F.mtime`). Verify alpha exists with new password.

7. **TestSync_ChangeMain_CrossMergeBails**
   A and B sync. A `change main` → newpass; pushes. B (with old `PSW_MAIN_PASSWORD=testpass`) attempts `add foo`. Expect exit 1, red message containing "Storage was re-encrypted".

8. **TestSync_NoRemoteConfigured**
   newVault (no remote in pswcfg). Run `add`, `get`, `change`, `remove`. Verify: no warnings about remote on stderr; no `.git/refs/remotes` ever created.

9. **TestSync_PSWGitRemoteZero_DisablesNetwork**
   newGitVaultWithRemote, but set `PSW_GIT_REMOTE=0` in env for all subsequent runs. `add foo` should commit locally but bare repo's HEAD must not advance. Verify: `git -C bare log --oneline | wc -l` unchanged after the add.

10. **TestSync_PushRejected_WarnsContinues**
    A and B both clone bare. A `add foo` → push succeeds. B (still on initial sync state) attempts `add bar` with `PSW_GIT_REMOTE=0` (commits locally only). B then re-enables remote and tries another `add baz`. Pre-pull merges A's foo. Push of B's history (bar+baz+merge+baz) should succeed. **Reformulation:** harder to engineer push-rejection deterministically in a test; consider skipping. If included, the test asserts that even on push failure (simulated by a chmod-readonly bare or a non-existent ssh remote), the local commit survives.

   Decision: **drop test #10** from Phase 1. Push-rejection happens in real-world races (rare); we trust the warn-and-continue path by inspection. Add only if a regression motivates it.

#### Tweak: `tests/log_test.go`

Add `PSW_GIT_REMOTE: "0"` to the env map in `newGitVault`.

### Step 12 — Documentation pass

- `CLAUDE.md`: add a paragraph under "Architecture > Data dir" describing remote sync, the two opt-out env vars, and the cross-merge-bail behavior. Keep terse.
- `pswcfg-template.toml`: comment line as specified above.
- No new top-level docs.

## Smart-merge algorithm reference

| F | L | R | Decision | Reasoning |
|---|---|---|---|---|
| 0 | 1 | 0 | keep L | local-added since fork |
| 0 | 0 | 1 | take R | remote-added since fork |
| 0 | 1 | 1 | mtime LWW; tie → R | independent additions |
| 1 | 1 | 0 | if L.mtime > F.mtime keep L; else drop | modification beats removal; otherwise honor removal |
| 1 | 0 | 1 | if R.mtime > F.mtime take R; else drop | symmetric |
| 1 | 1 | 1 | mtime LWW; tie → R | content reconcile |
| 1 | 0 | 0 | drop | both removed |
| 0 | 0 | 0 | n/a | not in any index |

Tie-break invariant: when mtimes are exactly equal on a content conflict, **remote wins**. Deterministic regardless of which side is local. Document in code comment.

## Cross-merge-bail (decryptability)

Triggers in two places:
- Fast-forward: `assertBlobDecryptable(remoteRef, mainPassword)` before the ff. If fails → return `ErrForkUndecryptable`.
- Divergent: `decryptBlobToRecords(forkSHA:storage.psw, mainPassword)` and same for remote. Either fails → return `ErrForkUndecryptable`.

User message (red, on stdout for cobra UX consistency, then `errExit`):

> Storage was re-encrypted under a different main password since the last sync. Push from the device that ran `change main` first, then retry on this device.

## Acceptance criteria (Done-when)

- `make test` passes (existing + new). Total test count grows by ~14 unit + ~9 integration = ~23 cases.
- A vault with no `remote =` in `pswcfg.toml` produces zero new stderr output vs. baseline. Diff `git log` against baseline `main` produces identical commit graph for the same actions.
- A vault with `remote =` set against a local bare repo:
  - After every mutation, the bare repo HEAD advances by exactly one commit (or two for divergent merge).
  - `psw get`, `psw`, `psw log` produce **zero** network calls (verifiable by pointing remote at an unreachable URL like `ssh://invalid.example/x` and asserting no warnings).
- `change main` from a stale device: red error, exit 1, no partial state mutation.
- Two devices alternating `add`/`change`/`remove` against shared bare converge to the same record set after one round-trip per side.
- Push failure (e.g. unreachable remote) prints exactly one yellow warning per command, never two; local commit survives; next mutation retries.

## Open risks

- **Merge commit construction with `git merge -s ours --no-commit` then overwriting staged file**: confirm git accepts the modified storage.psw at commit time without complaining about an "unmerged" state. Alternative if this is brittle: use plumbing (`git commit-tree -p local -p remote $(git write-tree)`) and `git update-ref refs/heads/<branch>`. Decide during step 7 implementation.
- **`omitempty` on `MTime int64`**: zero value is omitted from JSON, but legacy records also produce zero on unmarshal. This is intended (zero-mtime loses every comparison). Confirm `json` handles `int64` zero with `omitempty` consistently — Go stdlib does.
- **`pswcfg.toml` is committed**: writing `remote = "..."` into the committed file means the URL travels with the repo. If the user wants per-machine URLs (e.g. private SSH on dev, HTTPS-with-token on CI), they'd have to gitignore `pswcfg.toml` themselves. Out of scope here; flagged in `git-sync.md` open questions.
- **`git fetch` against a fresh bare with no commits**: for the very first push from device A, the bare has no `main` ref. `git fetch origin main` will fail. `gitPullAndMerge` handles this via `rev-parse --verify refs/remotes/origin/<branch>` — if no remote ref exists, return nil (nothing to pull). Verify in TestSync_AutoPushOnAdd.
- **Argon2id timing on every command**: each pull-before-mutate against a non-empty remote runs ~3 Argon2id ops (~300-900ms). Likely acceptable; revisit if user reports drag. Phase 2 spinner masks the pause.
- **Concurrent `psw` invocations on the same vault**: there's no lockfile. Two simultaneous mutations could clobber each other's commits. This pre-dates Phase 1; not a regression. Out of scope.

## Out of scope (deferred to Phase 2/3 or later)

- Spinner UI (Phase 2).
- go-git migration (Phase 3).
- `psw sync` command — explicitly rejected.
- Per-record interactive conflict resolver — explicitly rejected.
- Network timeout — currently relying on user's Ctrl-C; revisit if reports come in.
- Per-machine `pswcfg.local.toml` overlay — possible future work.

## Hand-off

Implement steps 1–12 in order. Commit at step boundaries (1, 2, 3+4, 5+6, 7, 8, 9+10, 11 unit, 11 integration, 12). Each commit must keep `make test` green. After step 12, mark Phase 1's checkbox in `git-sync.md`.
