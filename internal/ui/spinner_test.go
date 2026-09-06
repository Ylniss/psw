package ui

import (
	"errors"
	"testing"
)

// go test pipes stderr → non-TTY → WithSpinner forwards op's result unchanged.
func TestWithSpinner_NonTTYPassthrough(t *testing.T) {
	sentinel := errors.New("op failed")
	if got := WithSpinner("x", func() error { return sentinel }); !errors.Is(got, sentinel) {
		t.Fatalf("WithSpinner err = %v, want %v", got, sentinel)
	}
	if got := WithSpinner("x", func() error { return nil }); got != nil {
		t.Fatalf("WithSpinner err = %v, want nil", got)
	}
}

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
