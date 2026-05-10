package menu

import (
	"errors"
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
	phase    removePhase
	password string

	spinner    ui.SpinnerModel
	picker     storage.PickerModel
	store      *storage.Storage
	recordName string

	width, height int

	output    []string
	done      bool
	cancelled bool
}

func NewRemoveAction(password string) RemoveAction {
	return RemoveAction{
		phase:    removePhaseLoading,
		password: password,
		spinner:  ui.NewSpinnerModel("Decrypting"),
	}
}

func (a RemoveAction) Init() tea.Cmd {
	return tea.Batch(loadCmd(a.password, true), a.spinner.Init())
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
		a.picker = storage.NewPickerModel(names)
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
	cmd := tuiutil.UpdateInPlace(&a.picker, msg)
	if a.picker.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.picker.Done() {
		a.recordName = a.picker.Selection()
		a.store.RemoveRecord(a.recordName)
		a.spinner = ui.NewSpinnerModel("Saving")
		a.phase = removePhaseSaving
		return a, tea.Batch(saveCmd(a.store, "record removed"), a.spinner.Init())
	}
	return a, cmd
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
		return a.spinner.View()
	case removePhasePicking:
		return a.picker.View()
	}
	return tea.NewView("")
}

func (a RemoveAction) Done() bool         { return a.done }
func (a RemoveAction) Cancelled() bool    { return a.cancelled }
func (a RemoveAction) Output() []string   { return a.output }
func (a RemoveAction) NewPassword() string { return "" }

// humanizeLoadError swaps fork-undecryptable for its user-facing banner.
func humanizeLoadError(err error) string {
	if errors.Is(err, storage.ErrForkUndecryptable) {
		return storage.ForkUndecryptableUserMessage
	}
	return err.Error()
}
