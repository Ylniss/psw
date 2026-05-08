# Git sync with auto pull/push, smart merge, spinner, go-git

_Last updated: 2026-05-08 — commit `54366f3` (Phase 1) + uncommitted Phase 2 work._

_Status: **Phases 1 & 2 delivered** (Phase 1 committed, Phase 2 uncommitted). Phase 3 not started._

## Goal

Optional remote sync for `~/.psw`. When `remote = "..."` is set in `pswcfg.toml`, every mutating command (`add`, `change`, `change main`, `remove`) pulls before mutating and pushes after committing. Conflicts are resolved by a 3-way smart merge (decrypt local, decrypt remote, decrypt `git merge-base`) with newest-wins per record using a new `mtime` field stored inside the encrypted JSON. When `remote` is absent, current local-only behavior is unchanged. Read paths (`psw`, `psw get`, `psw log`) never touch the network. Eventually the shell-out to `git` is replaced with `go-git`.

## Constraints / non-goals

- Read commands never touch the network.
- Push failures never abort a mutation; the local commit always survives. User sees a yellow warning on stderr; next mutation will retry.
- No `psw sync` command. The smart merge handles common cases; `change main` cross-merge bails with a clear error message. Manual escape hatch is raw `git` in `~/.psw`.
- No interactive per-record conflict resolver. Newest-wins is automatic.
- No tombstones. Removed records are simply absent from the JSON. Merge-base disambiguates "removed" vs "never existed."
- No rename tracking metadata. Rename = remove + add, same as today.
- No record-level timestamps shown to the user. `mtime` is merge-only.
- No new "init" subcommand to bootstrap the remote URL. User configures `remote` in `pswcfg.toml`; no SSH/HTTPS setup logic owned by `psw`.
- `pswcfg.toml` is committed alongside `storage.psw` (existing behavior). Per-machine differences in `pswcfg.toml` (e.g. different remote URLs) are not supported in this plan; if it becomes a problem, a follow-up `pswcfg.local.toml` could be added later.

## Key decisions (pre-implementation)

### Conflict resolution: smart 3-way merge, newest-wins per record

Decision: on pull, decrypt local + remote + merge-base; per record, fork-presence determines remove/add structurally; for content conflicts (record present and differing on both sides) the higher `mtime` wins. Print a yellow merge summary on stderr listing replaced records.

Why: matches multi-device user expectations (LWW is what most consumer sync systems do); avoids per-record interactive prompts; predictable; cheap (3 decrypts total).

Rejected: (a) refusing to merge and requiring `psw sync --keep-local`/`--keep-remote` — too hostile in a multi-device setup which is the user's actual use case; (b) tombstones for removed records — pile up forever without a robust pruning rule; (c) walking divergent commits to derive per-record timestamps from commit history — slow (Argon2id × N commits) and `change main` falsely marks every record as "newest."

### Per-record `mtime` field stored inside encrypted JSON

Decision: add `MTime int64` (UTC milliseconds) to `storage.Record` with JSON tag `"mtime"`. Stamp centrally inside `Storage.AddRecord` and `Storage.UpdateRecord` (not at every caller). Old records without the field deserialize as zero — any real edit beats epoch zero on merge. No on-disk format change beyond the new JSON field; no `"PSW1"` magic bump.

Why: precise per-record signals (a `change main` re-encryption does not bump any record's `mtime`, so password rotations don't accidentally win every conflict); 3 decrypts per merge instead of N; minimal code surface.

Rejected: deriving timestamps from commit walk (see above); device-id tuple `(timestamp, device_uuid)` for tiebreaking — overkill, ms precision plus a deterministic fallback (remote-wins on exact tie) is enough.

### Remote URL source: `pswcfg.toml` only, optional

Decision: a top-level `remote = "..."` key in `pswcfg.toml`. Absent → all sync is no-op. Template ships the line commented with a one-line self-documenting comment.

Why: single source of truth; presence of the key is itself the toggle (no separate `auto_pull`/`auto_push` knobs needed); user can fully disable by commenting out one line.

Rejected: dual-source (TOML + native git config) — extra complexity for marginal flexibility; subcommand `psw remote set` — owns SSH/HTTPS setup, scope creep.

### Pull-before-mutate, push-after-commit

Decision: each of `add`, `change`, `change main`, `remove` runs pull → decrypt → mutate → encrypt → commit → push.

Why: minimizes the conflict window; pull-fail still lets the mutation proceed against local; push-fail leaves the commit local for the next pre-pull to retry.

Rejected: pull on startup / push on exit — startup latency on read commands is a non-starter; relying on exit-time push loses on `kill -9` / power loss; per-mutation push is the only honest default for a security tool.

### Failure modes: warn-and-continue on network errors

Decision: pull failure (network unreachable, fetch error) → yellow warning on stderr, mutation proceeds. Push failure → yellow warning on stderr, commit stays local. Hard error only when fork is undecryptable (main password changed across the merge boundary) — that one prints a red message naming the cause and exits 1.

Why: laptops are offline routinely; blocking a write on flaky wifi is hostile; existing posture (`git not installed` warning) matches.

Rejected: hard-fail on first push error — turns every offline day into a stuck CLI; queueing for retry — no daemon, single-shot CLI doesn't have a queue.

### Branch detection at runtime

Decision: auto-detect branch via `git symbolic-ref --short HEAD`. No `branch` key in `pswcfg.toml`. Pull/push uses `HEAD:<detected>`.

Why: no config to set or get wrong; matches one-user-many-devices reality.

Rejected: `branch` key in TOML — extra config surface for no real flexibility.

### Two phases of opt-out: `PSW_GIT=0` and `PSW_GIT_REMOTE=0`

Decision: keep `PSW_GIT=0` semantics unchanged (no init, no commit, no pull, no push). Add `PSW_GIT_REMOTE=0` for "local commits OK, no network." Existing tests using `PSW_GIT=0` continue working unchanged. The `log_test.go` case (which uses `PSW_GIT=""` to allow init/commit) gets `PSW_GIT_REMOTE=0` added so an unconfigured remote doesn't print warnings into its captured output.

Why: tests need init+commit (to verify `psw log`) without the remote; same control mechanism shape as elsewhere.

Rejected: a single `PSW_GIT=remote-only` mode — string env values are awkward; two booleans are clearer.

### Phasing: sync first, spinner second, go-git last

Decision: ship sync (Phase 1) shelling out to `git`. Add the spinner (Phase 2) once sync semantics are stable. Migrate to go-git (Phase 3) last.

Why: each phase is independently mergeable. Animation has nothing to render until pull/push exist. go-git introduces auth complexity (SSH agent, credential helpers, GPG signing) that should not be debugged simultaneously with sync semantics.

Rejected: bundling everything in one PR — 1500+ line diff with three independent risk surfaces; doing go-git first — auth complexity blocks shipping value.

## Repo context a fresh Claude needs

### File map (key files for this plan)

- `internal/storage/git.go` — current git integration, ~115 lines. Shell-out to `git` for `init`, `add`, `commit`, `log`. **All net-new pull/push/merge code lands here in Phase 1.** May split into `git_sync.go` if it crosses ~250 lines.
- `internal/storage/storage.go` — `Storage`, `Record`, `Save`, `GetOrCreateIfNotExists`. **`Record` gains `MTime`; `AddRecord`/`UpdateRecord` stamp it centrally.** `GetOrCreateIfNotExists` likely splits into `GetOrCreateForRead` / `GetOrCreateForMutate` so pull only runs on the mutate path.
- `internal/storage/config.go` — `StorageConfig`, `Config` (TOML), `InitConfig`. **Adds `Remote string` to `Config` parsed from `pswcfg.toml`.**
- `internal/storage/encryption.go` — AES-256-GCM with Argon2id KDF. Format `base64("PSW1" || salt[16] || gcm_seal_output)`. **Untouched by this plan.** Adding `mtime` to JSON does not change the encryption format.
- `internal/cli/{add,change,remove}.go` — call sites for mutations. Each currently calls `storage.GetOrCreateIfNotExists()` then `storage.GitCommit(...)` after `Save()`. **Switch to mutate-flavored loader; new pre-pull and post-push happen inside the storage layer, not in cobra commands.**
- `internal/cli/{root,get,log}.go` — read paths. **Switch to read-flavored loader; no network.**
- `pswcfg-template.toml` — seeded into `bin/` by `make build`, copied to `~/.psw/pswcfg.toml` on first run. **Adds commented `# remote = "..."` line.**
- `tests/helpers_test.go` — sets `PSW_GIT=0` for all cases. **Stays as-is; new `tests/git_helpers_test.go` adds `newGitVaultWithRemote` for sync-specific cases.**
- `tests/log_test.go` — currently the only test that uses `PSW_GIT=""`. **Add `PSW_GIT_REMOTE=0` to its env to silence "no remote" warnings.**

### Quirks to remember

- **Encrypted blob is a single base64 file.** `git` cannot text-merge it. Any merge logic must operate on decrypted record arrays, never on the byte-level blob.
- **Fresh salt and nonce per `Save()`** (`encryption.go` uses `cipher.NewGCMWithRandomNonce`). Same record content yields different ciphertexts each save. Do not use ciphertext-equality for change detection — use record-level structural diff.
- **Argon2id is slow.** ~100-300ms per decrypt. Budget 3 decrypts per pull = ~1s. Walking N divergent commits is N × that — that's why we use per-record `mtime` instead.
- **`Paths.gitRepoExists`** is set by `initGitRepoIfNotExists` (called from `GetOrCreateIfNotExists`). Pull/push must check this flag and return cleanly when false (e.g. user has no `git` on PATH or `PSW_GIT=0`).
- **`GitCommit` does surgical `git add storage.psw pswcfg.toml`**, not `git add .`. Preserve this — keeps stray dotfiles and the `storage.psw.legacy-bak` file out of history.
- **`prompt.YesOrNo` returns `false` on non-TTY stdin** without panicking. Same posture should apply to spinner: no-TTY → no UI, just run the op. Tests are not TTYs.
- **Errors via `errExit`** sentinel (in `internal/cli/root.go`) — print user-facing message, then `return errExit` so cobra exits 1 without dumping usage. Pull/push failures that should exit 1 (only fork-undecryptable case) follow this pattern. Warn-and-continue paths just print and return nil.
- **Colorize convention** (`github.com/TwiN/go-color`): green = success/record names, cyan = commands/hints, yellow = warnings, red = errors. Merge summary uses yellow ("we replaced something locally").
- **`slog.Debug` gated by `--verbose`** is the only place secret-adjacent data may log. Git URLs should be redacted (token in URL → strip).
- **`pswcfg.toml` is currently committed** along with `storage.psw`. If a user wants per-machine `remote` settings they'd need follow-up work (out of scope here).
- **Case-insensitive record lookup** (`strings.EqualFold` in `Storage.GetRecord` etc.). Merge logic should match records by case-folded name to be consistent.
- **`"main"` is reserved.** `psw add main` rejected; `psw change main` re-encrypts entire storage. The `change main` commit touches `storage.psw` but no record mtimes. The merge logic must not interpret a `change main` commit as "every record was changed."

### How tests run

`make test` builds `psw` once into `t.TempDir()` (in `tests/main_test.go` `TestMain`). Each test gets its own `PSW_HOME=t.TempDir()` and runs the binary as a subprocess with a controlled env (allow-list, not `os.Environ()`). Network-sync tests use a local **bare repo** in another temp dir as the remote — no real network, deterministic.

### Prior incidents / things not to redo

- The codebase recently dropped `cqroot` for `bubbletea`/`bubbles`. If touching prompts, follow the picker pattern in `internal/storage/picker.go` (custom delegate, manual ctrl+n/p arrows, `tea.WithAltScreen()`).
- `MEMORY.md` notes: prefer early returns; flatten branching with guard clauses; happy path stays leftmost. Apply throughout the new code.

## Phases

### [x] Phase 1: Optional remote sync with smart merge

- **Status:** Delivered. 48 integration tests + 16 merge unit subtests green. See `plans/git-sync-phase1.md` for the full detail. Notable deviations from the original plan:
  - `change main` routes through `GetOrCreateForMutate` too (pulls + merges before re-encrypting). Original plan had `change main` use `storage.Get` directly; that path bypassed `initGitRepoIfNotExists` so `Paths.gitRepoExists` stayed false and `GitCommit` silently no-op'd. Side-effect: dropped the ensure-double-confirm of the *current* main password (only the *new* password requires confirmation now).
  - `git init` uses `--initial-branch=main` (was `git init` plain). Aligns new vaults with the modern default and matches the bare-repo init in tests; existing vaults are unaffected.
  - 30s `gitNetworkTimeout` (`exec.CommandContext`) wraps fetch and push only — closes the "network timeout" open question. Local ops are unbounded as before.
  - Three `runGit` helpers (all in `internal/storage/git_sync.go`):
    - `runGit(args...)` — combined stdout+stderr, no timeout.
    - `runGitNetwork(args...)` — combined stdout+stderr, with `gitNetworkTimeout`.
    - `runGitStdout(args...)` — stdout only, stderr surfaced in error message.
  - Phase 2 spinner-wrap targets are `runGitNetwork` calls inside `GitFetch`/`GitPush` — easier than wrapping the public functions.
  - Warning helper renamed `printWarn` (was `warnYellow`); semantic-over-stylistic.
- **Goal:** End-to-end pull-before-mutate / push-after-commit with 3-way smart merge. Shell-out to `git`. No spinner yet.
- **Scope:**
  - `Record.MTime int64` field (`json:"mtime"`); central stamping in `Storage.AddRecord` / `Storage.UpdateRecord`.
  - `Config.Remote string` (`toml:"remote"`); `pswcfg-template.toml` ships commented example with one-line comment.
  - `internal/storage/git.go` grows: `GitPull`, `GitPush`, `gitMergeBase`, plus the smart-merge function operating on three decrypted `[]Record` slices.
  - Smart-merge algorithm: index records by case-folded `Name` across (fork, local, remote); for each name, decide structurally (was-removed vs not) using fork presence; for content conflicts, higher `mtime` wins; remote-wins on exact tie. Build merged `[]Record`, re-encrypt with current main password, write a single merge commit.
  - Merge summary printed on stderr (yellow): `Replaced N records with newer version from remote: alice, bob` etc.
  - Split `GetOrCreateIfNotExists` → `GetOrCreateForRead` (no pull) and `GetOrCreateForMutate` (pulls before decrypt).
  - Update CLI call sites: `add.go`, `change.go`, `remove.go` → mutate variant; `root.go`, `get.go`, `log.go` → read variant.
  - `change main` cross-merge: when fork can't be decrypted with current password, return a sentinel error, print red message ("main password changed since last sync — push from the device that changed it first"), exit 1.
  - Pull failure (network/fetch): yellow warning, mutation proceeds.
  - Push failure (network/rejected): yellow warning, commit stays local; on rejection, suggest retrying after the next pull will reconcile.
  - `PSW_GIT_REMOTE=0` env var skips pull/push; `PSW_GIT=0` continues to skip everything.
  - Branch auto-detect via `git symbolic-ref --short HEAD`.
  - No `psw sync` command. No new subcommands.
  - New `tests/sync_test.go` against a local bare repo: auto-push, auto-pull, smart-merge with disjoint adds, smart-merge with same-record different mtimes (newest wins), removed-on-one-side, `change main` cross-merge bails, no-remote (no network attempt), `PSW_GIT_REMOTE=0` honored.
  - Update `tests/log_test.go` env to include `PSW_GIT_REMOTE=0`.
- **Done when:**
  - `make test` passes (existing tests + new `sync_test.go`).
  - Vault without `remote =` set has identical behavior to current `main` — no warnings, no network calls, no diff vs. baseline `git log`.
  - Vault with `remote = ` set against a local bare remote pushes after every mutation, pulls before, and survives the smart-merge scenarios in tests.
  - `change main` followed by a sync from a stale device produces the red error, not a silent partial decrypt.
- **Risk:** medium. The smart-merge logic is the new code surface that can subtly drop data; tests must cover all six per-record states (in fork only / local only / remote only / fork+local / fork+remote / all three) plus the `change main` cross-merge bail.
- **Depends on:** none.

### [x] Phase 2: Pull/push spinner

- **Status:** Delivered (uncommitted). 1 new ui unit test + 48 integration tests + 16 merge unit subtests green. See `plans/git-sync-phase2.md` for the full detail. Notable deviations from the original plan:
  - **`runGitNetworkSpinner` helper** in `internal/storage/git_sync.go` deduplicates the two wrap sites (originally inline closures of ~6 lines each); `GitFetch`/`GitPush` collapse to one-line calls.
  - **Modern bubbletea pattern.** Original plan used a `tea.Cmd` that blocked on a result channel inside `Init()`. Refactored to `program.Send(doneMsg{})` from a bridge goroutine plus closed-channel signaling (`<-opDone`). Eliminates a `<-resultCh` double-read seam on `tea.Run()` errors and matches v1 idiom for external events. `doneMsg` becomes a flag-only struct; the captured `opErr` carries the error.
  - **Dropped `tea.WithoutSignalHandler()`.** The original plan included it; on review the rationale was thin (tea's default handler doesn't block SIGINT delivery to children, only adds deferred cursor-restore). Without it, Ctrl-C can leave the cursor hidden. Default handler is preferable.
  - **Pre-existing `remove.go` newline bug fixed.** `internal/cli/remove.go:66` used `Printf` without trailing `\n`; spinner exposed it by overwriting the success line. Pre-Phase 2 bug, fixed alongside.
  - **Unit test added** (the plan marked it optional). `internal/ui/spinner_test.go` covers the non-TTY pass-through path — the dominant path during `make test`.
- **Goal:** User-visible feedback while pull or push is in flight. Cosmetic only — no behavioral change.
- **Risk:** low. Spinner is contained, optional (TTY-gated), and the underlying ops are unchanged.
- **Depends on:** Phase 1.

### [ ] Phase 3: go-git migration

- **Goal:** Replace shell-out to the `git` binary with `github.com/go-git/go-git/v5`. Behavioral surface unchanged.
- **Scope:**
  - Add `github.com/go-git/go-git/v5` to `go.mod`. Update `gomod2nix.toml` and `vendorHash` (per `nix/flake.nix` per `CLAUDE.md`).
  - Rewrite the bodies of `initGitRepoIfNotExists`, `GitCommit`, `GitPull`, `GitPush`, `GitLog`, `gitMergeBase` in terms of go-git APIs (`PlainInit`, `Repository.Worktree`, `Worktree.Add`, `Worktree.Commit`, `Repository.Fetch`, `Repository.Push`, `Repository.Log`, `Repository.MergeBase`).
  - Auth helper: `gitAuth(remoteURL string) (transport.AuthMethod, error)`.
    - `git@…` / `ssh://`: try `ssh.NewSSHAgentAuth("git")` first. On agent failure, try `~/.ssh/id_ed25519` then `~/.ssh/id_rsa` via `ssh.NewPublicKeysFromFile`. If none, return `ErrAuthRequiresHelper`.
    - `https://`: if URL contains a token (userinfo), use `BasicAuth`. Otherwise check `git config credential.helper`; non-empty → return `ErrAuthRequiresHelper`.
  - Shell-out fallback dispatcher: each network call tries go-git; on `ErrAuthRequiresHelper` or `ErrSigningRequired`, retries via shell `git` if found on PATH. Otherwise surfaces a typed error.
  - GPG-signing detection: read `commit.gpgsign` from local + global git config. If true, `GitCommit` falls back to shell-out for the commit step (commit signing under go-git would need to drive the user's GPG agent, out of scope).
  - URL redaction in slog: any URL containing userinfo gets the password component stripped before logging.
- **Done when:**
  - `make test` passes against a local bare repo (no auth needed).
  - Manual smoke: vault with `remote = "git@github.com:user/repo.git"` and an ssh-agent loaded key pushes/pulls successfully.
  - Manual smoke: vault with `commit.gpgsign=true` produces signed commits (verified by `git log --show-signature`).
  - `git` binary not on PATH: vault with `remote = "https://token@host/repo.git"` still works for push/pull; vault with `remote = "git@host:repo.git"` and no agent fails with a clear error.
- **Risk:** medium. Auth is the failure surface. Mitigated by the shell-out fallback for unsupported cases; users with vanilla SSH-agent or token-in-URL setups get the pure-go path.
- **Depends on:** Phase 1 (sync semantics must be stable before swapping the backend).

## Decisions log (during implementation)

_Append-only. Format: `YYYY-MM-DD — decision — why`._

- 2026-05-08 — `change main` routes through `GetOrCreateForMutate` (not `storage.Get`) — the direct path bypassed `initGitRepoIfNotExists`, leaving `Paths.gitRepoExists=false` and silencing `GitCommit`. Also gives `change main` pull-before-mutate + cross-merge-bail semantics. Side-effect: dropped the ensure-double-confirm of the current password (kept for the new password).
- 2026-05-08 — `git init --initial-branch=main` for new vaults — aligns with the bare-repo default in tests and modern git conventions. Existing vaults are untouched (we never re-init).
- 2026-05-08 — 30s `gitNetworkTimeout` on fetch/push via `exec.CommandContext` — closes the "indefinite hang" open question. Hardcoded; not configurable yet.
- 2026-05-08 — three `runGit` variants instead of one — local ops, network ops (with timeout), and stdout-only ops have different needs. Names: `runGit` / `runGitNetwork` / `runGitStdout`.
- 2026-05-08 — `--initial-branch=main` deduplication: vault and bare repo both standardize on `main` so `detectBranch` and `git push origin <branch>` align in tests without per-test config.
- 2026-05-08 — `change main` cross-merge bail message centralized as `printForkUndecryptable()` in `internal/cli/helpers.go` — reused by `add`, `change`, `remove`.
- 2026-05-08 — Phase 2 spinner uses `program.Send(doneMsg{})` from a bridge goroutine (modern v1 idiom) instead of a `tea.Cmd` blocking on a channel — eliminates a `<-resultCh` double-read seam if `tea.Run()` errors.
- 2026-05-08 — Phase 2 dropped `tea.WithoutSignalHandler()` from the original plan — bubbletea's default SIGINT handler doesn't block child-process signal delivery; without it Ctrl-C may leave the cursor hidden.
- 2026-05-08 — `runGitNetworkSpinner(label, args...)` helper extracted in `internal/storage/git_sync.go` — collapses the two wrap sites in `GitFetch`/`GitPush` to one-liners and centralizes the spinner-label binding.

## Open questions

- Per-machine `pswcfg.toml` differences: if users want different `remote` URLs per machine, the current "commit `pswcfg.toml`" behavior conflicts. Possible follow-up: `pswcfg.local.toml` (gitignored) overlay. Out of scope here; revisit if it bites.
- ~~Network timeout~~ — addressed Phase 1 with 30s `gitNetworkTimeout`. Revisit if 30s proves too short for high-latency remotes (no flag yet to tune).
- Smart-merge tiebreaker for exactly equal `mtime`: chose remote-wins. If this causes confusion in practice (e.g. ms-precision collisions on fast multi-device automation), reconsider with a (mtime, hash) tuple.
- `change main` UX regression: dropped the ensure-double-confirm of the *current* password as a side-effect of routing through `GetOrCreateForMutate`. The new password still requires confirmation. Revisit if the change feels too lax.

## Hand-off

To detail a phase, start a fresh context and ask:

> Prepare a detailed plan for phase N from `plans/git-sync.md`.

**Before writing phase detail, verify the plan is not stale.** Compare the **Last updated** commit to current `HEAD`; read the files cited in **Repo context** (especially `internal/storage/git.go`, `internal/storage/storage.go`, `internal/storage/config.go`) to confirm they still exist and behave as described. If anything has drifted, surface the drift and update the plan before producing detail.
