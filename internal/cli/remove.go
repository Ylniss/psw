package cli

import (
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/strg"
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
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		recordName, err := resolveRecordName(storage, args, removeExactFlag)
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
		if recordName == "" {
			return nil
		}

		if !storage.Exists(recordName) {
			fmt.Printf("Record with name %s doesn't exists\n", color.InGreen(recordName))
			return nil
		}

		storage.RemoveRecord(recordName)

		if err := storage.Save(); err != nil {
			fmt.Println(err.Error())
			return nil
		}

		fmt.Printf("Record %s successfully removed", color.InGreen(recordName))
		strg.GitCommit("record removed")
		return nil
	},
}
