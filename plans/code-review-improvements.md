# Code review improvements

_Last updated: 2026-05-07 — Phase 1 done. Re-verify cited files against current HEAD before each phase._

## Goal
Land a multi-phase improvement pass on the psw codebase derived from a full review session. The headline change is moving from single-pass SHA-256 password derivation to Argon2id with a per-vault salt (security hardening), plus tightened file permissions and a surgical git-commit policy. Subsequent phases fix latent correctness bugs, consolidate two prompt frameworks down to one, and clean up Go style/idioms. "Done" means the encrypted vault uses Argon2id, the codebase has only one prompt framework (charmbracelet/bubbles), CI is green, the bug list from the review is empty, and CLAUDE.md reflects the new state.

## Constraints / non-goals
- **No bubbletea v1 → v2 migration.** Stay on bubbletea v1.x / bubbles v1.x.
- **No file locking on storage.psw.** Concurrent invocations remain a known limitation.
- **No in-memory secret zeroing.** Out of scope for this pass.
- **The `psw upgrade` migration command MUST NEVER be committed.** It is a one-time, local-only tool used to convert the user's existing vault. The shipped repo only ever understands the new v1 format.
- **Single-user assumption.** This is a personal tool; there is exactly one vault to migrate (the maintainer's). No public migration support, no compatibility shims, no "v0 read path" lingering in production code.

## Key decisions (pre-implementation)

- **KDF: Argon2id, not scrypt or PBKDF2.** Argon2id is the modern OWASP default and won the Password Hashing Competition. PBKDF2 is GPU-vulnerable; scrypt is fine but Argon2id is the cleaner choice for new code.
- **Argon2id params: m=64 MiB, t=2, p=4, keylen=32, saltlen=16.** OWASP "balanced for desktop." Unlock cost is ~200 ms per command — invisible to interactive use, adds ~6 s to the integration test suite (acceptable). Rejected the OWASP minimum (m=19 MiB / t=2 / p=1) — over-shooting params is the safer long-lived choice since the format embeds them implicitly.
- **Format: `base64("PSW1" || salt[16] || gcm_seal_output)`.** `gcm_seal_output` is whatever `cipher.NewGCMWithRandomNonce(...).Seal(...)` returns (Go 1.24+ — nonce is generated and prepended internally). Magic prefix `"PSW1"` discriminates v1 from anything else; today there is no "anything else" in shipped code, but the prefix future-proofs format bumps and makes hand-debugging easier.
- **Migration via one-time `psw upgrade`, never committed.** Rejected auto-migration on first read because (a) it requires keeping legacy decrypt code in production indefinitely, (b) the user only has one vault to migrate so a manual step is fine, (c) keeps the shipped codebase narrowly focused on v1. The `upgrade.go` file (containing both the cobra command and the legacy SHA-256 decrypt helper) is added, used locally, and removed before any commit.
- **Surgical git add (`git add storage.psw pswcfg.toml`).** Rejected `git add .` because the upgrade run produces `storage.psw.legacy-bak`, which `git add .` would silently stage — committing a legacy-encrypted backup of the vault into the user's git history forever. Surgical add is also a small general improvement (no risk of stray dotfiles getting committed).
- **Backup file inside `~/.psw/`.** Once `git add` is surgical, in-dir backup is fine. Rejected sibling-dir (`~/.psw-backups/`) because it splits state across two locations for no benefit.
- **Drop `joho/godotenv/autoload`.** Verified unused — integration tests inject env via `cmd.Env`, not via .env files. The autoload behavior is a footgun (psw run from any directory sources whatever `.env` is there).
- **Drop `cqroot/prompt` + `cqroot/multichoose`.** Both are bubbletea wrappers; we already depend on bubbletea+bubbles for the picker. Consolidating removes two deps, the only `panic()` in the codebase (`prmpt.YesOrNo` raw-mode setup), and 5 scattered `os.Exit(1)` sites tied to `prompt.ErrUserQuit` handling.
- **Convert `prmpt.YesOrNo` to bubbles too.** Rejected leaving it alone for consistency — same prompt framework everywhere, kills the panic.
- **`psw upgrade` co-locates legacy decrypt + command in one file.** Rejected splitting into `encryption_legacy.go` + `upgrade.go` — single `rm` is easier to get right than two `rm`s.

## Repo context a fresh Claude needs

- **Single-binary CLI plus a `clipclean` helper.** `cmd/psw/main.go` → `internal/cli`. `cmd/clipclean/main.go` is backgrounded by `psw get` to clear the clipboard after a timeout.
- **Storage location.** `~/.psw/` (overridable via `PSW_HOME`). Contains `storage.psw` (encrypted vault), `pswcfg.toml` (config), and a `.git/` repo when git is available.
- **Git integration is auto-init on first vault use.** `internal/strg/git.go` runs `git init` and commits per mutation. `PSW_GIT=0` opts out (used by tests). The `git add` line at `internal/strg/git.go:58` is currently `git add .` — Phase 1 changes it to `git add storage.psw pswcfg.toml`.
- **Storage format today.** `internal/strg/encryption.go`: `base64( nonce[12] || ciphertext_with_tag )`, key = `sha256(mainPass)`. No salt. Phase 1 replaces this entirely.
- **Two binaries built with same Make target.** `make build` produces `./bin/psw` (with version ldflag) and `./bin/clipclean`, plus copies `pswcfg-template.toml` → `bin/pswcfg.toml`.
- **Integration tests rebuild psw once per run.** `tests/main_test.go:TestMain` builds into `t.TempDir()`. Each test gets a fresh `PSW_HOME` via `newVault(t)`. Tests set `PSW_MAIN_PASSWORD=testpass`, `PSW_GIT=0`. They use `cmd.Env` directly (no .env file), so removing `joho/godotenv/autoload` does not affect tests.
- **Two prompt frameworks in use.** `internal/prmpt/prompts.go` uses `cqroot/prompt` for password and name input, and a hand-rolled raw-mode `YesOrNo`. `internal/strg/picker.go` uses `bubbletea` + `bubbles/list` for record selection. Phase 3 collapses everything onto bubbletea/bubbles.
- **Scripting/test mode escape hatches.** `PSW_HOME`, `PSW_MAIN_PASSWORD`, `PSW_NEW_MAIN_PASSWORD`, `PSW_GIT=0`. Documented in CLAUDE.md. Phase 3's prompt rewrite must preserve these — bubbletea on a non-TTY must fail loudly with a clear message, NOT hang. The env-var bypass should short-circuit the prompt entirely.
- **`prmpt.YesOrNo` non-TTY behavior.** Returns `false` on non-TTY stdin (scripting-safe). Phase 3 must preserve this.
- **Cobra error conventions.** `errExit = errors.New("")` is the sentinel for "exit 1, but I already printed the error" — used in `RunE` returns. `SilenceErrors`/`SilenceUsage` are set on rootCmd. Some paths print + `return nil` (exit 0), others `return errExit` (exit 1), others `os.Exit(1)` directly. Phase 4 narrows the third category.
- **`storage.psw.legacy-bak` is never auto-deleted.** Belt-and-suspenders for the migration.
- **Migration runbook is the unusual bit.** See Phase 1 for the exact local-only workflow. The `upgrade.go` file must NEVER be committed.

## Phases

### [x] Phase 1: Argon2id format + migration
- **Goal:** Vault on disk is v1 (Argon2id + GCM-with-random-nonce). Shipped code only understands v1. Permissions tightened. `joho/godotenv/autoload` gone. Git commits become surgical.
- **Scope:**
  - `internal/strg/git.go`: change `git add .` → `git add storage.psw pswcfg.toml`. Land FIRST so the upgrade run does not stage the bak file.
  - `internal/strg/filesys.go`: dir mode `0755 → 0700`, `copyFile` dst mode `0644 → 0600`.
  - `internal/strg/encryption.go`: replace SHA-256 KDF with Argon2id (`golang.org/x/crypto/argon2`, params m=65536/t=2/p=4/keylen=32). Replace `cipher.NewGCM` + manual nonce with `cipher.NewGCMWithRandomNonce`. Add 16-byte random salt per write. New format: `base64("PSW1" || salt[16] || gcm_seal_output)`. `os.WriteFile` mode `0644 → 0600`. Read path validates the magic prefix and rejects anything else with a clear error.
  - `cmd/psw/main.go`: drop `_ "github.com/joho/godotenv/autoload"` import. Remove from `go.mod` via `go mod tidy`.
  - `internal/cli/upgrade.go` (NEW, NEVER COMMITTED): cobra subcommand `psw upgrade`. Prompts for main password. Reads `storage.psw`, decrypts using a co-located `decryptLegacy()` helper (the old SHA-256 + GCM code), copies original to `storage.psw.legacy-bak` (refuse to overwrite if it already exists), re-encrypts using the new v1 path, writes to `storage.psw.tmp`, fsyncs, `os.Rename(tmp, storage.psw)`. Prints success summary. Do NOT auto-commit via `GitCommit` — the user can do that manually after verifying.
  - **Migration runbook (executed locally by the maintainer, NOT scripted):**
    1. Implement v1 production code AND `internal/cli/upgrade.go` in the same working tree.
    2. `make build`. Run `./bin/psw upgrade` against the real `~/.psw/` vault. Confirm "vault upgraded" message and presence of `~/.psw/storage.psw.legacy-bak`.
    3. Verify with `./bin/psw` (lists records) and `./bin/psw get <known-record>`. Roll back to bak if anything is wrong (`mv storage.psw.legacy-bak storage.psw`, revert local code).
    4. `rm internal/cli/upgrade.go`. `make build` again — confirms compile-clean without the legacy code.
    5. `make test` — full integration suite passes against fresh v1 vaults.
    6. Manually `git add` the production changes and commit. Verify `git status` shows no `upgrade.go` and no `storage.psw.legacy-bak` references.
    7. Push.
- **Done when:** v1 format only in shipped code; vault on disk is v1; integration tests green; `git log -p` does not show `upgrade.go` ever existing; `~/.psw/storage.psw.legacy-bak` exists locally as recovery (kept indefinitely).
- **Risk:** high — irreversible format change, manual local-only step, single-user vault is the only test target.
- **Depends on:** none. Must go first.

### [x] Phase 2: Correctness / latent bugs
- **Goal:** Six known bugs fixed, including the ones that cause user-visible weirdness (out-of-order listings after rename, case-sensitivity surprises, silent fall-through after a prompt error).
- **Scope:**
  - `internal/strg/storage.go`: `UpdateRecord` re-sorts records after writing back (fixes out-of-order listing after `change foo --rename=zzz`).
  - Case-sensitivity unification: pick one regime across `Exists`, `GetRecord`, `UpdateRecord`, `RemoveRecord`. Recommend case-insensitive everywhere (matches what `Exists` already does for the duplicate-name check). Update tests if any depend on the current asymmetry.
  - `internal/cli/change.go:60` (`changeMainPass`): add `return` after `fmt.Println(err.Error())` on the first prompt-error path.
  - `change main` should call through `Storage.Save` rather than `EncryptStringToStorage` directly. Either give `Save` an optional password override or have `change main` mutate `storage.MainPass` and call `Save()`.
  - `internal/prmpt/prompts.go:38`: replace `panic(err)` on `term.MakeRaw` failure with an error return; `YesOrNo` signature becomes `(string) (bool, error)` or returns `false` and logs.
  - Typo fixes: `Initilizing` → `Initializing` (`internal/strg/git.go:34`), `encypted` → `encrypted` (`internal/strg/encryption.go:52`), `doesn't exists` → `doesn't exist` (`internal/cli/remove.go:43`).
- **Done when:** integration tests still green; bug list from the review is empty.
- **Risk:** low.
- **Depends on:** Phase 1 (so the encryption module is settled before any storage refactors land).

### [ ] Phase 3: Library consolidation (cqroot → bubbles)
- **Goal:** Single prompt framework. `cqroot/prompt` + `cqroot/multichoose` removed from `go.mod`. `prmpt.YesOrNo` rewritten on bubbletea. All five `os.Exit(1)` sites in `internal/prmpt/prompts.go` become normal error returns.
- **Scope:**
  - Reimplement `PromptForName`, `PromptForRecordPass`, `PromptForMainPass`, `PromptForMainPassChange` on `bubbles/textinput` with `EchoMode = EchoPassword` for password fields. Match the existing API surface so callers don't change.
  - Reimplement `YesOrNo` as a tiny bubbletea model (single keypress y/n) — drops the raw-mode dance and the `term.MakeRaw` panic.
  - Preserve `PSW_MAIN_PASSWORD` / `PSW_NEW_MAIN_PASSWORD` env-var bypass — env check stays at the top of each prompt fn, short-circuits before launching bubbletea.
  - Non-TTY behavior: bubbletea on non-TTY hard-fails. Detect and return a clear error like "interactive prompt required; set PSW_MAIN_PASSWORD or run from a terminal" instead of hanging.
  - `go.mod`: remove `github.com/cqroot/prompt`, `github.com/cqroot/multichoose`. `go mod tidy`.
- **Done when:** integration tests green; cqroot deps absent from `go.mod`/`go.sum`; only one `panic()` in the repo (zero); only one prompt framework imported.
- **Risk:** medium — biggest substantive rewrite, but test coverage on the prompt flow is thin (mostly via env-var bypass), so manual smoke testing in a real TTY is required.
- **Depends on:** none after Phase 1; can ship before or after Phase 2.

### [ ] Phase 4: Code style / Go idioms
- **Goal:** golint-clean (or close); consistent error message style; `os.Exit` calls confined to `main` and CLI exit-code helpers.
- **Scope:**
  - Replace `fmt.Println(fmt.Sprintf(...))` with `fmt.Printf` at: `internal/cli/version.go:19`, `internal/cli/get.go:76, 87, 92, 100, 107`.
  - Fix `easyHandler.WithAttrs` (currently no-ops) so `slog` attrs propagate. Then convert `slog.Debug(fmt.Sprintf(...))` sites in `cli/change.go:121`, `cli/get.go:55`, `strg/storage.go:115`, `strg/filesys.go:16`, `strg/config.go:132` to structured calls (`slog.Debug("msg", "key", val)`).
  - Error message normalization across `internal/strg/encryption.go`, `internal/strg/filesys.go`, `internal/strg/git.go`: lowercase first letter, drop trailing punctuation, drop redundant "Error when …:" prefixes, drop awkward `"\n%w"` wrapping in favor of `": %w"`.
  - `os.Exit` cleanup OUTSIDE of what Phase 3 already removed: `internal/cli/helpers.go:21,26` (`--exact` validation) returns errors; `internal/strg/config.go` `InitConfig` becomes returning fn called from `PersistentPreRunE`.
  - `sort.Slice` → `slices.SortFunc` in `internal/strg/storage.go:62`.
  - `Storage.Exists` iterates `s.Records` directly instead of allocating a fresh slice via `GetNames()`. Same treatment for `GetNamesWithPart`.
  - `generateSha256Key` is gone with Phase 1; if any helper remains, prefer `sha256.Sum256(...)` form.
  - `fmt.Printf(coloredString, args)` → `fmt.Printf("%s\n", colored…)` at `internal/strg/git.go:34` and `internal/cli/change.go:161`.
  - Deduplicate the user/pass vs value branches in `internal/cli/get.go` via a `printSecret(label, secret string, reveal bool, dur int)` helper.
- **Done when:** integration tests green; `go vet` clean; no `fmt.Println(fmt.Sprintf(...))` matches in the tree; one consistent error-message style.
- **Risk:** low — mostly mechanical.
- **Depends on:** Phase 3 (which removes the prmpt `os.Exit` sites that would otherwise be in scope here).

### [ ] Phase 5: Smaller hygiene
- **Goal:** Tiny polish items grouped to keep them out of larger PRs.
- **Scope:**
  - `cmd/clipclean/main.go`: validate `duration > 0`; bare `return` if 0 or negative.
  - `.github/workflows/go.yml`: bump `actions/setup-go@v4` → `@v5`.
  - `gofumpt` (or `goimports`) pass on `internal/cli/change.go` and `internal/cli/get.go` (the `log/slog` import is currently separated by a blank line for no reason).
- **Done when:** CI green; CLI behaves identically.
- **Risk:** low.
- **Depends on:** none.

### [ ] Phase 6: CLAUDE.md update
- **Goal:** Project documentation reflects the new state. Anything a fresh Claude reads via CLAUDE.md is current.
- **Scope:**
  - Encryption section: replace "AES-256-GCM, key = sha256(mainPass)" with the Argon2id description (params, salt, format magic prefix, `NewGCMWithRandomNonce`).
  - Git integration: note the surgical `git add storage.psw pswcfg.toml` policy.
  - Dependencies: drop mentions of `cqroot/prompt` / `joho/godotenv` from any examples.
  - Conventions: if Phase 4 changed the error/exit conventions in any way that affects "Add subcommand", update that note.
  - Optional: short note that `psw upgrade` was a one-time, never-committed migration tool — for future-self reference, no detail required.
- **Done when:** `CLAUDE.md` accurately describes the post-changes state of the repo.
- **Risk:** zero.
- **Depends on:** all prior phases (it documents their outcome).

## Decisions log (during implementation)
_Append-only. Format: `YYYY-MM-DD — decision — why`._

- 2026-05-07 — Phase 1: skipped `.tmp`+rename atomic write in `upgrade.go` — `.legacy-bak` is the recovery path, matches the rest of the codebase's in-place-write semantics.
- 2026-05-07 — Phase 1: `upgrade.go` reads `PSW_HOME` directly instead of `strg.Cfg.storagePath` — keeps the never-committed file fully self-contained for a single-`rm` cleanup.
- 2026-05-07 — Phase 1: added `os.Chmod` after `os.WriteFile` in `encryptStringToFile` and `filesys.copyFile` — Go's `WriteFile` does not change the mode of an existing file.
- 2026-05-07 — Phase 2: skipped the parent plan's `encypted → encrypted` typo bullet — Phase 1's encryption.go rewrite already used the correct spelling.
- 2026-05-07 — Phase 2: extracted `Storage.sortRecords()` helper instead of inlining sort in two callsites — shared by `AddRecord` and `UpdateRecord`, clearer intent.
- 2026-05-07 — Phase 2: kept case-sensitive sort (`<`) — records cannot collide on case (`Exists` is `EqualFold`), so order is deterministic. Phase 4 will revisit when switching to `slices.SortFunc`.
- 2026-05-07 — Phase 2: skipped a B3 regression test — exercising the fall-through requires TTY-injected prompt errors; covered indirectly by the existing `TestChange_Main` happy path.
- 2026-05-07 — Phase 2: kept `YesOrNo` signature `(string) bool` — Phase 3 will rewrite this on bubbletea entirely, so any signature change now is wasted churn.

## Open questions
None.

## Hand-off
To detail a phase, start a fresh context and ask:
> Prepare a detailed plan for phase N from `plans/code-review-improvements.md`.

**Before writing phase detail, verify the plan is not stale.** Compare the **Last updated** commit to current `HEAD`; read the files cited in **Repo context** to confirm they still exist and behave as described. If anything has drifted, surface the drift and update the plan before producing detail.

**Phase 1 has a non-obvious local-only workflow** — the `internal/cli/upgrade.go` file is added to the working tree, used to migrate the maintainer's vault, and removed before any commit. A fresh detail session for Phase 1 must understand that workflow and treat the upgrade file as ephemeral, not as a deliverable.
