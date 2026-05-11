package menu

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type fakeFinishMsg struct{}

// fakeAction is a minimal Action used to drive MenuModel through running-action
// transitions. Receiving fakeFinishMsg sets done=true.
type fakeAction struct {
	output      []string
	newPassword string
	done        bool
	cancelled   bool
}

func (a fakeAction) Init() tea.Cmd { return nil }

func (a fakeAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(fakeFinishMsg); ok {
		a.done = true
	}
	return a, nil
}

func (a fakeAction) View() tea.View      { return tea.NewView("") }
func (a fakeAction) Done() bool          { return a.done }
func (a fakeAction) Cancelled() bool     { return a.cancelled }
func (a fakeAction) Output() []string    { return a.output }
func (a fakeAction) NewPassword() string { return a.newPassword }
func (a fakeAction) FooterHelp() string  { return "" }

func updateMenu(m MenuModel, msg tea.Msg) MenuModel {
	raw, _ := m.Update(msg)
	return raw.(MenuModel)
}

func TestMenuModel_PasswordPhaseEscQuits(t *testing.T) {
	m := NewMenuModel()
	if m.phase != menuPhaseEnterPassword {
		t.Fatalf("expected menuPhaseEnterPassword, got %v", m.phase)
	}
	raw, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = raw.(MenuModel)
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from password Esc")
	}
}

func TestMenuModel_SelectActionLKeyMovesRight(t *testing.T) {
	// In the column-major 3x2 grid, 'l' from get (0,0) jumps to change (0,1)
	// — index 2 in menuActions.
	m := NewMenuModel()
	m.phase = menuPhaseSelectAction
	m = updateMenu(m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.actionCursor != 2 {
		t.Fatalf("expected cursor 2 (change) after 'l' from get, got %d", m.actionCursor)
	}
}

func TestMenuModel_SelectActionHKeyMovesLeft(t *testing.T) {
	// 'h' from change (0,1) jumps to get (0,0).
	m := NewMenuModel()
	m.phase = menuPhaseSelectAction
	m.actionCursor = 2
	m = updateMenu(m, tea.KeyPressMsg{Code: 'h', Text: "h"})
	if m.actionCursor != 0 {
		t.Fatalf("expected cursor 0 (get) after 'h' from change, got %d", m.actionCursor)
	}
}

func TestMenuModel_SelectActionJKeyMovesDown(t *testing.T) {
	// 'j' from get (0,0) drops to add (1,0).
	m := NewMenuModel()
	m.phase = menuPhaseSelectAction
	m = updateMenu(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.actionCursor != 1 {
		t.Fatalf("expected cursor 1 (add) after 'j' from get, got %d", m.actionCursor)
	}
}

func TestMenuModel_SelectActionQQuits(t *testing.T) {
	m := NewMenuModel()
	m.phase = menuPhaseSelectAction
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from q at action-select")
	}
}

func TestMenuModel_RunningActionCompletionReturnsToSelect(t *testing.T) {
	m := NewMenuModel()
	m.phase = menuPhaseRunningAction
	m.activeAction = fakeAction{output: []string{"recordX done"}}
	m = updateMenu(m, fakeFinishMsg{})
	if m.phase != menuPhaseSelectAction {
		t.Fatalf("expected menuPhaseSelectAction after action.Done(), got %v", m.phase)
	}
	if m.activeAction != nil {
		t.Fatalf("activeAction should be nil after completion")
	}
	if m.lastOutput != "recordX done" {
		t.Fatalf("lastOutput = %q, want %q", m.lastOutput, "recordX done")
	}
}

func TestMenuModel_RunningActionNewPasswordPropagates(t *testing.T) {
	m := NewMenuModel()
	m.password = "old"
	m.phase = menuPhaseRunningAction
	m.activeAction = fakeAction{newPassword: "new"}
	m = updateMenu(m, fakeFinishMsg{})
	if m.password != "new" {
		t.Fatalf("password = %q, want %q", m.password, "new")
	}
}

func TestMenuModel_RunningActionCancelReturnsToSelectWithoutOutput(t *testing.T) {
	m := NewMenuModel()
	m.phase = menuPhaseRunningAction
	m.activeAction = fakeAction{cancelled: true, output: []string{"should-not-leak"}}
	m = updateMenu(m, struct{}{})
	if m.phase != menuPhaseSelectAction {
		t.Fatalf("expected menuPhaseSelectAction after cancellation, got %v", m.phase)
	}
	if m.lastOutput != "" {
		t.Fatalf("lastOutput should stay empty on cancel, got %q", m.lastOutput)
	}
}
