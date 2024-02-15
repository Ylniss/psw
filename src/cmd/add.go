package cmd

import (
	"errors"
	"io"
	"os"

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

		isStorageEmpty, err := isPathEmpty(storagePath)
		if err != nil {
			log.Fatalln(err)
		}

		if isStorageEmpty {
			err = firstTimeCreateEncryptedStorage()
			if err != nil {
				log.Fatalln(err)
			}
		}

		password, err := utils.PromptForPassword("Password")
		if err != nil {
			log.Fatalln(err)
		}

		storageContent, err := utils.DecryptStringFromFile(storageFile, password)
		if err != nil {
			log.Fatalln(err)
		}
		// todo: prompt for user and password and get name from args (if no name in args also prompt for it before)
		// add global line ending marker with complex structure (;+!_+_!+;)
		storageContent += recordMarker + "\nuser2000(;+!_+_!+;)\n" + "haseu" + "(;+!_+_!+;)\n"

		log.Debugf("new storage content:\n%s\n", storageContent)

		err = utils.EncryptStringToFile(storageFile, storageContent, password)
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func isPathEmpty(path string) (bool, error) {
	dir, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	_, err = dir.Readdir(1) // Attempt to read at least one entry from the directory.
	if err == nil {
		return false, nil // Path is not empty.
	}
	if err == io.EOF {
		return true, nil // Path is empty.
	}
	return false, err
}

func firstTimeCreateEncryptedStorage() error {
	password, err := utils.PromptForPassword("Enter main password")
	if err != nil {
		return err
	}

	passwordRepat, err := utils.PromptForPassword("Repeat main password")
	if err != nil {
		return err
	}

	if password != passwordRepat {
		return errors.New("Passwords don't match, try again")
	}

	err = utils.EncryptStringToFile(storageFile, "", password)
	if err != nil {
		return err
	}

	return nil
}
