package ui

import (
	"testing"
)

func TestSpinnerModel_DoneMsgCompletes(t *testing.T) {
	m := NewSpinnerModel("test")
	if m.Completed() {
		t.Fatal("fresh spinner should not be Completed()")
	}
	raw, _ := m.Update(DoneMsg{})
	m = raw.(SpinnerModel)
	if !m.Completed() {
		t.Fatal("expected Completed() true after DoneMsg")
	}
}

func TestSpinnerModel_OtherMsgsForwarded(t *testing.T) {
	// A non-DoneMsg should not flip Completed.
	m := NewSpinnerModel("test")
	raw, _ := m.Update(struct{}{})
	m = raw.(SpinnerModel)
	if m.Completed() {
		t.Fatal("unrelated msg should not complete spinner")
	}
}
