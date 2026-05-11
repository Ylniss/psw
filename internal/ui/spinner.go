package ui

import (
	"log/slog"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"

	"github.com/ylniss/psw/internal/tuiutil"
)

// spinnerShowAfter is the wait before painting; faster ops stay silent.
const spinnerShowAfter = 250 * time.Millisecond

var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

// SpinnersQuiet, when true, makes WithSpinner run op synchronously without a
// nested tea.Program. Set by hosts that already own the screen.
var SpinnersQuiet bool

// WithSpinner runs op, painting a labeled spinner on stderr if op exceeds
// spinnerShowAfter and stderr is a TTY. Op's error is returned unchanged.
func WithSpinner(label string, op func() error) error {
	if SpinnersQuiet || !isStderrTTY() {
		return op()
	}

	var opErr error
	opDone := make(chan struct{})
	go func() {
		opErr = op()
		close(opDone)
	}()

	timer := time.NewTimer(spinnerShowAfter)
	defer timer.Stop()
	select {
	case <-opDone:
		return opErr
	case <-timer.C:
	}

	program := tea.NewProgram(
		tuiutil.QuittingWrapper[SpinnerModel]{M: NewSpinnerModel(label)},
		tea.WithOutput(os.Stderr),
	)
	go func() {
		<-opDone
		program.Send(DoneMsg{})
	}()

	if _, err := program.Run(); err != nil {
		slog.Debug("spinner tea.Run failed", "err", err)
	}
	<-opDone
	return opErr
}

func isStderrTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = spinnerStyle
	return s
}

// DoneMsg tells SpinnerModel its underlying op has finished.
type DoneMsg struct{}

// SpinnerModel is a spinner driven by the caller via DoneMsg.
type SpinnerModel struct {
	label     string
	spinner   spinner.Model
	completed bool
}

func NewSpinnerModel(label string) SpinnerModel {
	return SpinnerModel{label: label, spinner: newSpinner()}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(DoneMsg); ok {
		m.completed = true
		return m, nil
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m SpinnerModel) View() tea.View {
	if m.completed {
		return tea.NewView("")
	}
	return tea.NewView(m.spinner.View() + " " + m.label)
}

// Done / Cancelled make SpinnerModel a tuiutil.Finishable. The spinner has no
// user-cancel path, so Cancelled is always false.
func (m SpinnerModel) Done() bool      { return m.completed }
func (m SpinnerModel) Cancelled() bool { return false }
