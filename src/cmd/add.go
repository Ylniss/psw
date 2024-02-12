package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add new named secrets",
	Long:  `Add username/password or a value that will be stored in provided filename`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Add command executed\n")
	},
}
