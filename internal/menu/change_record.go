package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/prompt"
)

func (a ChangeAction) updateConfirmRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if answer {
		a.inlineBanner = fmt.Sprintf("Current name: %s", color.InGreen(a.record.Name))
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
	a.inlineBanner = ""
	return a.advanceAfterRename(), nil
}

// advanceAfterRename branches based on record kind into the next confirm step.
func (a ChangeAction) advanceAfterRename() ChangeAction {
	if len(a.record.Value) == 0 {
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
		a.inlineBanner = ""
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
		a.inlineBanner = passwordMismatchBanner()
		a.pendingPassword = ""
		return a.toInput("New password", true, false, changePhaseEnterPassword)
	}
	a.record.Password = []byte(a.pendingPassword)
	return a.startSave()
}

func (a ChangeAction) updateConfirmValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if answer {
		return a.toInput("New value", true, false, changePhaseEnterValue)
	}
	return a.startSave()
}

func (a ChangeAction) updateEnterValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.record.Value = []byte(val)
	return a.startSave()
}

func (a ChangeAction) startSave() (tea.Model, tea.Cmd) {
	a.store.UpdateRecord(a.nameBeforeRename, a.record)
	return a.toSpinner("Saving", changePhaseSaving, saveCmd(a.store, "record updated"))
}
