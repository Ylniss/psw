package ui

import (
	"log/slog"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// spinnerThreshold is the wait before painting; faster ops stay silent.
const spinnerThreshold = 250 * time.Millisecond

var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

// WithSpinner runs op, painting a labeled spinner on stderr if op exceeds
// spinnerThreshold and stderr is a TTY. Op's error is returned unchanged.
func WithSpinner(label string, op func() error) error {
	if !isStderrTTY() {
		return op()
	}

	var opErr error
	opDone := make(chan struct{})
	go func() {
		opErr = op()
		close(opDone)
	}()

	timer := time.NewTimer(spinnerThreshold)
	defer timer.Stop()
	select {
	case <-opDone:
		return opErr
	case <-timer.C:
	}

	program := tea.NewProgram(
		spinnerModel{label: label, spinner: newSpinner()},
		tea.WithOutput(os.Stderr),
	)
	go func() {
		<-opDone
		program.Send(doneMsg{})
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

type doneMsg struct{}

type spinnerModel struct {
	label     string
	spinner   spinner.Model
	completed bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(doneMsg); ok {
		m.completed = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() tea.View {
	if m.completed {
		return tea.NewView("")
	}
	return tea.NewView(m.spinner.View() + " " + m.label)
}
