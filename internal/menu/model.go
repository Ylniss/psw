package menu

import (
	"fmt"
	imgcolor "image/color"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

const logoFlashDuration = 250 * time.Millisecond

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
	headerStars    prompt.StarState
	tickPending    bool
	logoFlashColor imgcolor.Color
	logoFlashUntil time.Time

	// Action select.
	actionCursor int

	// Running an action.
	activeAction Action

	lastOutput string

	// Main password.
	password *memguard.Enclave
}

func NewMenuModel() MenuModel {
	return MenuModel{
		phase:         menuPhaseEnterPassword,
		passwordInput: prompt.NewInputModel("Main password", true, true),
		headerStars:   prompt.NewStarState(),
	}
}

func (m MenuModel) Init() tea.Cmd { return m.passwordInput.Init() }

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.phase == menuPhaseRunningAction && m.activeAction != nil {
			adj := tea.WindowSizeMsg{Width: contentColumnWidth(msg.Width), Height: msg.Height - actionFrameHeight}
			cmd := tuiutil.UpdateInPlace(&m.activeAction, adj)
			return m, cmd
		}
		return m, nil

	case prompt.StarTickMsg:
		if m.phase != menuPhaseEnterPassword {
			m.tickPending = false
			return m, nil
		}
		tuiutil.UpdateInPlace(&m.passwordInput, msg)
		if m.headerAnimationActive() || m.passwordInput.StarsActive() {
			m.tickPending = true
			return m, prompt.StarTick()
		}
		m.tickPending = false
		return m, nil

	case passwordValidatedMsg:
		if msg.err != nil {
			m.passwordError = msg.err.Error()
			m.passwordInput.Reset()
			m.password = nil
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
	return m.headerStars.Active() || time.Now().Before(m.logoFlashUntil)
}

func (m *MenuModel) scheduleHeaderTick() tea.Cmd {
	if m.tickPending || !m.headerAnimationActive() {
		return nil
	}
	m.tickPending = true
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
			m.logoFlashColor = m.headerStars.RandomHeaderColor()
			m.logoFlashUntil = time.Now().Add(logoFlashDuration)
		}
	}
	cmd := tuiutil.UpdateInPlace(&m.passwordInput, msg)
	if m.passwordInput.Cancelled() {
		return m, tea.Quit
	}
	if m.passwordInput.Done() {
		m.password = memguard.NewEnclave([]byte(m.passwordInput.Value()))
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
		if m.actionCursor-menuGridRows >= 0 {
			m.actionCursor -= menuGridRows
		}
	case "right", "l":
		if m.actionCursor+menuGridRows < len(menuEntries) {
			m.actionCursor += menuGridRows
		}
	case "up", "k", "shift+tab":
		m.actionCursor = (m.actionCursor - 1 + len(menuEntries)) % len(menuEntries)
	case "down", "j", "tab":
		m.actionCursor = (m.actionCursor + 1) % len(menuEntries)
	case "1", "2", "3", "4", "5", "6":
		idx := int(k.String()[0] - '1')
		if idx < len(menuEntries) {
			m.actionCursor = idx
			return m.startAction(idx)
		}
	case "enter", "space":
		return m.startAction(m.actionCursor)
	}
	return m, nil
}

func (m MenuModel) startAction(i int) (tea.Model, tea.Cmd) {
	a := menuEntries[i].new(m.password)
	init := a.Init()
	// Action's picker needs a size before its first render or the list is empty.
	if m.width > 0 && m.height > 0 {
		tuiutil.UpdateInPlace(&a, tea.WindowSizeMsg{Width: contentColumnWidth(m.width), Height: m.height - actionFrameHeight})
	}
	m.activeAction = a
	m.phase = menuPhaseRunningAction
	return m, init
}

func (m MenuModel) routeToAction(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&m.activeAction, msg)
	if !(m.activeAction.Done() || m.activeAction.Cancelled()) {
		return m, cmd
	}
	if m.activeAction.Done() {
		m.lastOutput = strings.Join(m.activeAction.Output(), "\n")
		if pw := m.activeAction.NewPassword(); pw != nil {
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
			rendered, indent := alignBlock(m.width, contentColumnLeft(m.width), actionView.Content)
			b.WriteString(rendered)
			actionIndent = indent
		}
	}
	m.writeFooterAtBottom(&b)
	v := tea.NewView(b.String())
	v.AltScreen = true

	// Bubble the sub-view's cursor up, offset for chrome rows and indent.
	headerIndent := headerColumnLeft(m.width)
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
	rendered := make([]string, len(menuEntries))
	widths := make([]int, len(menuEntries))
	unselectedStyle := menuButtonStyle
	if m.phase == menuPhaseRunningAction {
		unselectedStyle = menuButtonStyleFaint
	}
	for i, e := range menuEntries {
		label := fmt.Sprintf("[%d] %s", i+1, e.name)
		if i == m.actionCursor {
			rendered[i] = menuSelectStyle.Render(buttonPrefixSelected + label)
		} else {
			rendered[i] = unselectedStyle.Render(buttonPrefixBlank + label)
		}
		widths[i] = lipgloss.Width(rendered[i])
	}
	columnWidth := contentColumnWidth(m.width)

	rows := make([]string, menuGridRows)
	for r := 0; r < menuGridRows; r++ {
		// menuEntries is column-major: index = col*menuGridRows + row —
		// the same arithmetic updateSelectAction uses for left/right.
		btnIdxs := make([]int, menuGridCols)
		for c := range btnIdxs {
			btnIdxs[c] = c*menuGridRows + r
		}
		rows[r] = layoutRowSpaceBetween(rendered, widths, btnIdxs, columnWidth)
	}
	grid := lipgloss.JoinVertical(lipgloss.Left, rows...)
	// MarginLeft anchors the grid at the column's left edge — same anchor
	// settings rows use, so leftmost/rightmost visible content align.
	b.WriteString(lipgloss.NewStyle().MarginLeft(contentColumnLeft(m.width)).Render(grid))
}

// writeFooterAtBottom pads with blank lines so the footer sits on the last
// terminal row. Minimum one blank line when content overflows.
func (m MenuModel) writeFooterAtBottom(b *strings.Builder) {
	if m.height == 0 {
		return
	}
	help := m.footerHelpText()
	if help == "" {
		return
	}
	footer := indentToFooter(m.width, menuHelpStyle.Render(help))
	used := lipgloss.Height(b.String())
	fh := lipgloss.Height(footer)
	pad := max(m.height-used-fh, 1)
	b.WriteString(strings.Repeat("\n", pad))
	b.WriteString(footer)
}

func (m MenuModel) footerHelpText() string {
	switch m.phase {
	case menuPhaseSelectAction:
		return selectActionHelp
	case menuPhaseRunningAction:
		if m.activeAction != nil {
			return m.activeAction.FooterHelp()
		}
	}
	return ""
}

func (m MenuModel) renderLastOutput(b *strings.Builder) {
	if m.lastOutput == "" {
		return
	}
	b.WriteString(wrapToContent(m.width, m.lastOutput))
}
