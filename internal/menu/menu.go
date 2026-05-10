// Package menu is the persistent `psw menu` TUI.
package menu

import (
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/ui"
)

// Captured storage warnings. The bubbletea cmd goroutine writes via
// appendWarning; the main loop reads via drainWarnings.
var (
	warnMu       sync.Mutex
	warnCaptured []string
)

func appendWarning(s string) {
	warnMu.Lock()
	warnCaptured = append(warnCaptured, s)
	warnMu.Unlock()
}

// drainWarnings returns and clears the captured warning slice.
func drainWarnings() []string {
	warnMu.Lock()
	defer warnMu.Unlock()
	out := warnCaptured
	warnCaptured = nil
	return out
}

// Run blocks until the user quits. Caller does the TTY check.
func Run() error {
	// The menu owns the screen; suppress nested spinners and route storage
	// warnings into the transcript instead of leaking onto stderr.
	ui.SpinnersQuiet = true
	storage.WarnSink = appendWarning
	defer func() {
		ui.SpinnersQuiet = false
		storage.WarnSink = nil
		drainWarnings()
	}()
	if _, err := tea.NewProgram(NewMenuModel()).Run(); err != nil {
		return fmt.Errorf("menu: %w", err)
	}
	return nil
}
