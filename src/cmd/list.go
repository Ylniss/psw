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
		log.Debugf("Storage path: %s\n", storagePath)
		password, err := utils.PromptForPassword("Password")
		if err != nil {
			log.Fatalln(err)
		}
		storageContent, err := utils.DecryptStringFromFile(storageFile, password)
		// todo: seek names only that are one line below marker always
		records := strings.Split(storageContent, recordMarker)
		names := lo.Map(records, func(record string, _ int) string { return strings.Split(record, "(;+!_+_!+;)")[0] })

		// fmt.Println(len(records))
		// // Iterate over the records, starting from index 1 because index 0 is before the first recordMarker
		// for i, record := range records {
		// 	if i == 0 {
		// 		// Skip the content before the first recordMarker
		// 		continue
		// 	}
		// 	fmt.Println(record)
		// }

		fmt.Printf("records: %v\n", records)
		fmt.Printf("names: %v\n", names)
	},
}
