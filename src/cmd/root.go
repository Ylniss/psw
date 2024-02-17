package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

type AppVars struct {
	storagePath     string
	storageFilePath string
	storageFileName string
}

var app AppVars

var rootCmd = &cobra.Command{
	Use: `psw         lists all record names
  psw`,
	Short: "psw is a simple password manager",
	Long: `psw is a simple password manager that employs AES SHA256 encryption to secure your passwords.
It consolidates your passwords into a single file, safeguarded by a main password you choose.
The initial interaction with any command prompts the setup of this main password. For added flexibility,
you can customize the storage file's default directory by setting the PSW_STORAGE_DIR environment variable
in your shell configuration file.`,
	Version: "0.4",
	Run: func(cmd *cobra.Command, args []string) {
		// list all record names on 'psw' command
		storage, err := strg.GetOrCreateIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		names := storage.GetNames()
		if len(names) == 0 {
			fmt.Println("No secrets found. Use 'add' command first.")
			return
		}

		for _, name := range names {
			fmt.Println(name)
		}
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

		// by default fallback to ~/.psw as a storage directory
		path = filepath.Join(home, ".psw")
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
