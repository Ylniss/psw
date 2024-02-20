package cmd

import (
	"fmt"

	"github.com/TwiN/go-color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

func init() {
	rootCmd.AddCommand(changeCmd)
}

var changeCmd = &cobra.Command{
	Use: `change [name] [flags]

Arguments:
  name    Optional name of the record to change. If omitted, you'll be prompted to provide it`,
	Short: "Change chosen record data",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		var recordName string
		if len(args) == 0 {
			recordName, err = strg.GetRecordNameWithFzf(storage)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			recordName = args[0]
		}

		record, isFound := storage.GetRecord(recordName)

		log.Debugf("cmd/change - record: %#v\n", record)

		if !isFound {
			fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
			return
		}
	},
}
