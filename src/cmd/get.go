package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
	"golang.design/x/clipboard"
)

var show bool

func init() {
	getCmd.Flags().BoolVarP(&show, "show", "s", false, "show secret inside terminal")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use: `get [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to select a record with fzf`,
	Short: "Get secrets from record with specified name",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var recordName string
		if len(args) == 0 {
			recordName, err = getRecordNamesWithFzf(storage)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			recordName = args[0]
		}

		record, isFound := storage.GetRecord(recordName)

		if !isFound {
			fmt.Printf("Record '%s' was not found\n", recordName)
			return
		}

		clipboard.Write(clipboard.FmtText, []byte(record.Pass))

		// Print user & pass or value, depending on what's available
		if record.Value == "" {
			fmt.Println(color.InYellow(record.User))
			if show {
				fmt.Println(color.InYellow(record.Pass))
			} else {
				fmt.Println(color.InYellow("Password has been copied to the clipboard. It will be cleared in 30 seconds."))
			}
		} else {
			fmt.Println(color.InYellow(record.Value))
		}

		syscmd := exec.Command("clipclean", "10")
		syscmd.Start()
	},
}

func getRecordNamesWithFzf(storage *strg.Storage) (string, error) {
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
