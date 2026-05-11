package cli

import (
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"

	"github.com/ylniss/psw/internal/storage"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or modify the user config",
	Long: `Show or modify the user config (pswcfg.toml).

Bare command prints the config file path. Use "set" to update a single key
or "reset" to restore the template defaults.

Configurable keys:
` + storage.ConfigKeysHelp(),
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
` + storage.ConfigKeysHelp() + `

TOML comments are not preserved across set; use reset to restore the template.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, ok := storage.LookupConfigKey(args[0])
		if !ok {
			fmt.Println(color.InRed(fmt.Sprintf("unknown config key: %s", args[0])))
			fmt.Println("valid keys: " + storage.ConfigKeyNames())
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

func init() {
	configCmd.AddCommand(configSetCmd, configResetCmd)
}
