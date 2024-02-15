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
	Short: "Add new named secrets",
	Long:  `Add username/password or a value that will be stored in provided filename`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Add command executed\n")

		password, created, err := utils.CreateEncryptedStorageIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// when storage already exists, prompt for password to access
		if !created && password == "" {
			password, err = utils.PromptForPassword("Password")
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		storageContent, err := utils.DecryptStringFromFile(app.storageFilePath, password)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// todo: prompt for user and password and get name from args (if no name in args also prompt for it before)
		storageContent += app.recordMarker + "\n" + "asdffewwf" + app.valueEndMarker + "\n" + "haseu" + app.valueEndMarker + "\n"

		log.Debugf("new storage content:\n%s\n", storageContent)

		err = utils.EncryptStringToFile(app.storageFilePath, storageContent, password)
		if err != nil {
			fmt.Println(err.Error())
		}
	},
}
