package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/pelletier/go-toml/v2"

	"github.com/ylniss/psw/internal/passgen"
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
	ClipboardTimeout int         `toml:"clipboard_timeout"`
	Remote           string      `toml:"remote"`
	PasswordGen      PasswordGen `toml:"password_gen"`
}

// PasswordGen mirrors the [password_gen] section. Pointer fields distinguish
// "key absent" (nil → use default) from "key explicitly set to 0/false".
type PasswordGen struct {
	Length       *int  `toml:"length"`
	MinDigits    *int  `toml:"min_digits"`
	MinSymbols   *int  `toml:"min_symbols"`
	MinUppercase *int  `toml:"min_uppercase"`
	MinLowercase *int  `toml:"min_lowercase"`
	AllowRepeat  *bool `toml:"allow_repeat"`
}

// Resolve fills in passgen defaults for any nil pointer fields.
func (p PasswordGen) Resolve() passgen.Options {
	o := passgen.DefaultOptions()
	if p.Length != nil {
		o.Length = *p.Length
	}
	if p.MinDigits != nil {
		o.MinDigits = *p.MinDigits
	}
	if p.MinSymbols != nil {
		o.MinSymbols = *p.MinSymbols
	}
	if p.MinUppercase != nil {
		o.MinUppercase = *p.MinUppercase
	}
	if p.MinLowercase != nil {
		o.MinLowercase = *p.MinLowercase
	}
	if p.AllowRepeat != nil {
		o.AllowRepeat = *p.AllowRepeat
	}
	return o
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
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("retrieve user config directory: %w", err)
		}
		path = filepath.Join(configDir, "psw")
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
