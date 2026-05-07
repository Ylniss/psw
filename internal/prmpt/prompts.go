package prmpt

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/TwiN/go-color"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ErrPromptCancelled is returned by input prompts when the user dismisses
// them via Esc or Ctrl-C. Callers translate it to a silent exit.
var ErrPromptCancelled = errors.New("prompt cancelled")

var (
	passwordsDontMatchMsg = "Passwords don't match, try again"
	errRequired           = errors.New("input required")
	errNoTTY              = errors.New("interactive prompt required: stdin is not a terminal")
	promptErrStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func validateRequired(content string) error {
	if len(content) < 1 {
		return errRequired
	}
	return nil
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

type inputModel struct {
	label     string
	input     textinput.Model
	errMsg    string
	submitted bool
	cancelled bool
}

func newInputModel(label string, password bool) inputModel {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Focus()
	if password {
		ti.EchoMode = textinput.EchoPassword
	}
	return inputModel{label: label, input: ti}
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if err := validateRequired(m.input.Value()); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.submitted = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	if m.cancelled {
		return ""
	}
	view := fmt.Sprintf("%s: %s", m.label, m.input.View())
	if m.errMsg != "" {
		view += "\n" + promptErrStyle.Render(m.errMsg)
	}
	return view
}

func runInput(label string, password bool) (string, error) {
	if !isTTY() {
		return "", errNoTTY
	}
	final, err := tea.NewProgram(newInputModel(label, password)).Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	fm, ok := final.(inputModel)
	if !ok {
		return "", fmt.Errorf("prompt returned unexpected model type %T", final)
	}
	if fm.cancelled {
		return "", ErrPromptCancelled
	}
	val := fm.input.Value()
	// Bubbletea wipes its inline render region on exit. Re-emit the answered
	// prompt so it persists in scrollback above the next prompt.
	display := val
	if password {
		display = strings.Repeat("*", len(val))
	}
	fmt.Printf("%s: %s\n", label, display)
	return val, nil
}

type yesNoModel struct {
	question string
	answer   bool
	decided  bool
}

func (m yesNoModel) Init() tea.Cmd { return nil }

func (m yesNoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "y", "Y":
			m.answer = true
			m.decided = true
			return m, tea.Quit
		case "n", "N":
			m.answer = false
			m.decided = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m yesNoModel) View() string {
	return fmt.Sprintf("%s (y/n)", m.question)
}

// YesOrNo treats cancel (Esc/Ctrl-C) and non-TTY stdin as "no" so scripts
// don't get stuck and Ctrl-C in a confirm dialog is interpreted as "bail out
// of this change," matching the existing scripting-safe contract.
func YesOrNo(question string) bool {
	if !isTTY() {
		return false
	}
	final, err := tea.NewProgram(yesNoModel{question: question}).Run()
	if err != nil {
		return false
	}
	fm, ok := final.(yesNoModel)
	if !ok {
		return false
	}
	answer := fm.decided && fm.answer
	ans := "n"
	if answer {
		ans = "y"
	}
	fmt.Printf("%s (y/n) %s\n", question, ans)
	return answer
}

func PromptForName(promptText string) (string, error) {
	return runInput(promptText, false)
}

func PromptForRecordPass() (string, error) {
	for {
		first, err := runInput("Password", true)
		if err != nil {
			return "", err
		}
		repeat, err := runInput("Repeat password", true)
		if err != nil {
			return "", err
		}
		if first == repeat {
			return first, nil
		}
		fmt.Println(color.InCyan(passwordsDontMatchMsg))
	}
}

func PromptForMainPassChange() (string, error) {
	return promptForMainPass(true, true)
}

func PromptForMainPass(ensure bool) (string, error) {
	return promptForMainPass(ensure, false)
}

func promptForMainPass(ensure bool, mainPassChange bool) (string, error) {
	envVar := "PSW_MAIN_PASSWORD"
	if mainPassChange {
		envVar = "PSW_NEW_MAIN_PASSWORD"
	}
	if envPass := os.Getenv(envVar); envPass != "" {
		return envPass, nil
	}

	askText := "Main password"
	repeatText := "Repeat main password"
	if mainPassChange {
		askText = "New main password"
		repeatText = "Repeat new main password"
	}

	for {
		first, err := runInput(askText, true)
		if err != nil {
			return "", err
		}
		if !ensure {
			return first, nil
		}
		repeat, err := runInput(repeatText, true)
		if err != nil {
			return "", err
		}
		if first == repeat {
			return first, nil
		}
		fmt.Println(color.InYellow(passwordsDontMatchMsg))
	}
}
