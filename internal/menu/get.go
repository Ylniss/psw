package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"

	"github.com/ylniss/psw/internal/clipclean"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

type getPhase int

const (
	getPhaseLoading getPhase = iota
	getPhasePicking
)

type GetAction struct {
	phase    getPhase
	password string

	spinner ui.SpinnerModel
	picker  storage.PickerModel
	store   *storage.Storage

	width, height int

	output    []string
	done      bool
	cancelled bool
}

func NewGetAction(password string) GetAction {
	return GetAction{
		phase:    getPhaseLoading,
		password: password,
		spinner:  ui.NewSpinnerModel("Decrypting"),
	}
}

func (a GetAction) Init() tea.Cmd {
	return tea.Batch(loadCmd(a.password, false), a.spinner.Init())
}

func (a GetAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.phase {
	case getPhaseLoading:
		return a.updateLoading(msg)
	case getPhasePicking:
		return a.updatePicking(msg)
	}
	return a, nil
}

func (a GetAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = w.Width, w.Height
	}
	if m, ok := msg.(storageLoadedMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(humanizeLoadError(m.err)))
			a.done = true
			return a, nil
		}
		a.store = m.store
		names := m.store.GetNames()
		if len(names) == 0 {
			a.output = append(a.output, fmt.Sprintf("No secrets found. Use %s command first.", color.InCyan("add")))
			a.done = true
			return a, nil
		}
		a.picker = storage.NewPickerModel(names)
		if a.width > 0 && a.height > 0 {
			tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
		}
		a.phase = getPhasePicking
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a GetAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.picker, msg)
	if a.picker.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.picker.Done() {
		a.copyAndFinish(a.picker.Selection())
		return a, nil
	}
	return a, cmd
}

func (a *GetAction) copyAndFinish(name string) {
	record, ok := a.store.GetRecord(name)
	if !ok {
		a.output = append(a.output, fmt.Sprintf("Record %s was not found", color.InGreen(name)))
		a.done = true
		return
	}
	clipDuration := storage.AppConfig.ClipboardTimeout
	if record.Value == "" {
		if err := clipboard.WriteAll(record.Password); err != nil {
			a.output = append(a.output, fmt.Sprintf("Failed to copy value to clipboard: %s", err))
			a.done = true
			return
		}
		a.output = append(a.output,
			"Username",
			color.InYellow(record.Username),
			"",
			color.InYellow(fmt.Sprintf("Password copied to the clipboard, it will be cleared in %d seconds", clipDuration)),
		)
	} else {
		if err := clipboard.WriteAll(record.Value); err != nil {
			a.output = append(a.output, fmt.Sprintf("Failed to copy value to clipboard: %s", err))
			a.done = true
			return
		}
		a.output = append(a.output, color.InYellow(fmt.Sprintf("Value copied to the clipboard, it will be cleared in %d seconds", clipDuration)))
	}
	if err := clipclean.Spawn(clipDuration); err != nil {
		a.output = append(a.output, fmt.Sprintf("clipclean error: %s", err))
	}
	a.done = true
}

func (a GetAction) View() tea.View {
	switch a.phase {
	case getPhaseLoading:
		return a.spinner.View()
	case getPhasePicking:
		return a.picker.View()
	}
	return tea.NewView("")
}

func (a GetAction) Done() bool          { return a.done }
func (a GetAction) Cancelled() bool     { return a.cancelled }
func (a GetAction) Output() []string    { return a.output }
func (a GetAction) NewPassword() string { return "" }
