package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
)

type removePhase int

const (
	removePhaseLoading removePhase = iota
	removePhasePicking
	removePhaseConfirming
	removePhaseSaving
)

type RemoveAction struct {
	baseAction

	phase    removePhase
	password string

	picker             storage.PickerModel
	store              *storage.Storage
	chosenNames        []string
	transcriptBaseline int // transcript length before the picker appends `> name` lines

	width, height int
}

func NewRemoveAction(password string) RemoveAction {
	return RemoveAction{
		baseAction: newBase("Syncing"),
		phase:      removePhaseLoading,
		password:   password,
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
	case removePhaseConfirming:
		return a.updateConfirming(msg)
	case removePhaseSaving:
		return a.updateSaving(msg)
	}
	return a, nil
}

func (a RemoveAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = w.Width, w.Height
	}
	store, done, cmd := a.handleLoadingMsg(msg, a.password)
	if done || store == nil {
		return a, cmd
	}
	a.store = store
	names := store.GetNames()
	if len(names) == 0 {
		a.output = append(a.output, "No records to remove.")
		a.done = true
		return a, nil
	}
	a.picker = storage.NewPickerModelMulti(names).WithoutHelp()
	if a.width > 0 && a.height > 0 {
		tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
	}
	a.transcriptBaseline = len(a.transcript)
	a.phase = removePhasePicking
	return a, nil
}

func (a RemoveAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	sels, ok, cmd := a.stepPickerMulti(&a.picker, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.chosenNames = sels
	n := len(a.chosenNames)
	a.initYesNoWithHint(
		fmt.Sprintf("Remove %d %s?", n, recordWord(n)),
		color.InGray("You can undo any action with ")+color.InCyan("psw rollback"),
	)
	a.phase = removePhaseConfirming
	return a, nil
}

func (a RemoveAction) updateConfirming(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Esc on confirm bounces to the picker (esc on the picker still aborts via stepPickerMulti).
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "esc" {
		return a.bounceToPicker()
	}
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		tuiutil.UpdateInPlace(&a.picker, w)
	}
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		// Ctrl+c only — esc handled above.
		a.cancelled = true
		return a, nil
	}
	if !a.yesNo.Done() {
		return a, cmd
	}
	if !a.yesNo.Answer() {
		return a.bounceToPicker()
	}
	a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
	for _, n := range a.chosenNames {
		a.store.RemoveRecord(n)
	}
	spinnerCmd := a.initSpinner("Saving")
	a.phase = removePhaseSaving
	return a, tea.Batch(saveCmd(a.store, storage.RemoveCommitMessage(len(a.chosenNames))), spinnerCmd)
}

func (a RemoveAction) bounceToPicker() (tea.Model, tea.Cmd) {
	a.transcript = a.transcript[:a.transcriptBaseline]
	a.picker.Reopen()
	a.phase = removePhasePicking
	return a, nil
}

func (a RemoveAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(storageSavedMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(m.err.Error()))
			a.done = true
			return a, nil
		}
		a.output = append(a.output, removedSuccessLine(a.chosenNames))
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
	case removePhaseConfirming:
		return prependTranscript(a.yesNo.View(), a.transcript)
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

func recordWord(n int) string {
	if n == 1 {
		return "record"
	}
	return "records"
}

func removedSuccessLine(names []string) string {
	if len(names) == 1 {
		return fmt.Sprintf("Record %s successfully removed", color.InGreen(names[0]))
	}
	coloredNames := make([]string, len(names))
	for i, n := range names {
		coloredNames[i] = color.InGreen(n)
	}
	return fmt.Sprintf("Removed %d records: %s", len(names), strings.Join(coloredNames, ", "))
}
