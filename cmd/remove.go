package cmd

import (
	"fmt"

	"github.com/TwiN/go-color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

func init() {
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use: `remove [name] [flags]

Arguments:
  name    Optional name of the record to remove. If omitted, you'll be prompted to provide it`,
	Short: "Remove chosen record",
	Long:  `Remove chosen record, all its data will be lost permanently`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var recordName string
		if len(args) == 0 {
			recordName, err = strg.GetRecordNameWithFzf(storage.GetNames())
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			recordName, err = strg.GetRecordNameWithFzf(storage.GetNamesWithPart(args[0]))
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		if !storage.Exists(recordName) {
			fmt.Printf("Record with name %s doesn't exists\n", color.InGreen(recordName))
			return
		}

		storage.RemoveRecord(recordName)

		storageJson, err := storage.ToJson()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		log.Debugf("new storage content:\n%s\n", storageJson)

		err = strg.EncryptStringToStorage(storageJson, storage.MainPass)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Printf("Record %s successfully removed\n", color.InGreen(recordName))
		err = strg.GitSync("record removed")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	},
}
