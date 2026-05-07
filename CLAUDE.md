# CLAUDE.md

Guidance for Claude Code in this repo.

## Build & install

- `make build` — builds `./bin/{psw,clipclean}` with `psw` ldflags `-X 'github.com/ylniss/psw/internal/cli.Version=$(VERSION)'` (top-level `VERSION` file = source of truth).

Nix flake (`nix/flake.nix`): update `gomod2nix.toml` + `vendorHash` on dep changes.

Integration tests under `tests/` (`make test`): `TestMain` builds `psw` once into `t.TempDir`; each test shells out against its own `PSW_HOME=t.TempDir()` vault with `PSW_GIT=0`.

## Two binaries, one repo

- `psw` (`cmd/psw/main.go` → `cli.Execute`) — the CLI. `joho/godotenv/autoload` → `.env` in CWD loaded at startup.
- `clipclean` (`cmd/clipclean/main.go`) — backgrounded by `psw get` to clear clipboard after timeout. Must be on `PATH` (covered by `make install`).

## Architecture

### Package layout

- `cmd/<binary>/main.go` — entry points. Thin wrappers; real logic under `internal/`.
- `internal/cli/` — Cobra commands (`package cli`); each file self-registers with `rootCmd` via `init()`. `rootCmd` (`root.go`) lists records when called bare; its `PersistentPreRun` runs `setupLogger()` + `strg.InitConfig()`. `Version` = ldflag target (see Build & install).
- `internal/strg/` — storage + encryption. `InitConfig` populates package-level singletons `Cfg` (paths) and `AppConfig` (parsed TOML).
- `internal/prmpt/` — TUI prompts. `YesOrNo` returns `false` on non-TTY stdin (no panic) — scripting-safe.
- `plans/` — design notes for in-flight or completed reshapes.

### Data dir (`~/.psw/`)

`strg.InitConfig` (from `rootCmd.PersistentPreRun`) ensures `~/.psw/` and loads `pswcfg.toml` (seeded from beside the executable on first run — why `make build` copies `pswcfg-template.toml` → `bin/`). On first storage access (`strg.GetOrCreateIfNotExists`), prompts for main password and (if `git` on `PATH`) `git init`s; `Cfg.gitRepoExists` gates per-mutation `GitCommit` calls. No-op when git unavailable.

### Encryption (`internal/strg/encryption.go`)

AES-256-GCM, key = `sha256(mainPass)`. Each write: fresh nonce → prepended to ciphertext → base64 → `storage.psw`. Decrypt failure → `"Wrong password."`. Format changes must keep `EncryptStringToStorage`/`DecryptStringFromStorage` aligned; existing storage becomes unreadable, no migration.

### Record model

`strg.Record` has `User`/`Pass` and `Value`, but each record uses **either** user+pass **or** single value — never both. Discriminator: `Value == ""`.

- `psw add` defaults to user+pass; `--single`/`-s` → single value.
- `psw get`, `change`, root listing all branch on `record.Value == ""` for which fields to show/edit.

`"main"` is reserved: `psw add main` rejected; `psw change main` re-encrypts entire storage under a new main password (does not modify any record).

### Interactive record selection

`get`/`change`/`remove` resolve via `strg.GetRecordNameInteractive` (`internal/strg/picker.go`) — in-process `bubbles/list` fuzzy picker, no `PATH` deps. When there's only one matching record, it's returned without launching the TUI — intentional, prevents confirming a forced choice; keep before changing selection logic. On Esc/Ctrl-C the picker returns `ErrPickerCancelled`, and `helpers.go` translates that to a silent exit.

## Conventions

- Output colorized via `github.com/TwiN/go-color`: record names green, hints/commands cyan, warnings yellow, errors red.
- Errors via cobra `RunE`: return `errExit` (empty-message sentinel in `internal/cli/root.go`) → exit 1 without usage dump; `SilenceErrors`/`SilenceUsage` on `rootCmd` keep prior UX (command already printed its colored error). Flag-validation/resolve paths still use `fmt.Println` + `os.Exit(1)`. Match surrounding style.
- `slog.Debug` gated by `--verbose`/`-v` is the only place secret-adjacent data may log; never `fmt.Println` raw passwords.
- Add subcommand: create `internal/cli/<name>.go` with a `*cobra.Command` and `rootCmd.AddCommand(...)` in `init()`.

## Testing / scripting mode

CLI can run unattended (no TUI prompts) via the env vars and flags below.

### Env vars

- `PSW_HOME=<path>` — override storage dir (default `~/.psw`). Tests get a fresh `t.TempDir()` per case.
- `PSW_MAIN_PASSWORD=<str>` — supplies main password; bypasses prompt + double-confirm on vault creation. Length-validated (≥4); too short fails loudly, no prompt fallback.
- `PSW_NEW_MAIN_PASSWORD=<str>` — new main password for `change main`. Same validation.
- `PSW_GIT=0` — skip auto `git init` + per-mutation `git commit` in the storage dir. Default behavior unchanged when unset.

Caveat: env-var passwords visible in `/proc/<pid>/environ`. Fine for tests/ephemeral scripts; not for daily use. No `--password` CLI flag (would expose via `ps`).

### Flags

Per-command flags: `psw <cmd> --help`. Notable quirk: when **any** of `change`'s `--rename/--username/--password/--value` is set, all unset-field y/n prompts are also skipped (those fields stay unchanged). Lets `change foo --password=new --exact` run unattended.

### Exit codes

Most error paths print and `return` (exit 0). Scripting paths exit 1 explicitly:
- `--exact` with missing arg or unknown name
- `add` flag mutual-exclusion violations
- `change` with field flag that doesn't match the record type
- `change main` with record-level flags
