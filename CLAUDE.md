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
- `internal/cli/` — Cobra commands (`package cli`). `rootCmd` (`root.go`) is the central registry — `init()` does `rootCmd.AddCommand(getCmd, addCmd, ...)`; each subcommand file just defines its `*cobra.Command` and per-command flags. Bare `psw` lists records; `PersistentPreRunE` runs `setupLogger()` + `storage.InitConfig()` (errors → cobra → `Execute` → exit 1). `Version` = ldflag target.
- `internal/storage/` — storage + encryption. `InitConfig` populates package singletons `Paths` (`StorageConfig`; paths + git-repo flag) and `AppConfig` (parsed TOML). `WarnSink` is a hook so menu mode can route `Warn(...)` into the action transcript instead of stderr.
- `internal/prompt/` — embeddable TUI primitives (`InputModel`, `YesNoModel`, `StarState`) plus standalone wrappers (`PromptForName`, `YesOrNo`, etc.) that wrap the model in `tuiutil.Quitter` for one-shot `tea.NewProgram` runs. `YesOrNo` returns `false` on non-TTY stdin (no panic) — scripting-safe.
- `internal/menu/` — persistent `psw menu` TUI. `MenuModel` orchestrates four `Action` implementations (`get/add/change/remove`) sharing a small `baseAction` (output/transcript/done/cancelled + `stepInput`/`stepYesNo`/`stepPicker` helpers).
- `internal/passgen/` — password generator (per-category minimums + Fisher-Yates shuffle on `crypto/rand`). Configured via `[password_gen]` section of `pswcfg.toml`.
- `internal/tuiutil/` — generic `Quitter[M]` and `UpdateInPlace[M]` shared across embeddable models.
- `internal/clipclean/` — spawns the `clipclean` helper, resolving the binary next to `psw` first (handles minimal-PATH launchers like `niri spawn-sh`).
- `internal/ui/` — `WithSpinner` + `SpinnerModel`. `SpinnersQuiet` flips synchronous mode for hosts that own the screen (menu mode).

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

`psw menu` (`internal/cli/menu.go` → `internal/menu/`) is a single persistent `tea.Program` that hosts every action as an embedded sub-model. Phases under the PSW ASCII header: (1) password entry — animated stars (`prompt.InputModel` with `animateStars=true`) plus a per-keypress non-cyan logo flash for 250ms; (2) action select — horizontal `get/add/change/remove` buttons, default `get`, ←/→ or h/l navigate, enter/j runs, q/esc/ctrl+c quits psw; (3) running an action — the chosen `Action` (in `internal/menu/{get,add,change,remove}.go`) drives a state machine over `prompt.InputModel` / `prompt.YesNoModel` / `storage.PickerModel` / `ui.SpinnerModel` and emits output lines that get appended to the menu's history (capped at 20 blocks, rendered above the buttons). After completion the user returns to action-select. Esc inside an action returns to action-select with no output appended; only Esc/q at action-select (or password phase) exits psw.

Each action embeds `baseAction` (output/transcript/done/cancelled + accessors) and uses `stepInput`/`stepYesNo`/`stepPicker` helpers to compress the per-phase boilerplate. The picker emits its own help line for standalone CLI usage; menu hosts call `PickerModel.WithoutHelp()` and surface the help via `Action.FooterHelp()` at the bottom row instead.

Async ops (Argon2id key derivation in `storage.LoadOrCreate`, save+commit+push in `storage.GitCommit`) run via `tea.Cmd`s in `internal/menu/cmds.go`; the action stays in a spinner phase until the result message arrives. Add and Change start with a y/n branch (single-value record? change main password?) since menu mode has no flags. `change main` rotates the cached main password in `MenuModel.password` (via `Action.NewPassword()`) so subsequent mutations decrypt correctly. `clipclean` is spawned as before; the persistent psw process stays alive and the child runs out its timer normally. Scrollback re-emit on the password phase stays plain `*`. Designed for terminal windows spawned on a hotkey (e.g. `foot -e psw menu` under niri/sway); foot exits when its child exits unless `-H`/`--hold` is passed, so closing the window is the launcher's job, not psw's. Non-TTY stdin → error + exit 1; no scripting mode. First-time vault creation through menu uses a single password input (no double-confirm) and silently encrypts an empty vault under that password; `psw add` is the recommended path for fresh setup. Storage `Warn(...)` calls during a menu run flow into the menu's `warnCollector` (set as `storage.WarnSink` for the duration of `Run()`) so they appear in the in-action transcript instead of stderr.

Standalone CLI calls (`psw get foo`, `psw add bar`, etc.) are independent — they live in `internal/cli/` and call the same primitives via the standalone wrappers (`runInput`, `YesOrNo`, `GetRecordNameInteractive`, `WithSpinner`). Each of those wraps the embeddable model in `tuiutil.Quitter[M]` (in `internal/tuiutil/`), which translates the model's `Done()`/`Cancelled()` into `tea.Quit` for `tea.NewProgram.Run()`.

## Conventions

- Colors via `github.com/TwiN/go-color`: record names green, hints/commands cyan, warnings yellow, errors red.
- Errors via cobra `RunE`: print user-facing message, `return errExit` (empty-message sentinel in `internal/cli/root.go`) → exit 1 without cobra usage dump. `SilenceErrors`/`SilenceUsage` on `rootCmd` keep prior UX. Flag-validation (e.g. `add`'s mutual-exclusion) and `resolveRecordName` `--exact` paths return `errExit`; callers thread `if errors.Is(err, errExit) { return errExit }`. Only `os.Exit(1)` outside `main`/tests is `cli.Execute`'s cobra-error fallback. Match surrounding style.
- `slog.Debug` gated by `--verbose`/`-v` is the only place secret-adjacent data may log; never `fmt.Println` raw passwords.
- Add subcommand: create `internal/cli/<name>.go` with a `*cobra.Command`, then list it in `rootCmd.AddCommand(...)` inside `internal/cli/root.go`'s `init()`.

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
