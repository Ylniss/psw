package prompt

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func updateInput(m InputModel, msg tea.Msg) InputModel {
	raw, _ := m.Update(msg)
	return raw.(InputModel)
}

func typeChar(m InputModel, r rune) InputModel {
	return updateInput(m, tea.KeyPressMsg{Code: r, Text: string(r)})
}

func TestInputModel_EscCancels(t *testing.T) {
	m := NewInputModel("test", false, false)
	m = updateInput(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after esc")
	}
	if m.Done() {
		t.Fatal("expected Done() false after esc")
	}
}

func TestInputModel_CtrlCCancels(t *testing.T) {
	m := NewInputModel("test", false, false)
	m = updateInput(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after ctrl+c")
	}
}

func TestInputModel_EnterEmptyValueShowsError(t *testing.T) {
	m := NewInputModel("test", false, false)
	m = updateInput(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Done() {
		t.Fatal("expected Done() false on empty enter")
	}
	if m.Cancelled() {
		t.Fatal("expected Cancelled() false on empty enter")
	}
}

func TestInputModel_EnterWithValueCommits(t *testing.T) {
	m := NewInputModel("test", false, false)
	m = typeChar(m, 'h')
	m = typeChar(m, 'i')
	m = updateInput(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Done() {
		t.Fatal("expected Done() true after enter with value")
	}
	if got := m.Value(); got != "hi" {
		t.Fatalf("Value() = %q, want %q", got, "hi")
	}
}

func TestInputModel_Reset(t *testing.T) {
	m := NewInputModel("test", false, false)
	m = typeChar(m, 'x')
	m = updateInput(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Done() {
		t.Fatal("setup: expected Done() true")
	}
	m.Reset()
	if m.Done() || m.Cancelled() {
		t.Fatal("Reset did not clear flags")
	}
	if m.Value() != "" {
		t.Fatalf("Reset did not clear value: %q", m.Value())
	}
}

func TestInputModel_AnimateStarsUsesEchoNone(t *testing.T) {
	m := NewInputModel("pw", true, true)
	m = typeChar(m, 'a')
	m = typeChar(m, 'b')
	if got := m.Value(); got != "ab" {
		t.Fatalf("Value() = %q, want %q", got, "ab")
	}
	if m.stars.Len() == 0 {
		t.Fatal("expected stars to accumulate when animateStars=true")
	}
}
