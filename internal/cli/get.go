package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prmpt"
	"github.com/ylniss/psw/internal/strg"
)

var (
	revealFlag    bool
	getExactFlag  bool
	getStdoutFlag bool
)

func init() {
	getCmd.Flags().BoolVarP(&revealFlag, "reveal", "r", false, "reveal secret inside terminal")
	getCmd.Flags().BoolVarP(&getExactFlag, "exact", "e", false, "exact name match; skip interactive picker and substring search")
	getCmd.Flags().BoolVar(&getStdoutFlag, "stdout", false, "print secret to stdout instead of clipboard (no labels, no color)")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use: `get [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to select a record interactively`,
	Short: "Get secrets from record with specified name",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storage, err := strg.GetOrCreateIfNotExists()
		clipDuration := strg.AppConfig.ClipboardTimeout

		if errors.Is(err, prmpt.ErrPromptCancelled) {
			return nil
		}
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		recordName, err := resolveRecordName(storage, args, getExactFlag)
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

		record, isFound := storage.GetRecord(recordName)

		slog.Debug("cmd/get", "record", fmt.Sprintf("%#v", record))

		if !isFound {
			fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
			return nil
		}

		if getStdoutFlag {
			if record.Value == "" {
				fmt.Println(record.Pass)
			} else {
				fmt.Println(record.Value)
			}
			return nil
		}

		if record.Value == "" {
			if err := clipboard.WriteAll(record.Pass); err != nil {
				fmt.Printf("Failed to copy value to clipboard: %s\n", err)
				return nil
			}
			fmt.Println("Username")
			fmt.Println(color.InYellow(record.User))
			fmt.Println()
			printSecret("Password", record.Pass, revealFlag, clipDuration)
		} else {
			if err := clipboard.WriteAll(record.Value); err != nil {
				fmt.Printf("Failed to copy value to clipboard: %s\n", err)
				return nil
			}
			printSecret("Value", record.Value, revealFlag, clipDuration)
		}

		syscmd := exec.Command("clipclean", fmt.Sprint(clipDuration))
		err = syscmd.Start()
		if err != nil {
			fmt.Printf("clipclean error: %s\n", err)
			return nil
		}
		return nil
	},
}

func printSecret(label, secret string, reveal bool, clipDur int) {
	fmt.Println(label)
	if reveal {
		fmt.Println(color.InYellow(secret))
		return
	}
	msg := fmt.Sprintf("*********** - copied to the clipboard, it will be cleared in %d seconds", clipDur)
	fmt.Println(color.InYellow(msg))
}
