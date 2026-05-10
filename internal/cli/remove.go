package cli

import (
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/storage"
)

var removeExactFlag bool

func init() {
	removeCmd.Flags().BoolVarP(&removeExactFlag, "exact", "e", false, "exact name match; skip interactive picker and substring search")
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
		if done, ret := handleCmdErr(err); done {
			return ret
		}

		recordName, err := resolveRecordName(store, args, removeExactFlag, nil)
		if done, ret := handleCmdErr(err); done {
			return ret
		}
		if recordName == "" {
			return nil
		}

		if !store.Exists(recordName) {
			fmt.Printf("Record with name %s doesn't exist\n", color.InGreen(recordName))
			return nil
		}

		store.RemoveRecord(recordName)

		if done, ret := handleCmdErr(store.Save()); done {
			return ret
		}

		fmt.Printf("Record %s successfully removed\n", color.InGreen(recordName))
		storage.GitCommit("record removed")
		return nil
	},
}
