package menu

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/tuiutil"
)

type fakeFinishMsg struct{}

// fakeAction is a minimal Action used to drive MenuModel through
// running-action transitions.
type fakeAction struct {
	output      []string
	newPassword *memguard.Enclave
	done        bool
	cancelled   bool
}

func (a fakeAction) Init() tea.Cmd { return nil }

func (a fakeAction) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(fakeFinishMsg); ok {
		a.done = true
	}
	return a, nil
}

func (a fakeAction) View() tea.View                 { return tea.NewView("") }
func (a fakeAction) Done() bool                     { return a.done }
func (a fakeAction) Cancelled() bool                { return a.cancelled }
func (a fakeAction) Output() []string               { return a.output }
func (a fakeAction) NewPassword() *memguard.Enclave { return a.newPassword }
func (a fakeAction) FooterHelp() string             { return "" }

// enclaveBytes opens the enclave and returns a copy of its bytes for comparison.
func enclaveBytes(t *testing.T, e *memguard.Enclave) []byte {
	t.Helper()
	buf, err := e.Open()
	if err != nil {
		t.Fatalf("open enclave: %v", err)
	}
	defer buf.Destroy()
	cp := make([]byte, buf.Size())
	copy(cp, buf.Bytes())
	return cp
}

func TestMenuModel_PasswordPhaseEscQuits(t *testing.T) {
	m := NewMenuModel()
	if m.phase != menuPhaseEnterPassword {
		t.Fatalf("expected menuPhaseEnterPassword, got %v", m.phase)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from password Esc")
	}
}

func TestMenuModel_SelectActionNavigation(t *testing.T) {
	// Column-major 3x2 grid: get(0,0) add(1,0) change(0,1) remove(1,1)
	// settings(0,2) rollback(1,2). h/l step a whole column and clamp;
	// j/k step one entry and wrap.
	cases := []struct {
		name  string
		start int
		key   rune
		want  int
	}{
		{"l from get to change", 0, 'l', 2},
		{"h from change to get", 2, 'h', 0},
		{"j from get to add", 0, 'j', 1},
		{"j from add to change", 1, 'j', 2},
		{"j from rollback wraps to get", 5, 'j', 0},
		{"k from get wraps to rollback", 0, 'k', 5},
		{"k from change to add", 2, 'k', 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMenuModel()
			m.phase = menuPhaseSelectAction
			m.actionCursor = tc.start
			tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})
			if m.actionCursor != tc.want {
				t.Fatalf("cursor = %d, want %d", m.actionCursor, tc.want)
			}
		})
	}
}

func TestMenuModel_SelectActionSpaceConfirms(t *testing.T) {
	// Space confirms the focused action, like enter.
	m := NewMenuModel()
	m.phase = menuPhaseSelectAction
	tuiutil.UpdateInPlace(&m, tea.KeyPressMsg{Code: ' ', Text: " "})
	if m.phase != menuPhaseRunningAction {
		t.Fatalf("expected menuPhaseRunningAction after space, got %v", m.phase)
	}
}

func TestMenuModel_SelectActionQQuits(t *testing.T) {
	m := NewMenuModel()
	m.phase = menuPhaseSelectAction
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from q at action-select")
	}
}

func TestMenuModel_RunningActionCompletionReturnsToSelect(t *testing.T) {
	m := NewMenuModel()
	m.phase = menuPhaseRunningAction
	m.activeAction = fakeAction{output: []string{"recordX done"}}
	tuiutil.UpdateInPlace(&m, fakeFinishMsg{})
	if m.phase != menuPhaseSelectAction {
		t.Fatalf("expected menuPhaseSelectAction after action.Done(), got %v", m.phase)
	}
	if m.activeAction != nil {
		t.Fatalf("activeAction should be nil after completion")
	}
	if m.lastOutput != "recordX done" {
		t.Fatalf("lastOutput = %q, want %q", m.lastOutput, "recordX done")
	}
}

func TestMenuModel_RunningActionNewPasswordPropagates(t *testing.T) {
	m := NewMenuModel()
	m.password = memguard.NewEnclave([]byte("old"))
	newEnc := memguard.NewEnclave([]byte("new"))
	m.phase = menuPhaseRunningAction
	m.activeAction = fakeAction{newPassword: newEnc}
	tuiutil.UpdateInPlace(&m, fakeFinishMsg{})
	if got := string(enclaveBytes(t, m.password)); got != "new" {
		t.Fatalf("password = %q, want %q", got, "new")
	}
}

func TestMenuModel_RunningActionCancelReturnsToSelectWithoutOutput(t *testing.T) {
	m := NewMenuModel()
	m.phase = menuPhaseRunningAction
	m.activeAction = fakeAction{cancelled: true, output: []string{"should-not-leak"}}
	tuiutil.UpdateInPlace(&m, struct{}{})
	if m.phase != menuPhaseSelectAction {
		t.Fatalf("expected menuPhaseSelectAction after cancellation, got %v", m.phase)
	}
	if m.lastOutput != "" {
		t.Fatalf("lastOutput should stay empty on cancel, got %q", m.lastOutput)
	}
}
