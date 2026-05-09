# psw TUI overhaul: menu persistence, animation, path move, help cleanup

_Last updated: 2026-05-09 — Phase 1 shipped (uncommitted at time of update)_

## Goal
Four bundled changes to psw: (1) make `psw menu` a persistent launcher that hosts actions as bubbletea sub-models so the menu UI stays visible and only Esc/q exits; (2) add randomized animated stars (1–3 per keypress, 1 per delete) plus a per-keypress logo color flash to the **main-password** input; (3) reorder `psw --help` commands and update the long description for accuracy (AES-256-GCM/Argon2id, new path); (4) move the data dir from `~/.psw/` to `$XDG_CONFIG_HOME/psw` (fallback `~/.config/psw`) with no migration.

## Constraints / non-goals
- Standalone CLI calls (e.g. `psw get foo`) keep their current flow: each prompt/picker spawns its own tea.NewProgram. Sub-modeling is a menu-only concern.
- Animation is restricted to **main-password** inputs (CLI prompt + menu). Record passwords (add, change), repeat-input fields (the "repeat main password" field counts as part of the same flow and IS animated), and any other text fields stay plain.
- Animation knobs are hardcoded (no pswcfg.toml additions). Defaults: add 1–3 per keystroke, remove 1 per backspace, blink 500ms after add, palette = standard 8-color terminal set.
- No migration from `~/.psw/`. Existing users move their data manually or re-init.
- No README rewrites (none exists). CLAUDE.md is updated.
- `change main`: when password rotates inside the persistent menu, the in-memory cached password is invalidated and re-prompted on the next mutation.

## Key decisions (pre-implementation)
- **Path = `$XDG_CONFIG_HOME/psw` with fallback `~/.config/psw`** — honors XDG (rejected: literal `~/.config/.psw` from initial spec — clarified to typo; rejected hardcoded `~/.config/psw` only — XDG support is cheap and standard).
- **No path migration** — user opted for hard cut. Accepts UX risk in exchange for less code.
- **Sub-model refactor for `psw menu` only** — standalone CLI keeps its dual tea.NewProgram pattern (rejected: full-tree refactor — uniform but ~3× the surface area; rejected: "loop with re-show" — visually noisy).
- **Adds-outpace-removes star rule** — keypress: add randint(1,3); backspace: remove 1; visible ≥ len(password); on len(password)==0 reset visible to 0. (Rejected: monotonic-only — accumulates; pure random — confusing.)
- **Stars blink ~500ms then settle, logo flashes ~150ms per keypress** — bounded animation, low CPU. (Rejected: continuous cycle, infinite blink.)
- **Animation knobs hardcoded** — keeps pswcfg.toml unchanged. (Rejected: TOML section, env vars.)
- **`menu` appended after `remove`** in help — visible but at end of action group. (Rejected: hidden, first.)

## Repo context a fresh Claude needs
- **Two binaries**: `psw` (CLI) and `clipclean` (clipboard wiper). `make build` produces `bin/psw`, `bin/clipclean`, and seeds `bin/pswcfg.toml` from `pswcfg-template.toml`. Top-level `VERSION` injected via ldflag.
- **Storage layout (current)**: `~/.psw/storage.psw`, `~/.psw/pswcfg.toml`. `setStoragePath` in `internal/storage/config.go:46` computes the dir; `PSW_HOME` env var overrides for tests. Tests use `t.TempDir()` and never touch the user's real path.
- **Encryption**: AES-256-GCM via Go 1.24+'s `cipher.NewGCMWithRandomNonce`; key = Argon2id (m=64MiB, t=2, p=4) with 16-byte per-vault salt; on-disk format `base64("PSW1" || salt[16] || gcm)`. `internal/storage/encryption.go`.
- **Cobra commands**: each lives in `internal/cli/<name>.go`, self-registers in `init()`. Default sort is alphabetical. To force user-specified order, flip `cobra.EnableCommandSorting = false` and centralize `AddCommand` calls in `root.go:init()` in the desired sequence — current scattered registrations follow alphabetical filename order which doesn't match the requested order.
- **rootCmd.Use** has a manual-indent hack (`internal/cli/root.go:30`). Verify `psw --help` rendering when touching.
- **Bubbletea version**: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`. AltScreen toggled via `tea.View.AltScreen = true`. `textinput` only has `EchoNormal/Password/None` — no per-char render hook. For animated stars, switch to `EchoNone` and render the visible string from model state, manually offsetting the cursor.
- **`menulayout` package** (`internal/menulayout/menulayout.go`) is a global indent/width singleton for `psw menu`'s dispatched subcommand. Persistent menu refactor (Phase 3b) likely retires it — sub-models receive layout via parent state.
- **Reusable models extracted in Phase 3a**: `inputModel` (`internal/prompt/prompts.go:38`), `yesNoModel` (same file:131), `pickerModel` (`internal/storage/picker.go:51`), `spinnerModel` (`internal/ui/spinner.go:71`). After extraction they expose constructors + `Done()`/`Result()`/`Cancelled()` accessors so the menu parent can host them. Standalone callers still wrap them in `tea.NewProgram`.
- **Menu mode password override**: `prompt.SetMainPasswordOverride` is an in-process variable (deliberately not env var — avoids `/proc/<pid>/environ` leak). Persistent-menu version invalidates after `change main` and re-prompts.
- **Tests**: integration tests under `tests/` build psw once into `t.TempDir`; each gets `PSW_HOME=t.TempDir()` with `PSW_GIT=0`. Path-agnostic via env var; safe across the path move. Menu-related tests in `tests/menu_test.go` need updates if menu's stdin/stdout contract changes.
- **CLAUDE.md** has three `~/.psw/` references and one "AES with SHA256" line — both updated in Phase 1.

## Phases

### [x] Phase 1: Path move + help cleanup (tasks 3 + 4) — shipped, see `plans/phase1-path-and-help.md`
- **Goal:** Switch the data dir to `$XDG_CONFIG_HOME/psw` (fallback `~/.config/psw`); fix help command order to `get, add, change, remove, menu, log, version, completion, help`; correct rootCmd Long description (AES-256-GCM + Argon2id + new path).
- **Scope:**
  - `internal/storage/config.go`: rewrite `setStoragePath` to consult `XDG_CONFIG_HOME` then `~/.config/psw`. `PSW_HOME` still wins.
  - `internal/cli/root.go`: set `cobra.EnableCommandSorting = false`. Centralize `AddCommand` calls in `root.go:init()` in user-specified order; remove duplicate `rootCmd.AddCommand(...)` lines from each command file's `init()`.
  - `internal/cli/root.go`: fix `Use:` if it renders awkwardly. Update `Long`: replace "AES encryption with SHA256" with "AES-256-GCM with Argon2id key derivation"; replace "directory ~/.psw" with the new path description.
  - `CLAUDE.md`: replace `~/.psw/` (six occurrences) with the new path description; correct any encryption text.
- **Done when:** `make build && make test` passes; `bin/psw` reads/writes the new dir; `psw --help` lists commands in the requested order with `menu` appended after `remove`; long description references AES-256-GCM/Argon2id and the new path.
- **Risk:** low
- **Depends on:** none

### [ ] Phase 2: Main-password animation (task 2)
- **Goal:** Each keystroke in a main-password input adds 1–3 colored stars; new stars blink for ~500ms then settle to a static palette color; backspace removes 1; visible never drops below `len(password)`; on empty input the visible string resets. While `psw menu` is on the action-select phase, each keypress flashes the PSW logo a random palette color for ~150ms.
- **Scope:**
  - New `internal/prompt/stars.go`: `starState` type with `add(n)/remove(n)/visible() string` (per-star color + blink-until-time), tick command. Hardcoded palette (8 standard terminal colors), add range [1,3], blink duration 500ms.
  - `internal/prompt/prompts.go`: opt-in flag on `inputModel` (e.g. `mainPassword bool`); when set, switch echo to `EchoNone`, hold a `starState`, render visible stars from state, advance cursor X to end of visible string. Wire into `promptForMainPassword` for both initial and "Repeat main password" inputs.
  - `internal/cli/menu.go`: same starState integration in `menuModel.passwordInput` rendering. Add per-keypress logo flash on phase=phaseSelectAction (random palette color for ~150ms via tick).
  - Tests: cover that wrong main password still errors correctly (regression guard for the input rewiring). Skip pixel-level animation tests — too brittle.
- **Done when:** main-password prompts (CLI + menu) show variable-count colored stars with blink-then-settle behavior; empty input shows empty stars; backspace shrinks visible by 1; `psw menu` logo flashes on each keypress in action-select; record-password and other inputs still render plain.
- **Risk:** medium — bubbletea cursor + render timing is fiddly; tick interval / key handling interaction needs attention.
- **Depends on:** none (Phase 1 not strictly required, but recommended order)

### [ ] Phase 3a: Extract reusable models (no behavior change)
- **Goal:** Convert `inputModel`, `yesNoModel`, `pickerModel`, `spinnerModel` into embeddable models with public constructors and `Done()`/`Result()`/`Cancelled()` accessors. Standalone callers (`runInput`, `YesOrNo`, `GetRecordNameInteractive`, `WithSpinner`) keep wrapping them in `tea.NewProgram`. Public API unchanged from the user's view.
- **Scope:**
  - Export model types from their packages (`prompt.InputModel`, `prompt.YesNoModel`, `storage.PickerModel`, `ui.SpinnerModel`) or move to a shared `internal/tuimodels/` package.
  - Standalone wrappers (`runInput` etc.) call the exported constructor and feed messages through.
  - Add unit tests for each model: synthesize key messages, assert state transitions and accessors.
- **Done when:** `make test` passes; standalone CLI flows behave identically; new exports are importable from `internal/cli` for use in Phase 3b.
- **Risk:** low-medium — touches several files but no logic change.
- **Depends on:** none (independent of Phase 1, 2)

### [ ] Phase 3b: Persistent menu (task 1)
- **Goal:** `psw menu` becomes one persistent tea.Program. User enters main password once at startup; thereafter the menu shows action buttons and an output area. Selecting an action runs an embedded sub-model under the menu; output renders in the menu's view. After completion the user returns to action-select. Only Esc/q exits psw. Past output stays visible.
- **Scope:**
  - New action sub-models in `internal/cli/menu/` (or similar): `getAction`, `addAction`, `changeAction`, `removeAction`. Each is a state machine hosting `PickerModel` + `InputModel` + `YesNoModel` instances and producing output strings into a result struct. Logic mirrors current RunE bodies, but Println→model state.
  - Refactor `menuModel`: phase enum extended (`phaseRunningAction`, possibly `phaseDisplayingResult`). `Update` routes messages to `m.activeAction.Update(msg)` while running. `View` composes header + buttons + (active sub-view OR last-output history).
  - Output stack: a slice of past output blocks rendered in the menu's view (above buttons, per spec — confirm in detail-phase planning).
  - Password lifecycle: cache main password in `menuModel`; after `change main` invalidate and re-prompt before next mutation. Retire `prompt.SetMainPasswordOverride` if no other caller remains.
  - Standalone CLI calls untouched: `psw get foo` directly still uses the existing dispatch path.
  - Drop `menulayout` package (or keep for standalone CLI only) — composed sub-models inside one program don't need a global singleton.
  - Tests: rewrite `tests/menu_test.go` to drive the persistent loop. Cover pick→run→return→pick→quit; cover `change main` invalidating cached password; cover Esc inside picker returning to menu (not exiting psw).
  - clipclean: `psw menu`-mode `get` still spawns it correctly (foreground process stays alive while clipclean runs in background; that's already the case but verify).
- **Done when:** `psw menu` runs an action and remains alive; user can run multiple actions back-to-back without restarting; Esc/q at action-select exits cleanly; Esc inside a sub-model returns to action-select; all integration tests pass; clipclean spawns correctly from menu-mode `get`.
- **Risk:** high — biggest change in the bundle.
- **Depends on:** Phase 3a

## Decisions log (during implementation)
_Append-only._

- **Phase 1, 2026-05-09**: Used `os.UserConfigDir()` for cross-platform default path (handles Linux XDG, Windows `%AppData%`, macOS `Library/Application Support` in one stdlib call). Spec said `$XDG_CONFIG_HOME` only — generalized to cover Windows on user request.
- **Phase 1, 2026-05-09**: Resolved the "completion vs help order" punt. Called `rootCmd.InitDefaultCompletionCmd()` then `InitDefaultHelpCmd()` in `init()` to force the spec order (cobra otherwise registers help first lazily).
- **Phase 1, 2026-05-09**: Replaced `Use:` multi-line hack with `Use: "psw"` and moved the no-args hint into `Long`. Cleaner `Usage:` block.

## Open questions
- Output stack layout details (above vs below buttons, max retained entries, scroll behavior) — defer to Phase 3b detail.

## Hand-off
To detail a phase, start a fresh context and ask:
> Prepare a detailed plan for phase N from `plans/menu-persistence-and-cleanup.md`.

**Before writing phase detail, verify the plan is not stale.** Compare the **Last updated** commit to current `HEAD`; read the files cited in **Repo context** to confirm they still exist and behave as described. If anything has drifted, surface the drift and update the plan before producing detail.
