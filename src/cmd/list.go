package cmd

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/ylniss/psw/utils"

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
		log.Debugf("Storage path: %s\n", app.storagePath)

		storageContent, _, err := utils.GetStorageContentOrCreateIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		records := strings.Split(storageContent, app.recordMarker)
		names := lo.Map(records, func(record string, _ int) string {
			return strings.Split(record, app.valueEndMarker)[0]
		})

		if len(names) == 1 && names[0] == "" {
			fmt.Println("No secrets found. Use 'add' command first.")
			return
		}

		for _, name := range names {
			fmt.Print(name)
		}
		fmt.Println()
	},
}
