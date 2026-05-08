package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/pelletier/go-toml/v2"
)

type StorageConfig struct {
	storagePath     string
	storageFilePath string
	storageFileName string
	configFilePath  string
	configFileName  string
	gitRepoExists   bool
}

var Paths = StorageConfig{
	storageFileName: "storage.psw",
	configFileName:  "pswcfg.toml",
	gitRepoExists:   false,
}

type Config struct {
	ClipboardTimeout int    `toml:"clipboard_timeout"`
	Remote           string `toml:"remote"`
}

var AppConfig Config

func InitConfig() error {
	if err := setStoragePath(); err != nil {
		return err
	}
	if err := loadConfig(); err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	return nil
}

func setStoragePath() error {
	// PSW_HOME overrides default storage dir (intended for tests/scripting).
	path := os.Getenv("PSW_HOME")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("retrieve home directory: %w", err)
		}
		path = filepath.Join(home, ".psw")
	}

	var err error
	Paths.storagePath, err = expandPathWithHomePrefix(path)
	if err != nil {
		return err
	}

	err = ensureDirExists(Paths.storagePath)
	if err != nil {
		return err
	}

	if Paths.storageFileName == "" {
		return errors.New("error when setting storage path, storage file name is not set")
	}

	Paths.storageFilePath = filepath.Join(Paths.storagePath, Paths.storageFileName)

	return nil
}

func loadConfig() error {
	Paths.configFilePath = filepath.Join(Paths.storagePath, Paths.configFileName)

	if err := ensureUserConfig(); err != nil {
		return err
	}

	if err := readConfigFile(); err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}
	return nil
}

func ensureUserConfig() error {
	exists, err := pathExists(Paths.configFilePath)
	if err != nil {
		return fmt.Errorf("error checking config file existence: %w", err)
	}
	if exists {
		return nil
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine executable path: %w", err)
	}
	binConfigPath := filepath.Join(filepath.Dir(binPath), Paths.configFileName)

	binExists, err := pathExists(binConfigPath)
	if err != nil {
		return fmt.Errorf("error checking binary config file existence: %w", err)
	}
	if !binExists {
		return errors.New("config file does not exist in the binary location")
	}

	if err := copyFile(binConfigPath, Paths.configFilePath); err != nil {
		return fmt.Errorf("failed to copy config file from %s to %s: %w", binConfigPath, Paths.configFilePath, err)
	}
	return nil
}

func readConfigFile() error {
	file, err := os.ReadFile(Paths.configFilePath)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := toml.Unmarshal(file, &AppConfig); err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	slog.Debug("config loaded", "config", fmt.Sprintf("%#v", AppConfig))

	return nil
}
