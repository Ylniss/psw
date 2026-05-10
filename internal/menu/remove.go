package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

type removePhase int

const (
	removePhaseLoading removePhase = iota
	removePhasePicking
	removePhaseSaving
)

type RemoveAction struct {
	baseAction

	phase    removePhase
	password string

	spinner    ui.SpinnerModel
	picker     storage.PickerModel
	store      *storage.Storage
	recordName string

	width, height int
}

func NewRemoveAction(password string) RemoveAction {
	return RemoveAction{
		phase:    removePhaseLoading,
		password: password,
		spinner:  ui.NewSpinnerModel("Syncing"),
	}
}

func (a RemoveAction) Init() tea.Cmd {
	return tea.Batch(pullCmd(a.password), a.spinner.Init())
}

func (a RemoveAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.phase {
	case removePhaseLoading:
		return a.updateLoading(msg)
	case removePhasePicking:
		return a.updatePicking(msg)
	case removePhaseSaving:
		return a.updateSaving(msg)
	}
	return a, nil
}

func (a RemoveAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = w.Width, w.Height
	}
	if m, ok := msg.(pullDoneMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(humanizeLoadError(m.err)))
			a.done = true
			return a, nil
		}
		a.transcript = append(a.transcript, m.warnings...)
		a.spinner = ui.NewSpinnerModel("Decrypting")
		return a, tea.Batch(decryptCmd(a.password), a.spinner.Init())
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
			a.output = append(a.output, "No records to remove.")
			a.done = true
			return a, nil
		}
		a.picker = storage.NewPickerModel(names, nil).WithoutHelp()
		if a.width > 0 && a.height > 0 {
			tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
		}
		a.phase = removePhasePicking
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a RemoveAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	sel, ok, cmd := a.stepPicker(&a.picker, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.recordName = sel
	a.store.RemoveRecord(a.recordName)
	a.spinner = ui.NewSpinnerModel("Saving")
	a.phase = removePhaseSaving
	return a, tea.Batch(saveCmd(a.store, "record removed"), a.spinner.Init())
}

func (a RemoveAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(storageSavedMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(m.err.Error()))
			a.done = true
			return a, nil
		}
		a.output = append(a.output, fmt.Sprintf("Record %s successfully removed", color.InGreen(a.recordName)))
		a.done = true
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a RemoveAction) View() tea.View {
	switch a.phase {
	case removePhaseLoading, removePhaseSaving:
		return prependTranscript(a.spinner.View(), a.transcript)
	case removePhasePicking:
		return prependTranscript(a.picker.View(), a.transcript)
	}
	return tea.NewView("")
}

func (a RemoveAction) NewPassword() string { return "" }
func (a RemoveAction) FooterHelp() string {
	if a.phase == removePhasePicking {
		return a.picker.Help()
	}
	return ""
}
