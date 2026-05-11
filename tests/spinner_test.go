package tests

import (
	"errors"
	"testing"

	"github.com/ylniss/psw/internal/ui"
)

// go test pipes stderr → non-TTY → WithSpinner forwards op's result unchanged.
func TestWithSpinner_NonTTYPassthrough(t *testing.T) {
	sentinel := errors.New("op failed")
	if got := ui.WithSpinner("x", func() error { return sentinel }); !errors.Is(got, sentinel) {
		t.Fatalf("WithSpinner err = %v, want %v", got, sentinel)
	}
	if got := ui.WithSpinner("x", func() error { return nil }); got != nil {
		t.Fatalf("WithSpinner err = %v, want nil", got)
	}
}
