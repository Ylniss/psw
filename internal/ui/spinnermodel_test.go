package ui

import (
	"testing"
)

func TestSpinnerModel_DoneMsgCompletes(t *testing.T) {
	m := NewSpinnerModel("test")
	if m.Done() {
		t.Fatal("fresh spinner should not be Done()")
	}
	raw, _ := m.Update(DoneMsg{})
	m = raw.(SpinnerModel)
	if !m.Done() {
		t.Fatal("expected Done() true after DoneMsg")
	}
}

func TestSpinnerModel_OtherMsgsForwarded(t *testing.T) {
	m := NewSpinnerModel("test")
	raw, _ := m.Update(struct{}{})
	m = raw.(SpinnerModel)
	if m.Done() {
		t.Fatal("unrelated msg should not complete spinner")
	}
}
