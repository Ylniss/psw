package cli

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
	"golang.org/x/term"
)

const pswHeader = `
 ████████   █████  █████ ███ █████
░░███░░███ ███░░  ░░███ ░███░░███
 ░███ ░███░░█████  ░███ ░███ ░███
 ░███ ░███ ░░░░███ ░░███████████
 ░███████  ██████   ░░████░████
 ░███░░░  ░░░░░░     ░░░░ ░░░░
 ░███
 █████
░░░░░                             `

// Widest line of pswHeader. Below this width the header is hidden.
const pswHeaderWidth = 34

var menuActions = []string{"get", "add", "change", "remove"}

var (
	menuHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	menuButtonStyle = lipgloss.NewStyle().Padding(0, 2)
	menuSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).Padding(0, 2)
	menuHelpStyle   = lipgloss.NewStyle().Faint(true)
	menuErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// Cached so View() doesn't re-style on every redraw.
var renderedHeader = menuHeaderStyle.Render(pswHeader)

func init() {
	rootCmd.AddCommand(menuCmd)
}

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive launcher for get/add/change/remove",
	Long:  "Show an interactive menu, then run the selected subcommand. Designed for terminals spawned on a hotkey (e.g. foot under niri).",
	Args:  cobra.NoArgs,
	RunE:  runMenu,
}

func runMenu(cmd *cobra.Command, args []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println(color.InRed("psw menu requires an interactive terminal"))
		return errExit
	}

	final, err := tea.NewProgram(newMenuModel()).Run()
	if err != nil {
		return fmt.Errorf("menu: %w", err)
	}
	m, ok := final.(menuModel)
	if !ok || m.cancelled || m.chosenAction == "" || m.password == "" {
		return nil
	}

	// Alt-screen exit wiped the menu render; re-emit so the header stays in scrollback.
	fmt.Println(renderedHeader)
	fmt.Println()

	prompt.SetMainPasswordOverride(m.password)
	defer prompt.ClearMainPasswordOverride()

	selected := subcommandFor(m.chosenAction)
	if selected == nil {
		return nil
	}
	return selected.RunE(selected, nil)
}

func subcommandFor(action string) *cobra.Command {
	switch action {
	case "get":
		return getCmd
	case "add":
		return addCmd
	case "change":
		return changeCmd
	case "remove":
		return removeCmd
	}
	return nil
}

type menuPhase int

const (
	phaseSelectAction menuPhase = iota
	phaseEnterPassword
)

type menuModel struct {
	phase         menuPhase
	cursor        int
	chosenAction  string
	password      string
	passwordInput textinput.Model
	passwordError string
	cancelled     bool
	width         int
}

func newMenuModel() menuModel {
	input := textinput.New()
	input.Prompt = ""
	input.EchoMode = textinput.EchoPassword
	return menuModel{passwordInput: input}
}

func (m menuModel) Init() tea.Cmd { return textinput.Blink }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyPressMsg:
		switch m.phase {
		case phaseSelectAction:
			return m.updateSelectAction(msg)
		case phaseEnterPassword:
			return m.updateEnterPassword(msg)
		}
	}
	if m.phase == phaseEnterPassword {
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m menuModel) updateSelectAction(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled = true
		return m, tea.Quit
	case "left", "h":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right", "l":
		if m.cursor < len(menuActions)-1 {
			m.cursor++
		}
	case "enter", "j":
		m.chosenAction = menuActions[m.cursor]
		m.phase = phaseEnterPassword
		return m, m.passwordInput.Focus()
	}
	return m, nil
}

func (m menuModel) updateEnterPassword(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "enter":
		if m.passwordInput.Value() == "" {
			m.passwordError = "Password required"
			return m, nil
		}
		m.password = m.passwordInput.Value()
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	m.passwordError = ""
	return m, cmd
}

func (m menuModel) View() tea.View {
	if m.cancelled || m.password != "" {
		return tea.NewView("")
	}
	var b strings.Builder
	if m.width == 0 || m.width >= pswHeaderWidth {
		b.WriteString(centerHorizontally(m.width, renderedHeader))
		b.WriteString("\n\n")
	}
	switch m.phase {
	case phaseSelectAction:
		m.renderSelectAction(&b)
	case phaseEnterPassword:
		m.renderEnterPassword(&b)
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m menuModel) renderSelectAction(b *strings.Builder) {
	buttons := make([]string, len(menuActions))
	for i, a := range menuActions {
		if i == m.cursor {
			buttons[i] = menuSelectStyle.Render(a)
		} else {
			buttons[i] = menuButtonStyle.Render(a)
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	b.WriteString(centerHorizontally(m.width, row))
	b.WriteString("\n\n")
	footer := menuHelpStyle.Render("←/→ or h/l select · enter or j run · q/esc quit")
	b.WriteString(centerHorizontally(m.width, footer))
}

func (m menuModel) renderEnterPassword(b *strings.Builder) {
	line := "Main password: " + m.passwordInput.View()
	b.WriteString(centerHorizontally(m.width, line))
	if m.passwordError != "" {
		b.WriteString("\n")
		b.WriteString(centerHorizontally(m.width, menuErrStyle.Render(m.passwordError)))
	}
}

// Width is 0 before the terminal reports its size; PlaceHorizontal collapses
// content at width 0, so skip centering then.
func centerHorizontally(width int, content string) string {
	if width == 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}
