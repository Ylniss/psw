package menu

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/clipclean"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
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
	password *memguard.Enclave

	picker prompt.PickerModel
	store  *storage.Storage

	countdownEnd        time.Time
	remainingSeconds    int
	recordName          string
	isSingleValueRecord bool
}

func NewGetAction(password *memguard.Enclave) GetAction {
	return GetAction{
		baseAction: newBase("Decrypting"),
		phase:      getPhaseLoading,
		password:   password,
	}
}

func (a GetAction) Init() tea.Cmd {
	return tea.Batch(loadCmd(a.password), a.spinner.Init())
}

func (a GetAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	a.captureSize(msg)
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
	store, cmd := a.handleLoadingMsg(msg, a.password)
	if store == nil {
		return a, cmd
	}
	a.store = store
	names := store.GetNames()
	if len(names) == 0 {
		a.finish(fmt.Sprintf("No records yet. Use %s to create one.", color.InCyan("add")))
		return a, nil
	}
	a.picker = prompt.NewPickerModel(names, nil).WithoutHelp()
	a.sizePicker(&a.picker)
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
		if s := k.String(); s == "esc" || s == "ctrl+c" {
			a.clearOutputAndDone()
		}
		return a, nil
	}
	if _, ok := msg.(countdownTickMsg); !ok {
		return a, nil
	}
	remainingSeconds := int(time.Until(a.countdownEnd).Seconds() + 0.5)
	if remainingSeconds <= 0 {
		a.clearOutputAndDone()
		return a, nil
	}
	a.remainingSeconds = remainingSeconds
	return a, countdownTick()
}

// Empty output + Done → MenuModel.lastOutput cleared (routeToAction writes unconditionally).
func (a *GetAction) clearOutputAndDone() {
	a.output = nil
	a.done = true
}

// Returns the first tick cmd; nil on early exit (record/clipboard/clipclean error).
func (a *GetAction) copyAndStartCountdown(recordName string) tea.Cmd {
	record, ok := a.store.GetRecord(recordName)
	if !ok {
		a.finish(fmt.Sprintf("Record %s was not found", color.InGreen(recordName)))
		return nil
	}
	clipboardTimeoutSeconds := storage.AppConfig.ClipboardTimeoutSeconds
	a.isSingleValueRecord = len(record.Value) != 0
	clipboardText := string(record.Password)
	if a.isSingleValueRecord {
		clipboardText = string(record.Value)
	}
	if err := clipclean.CopyAndSchedule(clipboardText, clipboardTimeoutSeconds); err != nil {
		a.finish(err.Error())
		return nil
	}
	if !a.isSingleValueRecord {
		a.output = append(a.output,
			"Username",
			color.InYellow(record.Username),
			"",
		)
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
			color.InYellow(fmt.Sprintf(" copied — clears in %ds", a.remainingSeconds))
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

func (a GetAction) FooterHelp() string {
	switch a.phase {
	case getPhasePicking:
		return a.picker.Help()
	case getPhaseCountdown:
		return "esc clear now · wipes at 0"
	}
	return ""
}
