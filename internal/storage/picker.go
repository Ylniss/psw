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
	pickerHelp      = "↑/↓ or ctrl+n/p navigate · enter select · esc cancel · type to filter"
	selectedPrefix  = "> "
	itemPaddingLeft = 2
	// Height reserved for the blank line + footer below the list.
	helpReservedLines = 2
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

// Custom ItemDelegate so each record renders on one line, not two.
type pickerDelegate struct {
	extras map[string]bool
}

func (pickerDelegate) Height() int                             { return 1 }
func (pickerDelegate) Spacing() int                            { return 0 }
func (pickerDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d pickerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	name := string(item.(pickerItem))
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
// never returns tea.Quit.
type PickerModel struct {
	list      list.Model
	chosen    string
	done      bool
	cancelled bool
}

// NewPickerModel builds a PickerModel over names + extras. Extras render
// yellow when unselected; selected items stay purple+bold regardless.
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
	l := list.New(items, pickerDelegate{extras: extrasSet}, 0, 0)
	l.Title = pickerTitle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // we render our own footer that matches the actual keybindings
	l.SetFilteringEnabled(true)
	// Open in filter mode with all items visible. SetFilterText("") fills
	// filteredItems; SetFilterState focuses the input. SetFilterState alone
	// would render a blank list (only the '/' key handler fills filteredItems).
	l.SetFilterText("")
	l.SetFilterState(list.Filtering)
	return PickerModel{list: l}
}

func (m PickerModel) Init() tea.Cmd { return nil }

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
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
				m.done = true
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

func (m PickerModel) View() tea.View {
	content := ""
	if !m.done && !m.cancelled {
		content = m.list.View() + "\n\n" + helpStyle.Render(pickerHelp)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m PickerModel) Done() bool       { return m.done }
func (m PickerModel) Cancelled() bool  { return m.cancelled }
func (m PickerModel) Selection() string { return m.chosen }

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

	final, err := tea.NewProgram(tuiutil.Quitter[PickerModel]{M: NewPickerModel(names, extras)}).Run()
	if err != nil {
		return "", fmt.Errorf("interactive picker failed: %w", err)
	}
	finalWrap, ok := final.(tuiutil.Quitter[PickerModel])
	if !ok {
		return "", fmt.Errorf("interactive picker returned unexpected model type %T", final)
	}
	if finalWrap.M.Cancelled() || finalWrap.M.Selection() == "" {
		return "", ErrPickerCancelled
	}
	return finalWrap.M.Selection(), nil
}
