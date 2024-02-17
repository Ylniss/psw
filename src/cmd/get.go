package cmd

import (
	"fmt"
	"os/exec"

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
	Use:   "get <name>",
	Short: "Get secrets from record with specified name",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists(app.storageFilePath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		recordName := args[0]
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

		syscmd := exec.Command("clipclean", "30", record.Pass)
		syscmd.Start()
	},
}
