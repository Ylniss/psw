package cmd

import (
	"fmt"
	"os"

	"github.com/TwiN/go-color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/prmpt"
	"github.com/ylniss/psw/strg"
)

var (
	changeExactFlag  bool
	changeRenameFlag string
	changeUserFlag   string
	changePassFlag   string
	changeValueFlag  string
)

func init() {
	changeCmd.Flags().BoolVarP(&changeExactFlag, "exact", "e", false, "exact name match; skip fzf and substring search")
	changeCmd.Flags().StringVar(&changeRenameFlag, "rename", "", "new record name (skips rename y/n + prompt)")
	changeCmd.Flags().StringVarP(&changeUserFlag, "username", "u", "", "new username (skips username y/n + prompt)")
	changeCmd.Flags().StringVar(&changePassFlag, "password", "", "new password (skips password y/n + prompt)")
	changeCmd.Flags().StringVar(&changeValueFlag, "value", "", "new value for value-only records (skips value y/n + prompt)")
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
			if cmd.Flags().Changed("rename") || cmd.Flags().Changed("username") ||
				cmd.Flags().Changed("password") || cmd.Flags().Changed("value") {
				fmt.Println("Record-level flags (--rename/--username/--password/--value) are not valid with 'change main'")
				os.Exit(1)
			}
			changeMainPass()
			strg.GitCommit("main password changed")
		} else {
			changeRecord(cmd, args)
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

func changeRecord(cmd *cobra.Command, args []string) {
	renameSet := cmd.Flags().Changed("rename")
	userSet := cmd.Flags().Changed("username")
	passSet := cmd.Flags().Changed("password")
	valSet := cmd.Flags().Changed("value")

	// If any field flag was passed, switch to non-interactive mode: only
	// touch the fields whose flag is set; leave the rest unchanged.
	anyFlagSet := renameSet || userSet || passSet || valSet

	storage, err := strg.GetOrCreateIfNotExists()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	recordName, err := resolveRecordName(storage, args, changeExactFlag)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	record, isFound := storage.GetRecord(recordName)

	log.Debugf("cmd/change - record: %#v\n", record)

	if !isFound {
		fmt.Printf("Record %s was not found\n", color.InGreen(recordName))
		return
	}

	if (userSet || passSet) && record.Value != "" {
		fmt.Printf("Record %s is value-only; --username/--password not applicable\n", color.InGreen(recordName))
		os.Exit(1)
	}
	if valSet && record.Value == "" {
		fmt.Printf("Record %s is user/pass; --value not applicable\n", color.InGreen(recordName))
		os.Exit(1)
	}

	if renameSet {
		if storage.Exists(changeRenameFlag) {
			fmt.Printf("Record with name %s already exists\n", color.InGreen(changeRenameFlag))
			return
		}
		record.Name = changeRenameFlag
	} else if !anyFlagSet {
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
	}

	if record.Value == "" {
		if userSet {
			record.User = changeUserFlag
		} else if !anyFlagSet {
			if yes := prmpt.YesOrNo("Do you want to change username?"); yes {
				newUser, err := prmpt.PromptForName("New username")
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				record.User = newUser
			}
		}

		if passSet {
			record.Pass = changePassFlag
		} else if !anyFlagSet {
			if yes := prmpt.YesOrNo("Do you want to change password?"); yes {
				newPass, err := prmpt.PromptForRecordPass()
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				record.Pass = newPass
			}
		}
	} else {
		if valSet {
			record.Value = changeValueFlag
		} else if !anyFlagSet {
			if yes := prmpt.YesOrNo("Do you want to change value?"); yes {
				newValue, err := prmpt.PromptForName("New value")
				if err != nil {
					fmt.Println(err.Error())
					return
				}

				record.Value = newValue
			}
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
