package storage

import (
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Picker visual / behavior knobs.
const (
	pickerTitle     = "Select a record"
	pickerHelp      = "↑/↓ or ctrl+n/p navigate · enter select · esc cancel · type to filter"
	selectedPrefix  = "> "
	itemPaddingLeft = 2
	// helpReservedLines is the height we subtract from the list so our blank
	// line + custom help footer don't push the bottom items off-screen.
	helpReservedLines = 2
)

var (
	selectedColor = lipgloss.Color("170")
	helpStyle     = lipgloss.NewStyle().Faint(true).PaddingLeft(itemPaddingLeft)
)

var ErrPickerCancelled = errors.New("selection cancelled")

type pickerItem string

func (i pickerItem) FilterValue() string { return string(i) }

// pickerDelegate satisfies bubbles/list.ItemDelegate. We supply a custom
// one — instead of list.NewDefaultDelegate — so each record renders as a
// single padded line rather than the default two-line title+description.
type pickerDelegate struct{}

func (pickerDelegate) Height() int                             { return 1 }
func (pickerDelegate) Spacing() int                            { return 0 }
func (pickerDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (pickerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	name := string(item.(pickerItem))
	style := lipgloss.NewStyle().PaddingLeft(itemPaddingLeft)
	if index == m.Index() {
		style = style.Foreground(selectedColor).Bold(true)
		name = selectedPrefix + name
	}
	fmt.Fprint(w, style.Render(name))
}

type pickerModel struct {
	list     list.Model
	chosen   string
	quitting bool
}

func (m pickerModel) Init() tea.Cmd {
	// Send a fake "/" keypress so the picker starts in filter mode; otherwise
	// the user's first keystroke would navigate the list instead of filtering.
	return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} }
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-helpReservedLines)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			// Pick the highlighted item on the first Enter, even while the
			// user is still typing the filter. Without this, bubbles/list
			// treats Enter in Filtering state as "apply filter" and the user
			// would have to press Enter twice to actually select.
			if item, ok := m.list.SelectedItem().(pickerItem); ok {
				m.chosen = string(item)
				return m, tea.Quit
			}
		case "up", "ctrl+p":
			// bubbles/list disables its CursorUp/Down bindings while filtering,
			// so we call them directly to keep arrow-key navigation alive and
			// to wire ctrl+n/p as aliases.
			m.list.CursorUp()
			return m, nil
		case "down", "ctrl+n":
			m.list.CursorDown()
			return m, nil
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
	return m.list.View() + "\n\n" + helpStyle.Render(pickerHelp)
}

// One name → return it without launching the TUI. Empty names → ("", nil).
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
	l := list.New(items, pickerDelegate{}, 0, 0)
	l.Title = pickerTitle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // we render our own footer that matches the actual keybindings
	l.SetFilteringEnabled(true)

	program := tea.NewProgram(pickerModel{list: l}, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return "", fmt.Errorf("interactive picker failed: %w", err)
	}
	finalModel, ok := final.(pickerModel)
	if !ok {
		return "", fmt.Errorf("interactive picker returned unexpected model type %T", final)
	}
	if finalModel.chosen == "" {
		return "", ErrPickerCancelled
	}
	return finalModel.chosen, nil
}
