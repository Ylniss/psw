package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/manifoldco/promptui"
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

		isStorageEmpty, err := isPathEmpty(storagePath)
		if err != nil {
			log.Fatalln(err)
		}

		if isStorageEmpty {
			password, err := propmptForPassword("Enter main password")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			passwordRepat, err := propmptForPassword("Repeat main password")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			if password != passwordRepat {
				fmt.Println("Passwords don't match, try again")
				return
			}

			fmt.Println(password)
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

func propmptForPassword(text string) (string, error) {
	validate := func(input string) error {
		if len(input) < 4 {
			return errors.New("Password must be at least 4 characters long")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    text,
		Mask:     '*',
		Validate: validate,
	}

	return prompt.Run()
}
