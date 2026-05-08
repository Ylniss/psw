package cli

import (
	"errors"
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

var removeExactFlag bool

func init() {
	removeCmd.Flags().BoolVarP(&removeExactFlag, "exact", "e", false, "exact name match; skip interactive picker and substring search")
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use: `remove [name] [flags]

Arguments:
  name    Optional name of the record to remove. If omitted, you'll be prompted to provide it`,
	Short: "Remove chosen record",
	Long:  `Remove chosen record, all its data will be lost permanently`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.GetOrCreateForMutate()
		if errors.Is(err, prompt.ErrPromptCancelled) {
			return nil
		}
		if errors.Is(err, storage.ErrForkUndecryptable) {
			printForkUndecryptable()
			return errExit
		}
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		recordName, err := resolveRecordName(store, args, removeExactFlag)
		if errors.Is(err, errExit) {
			return errExit
		}
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
		if recordName == "" {
			return nil
		}

		if !store.Exists(recordName) {
			fmt.Printf("Record with name %s doesn't exist\n", color.InGreen(recordName))
			return nil
		}

		store.RemoveRecord(recordName)

		if err := store.Save(); err != nil {
			fmt.Println(err.Error())
			return nil
		}

		fmt.Printf("Record %s successfully removed", color.InGreen(recordName))
		storage.GitCommit("record removed")
		return nil
	},
}
