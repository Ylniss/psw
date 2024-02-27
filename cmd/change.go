package cmd

import (
	"fmt"

	"github.com/TwiN/go-color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/prmpt"
	"github.com/ylniss/psw/strg"
)

func init() {
	rootCmd.AddCommand(changeCmd)
}

var changeCmd = &cobra.Command{
	Use: `change [name] [flags]

Arguments:
  name    Optional name of the record to change. If omitted, you'll be prompted to provide it`,
	Short: "Change chosen record data",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var recordName string
		if len(args) == 0 {
			recordName, err = strg.GetRecordNameWithFzf(storage)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			recordName = args[0]
		}

		if recordName == "main" {
			// todo : prpompt for old main pass and recreate whole encryption with newpass

			storageJson, err := storage.ToJson()
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			err = strg.EncryptStringToStorage(storageJson, "todo: prompted password")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			fmt.Printf("Record %s updated\n", color.InGreen(recordName))

			return
		}

		record, isFound := storage.GetRecord(recordName)

		log.Debugf("cmd/change - record: %#v\n", record)

		if !isFound {
			fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
			return
		}

		if yes := prmpt.YesOrNo("Do you want to change your name?"); yes {
			newName, err := prmpt.PromptForName("New name")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			record.Name = newName
		}

		if record.Value != "" {
		}

		storage.UpdateRecord(recordName, record)

		storageJson, err := storage.ToJson()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		log.Debugf("updated storage content:\n%s\n", storageJson)

		err = strg.EncryptStringToStorage(storageJson, storage.MainPass)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Printf("Record %s updated\n", color.InGreen(recordName))
	},
}
