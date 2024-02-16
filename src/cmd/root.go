package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type AppVars struct {
	storagePath     string
	storageFilePath string
	storageFileName string
}

var app AppVars

var rootCmd = &cobra.Command{
	Use:   "psw",
	Short: "psw is a simple password manager",
	Long: `psw is a simple password manager that employs AES SHA256 encryption to secure your passwords.
It consolidates your passwords into a single file, safeguarded by a main password you choose.
The initial interaction with any command prompts the setup of this main password. For added flexibility,
you can customize the storage file's default directory by setting the PSW_STORAGE_DIR environment variable
in your shell configuration file.`,
	Version: "0.3",
	Run: func(cmd *cobra.Command, args []string) {
		// display help when running just psw command
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func SetStoragePaths(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Error while retrieving home directory:\n%w", err)
		}

		// by default fallback to home directory as a storage directory
		path = home
	}

	var err error
	app.storagePath, err = expandPathWithHomePrefix(path)
	if err != nil {
		return err
	}

	err = ensureDirExists(app.storagePath)
	if err != nil {
		return err
	}

	if app.storageFileName == "" {
		return errors.New("Error when setting storage paths, storage file name is not set")
	}

	app.storageFilePath = filepath.Join(app.storagePath, app.storageFileName)

	return nil
}

func SetStorageFileName(name string) {
	app.storageFileName = name
}

func ensureDirExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("Error when trying to create directory:\n%w", err)
		}
		log.Debugf("Directory created: %s\n", path)
	} else if err != nil {
		return fmt.Errorf("Error when trying to check directory:\n%w", err)
	}

	return nil
}

func expandPathWithHomePrefix(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("Error while trying to epand ~ in directory as home:\n%w", err)
		}
		path = strings.Replace(path, "~", home, 1)
	}

	return path, nil
}
