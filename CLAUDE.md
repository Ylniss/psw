# CLAUDE.md

Guidance for Claude Code in this repo.

## Build & install

- `make build` → `./bin/{psw,clipclean}`; `psw` ldflag `-X 'github.com/ylniss/psw/internal/cli.Version=$(VERSION)'` (top-level `VERSION` = source of truth).

Nix flake (`nix/flake.nix`): bump `gomod2nix.toml` + `vendorHash` on dep changes.

`make test` runs `go test ./...`: unit tests beside their source, plus the integration tests in `tests/`, where `TestMain` builds `psw` once into `t.TempDir` and each test shells out against its own `PSW_HOME=t.TempDir()` vault with `PSW_GIT=0`.

## Two binaries, one repo

- `psw` (`cmd/psw/main.go` → `cli.Execute`) — CLI. No `.env` autoload.
- `clipclean` (`cmd/clipclean/main.go` → `clipclean.Run`) — backgrounded by `psw get` to clear clipboard after timeout. Must be on `PATH` (`make install` covers it).

## Architecture

### Package layout

- `cmd/<binary>/main.go` — entry points; thin wrappers, real logic under `internal/`.
- `internal/cli/` — Cobra commands (`package cli`). `rootCmd` (`root.go`) is the central registry; `init()` calls `rootCmd.AddCommand(getCmd, addCmd, ...)`; subcommand files define their `*cobra.Command` + per-command flags. Bare `psw` launches the interactive menu (TTY required; non-TTY exits 1 with "requires an interactive terminal"); `PersistentPreRunE` runs `setupLogger()` + `storage.InitConfig()` (errors → cobra → `Execute` → exit 1). `Version` = ldflag target. `psw config` is the pswcfg.toml-only command family: bare prints the config path; `set <key> <value>` writes via the `storage.ConfigKeys` registry; `reset` re-copies the binary-adjacent template. Both mutating subcommands call `GitCommit` directly (no `GetOrCreateFor*`); `tryGitCommit` flips `Paths.gitRepoExists` on demand if `.git/` exists. `set remote` does not auto-pull/push — next normal mutation handles sync.
- `internal/storage/` — storage + encryption. `InitConfig` populates package singletons `Paths` (`StoragePaths`; paths + git-repo flag) and `AppConfig` (parsed TOML). `WarnSink` is a hook so menu mode routes `Warn(...)` into the action transcript instead of stderr.
- `internal/prompt/` — embeddable TUI primitives (`InputModel`, `YesNoModel`, `PickerModel`, `StarState`) plus standalone wrappers (`PromptForName`, `YesOrNo`, etc.) that wrap the model in `tuiutil.Quitter` for one-shot `tea.NewProgram` runs. `YesOrNo` returns `false` on non-TTY stdin (no panic) — scripting-safe.
- `internal/menu/` — persistent menu TUI launched by bare `psw`. `MenuModel` orchestrates six `Action` implementations (`get/add/change/remove/settings/rollback`) sharing a small `baseAction` (output/transcript/done/cancelled + `stepInput`/`stepYesNo`/`stepPicker` helpers). Buttons render in a 4+2 grid (`menuEntries` in `layout.go`).
- `internal/passgen/` — password generator (per-category minimums + Fisher-Yates shuffle on `crypto/rand`). Configured via `[password_gen]` section of `pswcfg.toml`.
- `internal/tuiutil/` — generic `Quitter[M]` and `UpdateInPlace[M]` shared across embeddable models.
- `internal/clipclean/` — spawns the `clipclean` helper, resolving the binary next to `psw` first (handles minimal-PATH launchers like `niri spawn-sh`).
- `internal/ui/` — `WithSpinner` + `SpinnerModel`. `SpinnersQuiet` flips synchronous mode for hosts that own the screen (menu mode).

### Data dir

Resolved via `os.UserConfigDir()` + `psw` (Linux: `$XDG_CONFIG_HOME/psw` or `~/.config/psw`; Windows: `%AppData%\psw`; macOS: `~/Library/Application Support/psw`). `PSW_HOME` overrides for tests/scripting.

`storage.InitConfig` (from `rootCmd.PersistentPreRunE`) ensures the data dir + loads `pswcfg.toml` (seeded from beside the executable on first run — `make build` copies `pswcfg-template.toml` → `bin/`). Two storage entry points: `GetOrCreateForRead` (no network; `psw`, `psw get`, `psw log`) and `GetOrCreateForMutate` (pull → merge → return; `psw add/change/remove`, `change main`, `psw rollback`). Both prompt for main password + init the repo with `main` as default branch via go-git (`PlainInitWithOptions`); `Paths.gitRepoExists` gates per-mutation `GitCommit`. `GitCommit` stages only `storage.psw` + `pswcfg.toml` — keeps stray dotfiles and backups (e.g. `storage.psw.legacy-bak` from Phase 1's one-time upgrade) out of history. After commit, `GitCommit` calls `GitPush` best-effort (warn-yellow on failure, never propagates).

### Remote sync (optional)

`pswcfg.toml`'s `remote = "..."` opts in to git sync; absent → no-op. When set, every mutation runs pull → smart merge → mutate → commit → push; reads never touch the network. Smart merge (`internal/storage/merge.go`) uses per-record `Record.MTime` (UTC ms, stamped centrally in `Storage.AddRecord`/`UpdateRecord`) for last-write-wins; remote wins on exact tie. `change main` is special: re-encryption doesn't bump any record's mtime, so password rotation doesn't accidentally win every conflict. `psw rollback` is its mirror: `ApplyRollback` stamps every restored record's mtime to `now` so the rollback wins LWW on divergent peers; records added between the rollback target and HEAD that a peer modified offline still survive (modification-beats-removal — same semantic as `psw remove` racing a `psw change`). If merge needs to decrypt fork/remote `storage.psw` with a password it doesn't have (cross-merge after `change main` elsewhere), it returns `storage.ErrForkUndecryptable` and the CLI prints a red error suggesting the user push from the device that ran `change main` first. Two opt-out env vars: `PSW_GIT=0` (no git) and `PSW_GIT_REMOTE=0` (local commits OK, no network) — tests use both.

### Git backend (go-git with shell-out fallback)

Pure-Go via `go-git/v5`. **For HTTPS/SSH remotes the `git` binary is not a runtime dependency.** Local ops (`internal/storage/git_local.go`) — init, add, commit, log, merge-base, show-blob, rev-parse, is-ancestor, fast-forward, two-parent merge commit — never shell out. Network ops (`GitFetch`/`GitPush` in `internal/storage/git_sync.go`) try go-git first; on `ErrAuthRequiresHelper` (credential helper / no usable SSH key) or `ErrSigningRequired` (`commit.gpgsign=true`), fall back to `runGit*` if `git` is on `PATH`. Auth resolver (`internal/storage/git_auth.go`): SSH → ssh-agent then `~/.ssh/id_ed25519` / `id_rsa`; HTTPS → `git config credential.helper` (URLs with embedded userinfo are refused at config-set, load, and runtime via `ErrRemoteCredentialsInURL`; no silent shell-git fallback). Host-key verification uses OpenSSH `accept-new` (`internal/storage/git_hostkey.go` via `github.com/skeema/knownhosts`): unknown hosts auto-pin to `~/.ssh/known_hosts`, key changes fail loud via `ErrHostKeyChanged` (also non-fallback). **Edge case**: go-git's `file://` (and bare-path) transport shells out to `git-upload-pack`/`git-receive-pack`, so a local-bare-path remote needs `git` on `PATH` — developer-machine pattern (test suite uses it), not multi-device sync. When signing required and `git` not on `PATH`, `GitCommit` warns yellow ("record saved but not committed") and continues — same posture as push failures.

### Encryption (`internal/storage/encryption.go`)

AES-256-GCM via `cipher.NewGCMWithRandomNonce` (Go 1.24+; nonce generated + prepended internally per seal). Key = Argon2id(main password + 16-byte per-vault salt; m=64 MiB, t=3, p=4, keylen=32; OWASP "balanced for desktop"); fresh salt per write. Plaintext is length-prefixed (4-byte BE uint32 of true length) and zero-padded to a 4 KiB block before sealing — hides record count. On-disk: `base64("PSW2" || salt[16] || gcm_seal_output)`, mode 0600. The magic header bytes are bound as AAD to `gcm.Seal`/`gcm.Open`, so header tampering fails the AEAD tag check. Decrypt validates `"PSW2"` magic; `"PSW1"` returns `ErrPSW1Unsupported` (legacy, one-off upgrade tool); other magic → "unrecognized storage format"; wrong password → `"Wrong password"`. Format changes must bump the magic and keep `EncryptToStorage`/`DecryptFromStorage` aligned — existing storage becomes unreadable, no migration code in shipped tree. Both speak `[]byte`: `EncryptToStorage` wipes its plaintext input (memguard.NewEnclave convention), and the caller of `DecryptFromStorage` is responsible for wiping the returned slice.

### Memory hygiene

Long-lived secrets are protected by [`github.com/awnumar/memguard`](https://github.com/awnumar/memguard): `Storage.MainPassword` is a `*memguard.Enclave` (sealed, mlocked when opened, encrypted-at-rest in heap) — opened to a `LockedBuffer` only at encrypt/decrypt sites, `defer buf.Destroy()`. `cmd/psw/main.go` installs `memguard.CatchInterrupt()` + `defer memguard.Purge()` so SIGINT wipes all live buffers. Record-level secrets (`Record.Password`, `Record.Value`) are `[]byte` (JSON-serialized as base64 inside the encrypted envelope), wiped via `memguard.WipeBytes` on `Storage.RemoveRecord` / overwrite in `Storage.UpdateRecord`; `AddRecord`/`UpdateRecord` clone callers' input so callers may wipe freely. The bulk plaintext JSON between `Storage.MarshalRecords` and `EncryptToStorage` is also `[]byte` and wiped after seal.

Documented unavoidable leaks (caveats):
- `charm.land/bubbles/v2/textinput.Model.Value()` returns `string`; the textinput's internal buffer is unscrubbable until GC. Password prompts wrap the returned string in `memguard.NewEnclave([]byte(s))` immediately, leaving only that transient string for the GC.
- `atotto/clipboard.WriteAll(string)` requires a transient `string(record.Password)` conversion at the boundary; that heap copy lives until GC.

### Record model

`storage.Record`: `Username` (string, JSON tag `user`), `Password` (`[]byte`, JSON tag `pass,omitempty`), `Value` (`[]byte`, JSON tag `value,omitempty`), `MTime` (`json:"mtime,omitempty"`, UTC ms). Each record uses **either** user+pass **or** single value — never both. Discriminator: `len(Value) != 0`.

- `psw add` defaults to user+pass; `--single`/`-s` → single value.
- `psw get` and `change` both branch on `len(record.Value) == 0` for which fields to show/edit.
- Bytes printed/copied via `string(record.Password)` at CLI boundaries; valid UTF-8 round-trips.

`"main"` is reserved: `psw add main` rejected; `psw change main` re-encrypts entire storage under a new main password (no record mutated).

### Interactive record selection

`get`/`change`/`remove` resolve via `prompt.GetRecordNameInteractive` (`internal/prompt/picker.go`) — in-process `bubbles/list` fuzzy picker, no `PATH` deps. Single match returned without launching the TUI — intentional (prevents confirming a forced choice); keep before changing selection logic. On Esc/Ctrl-C picker returns `ErrPickerCancelled`; `helpers.go` translates to silent exit.

### Menu mode (hotkey terminals)

Bare `psw` (`internal/cli/root.go` → `internal/menu/`) = single persistent `tea.Program` hosting every action as an embedded sub-model. Phases under the PSW ASCII header: (1) password entry — animated stars (`prompt.InputModel` with `animateStars=true`) plus a per-keypress non-cyan logo flash for 250ms; (2) action select — 4+2 grid: row 1 `get/add/change/remove`, row 2 `settings/rollback` under cols 3-4 (`menuEntries` in `layout.go`). Default `get`. ←/→ or h/l move within a row, ↑/↓ or j/k move between rows in the same column, `1`-`6` jump directly, `enter` runs, `q/esc/ctrl+c` quits psw; (3) running an action — the chosen `Action` (in `internal/menu/{get,add,change,remove,settings,rollback}.go`) drives a state machine over `prompt.InputModel` / `prompt.YesNoModel` / `prompt.PickerModel` / `ui.SpinnerModel` and emits output lines appended to the menu's history (capped at 20 blocks, rendered above the buttons). After completion, returns to action-select. Esc inside an action returns to action-select with no output appended; only Esc/q at action-select (or password phase) exits psw.

Each action embeds `baseAction` (output/transcript/done/cancelled + accessors) and uses `stepInput`/`stepYesNo`/`stepPicker` helpers to compress per-phase boilerplate. The picker emits its own help line for standalone CLI usage; menu hosts call `PickerModel.WithoutHelp()` and surface help via `Action.FooterHelp()` at the bottom row instead.

Async ops (Argon2id key derivation in `storage.LoadOrCreate`, save+commit+push in `storage.GitCommit`) run via `tea.Cmd`s in `internal/menu/cmds.go`; the action stays in a spinner phase until the result message arrives. Add and Change start with a y/n branch (single-value record? change main password?) since menu mode has no flags. `change main` rotates the cached main-password enclave in `MenuModel.password` (via `Action.NewPassword() *memguard.Enclave`) so subsequent mutations decrypt correctly. `clipclean` is spawned as before; the persistent psw process stays alive and the child runs out its timer normally. Scrollback re-emit on the password phase stays plain `*`. Designed for terminal windows spawned on a hotkey (e.g. `foot -e psw` under niri/sway); foot exits when its child exits unless `-H`/`--hold` is passed, so closing the window is the launcher's job, not psw's. Non-TTY stdin → error + exit 1; no scripting mode. First-time vault creation through menu uses a single password input (no double-confirm) and silently encrypts an empty vault under that password; `psw add` is the recommended path for fresh setup. Storage `Warn(...)` calls during a menu run flow into the menu's `warnCollector` (set as `storage.WarnSink` for the duration of `Run()`) so they appear in the in-action transcript instead of stderr. The `settings` action edits pswcfg.toml directly (no storage decrypt, no pull): a 2-pane grid lists `storage.ConfigKeys` with resolved values; Enter prefills `prompt.InputModel` via `WithInitialValue`; Esc from the grid commits `config updated` if dirty. The `rollback` action mirrors the CLI rollback flow — pull → decrypt → pick → confirm → `storage.ApplyRollback` (which owns its own commit).

Standalone CLI calls (`psw get foo`, `psw add bar`, etc.) are independent — they live in `internal/cli/` and call the same primitives via the standalone wrappers (`runInput`, `YesOrNo`, `GetRecordNameInteractive`, `WithSpinner`). Each wraps the embeddable model in `tuiutil.Quitter[M]` (in `internal/tuiutil/`), which translates the model's `Done()`/`Cancelled()` into `tea.Quit` for `tea.NewProgram.Run()`.

## Conventions

- Colors via `github.com/TwiN/go-color`: record names green, hints/commands cyan, warnings yellow, errors red, rollback log lines purple, de-emphasized secondary hints gray.
- Errors via cobra `RunE`: print user-facing message, `return errExit` (empty-message sentinel in `internal/cli/root.go`) → exit 1 without cobra usage dump. `SilenceErrors`/`SilenceUsage` on `rootCmd` keep prior UX. Flag-validation (e.g. `add`'s mutual-exclusion) and `resolveRecordName` `--exact` paths return `errSilentExit`; callers thread `if errors.Is(err, errExit) { return errExit }`. Only `os.Exit(1)` outside `main`/tests is `cli.Execute`'s cobra-error fallback. Match surrounding style.
- `slog.Debug` gated by `--verbose`/`-v` may log non-secret signals (record names, sizes, counts) but never plaintext secrets or full record dumps; never `fmt.Println` raw passwords.
- Add subcommand: create `internal/cli/<name>.go` with a `*cobra.Command`, then list it in `rootCmd.AddCommand(...)` inside `internal/cli/root.go`'s `init()`.

## Testing / scripting mode

CLI runs unattended (no TUI prompts) via env vars + flags below.

### Env vars

- `PSW_HOME=<path>` — override storage dir (default = `os.UserConfigDir()/psw`). Tests get a fresh `t.TempDir()` per case.
- `PSW_MAIN_PASSWORD=<str>` — supplies main password; bypasses prompt + double-confirm on vault creation. Empty = unset (prompt).
- `PSW_NEW_MAIN_PASSWORD=<str>` — new main password for `change main`. Same handling.
- `PSW_GIT=0` — skip auto `git init` + per-mutation `git commit`. Default unchanged when unset.
- `PSW_FAST_ARGON=1` — weakens Argon2id (`t=1, m=64KiB, p=1`); tests only.
- `PSW_GIT_REMOTE=0` — local git commits OK; no fetch/pull/push. For offline mutations + sync tests simulating diverging devices.
- `PSW_ROLLBACK_TARGET=<short-sha>` — `psw rollback` only: skip the picker, use this SHA.
- `PSW_ROLLBACK_YES=1` — `psw rollback` only: skip the y/n confirm, assume yes. Must be set together with `PSW_ROLLBACK_TARGET`; partial set → exit 1.

Caveat: env-var passwords visible in `/proc/<pid>/environ`. Fine for tests/ephemeral scripts; not for daily use. No `--password` CLI flag (would expose via `ps`).

### Flags

Per-command: `psw <cmd> --help`. Quirk: when **any** of `change`'s `--rename/--username/--password/--value` is set, unset-field y/n prompts are also skipped (those fields stay unchanged). Lets `change foo --password=new --exact` run unattended.

### Config

`psw config` (bare) prints the resolved pswcfg.toml path. `psw config set <key> <value>` accepts: `clipboard_timeout`, `remote`, `length`, `min_digits`, `min_symbols`, `min_uppercase`, `min_lowercase`, `allow_repeat`. Unknown keys / type mismatches exit 1. `psw config reset` overwrites the user pswcfg.toml with the binary-adjacent template. Both write the file then call `GitCommit` (subject: `config updated` / `config reset`); `PSW_GIT=0` skips the commit and `Paths.gitRepoExists` is auto-detected from disk so config commits work even though `config` doesn't go through `GetOrCreateFor*`. TOML comments are not preserved across `set`; `reset` restores them.

### Exit codes

Most error paths print and `return` (exit 0). Scripting paths exit 1 explicitly:
- `--exact` with missing arg or unknown name
- `add` flag mutual-exclusion violations
- `change` with field flag that doesn't match the record type
- `change main` with record-level flags
- `config set <unknown_key>` or type mismatch
