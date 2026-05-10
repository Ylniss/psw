package storage

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func updatePicker(m PickerModel, msg tea.Msg) PickerModel {
	raw, _ := m.Update(msg)
	return raw.(PickerModel)
}

func TestPickerModel_EnterSelects(t *testing.T) {
	m := NewPickerModel([]string{"alpha", "beta", "gamma"})
	// WindowSize so list is rendered
	m = updatePicker(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Done() {
		t.Fatal("expected Done() true after enter")
	}
	if got := m.Selection(); got == "" {
		t.Fatal("expected non-empty Selection() after enter")
	}
}

func TestPickerModel_EscCancels(t *testing.T) {
	m := NewPickerModel([]string{"a", "b"})
	m = updatePicker(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after esc")
	}
	if m.Done() {
		t.Fatal("expected Done() false after esc")
	}
}

func TestPickerModel_CtrlCCancels(t *testing.T) {
	m := NewPickerModel([]string{"a", "b"})
	m = updatePicker(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatePicker(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after ctrl+c")
	}
}
