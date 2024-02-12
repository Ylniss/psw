package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "psw",
	Short: "psw is a simple password manager",
	Long: `
    A password manager using AES encryption that stores your
    passwords in a separate files that are easy to backup.`,
	Version: "0.1",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
