package storage

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"log/slog"

	"github.com/google/renameio/v2"
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

// ConfigFilePath returns the user pswcfg.toml path.
func (StorageConfig) ConfigFilePath() string { return Paths.configFilePath }

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
	if err := renameio.WriteFile(Paths.configFilePath, data, 0600); err != nil {
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

// ConfigKey describes a single configurable key.
//
//	Apply       parses+assigns the raw value to AppConfig; user-facing error on failure.
//	Current     raw editable value — safe to feed back into Apply (no decoration).
//	Display     grid-view rendering; may add "(default)" decoration for absent keys.
//	Adjust      nil for non-numeric; ±delta inline (h/l in the settings grid).
//	Description one-line help shown under the settings grid for the focused key.
type ConfigKey struct {
	Name        string
	Kind        string // "string" | "int" | "bool"
	Description string
	Apply       func(*Config, string) error
	Current     func(*Config) string
	Display     func(*Config) string
	Adjust      func(*Config, int)
}

// ConfigKeys drives both `psw config set` and the menu settings grid.
var ConfigKeys = buildConfigKeys()

func buildConfigKeys() []ConfigKey {
	defaults := passgen.DefaultOptions()
	return []ConfigKey{
		{
			Name: "clipboard_timeout", Kind: "int",
			Description: "Seconds before clipclean wipes a copied secret from the clipboard.",
			Apply: func(c *Config, s string) error {
				n, err := parseInt(s, "clipboard_timeout")
				if err != nil {
					return err
				}
				c.ClipboardTimeoutSeconds = n
				return nil
			},
			Current: func(c *Config) string { return strconv.Itoa(c.ClipboardTimeoutSeconds) },
			Adjust:  func(c *Config, d int) { c.ClipboardTimeoutSeconds = clamp0(c.ClipboardTimeoutSeconds + d) },
		},
		{
			Name: "remote", Kind: "string",
			Description: "Git remote URL for cross-device sync. Empty disables sync.",
			Apply: func(c *Config, s string) error {
				if err := validateRemoteURL(s); err != nil {
					return err
				}
				c.Remote = s
				return nil
			},
			Current: func(c *Config) string { return c.Remote },
		},
		passgenIntKey("length", "Total length of auto-generated passwords.",
			func(p *PasswordGen) **int { return &p.Length }, defaults.Length),
		passgenIntKey("min_digits", "Minimum digits (0-9) in generated passwords.",
			func(p *PasswordGen) **int { return &p.MinDigits }, defaults.MinDigits),
		passgenIntKey("min_symbols", "Minimum symbols (!@#$%^&*()-_=+[]{}<>?,./) in generated passwords.",
			func(p *PasswordGen) **int { return &p.MinSymbols }, defaults.MinSymbols),
		passgenIntKey("min_uppercase", "Minimum uppercase letters (A-Z) in generated passwords.",
			func(p *PasswordGen) **int { return &p.MinUppercase }, defaults.MinUppercase),
		passgenIntKey("min_lowercase", "Minimum lowercase letters (a-z) in generated passwords.",
			func(p *PasswordGen) **int { return &p.MinLowercase }, defaults.MinLowercase),
		passgenBoolKey("allow_repeat", "Whether a character may appear more than once in a generated password.",
			func(p *PasswordGen) **bool { return &p.AllowRepeat }, defaults.AllowRepeat),
	}
}

// passgenIntKey builds a ConfigKey for one [password_gen] int-pointer field.
// fieldSlot returns the storage slot; defaultValue is the package default,
// captured once at init.
func passgenIntKey(name, desc string, fieldSlot func(*PasswordGen) **int, defaultValue int) ConfigKey {
	return ConfigKey{
		Name: name, Kind: "int", Description: desc,
		Apply: func(c *Config, s string) error {
			return applyIntPtr(s, name, fieldSlot(&c.PasswordGen))
		},
		Current: func(c *Config) string {
			return intAsString(*fieldSlot(&c.PasswordGen), defaultValue)
		},
		Display: func(c *Config) string {
			return intAsStringWithDefaultTag(*fieldSlot(&c.PasswordGen), defaultValue)
		},
		Adjust: func(c *Config, delta int) {
			current := defaultValue
			if existing := *fieldSlot(&c.PasswordGen); existing != nil {
				current = *existing
			}
			next := clamp0(current + delta)
			*fieldSlot(&c.PasswordGen) = &next
		},
	}
}

// passgenBoolKey is passgenIntKey for bool fields. h/l both flip the value.
func passgenBoolKey(name, desc string, fieldSlot func(*PasswordGen) **bool, defaultValue bool) ConfigKey {
	return ConfigKey{
		Name: name, Kind: "bool", Description: desc,
		Apply: func(c *Config, s string) error {
			return applyBoolPtr(s, name, fieldSlot(&c.PasswordGen))
		},
		Current: func(c *Config) string {
			return boolAsString(*fieldSlot(&c.PasswordGen), defaultValue)
		},
		Display: func(c *Config) string {
			return boolAsStringWithDefaultTag(*fieldSlot(&c.PasswordGen), defaultValue)
		},
		Adjust: func(c *Config, _ int) {
			current := defaultValue
			if existing := *fieldSlot(&c.PasswordGen); existing != nil {
				current = *existing
			}
			flipped := !current
			*fieldSlot(&c.PasswordGen) = &flipped
		},
	}
}

// DisplayValue returns Display if set, else Current.
func (k ConfigKey) DisplayValue(c *Config) string {
	if k.Display != nil {
		return k.Display(c)
	}
	return k.Current(c)
}

// LookupConfigKey returns the registry entry for name, or false.
func LookupConfigKey(name string) (ConfigKey, bool) {
	for _, k := range ConfigKeys {
		if k.Name == name {
			return k, true
		}
	}
	return ConfigKey{}, false
}

// ConfigKeyNames returns a comma-separated list for error messages.
func ConfigKeyNames() string {
	names := make([]string, len(ConfigKeys))
	for i, k := range ConfigKeys {
		names[i] = k.Name
	}
	return strings.Join(names, ", ")
}

// ConfigKeysHelp formats keys as aligned "  name (kind)  description" lines
// for cobra Long.
func ConfigKeysHelp() string {
	headers := make([]string, len(ConfigKeys))
	maxHeader := 0
	for i, k := range ConfigKeys {
		headers[i] = fmt.Sprintf("%s (%s)", k.Name, k.Kind)
		if n := len(headers[i]); n > maxHeader {
			maxHeader = n
		}
	}
	var b strings.Builder
	for i, k := range ConfigKeys {
		fmt.Fprintf(&b, "  %-*s  %s\n", maxHeader, headers[i], k.Description)
	}
	return strings.TrimRight(b.String(), "\n")
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

func parseInt(s, name string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid int for %q: %v", name, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("value for %q must be >= 0", name)
	}
	return n, nil
}

// clamp0 floors v at 0 (matches parseInt's min-0 rule on typed values).
func clamp0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func applyIntPtr(s, name string, dst **int) error {
	n, err := parseInt(s, name)
	if err != nil {
		return err
	}
	*dst = &n
	return nil
}

func applyBoolPtr(s, name string, dst **bool) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("invalid bool for %q: %v", name, err)
	}
	*dst = &b
	return nil
}

// intAsString returns the value as a string. configured is the toml-loaded
// pointer (nil = key absent); falls back to defaultValue.
func intAsString(configured *int, defaultValue int) string {
	if configured == nil {
		return strconv.Itoa(defaultValue)
	}
	return strconv.Itoa(*configured)
}

// intAsStringWithDefaultTag is intAsString plus a "(default)" suffix when
// the resolved value equals defaultValue.
func intAsStringWithDefaultTag(configured *int, defaultValue int) string {
	value := defaultValue
	if configured != nil {
		value = *configured
	}
	if value == defaultValue {
		return fmt.Sprintf("%d (default)", defaultValue)
	}
	return strconv.Itoa(value)
}

func boolAsString(configured *bool, defaultValue bool) string {
	if configured == nil {
		return strconv.FormatBool(defaultValue)
	}
	return strconv.FormatBool(*configured)
}

func boolAsStringWithDefaultTag(configured *bool, defaultValue bool) string {
	value := defaultValue
	if configured != nil {
		value = *configured
	}
	if value == defaultValue {
		return fmt.Sprintf("%v (default)", defaultValue)
	}
	return strconv.FormatBool(value)
}
