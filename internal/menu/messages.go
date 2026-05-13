package menu

import "github.com/ylniss/psw/internal/storage"

type storageLoadedMsg struct {
	store *storage.Storage
	err   error
}

type storageSavedMsg struct {
	err error
}

type passwordValidatedMsg struct {
	err error
}

// pullDoneMsg returns warnings captured via storage.WarnSink for the
// transcript. Non-nil store means the merge decrypted; callers skip decryptCmd.
type pullDoneMsg struct {
	store    *storage.Storage
	err      error
	warnings []string
}

// rollbackAppliedMsg signals storage.ApplyRollback returned.
type rollbackAppliedMsg struct {
	err error
}

// configSavedMsg signals WriteAndCommitConfig returned.
type configSavedMsg struct {
	err error
}

type countdownTickMsg struct{}
