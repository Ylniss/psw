package prompt

import (
	"errors"
	"os"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// ErrPromptCancelled means user pressed Esc/Ctrl-C. Callers exit silently.
var ErrPromptCancelled = errors.New("prompt cancelled")

// PasswordMismatchMsg is the user-facing line shown when password + repeat differ.
// Exported so the menu can render it with its own styling.
const PasswordMismatchMsg = "Passwords don't match, try again"

var (
	errRequired      = errors.New("required")
	errNoTTY         = errors.New("interactive prompt required: stdin is not a terminal")
	promptErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func validateRequired(content string) error {
	if len(content) < 1 {
		return errRequired
	}
	return nil
}

// IsTTY reports whether stdin is an interactive terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
