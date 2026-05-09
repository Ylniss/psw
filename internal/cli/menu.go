package cli

import (
	"fmt"
	imgcolor "image/color"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/menulayout"
	"github.com/ylniss/psw/internal/prompt"
	"golang.org/x/term"
)

const logoFlashDuration = 250 * time.Millisecond

var defaultHeaderColor = lipgloss.Color("6")

// Trailing whitespace on the last line is incidental; lipgloss.Width takes
// the max line width regardless.
const pswHeader = `
 ████████     █████   █████ ███ █████
░░███░░███   ███░░   ░░███ ░███░░███
 ░███ ░███  ░░█████   ░███ ░███ ░███
 ░███ ░███   ░░░░███  ░░███████████
 ░███████    ██████    ░░████░████
 ░███░░░    ░░░░░░      ░░░░ ░░░░
 ░███
 █████
░░░░░                             `

// Widest line of pswHeader. Header is hidden below this terminal width.
var pswHeaderWidth = lipgloss.Width(pswHeader)

var menuActions = []string{"get", "add", "change", "remove"}

var (
	menuButtonStyle = lipgloss.NewStyle().Padding(0, 2)
	menuSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).Padding(0, 2)
	menuHelpStyle   = lipgloss.NewStyle().Faint(true)
	menuErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// Default-color header is cached for the post-menu display; the live menu
// View renders its own (possibly flashed) version per frame.
var renderedHeader = renderHeader(defaultHeaderColor)

func renderHeader(c imgcolor.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render(pswHeader)
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

	// Indent matches the centered header's left edge so the dispatched action's
	// prompts align with it.
	indent := 0
	if m.width > pswHeaderWidth {
		indent = (m.width - pswHeaderWidth) / 2
	}

	// Clear the regular screen so prior shell scrollback isn't visible during
	// the dispatched action — makes menu→action feel continuous.
	fmt.Print(ansi.EraseDisplay(2) + ansi.CursorHomePosition)

	menulayout.Set(indent, pswHeaderWidth)
	defer menulayout.Clear()
	prompt.SetMainPasswordOverride(m.password)
	defer prompt.ClearMainPasswordOverride()

	fmt.Println(menulayout.RenderIndent(renderedHeader))
	fmt.Println()

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
	phase          menuPhase
	cursor         int
	chosenAction   string
	password       string
	passwordInput  textinput.Model
	passwordError  string
	cancelled      bool
	width          int
	stars          prompt.StarState
	ticking        bool
	logoFlashColor imgcolor.Color
	logoFlashUntil time.Time
}

func newMenuModel() menuModel {
	input := textinput.New()
	input.Prompt = ""
	input.EchoMode = textinput.EchoNone
	return menuModel{
		passwordInput: input,
		stars:         prompt.NewStarState(),
	}
}

func (m menuModel) Init() tea.Cmd { return textinput.Blink }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case prompt.StarTickMsg:
		if m.animationActive() {
			return m, prompt.StarTick()
		}
		m.ticking = false
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

func (m menuModel) animationActive() bool {
	return m.stars.Active() || time.Now().Before(m.logoFlashUntil)
}

// kickTick returns a tick cmd if animation is active and no tick is in flight.
// Mutates m.ticking via pointer receiver.
func (m *menuModel) kickTick() tea.Cmd {
	if m.ticking || !m.animationActive() {
		return nil
	}
	m.ticking = true
	return prompt.StarTick()
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
	// Logo flash on every typing keypress in this phase.
	m.logoFlashColor = m.stars.RandomHeaderColor()
	m.logoFlashUntil = time.Now().Add(logoFlashDuration)
	prevLen := len(m.passwordInput.Value())
	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	m.passwordError = ""
	newLen := len(m.passwordInput.Value())
	switch {
	case newLen == 0:
		m.stars.ApplyEmpty()
	case newLen > prevLen:
		m.stars.ApplyKeystrokeAdd(newLen - prevLen)
	case newLen < prevLen:
		m.stars.ApplyKeystrokeRemove(prevLen - newLen)
	}
	tickCmd := m.kickTick()
	if tickCmd == nil {
		return m, cmd
	}
	return m, tea.Batch(cmd, tickCmd)
}

func (m menuModel) View() tea.View {
	if m.cancelled || m.password != "" {
		return tea.NewView("")
	}
	var b strings.Builder
	if m.width == 0 || m.width >= pswHeaderWidth {
		headerColor := defaultHeaderColor
		if time.Now().Before(m.logoFlashUntil) {
			headerColor = m.logoFlashColor
		}
		b.WriteString(centerHorizontally(m.width, renderHeader(headerColor)))
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
	footer := menuHelpStyle.Render("←/→ or h/l select · enter/j run · q/esc quit")
	b.WriteString(centerHorizontally(m.width, footer))
}

func (m menuModel) renderEnterPassword(b *strings.Builder) {
	line := "Main password: " + m.stars.View()
	b.WriteString(indentToHeader(m.width, line))
	if m.passwordError != "" {
		b.WriteString("\n")
		b.WriteString(indentToHeader(m.width, menuErrStyle.Render(m.passwordError)))
	}
}

// indentToHeader left-pads content to align with the centered header's left
// edge. Returns content unchanged when termWidth ≤ pswHeaderWidth.
func indentToHeader(termWidth int, content string) string {
	if termWidth <= pswHeaderWidth {
		return content
	}
	indent := (termWidth - pswHeaderWidth) / 2
	return lipgloss.NewStyle().MarginLeft(indent).Render(content)
}

// Width is 0 before the terminal reports its size; PlaceHorizontal collapses
// content at width 0, so skip centering then.
func centerHorizontally(width int, content string) string {
	if width == 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}
