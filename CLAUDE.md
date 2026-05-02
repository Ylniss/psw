# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & install

Makefile (shortcuts `b`/`i`/`c`):

- `make build` — `go mod tidy`, then build `./bin/psw` and `./bin/clipclean`. Copies `pswcfg-template.toml` → `./bin/pswcfg.toml` on first build (skipped if present). `psw` is built with `-ldflags="-X 'github.com/ylniss/psw/cmd.Version=$(VERSION)'"` — top-level `VERSION` file is the source of truth.
- `make install` — runs `build`, then `go install` for both binaries.
- `make clean` — wipes `./bin/`.

Nix flake (`nix/flake.nix`) builds both binaries via `gomod2nix` and copies `pswcfg-template.toml` → `$out/bin/pswcfg.toml`. Update `gomod2nix.toml` + `vendorHash` on dep changes.

CI (`.github/workflows/go.yml`): `make build`, sha256 the binaries, package `psw-$VERSION.tar.gz`, create a GitHub release tagged with `VERSION`.

No tests exist (`go test ./...` is a no-op).

## Two binaries, one repo

- `psw` (root `main.go` → `cmd.Execute`) is the CLI. `joho/godotenv/autoload` import → `.env` in CWD loaded at startup.
- `clipclean` (`clipclean/main.go`) is spawned in background by `psw get` after copying a secret. Snapshots clipboard, sleeps `clipboard_timeout` seconds (argv[1]), clears clipboard only if still matches snapshot. Must be on `PATH` for `psw get` — `make install` covers it.

## Architecture

### Package layout

- `cmd/` — Cobra commands. Each file self-registers with `rootCmd` via `init()`. `rootCmd` (`root.go`) lists all record names when invoked bare. `PersistentPreRun` calls `setupLogger()` + `strg.InitConfig()`.
- `strg/` — storage, encryption, config, git, filesys. Package-level singletons populated by `InitConfig`: `Cfg` (paths), `AppConfig` (parsed TOML).
- `prmpt/` — TUI prompts (cqroot/prompt for inputs, eiannone/keyboard for single-key y/n).

### Data dir lifecycle (`~/.psw/`)

`strg.InitConfig` (from `rootCmd.PersistentPreRun`):

1. Ensures `~/.psw/` exists.
2. Loads `~/.psw/pswcfg.toml`; if missing, copies `pswcfg.toml` from beside the executable into `~/.psw/`, then reads it. (Why `make build` seeds `bin/pswcfg.toml` from `pswcfg-template.toml`.)

On first storage access (`strg.GetOrCreateIfNotExists`):

3. If `~/.psw/storage.psw` missing → prompt for new main password, write encrypted `[]`.
4. If `~/.psw/.git` missing and `git` on `PATH` → `git init` + initial commit. Missing git → warn, continue. `Cfg.gitRepoExists` gates later commits.
5. Decrypt storage with main password.

Every successful `add`/`change`/`remove` → `strg.GitCommit(message)` (`git add . && git commit -m <message>` in `~/.psw/`). No-op when git unavailable.

### Encryption (`strg/encryption.go`)

AES-256-GCM, key = `sha256(mainPass)`. Each write: fresh random nonce → prepended to ciphertext → base64 → `storage.psw`. Any decryption failure → `"Wrong password."`. Format changes must keep `EncryptStringToStorage`/`DecryptStringFromStorage` aligned; existing storage becomes unreadable, no migration path.

### Record model

`strg.Record` has `User`/`Pass` and `Value`, but each record uses **either** user+pass **or** single value — never both. Discriminator: `Value == ""`.

- `psw add` defaults to user+pass; `--single`/`-s` → single value.
- `psw get`, `change`, root listing all branch on `record.Value == ""` for which fields to show/edit.

`"main"` is reserved: `psw add main` rejected; `psw change main` re-encrypts entire storage under a new main password (does not modify any record).

### fzf record selection

`get`/`change`/`remove` use `strg.GetRecordNameWithFzf`:

- Positional name given → candidates filtered by substring (`GetNamesWithPart`); else all names.
- Exactly one candidate → returned without fzf (recent "fzf fix when only 1 obj", commit `2691c58`).
- Else `fzf` spawned over stdin/stdout; must be on `PATH` or command errors.

The single-candidate short-circuit is intentional — prevents confirming a forced choice. Keep before changing selection logic.

## Conventions

- Output colorized via `github.com/TwiN/go-color`: record names green, hints/commands cyan, warnings yellow, errors red.
- Errors mostly surfaced by `fmt.Println(err.Error())` + `return`; no central handler. Match surrounding style.
- `log.Debugf` (logrus) gated by `--verbose`/`-v` — the only place secret-adjacent data may log; never `fmt.Println` raw passwords.
- Add subcommand: create `cmd/<name>.go` with a `*cobra.Command` and `rootCmd.AddCommand(...)` in `init()`.
