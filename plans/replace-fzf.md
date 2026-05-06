# Replace external `fzf` with in-process `bubbles/list` picker

Date: 2026-05-06
Status: planned

## Motivation

`psw get`/`change`/`remove` shell out to `fzf` for record selection (`internal/strg/storage.go:146`, `GetRecordNameWithFzf`). That makes `fzf` a hard runtime requirement — users must install it separately, and the install path differs per OS/distro. We want the picker to be in-process so the binary is self-contained.

The `charmbracelet/bubbletea` + `bubbles` + `lipgloss` stack is already in `go.sum` as an indirect dep (pulled via `github.com/cqroot/prompt`, which `internal/prmpt/` uses). `bubbles/list` is batteries-included with built-in fuzzy filtering — same algorithm fzf-users expect (`sahilm/fuzzy`). Reusing it is the smallest dep delta and matches the surrounding TUI style.

## Scope

- Replace `GetRecordNameWithFzf` with an in-process picker; preserve the single-candidate short-circuit (intentional per CLAUDE.md, avoids "confirming a forced choice").
- Update three call sites (`internal/cli/resolve.go`) and three flag descriptions (`get.go`/`change.go`/`remove.go`) so user-facing wording no longer says "fzf".
- Update CLAUDE.md.
- Tests should pass unchanged — integration tests resolve names via `--exact` and never hit the picker.

Out of scope: any change to `--exact` semantics, record model, scripting env vars, or non-interactive paths.

## Target API

Rename and re-implement in `internal/strg/storage.go`:

```go
// GetRecordNameInteractive shows an in-process fuzzy picker over names
// and returns the chosen one. Single-candidate short-circuits without UI.
// Empty input → ("", nil) with no UI shown.
func GetRecordNameInteractive(names []string) (string, error)
```

Behavior contract:
- `len(names) == 0` → return `("", nil)`. Caller already handles the empty-storage path; do not pop UI for nothing.
- `len(names) == 1` → return `names[0], nil` without launching the TUI (preserves current short-circuit).
- Otherwise: launch a full-screen alt-screen TUI; user types to filter (fuzzy), arrow keys navigate, Enter selects, Esc/Ctrl-C cancels.
- Cancel → return `("", errCancelled)` (new sentinel in `internal/strg/`). Callers in `resolve.go` should treat cancel as "exit 0, no error" (mirrors current fzf-on-Esc behaviour: `cmd.Run()` returns exit-1 which surfaces as a vague error today — this is a small UX upgrade).

Drop the old name everywhere. There are no external importers (single module, `internal/`).

## Implementation sketch

New file: `internal/strg/picker.go`

```go
package strg

import (
    "errors"
    "io"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/list"
    "github.com/charmbracelet/lipgloss"
)

var ErrPickerCancelled = errors.New("selection cancelled")

type pickerItem string
func (i pickerItem) FilterValue() string { return string(i) }

// Minimal one-line delegate (no description, no spacing).
type pickerDelegate struct{}
func (pickerDelegate) Height() int                             { return 1 }
func (pickerDelegate) Spacing() int                            { return 0 }
func (pickerDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (pickerDelegate) Render(w io.Writer, m list.Model, index int, it list.Item) {
    name := string(it.(pickerItem))
    style := lipgloss.NewStyle().PaddingLeft(2)
    if index == m.Index() {
        style = style.Foreground(lipgloss.Color("170")).Bold(true)
        name = "> " + name
    }
    io.WriteString(w, style.Render(name))
}

type pickerModel struct {
    list     list.Model
    chosen   string
    quitting bool
}

func (m pickerModel) Init() tea.Cmd {
    // Start in filter mode so typing immediately filters (fzf-like UX).
    // bubbles/list filter is bound to "/"; send a synthetic "/" on init.
    return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} }
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.list.SetSize(msg.Width, msg.Height)
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c":
            m.quitting = true
            return m, tea.Quit
        case "enter":
            // Only accept when filter UI is not in "type-to-filter" entry state,
            // OR when an item is highlighted. bubbles/list handles enter inside
            // filter mode as "apply filter and exit filter" — we want enter to
            // also pick the highlighted item. Easiest: peek at FilterState; if
            // the user is mid-filter, let list handle it; otherwise pick.
            if m.list.FilterState() != list.Filtering {
                if it, ok := m.list.SelectedItem().(pickerItem); ok {
                    m.chosen = string(it)
                    return m, tea.Quit
                }
            }
        }
    }
    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}

func (m pickerModel) View() string {
    if m.quitting {
        return ""
    }
    return m.list.View()
}

func GetRecordNameInteractive(names []string) (string, error) {
    if len(names) == 0 {
        return "", nil
    }
    if len(names) == 1 {
        return names[0], nil
    }

    items := make([]list.Item, len(names))
    for i, n := range names {
        items[i] = pickerItem(n)
    }
    l := list.New(items, pickerDelegate{}, 0, 0) // sized via WindowSizeMsg
    l.Title = "Select a record"
    l.SetShowStatusBar(false)
    l.SetShowHelp(true)
    l.SetFilteringEnabled(true)

    p := tea.NewProgram(pickerModel{list: l}, tea.WithAltScreen())
    final, err := p.Run()
    if err != nil {
        return "", err
    }
    fm := final.(pickerModel)
    if fm.chosen == "" {
        return "", ErrPickerCancelled
    }
    return fm.chosen, nil
}
```

Notes for the implementer:
- The `Init()` trick (synthetic `/` keypress) is the cleanest way to start in filter mode in bubbles v1. If it misbehaves on first paint, fall back to `l.SetFilterState(list.Filtering)` if available in the installed version, or accept the `/`-to-filter UX.
- Verify the go.mod versions: project is on `bubbles v1.0.0` / `bubbletea v1.3.10`. Use `github.com/charmbracelet/...` import paths (NOT the `charm.land/...` v2 paths shown in newer docs).
- Drop `bytes`, `os/exec`, `strings` imports from `internal/strg/storage.go` if no longer needed after removing `GetRecordNameWithFzf` (run `go vet` / `goimports`).

## Call-site updates

`internal/strg/storage.go:146` → delete `GetRecordNameWithFzf` entirely.

`internal/cli/resolve.go`:
```go
if len(args) == 0 {
    return strg.GetRecordNameInteractive(storage.GetNames())
}
return strg.GetRecordNameInteractive(storage.GetNamesWithPart(args[0]))
```
Plus: handle `ErrPickerCancelled` in callers (`get.go`, `change.go`, `remove.go`) — when name is empty AND err is `ErrPickerCancelled`, return nil (silent exit). Today these paths already handle empty name as "user gave up", so it's mostly checking err type. Search for existing call sites of `resolveRecordName` and confirm.

## Wording sweep

Drop "fzf" from user-visible strings. These flag descriptions exist today:

- `internal/cli/get.go:23` — `"exact name match; skip fzf and substring search"` → `"exact name match; skip interactive picker and substring search"`
- `internal/cli/get.go:32` — long help: `"prompted to select a record with fzf"` → `"prompted to select a record interactively"`
- `internal/cli/change.go:23` — same pattern as get.go:23
- `internal/cli/remove.go:16` — same pattern

`internal/cli/resolve.go:14` — comment `"existing substring + fzf selection flow"` → `"existing substring + interactive selection flow"`.

## go.mod / go.sum

Promote three indirects to direct requires (they already resolve):
- `github.com/charmbracelet/bubbles v1.0.0`
- `github.com/charmbracelet/bubbletea v1.3.10`
- `github.com/charmbracelet/lipgloss v1.1.0`

Run `go mod tidy` after the edit; it should reorganize the require blocks but not change versions or `go.sum`.

## Nix

`nix/flake.nix` uses `gomod2nix` — re-run `gomod2nix generate` (or whatever the project's regen step is) to refresh `gomod2nix.toml` with the now-direct requires. `vendorHash` should be unchanged since no new modules enter the closure (everything was already indirect). Verify by attempting a flake build; if hash drifts, update it.

## Docs

`CLAUDE.md`:
- Replace the `### fzf record selection` heading + paragraph (currently mentions `GetRecordNameWithFzf`, "`fzf` must be on `PATH`") with a paragraph describing the in-process picker, naming `GetRecordNameInteractive`, keeping the single-candidate short-circuit note.
- Search for any other `fzf` mentions in CLAUDE.md and update.

README (if it mentions `fzf` as a runtime dep) — grep and update.

Makefile / install steps: no change (we no longer need `fzf` on PATH; `make install` still installs `psw` + `clipclean`).

## Verification

1. `grep -rn "fzf" .` — should match only this plan, the changelog/git history, and possibly CI artifacts. Zero matches in `*.go` and `CLAUDE.md`/`README*`.
2. `go build ./...`
3. `make build` — produces `bin/psw`, `bin/clipclean` unchanged.
4. `make test` — integration tests use `--exact`, so they bypass the picker. All should pass without modification.
5. Manual smoke test:
   - `bin/psw add foo` (interactive); `bin/psw add bar`; `bin/psw add baz`.
   - `bin/psw get` with no arg → picker pops, type "ba" filters to bar/baz, Enter selects.
   - `bin/psw get qu` (substring miss) → picker shows nothing or empty list (matches today's behaviour: `GetNamesWithPart` returns `[]`, picker now returns `("", nil)`).
   - Single-record case: add a fresh vault with one record, run `bin/psw get` → must NOT pop the picker.
   - Esc/Ctrl-C in picker → exits cleanly, no error message, exit 0.
6. `bin/psw --version` echoes VERSION-file value (sanity).

## Risks watched

- **Filter-on-startup hack**: the synthetic `/` keypress in `Init()` is the documented-friendly way but may not feel instant. If first render shows the unfiltered list for one frame, that's acceptable; if it's actually buggy (e.g. swallows the first real keypress), fall back to non-filter-on-startup and tell the user to press `/`. Worst case is small UX delta from fzf, not correctness.
- **Terminal teardown on Ctrl-C**: bubbletea handles its own raw-mode cleanup; verify the terminal isn't left in a weird state after an aborted picker (common bubbletea gotcha if `tea.Quit` isn't returned). The `quitting` flag + returning `tea.Quit` covers this.
- **Tiny terminals**: bubbles/list copes with `WindowSizeMsg` resizes. Initial size of 0×0 is fine because `WindowSizeMsg` fires on startup. Verify on an ~20-line terminal.
- **Non-TTY stdin**: today, `fzf` errors out clearly when run without a TTY. bubbletea will likewise fail to enter raw mode; ensure the resulting error is surfaced (not silently swallowed). The integration tests don't exercise this because they use `--exact`. Acceptable to leave as-is unless we see complaints.
- **gomod2nix.toml drift**: regenerate; if `vendorHash` shifts, update it. Reference: `plans/restructure.md` notes the same dep-tracking caveat.

## Out-of-band cleanup (optional, do NOT bundle)

- The `errCancelled` / `errExit` patterns in `internal/cli/root.go` could absorb the new `ErrPickerCancelled` so callers don't need bespoke checks. If the diff stays clean without it, skip.
