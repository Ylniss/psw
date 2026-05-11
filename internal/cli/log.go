package cli

import (
	"fmt"
	"strings"

	color "github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/storage"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show storage commit history",
	Long:  "Prints commits from the storage git repository: short SHA, date and message, colorized by action type.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ok, err := storage.IsGitRepo()
		if err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		if !ok {
			fmt.Println(color.InRed("Storage is not a git repository, no log to show."))
			return errSilentExit
		}
		entries, err := storage.GitLog()
		if err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		for _, entry := range entries {
			fmt.Printf("%s  %s  %s\n",
				color.InCyan(entry.ShortSHA),
				entry.Time.Format("2006-01-02 15:04"),
				colorizeLogMessage(entry.Message),
			)
		}
		return nil
	},
}

func colorizeLogMessage(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "rollback"):
		return color.InPurple(msg)
	case strings.Contains(low, "add"):
		return color.InGreen(msg)
	case strings.Contains(low, "update"), strings.Contains(low, "change"):
		return color.InYellow(msg)
	case strings.Contains(low, "remove"):
		return color.InRed(msg)
	}
	return msg
}
