package strg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type StorageCfg struct {
	storagePath     string
	storageFilePath string
	storageFileName string
	recordMarker    string
	valueEndMarker  string
}

var cfg = StorageCfg{
	recordMarker:    "!===##$$##$$##$$##$$===!\n",
	valueEndMarker:  "(;+!_+_!+;)\n",
	storageFileName: "storage.psw",
}

func init() {
	// todo: instead of env var will use confing file
	err := setStoragePaths(os.Getenv("PSW_STORAGE_DIR"))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func setStoragePaths(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Error while retrieving home directory:\n%w", err)
		}

		// by default fallback to ~/.psw as a storage directory
		path = filepath.Join(home, ".psw")
	}

	var err error
	cfg.storagePath, err = expandPathWithHomePrefix(path)
	if err != nil {
		return err
	}

	err = ensureDirExists(cfg.storagePath)
	if err != nil {
		return err
	}

	if cfg.storageFileName == "" {
		return errors.New("Error when setting storage paths, storage file name is not set")
	}

	cfg.storageFilePath = filepath.Join(cfg.storagePath, cfg.storageFileName)

	return nil
}
