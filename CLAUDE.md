# CLAUDE.md

Guidance for Claude Code in this repo.

## Build & install

- `make build` — builds `./bin/{psw,clipclean}`; `psw` ldflag `-X 'github.com/ylniss/psw/internal/cli.Version=$(VERSION)'` (top-level `VERSION` file = source of truth).

Nix flake (`nix/flake.nix`): bump `gomod2nix.toml` + `vendorHash` on dep changes.

Integration tests under `tests/` (`make test`): `TestMain` builds `psw` once into `t.TempDir`; each test shells out against its own `PSW_HOME=t.TempDir()` vault with `PSW_GIT=0`.

## Two binaries, one repo

- `psw` (`cmd/psw/main.go` → `cli.Execute`) — CLI. No `.env` autoload.
- `clipclean` (`cmd/clipclean/main.go`) — backgrounded by `psw get` to clear clipboard after timeout. Must be on `PATH` (covered by `make install`).

## Architecture

### Package layout

- `cmd/<binary>/main.go` — entry points; thin wrappers, real logic under `internal/`.
- `internal/cli/` — Cobra commands (`package cli`); each file self-registers with `rootCmd` via `init()`. `rootCmd` (`root.go`) lists records when bare; `PersistentPreRunE` runs `setupLogger()` + `storage.InitConfig()` (errors → cobra → `Execute` → exit 1). `Version` = ldflag target.
- `internal/storage/` — storage + encryption. `InitConfig` populates package singletons `Paths` (`StorageConfig`; paths + git-repo flag) and `AppConfig` (parsed TOML).
- `internal/prompt/` — TUI prompts. `YesOrNo` returns `false` on non-TTY stdin (no panic) — scripting-safe.
- `plans/` — design notes for in-flight or completed reshapes.

### Data dir

Resolved via `os.UserConfigDir()` + `psw` (Linux: `$XDG_CONFIG_HOME/psw` or `~/.config/psw`; Windows: `%AppData%\psw`; macOS: `~/Library/Application Support/psw`). `PSW_HOME` overrides for tests/scripting.

`storage.InitConfig` (called from `rootCmd.PersistentPreRunE`) ensures the data dir and loads `pswcfg.toml` (seeded from beside the executable on first run — `make build` copies `pswcfg-template.toml` → `bin/`). Two storage entry points: `GetOrCreateForRead` (no network; `psw`, `psw get`, `psw log`) and `GetOrCreateForMutate` (pull → merge → return; `psw add/change/remove`, `change main`). Both prompt for main password and init the repo with `main` as default branch via go-git (`PlainInitWithOptions`); `Paths.gitRepoExists` gates per-mutation `GitCommit`. `GitCommit` stages `storage.psw` + `pswcfg.toml` only (not the whole tree) — keeps stray dotfiles and backups (e.g. `storage.psw.legacy-bak` from Phase 1's one-time upgrade) out of history. After commit, `GitCommit` calls `GitPush` (best-effort: warn-yellow on failure, never propagates).

### Remote sync (optional)

`pswcfg.toml`'s `remote = "..."` opts in to git sync; absent → no-op. When set, every mutation runs pull → smart merge → mutate → commit → push; reads never touch the network. Smart merge (`internal/storage/merge.go`) uses per-record `Record.MTime` (UTC ms, stamped centrally in `Storage.AddRecord`/`UpdateRecord`) for last-write-wins; remote wins on exact tie. `change main` is special: re-encryption doesn't bump any record's mtime, so password rotation doesn't accidentally win every conflict. If merge needs to decrypt fork/remote `storage.psw` with a password it doesn't have (cross-merge after `change main` elsewhere), it returns `storage.ErrForkUndecryptable` and the CLI prints a red error suggesting the user push from the device that ran `change main` first. Two opt-out env vars: `PSW_GIT=0` (no git) and `PSW_GIT_REMOTE=0` (local commits OK, no network) — tests use both.

### Git backend (go-git with shell-out fallback)

Pure-Go via `go-git/v5`. **For HTTPS/SSH remotes the `git` binary is not a runtime dependency.** Local ops (`internal/storage/git_local.go`) — init, add, commit, log, merge-base, show-blob, rev-parse, is-ancestor, fast-forward, two-parent merge commit — never shell out. Network ops (`GitFetch`/`GitPush` in `internal/storage/git_sync.go`) try go-git first; on `ErrAuthRequiresHelper` (credential helper / no usable SSH key) or `ErrSigningRequired` (`commit.gpgsign=true`), fall back to `runGit*` if `git` is on `PATH`. Auth resolver (`internal/storage/git_auth.go`): SSH → ssh-agent then `~/.ssh/id_ed25519` / `id_rsa`; HTTPS → BasicAuth from URL userinfo if present, else check `git config credential.helper`. Host-key verification permissive (`InsecureIgnoreHostKey`) — same posture as desktop git's `accept-new`. **Edge case**: go-git's `file://` (and bare-path) transport shells out to `git-upload-pack`/`git-receive-pack`, so a local-bare-path remote needs `git` on `PATH` — developer-machine pattern (test suite uses it), not normal multi-device sync. When signing required and `git` not on `PATH`, `GitCommit` warns yellow ("record saved but not committed") and continues — same posture as push failures.

### Encryption (`internal/storage/encryption.go`)

AES-256-GCM via `cipher.NewGCMWithRandomNonce` (Go 1.24+; nonce generated + prepended internally per seal). Key = Argon2id(main password + 16-byte per-vault salt; m=64 MiB, t=2, p=4, keylen=32; OWASP "balanced for desktop"); fresh salt per write. On-disk: `base64("PSW1" || salt[16] || gcm_seal_output)`, mode 0600. Decrypt validates `"PSW1"` magic; failure → `"Wrong password."`. Format changes must bump the magic and keep `EncryptStringToStorage`/`DecryptStringFromStorage` aligned — existing storage becomes unreadable, no migration code in shipped tree.

### Record model

`storage.Record`: `Username`/`Password` (JSON tags `user`/`pass` for on-disk compat), `Value`, `MTime` (`json:"mtime,omitempty"`, UTC ms). Each record uses **either** user+pass **or** single value — never both. Discriminator: `Value == ""`.

- `psw add` defaults to user+pass; `--single`/`-s` → single value.
- `psw get`, `change`, root listing all branch on `record.Value == ""` for which fields to show/edit.

`"main"` is reserved: `psw add main` rejected; `psw change main` re-encrypts entire storage under a new main password (no record mutated).

### Interactive record selection

`get`/`change`/`remove` resolve via `storage.GetRecordNameInteractive` (`internal/storage/picker.go`) — in-process `bubbles/list` fuzzy picker, no `PATH` deps. Single matching record returned without launching the TUI — intentional (prevents confirming a forced choice); keep before changing selection logic. On Esc/Ctrl-C picker returns `ErrPickerCancelled`; `helpers.go` translates to silent exit.

### Menu mode (hotkey terminals)

`psw menu` (`internal/cli/menu.go`) opens a unified launcher with two phases under the PSW ASCII header: (1) action select — horizontal `get/add/change/remove` buttons, default `get`, ←/→ navigates; (2) main password input — Enter on the action transitions in-place (header stays visible). After Enter on password, the TUI exits and the chosen subcommand's `RunE` is invoked. Password is passed through `prompt.SetMainPasswordOverride` (in-process variable, not env var — avoids `/proc/<pid>/environ` leak) so the dispatched action's storage load skips its own password prompt. From there the action's existing flow runs (picker for get/change/remove; name+username+password prompts for add); header is wiped. Esc/Ctrl-C cancels at any phase. Designed for terminal windows spawned on a hotkey (e.g. `foot -e psw menu` under niri/sway); foot exits when its child exits unless `-H`/`--hold` is passed, so closing the window is the launcher's job, not psw's. Non-TTY stdin → error + exit 1; no scripting mode. First-time vault creation through menu skips the password double-confirm (single input only) — `psw add` is the recommended path for fresh setup.

## Conventions

- Colors via `github.com/TwiN/go-color`: record names green, hints/commands cyan, warnings yellow, errors red.
- Errors via cobra `RunE`: print user-facing message, `return errExit` (empty-message sentinel in `internal/cli/root.go`) → exit 1 without cobra usage dump. `SilenceErrors`/`SilenceUsage` on `rootCmd` keep prior UX. Flag-validation (e.g. `add`'s mutual-exclusion) and `resolveRecordName` `--exact` paths return `errExit`; callers thread `if errors.Is(err, errExit) { return errExit }`. Only `os.Exit(1)` outside `main`/tests is `cli.Execute`'s cobra-error fallback. Match surrounding style.
- `slog.Debug` gated by `--verbose`/`-v` is the only place secret-adjacent data may log; never `fmt.Println` raw passwords.
- Add subcommand: create `internal/cli/<name>.go` with a `*cobra.Command` and `rootCmd.AddCommand(...)` in `init()`.

## Testing / scripting mode

CLI runs unattended (no TUI prompts) via env vars + flags below.

### Env vars

- `PSW_HOME=<path>` — override storage dir (default = `os.UserConfigDir()/psw`). Tests get a fresh `t.TempDir()` per case.
- `PSW_MAIN_PASSWORD=<str>` — supplies main password; bypasses prompt + double-confirm on vault creation. Empty = unset (prompt).
- `PSW_NEW_MAIN_PASSWORD=<str>` — new main password for `change main`. Same handling.
- `PSW_GIT=0` — skip auto `git init` + per-mutation `git commit`. Default unchanged when unset.
- `PSW_GIT_REMOTE=0` — local git commits OK; no fetch/pull/push. For offline mutations + sync tests simulating diverging devices.

Caveat: env-var passwords visible in `/proc/<pid>/environ`. Fine for tests/ephemeral scripts; not for daily use. No `--password` CLI flag (would expose via `ps`).

### Flags

Per-command: `psw <cmd> --help`. Quirk: when **any** of `change`'s `--rename/--username/--password/--value` is set, unset-field y/n prompts are also skipped (those fields stay unchanged). Lets `change foo --password=new --exact` run unattended.

### Exit codes

Most error paths print and `return` (exit 0). Scripting paths exit 1 explicitly:
- `--exact` with missing arg or unknown name
- `add` flag mutual-exclusion violations
- `change` with field flag that doesn't match the record type
- `change main` with record-level flags
