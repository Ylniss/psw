package cli

import (
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/storage"
)

var (
	changeExactFlag    bool
	changeRenameFlag   string
	changeUsernameFlag string
	changePasswordFlag string
	changeValueFlag    string
)

func init() {
	changeCmd.Flags().BoolVarP(&changeExactFlag, "exact", "e", false, "exact name match; skip interactive picker and substring search")
	changeCmd.Flags().StringVar(&changeRenameFlag, "rename", "", "new record name (skips rename y/n + prompt)")
	changeCmd.Flags().StringVarP(&changeUsernameFlag, "username", "u", "", "new username (skips username y/n + prompt)")
	changeCmd.Flags().StringVar(&changePasswordFlag, "password", "", "new password (skips password y/n + prompt)")
	changeCmd.Flags().StringVar(&changeValueFlag, "value", "", "new value for value-only records (skips value y/n + prompt)")
}

var changeCmd = &cobra.Command{
	Use: `change [name] [flags]

Arguments:
  name    Optional name of the record to change. If omitted, you'll be prompted to provide it.
          You can also change your main password with this command, just pass 'main' as name`,
	Short: "Change chosen record data",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && (args[0] == storage.MainPasswordAlias || args[0] == storage.MainPasswordName) {
			if err := rejectFieldFlagsForMain(cmd); err != nil {
				return err
			}
			store, err := storage.GetOrCreateForMutate()
			if err != nil {
				return handleCmdErr(err)
			}
			return changeMainPassword(store)
		}
		return changeRecord(cmd, args)
	},
}
