# Phase 2 detail: Correctness / latent bugs

_Detail of Phase 2 from `plans/code-review-improvements.md`. Verified against HEAD `4e8d2e1` (Phase 1 landed). The cited files all match the parent plan with one drift correction noted below._

## Goal

Six latent bugs and one consistency issue, all low-risk. After Phase 2: `change` does not silently fall through after a failed prompt; `change main` re-uses `Storage.Save` (single write path); record listings stay sorted after `change --rename`; `psw get FOO` finds a record named `foo`; `prmpt.YesOrNo` does not panic on `term.MakeRaw` failures; two visible typos are fixed. Integration suite stays green and gains two regression tests.

## Drift notes (vs. parent plan)

- **Parent plan bullet "encypted → encrypted (encryption.go:52)" is already fixed.** Phase 1 rewrote `internal/strg/encryption.go` end-to-end; the new strings (`"failed to write encrypted file"`, `"failed to chmod encrypted file"`) already use the correct spelling. Skip this typo from Phase 2.
- The other two typos (`Initilizing` in `git.go:34`, `doesn't exists` in `remove.go:43`) are still present at HEAD `4e8d2e1`. Confirmed by re-reading both files.

## Pre-flight

1. `git rev-parse HEAD` should be `4e8d2e1` (the Phase 1 commit). If newer, re-verify drift.
2. `git status` should be clean apart from the still-uncommitted `plans/replace-fzf.md` deletion.
3. `make build && make test` baseline green. Test suite is currently ~18 s due to Argon2id from Phase 1; that's the new baseline.
4. **No vault data is touched in Phase 2.** All changes are code-only and reversible via `git checkout` if anything goes wrong.

## Bug-by-bug fixes

### B1 — `Storage.UpdateRecord` does not re-sort after rename

`internal/strg/storage.go:76-83`. Current code mutates `s.Records[i]` in place; if `change --rename` changes a name, the slice's alphabetical invariant is broken (since `AddRecord` is the only place that sorts). Visible as: `psw` listing shows records out of order after a rename.

Fix: extract the sort into a helper and call from both `AddRecord` and `UpdateRecord`:

```go
func (s *Storage) sortRecords() {
    sort.Slice(s.Records, func(i, j int) bool {
        return s.Records[i].Name < s.Records[j].Name
    })
}

func (s *Storage) AddRecord(r *Record) {
    s.Records = append(s.Records, *r)
    s.sortRecords()
}

func (s *Storage) UpdateRecord(name string, updatedRecord Record) {
    for i, r := range s.Records {
        if strings.EqualFold(r.Name, name) {
            s.Records[i] = updatedRecord
            s.sortRecords()
            return
        }
    }
}
```

(The `EqualFold` is from B2; the two changes are interleaved in this same function.)

Note: sort is left case-sensitive (uppercase before lowercase per ASCII). Records cannot collide on case (`Storage.Exists` already rejects via `EqualFold`), so the sort is deterministic regardless. Phase 4 will swap `sort.Slice` for `slices.SortFunc`; not Phase 2's concern.

### B2 — Case-sensitivity unification across lookup funcs

`internal/strg/storage.go`. `Exists` already uses `strings.EqualFold`; `GetRecord`, `UpdateRecord`, `RemoveRecord`, and `GetNamesWithPart` use exact string compares. The asymmetry creates a real not-found bug:

- `psw get FOO --exact` against a vault containing `foo`: `helpers.go` calls `storage.Exists("FOO")` → `true` (EqualFold), passes `"FOO"` through; `get.go` then calls `storage.GetRecord("FOO")` → `({}, false)`; user sees `"Record FOO was not found"`. Helper greenlit but lookup failed.

Fixes:

```go
// GetRecord
func (s *Storage) GetRecord(name string) (Record, bool) {
    for _, r := range s.Records {
        if strings.EqualFold(r.Name, name) {
            return r, true
        }
    }
    return Record{}, false
}

// RemoveRecord
func (s *Storage) RemoveRecord(name string) {
    s.Records = slices.DeleteFunc(s.Records, func(r Record) bool {
        return strings.EqualFold(r.Name, name)
    })
}

// GetNamesWithPart — substring becomes case-insensitive
func (s *Storage) GetNamesWithPart(namePart string) []string {
    lp := strings.ToLower(namePart)
    var matched []string
    for _, name := range s.GetNames() {
        if strings.Contains(strings.ToLower(name), lp) {
            matched = append(matched, name)
        }
    }
    return matched
}
```

`UpdateRecord` covered in B1.

Picker (`internal/strg/picker.go`) is unaffected — its filter uses `bubbles/list`'s built-in matching which is already case-insensitive (sahilm/fuzzy default). No callsite changes needed.

Test impact: existing tests use lowercase names throughout (`grep -i 'case\|EqualFold\|ToLower\|ToUpper' tests/` returns nothing). No tests break. Add one new regression test (see § New tests).

### B3 — `changeMainPass` falls through on first prompt error

`internal/cli/change.go:62-64`:

```go
mainPass, err := prmpt.PromptForMainPass(true)
if err != nil {
    fmt.Println(err.Error())
    // <-- missing return
}
storage, err := strg.Get(mainPass) // mainPass empty; cascades
```

Fix: add `return` after the println.

```go
mainPass, err := prmpt.PromptForMainPass(true)
if err != nil {
    fmt.Println(err.Error())
    return
}
```

(The other error paths in `changeMainPass` already have `return` — only the first is broken.)

### B4 — `change main` should route through `Storage.Save`

`internal/cli/change.go:72-88`. Current code calls `storage.ToJson()` + `strg.EncryptStringToStorage(json, newPass)` directly, bypassing `Save()`. The pre-Phase-2 doc comment on `Save` even acknowledges this: "_Use ToJson + EncryptStringToStorage directly when re-encrypting under a different password_". Phase 2 unifies the write path.

Mechanism: `strg.Get` already populates `storage.MainPass = mainPass`. Mutating `MainPass` and calling `Save` re-encrypts under the new password, since `Save` does `EncryptStringToStorage(json, s.MainPass)`.

Fix:

```go
func changeMainPass() {
    fmt.Println(color.InCyan("You are changing your main password!\nFirst enter your current password"))

    mainPass, err := prmpt.PromptForMainPass(true)
    if err != nil {
        fmt.Println(err.Error())
        return
    }

    storage, err := strg.Get(mainPass)
    if err != nil {
        fmt.Println(err.Error())
        return
    }

    newMainPass, err := prmpt.PromptForMainPassChange()
    if err != nil {
        fmt.Println(err.Error())
        return
    }

    storage.MainPass = newMainPass
    if err := storage.Save(); err != nil {
        fmt.Println(err.Error())
        return
    }

    fmt.Println(color.InGreen("Main password changed"))
}
```

Note the function is now also pruned of its dead trailing `return` and the unnecessary `ToJson()` call. The `strg` import for `EncryptStringToStorage` is still needed elsewhere in the file (none — `change.go` only uses `strg.Get`, `strg.GetOrCreateIfNotExists`, `strg.GitCommit`, `strg.Record`); leave imports as-is, `goimports`/`go build` will catch.

Update `Storage.Save` doc comment (`internal/strg/storage.go:107-110`):

```go
// Save serializes records to JSON and writes them encrypted under the
// storage's current MainPass. To change the main password, mutate
// storage.MainPass before calling Save.
func (s *Storage) Save() error {
```

Existing `TestChange_Main` covers this end-to-end (changes pass, decrypts under new pass, fails under old pass). Refactor is observation-equivalent.

### B5 — `prmpt.YesOrNo` panics on `term.MakeRaw` failure

`internal/prmpt/prompts.go:35-38`. `term.MakeRaw` failure currently calls `panic(err)`. Fix to match the existing non-TTY behavior (returns `false`, scripting-safe):

```go
oldState, err := term.MakeRaw(fd)
if err != nil {
    fmt.Fprintln(os.Stderr, color.InYellow(
        "warning: cannot enter raw mode for y/n prompt; defaulting to no"))
    return false
}
```

Phase 3 will rewrite this whole function on bubbletea, so don't change the signature — that would force Phase 3 callers to handle an error too, which is wasted churn. Returning `false` matches the documented non-TTY contract in `CLAUDE.md` ("non-TTY stdin (no panic) — scripting-safe").

### B6 — Typo fixes

- `internal/strg/git.go:34`: `Initilizing` → `Initializing`.
- `internal/cli/remove.go:43`: `doesn't exists` → `doesn't exist`.

(The encryption.go typo from the parent plan is already fixed — see drift note above.)

Test impact: `grep -rn 'Initilizing\|doesn.t exists' tests/` returns nothing. Safe to fix.

## New tests

Two regression tests in `tests/`. Both use `newVault` + non-interactive flag-driven `psw` invocations.

### `TestList_OrderAfterRename` (in `tests/list_test.go`)

Guards B1.

```go
func TestList_OrderAfterRename(t *testing.T) {
    t.Parallel()
    v := newVault(t)
    runPsw(t, v, "add", "alpha", "-u", "u", "--password=p")
    runPsw(t, v, "add", "beta", "-u", "u", "--password=p")
    runPsw(t, v, "change", "alpha", "--rename=zeta", "--exact")
    r := runPsw(t, v)
    mustExit(t, r, 0)
    var names []string
    for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
        if line == "" {
            continue
        }
        names = append(names, strings.SplitN(line, ".", 2)[0])
    }
    if len(names) != 2 || names[0] != "beta" || names[1] != "zeta" {
        t.Fatalf("expected [beta zeta] after rename, got %v\nstdout: %s", names, r.stdout)
    }
}
```

### `TestGet_CaseInsensitiveLookup` (in `tests/get_test.go`)

Guards B2.

```go
func TestGet_CaseInsensitiveLookup(t *testing.T) {
    t.Parallel()
    v := newVault(t)
    runPsw(t, v, "add", "Foo", "-u", "u", "--password=secret")
    r := runPsw(t, v, "get", "FOO", "--exact", "--stdout")
    mustExit(t, r, 0)
    mustEqual(t, trimmed(r), "secret")
}
```

(Skipping a regression test for B3 — exercising the fall-through requires injecting a mid-prompt error, hard without TTY mocking. B3's fix is a one-line `return` and the existing `TestChange_Main` happy path still covers normal operation.)

## Order of operations

Single working-tree pass; one commit at the end. Order within the tree:

```
1. internal/strg/storage.go         # B1 (sortRecords helper + UpdateRecord re-sort)
                                    # B2 (EqualFold in GetRecord/UpdateRecord/RemoveRecord)
                                    # B2 (lowercase substring in GetNamesWithPart)
                                    # B4 (Save() doc comment update)
2. internal/cli/change.go           # B3 (return after first prompt err)
                                    # B4 (changeMainPass uses Save)
3. internal/prmpt/prompts.go        # B5 (no-panic, return false + warn)
4. internal/strg/git.go             # B6 typo
5. internal/cli/remove.go           # B6 typo
6. tests/list_test.go               # add TestList_OrderAfterRename
7. tests/get_test.go                # add TestGet_CaseInsensitiveLookup
8. make build                       # confirms compile
9. make test                        # confirms green incl. new tests
10. git add <listed files> && git commit
```

Files touched: 5 production + 2 test files. No `go.mod`/`go.sum` churn (no new deps).

## Verification checklist

- [ ] `make build` clean
- [ ] `make test` green; total runtime within ±10% of post-Phase-1 baseline (~18 s)
- [ ] New tests `TestList_OrderAfterRename` and `TestGet_CaseInsensitiveLookup` both PASS
- [ ] `git grep -nE 'Initilizing|doesn.t exists|encypted'` returns nothing in production code
- [ ] `git grep -nE 'panic\(' internal/` shows no panics in `prmpt/`
- [ ] `git grep -n 'r.Name == name' internal/strg/storage.go` returns nothing (all replaced with EqualFold)
- [ ] `TestChange_Main` still passes — refactor is behavior-equivalent

## Commit grouping

One commit, matching Phase 1's pattern. Suggested title (under 70 chars):

```
fix: phase 2 latent bugs; case-insensitive lookup; change main via Save
```

Suggested body:

```
Phase 2 of plans/code-review-improvements.md.

- Storage.UpdateRecord re-sorts after mutation; fixes out-of-order
  listings after `change --rename`.
- GetRecord, UpdateRecord, RemoveRecord, GetNamesWithPart now use
  case-insensitive matching, matching what Exists already does. Fixes
  `psw get FOO --exact` failing on a record stored as "foo".
- changeMainPass: add missing `return` after first prompt-error println;
  re-encrypt via storage.MainPass = new + storage.Save() instead of
  EncryptStringToStorage directly. Storage.Save doc updated.
- prmpt.YesOrNo: term.MakeRaw failure no longer panics; warns and
  returns false (matches non-TTY contract). Phase 3 will rewrite this
  on bubbletea anyway.
- Typo fixes: Initilizing -> Initializing, doesn't exists -> doesn't
  exist.
- Tests: TestList_OrderAfterRename, TestGet_CaseInsensitiveLookup.
```

## Risks

- **Low.** All changes are local code edits; no format changes, no migration needed.
- **Behavioral change for case-insensitive lookup**: `psw get FOO` now succeeds where it previously failed, and `psw remove FOO` now removes a `foo` record. This is a UX improvement, not a regression. No existing test depends on case-sensitive failure.
- **`change main` refactor**: theoretically a write-path change (now goes via `Save` → which calls `ToJson` then `EncryptStringToStorage`). End result is byte-identical to before since `EncryptStringToStorage` is the same function the old code called. `TestChange_Main` exercises the full round-trip (encrypt under new pass, decrypt with new pass, fail with old pass) and will catch any regression.
- **`YesOrNo` no-panic fix is a no-op in practice**: `term.MakeRaw` essentially never fails on a real TTY; the panic was effectively unreachable. The fix is defense-in-depth, not bug-fix-with-user-impact.

## Decisions log additions for Phase 2 commit

Append to parent plan's "Decisions log" before commit:

```
- 2026-MM-DD — Phase 2: kept sort.Slice (case-sensitive) for now; switching to slices.SortFunc and any sort-case rethink is Phase 4's job.
- 2026-MM-DD — Phase 2: extracted `Storage.sortRecords()` helper instead of inlining sort calls in two places — clearer intent, two-line refactor, shared by Add and Update.
- 2026-MM-DD — Phase 2: skipped a B3 regression test — exercising the fall-through requires TTY-injected prompt errors, hard to do without mocking.
- 2026-MM-DD — Phase 2: kept YesOrNo signature as `(string) bool` — Phase 3 rewrite on bubbletea will replace the function entirely, so any signature change now is wasted churn.
```

## Hand-off

Implementer: read this top-to-bottom. The bug-by-bug section is the source of truth; the order-of-operations is just one valid sequence (any order works since the changes are independent). After commit, mark `### [ ] Phase 2` → `### [x] Phase 2` in `plans/code-review-improvements.md` and append the decisions log entries above.
