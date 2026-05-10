package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"
	passgen "github.com/sethvargo/go-password/password"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
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
	phase    addPhase
	password string

	spinner ui.SpinnerModel
	yesNo   prompt.YesNoModel
	input   prompt.InputModel
	store   *storage.Storage

	isSingleValue   bool
	recordName      string
	username        string
	pendingPassword string

	banner     string
	transcript []string

	output    []string
	done      bool
	cancelled bool
}

func NewAddAction(password string) AddAction {
	return AddAction{
		phase:    addPhaseLoading,
		password: password,
		spinner:  ui.NewSpinnerModel("Decrypting"),
	}
}

func (a AddAction) Init() tea.Cmd {
	return tea.Batch(loadCmd(a.password, true), a.spinner.Init())
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
	if m, ok := msg.(storageLoadedMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(humanizeLoadError(m.err)))
			a.done = true
			return a, nil
		}
		a.store = m.store
		return a.toYesNo("Add a single value record?", addPhaseAskSingle)
	}
	cmd := tuiutil.UpdateInPlace(&a.spinner, msg)
	return a, cmd
}

func (a AddAction) updateAskSingle(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.yesNo.Done() {
		a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
		a.isSingleValue = a.yesNo.Answer()
		a.banner = ""
		return a.toInput("Record name", false, false, addPhaseEnterName)
	}
	return a, cmd
}

func (a AddAction) updateEnterName(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if !a.input.Done() {
		return a, cmd
	}
	a.transcript = append(a.transcript, formatInputLine(a.input))
	name := a.input.Value()
	lower := strings.ToLower(name)
	if lower == "main" || lower == "main-password" {
		a.output = append(a.output, fmt.Sprintf("Name %s is reserved. %s command uses it for changing main password",
			color.InGreen(name), color.InCyan("change")))
		a.done = true
		return a, nil
	}
	if a.store.Exists(name) {
		a.output = append(a.output, fmt.Sprintf("Record with name %s already exists", color.InGreen(name)))
		a.done = true
		return a, nil
	}
	a.recordName = name
	if a.isSingleValue {
		return a.toInput("Value", false, false, addPhaseEnterValue)
	}
	return a.toInput("Username", false, false, addPhaseEnterUsername)
}

func (a AddAction) updateEnterUsername(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.username = a.input.Value()
		return a.toYesNo("Auto-generate password?", addPhaseAskGenerate)
	}
	return a, cmd
}

func (a AddAction) updateAskGenerate(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.yesNo, msg)
	if a.yesNo.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if !a.yesNo.Done() {
		return a, cmd
	}
	a.transcript = append(a.transcript, formatYesNoLine(a.yesNo))
	if !a.yesNo.Answer() {
		return a.toInput("Password", true, false, addPhaseEnterPassword)
	}
	generated, err := passgen.Generate(16, 4, 6, false, true)
	if err != nil {
		a.output = append(a.output, color.InRed(err.Error()))
		a.done = true
		return a, nil
	}
	a.store.AddRecord(&storage.Record{Name: a.recordName, Username: a.username, Password: generated})
	return a.toSpinner("Saving", addPhaseSaving, saveCmd(a.store, "added new record"))
}

func (a AddAction) updateEnterPassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.pendingPassword = a.input.Value()
		return a.toInput("Repeat password", true, false, addPhaseEnterPasswordRepeat)
	}
	return a, cmd
}

func (a AddAction) updateEnterPasswordRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return a.toInput("Password", true, false, addPhaseEnterPassword)
	}
	a.store.AddRecord(&storage.Record{Name: a.recordName, Username: a.username, Password: a.pendingPassword})
	return a.toSpinner("Saving", addPhaseSaving, saveCmd(a.store, "added new record"))
}

func (a AddAction) updateEnterValue(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := tuiutil.UpdateInPlace(&a.input, msg)
	if a.input.Cancelled() {
		a.cancelled = true
		return a, nil
	}
	if a.input.Done() {
		a.transcript = append(a.transcript, formatInputLine(a.input))
		a.store.AddRecord(&storage.Record{Name: a.recordName, Value: a.input.Value()})
		return a.toSpinner("Saving", addPhaseSaving, saveCmd(a.store, "added new record"))
	}
	return a, cmd
}

func (a AddAction) updateSaving(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, ok := msg.(storageSavedMsg); ok {
		if m.err != nil {
			a.output = append(a.output, color.InRed(m.err.Error()))
			a.done = true
			return a, nil
		}
		if a.isSingleValue {
			a.output = append(a.output, fmt.Sprintf("Value set successfully in %s record", color.InGreen(a.recordName)))
		} else {
			a.output = append(a.output, fmt.Sprintf("Username/password set successfully in %s record", color.InGreen(a.recordName)))
		}
		a.done = true
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
		return prependTranscript(prependBanner(a.input.View(), a.banner), a.transcript)
	}
	return tea.NewView("")
}

func (a AddAction) Done() bool             { return a.done }
func (a AddAction) Cancelled() bool        { return a.cancelled }
func (a AddAction) Output() []string       { return a.output }
func (a AddAction) NewPassword() string    { return "" }
func (a AddAction) Transcript() []string   { return a.transcript }
