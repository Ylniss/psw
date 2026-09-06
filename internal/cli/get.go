package cli

import (
	"fmt"
	"log/slog"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/clipclean"
	"github.com/ylniss/psw/internal/storage"
)

var (
	revealFlag      bool
	getExactFlag    bool
	getStdoutFlag   bool
	getUsernameFlag bool
)

func init() {
	getCmd.Flags().BoolVarP(&revealFlag, "reveal", "r", false, "reveal secret inside terminal")
	getCmd.Flags().BoolVarP(&getExactFlag, "exact", "e", false, "exact name match; skip interactive picker and substring search")
	getCmd.Flags().BoolVar(&getStdoutFlag, "stdout", false, "print secret to stdout instead of clipboard (no labels, no color)")
	getCmd.Flags().BoolVar(&getUsernameFlag, "username", false, "with --stdout, print username instead of password (user/pass records only)")
}

var getCmd = &cobra.Command{
	Use: `get [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to select a record interactively`,
	Short: "Get secrets from record with specified name",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getUsernameFlag && !getStdoutFlag {
			fmt.Println("--username requires --stdout")
			return errSilentExit
		}

		store, err := storage.GetOrCreateForRead()
		if err != nil {
			return handleCmdErr(err)
		}

		recordName, err := resolveRecordName(store, args, getExactFlag, nil)
		if err != nil {
			return handleCmdErr(err)
		}
		if recordName == "" {
			return nil
		}

		record, isFound := store.GetRecord(recordName)

		slog.Debug("cmd/get", "name", record.Name, "kind", recordType(record))

		if !isFound {
			fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
			return nil
		}

		if getStdoutFlag {
			return printRecordToStdout(record, recordName)
		}
		return copyRecordToClipboard(record, recordName)
	},
}

// printRecordToStdout writes raw stdout for piping (e.g. `psw get foo --stdout
// | xclip`); no labels, no color, no menu indent.
func printRecordToStdout(record storage.Record, recordName string) error {
	if getUsernameFlag {
		if len(record.Value) != 0 {
			fmt.Printf("Record %s has no username (it stores a value)\n", color.InGreen(recordName))
			return errSilentExit
		}
		fmt.Println(record.Username)
		return nil
	}
	if len(record.Value) == 0 {
		fmt.Println(string(record.Password))
	} else {
		fmt.Println(string(record.Value))
	}
	return nil
}

func copyRecordToClipboard(record storage.Record, recordName string) error {
	clipboardTimeoutSeconds := storage.AppConfig.ClipboardTimeoutSeconds
	isUserPass := len(record.Value) == 0
	label, secret := "Value", string(record.Value)
	if isUserPass {
		label, secret = "Password", string(record.Password)
	}
	if err := clipclean.CopyAndSchedule(secret, clipboardTimeoutSeconds); err != nil {
		fmt.Println(err)
		return nil
	}
	if isUserPass {
		fmt.Println("Username")
		fmt.Println(color.InYellow(record.Username))
		fmt.Println()
	}
	printSecret(label, recordName, secret, revealFlag, clipboardTimeoutSeconds)
	return nil
}

func printSecret(label, recordName, secret string, reveal bool, clipboardTimeoutSeconds int) {
	if reveal {
		fmt.Println(color.InYellow(secret))
		return
	}
	// Yellow each segment — InCyan's trailing reset would kill an outer yellow wrap.
	fmt.Println(color.InYellow(label+" for ") +
		color.InCyan(recordName) +
		color.InYellow(fmt.Sprintf(" copied — clears in %ds", clipboardTimeoutSeconds)))
}
