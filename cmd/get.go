package cmd

import (
	"fmt"
	"os/exec"

	"github.com/TwiN/go-color"
	"github.com/atotto/clipboard"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

var revealFlag bool

func init() {
	getCmd.Flags().BoolVarP(&revealFlag, "reveal", "r", false, "reveal secret inside terminal")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use: `get [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to select a record with fzf`,
	Short: "Get secrets from record with specified name",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists()
		clipDuration := strg.AppConfig.ClipboardTimeout

		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var recordName string
		if len(args) == 0 {
			recordName, err = strg.GetRecordNameWithFzf(storage.GetNames())
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			recordName, err = strg.GetRecordNameWithFzf(storage.GetNamesWithPart(args[0]))
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		record, isFound := storage.GetRecord(recordName)

		log.Debugf("cmd/get - record: %#v\n", record)

		if !isFound {
			fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
			return
		}

		// Print and copy to clipboard user & pass or value,
		// depending on what is stored in the record
		if record.Value == "" {
			err = clipboard.WriteAll(record.Pass)
			if err != nil {
				fmt.Println(fmt.Sprintf("Failed to copy value to clipboard: %s", err.Error()))
				return
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
				return
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
			return
		}
	},
}
