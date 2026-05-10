package menu

import (
	"strings"

	tea "charm.land/bubbletea/v2"
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

// Action is one menu operation: get / add / change / remove.
type Action interface {
	tea.Model
	Done() bool
	Cancelled() bool
	// Output is the lines to append to history. Read after Done.
	Output() []string
	// NewPassword is the rotated main password, non-empty after `change main`.
	NewPassword() string
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
