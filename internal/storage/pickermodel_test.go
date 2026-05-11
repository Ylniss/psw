package storage

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func updatePicker(m PickerModel, msg tea.Msg) PickerModel {
	raw, _ := m.Update(msg)
	return raw.(PickerModel)
}

func driveKeys(m PickerModel, keys ...tea.KeyPressMsg) PickerModel {
	for _, k := range keys {
		m = updatePicker(m, k)
	}
	return m
}

var (
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keySpace = tea.KeyPressMsg{Code: ' ', Text: " "}
)

func sizedMulti(names ...string) PickerModel {
	m := NewPickerModelMulti(names)
	return updatePicker(m, tea.WindowSizeMsg{Width: 80, Height: 24})
}

func TestPickerModel_EnterSelects(t *testing.T) {
	m := NewPickerModel([]string{"alpha", "beta", "gamma"}, nil)
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
	m := NewPickerModel([]string{"a", "b"}, nil)
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
	m := NewPickerModel([]string{"a", "b"}, nil)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatePicker(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after ctrl+c")
	}
}

func TestPickerMulti_ToggleAndConfirmPreservesListOrder(t *testing.T) {
	m := sizedMulti("a", "b", "c")
	m = driveKeys(m, keySpace, keyDown, keyDown, keySpace, keyEnter)
	if !m.Done() {
		t.Fatalf("expected Done=true after enter")
	}
	if got, want := m.Selections(), []string{"a", "c"}; !slices.Equal(got, want) {
		t.Fatalf("Selections = %v, want %v", got, want)
	}
}

func TestPickerMulti_NoTogglesFallsThroughToCursor(t *testing.T) {
	m := sizedMulti("a", "b", "c")
	m = driveKeys(m, keyEnter)
	if !m.Done() {
		t.Fatalf("expected Done=true after enter")
	}
	if got := m.Selections(); got != nil {
		t.Fatalf("Selections = %v, want nil for no-toggle path", got)
	}
	if got := m.Selection(); got != "a" {
		t.Fatalf("Selection = %q, want %q (cursor's first item)", got, "a")
	}
}

func TestPickerMulti_HelpTextDistinguishesMode(t *testing.T) {
	single := NewPickerModel([]string{"a"}, nil).Help()
	multi := NewPickerModelMulti([]string{"a"}).Help()
	if !strings.Contains(single, "enter select") {
		t.Fatalf("single help missing 'enter select': %q", single)
	}
	if !strings.Contains(multi, "space toggle") {
		t.Fatalf("multi help missing 'space toggle': %q", multi)
	}
	if strings.Contains(single, "space toggle") {
		t.Fatalf("single help leaked 'space toggle': %q", single)
	}
}

func TestPickerMulti_ReopenPreservesToggledSet(t *testing.T) {
	m := sizedMulti("a", "b", "c")
	m = driveKeys(m, keySpace, keyDown, keyDown, keySpace, keyEnter)
	if !m.Done() {
		t.Fatalf("expected Done=true after first enter")
	}
	m.Reopen()
	if m.Done() {
		t.Fatalf("Reopen should clear Done")
	}
	if m.Selections() != nil {
		t.Fatalf("Reopen should clear selections")
	}
	m = driveKeys(m, keyEnter)
	if got, want := m.Selections(), []string{"a", "c"}; !slices.Equal(got, want) {
		t.Fatalf("Selections after reopen+enter = %v, want %v", got, want)
	}
}

func TestPickerMulti_ToggleTwiceDeselects(t *testing.T) {
	m := sizedMulti("a", "b")
	m = driveKeys(m, keySpace, keySpace, keyEnter)
	if got := m.Selections(); got != nil {
		t.Fatalf("Selections = %v, want nil after toggle-toggle", got)
	}
	if got := m.Selection(); got != "a" {
		t.Fatalf("Selection = %q, want %q (fall-through)", got, "a")
	}
}
