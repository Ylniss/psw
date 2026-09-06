package prompt

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/tuiutil"
)

func TestYesNoModel_YesCommits(t *testing.T) {
	m := NewYesNoModel("ok?")
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: 'y', Text: "y"})
	if !m.Done() {
		t.Fatal("expected Done() true after y")
	}
	if !m.Answer() {
		t.Fatal("expected Answer() true after y")
	}
}

func TestYesNoModel_NoCommits(t *testing.T) {
	m := NewYesNoModel("ok?")
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	if !m.Done() {
		t.Fatal("expected Done() true after n")
	}
	if m.Answer() {
		t.Fatal("expected Answer() false after n")
	}
}

func TestYesNoModel_UppercaseAlsoWorks(t *testing.T) {
	m := NewYesNoModel("ok?")
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	if !m.Done() || !m.Answer() {
		t.Fatal("expected Y to commit yes")
	}
}

func TestYesNoModel_EscCancels(t *testing.T) {
	m := NewYesNoModel("ok?")
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after esc")
	}
	if m.Done() {
		t.Fatal("expected Done() false after esc")
	}
}

func TestYesNoModel_CtrlCCancels(t *testing.T) {
	m := NewYesNoModel("ok?")
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.Cancelled() {
		t.Fatal("expected Cancelled() true after ctrl+c")
	}
}

func TestYesNoModel_OtherKeyIgnored(t *testing.T) {
	m := NewYesNoModel("ok?")
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.Done() || m.Cancelled() {
		t.Fatal("unrelated key should not change state")
	}
}
