package cli

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
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
		if len(args) == 1 && args[0] == "main" {
			if cmd.Flags().Changed("rename") || cmd.Flags().Changed("username") ||
				cmd.Flags().Changed("password") || cmd.Flags().Changed("value") {
				fmt.Println("Record-level flags (--rename/--username/--password/--value) are not valid with 'change main'")
				return errExit
			}
			if err := changeMainPassword(); err != nil {
				return err
			}
			if err := storage.GitCommit("main password changed"); err != nil {
				fmt.Println(err.Error())
			}
			return nil
		}
		if err := changeRecord(cmd, args); err != nil {
			return err
		}
		storage.GitCommit("record updated")
		return nil
	},
}

func changeMainPassword() error {
	fmt.Println(color.InCyan("You are changing your main password!"))

	store, err := storage.GetOrCreateForMutate()
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return nil
	}
	if errors.Is(err, storage.ErrForkUndecryptable) {
		printForkUndecryptable()
		return errExit
	}
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	newMainPassword, err := prompt.PromptForMainPasswordChange()
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return nil
	}
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	store.MainPassword = newMainPassword
	if err := store.Save(); err != nil {
		fmt.Println(err.Error())
		return nil
	}

	fmt.Println(color.InGreen("Main password changed"))
	return nil
}

func changeRecord(cmd *cobra.Command, args []string) error {
	renameSet := cmd.Flags().Changed("rename")
	usernameSet := cmd.Flags().Changed("username")
	passwordSet := cmd.Flags().Changed("password")
	valueSet := cmd.Flags().Changed("value")

	// If any field flag was passed, switch to non-interactive mode: only
	// touch the fields whose flag is set; leave the rest unchanged.
	anyFlagSet := renameSet || usernameSet || passwordSet || valueSet

	store, err := storage.GetOrCreateForMutate()
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return nil
	}
	if errors.Is(err, storage.ErrForkUndecryptable) {
		printForkUndecryptable()
		return errExit
	}
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	recordName, err := resolveRecordName(store, args, changeExactFlag)
	if errors.Is(err, errExit) {
		return errExit
	}
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	if recordName == "" {
		return nil
	}
	record, isFound := store.GetRecord(recordName)

	slog.Debug("cmd/change", "record", fmt.Sprintf("%#v", record))

	if !isFound {
		fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
		return nil
	}

	if (usernameSet || passwordSet) && record.Value != "" {
		fmt.Printf("Record %s is value-only; --username/--password not applicable\n", color.InGreen(recordName))
		return errExit
	}
	if valueSet && record.Value == "" {
		fmt.Printf("Record %s is user/pass; --value not applicable\n", color.InGreen(recordName))
		return errExit
	}

	if !applyOrPromptRename(&record, store, renameSet, anyFlagSet) {
		return nil
	}

	if record.Value == "" {
		if !applyOrPromptUsername(&record, usernameSet, anyFlagSet) {
			return nil
		}
		if !applyOrPromptPassword(&record, passwordSet, anyFlagSet) {
			return nil
		}
	} else {
		if !applyOrPromptValue(&record, valueSet, anyFlagSet) {
			return nil
		}
	}

	store.UpdateRecord(recordName, record)

	if err := store.Save(); err != nil {
		fmt.Println(err.Error())
		return nil
	}

	fmt.Println(color.InGreen("Record updated"))
	return nil
}

// applyOrPromptRename returns false when the caller should abort (error or
// duplicate name); true means the field has been processed (either set,
// skipped via flag-mode, or the user declined the prompt).
func applyOrPromptRename(record *storage.Record, store *storage.Storage, flagSet, anyFlagSet bool) bool {
	if flagSet {
		if store.Exists(changeRenameFlag) {
			fmt.Printf("Record with name %s already exists\n", color.InGreen(changeRenameFlag))
			return false
		}
		record.Name = changeRenameFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prompt.YesOrNo("Do you want to change record name?") {
		return true
	}
	fmt.Printf("Current name: %s\n", color.InGreen(record.Name))
	newName, err := prompt.PromptForName("New name")
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	if store.Exists(newName) {
		fmt.Printf("Record with name %s already exists\n", color.InGreen(newName))
		return false
	}
	record.Name = newName
	return true
}

func applyOrPromptUsername(record *storage.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Username = changeUsernameFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prompt.YesOrNo("Do you want to change username?") {
		return true
	}
	newUsername, err := prompt.PromptForName("New username")
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	record.Username = newUsername
	return true
}

func applyOrPromptPassword(record *storage.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Password = changePasswordFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prompt.YesOrNo("Do you want to change password?") {
		return true
	}
	newPassword, err := prompt.PromptForRecordPassword()
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	record.Password = newPassword
	return true
}

func applyOrPromptValue(record *storage.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Value = changeValueFlag
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prompt.YesOrNo("Do you want to change value?") {
		return true
	}
	newValue, err := prompt.PromptForName("New value")
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return false
	}
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	record.Value = newValue
	return true
}
