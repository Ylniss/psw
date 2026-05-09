# Phase 1 detail: path move + help cleanup

_Parent: `plans/menu-persistence-and-cleanup.md` (Phase 1)_
_Last updated: 2026-05-09 — shipped (uncommitted at time of update)_

## Status: shipped
All subtasks landed. Two open questions resolved in-flight:
- **`Use:` cleanup applied** — replaced multi-line hack with `Use: "psw"`; moved the no-args hint into `Long` ("Run 'psw' with no arguments to list all stored record names.").
- **`completion`/`help` order: fixed** — called `rootCmd.InitDefaultCompletionCmd()` then `InitDefaultHelpCmd()` in `root.go` `init()`; renders in spec order (`completion, help`).
- **macOS path: deferred** — kept `os.UserConfigDir()` default (`~/Library/Application Support/psw`). No macOS user has objected.

`make build && make test` pass; `bin/psw --help` order is `get, add, change, remove, menu, log, version, completion, help`.

## Goal
Two bundled changes shipped together:
1. **Path move**: data dir moves from `~/.psw/` to the OS user config dir (`~/.config/psw` on Linux, `%AppData%\psw` on Windows, `~/Library/Application Support/psw` on macOS). `PSW_HOME` still wins. No migration code.
2. **Help cleanup**: `psw --help` lists commands in the order `get, add, change, remove, menu, log, version, completion, help`; long description corrected to "AES-256-GCM with Argon2id key derivation" and the new path.

## Drift check vs parent plan
Verified at commit 3f96999:
- ✅ `setStoragePath` at `internal/storage/config.go:46` — present.
- ✅ Each `internal/cli/<name>.go` self-registers via `init()` — confirmed in `version.go:12`, `get.go:26`, `remove.go:16`, `menu.go:49`, `change.go:28`, `add.go:29`, `log.go:13`.
- ✅ rootCmd `Long` references "AES encryption with SHA256" and `~/.psw` (`internal/cli/root.go:33-35`).
- ⚠️ Parent plan claims **six** `~/.psw/` references in CLAUDE.md; actual count is **three** (lines 28, 30, 74). Parent plan is stale on count, not on substance.
- ✅ Tests use `PSW_HOME=t.TempDir()` (`tests/helpers_test.go:52`) — path-agnostic, unaffected by default-path change.
- ✅ User already moved local data: `~/.config/psw/` populated, `~/.psw/` gone. New code will pick it up automatically.

## Cross-platform decision
**Use `os.UserConfigDir()` from the Go stdlib.** Single call gives the right path on all three platforms:

| OS | Returned path | psw data dir |
|---|---|---|
| Linux | `$XDG_CONFIG_HOME` if set, else `$HOME/.config` | `~/.config/psw` |
| Windows | `%AppData%` (e.g. `C:\Users\X\AppData\Roaming`) | `%AppData%\psw` |
| macOS | `$HOME/Library/Application Support` | `~/Library/Application Support/psw` |

Trade-off: macOS users get the OS-conventional path, not a `~/.config`-style XDG path. This is the Go-idiomatic choice and matches what most modern CLIs do. If a macOS user ever asks for XDG-style on macOS, swap to a custom resolver — minor change. Flagged as an open question; defaulting to `os.UserConfigDir()`.

## Subtasks

### 1. Storage path resolution
**File:** `internal/storage/config.go`

Replace the body of `setStoragePath` (lines 46-75). Current default branch:
```go
home, err := os.UserHomeDir()
if err != nil {
    return fmt.Errorf("retrieve home directory: %w", err)
}
path = filepath.Join(home, ".psw")
```

New default branch:
```go
configDir, err := os.UserConfigDir()
if err != nil {
    return fmt.Errorf("retrieve user config directory: %w", err)
}
path = filepath.Join(configDir, "psw")
```

Keep the `PSW_HOME` env-var override and the subsequent `expandPathWithHomePrefix` / `ensureDirExists` calls untouched. `expandPathWithHomePrefix` is now a no-op for the default branch (absolute path) but still needed for `PSW_HOME=~/foo` use cases — leave it.

**No migration logic.** If `~/.psw/` exists and the new path is empty, psw treats it as a first-run vault. Existing user (you) already moved data manually.

### 2. Help command order
**Files:** `internal/cli/root.go` + every `internal/cli/<name>.go` with an `init()` that calls `rootCmd.AddCommand`.

In `internal/cli/root.go`:
- Add `cobra.EnableCommandSorting = false` to the existing `init()` (above the flag binding).
- Centralize registration in the same `init()`:
  ```go
  rootCmd.AddCommand(getCmd, addCmd, changeCmd, removeCmd, menuCmd, logCmd, versionCmd)
  ```

In each of `get.go`, `add.go`, `change.go`, `remove.go`, `menu.go`, `log.go`, `version.go`:
- Delete the `rootCmd.AddCommand(...)` line from the file's `init()`.
- If that leaves `init()` empty, delete the function. Otherwise keep the rest (e.g. flag bindings).

Go init order makes this safe: package-level `var <name>Cmd = &cobra.Command{...}` initializers run before any `init()`, so all `*Cmd` values are populated when `root.go`'s `init()` runs. Files compile in any order; `init()` execution order across files is by lexical filename, but our centralized `init()` doesn't depend on other files' `init()`s.

**Cobra's auto-added `completion` and `help`:** with sorting disabled, these typically render after explicit registrations. Verify after the change by running `bin/psw --help` — they should appear at the bottom in that order. If `help` shows mid-list, no fix available short of swapping `cobra.Command.SetHelpCommand` for a custom one — punt unless rendering is actually wrong.

### 3. Long description and Use: line
**File:** `internal/cli/root.go:30-39`

Replace `Long`:
```go
Long: `psw is a simple password manager that secures your passwords using AES-256-GCM with Argon2id key derivation.

A 'psw' directory is created under your user config directory (e.g. ~/.config/psw on Linux, %AppData%\psw on Windows) to store:
storage.psw: an encrypted file where your passwords are saved.
pswcfg.toml: a configuration file for customizing app behavior.

On first use, you'll set a main password to protect your stored passwords.`,
```

`Use:` is currently:
```go
Use: `psw        lists all stored record names
  psw`,
```

This is a hack that injects a description into the cobra-generated `Usage:` block. Verify it still renders sanely after `EnableCommandSorting = false`; cobra's usage template renders `Use` independent of the command list, so behavior should be unchanged. Touch only if rendering is visibly broken.

### 4. CLAUDE.md
**File:** `CLAUDE.md`

Three `~/.psw` occurrences to update (line numbers from current HEAD):
- Line 28 heading: `### Data dir (~/.psw/)` → `### Data dir`
- Line 30: `ensures ~/.psw/ and loads pswcfg.toml` → `ensures the data dir (XDG config dir + 'psw'; e.g. ~/.config/psw on Linux, %AppData%\psw on Windows) and loads pswcfg.toml`
- Line 74 (Testing/scripting): `PSW_HOME=<path> — override storage dir (default ~/.psw)` → `PSW_HOME=<path> — override storage dir (default = $XDG_CONFIG_HOME/psw on Linux, %AppData%\psw on Windows; via os.UserConfigDir)`

Keep the wording terse; matches the surrounding shorthand style.

### 5. No-op verification list
These do **not** need changes:
- `Makefile` — no path references (only `BIN_DIR` and template copy).
- `pswcfg-template.toml` — content unrelated to data dir location.
- `nix/flake.nix` — references the package name `psw`, not the data dir.
- `gomod2nix.toml` / `vendorHash` — only bumped on dep changes, none here.
- `tests/helpers_test.go` — uses `PSW_HOME=t.TempDir()`; default-path change irrelevant.
- All other `tests/*.go` — same.
- `internal/storage/filesys.go` — `expandPathWithHomePrefix` still useful for `PSW_HOME=~/foo`.

## Done when
- `make build && make test` passes (no test changes expected).
- `bin/psw` reads/writes from `~/.config/psw/` on this Linux machine (verify by running `bin/psw` with no args after a clean build; should list existing records).
- `bin/psw --help` lists subcommands in this order: `get, add, change, remove, menu, log, version, completion, help`.
- `bin/psw --help` long description includes "AES-256-GCM with Argon2id key derivation" and references the OS user config directory (no `~/.psw`).
- `grep -r "~/\.psw" CLAUDE.md internal/` returns no matches.
- `grep -r "AES.*SHA256" internal/` returns no matches.

## Risks
- **Help rendering**: cobra's templating with `EnableCommandSorting = false` is well-tested but the `Use:` hack is unusual; eyeball the output before declaring done.
- **Windows path correctness**: code-only change, no Windows CI here. The `os.UserConfigDir()` API is documented and stable; `filepath.Join` handles separators. Confidence high without a Windows test rig.
- **macOS UX surprise**: `~/Library/Application Support/psw` is unconventional for a CLI vault. Acceptable trade-off for stdlib simplicity. Track as open question.

## Open questions
- ~~macOS path: keep `os.UserConfigDir()` or override to `~/.config/psw`?~~ — kept stdlib default; revisit if a macOS user objects.
- ~~`Use:` line rewrite~~ — applied: `Use: "psw"` + no-args hint in `Long`.

## Files touched
- `internal/storage/config.go` (1 function body)
- `internal/cli/root.go` (init body + Long string)
- `internal/cli/get.go`, `add.go`, `change.go`, `remove.go`, `menu.go`, `log.go`, `version.go` (one line removed each, possibly empty init removed)
- `CLAUDE.md` (3 line edits)

## Hand-off
Phase shipped — this file is now historical reference for what landed. Future phases pick up from `plans/menu-persistence-and-cleanup.md`.
