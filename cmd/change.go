package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(changeCmd)
}

var changeCmd = &cobra.Command{
	Use:   "change",
	Short: "Change current password to a new one",
	Run: func(cmd *cobra.Command, args []string) {
	},
}
