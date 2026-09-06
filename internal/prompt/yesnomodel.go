package prompt

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/tuiutil"
)

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
	final, err := tuiutil.Run(NewYesNoModel(question))
	if err != nil {
		return false
	}
	answer := final.Done() && final.Answer()
	answerText := "n"
	if answer {
		answerText = "y"
	}
	fmt.Printf("%s (y/n) %s\n", question, answerText)
	return answer
}
