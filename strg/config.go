package strg

import (
	"errors"
	"fmt"
	"github.com/pelletier/go-toml"
	"os"
	"path/filepath"
)

type StorageCfg struct {
	storagePath     string
	storageFilePath string
	storageFileName string
	configFileName  string
}

var Cfg = StorageCfg{
	storageFileName: "storage.psw",
	configFileName:  "pswcfg.toml",
}

type Config struct {
	Psw PswConfig `toml:"psw"`
}

type PswConfig struct {
	ClipboardTimeout int `toml:"clipboard_timeout"`
}

func LoadConfig() (*Config, error) {
	path := filepath.Join(Cfg.storageFilePath, "pswcfg.toml")

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := toml.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

func init() {
	err := setStoragePaths("")
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
	Cfg.storagePath, err = expandPathWithHomePrefix(path)
	if err != nil {
		return err
	}

	err = ensureDirExists(Cfg.storagePath)
	if err != nil {
		return err
	}

	if Cfg.storageFileName == "" {
		return errors.New("Error when setting storage paths, storage file name is not set")
	}

	Cfg.storageFilePath = filepath.Join(Cfg.storagePath, Cfg.storageFileName)

	return nil
}
