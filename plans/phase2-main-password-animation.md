# Phase 2 detail: main-password animation

_Parent: `plans/menu-persistence-and-cleanup.md` (Phase 2)_
_Last updated: 2026-05-09 — shipped (uncommitted at time of update)_

## Status: shipped

### Divergences from plan (settled during user testing)
- **Star distribution simplified to 1–2 (50/50) per char.** Plan had `{-1: 30%, +1: 20%, +2: 30%, +3: 20%}` with shrinks-on-add for "dynamism"; user tested it, preferred a simpler always-grow rule. Mean is +1.5 per char.
- **All stars from one keystroke share one color.** Plan had per-star random colors (each `Add` rolled independently); user wanted unison so a single keystroke reads as one visual event. Implemented via `addWithColor(n, c)` called once per `ApplyKeystrokeAdd`.
- **Header flash moved to password phase only.** Plan put the flash on action-select keypresses; user inverted: header is calm during navigation, flashes only while typing the password. `updateSelectAction` no longer touches `logoFlash*`.
- **No-repeat-color rule for both stars and header.** Per-keystroke color picker tracks the previous index (`lastStarIdx`, `lastHeaderIdx`) and excludes it via `pickDistinctIdx`, so consecutive keystrokes never blink the same color.
- **Cursor X under EchoNone is set absolutely, not additively.** `textinput.Cursor().Position.X` reports `len(value)` even when no chars are rendered, so the prior `+= prefixWidth + indent + stars.Len()` double-counted by `len(value)`. Fixed: when `animateStars`, X = `prefixWidth + indent + stars.Len()` (absolute).

## Goal
Each keystroke in a **main-password** input adds 1–3 colored stars on average, but with a 30% chance per keystroke of removing 1 instead — so the visible string statistically grows while occasionally dipping (more dynamic than monotonic). New stars appear in a random palette color, hold that color for ~500ms, then settle to the **default terminal foreground** (so settled stars look like a plain `*` mask). Stars **never become invisible** — the animation is a color shift, not a visibility toggle. Backspace removes 1 star deterministically. Emptying the input resets visible stars to 0. While `psw menu` is on the action-select phase, each keypress shifts the PSW header to a random palette color (excluding default cyan, so the change is always visible) for ~250ms, then back. Record-password and other inputs stay plain (`textinput.EchoPassword`).

## Drift check vs parent plan
Verified at commit 949777d (`develop`):
- ✅ `internal/prompt/prompts.go` — private `inputModel` (`prompts.go:38`); `EchoMode` set in `newInputModel` (`prompts.go:51`).
- ✅ `internal/cli/menu.go` — `phaseSelectAction`/`phaseEnterPassword` (`menu.go:113-115`); `passwordInput` uses `textinput.EchoPassword` (`menu.go:132`).
- ✅ `promptForMainPassword` calls `runInput` for both initial and "Repeat main password" (`prompts.go:240,247`) — both should be animated.
- ✅ Bubbletea v2 `tea.Tick(d, fn)` confirmed via vendored API; not yet used in this repo.
- ✅ `lipgloss.Color` in lipgloss v2 is a **function** returning `image/color.Color`, not a type. Stored colors must be typed `image/color.Color`.
- ✅ `tests/menu_test.go` (21 lines) and `tests/prompt_test.go` (19 lines) don't assert visual output — safe across this phase.

No drift on substance. Plan can proceed.

## Lessons from prior implementation attempt (reverted, uncommitted)
A prior implementation (this conversation, then reverted at user request) shipped close to spec but two pieces felt wrong in user testing:
1. **Stars went visibility-toggle (` ` ↔ `*`) during blink.** User wants stars *always visible*, only the **color** changes.
2. **Header color flash (random palette color for 150ms) did not register visually.** Likely causes: (a) the random pool included default cyan, so ~14% of flashes were no-ops; (b) 150ms is short and a single redraw per tick (100ms) gave at most one in-flash frame on slow keypresses; (c) the redraw cadence may not match user expectation. New plan addresses all three.

These lessons are encoded in §"Design decisions". Don't re-derive — just follow.

## Design decisions (locked in)

1. **Stars are always visible. The "blink" is a color flash only.**
   - During each star's 500ms blink window: render `*` in its assigned random palette color (one color per star, picked at Add).
   - After the 500ms window: render `*` in the **default terminal foreground** (no color override — `lipgloss.NewStyle().Render("*")` or just plain `"*"`).
   - At no point is the cell rendered as a space. Settled stars look identical to the old `EchoPassword` `*`.

2. **Per-keystroke star delta is randomized, biased toward growth, but allows shrinks.**
   - When `len(value)` increases by `d` chars (typing or paste): for **each** of the `d` added chars, sample one outcome from the distribution below and apply.
     - `-1` (remove 1): 30%
     - `+1`: 20%
     - `+2`: 30%
     - `+3`: 20%
     - Mean = +1.0. So `n` keystrokes add ~`n` stars on average, with ~30% per-stroke shrinks adding visible dynamism.
   - When `len(value)` decreases (backspace, ctrl-w): remove `|delta_chars|` stars deterministically (1 per backspace; more for word-delete). No randomization.
   - When `len(value) == 0`: reset visible stars to 0.
   - Floor: visible never goes below 0 (clamp Remove). **Drop the prior `visible ≥ len(password)` invariant** — it conflicts with random shrinks on add and the user has accepted that the visible count diverges from the actual length.

3. **Per-star color picked at Add, held during blink, then dropped.**
   - Each `star` carries a `flashColor color.Color` and a `blinkUntil time.Time`.
   - During blink: render with `Foreground(flashColor)`.
   - After blink: render with no foreground override (default terminal color).
   - Star palette = `["1","2","3","4","5","6","7"]` (skip black; cyan IS allowed for stars since they pop against the prompt label, only the header excludes cyan).

4. **Logo flash on action-select keypress: 250ms color shift, random color from a cyan-excluded palette.**
   - Header palette = `["1","2","3","4","5","7"]` (no `"6"` cyan; that's the default header color).
   - On every keypress (including arrow/h/l navigation; including unrecognized keys; **except** quit keys ctrl+c/esc/q which exit before flash), set `m.logoFlashColor = randomFromHeaderPalette()` and `m.logoFlashUntil = time.Now() + 250ms`.
   - Render header with the flash color while `time.Now().Before(m.logoFlashUntil)`; otherwise default cyan.
   - 250ms instead of 150ms so the flash spans at least 2 ticks (at 100ms tick interval) — guarantees the user sees at least one flashed frame and one settled frame.

5. **Tick scheduling: 100ms `tea.Tick`, on demand.**
   - Single shared tick across stars + header (same `prompt.StarTickMsg`/`prompt.StarTick()`).
   - Active predicate: `stars.Active() || time.Now().Before(m.logoFlashUntil)`.
   - Update returns the next tick cmd while active; returns no cmd otherwise (clears `ticking` flag). Each `Add` or new flash kicks a fresh tick if `!ticking`.

6. **`StarState` lives in `internal/prompt`, exported. Type for stored colors is `image/color.Color`.**
   - `lipgloss.Color()` in v2 returns `image/color.Color` — store that.
   - `StarState` is a value type with internal `*rand.Rand` (lazy-initialized so the zero value is usable).

7. **`runInput` gains a third bool `animateStars`.** Internal API.
   - `promptForMainPassword` passes `true` for both initial and repeat prompts.
   - `PromptForName` / `PromptForRecordPassword` pass `false`.
   - `inputModel` with `animateStars=true` switches to `textinput.EchoNone` and renders stars from its own `StarState`.

8. **Cursor positioning under `EchoNone`.** `inputModel.View()` already adds `prefixWidth + indent` to the cursor X. Add `m.stars.Len()` so the caret sits after the rendered stars. Same in `menuModel`.

9. **Scrollback re-emit stays plain `strings.Repeat("*", lipgloss.Width(val))`.** Animation is interactive only; the line dumped to scrollback after the TUI exits stays masked as before. No color, no animation.

## Subtasks

### 1. New file: `internal/prompt/stars.go`
- Imports: `image/color`, `math/rand`, `strings`, `time`, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`.
- `var starPalette = []color.Color{lipgloss.Color("1"), …, lipgloss.Color("7")}` — full palette for stars (excluding "0" black).
- `var headerFlashPalette = []color.Color{lipgloss.Color("1"), "2", "3", "4", "5", "7"}` — excludes cyan ("6") so the header flash always differs from default. (Either define this here or in `menu.go`; recommend `stars.go` to keep all palettes co-located.)
- `type star struct { flashColor color.Color; blinkUntil time.Time }`.
- `type StarState struct { stars []star; rng *rand.Rand }` — `rng` lazy-initialized via internal `ensureRNG()`.
- Methods:
  - `Add(n int)` — appends `n` stars, each with a random `starPalette` color and `blinkUntil = time.Now().Add(500ms)`.
  - `Remove(n int)` — pops up to `n` from the tail, clamped to 0.
  - `Reset()` — empties the slice.
  - `Len() int`.
  - `Active() bool` — true iff any star's `blinkUntil` is in the future.
  - `View() string` — for each star: if `time.Now().Before(blinkUntil)`, render `lipgloss.NewStyle().Foreground(flashColor).Render("*")`; else render plain `"*"`. Concatenate.
  - `RandomHeaderColor() color.Color` — picks from `headerFlashPalette`. Used by menuModel.
  - `ApplyKeystrokeAdd(deltaChars int)` — for each of `deltaChars` chars, roll the distribution {-1: 30%, +1: 20%, +2: 30%, +3: 20%} and apply (Add or Remove). Returns `true` if state changed.
  - `ApplyKeystrokeRemove(deltaChars int)` — `Remove(deltaChars)` deterministic. Returns `true` if state changed.
  - `ApplyEmpty()` — `Reset()`. Returns `true` if was non-empty.
- `type StarTickMsg time.Time`; `func StarTick() tea.Cmd { return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return StarTickMsg(t) }) }`.
- Unit tests in `stars_test.go`:
  - `Add` / `Remove` / `Reset` / `Len` basic behavior.
  - `Active` true after `Add`.
  - `ApplyKeystrokeAdd` over many trials: mean delta ≈ +1.0 ± 0.2 (sample size ~1000 to keep test fast and not flaky).
  - `ApplyKeystrokeAdd` produces all four outcomes (-1/+1/+2/+3) over many trials.
  - Skip blink-timing tests — depends on `time.Now()`; brittle.

### 2. Wire into `inputModel` (`internal/prompt/prompts.go`)
- Extend `inputModel` struct: add `animateStars bool`, `stars StarState`, `ticking bool`.
- Update `newInputModel` signature: `newInputModel(label string, password, animateStars bool) inputModel`.
  - When `animateStars`: `textInput.EchoMode = textinput.EchoNone`; initialize `stars = NewStarState()`.
  - When `animateStars=false` and `password=true`: keep `EchoPassword` (existing behavior).
- In `Update`:
  - On `tea.KeyPressMsg` with ctrl+c/esc/enter: existing behavior (cancel/quit).
  - On `StarTickMsg`: if `animateStars && stars.Active()`, return another `StarTick()`; else clear `ticking`, no cmd.
  - Otherwise (regular text input): record `prevLen = len(m.input.Value())`, delegate to `m.input.Update(msg)`, compute `newLen`. If `!animateStars`, return as today. Else:
    - If `newLen == 0`: `m.stars.ApplyEmpty()`.
    - Else if `newLen > prevLen`: `m.stars.ApplyKeystrokeAdd(newLen - prevLen)`.
    - Else if `newLen < prevLen`: `m.stars.ApplyKeystrokeRemove(prevLen - newLen)`.
  - If `stars.Active() && !m.ticking`: set `m.ticking = true`, return `tea.Batch(textCmd, StarTick())`. Else return `m, textCmd`.
- In `View`:
  - If `animateStars`, replace `m.input.View()` with `m.stars.View()` in the body.
  - Cursor X offset adds `m.stars.Len()` when animating.

### 3. Update callers (`internal/prompt/prompts.go`)
- Add third arg through `runInput`: `func runInput(label string, password, animateStars bool) (string, error)`.
- `PromptForName` → `runInput(promptText, false, false)`.
- `PromptForRecordPassword` → both calls `runInput(..., true, false)`.
- `promptForMainPassword` → both calls `runInput(..., true, true)` (animated).
- `runInput` post-tea scrollback re-emit: keep `strings.Repeat("*", lipgloss.Width(val))` for both `password` and `animateStars` cases (animation is interactive only).

### 4. Wire into `menuModel.passwordInput` (`internal/cli/menu.go`)
- Imports: add `image/color` (as `imgcolor` or similar to disambiguate from `github.com/TwiN/go-color`) and `time`.
- `menuModel` gains: `stars prompt.StarState`, `ticking bool`, `logoFlashColor imgcolor.Color`, `logoFlashUntil time.Time`.
- `newMenuModel`: change `input.EchoMode = textinput.EchoPassword` → `textinput.EchoNone`. Initialize `stars: prompt.NewStarState()`.
- `Update` top level: handle `prompt.StarTickMsg` — if `stars.Active() || time.Now().Before(m.logoFlashUntil)`, return `prompt.StarTick()`; else clear `ticking`, no cmd.
- `updateEnterPassword`: same delta-tracking pattern as `inputModel`. After delegating to `m.passwordInput.Update(msg)`, compute newLen vs prevLen and call `ApplyEmpty` / `ApplyKeystrokeAdd` / `ApplyKeystrokeRemove`. Kick `prompt.StarTick()` if `stars.Active() && !m.ticking`.
- `renderEnterPassword`: replace `m.passwordInput.View()` with `m.stars.View()`.

### 5. Logo flash on `phaseSelectAction` (`internal/cli/menu.go`)
- Drop the `var renderedHeader = menuHeaderStyle.Render(pswHeader)` global. Replace with a function `renderHeader(c imgcolor.Color) string { return lipgloss.NewStyle().Foreground(c).Render(pswHeader) }`. Keep one cached default-color string (`renderedHeader = renderHeader(lipgloss.Color("6"))`) for the post-menu display only (after `tea.NewProgram.Run()` returns).
- `updateSelectAction`: at the **top**, before any switch case mutates `cursor`/`phase`:
  ```go
  switch msg.String() {
  case "ctrl+c", "esc", "q":
      m.cancelled = true
      return m, tea.Quit
  }
  m.logoFlashColor = m.stars.RandomHeaderColor()
  m.logoFlashUntil = time.Now().Add(250 * time.Millisecond)
  // then process navigation keys
  ```
  Kick `prompt.StarTick()` if `!m.ticking`. Append to any nav-action cmd via `tea.Batch`.
- `View`: when `time.Now().Before(m.logoFlashUntil)`, render with `m.logoFlashColor`; else with `lipgloss.Color("6")`.

### 6. Tests
- `internal/prompt/stars_test.go` — see subtask 1.
- Existing integration tests (`tests/`) stay green. The wrong-password regression path (`tests/get_test.go:57-61`, `tests/change_test.go:118-125`) doesn't touch interactive rendering — passes via env var `PSW_MAIN_PASSWORD`. Confirms the input rewiring didn't break the value plumbing.
- **Do not** add brittle pixel-level animation tests.

### 7. CLAUDE.md note
- One sentence in the "Menu mode" section: main-password inputs render colored stars whose color flashes briefly per keystroke and settles to default terminal color; logo flashes a non-cyan random color on each action-select keypress; scrollback re-emit stays plain `*`. Don't expand.

## Manual smoke tests (must pass)
After implementation, before declaring done, run these by hand. Type-check and integration tests don't catch animation correctness.

1. **Fresh vault flow:** `PSW_HOME=/tmp/pswtest bin/psw` (no existing vault). Type a password. Confirm:
   - Each char produces a `*`-like star.
   - Star color flashes for ~500ms then settles to default terminal color.
   - Stars never disappear/reappear — only the color changes.
   - Typing 10 chars yields ~10 visible stars on average (sometimes 8, sometimes 12).
   - Backspace shrinks visible by 1 each press.
   - Clearing the line empties the stars.
2. **Menu logo flash:** `bin/psw menu`. Press left/right repeatedly. Confirm:
   - The PSW logo briefly changes color on every keypress.
   - The flash is visibly different from the default cyan (never blends in).
   - No press produces zero visible change.
3. **Standalone inputs unaffected:** `bin/psw add foo`. Type a password. Confirm record-password input renders plain masked `*` (no animation, no color).

## Risks
- **Bubbletea v2 redraw cadence:** if View doesn't redraw between ticks, the flash may render only once. Verify in subtask 1's POC that returning a `tea.Tick` cmd actually triggers multiple `View` calls. If not, increase tick frequency or investigate `tea.NewProgram` options.
- **Cursor placement under `EchoNone`:** `m.input.Cursor()` may return nil. If so, fabricate one (manually-positioned). Verify with `bin/psw` (initial password prompt on a fresh vault) before continuing.
- **Header re-render on each tick** is slightly more expensive than the cached version. ASCII block + 8-color render is microseconds; non-issue. Mention it but don't optimize prematurely.
- **`image/color` vs `github.com/TwiN/go-color` import collision** — use a named import (`imgcolor "image/color"`) in `menu.go` to disambiguate.

## Open questions
None blocking. Smoke-test results will catch any final UX surprises.

## Hand-off
Implementer order:
1. Subtask 1 (`stars.go` + unit test) — proves `StarState` math (especially `ApplyKeystrokeAdd` distribution).
2. Subtask 2 + 3 (inputModel wiring) — `bin/psw` initial-password prompt animates.
3. Subtask 4 + 5 (menu wiring) — `bin/psw menu` animates password + header.
4. Subtask 6 (tests).
5. Subtask 7 (CLAUDE.md).
6. Run §"Manual smoke tests". **Don't declare done before these pass.**

After each subtask: `make build && make test`. Report ANY visual surprise to the user before assuming spec is met.
