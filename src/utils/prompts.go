package utils

import (
	"errors"
	"fmt"
	"os"

	color "github.com/TwiN/go-color"
	"github.com/manifoldco/promptui"
)

func PromptForPassword(text string) (string, error) {
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

func GetStorageContentOrCreateIfNotExists(storageFilePath string) (storageContent string, password string, err error) {
	password, created, err := createEncryptedStorageIfNotExists(storageFilePath)
	if err != nil {
		return "", password, err
	}

	// when storage already exists, prompt for password to access
	if !created && password == "" {
		password, err = PromptForPassword("Password")
		if err != nil {
			return "", password, err
		}
	}

	storageContent, err = DecryptStringFromFile(storageFilePath, password)
	if err != nil {
		return "", password, err
	}
	return storageContent, password, nil
}

// returns true and password used to create storage if created storage
// or false with empty string when error occured or storage already existed
func createEncryptedStorageIfNotExists(storageFilePath string) (string, bool, error) {
	storageFileExists, err := fileExists(storageFilePath)
	if err != nil {
		return "", false, err
	}

	if storageFileExists {
		return "", false, nil
	}

	fmt.Println("No encrypted storage found. Set your main password that will be used to decrypt your secrets.")

	password, err := PromptForPassword("Enter main password")
	if err != nil {
		return "", false, err
	}

	passwordRepat, err := PromptForPassword("Repeat main password")
	if err != nil {
		return "", false, err
	}

	if password != passwordRepat {
		return "", false, errors.New("Passwords don't match, try again")
	} else {
		fmt.Println(color.Ize(color.Green, "Main password set successfully"))
	}

	err = EncryptStringToFile(storageFilePath, "", password)
	if err != nil {
		return "", false, err
	}

	return password, true, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("Error when checking if file %s exists:\n%w", path, err)
}
