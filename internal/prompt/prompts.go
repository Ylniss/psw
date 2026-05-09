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
	"github.com/ylniss/psw/internal/menulayout"
	"golang.org/x/term"
)

// ErrPromptCancelled means user pressed Esc/Ctrl-C. Callers exit silently.
var ErrPromptCancelled = errors.New("prompt cancelled")

var (
	passwordMismatchMsg = "Passwords don't match, try again"
	errRequired         = errors.New("input required")
	errNoTTY            = errors.New("interactive prompt required: stdin is not a terminal")
	promptErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
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
	prefix       string
	prefixWidth  int
	input        textinput.Model
	errMsg       string
	cancelled    bool
	animateStars bool
	stars        StarState
	ticking      bool
}

func newInputModel(label string, password, animateStars bool) inputModel {
	textInput := textinput.New()
	textInput.Prompt = ""
	textInput.SetVirtualCursor(false)
	textInput.Focus()
	switch {
	case animateStars:
		textInput.EchoMode = textinput.EchoNone
	case password:
		textInput.EchoMode = textinput.EchoPassword
	}
	prefix := label + ": "
	m := inputModel{
		prefix:       prefix,
		prefixWidth:  lipgloss.Width(prefix),
		input:        textInput,
		animateStars: animateStars,
	}
	if animateStars {
		m.stars = NewStarState()
	}
	return m
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
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
	case StarTickMsg:
		if m.animateStars && m.stars.Active() {
			return m, StarTick()
		}
		m.ticking = false
		return m, nil
	}

	prevLen := len(m.input.Value())
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if !m.animateStars {
		return m, cmd
	}
	newLen := len(m.input.Value())
	changed := false
	switch {
	case newLen == 0:
		changed = m.stars.ApplyEmpty()
	case newLen > prevLen:
		changed = m.stars.ApplyKeystrokeAdd(newLen - prevLen)
	case newLen < prevLen:
		changed = m.stars.ApplyKeystrokeRemove(prevLen - newLen)
	}
	if !changed {
		return m, cmd
	}
	if m.stars.Active() && !m.ticking {
		m.ticking = true
		return m, tea.Batch(cmd, StarTick())
	}
	return m, cmd
}

func (m inputModel) View() tea.View {
	if m.cancelled {
		return tea.NewView("")
	}
	body := m.input.View()
	cursorOffset := 0
	if m.animateStars {
		body = m.stars.View()
		cursorOffset = m.stars.Len()
	}
	content := m.prefix + body
	if m.errMsg != "" {
		content += "\n" + promptErrorStyle.Render(m.errMsg)
	}
	indent := menulayout.Indent()
	content = menulayout.RenderIndent(content)
	v := tea.NewView(content)
	if c := m.input.Cursor(); c != nil {
		// Under EchoNone the textinput reports cursor X = len(value), which
		// would double-count once we add stars.Len(). Set X absolutely when
		// animating; otherwise add the offset as before.
		if m.animateStars {
			c.Position.X = m.prefixWidth + indent + cursorOffset
		} else {
			c.Position.X += m.prefixWidth + indent
		}
		v.Cursor = c
	}
	return v
}

func runInput(label string, password, animateStars bool) (string, error) {
	if !isTTY() {
		return "", errNoTTY
	}
	final, err := tea.NewProgram(newInputModel(label, password, animateStars)).Run()
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
	if password || animateStars {
		// Cell width, not bytes — matches textinput's EchoPassword.
		display = strings.Repeat("*", lipgloss.Width(val))
	}
	fmt.Print(menulayout.Render(fmt.Sprintf("%s: %s\n", label, display)))
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
	content := menulayout.RenderIndent(fmt.Sprintf("%s (y/n)", m.question))
	return tea.NewView(content)
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
	fmt.Print(menulayout.Render(fmt.Sprintf("%s (y/n) %s\n", question, ans)))
	return answer
}

func PromptForName(promptText string) (string, error) {
	return runInput(promptText, false, false)
}

func PromptForRecordPassword() (string, error) {
	for {
		first, err := runInput("Password", true, false)
		if err != nil {
			return "", err
		}
		repeat, err := runInput("Repeat password", true, false)
		if err != nil {
			return "", err
		}
		if first == repeat {
			return first, nil
		}
		fmt.Print(menulayout.Render(color.InYellow(passwordMismatchMsg) + "\n"))
	}
}

func PromptForMainPasswordChange() (string, error) {
	return promptForMainPassword(true, true)
}

func PromptForMainPassword(ensure bool) (string, error) {
	return promptForMainPassword(ensure, false)
}

var mainPasswordOverride string

// While set, the load-path main-password prompt returns this instead of asking.
// Skipped on change-main's "new password" prompt.
func SetMainPasswordOverride(pw string) { mainPasswordOverride = pw }
func ClearMainPasswordOverride()        { mainPasswordOverride = "" }

func promptForMainPassword(ensure bool, mainPasswordChange bool) (string, error) {
	if !mainPasswordChange && mainPasswordOverride != "" {
		return mainPasswordOverride, nil
	}
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
		first, err := runInput(askText, true, true)
		if err != nil {
			return "", err
		}
		if !ensure {
			return first, nil
		}
		repeat, err := runInput(repeatText, true, true)
		if err != nil {
			return "", err
		}
		if first == repeat {
			return first, nil
		}
		fmt.Print(menulayout.Render(color.InYellow(passwordMismatchMsg) + "\n"))
	}
}
