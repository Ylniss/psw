package menu

import (
	"fmt"
	imgcolor "image/color"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

const logoFlashDuration = 250 * time.Millisecond

// actionChromeHeight is the rows above an action sub-view (header + spacers
// + buttons row). Computed by rendering the same shape View() emits, so the
// constant tracks layout changes automatically.
var actionChromeHeight = lipgloss.Height(renderActionChrome())

func renderActionChrome() string {
	var b strings.Builder
	b.WriteString(renderHeader(defaultHeaderColor))
	b.WriteString("\n\n")
	b.WriteString(menuButtonStyle.Render("[1] x"))
	b.WriteString("\n\n")
	return b.String()
}

var menuActions = []string{"get", "add", "change", "remove"}

var (
	menuButtonStyle = lipgloss.NewStyle().Padding(0, 2)
	menuSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).Padding(0, 2)
	menuHelpStyle   = lipgloss.NewStyle().Faint(true)
	menuErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type menuPhase int

const (
	menuPhaseEnterPassword menuPhase = iota
	menuPhaseValidatingPassword
	menuPhaseSelectAction
	menuPhaseRunningAction
)

type MenuModel struct {
	phase         menuPhase
	width, height int

	// Password phase.
	passwordInput   prompt.InputModel
	passwordError   string
	passwordSpinner ui.SpinnerModel

	// Header animation.
	stars          prompt.StarState
	tickInFlight   bool
	logoFlashColor imgcolor.Color
	logoFlashUntil time.Time

	// Action select.
	actionCursor int

	// Running an action.
	activeAction Action

	lastOutput string

	// Main password.
	password string
}

func NewMenuModel() MenuModel {
	return MenuModel{
		phase:         menuPhaseEnterPassword,
		passwordInput: prompt.NewInputModel("Main password", true, true),
		stars:         prompt.NewStarState(),
	}
}

func (m MenuModel) Init() tea.Cmd { return m.passwordInput.Init() }

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.phase == menuPhaseRunningAction && m.activeAction != nil {
			adj := tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height - actionChromeHeight}
			cmd := tuiutil.UpdateInPlace(&m.activeAction, adj)
			return m, cmd
		}
		return m, nil

	case prompt.StarTickMsg:
		if m.phase != menuPhaseEnterPassword {
			m.tickInFlight = false
			return m, nil
		}
		tuiutil.UpdateInPlace(&m.passwordInput, msg)
		if m.headerAnimationActive() || m.passwordInput.StarsActive() {
			m.tickInFlight = true
			return m, prompt.StarTick()
		}
		m.tickInFlight = false
		return m, nil

	case passwordValidatedMsg:
		if msg.err != nil {
			m.passwordError = msg.err.Error()
			m.passwordInput.Reset()
			m.password = ""
			m.phase = menuPhaseEnterPassword
			return m, m.passwordInput.Init()
		}
		m.passwordError = ""
		m.phase = menuPhaseSelectAction
		return m, nil

	case storageLoadedMsg, storageSavedMsg:
		if m.phase == menuPhaseRunningAction && m.activeAction != nil {
			return m.routeToAction(msg)
		}
		// Late delivery: action already finished or never ran. Drop quietly.
		slog.Debug("menu: dropping stale storage msg", "phase", m.phase, "msg", fmt.Sprintf("%T", msg))
		return m, nil
	}

	switch m.phase {
	case menuPhaseEnterPassword:
		return m.updateEnterPassword(msg)
	case menuPhaseValidatingPassword:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
		}
		cmd := tuiutil.UpdateInPlace(&m.passwordSpinner, msg)
		return m, cmd
	case menuPhaseSelectAction:
		return m.updateSelectAction(msg)
	case menuPhaseRunningAction:
		return m.routeToAction(msg)
	}
	return m, nil
}

func (m MenuModel) headerAnimationActive() bool {
	return m.stars.Active() || time.Now().Before(m.logoFlashUntil)
}

func (m *MenuModel) scheduleHeaderTick() tea.Cmd {
	if m.tickInFlight || !m.headerAnimationActive() {
		return nil
	}
	m.tickInFlight = true
	return prompt.StarTick()
}

func (m MenuModel) updateEnterPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
		// Flash the logo on each typed key. Enter is the phase exit, so skip it.
		if k.String() != "enter" {
			m.logoFlashColor = m.stars.RandomHeaderColor()
			m.logoFlashUntil = time.Now().Add(logoFlashDuration)
		}
	}
	cmd := tuiutil.UpdateInPlace(&m.passwordInput, msg)
	if m.passwordInput.Cancelled() {
		return m, tea.Quit
	}
	if m.passwordInput.Done() {
		m.password = m.passwordInput.Value()
		m.passwordError = ""
		m.passwordSpinner = ui.NewSpinnerModel("Decrypting")
		m.phase = menuPhaseValidatingPassword
		return m, tea.Batch(validatePasswordCmd(m.password), m.passwordSpinner.Init())
	}
	tickCmd := m.scheduleHeaderTick()
	if tickCmd == nil {
		return m, cmd
	}
	return m, tea.Batch(cmd, tickCmd)
}

func (m MenuModel) updateSelectAction(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "left", "h":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "right", "l":
		if m.actionCursor < len(menuActions)-1 {
			m.actionCursor++
		}
	case "1", "2", "3", "4":
		idx := int(k.String()[0] - '1')
		if idx < len(menuActions) {
			m.actionCursor = idx
			return m.startAction(menuActions[idx])
		}
	case "enter", "j":
		return m.startAction(menuActions[m.actionCursor])
	}
	return m, nil
}

func (m MenuModel) startAction(name string) (tea.Model, tea.Cmd) {
	a, init := newAction(name, m.password)
	if a == nil {
		return m, nil
	}
	// Action's picker needs a size before its first render or the list is empty.
	if m.width > 0 && m.height > 0 {
		tuiutil.UpdateInPlace(&a, tea.WindowSizeMsg{Width: m.width, Height: m.height - actionChromeHeight})
	}
	m.activeAction = a
	m.phase = menuPhaseRunningAction
	return m, init
}

func (m MenuModel) routeToAction(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.activeAction == nil {
		m.phase = menuPhaseSelectAction
		return m, nil
	}
	cmd := tuiutil.UpdateInPlace(&m.activeAction, msg)
	if !(m.activeAction.Done() || m.activeAction.Cancelled()) {
		return m, cmd
	}
	if m.activeAction.Done() {
		if out := strings.Join(m.activeAction.Output(), "\n"); out != "" {
			m.lastOutput = out
		}
		if pw := m.activeAction.NewPassword(); pw != "" {
			m.password = pw
		}
	}
	m.activeAction = nil
	m.phase = menuPhaseSelectAction
	return m, cmd
}

func (m MenuModel) View() tea.View {
	var b strings.Builder
	if m.width == 0 || m.width >= pswHeaderWidth {
		headerColor := defaultHeaderColor
		if time.Now().Before(m.logoFlashUntil) {
			headerColor = m.logoFlashColor
		}
		b.WriteString(centerHorizontally(m.width, renderHeader(headerColor)))
		b.WriteString("\n\n")
	}
	// Buttons hidden until the password is validated.
	if m.phase != menuPhaseEnterPassword && m.phase != menuPhaseValidatingPassword {
		m.renderButtons(&b)
		b.WriteString("\n\n")
	}

	var actionView tea.View
	pwErrHeight := 0
	yBeforeSubView := strings.Count(b.String(), "\n")
	actionIndent := 0
	switch m.phase {
	case menuPhaseEnterPassword:
		if m.passwordError != "" {
			rendered := wrapToHeader(m.width, menuErrStyle.Render(m.passwordError))
			b.WriteString(rendered)
			b.WriteString("\n")
			pwErrHeight = lipgloss.Height(rendered)
		}
		b.WriteString(indentToHeader(m.width, m.passwordInput.View().Content))
	case menuPhaseValidatingPassword:
		b.WriteString(indentToHeader(m.width, m.passwordSpinner.View().Content))
	case menuPhaseSelectAction:
		m.renderLastOutput(&b)
	case menuPhaseRunningAction:
		if m.activeAction != nil {
			actionView = m.activeAction.View()
			rendered, indent := alignBlock(m.width, firstButtonLeftCol(m.width), actionView.Content)
			b.WriteString(rendered)
			actionIndent = indent
			if help := m.activeAction.FooterHelp(); help != "" {
				b.WriteString("\n\n")
				b.WriteString(indentToFooter(m.width, menuHelpStyle.Render(help)))
			}
		}
	}
	m.writeFooterAtBottom(&b)
	v := tea.NewView(b.String())
	v.AltScreen = true

	// Bubble the sub-view's cursor up, offset for chrome rows and indent.
	headerIndent := 0
	if m.width > pswHeaderWidth {
		headerIndent = (m.width - pswHeaderWidth) / 2
	}
	switch m.phase {
	case menuPhaseEnterPassword:
		if src := m.passwordInput.View().Cursor; src != nil {
			c := *src
			c.Position.X += headerIndent
			c.Position.Y += yBeforeSubView + pwErrHeight
			v.Cursor = &c
		}
	case menuPhaseRunningAction:
		if src := actionView.Cursor; src != nil {
			c := *src
			c.Position.X += actionIndent
			c.Position.Y += yBeforeSubView
			v.Cursor = &c
		}
	}
	return v
}

func (m MenuModel) renderButtons(b *strings.Builder) {
	buttons := make([]string, len(menuActions))
	for i, a := range menuActions {
		label := fmt.Sprintf("[%d] %s", i+1, a)
		if i == m.actionCursor {
			buttons[i] = menuSelectStyle.Render(label)
		} else {
			buttons[i] = menuButtonStyle.Render(label)
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	b.WriteString(centerHorizontally(m.width, row))
}

// buttonsRowWidth measures the rendered buttons row so action sub-views can
// align under [1] regardless of changes to button labels.
func buttonsRowWidth() int {
	buttons := make([]string, len(menuActions))
	for i, a := range menuActions {
		label := fmt.Sprintf("[%d] %s", i+1, a)
		buttons[i] = menuButtonStyle.Render(label)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	return lipgloss.Width(row)
}

// firstButtonLeftCol returns the terminal column where [1]'s leftmost char
// renders: buttons row left edge plus the button style's left padding.
func firstButtonLeftCol(termWidth int) int {
	leftEdge := (termWidth - buttonsRowWidth()) / 2
	if leftEdge < 0 {
		leftEdge = 0
	}
	return leftEdge + menuButtonStyle.GetPaddingLeft()
}

const footerHelp = "←→/1-4 select · enter run · esc quit"

// writeFooterAtBottom pads with blank lines so the footer sits on the last
// terminal row, aligned to the header column like the rest of the UI.
// Falls back to one blank line when the content is too tall.
func (m MenuModel) writeFooterAtBottom(b *strings.Builder) {
	if m.phase != menuPhaseSelectAction || m.height == 0 {
		return
	}
	footer := indentToFooter(m.width, menuHelpStyle.Render(footerHelp))
	used := lipgloss.Height(b.String())
	fh := lipgloss.Height(footer)
	pad := m.height - used - fh
	if pad < 1 {
		pad = 1
	}
	b.WriteString(strings.Repeat("\n", pad))
	b.WriteString(footer)
}

func (m MenuModel) renderLastOutput(b *strings.Builder) {
	if m.lastOutput == "" {
		return
	}
	b.WriteString(wrapToHeader(m.width, m.lastOutput))
}
