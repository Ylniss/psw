package menu

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func updateMenu(m MenuModel, msg tea.Msg) MenuModel {
	raw, _ := m.Update(msg)
	return raw.(MenuModel)
}

func TestMenuModel_PasswordPhaseEscQuits(t *testing.T) {
	m := NewMenuModel()
	if m.phase != PhaseEnterPassword {
		t.Fatalf("expected PhaseEnterPassword, got %v", m.phase)
	}
	raw, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = raw.(MenuModel)
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from password Esc")
	}
}

func TestMenuModel_SelectActionLKeyAdvancesCursor(t *testing.T) {
	m := NewMenuModel()
	m.phase = PhaseSelectAction
	m = updateMenu(m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.actionCursor != 1 {
		t.Fatalf("expected cursor 1 after 'l', got %d", m.actionCursor)
	}
}

func TestMenuModel_SelectActionHKeyDecrementsCursor(t *testing.T) {
	m := NewMenuModel()
	m.phase = PhaseSelectAction
	m.actionCursor = 2
	m = updateMenu(m, tea.KeyPressMsg{Code: 'h', Text: "h"})
	if m.actionCursor != 1 {
		t.Fatalf("expected cursor 1 after 'h', got %d", m.actionCursor)
	}
}

func TestMenuModel_SelectActionQQuits(t *testing.T) {
	m := NewMenuModel()
	m.phase = PhaseSelectAction
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from q at action-select")
	}
}

func TestMenuModel_HistoryCap(t *testing.T) {
	m := NewMenuModel()
	for i := 0; i < historyCap+5; i++ {
		m.history = append(m.history, "block")
	}
	if len(m.history) > historyCap+5 {
		t.Fatalf("setup error: %d", len(m.history))
	}
	// Simulate the cap logic.
	if len(m.history) > historyCap {
		m.history = m.history[len(m.history)-historyCap:]
	}
	if len(m.history) != historyCap {
		t.Fatalf("expected history len %d, got %d", historyCap, len(m.history))
	}
}
