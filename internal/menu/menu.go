// Package menu is the persistent `psw menu` TUI.
package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/ui"
)

// Run blocks until the user quits. Caller does the TTY check.
func Run() error {
	// The menu owns the screen; suppress nested spinners.
	ui.SpinnersQuiet = true
	defer func() { ui.SpinnersQuiet = false }()
	if _, err := tea.NewProgram(NewMenuModel()).Run(); err != nil {
		return fmt.Errorf("menu: %w", err)
	}
	return nil
}
