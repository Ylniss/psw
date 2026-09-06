package storage

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/pelletier/go-toml/v2"

	"github.com/ylniss/psw/internal/passgen"
)

type StoragePaths struct {
	storagePath     string
	storageFilePath string
	storageFileName string
	configFilePath  string
	configFileName  string
	gitRepoExists   bool
}

var Paths = StoragePaths{
	storageFileName: "storage.psw",
	configFileName:  "pswcfg.toml",
	gitRepoExists:   false,
}

// ConfigFilePath returns the user pswcfg.toml path.
func (StoragePaths) ConfigFilePath() string { return Paths.configFilePath }

type Config struct {
	ClipboardTimeoutSeconds int         `toml:"clipboard_timeout"`
	Remote                  string      `toml:"remote"`
	PasswordGen             PasswordGen `toml:"password_gen"`
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

	Paths.storageFilePath = filepath.Join(Paths.storagePath, Paths.storageFileName)

	return nil
}

func loadConfig() error {
	Paths.configFilePath = filepath.Join(Paths.storagePath, Paths.configFileName)

	if err := ensureUserConfig(); err != nil {
		return err
	}

	return readConfigFile()
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

	AppConfig = Config{}
	if err := toml.Unmarshal(file, &AppConfig); err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	if err := validateRemoteURL(AppConfig.Remote); err != nil {
		Warn("ignoring remote in %s: %v", Paths.configFilePath, err)
		AppConfig.Remote = ""
	}

	slog.Debug("config loaded",
		"remote", redactURL(AppConfig.Remote),
		"clipboard_timeout", AppConfig.ClipboardTimeoutSeconds,
	)

	return nil
}

// SaveConfig writes AppConfig to pswcfg.toml atomically at mode 0600.
// Comments and field order not preserved (go-toml/v2 limitation).
func SaveConfig() error {
	if Paths.configFilePath == "" {
		return errors.New("config file path not initialized")
	}
	data, err := toml.Marshal(AppConfig)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := writeFileAtomic(Paths.configFilePath, data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// WriteAndCommitConfig writes pswcfg.toml then commits with subject.
// Commit/push errors are best-effort (warn via GitCommit).
func WriteAndCommitConfig(commitSubject string) error {
	if err := SaveConfig(); err != nil {
		return err
	}
	_ = GitCommit(commitSubject)
	return nil
}

// ResetConfigToTemplate overwrites pswcfg.toml with the binary-adjacent
// template and reloads AppConfig.
func ResetConfigToTemplate() error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine executable path: %w", err)
	}
	template := filepath.Join(filepath.Dir(binPath), Paths.configFileName)
	exists, err := pathExists(template)
	if err != nil {
		return fmt.Errorf("check template: %w", err)
	}
	if !exists {
		return fmt.Errorf("template config not found next to binary at %s", template)
	}
	if err := copyFile(template, Paths.configFilePath); err != nil {
		return fmt.Errorf("copy template: %w", err)
	}
	return readConfigFile()
}

// validateRemoteURL rejects http(s) URLs that embed userinfo. Empty strings
// are allowed (disables sync). Non-http schemes (ssh://, git@, file://) carry
// no credentials and aren't inspected.
func validateRemoteURL(s string) error {
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "http://") {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}
	if u.User != nil {
		return fmt.Errorf("credentials in URL are not allowed; remove the user:password@ prefix and use ssh, ssh-agent, or git's credential.helper")
	}
	return nil
}
