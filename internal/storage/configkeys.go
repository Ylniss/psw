package storage

import (
	"fmt"
	"strconv"

	"github.com/ylniss/psw/internal/passgen"
)

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
				n, err := parseNonNegativeInt(s, "clipboard_timeout")
				if err != nil {
					return err
				}
				c.ClipboardTimeoutSeconds = n
				return nil
			},
			Current: func(c *Config) string { return strconv.Itoa(c.ClipboardTimeoutSeconds) },
			Adjust:  func(c *Config, d int) { c.ClipboardTimeoutSeconds = max(c.ClipboardTimeoutSeconds+d, 0) },
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
			next := max(current+delta, 0)
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

func parseNonNegativeInt(s, name string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid int for %q: %v", name, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("value for %q must be >= 0", name)
	}
	return n, nil
}

func applyIntPtr(s, name string, dst **int) error {
	n, err := parseNonNegativeInt(s, name)
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
