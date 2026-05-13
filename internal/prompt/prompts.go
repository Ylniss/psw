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
	"github.com/awnumar/memguard"
	"golang.org/x/term"

	"github.com/ylniss/psw/internal/tuiutil"
)

// ErrPromptCancelled means user pressed Esc/Ctrl-C. Callers exit silently.
var ErrPromptCancelled = errors.New("prompt cancelled")

// PasswordMismatchMsg is the user-facing line shown when password + repeat differ.
// Exported so the menu can render it with its own styling.
const PasswordMismatchMsg = "Passwords don't match, try again"

var (
	errRequired      = errors.New("input required")
	errNoTTY         = errors.New("interactive prompt required: stdin is not a terminal")
	promptErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func validateRequired(content string) error {
	if len(content) < 1 {
		return errRequired
	}
	return nil
}

// IsTTY reports whether stdin is an interactive terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// InputModel is a single-line text input with optional password masking and
// animated stars. Sets done/cancelled flags; never returns tea.Quit.
type InputModel struct {
	prefix       string
	prefixWidth  int
	input        textinput.Model
	errMsg       string
	done         bool
	cancelled    bool
	password     bool
	animateStars bool
	stars        StarState
	ticking      bool
}

func NewInputModel(label string, password, animateStars bool) InputModel {
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
	m := InputModel{
		prefix:       prefix,
		prefixWidth:  lipgloss.Width(prefix),
		input:        textInput,
		password:     password,
		animateStars: animateStars,
	}
	if animateStars {
		m.stars = NewStarState()
	}
	return m
}

func (m InputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, nil
		case "enter":
			if err := validateRequired(m.input.Value()); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.done = true
			return m, nil
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

func (m InputModel) View() tea.View {
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
	v := tea.NewView(content)
	if c := m.input.Cursor(); c != nil {
		// Under EchoNone the textinput reports cursor X = len(value), which
		// would double-count once we add stars.Len(). Set X absolutely when
		// animating; otherwise add the offset as before.
		if m.animateStars {
			c.Position.X = m.prefixWidth + cursorOffset
		} else {
			c.Position.X += m.prefixWidth
		}
		v.Cursor = c
	}
	return v
}

func (m InputModel) Done() bool        { return m.done }
func (m InputModel) Cancelled() bool   { return m.cancelled }
func (m InputModel) Value() string     { return m.input.Value() }
func (m InputModel) StarsActive() bool { return m.animateStars && m.stars.Active() }
func (m InputModel) Prefix() string    { return m.prefix }
func (m InputModel) Hidden() bool      { return m.password || m.animateStars }

// Reset clears value/error/done/cancelled. Prefix, mode, and animation persist.
func (m *InputModel) Reset() {
	m.input.SetValue("")
	m.errMsg = ""
	m.done = false
	m.cancelled = false
	if m.animateStars {
		m.stars.Reset()
	}
}

// WithInitialValue prefills the input. Plain inputs only — password/animated
// modes mask the prefill.
func (m InputModel) WithInitialValue(v string) InputModel {
	m.input.SetValue(v)
	return m
}

func runInput(label string, password, animateStars bool) (string, error) {
	if !IsTTY() {
		return "", errNoTTY
	}
	final, err := tea.NewProgram(tuiutil.QuittingWrapper[InputModel]{M: NewInputModel(label, password, animateStars)}).Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	quitter, ok := final.(tuiutil.QuittingWrapper[InputModel])
	if !ok {
		return "", fmt.Errorf("prompt returned unexpected model type %T", final)
	}
	if quitter.M.Cancelled() {
		return "", ErrPromptCancelled
	}
	val := quitter.M.Value()
	// Bubbletea wipes its render region on exit; re-emit to keep the answer in scrollback.
	display := val
	if password || animateStars {
		// Cell width, not bytes — matches textinput's EchoPassword.
		display = strings.Repeat("*", lipgloss.Width(val))
	}
	fmt.Printf("%s: %s\n", label, display)
	return val, nil
}

// YesNoModel is a y/n prompt. Sets done/cancelled flags; never returns tea.Quit.
type YesNoModel struct {
	question  string
	hint      string
	answer    bool
	done      bool
	cancelled bool
}

func NewYesNoModel(question string) YesNoModel {
	return YesNoModel{question: question}
}

// WithHint attaches a secondary line rendered below `(y/n)`. The caller is
// responsible for any styling (color codes etc.).
func (m YesNoModel) WithHint(hint string) YesNoModel {
	m.hint = hint
	return m
}

func (m YesNoModel) Init() tea.Cmd { return nil }

func (m YesNoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "y", "Y":
			m.answer = true
			m.done = true
			return m, nil
		case "n", "N":
			m.answer = false
			m.done = true
			return m, nil
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, nil
		}
	}
	return m, nil
}

func (m YesNoModel) View() tea.View {
	content := fmt.Sprintf("%s (y/n)", m.question)
	if m.hint != "" {
		content += "\n" + m.hint
	}
	return tea.NewView(content)
}

func (m YesNoModel) Done() bool       { return m.done }
func (m YesNoModel) Cancelled() bool  { return m.cancelled }
func (m YesNoModel) Answer() bool     { return m.answer }
func (m YesNoModel) Question() string { return m.question }

// YesOrNo returns false on Esc/Ctrl-C or non-TTY stdin (keeps scripts unblocked).
func YesOrNo(question string) bool {
	if !IsTTY() {
		return false
	}
	final, err := tea.NewProgram(tuiutil.QuittingWrapper[YesNoModel]{M: NewYesNoModel(question)}).Run()
	if err != nil {
		return false
	}
	quitter, ok := final.(tuiutil.QuittingWrapper[YesNoModel])
	if !ok {
		return false
	}
	answer := quitter.M.Done() && quitter.M.Answer()
	ans := "n"
	if answer {
		ans = "y"
	}
	fmt.Printf("%s (y/n) %s\n", question, ans)
	return answer
}

func PromptForName(promptText string) (string, error) {
	return runInput(promptText, false, false)
}

// PromptForSecretValue is PromptForName with masking. Single entry — no
// double-confirm: single-value records hold user-typed strings rare enough
// that a re-confirm prompt would cost more than it saves.
func PromptForSecretValue(promptText string) (string, error) {
	return runInput(promptText, true, false)
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
		fmt.Println(color.InYellow(PasswordMismatchMsg))
	}
}

func PromptForMainPasswordChange() (string, error) {
	return promptForMainPassword(true, true)
}

func PromptForMainPassword(confirm bool) (string, error) {
	return promptForMainPassword(confirm, false)
}

// PromptForMainPasswordEnclave wraps PromptForMainPassword and seals the
// result into a memguard Enclave. The intermediate string from the textinput
// is unscrubbable (lives until GC) — that's the documented memory-hygiene
// boundary.
func PromptForMainPasswordEnclave(confirm bool) (*memguard.Enclave, error) {
	s, err := PromptForMainPassword(confirm)
	if err != nil {
		return nil, err
	}
	return memguard.NewEnclave([]byte(s)), nil
}

// PromptForMainPasswordChangeEnclave is PromptForMainPasswordChange returning an Enclave.
func PromptForMainPasswordChangeEnclave() (*memguard.Enclave, error) {
	s, err := PromptForMainPasswordChange()
	if err != nil {
		return nil, err
	}
	return memguard.NewEnclave([]byte(s)), nil
}

func promptForMainPassword(confirm bool, mainPasswordChange bool) (string, error) {
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
		if !confirm {
			return first, nil
		}
		repeat, err := runInput(repeatText, true, true)
		if err != nil {
			return "", err
		}
		if first == repeat {
			return first, nil
		}
		fmt.Println(color.InYellow(PasswordMismatchMsg))
	}
}
