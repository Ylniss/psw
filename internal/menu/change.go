package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
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
	password *memguard.Enclave

	picker storage.PickerModel
	store  *storage.Storage

	rotatingMainPassword bool
	newMainPassword      string
	pendingPassword      string

	nameBeforeRename string
	record           storage.Record

	width, height int

	inlineBanner string

	rotatedMainPassword *memguard.Enclave // non-nil after successful change-main
}

func NewChangeAction(password *memguard.Enclave) ChangeAction {
	return ChangeAction{
		baseAction: newBase("Syncing"),
		phase:      changePhaseLoading,
		password:   password,
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
	store, done, cmd := a.handleLoadingMsg(msg, a.password)
	if done || store == nil {
		return a, cmd
	}
	a.store = store
	a.picker = storage.NewPickerModel(store.GetNames(), []string{storage.MainPasswordKeywordLong}).WithoutHelp()
	if a.width > 0 && a.height > 0 {
		tuiutil.UpdateInPlace(&a.picker, tea.WindowSizeMsg{Width: a.width, Height: a.height})
	}
	a.phase = changePhasePicking
	return a, nil
}

func (a ChangeAction) updatePicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	sel, ok, cmd := a.stepPicker(&a.picker, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if sel == storage.MainPasswordKeywordLong {
		a.rotatingMainPassword = true
		return a.toInput("New main password", true, true, changePhaseEnterNewMain)
	}
	a.nameBeforeRename = sel
	rec, found := a.store.GetRecord(sel)
	if !found {
		a.finish(fmt.Sprintf("Record %s was not found", color.InGreen(sel)))
		return a, nil
	}
	a.record = rec
	return a.toYesNo("Do you want to change record name?", changePhaseConfirmRename)
}

func (a ChangeAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(storageSavedMsg); ok {
		if m.err != nil {
			a.finishErr(m.err)
			return a, nil
		}
		if a.rotatingMainPassword {
			a.rotatedMainPassword = memguard.NewEnclave([]byte(a.newMainPassword))
			a.finish(color.InGreen("Main password changed"))
		} else {
			a.finish(fmt.Sprintf("Record %s was updated successfully", color.InGreen(a.record.Name)))
		}
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
		return prependTranscript(prependBanner(a.input.View(), a.inlineBanner), a.transcript)
	}
	return tea.NewView("")
}

func (a ChangeAction) NewPassword() *memguard.Enclave { return a.rotatedMainPassword }

func (a ChangeAction) FooterHelp() string {
	if a.phase == changePhasePicking {
		return a.picker.Help()
	}
	return ""
}
