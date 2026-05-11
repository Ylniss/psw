package cli

import (
	"fmt"
	"log/slog"

	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/clipclean"
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
}

var getCmd = &cobra.Command{
	Use: `get [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to select a record interactively`,
	Short: "Get secrets from record with specified name",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.GetOrCreateForRead()
		if done, ret := handleCmdErr(err); done {
			return ret
		}
		clipboardTimeoutSeconds := storage.AppConfig.ClipboardTimeoutSeconds

		recordName, err := resolveRecordName(store, args, getExactFlag, nil)
		if done, ret := handleCmdErr(err); done {
			return ret
		}
		if recordName == "" {
			return nil
		}

		record, isFound := store.GetRecord(recordName)

		slog.Debug("cmd/get", "record", fmt.Sprintf("%#v", record))

		if !isFound {
			fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
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
				fmt.Printf("Failed to copy value to clipboard: %s\n", err)
				return nil
			}
			fmt.Println("Username")
			fmt.Println(color.InYellow(record.Username))
			fmt.Println()
			printSecret("Password", recordName, record.Password, revealFlag, clipboardTimeoutSeconds)
		} else {
			if err := clipboard.WriteAll(record.Value); err != nil {
				fmt.Printf("Failed to copy value to clipboard: %s\n", err)
				return nil
			}
			printSecret("Value", recordName, record.Value, revealFlag, clipboardTimeoutSeconds)
		}

		if err := clipclean.Spawn(clipboardTimeoutSeconds); err != nil {
			fmt.Printf("clipclean error: %s\n", err)
			return nil
		}
		return nil
	},
}

func printSecret(label, recordName, secret string, reveal bool, clipboardTimeoutSeconds int) {
	if reveal {
		fmt.Println(color.InYellow(secret))
		return
	}
	// Yellow each segment — InCyan's trailing reset would kill an outer yellow wrap.
	fmt.Println(color.InYellow(label+" for ") +
		color.InCyan(recordName) +
		color.InYellow(fmt.Sprintf(" copied to the clipboard, it will be cleared in %d seconds", clipboardTimeoutSeconds)))
}
