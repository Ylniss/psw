package cli

import (
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

func changeMainPassword() error {
	store, err := storage.GetOrCreateForMutate()
	if done, ret := handleCmdErr(err); done {
		return ret
	}
	return changeMainPasswordOnStore(store)
}

// changeMainPasswordOnStore lets the picker path reuse the store loaded by
// changeRecord instead of re-prompting via GetOrCreateForMutate.
func changeMainPasswordOnStore(store *storage.Storage) error {
	fmt.Println(color.InCyan("You are changing your main password!"))
	newMainPassword, err := prompt.PromptForMainPasswordChangeEnclave()
	if done, ret := handleCmdErr(err); done {
		return ret
	}
	store.MainPassword = newMainPassword
	if done, ret := handleCmdErr(store.Save()); done {
		return ret
	}
	fmt.Println(color.InGreen("Main password changed"))
	return nil
}

func rejectFieldFlagsForMain(cmd *cobra.Command) error {
	if cmd.Flags().Changed("rename") || cmd.Flags().Changed("username") ||
		cmd.Flags().Changed("password") || cmd.Flags().Changed("value") {
		fmt.Println("Record-level flags (--rename/--username/--password/--value) are not valid with 'change main' / 'change main-password'")
		return errSilentExit
	}
	return nil
}
