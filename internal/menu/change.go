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
	baseAction

	phase    changePhase
	password string

	spinner ui.SpinnerModel
	yesNo   prompt.YesNoModel
	input   prompt.InputModel
	picker  storage.PickerModel
	store   *storage.Storage

	rotatingMainPassword bool
	newMain              string
	pendingPassword      string

	nameBeforeRename string
	record           storage.Record

	width, height int

	banner string

	newPassword string // non-empty after successful change-main
}

func NewChangeAction(password string) ChangeAction {
	return ChangeAction{
		phase:    changePhaseLoading,
		password: password,
		spinner:  ui.NewSpinnerModel("Syncing"),
	}
}

func (a ChangeAction) Init() tea.Cmd {
	return tea.Batch(pullCmd(a.password), a.spinner.Init())
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
		a.picker = storage.NewPickerModel(a.store.GetNames(), []string{"main-password"}).WithoutHelp()
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
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.newMain = val
	a.banner = ""
	return a.toInput("Repeat new main password", true, true, changePhaseEnterNewMainRepeat)
}

func (a ChangeAction) updateEnterNewMainRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if val != a.newMain {
		a.banner = passwordMismatchBanner
		a.newMain = ""
		return a.toInput("New main password", true, true, changePhaseEnterNewMain)
	}
	a.store.MainPassword = a.newMain
	return a.toSpinner("Saving", changePhaseSaving, saveCmd(a.store, "main password changed"))
}

func (a ChangeAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	sel, ok, cmd := a.stepPicker(&a.picker, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if sel == "main-password" {
		a.rotatingMainPassword = true
		return a.toInput("New main password", true, true, changePhaseEnterNewMain)
	}
	a.nameBeforeRename = sel
	rec, found := a.store.GetRecord(sel)
	if !found {
		a.output = append(a.output, fmt.Sprintf("Record %s was not found", color.InGreen(sel)))
		a.done = true
		return a, nil
	}
	a.record = rec
	return a.toYesNo("Do you want to change record name?", changePhaseConfirmRename)
}

func (a ChangeAction) updateConfirmRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if answer {
		a.banner = fmt.Sprintf("Current name: %s", color.InGreen(a.record.Name))
		return a.toInput("New name", false, false, changePhaseEnterRename)
	}
	return a.advanceAfterRename(), nil
}

func (a ChangeAction) updateEnterRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	newName, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
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
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if answer {
		return a.toInput("New username", false, false, changePhaseEnterUsername)
	}
	return a.toYesNo("Do you want to change password?", changePhaseConfirmPassword)
}

func (a ChangeAction) updateEnterUsername(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.record.Username = val
	return a.toYesNo("Do you want to change password?", changePhaseConfirmPassword)
}

func (a ChangeAction) updateConfirmPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if answer {
		a.banner = ""
		return a.toInput("New password", true, false, changePhaseEnterPassword)
	}
	return a.startSave()
}

func (a ChangeAction) updateEnterPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.pendingPassword = val
	return a.toInput("Repeat new password", true, false, changePhaseEnterPasswordRepeat)
}

func (a ChangeAction) updateEnterPasswordRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if val != a.pendingPassword {
		a.banner = passwordMismatchBanner
		a.pendingPassword = ""
		return a.toInput("New password", true, false, changePhaseEnterPassword)
	}
	a.record.Password = a.pendingPassword
	return a.startSave()
}

func (a ChangeAction) updateConfirmValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if answer {
		return a.toInput("New value", false, false, changePhaseEnterValue)
	}
	return a.startSave()
}

func (a ChangeAction) updateEnterValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.record.Value = val
	return a.startSave()
}

func (a ChangeAction) startSave() (tea.Model, tea.Cmd) {
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
			a.output = append(a.output, fmt.Sprintf("Record %s was updated successfully", color.InGreen(a.record.Name)))
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

func (a ChangeAction) NewPassword() string { return a.newPassword }
func (a ChangeAction) FooterHelp() string {
	if a.phase == changePhasePicking {
		return a.picker.Help()
	}
	return ""
}
