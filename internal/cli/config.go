package cli

import (
	"fmt"
	"strings"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"

	"github.com/ylniss/psw/internal/storage"
)

func init() {
	configCmd.AddCommand(configSetCmd, configResetCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or modify the user config",
	Long: `Show or modify the user config (pswcfg.toml).

Bare command prints the config file path. Use "set" to update a single key
or "reset" to restore the template defaults.

Configurable keys:
` + configKeysHelp(),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(storage.Paths.ConfigFilePath())
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config key",
	Long: `Set a config key.

Configurable keys:
` + configKeysHelp() + `

TOML comments are not preserved across set; use reset to restore the template.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, ok := storage.LookupConfigKey(args[0])
		if !ok {
			fmt.Println(color.InRed(fmt.Sprintf("unknown config key: %s", args[0])))
			fmt.Println("valid keys: " + configKeyNames())
			return errSilentExit
		}
		if err := key.Apply(&storage.AppConfig, args[1]); err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		if err := storage.WriteAndCommitConfig("config updated"); err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the user config to template defaults",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := storage.ResetConfigToTemplate(); err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		_ = storage.GitCommit("config reset")
		return nil
	},
}

// configKeyNames returns a comma-separated list for error messages.
func configKeyNames() string {
	names := make([]string, len(storage.ConfigKeys))
	for i, k := range storage.ConfigKeys {
		names[i] = k.Name
	}
	return strings.Join(names, ", ")
}

// configKeysHelp formats keys as aligned "  name (kind)  description" lines
// for cobra Long.
func configKeysHelp() string {
	headers := make([]string, len(storage.ConfigKeys))
	maxHeaderWidth := 0
	for i, k := range storage.ConfigKeys {
		headers[i] = fmt.Sprintf("%s (%s)", k.Name, k.Kind)
		if n := len(headers[i]); n > maxHeaderWidth {
			maxHeaderWidth = n
		}
	}
	var b strings.Builder
	for i, k := range storage.ConfigKeys {
		fmt.Fprintf(&b, "  %-*s  %s\n", maxHeaderWidth, headers[i], k.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}
