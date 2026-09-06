package cli

import (
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

// changeMainPassword re-encrypts the already-loaded store under a new main
// password, then commits. Callers reject the record-field flags first, before
// the store loads — a bad flag combination shouldn't cost a password prompt.
func changeMainPassword(store *storage.Storage) error {
	fmt.Println(color.InCyan("Changing your main password"))
	newMainPassword, err := prompt.PromptMainPasswordChange()
	if err != nil {
		return handleCmdErr(err)
	}
	store.MainPassword = newMainPassword
	if err := store.Save(); err != nil {
		return handleCmdErr(err)
	}
	fmt.Println(color.InGreen("Main password changed"))
	storage.GitCommit("main password changed")
	return nil
}

func rejectFieldFlagsForMain(cmd *cobra.Command) error {
	if cmd.Flags().Changed("rename") || cmd.Flags().Changed("username") ||
		cmd.Flags().Changed("password") || cmd.Flags().Changed("value") {
		fmt.Printf("--rename/--username/--password/--value cannot be used with %s\n", color.InCyan("change main"))
		return errSilentExit
	}
	return nil
}
