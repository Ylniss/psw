package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

type changePhase int

const (
	changePhaseLoading changePhase = iota
	changePhaseEnterNewMain
	changePhaseEnterNewMainRepeat
	changePhasePicking
	changePhaseConfirmRename
	changePhaseEnterRename
	changePhaseConfirmUsername
	changePhaseEnterUsername
	changePhaseConfirmPassword
	changePhaseEnterPassword
	changePhaseEnterPasswordRepeat
	changePhaseConfirmValue
	changePhaseEnterValue
	changePhaseSaving
)

type ChangeAction struct {
	phase    changePhase
	password string

	spinner ui.SpinnerModel
	yesNo   prompt.YesNoModel
	input   prompt.InputModel
	picker  storage.PickerModel
	store   *storage.Storage

	rotatingMainPassword    bool
	newMain       string
	pendingPassword string

	nameBeforeRename string
	record       storage.Record

	width, height int

	banner     string
	transcript []string

	output      []string
	done        bool
	cancelled   bool
	newPassword string // non-empty after successful change-main
}

func NewChangeAction(password string) ChangeAction {
	return ChangeAction{
		phase:    changePhaseLoading,
		password: password,
		spinner:  ui.NewSpinnerModel("Decrypting"),
	}
}

func (a ChangeAction) Init() tea.Cmd {
	return tea.Batch(loadCmd(a.password, true), a.spinner.Init())
}

func (a ChangeAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.phase {
	case changePhaseLoading:
		return a.updateLoading(msg)
	case changePhaseEnterNewMain:
		return a.updateEnterNewMain(msg)
	case changePhaseEnterNewMainRepeat:
		return a.updateEnterNewMainRepeat(msg)
	case changePhasePicking:
		return a.updatePicking(msg)
	case changePhaseConfirmRename:
		return a.updateConfirmRename(msg)
	case changePhaseEnterRename:
		return a.updateEnterRename(msg)
	case changePhaseConfirmUsername:
		return a.updateConfirmUsername(msg)
	case changePhaseEnterUsername:
		return a.updateEnterUsername(msg)
	case changePhaseConfirmPassword:
		return a.updateConfirmPassword(msg)
	case changePhaseEnterPassword:
		return a.updateEnterPassword(msg)
	case changePhaseEnterPasswordRepeat:
		return a.updateEnterPasswordRepeat(msg)
	case changePhaseConfirmValue:
		return a.updateConfirmValue(msg)
	case changePhaseEnterValue:
		return a.updateEnterValue(msg)
	case changePhaseSaving:
		return a.updateSaving(msg)
	}
	return a, nil
}

func (a ChangeAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		a.picker = storage.NewPickerModel(a.store.GetNames(), []string{"main-password"})
		if a.width > 0 && a.height > 0 {
			tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
		}
		a.phase = changePhasePicking
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a ChangeAction) updateEnterNewMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.newMain = a.input.Value()
		a.banner = ""
		return a.toInput("Repeat new main password", true, true, changePhaseEnterNewMainRepeat)
	}
	return a, cmd
}

func (a ChangeAction) updateEnterNewMainRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if !a.input.Done() {
		return a, cmd
	}
	a.transcript = append(a.transcript, formatInputLine(a.input))
	if a.input.Value() != a.newMain {
		a.banner = passwordMismatchBanner
		a.newMain = ""
		return a.toInput("New main password", true, true, changePhaseEnterNewMain)
	}
	a.store.MainPassword = a.newMain
	return a.toSpinner("Saving", changePhaseSaving, saveCmd(a.store, "main password changed"))
}

func (a ChangeAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.picker, msg)
	if a.picker.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.picker.Done() {
		sel := a.picker.Selection()
		a.transcript = append(a.transcript, "> "+sel)
		if sel == "main-password" {
			a.rotatingMainPassword = true
			return a.toInput("New main password", true, true, changePhaseEnterNewMain)
		}
		a.nameBeforeRename = sel
		rec, ok := a.store.GetRecord(sel)
		if !ok {
			a.output = append(a.output, fmt.Sprintf("Record %s was not found", color.InGreen(sel)))
			a.done = true
			return a, nil
		}
		a.record = rec
		return a.toYesNo("Do you want to change record name?", changePhaseConfirmRename)
	}
	return a, cmd
}

func (a ChangeAction) updateConfirmRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.yesNo.Done() {
		a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
		if a.yesNo.Answer() {
			a.banner = fmt.Sprintf("Current name: %s", color.InGreen(a.record.Name))
			return a.toInput("New name", false, false, changePhaseEnterRename)
		}
		return a.advanceAfterRename(), nil
	}
	return a, cmd
}

func (a ChangeAction) updateEnterRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if !a.input.Done() {
		return a, cmd
	}
	a.transcript = append(a.transcript, formatInputLine(a.input))
	newName := a.input.Value()
	if a.store.Exists(newName) {
		a.output = append(a.output, fmt.Sprintf("Record with name %s already exists", color.InGreen(newName)))
		a.done = true
		return a, nil
	}
	a.record.Name = newName
	a.banner = ""
	return a.advanceAfterRename(), nil
}

// advanceAfterRename branches based on record kind into the next confirm step.
func (a ChangeAction) advanceAfterRename() ChangeAction {
	if a.record.Value == "" {
		a.yesNo = prompt.NewYesNoModel("Do you want to change username?")
		a.phase = changePhaseConfirmUsername
		return a
	}
	a.yesNo = prompt.NewYesNoModel("Do you want to change value?")
	a.phase = changePhaseConfirmValue
	return a
}

func (a ChangeAction) updateConfirmUsername(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.yesNo.Done() {
		a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
		if a.yesNo.Answer() {
			return a.toInput("New username", false, false, changePhaseEnterUsername)
		}
		return a.toYesNo("Do you want to change password?", changePhaseConfirmPassword)
	}
	return a, cmd
}

func (a ChangeAction) updateEnterUsername(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.record.Username = a.input.Value()
		return a.toYesNo("Do you want to change password?", changePhaseConfirmPassword)
	}
	return a, cmd
}

func (a ChangeAction) updateConfirmPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.yesNo.Done() {
		a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
		if a.yesNo.Answer() {
			a.banner = ""
			return a.toInput("New password", true, false, changePhaseEnterPassword)
		}
		return a.startSavePath()
	}
	return a, cmd
}

func (a ChangeAction) updateEnterPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.pendingPassword = a.input.Value()
		return a.toInput("Repeat new password", true, false, changePhaseEnterPasswordRepeat)
	}
	return a, cmd
}

func (a ChangeAction) updateEnterPasswordRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if !a.input.Done() {
		return a, cmd
	}
	a.transcript = append(a.transcript, formatInputLine(a.input))
	if a.input.Value() != a.pendingPassword {
		a.banner = passwordMismatchBanner
		a.pendingPassword = ""
		return a.toInput("New password", true, false, changePhaseEnterPassword)
	}
	a.record.Password = a.pendingPassword
	return a.startSavePath()
}

func (a ChangeAction) updateConfirmValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.yesNo.Done() {
		a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
		if a.yesNo.Answer() {
			return a.toInput("New value", false, false, changePhaseEnterValue)
		}
		return a.startSavePath()
	}
	return a, cmd
}

func (a ChangeAction) updateEnterValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.record.Value = a.input.Value()
		return a.startSavePath()
	}
	return a, cmd
}

func (a ChangeAction) startSavePath() (tea.Model, tea.Cmd) {
	a.store.UpdateRecord(a.nameBeforeRename, a.record)
	return a.toSpinner("Saving", changePhaseSaving, saveCmd(a.store, "record updated"))
}

func (a ChangeAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(storageSavedMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(m.err.Error()))
			a.done = true
			return a, nil
		}
		if a.rotatingMainPassword {
			a.output = append(a.output, color.InGreen("Main password changed"))
			a.newPassword = a.newMain
		} else {
			a.output = append(a.output, color.InGreen("Record updated"))
		}
		a.done = true
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a ChangeAction) View() tea.View {
	switch a.phase {
	case changePhaseLoading, changePhaseSaving:
		return prependTranscript(a.spinner.View(), a.transcript)
	case changePhasePicking:
		return a.picker.View()
	case changePhaseConfirmRename,
		changePhaseConfirmUsername,
		changePhaseConfirmPassword,
		changePhaseConfirmValue:
		return prependTranscript(a.yesNo.View(), a.transcript)
	case changePhaseEnterNewMain,
		changePhaseEnterNewMainRepeat,
		changePhaseEnterRename,
		changePhaseEnterUsername,
		changePhaseEnterPassword,
		changePhaseEnterPasswordRepeat,
		changePhaseEnterValue:
		return prependTranscript(prependBanner(a.input.View(), a.banner), a.transcript)
	}
	return tea.NewView("")
}

func (a ChangeAction) Done() bool          { return a.done }
func (a ChangeAction) Cancelled() bool     { return a.cancelled }
func (a ChangeAction) Output() []string    { return a.output }
func (a ChangeAction) NewPassword() string { return a.newPassword }
func (a ChangeAction) Transcript() []string { return a.transcript }
