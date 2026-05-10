package menu

import (
	imgcolor "image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

const (
	logoFlashDuration = 250 * time.Millisecond
	historyCap        = 20
	// blank line after header + buttons row + blank line after buttons.
	extraChromeRows = 3
)

// chromeHeight is rows used by header + buttons + spacing. Action sub-view
// height = terminal height - chromeHeight.
var chromeHeight = lipgloss.Height(renderHeader(defaultHeaderColor)) + extraChromeRows

var menuActions = []string{"get", "add", "change", "remove"}

var (
	menuButtonStyle = lipgloss.NewStyle().Padding(0, 2)
	menuSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).Padding(0, 2)
	menuHelpStyle   = lipgloss.NewStyle().Faint(true)
	menuErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type Phase int

const (
	PhaseEnterPassword Phase = iota
	PhaseValidatingPassword
	PhaseSelectAction
	PhaseRunningAction
)

type MenuModel struct {
	phase         Phase
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

	// Output history; capped at historyCap blocks.
	history []string

	// Main password.
	password string
}

func NewMenuModel() MenuModel {
	return MenuModel{
		phase:         PhaseEnterPassword,
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
		if m.phase == PhaseRunningAction && m.activeAction != nil {
			adj := tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height - chromeHeight}
			cmd := tuiutil.UpdateInPlace(&m.activeAction, adj)
			return m, cmd
		}
		return m, nil

	case prompt.StarTickMsg:
		if m.phase != PhaseEnterPassword {
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
			m.phase = PhaseEnterPassword
			return m, m.passwordInput.Init()
		}
		m.passwordError = ""
		m.phase = PhaseSelectAction
		return m, nil

	case storageLoadedMsg, storageSavedMsg:
		if m.phase == PhaseRunningAction && m.activeAction != nil {
			return m.routeToAction(msg)
		}
		return m, nil
	}

	switch m.phase {
	case PhaseEnterPassword:
		return m.updateEnterPassword(msg)
	case PhaseValidatingPassword:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
		}
		cmd := tuiutil.UpdateInPlace(&m.passwordSpinner, msg)
		return m, cmd
	case PhaseSelectAction:
		return m.updateSelectAction(msg)
	case PhaseRunningAction:
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
		m.phase = PhaseValidatingPassword
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
		tuiutil.UpdateInPlace(&a, tea.WindowSizeMsg{Width: m.width, Height: m.height - chromeHeight})
	}
	m.activeAction = a
	m.phase = PhaseRunningAction
	return m, init
}

func (m MenuModel) routeToAction(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.activeAction == nil {
		m.phase = PhaseSelectAction
		return m, nil
	}
	cmd := tuiutil.UpdateInPlace(&m.activeAction, msg)
	if !(m.activeAction.Done() || m.activeAction.Cancelled()) {
		return m, cmd
	}
	if m.activeAction.Done() {
		out := strings.Join(m.activeAction.Output(), "\n")
		if out != "" {
			m.history = append(m.history, out)
			if len(m.history) > historyCap {
				m.history = m.history[len(m.history)-historyCap:]
			}
		}
		if pw := m.activeAction.NewPassword(); pw != "" {
			m.password = pw
		}
	}
	m.activeAction = nil
	m.phase = PhaseSelectAction
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
	if m.phase != PhaseEnterPassword && m.phase != PhaseValidatingPassword {
		m.renderButtons(&b)
		b.WriteString("\n\n")
	}

	var actionView tea.View
	pwErrHeight := 0
	yBeforeSubView := strings.Count(b.String(), "\n")
	switch m.phase {
	case PhaseEnterPassword:
		if m.passwordError != "" {
			rendered := wrapToHeader(m.width, menuErrStyle.Render(m.passwordError))
			b.WriteString(rendered)
			b.WriteString("\n")
			pwErrHeight = lipgloss.Height(rendered)
		}
		b.WriteString(indentToHeader(m.width, m.passwordInput.View().Content))
	case PhaseValidatingPassword:
		b.WriteString(indentToHeader(m.width, m.passwordSpinner.View().Content))
	case PhaseSelectAction:
		m.renderHistory(&b)
	case PhaseRunningAction:
		if m.activeAction != nil {
			actionView = m.activeAction.View()
			b.WriteString(indentToHeader(m.width, actionView.Content))
		}
	}
	m.writeFooterAtBottom(&b)
	v := tea.NewView(b.String())
	v.AltScreen = true

	// Bubble the sub-view's cursor up, offset for chrome rows and indent.
	xIndent := 0
	if m.width > pswHeaderWidth {
		xIndent = (m.width - pswHeaderWidth) / 2
	}
	switch m.phase {
	case PhaseEnterPassword:
		if src := m.passwordInput.View().Cursor; src != nil {
			c := *src
			c.Position.X += xIndent
			c.Position.Y += yBeforeSubView + pwErrHeight
			v.Cursor = &c
		}
	case PhaseRunningAction:
		if src := actionView.Cursor; src != nil {
			c := *src
			c.Position.X += xIndent
			c.Position.Y += yBeforeSubView
			v.Cursor = &c
		}
	}
	return v
}

func (m MenuModel) renderButtons(b *strings.Builder) {
	buttons := make([]string, len(menuActions))
	for i, a := range menuActions {
		if i == m.actionCursor {
			buttons[i] = menuSelectStyle.Render(a)
		} else {
			buttons[i] = menuButtonStyle.Render(a)
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	b.WriteString(centerHorizontally(m.width, row))
}

const footerHelp = "←/→ or h/l select · enter/j run · q/esc quit"

// writeFooterAtBottom pads with blank lines so the footer sits on the last
// terminal row. Falls back to one blank line when the content is too tall.
func (m MenuModel) writeFooterAtBottom(b *strings.Builder) {
	if m.phase != PhaseSelectAction || m.height == 0 {
		return
	}
	footer := centerHorizontally(m.width, menuHelpStyle.Render(footerHelp))
	used := lipgloss.Height(b.String())
	fh := lipgloss.Height(footer)
	pad := m.height - used - fh
	if pad < 1 {
		pad = 1
	}
	b.WriteString(strings.Repeat("\n", pad))
	b.WriteString(footer)
}

func (m MenuModel) renderHistory(b *strings.Builder) {
	for i, block := range m.history {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(wrapToHeader(m.width, block))
	}
}
