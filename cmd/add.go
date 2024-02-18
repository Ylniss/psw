package cmd

import (
	"fmt"
	"strings"

	"github.com/TwiN/go-color"
	passgen "github.com/sethvargo/go-password/password"
	log "github.com/sirupsen/logrus"
	"github.com/ylniss/psw/strg"
	"github.com/ylniss/psw/utils"

	"github.com/spf13/cobra"
)

var (
	singleValFlag bool
	genPassFlag   bool
)

func init() {
	addCmd.Flags().BoolVarP(&singleValFlag, "single", "s", false, "add single value into a record instead of username/password")
	addCmd.Flags().BoolVarP(&genPassFlag, "generate", "g", false, "auto generate random password")
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use: `add [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to provide it`,
	Short: "Add new record with secrets",
	Long:  `Add username/password or a value that will be stored in a record with provided name`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if singleValFlag && genPassFlag {
			fmt.Printf("Flags %s and %s cannot be used together. %s works only for passwords.\n",
				color.InCyan("--single"),
				color.InCyan("--generate"),
				color.InCyan("--generate"))
			return
		}

		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		recordName, err := getRecordName(args)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if strings.ToLower(recordName) == "main" {
			fmt.Printf("Name %s is reserved. %s command uses it for changing main password\n", color.InGreen("main"), color.InCyan("change"))
			return
		}

		if storage.IsDuplicate(recordName) {
			fmt.Printf("Record with name %s already exists\n", color.InGreen(recordName))
			return
		}

		if singleValFlag {
			recordVal, err := utils.PromptForName("Value")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			storage.AddRecord(&strg.Record{Name: recordName, Value: recordVal})
		} else {
			recordUser, err := utils.PromptForName("Username")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			recordPass, err := getOrGenerateRecordPass()
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			storage.AddRecord(&strg.Record{Name: recordName, User: recordUser, Pass: recordPass})
		}

		storageStr := storage.String()

		log.Debugf("new storage content:\n%s", storageStr)

		err = strg.EncryptStringToStorage(storageStr, storage.MainPass)
		if err != nil {
			fmt.Println(err.Error())
		}
	},
}

func getRecordName(args []string) (string, error) {
	var recordName string
	var err error
	if len(args) == 0 {
		recordName, err = utils.PromptForName("Record name")
		if err != nil {
			return "", err
		}
	} else {
		recordName = args[0]
	}
	return recordName, nil
}

func getOrGenerateRecordPass() (string, error) {
	var recordPass string
	var err error

	if genPassFlag {
		recordPass, err = passgen.Generate(16, 4, 6, false, true)
	} else {
		recordPass, err = utils.PromptForRecordPass()
	}

	if err != nil {
		return "", err
	}

	return recordPass, nil
}
