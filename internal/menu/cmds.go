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

// pullCmd runs storage.GitPullAndMerge and captures any warnings emitted
// during the pull (timeout fallback, merge summary lines).
func pullCmd(password string) tea.Cmd {
	return func() tea.Msg {
		drainWarnings() // discard noise from before this run
		err := storage.GitPullAndMerge(password)
		return pullDoneMsg{err: err, warnings: drainWarnings()}
	}
}

// decryptCmd loads storage.psw with the cached password. Assumes the vault
// and git repo already exist (validatePasswordCmd ran earlier).
func decryptCmd(password string) tea.Cmd {
	return func() tea.Msg {
		s, err := storage.Get(password)
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
