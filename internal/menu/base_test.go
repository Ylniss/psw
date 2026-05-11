package menu

import (
	"strings"
	"testing"
)

func TestBaseAction_InitInputSetsModelAndReturnsCmd(t *testing.T) {
	var b baseAction
	cmd := b.initInput("Label", false, false)
	if b.input.Prefix() != "Label: " {
		t.Fatalf("expected prefix %q, got %q", "Label: ", b.input.Prefix())
	}
	if cmd == nil {
		t.Fatal("initInput must return a non-nil cmd (textinput.Blink)")
	}
}

func TestBaseAction_InitInputPasswordHidesValue(t *testing.T) {
	var b baseAction
	b.initInput("PW", true, false)
	if !b.input.Hidden() {
		t.Fatal("password input must be Hidden()")
	}
}

func TestBaseAction_InitYesNoSetsQuestionAndReturnsNil(t *testing.T) {
	var b baseAction
	cmd := b.initYesNo("Are you sure?")
	if b.yesNo.Question() != "Are you sure?" {
		t.Fatalf("expected question %q, got %q", "Are you sure?", b.yesNo.Question())
	}
	if cmd != nil {
		t.Fatal("initYesNo must return nil cmd (no init)")
	}
}

func TestBaseAction_InitSpinnerSetsLabelAndReturnsCmd(t *testing.T) {
	var b baseAction
	cmd := b.initSpinner("Loading")
	if !strings.Contains(b.spinner.View().Content, "Loading") {
		t.Fatalf("expected spinner view to contain %q, got %q", "Loading", b.spinner.View().Content)
	}
	if cmd == nil {
		t.Fatal("initSpinner must return a non-nil cmd (spinner.Tick)")
	}
}

// initInput must overwrite a previous input rather than carry over state.
func TestBaseAction_InitInputResetsBetweenCalls(t *testing.T) {
	var b baseAction
	b.initInput("First", false, false)
	b.initInput("Second", false, false)
	if b.input.Prefix() != "Second: " {
		t.Fatalf("expected second prefix to win, got %q", b.input.Prefix())
	}
}
