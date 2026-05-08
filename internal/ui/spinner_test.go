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
