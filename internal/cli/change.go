package cli

import (
	"errors"
	"fmt"

	"log/slog"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prmpt"
	"github.com/ylniss/psw/internal/strg"
)

var (
	changeExactFlag  bool
	changeRenameFlag string
	changeUserFlag   string
	changePassFlag   string
	changeValueFlag  string
)

func init() {
	changeCmd.Flags().BoolVarP(&changeExactFlag, "exact", "e", false, "exact name match; skip interactive picker and substring search")
	changeCmd.Flags().StringVar(&changeRenameFlag, "rename", "", "new record name (skips rename y/n + prompt)")
	changeCmd.Flags().StringVarP(&changeUserFlag, "username", "u", "", "new username (skips username y/n + prompt)")
	changeCmd.Flags().StringVar(&changePassFlag, "password", "", "new password (skips password y/n + prompt)")
	changeCmd.Flags().StringVar(&changeValueFlag, "value", "", "new value for value-only records (skips value y/n + prompt)")
	rootCmd.AddCommand(changeCmd)
}

var changeCmd = &cobra.Command{
	Use: `change [name] [flags]

Arguments:
  name    Optional name of the record to change. If omitted, you'll be prompted to provide it.
          You can also change your main password with this command, just pass 'main' as name`,
	Short: "Change chosen record data",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && args[0] == "main" {
			if cmd.Flags().Changed("rename") || cmd.Flags().Changed("username") ||
				cmd.Flags().Changed("password") || cmd.Flags().Changed("value") {
				fmt.Println("Record-level flags (--rename/--username/--password/--value) are not valid with 'change main'")
				return errExit
			}
			changeMainPass()
			strg.GitCommit("main password changed")
			return nil
		}
		if err := changeRecord(cmd, args); err != nil {
			return err
		}
		strg.GitCommit("record updated")
		return nil
	},
}

func changeMainPass() {
	fmt.Println(color.InCyan("You are changing your main password!\nFirst enter your current password"))

	mainPass, err := prmpt.PromptForMainPass(true) // prompt with ensure
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return
	}
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	storage, err := strg.Get(mainPass)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	newMainPass, err := prmpt.PromptForMainPassChange()
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return
	}
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	storage.MainPass = newMainPass
	if err := storage.Save(); err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(color.InGreen("Main password changed"))
}

func changeRecord(cmd *cobra.Command, args []string) error {
	renameSet := cmd.Flags().Changed("rename")
	userSet := cmd.Flags().Changed("username")
	passSet := cmd.Flags().Changed("password")
	valSet := cmd.Flags().Changed("value")

	// If any field flag was passed, switch to non-interactive mode: only
	// touch the fields whose flag is set; leave the rest unchanged.
	anyFlagSet := renameSet || userSet || passSet || valSet

	storage, err := strg.GetOrCreateIfNotExists()
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return nil
	}
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	recordName, err := resolveRecordName(storage, args, changeExactFlag)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	if recordName == "" {
		return nil
	}
	record, isFound := storage.GetRecord(recordName)

	slog.Debug(fmt.Sprintf("cmd/change - record: %#v", record))

	if !isFound {
		fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
		return nil
	}

	if (userSet || passSet) && record.Value != "" {
		fmt.Printf("Record %s is value-only; --username/--password not applicable\n", color.InGreen(recordName))
		return errExit
	}
	if valSet && record.Value == "" {
		fmt.Printf("Record %s is user/pass; --value not applicable\n", color.InGreen(recordName))
		return errExit
	}

	if !applyOrPromptRename(&record, storage, renameSet, anyFlagSet) {
		return nil
	}

	if record.Value == "" {
		if !applyOrPromptUsername(&record, userSet, anyFlagSet) {
			return nil
		}
		if !applyOrPromptPassword(&record, passSet, anyFlagSet) {
			return nil
		}
	} else {
		if !applyOrPromptValue(&record, valSet, anyFlagSet) {
			return nil
		}
	}

	storage.UpdateRecord(recordName, record)

	if err := storage.Save(); err != nil {
		fmt.Println(err.Error())
		return nil
	}

	fmt.Printf(color.InGreen("Record updated\n"))
	return nil
}

// applyOrPromptRename returns false when the caller should abort (error or
// duplicate name); true means the field has been processed (either set,
// skipped via flag-mode, or the user declined the prompt).
func applyOrPromptRename(record *strg.Record, storage *strg.Storage, flagSet, anyFlagSet bool) bool {
	if flagSet {
		if storage.Exists(changeRenameFlag) {
			fmt.Printf("Record with name %s already exists\n", color.InGreen(changeRenameFlag))
			return false
		}
		record.Name = changeRenameFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prmpt.YesOrNo("Do you want to change record name?") {
		return true
	}
	fmt.Printf("Current name: %s\n", color.InGreen(record.Name))
	newName, err := prmpt.PromptForName("New name")
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	if storage.Exists(newName) {
		fmt.Printf("Record with name %s already exists\n", color.InGreen(newName))
		return false
	}
	record.Name = newName
	return true
}

func applyOrPromptUsername(record *strg.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.User = changeUserFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prmpt.YesOrNo("Do you want to change username?") {
		return true
	}
	newUser, err := prmpt.PromptForName("New username")
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	record.User = newUser
	return true
}

func applyOrPromptPassword(record *strg.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Pass = changePassFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prmpt.YesOrNo("Do you want to change password?") {
		return true
	}
	newPass, err := prmpt.PromptForRecordPass()
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	record.Pass = newPass
	return true
}

func applyOrPromptValue(record *strg.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Value = changeValueFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prmpt.YesOrNo("Do you want to change value?") {
		return true
	}
	newValue, err := prmpt.PromptForName("New value")
	if errors.Is(err, prmpt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	record.Value = newValue
	return true
}
