package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/ylniss/psw/utils"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add new record with secrets",
	Long:  `Add username/password or a value that will be stored in a record with provided name`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Add command executed\n")

		storageContent, mainPass, err := utils.GetStorageContentOrCreateIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		recordName, err := getRecordName(args)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		username, err := utils.PromptForName("Username")
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		recordPass, err := utils.PromptForRecordPass()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		storageContent += app.recordMarker + "\n" + recordName + app.valueEndMarker + "\n" + username + app.valueEndMarker + "\n" + recordPass + app.valueEndMarker + "\n"

		log.Debugf("new storage content:\n%s\n", storageContent)

		err = utils.EncryptStringToFile(app.storageFilePath, storageContent, mainPass)
		if err != nil {
			fmt.Println(err.Error())
		}
	},
}

func getRecordName(args []string) (string, error) {
	var recordName string
	var err error
	if len(args) == 0 {
		recordName, err = utils.PromptForName("Record name")
		if err != nil {
			return "", err
		}
	} else {
		recordName = args[0]
	}
	return recordName, nil
}
