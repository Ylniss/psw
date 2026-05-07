# Phase 1 detail: Argon2id format + migration

_Detail of Phase 1 from `plans/code-review-improvements.md`. Verified against HEAD `cdaac94` — no drift in cited files (encryption.go, git.go, filesys.go, cmd/psw/main.go, prmpt/prompts.go, go.mod, tests/main_test.go all match the parent plan's description)._

## Goal

Replace the SHA-256 KDF with Argon2id (per-vault salt, OWASP-balanced params) and the manual-nonce GCM with `cipher.NewGCMWithRandomNonce`. Add a `"PSW1"` magic prefix so future format bumps are unambiguous. Tighten file/dir permissions to 0600/0700. Drop `joho/godotenv/autoload`. Make `internal/strg/git.go`'s commit step add only `storage.psw` and `pswcfg.toml`. Migrate the maintainer's existing vault via a one-time `psw upgrade` cobra command that lives in the working tree, runs locally, and is **deleted before any commit**.

The shipped repo, after Phase 1 lands, only understands the v1 format. There is no v0-read path in production code, no auto-migration on first read, no `psw upgrade` command in git history.

## Constraints (re-stated for handoff)

- `internal/cli/upgrade.go` MUST NEVER be committed. `git log -p` after Phase 1 must not show this file ever existed.
- Single-user assumption: there is exactly one vault to migrate (the maintainer's, at `~/.psw/storage.psw`). No public migration support.
- No bubbletea v2, no file locking, no in-memory zeroing — explicitly out of scope.

## Pre-flight

1. Confirm `git rev-parse HEAD == cdaac94`. If not, re-verify drift by re-reading files in `## Repo context` of the parent plan.
2. `git status` should be clean apart from `plans/code-review-improvements.md` (deleted `plans/replace-fzf.md`, untracked `plans/code-review-improvements.md` + `plans/phase1-argon2id-migration.md` are expected).
3. `make build && make test` — baseline green. If red, fix before starting Phase 1.
4. **Paranoid out-of-tree backup**: `cp ~/.psw/storage.psw ~/psw-paranoid-bak-$(date +%Y%m%d-%H%M).psw`. The in-dir `.legacy-bak` is the primary fallback; this is belt-and-suspenders against a botched upgrade run.
5. Confirm `golang.org/x/crypto` is reachable: `go list -m golang.org/x/crypto` (will fail until added — that's expected; just verify network/module resolution works).

## File-by-file changes

### 1. `internal/strg/git.go` — surgical `git add` (LAND FIRST)

Why first: every `psw` mutation in the rest of the runbook (and any subsequent `psw add` the maintainer runs before the upgrade.go file is removed and committed) calls `GitCommit`. With the current `git add .`, that would silently stage `storage.psw.legacy-bak` into the vault's own git history.

Change line 58:

```go
// before
cmd := exec.Command("git", "add", ".")
// after
cmd := exec.Command("git", "add", "storage.psw", "pswcfg.toml")
```

Do **not** fix the `Initilizing` typo at line 34 — that's Phase 2.

### 2. `internal/strg/filesys.go` — permission tightening

Two edits:

- Line 12: `os.MkdirAll(path, 0755)` → `os.MkdirAll(path, 0700)`.
- Line 55: `os.WriteFile(dst, input, 0644)` → `os.WriteFile(dst, input, 0600)`, then add `os.Chmod(dst, 0600)` immediately after the write succeeds (Go's `WriteFile` does not change the mode of an *existing* file, so we need explicit chmod for re-runs).

```go
// in copyFile
if err := os.WriteFile(dst, input, 0600); err != nil {
    return fmt.Errorf("failed to write file to destination: %w", err)
}
if err := os.Chmod(dst, 0600); err != nil {
    return fmt.Errorf("failed to chmod destination file: %w", err)
}
return nil
```

Note: `os.MkdirAll` similarly does not change the mode of an existing directory. The maintainer's existing `~/.psw/` will keep its current mode after these changes — `psw upgrade` (below) handles the chmod for existing dirs/files.

### 3. `internal/strg/encryption.go` — full replacement

Delete `generateSha256Key`. Replace the file body with the version below. The shape stays the same (`EncryptStringToStorage` / `DecryptStringFromStorage`) so no caller changes.

```go
package strg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	magicV1      = "PSW1"
	saltLen      = 16
	keyLen       = 32
	argonTime    = 2
	argonMemory  = 64 * 1024 // KiB → 64 MiB
	argonThreads = 4
)

func EncryptStringToStorage(plainText, password string) error {
	return encryptStringToFile(Cfg.storageFilePath, plainText, password)
}

func DecryptStringFromStorage(password string) (string, error) {
	return decryptStringFromFile(Cfg.storageFilePath, password)
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keyLen)
}

func encryptStringToFile(filePath, plainText, password string) error {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// NewGCMWithRandomNonce: nonce is generated internally and prepended
	// to the output. Both nonce args to Seal/Open must be nil.
	sealed := gcm.Seal(nil, nil, []byte(plainText), nil)

	payload := make([]byte, 0, len(magicV1)+saltLen+len(sealed))
	payload = append(payload, magicV1...)
	payload = append(payload, salt...)
	payload = append(payload, sealed...)

	encoded := base64.StdEncoding.EncodeToString(payload)
	if err := os.WriteFile(filePath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}
	if err := os.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("failed to chmod encrypted file: %w", err)
	}
	return nil
}

func decryptStringFromFile(filePath, password string) (string, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read storage file: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", fmt.Errorf("failed to decode storage: %w", err)
	}

	if len(payload) < len(magicV1)+saltLen {
		return "", errors.New("storage file is corrupted or unrecognized")
	}
	if string(payload[:len(magicV1)]) != magicV1 {
		return "", errors.New("unrecognized storage format; expected PSW1")
	}

	salt := payload[len(magicV1) : len(magicV1)+saltLen]
	sealed := payload[len(magicV1)+saltLen:]

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	plain, err := gcm.Open(nil, nil, sealed, nil)
	if err != nil {
		return "", errors.New("Wrong password.")
	}
	return string(plain), nil
}
```

Notes:
- `cipher.NewGCMWithRandomNonce` is Go 1.24+; module is on `go 1.26.0` so it's available.
- The error string `"Wrong password."` is preserved verbatim — `internal/cli/get.go` and the integration tests rely on this string. (Verify by `grep -r "Wrong password" internal/ tests/` before changing.)
- Per-write Argon2id derivation costs ~150–250 ms on a desktop. Each `psw` invocation does at most one decrypt + one encrypt, so user-perceived cost is one derivation per command. Tests do many invocations; expect ~6 s added to the integration suite.

### 4. `cmd/psw/main.go` — drop dotenv autoload

Remove the autoload import. Final file:

```go
package main

import "github.com/ylniss/psw/internal/cli"

func main() {
	cli.Execute()
}
```

Verified that nothing in tests depends on `.env` autoload — `tests/main_test.go` injects env via `cmd.Env` (and via `os.Setenv` in helper code).

### 5. `go.mod` / `go.sum` — dependency churn

After steps 3–4:

```
go get golang.org/x/crypto/argon2
go mod tidy
```

Expected diffs:
- `+ golang.org/x/crypto v0.x.y` (new direct require; transitively pulls `golang.org/x/sys` which is already there)
- `- github.com/joho/godotenv v1.5.1`

### 6. `internal/cli/upgrade.go` — NEW, NEVER COMMITTED

Single file containing both the cobra command and the legacy SHA-256 decrypt helper. One file = one `rm` to forget.

```go
package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	color "github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prmpt"
	"github.com/ylniss/psw/internal/strg"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "(local-only) Migrate vault from legacy SHA-256 to v1 Argon2id.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpgrade()
	},
}

func init() { rootCmd.AddCommand(upgradeCmd) }

func runUpgrade() error {
	storagePath := storageDirForUpgrade()
	storageFilePath := filepath.Join(storagePath, "storage.psw")
	bakPath := storageFilePath + ".legacy-bak"

	if _, err := os.Stat(bakPath); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", bakPath)
	}
	if _, err := os.Stat(storageFilePath); err != nil {
		return fmt.Errorf("storage file %s not found: %w", storageFilePath, err)
	}

	pass, err := prmpt.PromptForMainPass(false)
	if err != nil {
		return err
	}

	plain, err := decryptLegacy(storageFilePath, pass)
	if err != nil {
		return fmt.Errorf("legacy decrypt failed (wrong password?): %w", err)
	}

	original, err := os.ReadFile(storageFilePath)
	if err != nil {
		return fmt.Errorf("read storage: %w", err)
	}
	if err := os.WriteFile(bakPath, original, 0600); err != nil {
		return fmt.Errorf("write bak: %w", err)
	}

	// Re-encrypt under v1 — overwrites storage.psw at Cfg.storageFilePath,
	// which equals storageFilePath above (PersistentPreRun has run InitConfig).
	if err := strg.EncryptStringToStorage(plain, pass); err != nil {
		return fmt.Errorf("re-encrypt: %w (recover with: mv %s %s)", err, bakPath, storageFilePath)
	}

	// Tighten permissions on the existing vault dir + cfg file (encryptStringToFile
	// already chmods storage.psw). Don't fail the upgrade on chmod errors — log only.
	if err := os.Chmod(storagePath, 0700); err != nil {
		fmt.Println(color.InYellow(fmt.Sprintf("warn: chmod 0700 %s: %v", storagePath, err)))
	}
	if cfgPath := filepath.Join(storagePath, "pswcfg.toml"); fileExists(cfgPath) {
		if err := os.Chmod(cfgPath, 0600); err != nil {
			fmt.Println(color.InYellow(fmt.Sprintf("warn: chmod 0600 %s: %v", cfgPath, err)))
		}
	}

	fmt.Println(color.InGreen("Vault upgraded to v1 (Argon2id)."))
	fmt.Println(color.InCyan("Backup at: " + bakPath))
	fmt.Println(color.InCyan("Verify with: ./bin/psw   (lists records)  and  ./bin/psw get <known-record>"))
	return nil
}

func storageDirForUpgrade() string {
	if v := os.Getenv("PSW_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".psw")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// decryptLegacy: SHA-256 → AES-GCM, base64(nonce[12] || ciphertext+tag).
// Used only by `psw upgrade`. Removed with the rest of upgrade.go before commit.
func decryptLegacy(filePath, password string) (string, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write([]byte(password))
	key := hasher.Sum(nil)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("data too short")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", errors.New("wrong password")
	}
	return string(plain), nil
}
```

Design notes for upgrade.go:
- Skips the `.tmp` + atomic-rename dance from the parent plan. Justification: `EncryptStringToStorage` overwrites in place, matching the rest of the codebase. The `.legacy-bak` is the recovery path; if the encrypt step fails mid-flight, `mv storage.psw.legacy-bak storage.psw` restores. A botched mid-write is the same risk every other `psw` mutation has.
- Does NOT call `strg.GitCommit` — the maintainer commits the vault manually post-verification. Avoids a bad-commit-then-revert dance in the vault's git history.
- Reads `PSW_HOME` directly rather than reaching into `Cfg.storagePath` (unexported). Keeps upgrade.go fully self-contained — no temporary getter to add to `strg.Config` and forget to remove.
- chmod errors are warnings, not fatal. The migration's hard requirement is data integrity; permission tightening is best-effort.

## Order of operations in the working tree

```
0. (already done) git status clean; paranoid bak made
1. edit internal/strg/git.go              # surgical add — first, see § 1
2. edit internal/strg/filesys.go          # 0700/0600 + chmod
3. edit internal/strg/encryption.go       # full replacement
4. edit cmd/psw/main.go                   # drop autoload
5. go get golang.org/x/crypto/argon2 && go mod tidy
6. write internal/cli/upgrade.go          # NEW, never committed
7. make build                             # both binaries compile
8. ./bin/psw upgrade                      # see runbook
9. ./bin/psw                              # list records
10. ./bin/psw get <known-record>          # spot check
11. ./bin/psw add __upgrade_smoke --single  # confirm write path works
12. ./bin/psw remove __upgrade_smoke      # clean up
13. rm internal/cli/upgrade.go
14. make build                            # confirms compile-clean without legacy code
15. make test                             # full integration suite
16. git status                            # verify NO upgrade.go, NO .legacy-bak
17. git add <specific files>; git commit
18. git push (optional)
```

## Migration runbook (verbose, for the executing maintainer)

Run from `/home/yolan/stuff/repo/psw`.

```
# Step 1: snapshot the working tree
git status                                    # expect: branch develop, plan files only
cp ~/.psw/storage.psw ~/psw-paranoid-bak-$(date +%Y%m%d-%H%M).psw

# Step 2: implement steps 1–6 from the order list above (edits + go.mod tidy + new upgrade.go)
# Step 3: build
make build
ls -la bin/psw bin/clipclean                  # both present

# Step 4: run upgrade against the real vault
./bin/psw upgrade
# Expected output:
#   prompt for main password
#   "Vault upgraded to v1 (Argon2id)."
#   "Backup at: ~/.psw/storage.psw.legacy-bak"
ls -la ~/.psw/                                # confirm storage.psw, storage.psw.legacy-bak both exist

# Step 5: confirm decrypt works under the new format
./bin/psw                                     # lists records — same set as before upgrade
./bin/psw get <known-record-name>             # confirms a record decrypts and clipboard works

# Step 6: confirm write-path works post-upgrade (catches "old git-add . staged the bak file" regressions)
./bin/psw add __upgrade_smoke --single
# enter any value at the prompt
./bin/psw remove __upgrade_smoke
cd ~/.psw && git log --oneline -5             # confirm last commits do NOT include storage.psw.legacy-bak
                                              # `git show <sha>` should only diff storage.psw

# Step 7: rollback rehearsal (optional but recommended on first attempt)
# Verify you know the recovery command WITHOUT actually rolling back:
echo "to rollback: mv ~/.psw/storage.psw.legacy-bak ~/.psw/storage.psw"

# Step 8: remove upgrade.go and rebuild
rm internal/cli/upgrade.go
make build                                    # MUST succeed — no references to legacy code remain
make test                                     # full integration suite, ~Y seconds

# Step 9: verify nothing leaks into the source repo's git status
git status
# Expected:
#   modified: internal/strg/git.go
#   modified: internal/strg/filesys.go
#   modified: internal/strg/encryption.go
#   modified: cmd/psw/main.go
#   modified: go.mod
#   modified: go.sum
#   modified: plans/code-review-improvements.md   (mark Phase 1 [x] before commit)
#   (optionally) deleted: plans/replace-fzf.md
#   (optionally) untracked: plans/phase1-argon2id-migration.md
# NOT in this list: internal/cli/upgrade.go (must be absent)

# Step 10: surgical staging + commit
git add internal/strg/git.go internal/strg/filesys.go internal/strg/encryption.go \
        cmd/psw/main.go go.mod go.sum plans/code-review-improvements.md
# decide whether to also commit plans/phase1-argon2id-migration.md and plans/replace-fzf.md deletion
git commit
git log --stat -1                              # final sanity: no upgrade.go, no .legacy-bak

# Step 11: push (optional)
git push
```

## Verification checklist (Phase 1 done when ALL pass)

- [ ] `make build` clean
- [ ] `make test` clean (expect ~6 s longer than baseline due to Argon2id)
- [ ] `~/.psw/storage.psw` decrypts under both `./bin/psw` and CI binaries; record listing matches pre-upgrade state
- [ ] `~/.psw/storage.psw.legacy-bak` exists locally (kept indefinitely as recovery)
- [ ] `~/.psw/.git` log shows no commits that include `storage.psw.legacy-bak`
- [ ] `git log -p --all -- internal/cli/upgrade.go` returns empty (file never existed in source git history)
- [ ] `git grep -i 'godotenv\|sha256' -- internal/ cmd/` finds no production references (test files / vendor are fine)
- [ ] `stat -c '%a' ~/.psw ~/.psw/storage.psw ~/.psw/pswcfg.toml` returns `700 600 600`

## Rollback

Tiered, depending on how far the upgrade got:

1. **`psw upgrade` failed before writing storage.psw**: nothing to do. `storage.psw` is untouched, `.legacy-bak` may or may not exist. Delete `.legacy-bak` if present.
2. **`psw upgrade` succeeded but verification failed** (e.g. `./bin/psw get` returns wrong-password): `mv ~/.psw/storage.psw.legacy-bak ~/.psw/storage.psw`. Then `git checkout -- internal/strg/encryption.go internal/strg/filesys.go internal/strg/git.go cmd/psw/main.go go.mod go.sum && rm internal/cli/upgrade.go && make build`.
3. **Catastrophic** (legacy-bak was lost or also corrupted): restore from the paranoid out-of-tree backup made in pre-flight step 4. Then case 2.

## Notes / risks

- **Single test target**: the only real-world v0→v1 migration is the maintainer's vault. There's no second corpus to validate against. Mitigation: paranoid out-of-tree backup before running.
- **Argon2id params chosen for desktop, not CI runners**: CI machines are typically weaker than developer desktops; the 6 s overhead estimate could be 10–15 s on GitHub-hosted runners. If CI gets unacceptably slow, options are (a) bump CI machine size, (b) cache the test-vault setup, (c) introduce a `PSW_TEST_KDF_FAST=1` env var that downgrades params for tests (not recommended — tests would no longer exercise prod KDF cost). Don't preemptively optimize; measure first.
- **`storage.psw.legacy-bak` is never auto-deleted**: deliberate. Belt-and-suspenders, kept indefinitely. The maintainer can manually delete it after a few weeks of confidence.
- **No `.gitignore` change in `~/.psw/`**: surgical `git add` already prevents `*.legacy-bak` from being staged; adding a `.gitignore` would be a secondary defense layer. Not done in Phase 1; can be added later if needed.
- **Order matters for step 1**: `internal/strg/git.go` must change before any `psw` mutation runs in this working tree, because the upgrade verification (step 6 above: `psw add __upgrade_smoke`) calls `GitCommit` which uses whatever `git add` form is compiled into the binary. Implementing in the order listed (git.go first) guarantees the verification add doesn't stage `.legacy-bak`.
- **`change main` regression risk**: Phase 1 changes the encrypt path. `internal/cli/change.go`'s `changeMainPass` calls `strg.EncryptStringToStorage` directly (not via `Storage.Save`). After Phase 1, `change main` re-encrypts under a fresh salt — this is correct and desired. Add an integration test if the suite doesn't already cover `change main` end-to-end (check `tests/` before adding).

## Decisions log additions for Phase 1 commit

After the commit lands, append to `plans/code-review-improvements.md`'s "Decisions log":

```
2026-MM-DD — skipped .tmp+rename atomic write in upgrade.go — `.legacy-bak` is the recovery path, matches existing codebase write semantics
2026-MM-DD — upgrade.go reads PSW_HOME directly instead of strg.Cfg.storagePath — keeps upgrade.go fully self-contained for single-rm cleanup
2026-MM-DD — added os.Chmod after os.WriteFile in encryptStringToFile + filesys.copyFile — Go's WriteFile does not change mode of existing files
```

## Hand-off

Implementer: read this file top-to-bottom. The runbook in "Migration runbook (verbose)" is the authoritative checklist. If something diverges (e.g. `cipher.NewGCMWithRandomNonce` API differs from what's shown — verify with `go doc crypto/cipher.NewGCMWithRandomNonce` before starting), surface the divergence and update this file before proceeding.
