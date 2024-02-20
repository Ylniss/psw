package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/TwiN/go-color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
	"golang.design/x/clipboard"
)

var (
	revealFlag   bool
	clipDuration int
)

func init() {
	clipDuration = 30
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
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var recordName string
		if len(args) == 0 {
			recordName, err = getRecordNameWithFzf(storage)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			recordName = args[0]
		}

		record, isFound := storage.GetRecord(recordName)

		log.Debugf("cmd/get - record: %#v\n", record)

		if !isFound {
			fmt.Printf("Record '%s' was not found\n", recordName)
			return
		}

		// Print and copy to clipboard user & pass or value,
		// depending on what is stored in the record
		if record.Value == "" {
			clipboard.Write(clipboard.FmtText, []byte(record.Pass))

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
			clipboard.Write(clipboard.FmtText, []byte(record.Value))

			fmt.Println("Value")
			if revealFlag {
				fmt.Println(color.InYellow(record.Value))
			} else {
				fmt.Println(color.InYellow(fmt.Sprintf("*********** - copied to the clipboard, it will be cleared in %d seconds", clipDuration)))
			}
		}

		syscmd := exec.Command("clipclean", fmt.Sprint(clipDuration))
		syscmd.Start()
	},
}

func getRecordNameWithFzf(storage *strg.Storage) (string, error) {
	// Check if fzf is installed
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf is not installed. Please install fzf to use this feature or use 'psw get <name>' instead")
	}

	cmd := exec.Command("fzf")

	var input bytes.Buffer
	input.WriteString(strings.Join(storage.GetNames(), "\n"))
	cmd.Stdin = &input

	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Failed to run fzf:\n%w", err)
	}

	return strings.TrimSpace(output.String()), nil
}
