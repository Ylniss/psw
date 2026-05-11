package menu

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/storage"
)

// formatLoadError swaps fork-undecryptable for its user-facing banner.
func formatLoadError(err error) string {
	if errors.Is(err, storage.ErrForkUndecryptable) {
		return storage.ForkUndecryptableUserMessage
	}
	return err.Error()
}

// loadCmd decrypts storage; pull=true also fetches and merges first.
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
		warns.drain() // discard noise from before this run
		err := storage.GitPullAndMerge(password)
		return pullDoneMsg{err: err, warnings: warns.drain()}
	}
}

// decryptCmd loads storage.psw with the cached password. Assumes the vault
// and git repo already exist.
func decryptCmd(password string) tea.Cmd {
	return func() tea.Msg {
		s, err := storage.Decrypt(password)
		return storageLoadedMsg{store: s, err: err}
	}
}

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

// applyRollbackCmd runs storage.ApplyRollback (which owns the commit subject).
func applyRollbackCmd(s *storage.Storage, target storage.LogEntry, records []storage.Record) tea.Cmd {
	return func() tea.Msg {
		return rollbackAppliedMsg{err: storage.ApplyRollback(s, target, records)}
	}
}

// saveConfigCmd runs storage.WriteAndCommitConfig("config updated").
// Shared with `psw config set`.
func saveConfigCmd() tea.Cmd {
	return func() tea.Msg {
		return configSavedMsg{err: storage.WriteAndCommitConfig("config updated")}
	}
}
