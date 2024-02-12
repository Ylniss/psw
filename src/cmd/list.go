package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all files storing secrets",
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("List command executed\n")
		log.Debugf("Storage path: %s\n", storagePath)
	},
}
