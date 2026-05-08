# CLAUDE.md

Guidance for Claude Code in this repo.

## Build & install

- `make build` — builds `./bin/{psw,clipclean}` with `psw` ldflags `-X 'github.com/ylniss/psw/internal/cli.Version=$(VERSION)'` (top-level `VERSION` file = source of truth).

Nix flake (`nix/flake.nix`): update `gomod2nix.toml` + `vendorHash` on dep changes.

Integration tests under `tests/` (`make test`): `TestMain` builds `psw` once into `t.TempDir`; each test shells out against its own `PSW_HOME=t.TempDir()` vault with `PSW_GIT=0`.

## Two binaries, one repo

- `psw` (`cmd/psw/main.go` → `cli.Execute`) — the CLI. No `.env` autoload at startup.
- `clipclean` (`cmd/clipclean/main.go`) — backgrounded by `psw get` to clear clipboard after timeout. Must be on `PATH` (covered by `make install`).

## Architecture

### Package layout

- `cmd/<binary>/main.go` — entry points. Thin wrappers; real logic under `internal/`.
- `internal/cli/` — Cobra commands (`package cli`); each file self-registers with `rootCmd` via `init()`. `rootCmd` (`root.go`) lists records when bare; its `PersistentPreRunE` runs `setupLogger()` + `storage.InitConfig()` (errors propagate through cobra → `Execute` → exit 1). `Version` = ldflag target (see Build & install).
- `internal/storage/` — storage + encryption. `InitConfig` populates package-level singletons `Paths` (`StorageConfig`-typed; paths + git-repo flag) and `AppConfig` (parsed TOML).
- `internal/prompt/` — TUI prompts. `YesOrNo` returns `false` on non-TTY stdin (no panic) — scripting-safe.
- `plans/` — design notes for in-flight or completed reshapes.

### Data dir (`~/.psw/`)

`storage.InitConfig` (from `rootCmd.PersistentPreRunE`) ensures `~/.psw/` and loads `pswcfg.toml` (seeded from beside the executable on first run — hence `make build` copies `pswcfg-template.toml` → `bin/`). Two storage entry points: `storage.GetOrCreateForRead` (no network — used by `psw`, `psw get`, `psw log`) and `storage.GetOrCreateForMutate` (pulls from remote first, then merges — used by `psw add`, `psw change`, `psw remove`, `psw change main`). Both prompt for main password and init the repo with `main` as the default branch via go-git (`PlainInitWithOptions`); `Paths.gitRepoExists` gates per-mutation `GitCommit`. `GitCommit` stages surgically (`storage.psw` + `pswcfg.toml`, not the whole tree) — keeps stray dotfiles and backups (e.g. `storage.psw.legacy-bak` from Phase 1's one-time upgrade) out of history. After commit, `GitCommit` calls `GitPush` which is best-effort (warn-yellow on failure, never propagates).

### Remote sync (optional)

`pswcfg.toml`'s `remote = "..."` opts in to git sync. Absent → all sync is no-op. When set, every mutation runs pull → smart merge → mutate → commit → push; reads never touch the network. The smart merge (`internal/storage/merge.go`) uses per-record `Record.MTime` (UTC ms, stamped centrally in `Storage.AddRecord`/`UpdateRecord`) for last-write-wins; remote wins on exact tie. `change main` is special: re-encryption commits don't bump any record's mtime, so password rotation doesn't accidentally win every conflict. If the merge would need to decrypt fork or remote storage.psw with a password it doesn't have (cross-merge after `change main` from another device), it returns `storage.ErrForkUndecryptable` and the CLI prints a red error suggesting the user push from the device that ran `change main` first. Two opt-out env vars: `PSW_GIT=0` (no git at all) and `PSW_GIT_REMOTE=0` (local commits OK, no network) — tests use both.

### Git backend (go-git with shell-out fallback)

The git layer is pure-Go via `go-git/v5`. **For normal HTTPS/SSH remotes the `git` binary is not a runtime dependency.** Local ops (`internal/storage/git_local.go`) — init, add, commit, log, merge-base, show-blob, rev-parse, is-ancestor, fast-forward, two-parent merge commit — never shell out. Network ops (`GitFetch`/`GitPush` in `internal/storage/git_sync.go`) try go-git first; on `ErrAuthRequiresHelper` (credential helper, no usable SSH key) or `ErrSigningRequired` (commit.gpgsign=true), they fall back to `runGit*` shell-out helpers if `git` is on `PATH`. Auth resolver (`internal/storage/git_auth.go`): SSH tries ssh-agent then `~/.ssh/id_ed25519` / `id_rsa`; HTTPS uses BasicAuth from URL userinfo if present; otherwise checks `git config credential.helper`. Host-key verification is permissive (`InsecureIgnoreHostKey`) — same posture as desktop git's `accept-new`. **Edge case**: go-git's `file://` (and bare-path) transport delegates to `git-upload-pack`/`git-receive-pack` internally, so a vault with a local-bare-path remote still needs `git` on `PATH` — that's a developer-machine pattern (and what the test suite uses), not normal multi-device sync. When signing is required and `git` isn't on `PATH`, `GitCommit` warns yellow ("record saved but not committed") and continues — same posture as push failures.

### Encryption (`internal/storage/encryption.go`)

AES-256-GCM via `cipher.NewGCMWithRandomNonce` (Go 1.24+; nonce generated and prepended internally per seal). Key = Argon2id over main password + 16-byte per-vault salt (m=64 MiB, t=2, p=4, keylen=32; OWASP "balanced for desktop"); fresh salt per write. On-disk: `base64("PSW1" || salt[16] || gcm_seal_output)`, mode 0600. Decrypt validates the `"PSW1"` magic and rejects anything else; failure → `"Wrong password."`. Format changes must bump the magic and keep `EncryptStringToStorage`/`DecryptStringFromStorage` aligned — existing storage becomes unreadable, no migration code in shipped tree.

### Record model

`storage.Record` has `Username`/`Password` (JSON tags `user`/`pass` for on-disk compatibility), `Value`, and `MTime` (`json:"mtime,omitempty"`, UTC ms). Each record uses **either** user+pass **or** single value — never both. Discriminator: `Value == ""`.

- `psw add` defaults to user+pass; `--single`/`-s` → single value.
- `psw get`, `change`, root listing all branch on `record.Value == ""` for which fields to show/edit.

`"main"` is reserved: `psw add main` rejected; `psw change main` re-encrypts entire storage under a new main password (does not modify any record).

### Interactive record selection

`get`/`change`/`remove` resolve via `storage.GetRecordNameInteractive` (`internal/storage/picker.go`) — in-process `bubbles/list` fuzzy picker, no `PATH` deps. When there's only one matching record, it's returned without launching the TUI — intentional, prevents confirming a forced choice; keep before changing selection logic. On Esc/Ctrl-C the picker returns `ErrPickerCancelled`, and `helpers.go` translates that to a silent exit.

## Conventions

- Output colorized via `github.com/TwiN/go-color`: record names green, hints/commands cyan, warnings yellow, errors red.
- Errors via cobra `RunE`: print user-facing message, then `return errExit` (empty-message sentinel in `internal/cli/root.go`) → exit 1 without cobra usage dump. `SilenceErrors`/`SilenceUsage` on `rootCmd` keep prior UX. Flag-validation (e.g. `add`'s mutual-exclusion) and `resolveRecordName` `--exact` paths return `errExit`; callers thread it via `if errors.Is(err, errExit) { return errExit }`. Only `os.Exit(1)` outside `main`/tests is `cli.Execute`'s cobra-error fallback. Match surrounding style.
- `slog.Debug` gated by `--verbose`/`-v` is the only place secret-adjacent data may log; never `fmt.Println` raw passwords.
- Add subcommand: create `internal/cli/<name>.go` with a `*cobra.Command` and `rootCmd.AddCommand(...)` in `init()`.

## Testing / scripting mode

CLI can run unattended (no TUI prompts) via the env vars and flags below.

### Env vars

- `PSW_HOME=<path>` — override storage dir (default `~/.psw`). Tests get a fresh `t.TempDir()` per case.
- `PSW_MAIN_PASSWORD=<str>` — supplies main password; bypasses prompt + double-confirm on vault creation. Empty value treated as unset (falls through to prompt).
- `PSW_NEW_MAIN_PASSWORD=<str>` — new main password for `change main`. Same handling.
- `PSW_GIT=0` — skip auto `git init` + per-mutation `git commit` in the storage dir. Default behavior unchanged when unset.
- `PSW_GIT_REMOTE=0` — local git commits OK, but no fetch/pull/push. Useful for offline mutations + sync tests that simulate diverging devices.

Caveat: env-var passwords visible in `/proc/<pid>/environ`. Fine for tests/ephemeral scripts; not for daily use. No `--password` CLI flag (would expose via `ps`).

### Flags

Per-command flags: `psw <cmd> --help`. Notable quirk: when **any** of `change`'s `--rename/--username/--password/--value` is set, all unset-field y/n prompts are also skipped (those fields stay unchanged). Lets `change foo --password=new --exact` run unattended.

### Exit codes

Most error paths print and `return` (exit 0). Scripting paths exit 1 explicitly:
- `--exact` with missing arg or unknown name
- `add` flag mutual-exclusion violations
- `change` with field flag that doesn't match the record type
- `change main` with record-level flags
