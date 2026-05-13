package cli

import (
	"fmt"
	"log/slog"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

func changeRecord(cmd *cobra.Command, args []string) error {
	renameSet := cmd.Flags().Changed("rename")
	usernameSet := cmd.Flags().Changed("username")
	passwordSet := cmd.Flags().Changed("password")
	valueSet := cmd.Flags().Changed("value")

	// If any field flag was passed, switch to non-interactive mode: only
	// touch the fields whose flag is set; leave the rest unchanged.
	anyFlagSet := renameSet || usernameSet || passwordSet || valueSet

	store, err := storage.GetOrCreateForMutate()
	if done, ret := handleCmdErr(err); done {
		return ret
	}

	recordName, err := resolveRecordName(store, args, changeExactFlag, []string{storage.MainPasswordKeywordLong})
	if done, ret := handleCmdErr(err); done {
		return ret
	}
	if recordName == "" {
		return nil
	}
	if recordName == storage.MainPasswordKeywordLong {
		if err := rejectFieldFlagsForMain(cmd); err != nil {
			return err
		}
		if err := changeMainPasswordOnStore(store); err != nil {
			return err
		}
		storage.GitCommit("main password changed")
		return nil
	}
	record, isFound := store.GetRecord(recordName)

	slog.Debug("cmd/change", "name", record.Name, "kind", recordKindLabel(record))

	if !isFound {
		fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
		return nil
	}

	if (usernameSet || passwordSet) && len(record.Value) != 0 {
		fmt.Printf("Record %s is value-only; --username/--password not applicable\n", color.InGreen(recordName))
		return errSilentExit
	}
	if valueSet && len(record.Value) == 0 {
		fmt.Printf("Record %s is user/pass; --value not applicable\n", color.InGreen(recordName))
		return errSilentExit
	}

	if !applyOrPromptRename(&record, store, renameSet, anyFlagSet) {
		return nil
	}

	if len(record.Value) == 0 {
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

	if done, ret := handleCmdErr(store.Save()); done {
		return ret
	}

	fmt.Printf("Record %s was updated successfully\n", color.InGreen(record.Name))
	storage.GitCommit("record updated")
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
	if handlePromptErr(err) {
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
	if handlePromptErr(err) {
		return false
	}
	record.Username = newUsername
	return true
}

func applyOrPromptPassword(record *storage.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Password = []byte(changePasswordFlag)
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prompt.YesOrNo("Do you want to change password?") {
		return true
	}
	newPassword, err := prompt.PromptForRecordPassword()
	if handlePromptErr(err) {
		return false
	}
	record.Password = []byte(newPassword)
	return true
}

func applyOrPromptValue(record *storage.Record, flagSet, anyFlagSet bool) bool {
	if flagSet {
		record.Value = []byte(changeValueFlag)
		return true
	}
	if anyFlagSet {
		return true
	}
	if !prompt.YesOrNo("Do you want to change value?") {
		return true
	}
	newValue, err := prompt.PromptForSecretValue("New value")
	if handlePromptErr(err) {
		return false
	}
	record.Value = []byte(newValue)
	return true
}
