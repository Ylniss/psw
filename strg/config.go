package strg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/pelletier/go-toml/v2"
)

type StorageCfg struct {
	storagePath     string
	storageFilePath string
	storageFileName string
	configFilePath  string
	configFileName  string
	gitRepoExists   bool
}

var Cfg = StorageCfg{
	storageFileName: "storage.psw",
	configFileName:  "pswcfg.toml",
	gitRepoExists:   false,
}

type Config struct {
	ClipboardTimeout int `toml:"clipboard_timeout"`
}

var AppConfig Config

func InitConfig() {
	err := setStoragePath()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = loadConfig()
	if err != nil {
		fmt.Println("Failed to load configuration:", err.Error())
		os.Exit(1)
	}
}

func setStoragePath() error {
	// PSW_HOME overrides default storage dir (intended for tests/scripting).
	path := os.Getenv("PSW_HOME")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Error while retrieving home directory:\n%w", err)
		}
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
		return errors.New("error when setting storage path, storage file name is not set")
	}

	Cfg.storageFilePath = filepath.Join(Cfg.storagePath, Cfg.storageFileName)

	return nil
}

func loadConfig() error {
	Cfg.configFilePath = filepath.Join(Cfg.storagePath, Cfg.configFileName)

	if err := ensureUserConfig(); err != nil {
		return err
	}

	if err := readConfigFile(); err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}
	return nil
}

func ensureUserConfig() error {
	exists, err := pathExists(Cfg.configFilePath)
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
	binConfigPath := filepath.Join(filepath.Dir(binPath), Cfg.configFileName)

	binExists, err := pathExists(binConfigPath)
	if err != nil {
		return fmt.Errorf("error checking binary config file existence: %w", err)
	}
	if !binExists {
		return errors.New("config file does not exist in the binary location")
	}

	if err := copyFile(binConfigPath, Cfg.configFilePath); err != nil {
		return fmt.Errorf("failed to copy config file from %s to %s: %w", binConfigPath, Cfg.configFilePath, err)
	}
	return nil
}

func readConfigFile() error {
	file, err := os.ReadFile(Cfg.configFilePath)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := toml.Unmarshal(file, &AppConfig); err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	slog.Debug(fmt.Sprintf("Config loaded: %#v", AppConfig))

	return nil
}
