// Package menu is the persistent `psw menu` TUI.
package menu

import (
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/ui"
)

// warnCollector funnels storage.Warn output into the menu transcript.
// The bubbletea cmd goroutine writes via append; the main loop reads via drain.
type warnCollector struct {
	mu    sync.Mutex
	lines []string
}

func (c *warnCollector) append(s string) {
	c.mu.Lock()
	c.lines = append(c.lines, s)
	c.mu.Unlock()
}

func (c *warnCollector) drain() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.lines
	c.lines = nil
	return out
}

// warns is the active collector for the running menu session.
// storage.WarnSink is a package-level hook by design (the storage package
// can't depend on menu); the collector itself is reentrancy-safe but only
// one menu Run() owns the sink at a time.
var warns warnCollector

// Run blocks until the user quits. Caller does the TTY check.
func Run() error {
	// The menu owns the screen; suppress nested spinners and route storage
	// warnings into the transcript instead of leaking onto stderr.
	ui.SpinnersQuiet = true
	storage.WarnSink = warns.append
	defer func() {
		ui.SpinnersQuiet = false
		storage.WarnSink = nil
		warns.drain()
	}()
	if _, err := tea.NewProgram(NewMenuModel()).Run(); err != nil {
		return fmt.Errorf("menu: %w", err)
	}
	return nil
}
