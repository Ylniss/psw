package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

var rootCmd = &cobra.Command{
	Use: `psw         lists all record names
  psw`,
	Short: "psw is a simple password manager",
	Long: `psw is a simple password manager that employs AES SHA256 encryption to secure your passwords.
It consolidates your passwords into a single file, safeguarded by a main password you choose.
The initial interaction with any command prompts the setup of this main password. For added flexibility,
you can customize the storage file's default directory by setting the PSW_STORAGE_DIR environment variable
in your shell configuration file.`,
	Version: "0.4",
	Run: func(cmd *cobra.Command, args []string) {
		// list all record names on 'psw' command
		storage, err := strg.GetOrCreateIfNotExists()
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
