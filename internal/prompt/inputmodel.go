package prompt

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/tuiutil"
)

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

func (m InputModel) Init() tea.Cmd { return nil }

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
		// animating.
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
	final, err := tuiutil.Run(NewInputModel(label, password, animateStars))
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	if final.Cancelled() {
		return "", ErrPromptCancelled
	}
	val := final.Value()
	// Bubbletea wipes its render region on exit; re-emit to keep the answer in scrollback.
	display := val
	if password || animateStars {
		// Cell width, not bytes — matches textinput's EchoPassword.
		display = strings.Repeat("*", lipgloss.Width(val))
	}
	fmt.Printf("%s: %s\n", label, display)
	return val, nil
}

func PromptForName(promptText string) (string, error) {
	return runInput(promptText, false, false)
}

// PromptForSecretValue prompts with masking. No double-confirm — single-value
// entries are typed once.
func PromptForSecretValue(promptText string) (string, error) {
	return runInput(promptText, true, false)
}

func PromptForRecordPassword() (string, error) {
	return promptTwiceMatching("Password", "Repeat password", false)
}

// promptTwiceMatching asks for a secret twice and repeats both prompts until
// the two entries match.
func promptTwiceMatching(askText, repeatText string, animateStars bool) (string, error) {
	for {
		first, err := runInput(askText, true, animateStars)
		if err != nil {
			return "", err
		}
		repeat, err := runInput(repeatText, true, animateStars)
		if err != nil {
			return "", err
		}
		if first == repeat {
			return first, nil
		}
		fmt.Println(color.InYellow(PasswordMismatchMsg))
	}
}

// PromptMainPassword prompts for the main password and seals it. Intermediate
// textinput string lives until GC.
func PromptMainPassword(confirm bool) (*memguard.Enclave, error) {
	s, err := promptForMainPassword(confirm, false)
	if err != nil {
		return nil, err
	}
	return memguard.NewEnclave([]byte(s)), nil
}

// PromptMainPasswordChange is PromptMainPassword for the rotation flow.
func PromptMainPasswordChange() (*memguard.Enclave, error) {
	s, err := promptForMainPassword(true, true)
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

	if !confirm {
		return runInput(askText, true, true)
	}
	return promptTwiceMatching(askText, repeatText, true)
}
