package menu

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/storage"
)

// loadCmd decrypts storage. pull=true also fetches and merges from remote.
func loadCmd(password string, pull bool) tea.Cmd {
	return func() tea.Msg {
		s, err := storage.LoadOrCreate(password, pull)
		return storageLoadedMsg{store: s, err: err}
	}
}

// validatePasswordCmd decrypts (or creates) the vault and reports the result.
func validatePasswordCmd(password string) tea.Cmd {
	return func() tea.Msg {
		_, err := storage.LoadOrCreate(password, false)
		return passwordValidatedMsg{err: err}
	}
}

// saveCmd encrypts s, commits, and best-effort pushes. Push errors are
// printed by storage and not propagated.
func saveCmd(s *storage.Storage, commitMsg string) tea.Cmd {
	return func() tea.Msg {
		if err := s.Save(); err != nil {
			return storageSavedMsg{err: err}
		}
		_ = storage.GitCommit(commitMsg)
		return storageSavedMsg{}
	}
}
