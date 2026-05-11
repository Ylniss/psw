package storage

import (
	"errors"
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ylniss/psw/internal/tuiutil"
)

// Picker visual / behavior knobs.
const (
	pickerTitle     = "Select a record"
	pickerHelp      = "↑/↓ or ctrl+n/p navigate · enter select\nesc cancel · type to filter"
	pickerMultiHelp = "↑/↓ navigate · space toggle · enter confirm\nesc cancel · type to filter"
	selectedPrefix  = "> "
	itemPaddingLeft = 2
	// Height reserved for the blank line + footer below the list.
	helpReservedLines = 2

	multiMarkerOn  = "[x] "
	multiMarkerOff = "[ ] "
)

var (
	selectedColor     = lipgloss.Color("170")
	extraColor        = lipgloss.Color("3") // ANSI yellow, matches color.InYellow
	itemStyle         = lipgloss.NewStyle().PaddingLeft(itemPaddingLeft)
	itemSelectedStyle = itemStyle.Foreground(selectedColor).Bold(true)
	extraStyle        = itemStyle.Foreground(extraColor)
	helpStyle         = lipgloss.NewStyle().Faint(true).PaddingLeft(itemPaddingLeft)
)

var ErrPickerCancelled = errors.New("selection cancelled")

type pickerItem string

func (i pickerItem) FilterValue() string { return string(i) }

// One-line renderer (default delegate uses two lines). In multi mode the
// toggled map is shared by reference with PickerModel — Render sees toggle
// changes without a SetDelegate call.
type pickerDelegate struct {
	extras  map[string]bool
	multi   bool
	toggled map[string]bool
}

func (pickerDelegate) Height() int                             { return 1 }
func (pickerDelegate) Spacing() int                            { return 0 }
func (pickerDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d pickerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	name := string(item.(pickerItem))
	if d.multi {
		marker := multiMarkerOff
		if d.toggled[name] {
			marker = multiMarkerOn
		}
		if index == m.Index() {
			fmt.Fprint(w, itemSelectedStyle.Render(selectedPrefix+marker+name))
			return
		}
		style := itemStyle
		if d.extras[name] {
			style = extraStyle
		}
		fmt.Fprint(w, style.Render(marker+name))
		return
	}
	if index == m.Index() {
		fmt.Fprint(w, itemSelectedStyle.Render(selectedPrefix+name))
		return
	}
	style := itemStyle
	if d.extras[name] {
		style = extraStyle
	}
	fmt.Fprint(w, style.Render(name))
}

// PickerModel is a fuzzy-filter list picker. Sets done/cancelled flags;
// never returns tea.Quit. Multi mode (NewPickerModelMulti) adds a toggled
// set, space-to-toggle, and Selections().
type PickerModel struct {
	list       list.Model
	chosen     string
	selections []string
	done       bool
	cancelled  bool
	showHelp   bool
	multi      bool
	toggled    map[string]bool
}

// NewPickerModel builds a single-select PickerModel over names + extras.
// Extras render yellow when unselected; selected items stay purple+bold.
func NewPickerModel(names, extras []string) PickerModel {
	extrasSet := make(map[string]bool, len(extras))
	for _, e := range extras {
		extrasSet[e] = true
	}
	items := make([]list.Item, 0, len(names)+len(extras))
	for _, n := range names {
		items = append(items, pickerItem(n))
	}
	for _, e := range extras {
		items = append(items, pickerItem(e))
	}
	return PickerModel{list: newPickerList(items, pickerDelegate{extras: extrasSet}), showHelp: true}
}

// NewPickerModelMulti builds a multi-select PickerModel. No extras —
// main-password (the one extras caller) stays single-select.
func NewPickerModelMulti(names []string) PickerModel {
	toggled := map[string]bool{}
	items := make([]list.Item, 0, len(names))
	for _, n := range names {
		items = append(items, pickerItem(n))
	}
	l := newPickerList(items, pickerDelegate{multi: true, toggled: toggled})
	return PickerModel{list: l, showHelp: true, multi: true, toggled: toggled}
}

func newPickerList(items []list.Item, delegate pickerDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.Title = pickerTitle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // we render our own footer that matches the actual keybindings
	l.SetFilteringEnabled(true)
	// Open in filter mode with all items visible. SetFilterText("") fills
	// filteredItems; SetFilterState focuses the input. SetFilterState alone
	// would render a blank list (only the '/' key handler fills filteredItems).
	l.SetFilterText("")
	l.SetFilterState(list.Filtering)
	return l
}

// WithoutHelp disables the in-view help footer. Use when the host renders
// its own footer (e.g. menu mode anchors help to the bottom row).
func (m PickerModel) WithoutHelp() PickerModel {
	m.showHelp = false
	return m
}

// Help returns the unstyled help text for hosts that supplied WithoutHelp.
func (m PickerModel) Help() string {
	if m.multi {
		return pickerMultiHelp
	}
	return pickerHelp
}

// Reopen clears done/selections so the picker accepts input again after a
// host bounce-back (e.g. user declined the confirm). Toggled set is
// preserved so the user keeps their selections.
func (m *PickerModel) Reopen() {
	m.done = false
	m.selections = nil
}

func (m PickerModel) Init() tea.Cmd { return nil }

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Always reserve 2 lines below the list — either picker's own help
		// (showHelp=true) or the host's external footer (showHelp=false).
		m.list.SetSize(msg.Width, msg.Height-helpReservedLines)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, nil
		case "enter":
			// Select on first Enter, even mid-filter. bubbles/list otherwise
			// treats Enter in Filtering as "apply filter" — would need two presses.
			if item, ok := m.list.SelectedItem().(pickerItem); ok {
				m.chosen = string(item)
				if m.multi && len(m.toggled) > 0 {
					m.selections = m.collectToggled()
				}
				m.done = true
				return m, nil
			}
		case "space":
			// Toggle in multi mode. Trade-off: filtering names containing a
			// space stops working here (psw names don't have spaces).
			// Single mode falls through to the list filter.
			if m.multi {
				if item, ok := m.list.SelectedItem().(pickerItem); ok {
					name := string(item)
					if m.toggled[name] {
						delete(m.toggled, name)
					} else {
						m.toggled[name] = true
					}
				}
				return m, nil
			}
		case "up", "ctrl+p":
			// bubbles/list disables arrow-nav while filtering; call directly. Also wires ctrl+n/p.
			m.list.CursorUp()
			return m, nil
		case "down", "ctrl+n":
			m.list.CursorDown()
			return m, nil
		}
	case list.FilterMatchesMsg:
		// bubbles/list doesn't snap cursor to top when the filter narrows; reset it.
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		m.list.ResetSelected()
		return m, cmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// collectToggled returns toggled names in list (insertion) order.
func (m PickerModel) collectToggled() []string {
	out := make([]string, 0, len(m.toggled))
	for _, it := range m.list.Items() {
		name := string(it.(pickerItem))
		if m.toggled[name] {
			out = append(out, name)
		}
	}
	return out
}

func (m PickerModel) View() tea.View {
	content := ""
	if !m.done && !m.cancelled {
		content = m.list.View()
		if m.showHelp {
			content += "\n\n" + helpStyle.Render(m.Help())
		}
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m PickerModel) Done() bool           { return m.done }
func (m PickerModel) Cancelled() bool      { return m.cancelled }
func (m PickerModel) Selection() string    { return m.chosen }
func (m PickerModel) Selections() []string { return m.selections }

// ResolvedSelections returns Selections() if any toggled; otherwise falls
// back to [Selection()] (cursor item) for the no-toggles fast path. Returns
// nil when neither was set (no items / no cursor) — caller treats as cancel.
func (m PickerModel) ResolvedSelections() []string {
	if len(m.selections) > 0 {
		return m.selections
	}
	if m.chosen == "" {
		return nil
	}
	return []string{m.chosen}
}

// One item across names+extras → return it without launching the TUI.
// Empty → ("", nil).
func GetRecordNameInteractive(names, extras []string) (string, error) {
	total := len(names) + len(extras)
	if total == 0 {
		return "", nil
	}
	if total == 1 {
		if len(names) == 1 {
			return names[0], nil
		}
		return extras[0], nil
	}

	final, err := tea.NewProgram(tuiutil.QuittingWrapper[PickerModel]{M: NewPickerModel(names, extras)}).Run()
	if err != nil {
		return "", fmt.Errorf("interactive picker failed: %w", err)
	}
	quitter, ok := final.(tuiutil.QuittingWrapper[PickerModel])
	if !ok {
		return "", fmt.Errorf("interactive picker returned unexpected model type %T", final)
	}
	if quitter.M.Cancelled() || quitter.M.Selection() == "" {
		return "", ErrPickerCancelled
	}
	return quitter.M.Selection(), nil
}

// GetRecordNamesInteractive runs a multi-select picker. Single-name fast
// path returns [name] without launching the TUI. Otherwise returns
// ResolvedSelections from the picker (cancel → ErrPickerCancelled).
func GetRecordNamesInteractive(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if len(names) == 1 {
		return []string{names[0]}, nil
	}

	final, err := tea.NewProgram(tuiutil.QuittingWrapper[PickerModel]{M: NewPickerModelMulti(names)}).Run()
	if err != nil {
		return nil, fmt.Errorf("interactive picker failed: %w", err)
	}
	quitter, ok := final.(tuiutil.QuittingWrapper[PickerModel])
	if !ok {
		return nil, fmt.Errorf("interactive picker returned unexpected model type %T", final)
	}
	if quitter.M.Cancelled() {
		return nil, ErrPickerCancelled
	}
	sels := quitter.M.ResolvedSelections()
	if sels == nil {
		return nil, ErrPickerCancelled
	}
	return sels, nil
}
