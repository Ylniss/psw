package menu

import (
	"errors"
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"

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
	password string

	store   *storage.Storage
	picker  storage.PickerModel
	entryByLabel map[string]storage.LogEntry
	target  storage.LogEntry
	records []storage.Record

	width, height int
}

func NewRollbackAction(password string) RollbackAction {
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
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = w.Width, w.Height
	}
	store, done, cmd := a.handleLoadingMsg(msg, a.password)
	if done || store == nil {
		return a, cmd
	}
	a.store = store

	entries, err := storage.GitLog()
	if err != nil {
		a.finishErr(err)
		return a, nil
	}
	headShort, err := storage.HeadShortSHA()
	if err != nil {
		a.finishErr(err)
		return a, nil
	}
	picks := make([]storage.LogEntry, 0, len(entries))
	for _, e := range entries {
		if e.ShortSHA != headShort {
			picks = append(picks, e)
		}
	}
	if len(picks) == 0 {
		a.finish("No previous commits to roll back to.")
		return a, nil
	}
	// Newest first — most likely rollback targets at the top. Matches CLI.
	slices.Reverse(picks)

	labels := make([]string, len(picks))
	a.entryByLabel = make(map[string]storage.LogEntry, len(picks))
	for i, e := range picks {
		label := fmt.Sprintf("%s  %s  %s", e.ShortSHA, e.Time.Format("2006-01-02 15:04"), e.Message)
		labels[i] = label
		a.entryByLabel[label] = e
	}
	a.picker = storage.NewPickerModel(labels, nil).WithoutHelp()
	if a.width > 0 && a.height > 0 {
		tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
	}
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
	if errors.Is(err, storage.ErrForkUndecryptable) {
		a.finish(color.InRed(storage.ForkUndecryptableUserMessage))
		return a, nil
	}
	if err != nil {
		a.finishErr(err)
		return a, nil
	}
	a.records = records
	a.initYesNo(fmt.Sprintf("Replace records with snapshot from %s (%s)?", a.target.ShortSHA, a.target.Message))
	a.phase = rollbackPhaseConfirming
	return a, nil
}

func (a RollbackAction) updateConfirming(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
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

func (a RollbackAction) NewPassword() string { return "" }
func (a RollbackAction) FooterHelp() string {
	if a.phase == rollbackPhasePicking {
		return a.picker.Help()
	}
	return ""
}
