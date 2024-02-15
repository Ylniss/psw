package cmd

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	storagePath  string
	storageFile  string
	recordMarker string
)

var rootCmd = &cobra.Command{
	Use:   "psw",
	Short: "psw is a simple password manager",
	Long: `A password manager using AES encryption that stores your
passwords in a separate files that are easy to backup.`,
	Version: "0.2",
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
	var err error
	storagePath, err = expandPathWithHomePrefix(path)
	if err != nil {
		return err
	}

	err = ensureDirExists(storagePath)
	if err != nil {
		return err
	}

	storageFile = storagePath + "/storage.psw"

	return nil
}

func SetRecordMarker(marker string) {
	recordMarker = marker
}

func ensureDirExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return err
		}
		log.Debugf("Directory created: %s\n", path)
	} else if err != nil {
		return err
	}

	return nil
}

func expandPathWithHomePrefix(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = strings.Replace(path, "~", home, 1)
	}

	return path, nil
}
