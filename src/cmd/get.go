package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(changeCmd)
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get secrets under specified name",
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Get command executed\n")
	},
}
