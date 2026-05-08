package prompt

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"golang.org/x/term"
)

// ErrPromptCancelled means user pressed Esc/Ctrl-C. Callers exit silently.
var ErrPromptCancelled = errors.New("prompt cancelled")

var (
	passwordMismatchMsg = "Passwords don't match, try again"
	errRequired               = errors.New("input required")
	errNoTTY                  = errors.New("interactive prompt required: stdin is not a terminal")
	promptErrorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
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
	prefix      string
	prefixWidth int
	input       textinput.Model
	errMsg      string
	cancelled   bool
}

func newInputModel(label string, password bool) inputModel {
	textInput := textinput.New()
	textInput.Prompt = ""
	textInput.SetVirtualCursor(false)
	textInput.Focus()
	if password {
		textInput.EchoMode = textinput.EchoPassword
	}
	prefix := label + ": "
	return inputModel{
		prefix:      prefix,
		prefixWidth: lipgloss.Width(prefix),
		input:       textInput,
	}
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if err := validateRequired(m.input.Value()); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m inputModel) View() tea.View {
	if m.cancelled {
		return tea.NewView("")
	}
	content := m.prefix + m.input.View()
	if m.errMsg != "" {
		content += "\n" + promptErrorStyle.Render(m.errMsg)
	}
	v := tea.NewView(content)
	if c := m.input.Cursor(); c != nil {
		c.Position.X += m.prefixWidth
		v.Cursor = c
	}
	return v
}

func runInput(label string, password bool) (string, error) {
	if !isTTY() {
		return "", errNoTTY
	}
	final, err := tea.NewProgram(newInputModel(label, password)).Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	finalModel, ok := final.(inputModel)
	if !ok {
		return "", fmt.Errorf("prompt returned unexpected model type %T", final)
	}
	if finalModel.cancelled {
		return "", ErrPromptCancelled
	}
	val := finalModel.input.Value()
	// Bubbletea wipes its render region on exit; re-emit to keep the answer in scrollback.
	display := val
	if password {
		// Cell width, not bytes — matches textinput's EchoPassword.
		display = strings.Repeat("*", lipgloss.Width(val))
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
	if k, ok := msg.(tea.KeyPressMsg); ok {
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

func (m yesNoModel) View() tea.View {
	return tea.NewView(fmt.Sprintf("%s (y/n)", m.question))
}

// YesOrNo returns false on cancel (Esc/Ctrl-C) or non-TTY stdin — keeps scripts unblocked and treats Ctrl-C as "bail out".
func YesOrNo(question string) bool {
	if !isTTY() {
		return false
	}
	final, err := tea.NewProgram(yesNoModel{question: question}).Run()
	if err != nil {
		return false
	}
	finalModel, ok := final.(yesNoModel)
	if !ok {
		return false
	}
	answer := finalModel.decided && finalModel.answer
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

func PromptForRecordPassword() (string, error) {
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
		fmt.Println(color.InYellow(passwordMismatchMsg))
	}
}

func PromptForMainPasswordChange() (string, error) {
	return promptForMainPassword(true, true)
}

func PromptForMainPassword(ensure bool) (string, error) {
	return promptForMainPassword(ensure, false)
}

func promptForMainPassword(ensure bool, mainPasswordChange bool) (string, error) {
	envVar := "PSW_MAIN_PASSWORD"
	if mainPasswordChange {
		envVar = "PSW_NEW_MAIN_PASSWORD"
	}
	if envPass := os.Getenv(envVar); envPass != "" {
		return envPass, nil
	}

	askText := "Main password"
	repeatText := "Repeat main password"
	if mainPasswordChange {
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
		fmt.Println(color.InYellow(passwordMismatchMsg))
	}
}
