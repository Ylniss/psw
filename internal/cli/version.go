package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of psw",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("psw v%s\n", Version)
	},
}
