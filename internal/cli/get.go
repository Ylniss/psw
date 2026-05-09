package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
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
		store, err := storage.GetOrCreateForRead()
		clipDuration := storage.AppConfig.ClipboardTimeout

		if errors.Is(err, prompt.ErrPromptCancelled) {
			return nil
		}
		if err != nil {
			menuPrintln(err.Error())
			return nil
		}

		recordName, err := resolveRecordName(store, args, getExactFlag)
		if errors.Is(err, errExit) {
			return errExit
		}
		if err != nil {
			menuPrintln(err.Error())
			return nil
		}
		if recordName == "" {
			return nil
		}

		record, isFound := store.GetRecord(recordName)

		slog.Debug("cmd/get", "record", fmt.Sprintf("%#v", record))

		if !isFound {
			menuPrintf("Record %s was not found\n", color.InGreen(recordName))
			return nil
		}

		if getStdoutFlag {
			// Raw stdout for piping (e.g. `psw get foo --stdout | xclip`); no labels, no color, no menu indent.
			if record.Value == "" {
				fmt.Println(record.Password)
			} else {
				fmt.Println(record.Value)
			}
			return nil
		}

		if record.Value == "" {
			if err := clipboard.WriteAll(record.Password); err != nil {
				menuPrintf("Failed to copy value to clipboard: %s\n", err)
				return nil
			}
			menuPrintln("Username")
			menuPrintln(color.InYellow(record.Username))
			fmt.Println()
			printSecret("Password", record.Password, revealFlag, clipDuration)
		} else {
			if err := clipboard.WriteAll(record.Value); err != nil {
				menuPrintf("Failed to copy value to clipboard: %s\n", err)
				return nil
			}
			printSecret("Value", record.Value, revealFlag, clipDuration)
		}

		clipcleanCmd := exec.Command("clipclean", fmt.Sprint(clipDuration))
		err = clipcleanCmd.Start()
		if err != nil {
			menuPrintf("clipclean error: %s\n", err)
			return nil
		}
		return nil
	},
}

func printSecret(label, secret string, reveal bool, clipDuration int) {
	if reveal {
		menuPrintln(color.InYellow(secret))
		return
	}
	msg := fmt.Sprintf("%s copied to the clipboard, it will be cleared in %d seconds", label, clipDuration)
	menuPrintln(color.InYellow(msg))
}
