package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/ui"
)

// passwordMismatchBanner is shown above the input on a repeat-password mismatch.
var passwordMismatchBanner = color.InYellow("Passwords don't match, try again")

// Phase-transition helpers. Caller pattern: `return a.toX(...)`.

func (a AddAction) toInput(label string, password, animate bool, phase addPhase) (tea.Model, tea.Cmd) {
	a.input = prompt.NewInputModel(label, password, animate)
	a.phase = phase
	return a, a.input.Init()
}

func (a AddAction) toYesNo(question string, phase addPhase) (tea.Model, tea.Cmd) {
	a.yesNo = prompt.NewYesNoModel(question)
	a.phase = phase
	return a, nil
}

func (a AddAction) toSpinner(label string, phase addPhase, op tea.Cmd) (tea.Model, tea.Cmd) {
	a.spinner = ui.NewSpinnerModel(label)
	a.phase = phase
	return a, tea.Batch(op, a.spinner.Init())
}

func (a ChangeAction) toInput(label string, password, animate bool, phase changePhase) (tea.Model, tea.Cmd) {
	a.input = prompt.NewInputModel(label, password, animate)
	a.phase = phase
	return a, a.input.Init()
}

func (a ChangeAction) toYesNo(question string, phase changePhase) (tea.Model, tea.Cmd) {
	a.yesNo = prompt.NewYesNoModel(question)
	a.phase = phase
	return a, nil
}

func (a ChangeAction) toSpinner(label string, phase changePhase, op tea.Cmd) (tea.Model, tea.Cmd) {
	a.spinner = ui.NewSpinnerModel(label)
	a.phase = phase
	return a, tea.Batch(op, a.spinner.Init())
}

// prependBanner adds banner above v.Content and bumps the cursor row.
// Defensive cursor copy avoids aliasing the sub-model's cursor.
func prependBanner(v tea.View, banner string) tea.View {
	if banner == "" {
		return v
	}
	v.Content = banner + "\n" + v.Content
	if v.Cursor != nil {
		c := *v.Cursor
		c.Position.Y += strings.Count(banner, "\n") + 1
		v.Cursor = &c
	}
	return v
}

// prependTranscript prepends transcript lines above v.Content with a newline
// separator. Bumps the cursor row by the prepended row count.
func prependTranscript(v tea.View, transcript []string) tea.View {
	if len(transcript) == 0 {
		return v
	}
	block := strings.Join(transcript, "\n")
	v.Content = block + "\n" + v.Content
	if v.Cursor != nil {
		c := *v.Cursor
		c.Position.Y += strings.Count(block, "\n") + 1
		v.Cursor = &c
	}
	return v
}

func formatYesNoLine(m prompt.YesNoModel) string {
	ans := "n"
	if m.Answer() {
		ans = "y"
	}
	return fmt.Sprintf("%s (y/n) %s", m.Question(), ans)
}

// formatInputLine renders a completed input as "Label: value"; hidden inputs
// (password / animated-stars) show asterisks matching the value's cell width.
func formatInputLine(m prompt.InputModel) string {
	val := m.Value()
	if m.Hidden() {
		val = strings.Repeat("*", lipgloss.Width(val))
	}
	return m.Prefix() + val
}

// Action is one menu operation: get / add / change / remove.
type Action interface {
	tea.Model
	Done() bool
	Cancelled() bool
	// Output is the action's display lines, read after Done.
	Output() []string
	// NewPassword is the rotated main password, non-empty after `change main`.
	NewPassword() string
	// Transcript is intra-action prompt history, shown above the active sub-view.
	Transcript() []string
}

// newAction builds an Action by name and returns its initial Cmd.
func newAction(name, password string) (Action, tea.Cmd) {
	switch name {
	case "get":
		a := NewGetAction(password)
		return a, a.Init()
	case "add":
		a := NewAddAction(password)
		return a, a.Init()
	case "change":
		a := NewChangeAction(password)
		return a, a.Init()
	case "remove":
		a := NewRemoveAction(password)
		return a, a.Init()
	}
	return nil, nil
}
