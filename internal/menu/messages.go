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

// pullDoneMsg carries warnings captured via storage.WarnSink during the pull
// so the action can append them to its transcript.
type pullDoneMsg struct {
	err      error
	warnings []string
}
