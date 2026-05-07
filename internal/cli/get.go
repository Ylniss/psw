package cli

import (
	"fmt"
	"os/exec"

	"log/slog"

	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
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

		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		recordName, err := resolveRecordName(storage, args, getExactFlag)
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
		if recordName == "" {
			return nil
		}

		record, isFound := storage.GetRecord(recordName)

		slog.Debug(fmt.Sprintf("cmd/get - record: %#v", record))

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

		// Print and copy to clipboard user & pass or value,
		// depending on what is stored in the record
		if record.Value == "" {
			err = clipboard.WriteAll(record.Pass)
			if err != nil {
				fmt.Println(fmt.Sprintf("Failed to copy value to clipboard: %s", err.Error()))
				return nil
			}

			fmt.Println("Username")
			fmt.Println(color.InYellow(record.User))
			fmt.Println()
			fmt.Println("Password")
			if revealFlag {
				fmt.Println(color.InYellow(record.Pass))
			} else {
				fmt.Println(color.InYellow(fmt.Sprintf("*********** - copied to the clipboard, it will be cleared in %d seconds", clipDuration)))
			}
		} else {
			err = clipboard.WriteAll(record.Value)
			if err != nil {
				fmt.Println(fmt.Sprintf("Failed to copy value to clipboard: %s", err.Error()))
				return nil
			}

			fmt.Println("Value")
			if revealFlag {
				fmt.Println(color.InYellow(record.Value))
			} else {
				fmt.Println(color.InYellow(fmt.Sprintf("*********** - copied to the clipboard, it will be cleared in %d seconds", clipDuration)))
			}
		}

		syscmd := exec.Command("clipclean", fmt.Sprint(clipDuration))
		err = syscmd.Start()
		if err != nil {
			fmt.Println(fmt.Sprintf("clipclean error: %s", err.Error()))
			return nil
		}
		return nil
	},
}
