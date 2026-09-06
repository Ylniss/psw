package menu

import (
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/prompt"
)

// passwordMismatchBanner is shown above the input on a repeat-password mismatch.
// Lazy so it captures color.Yellow after cli's init() restores it on Windows.
var passwordMismatchBanner = sync.OnceValue(func() string {
	return color.InYellow(prompt.PasswordMismatchMsg)
})

// Phase-transition helpers. Caller pattern: `return a.toX(...)`. Wrappers
// are duplicated per action because Go has no method-level generics over
// the phase enum; bodies delegate to baseAction's initX helpers.

func (a AddAction) toInput(label string, password, animate bool, phase addPhase) (tea.Model, tea.Cmd) {
	cmd := a.initInput(label, password, animate)
	a.phase = phase
	return a, cmd
}

func (a AddAction) toYesNo(question string, phase addPhase) (tea.Model, tea.Cmd) {
	a.initYesNo(question)
	a.phase = phase
	return a, nil
}

func (a AddAction) toSpinner(label string, phase addPhase, op tea.Cmd) (tea.Model, tea.Cmd) {
	cmd := a.initSpinner(label)
	a.phase = phase
	return a, tea.Batch(op, cmd)
}

func (a ChangeAction) toInput(label string, password, animate bool, phase changePhase) (tea.Model, tea.Cmd) {
	cmd := a.initInput(label, password, animate)
	a.phase = phase
	return a, cmd
}

func (a ChangeAction) toYesNo(question string, phase changePhase) (tea.Model, tea.Cmd) {
	a.initYesNo(question)
	a.phase = phase
	return a, nil
}

func (a ChangeAction) toSpinner(label string, phase changePhase, op tea.Cmd) (tea.Model, tea.Cmd) {
	cmd := a.initSpinner(label)
	a.phase = phase
	return a, tea.Batch(op, cmd)
}

// Action is one menu operation: get, add, change, remove, settings or rollback.
type Action interface {
	tea.Model
	Done() bool
	Cancelled() bool
	// Output is the action's display lines, read after Done.
	Output() []string
	// NewPassword is the rotated main password, non-nil after `change main`.
	NewPassword() *memguard.Enclave
	// FooterHelp is the help line to render at the menu's bottom row;
	// empty when the action's current sub-view doesn't want a footer.
	FooterHelp() string
}
