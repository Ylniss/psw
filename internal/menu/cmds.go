package menu

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/storage"
)

// loadCmd decrypts storage without touching the network.
func loadCmd(password *memguard.Enclave) tea.Cmd {
	return func() tea.Msg {
		s, err := storage.LoadOrCreate(password, false)
		return storageLoadedMsg{store: s, err: err}
	}
}

// pullCmd runs storage.GitPullAndMerge and captures pull-time warnings
// (timeout fallback, merge summary). If a merge ran, the returned store
// is already decrypted so callers skip a second Argon2id derive.
func pullCmd(password *memguard.Enclave) tea.Cmd {
	return func() tea.Msg {
		warns.drain() // discard noise from before this run
		store, err := storage.GitPullAndMerge(password)
		return pullDoneMsg{store: store, err: err, warnings: warns.drain()}
	}
}

// decryptCmd loads storage.psw with the cached password. Assumes the vault
// and git repo already exist.
func decryptCmd(password *memguard.Enclave) tea.Cmd {
	return func() tea.Msg {
		s, err := storage.Decrypt(password)
		return storageLoadedMsg{store: s, err: err}
	}
}

func validatePasswordCmd(password *memguard.Enclave) tea.Cmd {
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

// tea.Every (not Tick) so the displayed seconds align with the wall clock.
func countdownTick() tea.Cmd {
	return tea.Every(time.Second, func(time.Time) tea.Msg { return countdownTickMsg{} })
}
