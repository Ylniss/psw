package menu

import (
	tea "charm.land/bubbletea/v2"
	"github.com/awnumar/memguard"
)

func (a ChangeAction) updateEnterNewMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	a.newMainPassword = val
	a.inlineBanner = ""
	return a.toInput("Repeat new main password", true, true, changePhaseEnterNewMainRepeat)
}

func (a ChangeAction) updateEnterNewMainRepeat(msg tea.Msg) (tea.Model, tea.Cmd) {
	val, ok, cmd := a.stepInput(&a.input, msg)
	if a.cancelled || !ok {
		return a, cmd
	}
	if val != a.newMainPassword {
		a.inlineBanner = passwordMismatchBanner()
		a.newMainPassword = ""
		return a.toInput("New main password", true, true, changePhaseEnterNewMain)
	}
	a.store.MainPassword = memguard.NewEnclave([]byte(a.newMainPassword))
	return a.toSpinner("Saving", changePhaseSaving, saveCmd(a.store, "main password changed"))
}
