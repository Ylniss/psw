package utils

import (
	"fmt"
	"os"

	color "github.com/TwiN/go-color"
)

func GetStorageContentOrCreateIfNotExists(storageFilePath string) (storageContent string, password string, err error) {
	password, created, err := createEncryptedStorageIfNotExists(storageFilePath)
	if err != nil {
		return "", password, err
	}

	// when storage already exists, prompt for password to access
	if !created && password == "" {
		password, err = PromptForMainPass(false)
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

	password, err := PromptForMainPass(true)
	if err != nil {
		return "", false, err
	}

	err = EncryptStringToFile(storageFilePath, "", password)
	fmt.Println(color.Ize(color.Green, "Main password set successfully"))

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
