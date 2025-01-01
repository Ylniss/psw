package cmd

import (
	"fmt"

	"github.com/TwiN/go-color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/prmpt"
	"github.com/ylniss/psw/strg"
)

func init() {
	rootCmd.AddCommand(changeCmd)
}

var changeCmd = &cobra.Command{
	Use: `change [name] [flags]

Arguments:
  name    Optional name of the record to change. If omitted, you'll be prompted to provide it.
          You can also change your main password with this command, just pass 'main' as name`,
	Short: "Change chosen record data",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 && args[0] == "main" {
			changeMainPass()
			strg.GitCommit("main password changed")
		} else {
			changeRecord(args)
			strg.GitCommit("record updated")
		}
	},
}

func changeMainPass() {
	fmt.Println(color.InCyan("You are changing your main password!\nFirst enter your current password"))

	mainPass, err := prmpt.PromptForMainPass(true) // prompt with ensure
	if err != nil {
		fmt.Println(err.Error())
	}

	storage, err := strg.Get(mainPass)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	storageJson, err := storage.ToJson()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	newMainPass, err := prmpt.PromptForMainPassChange()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = strg.EncryptStringToStorage(storageJson, newMainPass)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(color.InGreen("Main password changed"))

	return
}

func changeRecord(args []string) {
	storage, err := strg.GetOrCreateIfNotExists()
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

	log.Debugf("cmd/change - record: %#v\n", record)

	if !isFound {
		fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
		return
	}

	if yes := prmpt.YesOrNo("Do you want to change record name?"); yes {
		newName, err := prmpt.PromptForName("New name")
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if storage.Exists(newName) {
			fmt.Printf("Record with name %s already exists\n", color.InGreen(newName))
			return
		}

		record.Name = newName
	}

	if record.Value == "" {
		if yes := prmpt.YesOrNo("Do you want to change username?"); yes {
			newUser, err := prmpt.PromptForName("New username")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			record.User = newUser
		}

		if yes := prmpt.YesOrNo("Do you want to change password?"); yes {
			newPass, err := prmpt.PromptForRecordPass()
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			record.Pass = newPass
		}
	} else {
		if yes := prmpt.YesOrNo("Do you want to change value?"); yes {
			newValue, err := prmpt.PromptForName("New value")
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			record.Value = newValue
		}
	}

	storage.UpdateRecord(recordName, record)

	storageJson, err := storage.ToJson()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	log.Debugf("updated storage content:\n%s\n", storageJson)

	err = strg.EncryptStringToStorage(storageJson, storage.MainPass)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Printf(color.InGreen("Record updated\n"))
}
