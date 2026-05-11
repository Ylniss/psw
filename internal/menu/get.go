package menu

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"

	"github.com/ylniss/psw/internal/clipclean"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
)

type getPhase int

const (
	getPhaseLoading getPhase = iota
	getPhasePicking
	getPhaseCountdown
)

type GetAction struct {
	baseAction

	phase    getPhase
	password string

	picker storage.PickerModel
	store  *storage.Storage

	countdownEnd        time.Time
	remainingSeconds    int
	recordName          string
	isSingleValueRecord bool

	width, height int
}

func NewGetAction(password string) GetAction {
	return GetAction{
		baseAction: newBase("Decrypting"),
		phase:      getPhaseLoading,
		password:   password,
	}
}

func (a GetAction) Init() tea.Cmd {
	return tea.Batch(loadCmd(a.password, false), a.spinner.Init())
}

func (a GetAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = w.Width, w.Height
	}
	switch a.phase {
	case getPhaseLoading:
		return a.updateLoading(msg)
	case getPhasePicking:
		return a.updatePicking(msg)
	case getPhaseCountdown:
		return a.updateCountdown(msg)
	}
	return a, nil
}

func (a GetAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	store, done, cmd := a.handleLoadingMsg(msg, a.password)
	if done || store == nil {
		return a, cmd
	}
	a.store = store
	names := store.GetNames()
	if len(names) == 0 {
		a.finish(fmt.Sprintf("No secrets found. Use %s command first.", color.InCyan("add")))
		return a, nil
	}
	a.picker = storage.NewPickerModel(names, nil).WithoutHelp()
	if a.width > 0 && a.height > 0 {
		tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
	}
	a.phase = getPhasePicking
	return a, nil
}

func (a GetAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	sel, ok, cmd := a.stepPicker(&a.picker, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	tickCmd := a.copyAndStartCountdown(sel)
	return a, tickCmd
}

func (a GetAction) updateCountdown(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc", "ctrl+c":
			a.wipeAndDone()
			return a, nil
		}
		return a, nil
	}
	if _, ok := msg.(countdownTickMsg); !ok {
		return a, nil
	}
	remainingSeconds := int(time.Until(a.countdownEnd).Seconds() + 0.5)
	if remainingSeconds <= 0 {
		a.wipeAndDone()
		return a, nil
	}
	a.remainingSeconds = remainingSeconds
	return a, countdownTick()
}

// Empty output + Done → MenuModel.lastOutput cleared (routeToAction writes unconditionally).
func (a *GetAction) wipeAndDone() {
	a.output = nil
	a.done = true
}

// tea.Every (not Tick) so the displayed seconds align with the wall clock.
func countdownTick() tea.Cmd {
	return tea.Every(time.Second, func(time.Time) tea.Msg { return countdownTickMsg{} })
}

// Returns the first tick cmd; nil on early exit (record/clipboard/clipclean error).
func (a *GetAction) copyAndStartCountdown(recordName string) tea.Cmd {
	record, ok := a.store.GetRecord(recordName)
	if !ok {
		a.finish(fmt.Sprintf("Record %s was not found", color.InGreen(recordName)))
		return nil
	}
	clipboardTimeoutSeconds := storage.AppConfig.ClipboardTimeoutSeconds
	var clipboardText string
	if record.Value == "" {
		clipboardText = record.Password
		a.isSingleValueRecord = false
	} else {
		clipboardText = record.Value
		a.isSingleValueRecord = true
	}
	if err := clipboard.WriteAll(clipboardText); err != nil {
		a.finish(fmt.Sprintf("Failed to copy value to clipboard: %s", err))
		return nil
	}
	if !a.isSingleValueRecord {
		a.output = append(a.output,
			"Username",
			color.InYellow(record.Username),
			"",
		)
	}
	if err := clipclean.Spawn(clipboardTimeoutSeconds); err != nil {
		a.output = append(a.output, fmt.Sprintf("clipclean error: %s", err))
		a.done = true
		return nil
	}
	a.recordName = recordName
	a.countdownEnd = time.Now().Add(time.Duration(clipboardTimeoutSeconds) * time.Second)
	a.remainingSeconds = clipboardTimeoutSeconds
	a.phase = getPhaseCountdown
	return countdownTick()
}

func (a GetAction) View() tea.View {
	switch a.phase {
	case getPhaseLoading:
		return a.spinner.View()
	case getPhasePicking:
		return a.picker.View()
	case getPhaseCountdown:
		secretLabel := "Password"
		if a.isSingleValueRecord {
			secretLabel = "Value"
		}
		// Yellow each segment — InCyan's trailing reset would kill an outer yellow wrap.
		countdownLine := color.InYellow(secretLabel+" for ") +
			color.InCyan(a.recordName) +
			color.InYellow(fmt.Sprintf(" copied to the clipboard, it will be cleared in %d seconds", a.remainingSeconds))
		lines := append([]string(nil), a.output...)
		lines = append(lines, countdownLine)
		content := strings.Join(lines, "\n")
		if a.width > 0 {
			content = lipgloss.Wrap(content, a.width, " ")
		}
		return tea.NewView(content)
	}
	return tea.NewView("")
}

func (a GetAction) NewPassword() string { return "" }
func (a GetAction) FooterHelp() string {
	switch a.phase {
	case getPhasePicking:
		return a.picker.Help()
	case getPhaseCountdown:
		return "esc clear now · wipes at 0"
	}
	return ""
}
