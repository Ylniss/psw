package menu

import "github.com/ylniss/psw/internal/storage"

// storageLoadedMsg is the result of loadCmd.
type storageLoadedMsg struct {
	store *storage.Storage
	err   error
}

// storageSavedMsg is the result of saveCmd.
type storageSavedMsg struct {
	err error
}

// passwordValidatedMsg is the result of validatePasswordCmd.
type passwordValidatedMsg struct {
	err error
}
