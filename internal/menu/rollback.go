package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
)

type rollbackPhase int

const (
	rollbackPhaseLoading rollbackPhase = iota
	rollbackPhasePicking
	rollbackPhaseConfirming
	rollbackPhaseSaving
)

type RollbackAction struct {
	baseAction

	phase    rollbackPhase
	password *memguard.Enclave

	store        *storage.Storage
	picker       prompt.PickerModel
	entryByLabel map[string]storage.LogEntry
	target       storage.LogEntry
	records      []storage.Record
}

func NewRollbackAction(password *memguard.Enclave) RollbackAction {
	return RollbackAction{
		baseAction: newBase("Syncing"),
		phase:      rollbackPhaseLoading,
		password:   password,
	}
}

func (a RollbackAction) Init() tea.Cmd {
	return tea.Batch(pullCmd(a.password), a.spinner.Init())
}

func (a RollbackAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	a.captureSize(msg)
	switch a.phase {
	case rollbackPhaseLoading:
		return a.updateLoading(msg)
	case rollbackPhasePicking:
		return a.updatePicking(msg)
	case rollbackPhaseConfirming:
		return a.updateConfirming(msg)
	case rollbackPhaseSaving:
		return a.updateSaving(msg)
	}
	return a, nil
}

func (a RollbackAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	store, cmd := a.handleLoadingMsg(msg, a.password)
	if store == nil {
		return a, cmd
	}
	a.store = store

	labels, byLabel, err := storage.RollbackPicks()
	if err != nil {
		a.finishErr(err)
		return a, nil
	}
	if len(labels) == 0 {
		a.finish("Nothing to roll back to.")
		return a, nil
	}
	a.entryByLabel = byLabel
	a.picker = prompt.NewPickerModel(labels, nil).WithoutHelp()
	a.sizePicker(&a.picker)
	a.phase = rollbackPhasePicking
	return a, nil
}

func (a RollbackAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	sel, ok, cmd := a.stepPicker(&a.picker, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	entry, found := a.entryByLabel[sel]
	if !found {
		a.finish(color.InRed(fmt.Sprintf("internal: picker returned unrecognized label %q", sel)))
		return a, nil
	}
	a.target = entry

	records, err := storage.LoadCommitRecords(a.target.ShortSHA, a.password)
	if err != nil {
		a.finish(color.InRed(formatLoadError(err)))
		return a, nil
	}
	a.records = records
	a.initYesNo(fmt.Sprintf("Replace records with snapshot from %s (%s)?", a.target.ShortSHA, a.target.Message))
	a.phase = rollbackPhaseConfirming
	return a, nil
}

func (a RollbackAction) updateConfirming(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if !answer {
		a.done = true
		return a, nil
	}
	spinnerCmd := a.initSpinner("Saving")
	a.phase = rollbackPhaseSaving
	return a, tea.Batch(applyRollbackCmd(a.store, a.target, a.records), spinnerCmd)
}

func (a RollbackAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(rollbackAppliedMsg); ok {
		if m.err != nil {
			a.finishErr(m.err)
			return a, nil
		}
		a.finish(fmt.Sprintf("Rolled back to %s: %s", color.InCyan(a.target.ShortSHA), a.target.Message))
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a RollbackAction) View() tea.View {
	switch a.phase {
	case rollbackPhaseLoading, rollbackPhaseSaving:
		return prependTranscript(a.spinner.View(), a.transcript)
	case rollbackPhasePicking:
		return prependTranscript(a.picker.View(), a.transcript)
	case rollbackPhaseConfirming:
		return prependTranscript(a.yesNo.View(), a.transcript)
	}
	return tea.NewView("")
}

func (a RollbackAction) FooterHelp() string {
	if a.phase == rollbackPhasePicking {
		return a.picker.Help()
	}
	return ""
}
