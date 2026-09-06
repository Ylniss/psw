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
	Use: `remove [name...] [flags]

Arguments:
  name    Optional record name(s). With --exact, every listed name must match an existing record. Without --exact, an interactive picker opens; one substring filter argument is allowed.`,
	Short: "Remove chosen records",
	Long:  `Remove chosen records, all their data will be lost permanently`,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.GetOrCreateForMutate()
		if err != nil {
			return handleCmdErr(err)
		}

		names, err := resolveRecordNames(store, args, removeExactFlag)
		if err != nil {
			return handleCmdErr(err)
		}
		if len(names) == 0 {
			return nil
		}

		for _, n := range names {
			store.RemoveRecord(n)
		}

		if err := store.Save(); err != nil {
			return handleCmdErr(err)
		}

		for _, n := range names {
			fmt.Printf("Removed %s\n", color.InGreen(n))
		}
		storage.GitCommit(storage.RemoveCommitMessage(len(names)))
		return nil
	},
}
