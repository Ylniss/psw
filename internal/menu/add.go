package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
)

type addPhase int

const (
	addPhaseLoading addPhase = iota
	addPhaseAskSingle
	addPhaseEnterName
	addPhaseEnterUsername
	addPhaseAskGenerate
	addPhaseEnterPassword
	addPhaseEnterPasswordRepeat
	addPhaseEnterValue
	addPhaseSaving
)

type AddAction struct {
	baseAction

	phase    addPhase
	password *memguard.Enclave

	store *storage.Storage

	isSingleValue   bool
	recordName      string
	username        string
	pendingPassword string

	inlineBanner string
}

func NewAddAction(password *memguard.Enclave) AddAction {
	return AddAction{
		baseAction: newBase("Syncing"),
		phase:      addPhaseLoading,
		password:   password,
	}
}

func (a AddAction) Init() tea.Cmd {
	return tea.Batch(pullCmd(a.password), a.spinner.Init())
}

func (a AddAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.phase {
	case addPhaseLoading:
		return a.updateLoading(msg)
	case addPhaseAskSingle:
		return a.updateAskSingle(msg)
	case addPhaseEnterName:
		return a.updateEnterName(msg)
	case addPhaseEnterUsername:
		return a.updateEnterUsername(msg)
	case addPhaseAskGenerate:
		return a.updateAskGenerate(msg)
	case addPhaseEnterPassword:
		return a.updateEnterPassword(msg)
	case addPhaseEnterPasswordRepeat:
		return a.updateEnterPasswordRepeat(msg)
	case addPhaseEnterValue:
		return a.updateEnterValue(msg)
	case addPhaseSaving:
		return a.updateSaving(msg)
	}
	return a, nil
}

func (a AddAction) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	store, done, cmd := a.handleLoadingMsg(msg, a.password)
	if done || store == nil {
		return a, cmd
	}
	a.store = store
	return a.toYesNo("Add a single value record?", addPhaseAskSingle)
}

func (a AddAction) updateAskSingle(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.isSingleValue = answer
	a.inlineBanner = ""
	return a.toInput("Record name", false, false, addPhaseEnterName)
}

func (a AddAction) updateEnterName(msg tea.Msg) (tea.Model, tea.Cmd) {
	name, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	lower := strings.ToLower(name)
	if lower == storage.MainPasswordKeywordShort || lower == storage.MainPasswordKeywordLong {
		a.finish(fmt.Sprintf("Name %s is reserved. %s command uses it for changing main password",
			color.InGreen(name), color.InCyan("change")))
		return a, nil
	}
	if a.store.Exists(name) {
		a.finish(fmt.Sprintf("Record with name %s already exists", color.InGreen(name)))
		return a, nil
	}
	a.recordName = name
	if a.isSingleValue {
		return a.toInput("Value", true, false, addPhaseEnterValue)
	}
	return a.toInput("Username", false, false, addPhaseEnterUsername)
}

func (a AddAction) updateEnterUsername(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.username = val
	return a.toYesNo("Auto-generate password?", addPhaseAskGenerate)
}

func (a AddAction) updateAskGenerate(msg tea.Msg) (tea.Model, tea.Cmd) {
	answer, ok, cmd := a.stepYesNo(&a.yesNo, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if !answer {
		return a.toInput("Password", true, false, addPhaseEnterPassword)
	}
	generated, err := storage.GenerateRecordPassword()
	if err != nil {
		a.finishErr(err)
		return a, nil
	}
	a.store.AddRecord(&storage.Record{Name: a.recordName, Username: a.username, Password: []byte(generated)})
	return a.toSpinner("Saving", addPhaseSaving, saveCmd(a.store, "added new record"))
}

func (a AddAction) updateEnterPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.pendingPassword = val
	return a.toInput("Repeat password", true, false, addPhaseEnterPasswordRepeat)
}

func (a AddAction) updateEnterPasswordRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if val != a.pendingPassword {
		a.inlineBanner = passwordMismatchBanner
		a.pendingPassword = ""
		return a.toInput("Password", true, false, addPhaseEnterPassword)
	}
	a.store.AddRecord(&storage.Record{Name: a.recordName, Username: a.username, Password: []byte(a.pendingPassword)})
	return a.toSpinner("Saving", addPhaseSaving, saveCmd(a.store, "added new record"))
}

func (a AddAction) updateEnterValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.store.AddRecord(&storage.Record{Name: a.recordName, Value: []byte(val)})
	return a.toSpinner("Saving", addPhaseSaving, saveCmd(a.store, "added new record"))
}

func (a AddAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(storageSavedMsg); ok {
		if m.err != nil {
			a.finishErr(m.err)
			return a, nil
		}
		if a.isSingleValue {
			a.finish(fmt.Sprintf("Value set successfully in %s record", color.InGreen(a.recordName)))
		} else {
			a.finish(fmt.Sprintf("Username/password set successfully in %s record", color.InGreen(a.recordName)))
		}
		return a, nil
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a AddAction) View() tea.View {
	switch a.phase {
	case addPhaseLoading, addPhaseSaving:
		return prependTranscript(a.spinner.View(), a.transcript)
	case addPhaseAskSingle, addPhaseAskGenerate:
		return prependTranscript(a.yesNo.View(), a.transcript)
	case addPhaseEnterName, addPhaseEnterUsername, addPhaseEnterPassword, addPhaseEnterPasswordRepeat, addPhaseEnterValue:
		return prependTranscript(prependBanner(a.input.View(), a.inlineBanner), a.transcript)
	}
	return tea.NewView("")
}

func (a AddAction) NewPassword() *memguard.Enclave { return nil }
func (a AddAction) FooterHelp() string              { return "" }
