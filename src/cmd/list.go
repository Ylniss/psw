package cmd

import (
	"fmt"

	"github.com/ylniss/psw/strg"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all files storing secrets",
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		names := storage.GetNames()
		if len(names) == 0 {
			fmt.Println("No secrets found. Use 'add' command first.")
			return
		}

		for _, name := range names {
			fmt.Println(name)
		}
	},
}
