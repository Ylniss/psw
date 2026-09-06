package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
)

type settingsPhase int

const (
	settingsPhaseGrid settingsPhase = iota
	settingsPhaseEditing
	settingsPhaseSaving
)

var (
	// settingsValueStyle is ANSI yellow ("3"), matching psw get's
	// username/value coloring.
	settingsValueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	settingsSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	settingsDescStyle     = lipgloss.NewStyle().Italic(true).Faint(true)
	settingsDotsStyle     = lipgloss.NewStyle().Faint(true)
)

type SettingsAction struct {
	baseAction

	phase settingsPhase

	cursor     int
	dirty      bool
	editingKey storage.ConfigKey
	editError  string
}

func NewSettingsAction() SettingsAction {
	return SettingsAction{
		baseAction: newBase("Saving"),
		phase:      settingsPhaseGrid,
	}
}

func (a SettingsAction) Init() tea.Cmd { return nil }

func (a SettingsAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	a.captureSize(msg)
	switch a.phase {
	case settingsPhaseGrid:
		return a.updateGrid(msg)
	case settingsPhaseEditing:
		return a.updateEditing(msg)
	case settingsPhaseSaving:
		return a.updateSaving(msg)
	}
	return a, nil
}

func (a SettingsAction) updateGrid(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return a, nil
	}
	switch k.String() {
	case "ctrl+c":
		a.cancelled = true
		return a, nil
	case "esc", "q":
		if !a.dirty {
			a.cancelled = true
			return a, nil
		}
		spinnerCmd := a.initSpinner("Saving")
		a.phase = settingsPhaseSaving
		return a, tea.Batch(saveConfigCmd(), spinnerCmd)
	case "up", "k", "shift+tab":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j", "tab":
		if a.cursor < len(storage.ConfigKeys)-1 {
			a.cursor++
		}
	case "left", "h":
		if adj := storage.ConfigKeys[a.cursor].Adjust; adj != nil {
			adj(&storage.AppConfig, -1)
			a.dirty = true
		}
	case "right", "l":
		if adj := storage.ConfigKeys[a.cursor].Adjust; adj != nil {
			adj(&storage.AppConfig, +1)
			a.dirty = true
		}
	case "enter", "space":
		a.editingKey = storage.ConfigKeys[a.cursor]
		cmd := a.initInput("New value for "+a.editingKey.Name, false, false)
		a.input = a.input.WithInitialValue(a.editingKey.Current(&storage.AppConfig))
		a.editError = ""
		a.phase = settingsPhaseEditing
		return a, cmd
	}
	return a, nil
}

func (a SettingsAction) updateEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "esc" {
		a.editError = ""
		a.phase = settingsPhaseGrid
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		// Ctrl+c only — esc handled above.
		a.cancelled = true
		return a, nil
	}
	if !a.input.Done() {
		return a, cmd
	}
	value := a.input.Value()
	if err := a.editingKey.Apply(&storage.AppConfig, value); err != nil {
		a.editError = err.Error()
		initCmd := a.initInput("New value for "+a.editingKey.Name, false, false)
		a.input = a.input.WithInitialValue(value)
		return a, initCmd
	}
	a.dirty = true
	a.editError = ""
	a.phase = settingsPhaseGrid
	return a, nil
}

func (a SettingsAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(configSavedMsg); ok {
		if m.err != nil {
			a.finishErr(m.err)
			return a, nil
		}
		a.finish(fmt.Sprintf("Settings saved to %s", color.InCyan(storage.Paths.ConfigFilePath())))
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a SettingsAction) View() tea.View {
	switch a.phase {
	case settingsPhaseGrid:
		content := a.renderGrid()
		if desc := storage.ConfigKeys[a.cursor].Description; desc != "" {
			content += "\n\n" + settingsDescStyle.Width(a.columnWidth()).Render(desc)
		}
		return tea.NewView(content)
	case settingsPhaseEditing:
		header := fmt.Sprintf("Editing %s (current: %s)",
			color.InCyan(a.editingKey.Name),
			a.editingKey.DisplayValue(&storage.AppConfig),
		)
		banner := ""
		if a.editError != "" {
			banner = color.InRed(a.editError)
		}
		view := prependBanner(a.input.View(), banner)
		// Trailing newline gives the blank row between header and input.
		return prependBanner(view, header+"\n")
	case settingsPhaseSaving:
		return a.spinner.View()
	}
	return tea.NewView("")
}

func (a SettingsAction) renderGrid() string {
	rowWidth := a.columnWidth()
	var b strings.Builder
	for i, k := range storage.ConfigKeys {
		name := k.Name
		value := k.DisplayValue(&storage.AppConfig)
		prefix := "  "
		if i == a.cursor {
			prefix = "> "
		}
		gap := max(rowWidth-lipgloss.Width(prefix)-lipgloss.Width(name)-lipgloss.Width(value), 4)
		dots := strings.Repeat(".", gap-2)

		var line string
		if i == a.cursor {
			line = settingsSelectedStyle.Render(prefix+name+" ") +
				settingsDotsStyle.Render(dots) +
				settingsSelectedStyle.Render(" "+value)
		} else {
			line = prefix + name + " " +
				settingsDotsStyle.Render(dots) +
				" " + settingsValueStyle.Render(value)
		}
		b.WriteString(line)
		if i < len(storage.ConfigKeys)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (a SettingsAction) columnWidth() int {
	if a.width <= 0 {
		return pswHeaderWidth + 2*contentPaddingX
	}
	return contentColumnWidth(a.width)
}

func (a SettingsAction) FooterHelp() string {
	switch a.phase {
	case settingsPhaseGrid:
		return "↑/↓ or k/j or tab/shift+tab navigate · ←/→ or h/l ±1\nenter/space edit · esc save & exit"
	case settingsPhaseEditing:
		return "enter accept · esc back to grid"
	}
	return ""
}
