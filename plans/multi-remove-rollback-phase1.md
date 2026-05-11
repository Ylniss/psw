# Phase 1 detail — Multi-select removal (menu + CLI)

_Parent plan: `plans/multi-remove-rollback.md` (HEAD `bec2d18`)._
_Drift check: parent plan's `Last updated` commit equals current HEAD; cited files (`internal/storage/picker.go`, `internal/menu/remove.go`, `internal/cli/remove.go`, `internal/storage/storage.go`) match descriptions._

## Scope confirmation

In:
- Separate `NewPickerModelMulti(names)` constructor — single-select (`NewPickerModel`) stays the default for `get`/`change`.
- Menu `remove`: multi-pick → y/n confirm with rollback hint → save once → single commit. Decline on the y/n bounces back to the picker with selections preserved.
- CLI `remove`: variadic args; zero args → multi-picker TUI launched from `psw remove` directly (not via `psw menu`); `--exact` validates each arg.
- Integration tests for new CLI shapes.

Out:
- Picker behavior for `get`/`change` (untouched).
- CLI confirmation prompt for remove (today's behavior is silent; menu-only confirm matches plan).
- `psw rollback` (Phase 2).
- Version bump (Phase 3).

## Key design decisions

### D1 — Separate constructor; multi state lives on `PickerModel`, shared with delegate via map reference
Add `NewPickerModelMulti(names []string) PickerModel` that constructs a multi-mode picker from the start: it builds `pickerDelegate{multi: true, toggled: m.toggled}` and passes it to `list.New`. No `extras` parameter — `extras` is only used by `change` for the `main-password` keyword, which stays single-select.

`pickerDelegate` gains `multi bool` and `toggled map[string]bool`; `PickerModel` gains `multi bool`, `toggled map[string]bool`, and `selections []string`. The same `toggled` map header lives on both the model and the delegate value handed to `list.New`. Maps are reference types, so toggling via `m.toggled[name] = true` from `Update()` is visible to the next `Render()` call — no `SetDelegate` swap needed. Confirmed against bubbles v2 docs (delegate is captured by value at `list.New`; the captured value contains the shared map header).

### D2 — Space toggles, intercepted before `list.Update`
`PickerModel.Update` already intercepts `enter`/`up`/`down`/`ctrl+n,p` before the list sees them. Add a `" "` (space) case that fires only when `m.multi`. Cost: record names containing space cannot be filter-narrowed via space — psw record names don't normally contain spaces, so this is acceptable; document in a one-line comment on the case.

### D3 — Enter fall-through preserves current single-select UX
Enter behavior: if `len(m.toggled) > 0` → done with selections; else → done with cursor's item (today's path). `Selections() []string` returns the toggled set in the list's display order; `Selection() string` keeps returning the cursor item. Multi callers do `sel := p.Selections(); if len(sel) == 0 { sel = []string{p.Selection()} }`. Single callers ignore `Selections()` entirely.

### D4 — Render: marker in fixed column, alignment matches today
Single mode unchanged. Multi mode prefixes every item with `[x] ` or `[ ] `. Cursor row replaces the 2-cell left padding with `> ` (today's behavior); non-cursor row keeps 2-cell padding. Result: name column stays at the same offset whether the row is cursor or non-cursor. Concrete examples (cursor on row 1):

```
> [x] alice
  [ ] bob
  [x] charlie
```

### D5 — Help text is mode-aware
`Help() string` returns one of two consts based on `m.multi`:
- single (existing): `↑/↓ or ctrl+n/p navigate · enter select · esc cancel · type to filter`
- multi (new): `↑/↓ navigate · space toggle · enter confirm · esc cancel · type to filter`

Menu's `RemoveAction.FooterHelp()` already calls `picker.Help()` — propagation is automatic.

### D6 — CLI variadic surface (strict)
- `--exact`, ≥1 args: validate every arg against `store.Exists`; collect missing names; on any miss print `Records not found: a, b, c` (one line listing all missing names) and return `errSilentExit`. No mutation runs unless every arg matched.
- no `--exact`, 0 args: launch the multi-picker TUI directly from `psw remove` (same single-screen bubbletea program as today's single-select picker — *not* `psw menu`). On cancel: silent exit (existing `ErrPickerCancelled` → `nil, nil` translation).
- no `--exact`, 1 arg: substring filter into multi-picker (today's pre-filter, still single-match fast-path returning `[name]` without TUI).
- no `--exact`, ≥2 args: error `multiple names require --exact` + `errSilentExit`. (Rejected union-of-substrings: too magical; ambiguous when one substring matches a superset of another.)

### D7 — Commit message
- N == 1: `record removed` (unchanged from today).
- N >= 2: `removed N records` (per parent plan).

### D8 — Menu confirm step semantics
- y → run removals, save, single commit, return to action-select with the success line.
- n → return to the picker with the toggled set preserved so the user can deselect/select more, then re-confirm. Picker's `Done()`/`selections` must be cleared on the round-trip — add `(m *PickerModel) Reopen()` that resets `m.done = false` and `m.selections = nil`. Toggled set is intentionally *not* cleared.
- Esc on the y/n → same as `n` (bounce back to picker, selections preserved). Override the default `stepYesNo` behavior of treating Esc as cancel — the confirm step intercepts Esc and routes it to the same handler as a `no` answer.
- Esc on the picker → still aborts the whole remove flow with no output appended (today's behavior; unchanged).
- Confirmation question text: `"Remove these N records? You can undo any action with `psw rollback`"` with `N` pluralized via fmt (`record` for 1, `records` otherwise). Names appear in the transcript above the y/n via picker's per-selection lines.
- Transcript on bounce-back: the picker's `> name` lines that were appended when the user first hit Enter must NOT stack up on a second confirm round. Strip the picker's most recent block of `> name` lines from `a.transcript` when re-entering picking phase, then re-append on the next Enter. Implementation: track the transcript length before `stepPickerMulti` appends, restore on bounce.

### D9 — Picker transcript for multi-select
`baseAction.stepPicker` today appends one `> name` line. For multi we need one line per selection. Add a sibling helper `stepPickerMulti(p *PickerModel, msg tea.Msg) (selections []string, ready bool, cmd tea.Cmd)` that appends `> name` per selection in display order. Keeps `stepPicker` untouched for `get`/`change`.

## File-by-file changes

### `internal/storage/picker.go`
- Add to `pickerDelegate`: `multi bool`, `toggled map[string]bool`. `Render` branches on `d.multi`; in multi mode, builds `marker + name` (`[x] ` / `[ ] `) and applies the same cursor/extra style logic as today.
- Add to `PickerModel`: `multi bool`, `toggled map[string]bool`, `selections []string`.
- New top-level `NewPickerModelMulti(names []string) PickerModel` that constructs the model with `multi: true`, an allocated `toggled` map, and a delegate built from the same map header (`pickerDelegate{multi: true, toggled: toggledMap}`). No `extras` slot — `extras` is only used by `change` for `main-password` (single-select).
- `Update`: add `case " ":` (multi only) that toggles `m.toggled[name]` for the cursor's item; returns `m, nil`. Comment explicitly that this prevents space from filtering — acceptable because record names don't contain spaces. Modify `case "enter":` — if `m.multi`, populate `m.selections` from `m.toggled` in list order (walk visible items; keep those in the map). Leave `m.chosen` set to the cursor's item for the no-toggles fall-through. Set `m.done` either way.
- `(m *PickerModel) Reopen()` — pointer receiver; sets `m.done = false`, `m.selections = nil`. Used by menu remove on y/n decline. `m.toggled` is intentionally preserved.
- New const `pickerMultiHelp = "↑/↓ navigate · space toggle · enter confirm · esc cancel · type to filter"`. `Help()` returns `pickerMultiHelp` when `m.multi`, else the existing `pickerHelp`.
- New accessor `(m PickerModel) Selections() []string` returning `m.selections`.
- New top-level `GetRecordNamesInteractive(names []string) ([]string, error)` mirroring `GetRecordNameInteractive`: builds via `NewPickerModelMulti`; fast-path on `len(names) == 1` returns `[]string{names[0]}`; runs `tea.NewProgram(tuiutil.QuittingWrapper[PickerModel]{...})`; on cancel returns `nil, ErrPickerCancelled`; on done returns `Selections()` if non-empty else `[]string{Selection()}`.

Note: `NewPickerModel(names, extras)` stays untouched — used by `get`/`change` for substring-match + the `main-password` extra.

### `internal/menu/base.go`
- Add `stepPickerMulti(p *storage.PickerModel, msg tea.Msg) ([]string, bool, tea.Cmd)`. Body parallels `stepPicker` but: on `p.Done()`, computes `sels := p.Selections(); if len(sels) == 0 { sels = []string{p.Selection()} }` and appends one transcript line per `sel`.

### `internal/menu/remove.go`
- New phase enum value `removePhaseConfirming` between `removePhasePicking` and `removePhaseSaving`.
- `RemoveAction` fields: replace `recordName string` with `recordNames []string`. Add `transcriptLenBeforePick int` to support bounce-back trim.
- `updateLoading` change: `a.picker = storage.NewPickerModelMulti(names).WithoutHelp()`.
- `updatePicking`: snapshot `a.transcriptLenBeforePick = len(a.transcript)` before calling `stepPickerMulti`. On selection, store result in `a.recordNames`. Init y/n with `question := fmt.Sprintf("Remove %d %s? You can undo any action with `+"`psw rollback`"+`", len(a.recordNames), pluralize(len(a.recordNames)))` and advance to `removePhaseConfirming`. Add `pluralize(n int) string` helper colocated in remove.go (returns `"record"` / `"records"`).
- `updateConfirming` (new): drive `prompt.YesNoModel` directly (don't reuse `stepYesNo` — its Esc handler sets `cancelled`, but here Esc means decline-bounce-back, not abort). Pseudocode:
  ```go
  if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "esc") {
      return a.bounceToPicker(), nil
  }
  cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
  if a.yesNo.Cancelled() { // ctrl+c only — esc handled above
      a.cancelled = true
      return a, nil
  }
  if a.yesNo.Done() {
      a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
      if !a.yesNo.Answer() {
          return a.bounceToPicker(), nil
      }
      for _, n := range a.recordNames { a.store.RemoveRecord(n) }
      return a.toSpinner("Saving", removePhaseSaving, saveCmd(a.store, commitMsgFor(len(a.recordNames))))
  }
  return a, cmd
  ```
  `bounceToPicker()` helper: trim transcript back to `transcriptLenBeforePick` (drops the `> name` lines AND any `formatYesNoLine` that was just appended for "n"), call `a.picker.Reopen()`, set `a.phase = removePhasePicking`, return `a`. Note: appending the formatYesNoLine before the bounce is intentional — gets trimmed along with the picker lines, so the user sees a clean picker on bounce, but if they re-confirm with "y" later, the round-trip is fully replaced by the second attempt's transcript.

  Decision sub-point: should the formatYesNoLine for the "n" answer be appended at all if it's about to be trimmed? Cleaner to skip the append when `!answer`. Updated pseudocode skips it.
- `commitMsgFor(n int) string`: returns `"record removed"` for `n == 1`, `fmt.Sprintf("removed %d records", n)` otherwise.
- `updateSaving`: success line for `n == 1` keeps today's `fmt.Sprintf("Record %s successfully removed", color.InGreen(name))`; for `n >= 2` use `fmt.Sprintf("Removed %d records: %s", n, strings.Join(coloredNames, ", "))`. Branch on `len(a.recordNames)`.
- `View()`: add `removePhaseConfirming` to the yesNo branch (via `prependTranscript(a.yesNo.View(), a.transcript)`).
- `FooterHelp()`: still returns `a.picker.Help()` only during `removePhasePicking`. (Help becomes the multi help automatically since picker is in multi mode.)

### `internal/cli/helpers.go`
- New `resolveRecordNames(store *storage.Storage, args []string, exact bool) ([]string, error)`. Body per D6:
  - `--exact` with 0 args: print `--exact requires at least one record name argument`, return `errSilentExit`.
  - `--exact` with ≥1 args: walk args, collect missing into `var missing []string`. If `len(missing) > 0`: print `Records not found: <missing joined by ", ">` (each name green via `color.InGreen`), return `errSilentExit`. Else dedupe args and return.
  - non-exact + ≥2 args: print `multiple names require --exact`, return `errSilentExit`.
  - non-exact + 0 args: call `storage.GetRecordNamesInteractive(store.GetNames())`.
  - non-exact + 1 arg: call `storage.GetRecordNamesInteractive(store.GetNamesWithPart(args[0]))`.
  - Translate `storage.ErrPickerCancelled` → `nil, nil` (matches `resolveRecordName`).

### `internal/cli/remove.go`
- `Args: cobra.ArbitraryArgs` (was `MaximumNArgs(1)`).
- Update `Use`/`Short`/`Long` to document `psw remove [name...]`. New `Use` line:
  ```
  remove [name...] [flags]

  Arguments:
    name    Optional record name(s). With --exact, all listed names are removed; without --exact, opens an interactive picker (one substring filter argument allowed).
  ```
- `RunE`: call `resolveRecordNames`, dedupe (defensive — `--exact foo foo` shouldn't try to remove twice), loop `store.RemoveRecord`, single `Save()`, single `GitCommit(commitMsgFor(n))`. Print one `Record %s successfully removed` line per name (preserves existing per-name stdout format that `TestRemove_Success` asserts).

## Test plan (`tests/remove_test.go`)

Keep existing `TestRemove_Success` and `TestRemove_MissingRecord` (back-compat: single-arg `--exact` path is unchanged).

Add:
- `TestRemove_MultiExact`: add `a`, `b`, `c`; `psw remove a c --exact`; expect exit 0, `b` remains in subsequent list.
- `TestRemove_MultiExactOneMissing`: add `a`; `psw remove a missing --exact`; expect exit 1, stdout contains `Records not found: missing`, `a` still present after (atomicity: validation happens before any mutation).
- `TestRemove_MultiExactSeveralMissing`: add `a`; `psw remove a x y --exact`; expect exit 1, stdout contains `Records not found: x, y`, `a` still present.
- `TestRemove_MultiArgsRequireExact`: add `a`, `b`; `psw remove a b` (no `--exact`); expect exit 1, `multiple names require --exact`.
- `TestRemove_NoArgsListsForPicker`: shells out under a TTY-less harness — picker can't run, so this path can't be tested via `runPsw`. Skip integration coverage; rely on `picker.go` unit test below.

Picker unit test (new file `internal/storage/picker_test.go` if none exists; otherwise extend):
- Construct `NewPickerModelMulti([]string{"a","b","c"})`, drive a sequence of `tea.Msg`s (window size, space, down, space, enter), assert `Selections() == ["a","c"]` and `Done() == true`.
- Same flow with no toggles before enter: assert `Selections() == nil` and `Selection() == "a"` (cursor's first item after fresh open).
- Help text switches: `NewPickerModel(...).Help()` contains `enter select`; `NewPickerModelMulti(...).Help()` contains `space toggle`.
- Reopen preserves toggled set: toggle `a` and `c`, press enter (Done=true), call `Reopen()`, assert `Done() == false`, `Selections() == nil`, then resend enter and assert `Selections() == ["a","c"]` (toggled set survived).

Menu remove: there's currently no menu integration that drives keystrokes through the TUI (`tests/menu_test.go` only checks non-TTY error + help listing). Adding a full bubble-tea-driven test is out of phase 1 scope; rely on manual smoke-test via `psw menu` post-implementation. Note this gap explicitly in the Decisions log.

## Risk surface

| Risk | Mitigation |
|---|---|
| Multi-mode flag leaks into get/change picker | `NewPickerModelMulti` is a separate constructor; existing `NewPickerModel` callers are unmodified. Picker unit tests assert `Selections()` is empty for the single-select constructor. |
| Filtering by a name with a space breaks in multi mode | Document in a comment on the space-key case; psw names don't have spaces in practice. Could be revisited if it bites. |
| Delegate map-share not visible to list's render copy | Map is a reference type; once the same map header is in both the model field and the delegate value handed to `list.New`, mutations on `m.toggled[name]` are visible to the delegate's `Render`. Verified by docs (no `SetDelegate` needed). Picker unit test confirms behavior end-to-end. |
| Transcript bloat for very large multi-pick | Transcript already caps in menu mode (model.go-level concern), not worsened by N-line picker selections. |
| Validation skipped → partial removal | `resolveRecordNames` validates *all* `--exact` args before returning; mutation loop runs only after `store.Save()` succeeds. No partial-state failure modes (in-memory remove + single save). |

## Verification steps (post-implementation)

1. `make test` — green, including new tests.
2. `make build` — clean build of both binaries.
3. Manual menu smoke-test:
   - `psw menu` → enter password → `remove`.
   - Toggle two records with space, see `[x]` markers, see updated multi help line.
   - Press enter → see y/n confirm with both names listed in transcript and `Remove 2 records? You can undo any action with `+"`psw rollback`"+``.
   - `n` → bounces back to picker; `[x]` markers still visible on the two toggled rows; toggle a third with space; press enter → confirm now reads `Remove 3 records?`; transcript above lists three names.
   - Esc on the y/n → same as `n` (bounces back to picker, selections preserved).
   - Esc on the picker (not the y/n) → exits remove with no last-output line.
   - Confirm with `y` → see `Removed N records: a, b, c` line in last-output.
   - Re-enter remove menu, hit enter on cursor row without toggling → fast-path single-select; y/n reads `Remove 1 record?`; on `y` last-output is `Record foo successfully removed` (single-record legacy line).
4. CLI smoke-test:
   - `psw remove a c --exact` removes both (vault with a, b, c → b remains).
   - `psw remove a missing --exact` exits 1, stdout `Records not found: missing`, vault unchanged.
   - `psw remove a x y --exact` exits 1, stdout `Records not found: x, y`, `a` still present.
   - `psw remove a b` exits 1 with `multiple names require --exact`.
   - `psw remove a` (1 arg, single substring match, no `--exact`) → fast-path removes (today's behavior preserved).
   - `psw remove` (0 args, in a real terminal) → multi-picker TUI opens; toggle two with space; enter; per-name `Record %s successfully removed` lines; commit message `removed 2 records`.
5. Verify single-select callers untouched: `psw get foo` (substring) and `psw change foo` open the original single-select picker; help line still says `enter select`.

## Out-of-scope notes for Phase 2

- Phase 2 will add `extras`-supporting multi-picker only if rollback needs it (it doesn't — rollback's picker is single-select over commits, not records).
- The `Selections()` accessor and the `pickerMultiHelp` const land in this phase and are reused as-is.
