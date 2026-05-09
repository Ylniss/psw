package cli

import (
	"errors"
	"strings"

	"github.com/TwiN/go-color"
	passgen "github.com/sethvargo/go-password/password"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"

	"github.com/spf13/cobra"
)

var (
	singleValueFlag      bool
	generatePasswordFlag bool
	addUsernameFlag      string
	addPasswordFlag      string
	addValueFlag         string
)

func init() {
	addCmd.Flags().BoolVarP(&singleValueFlag, "single", "s", false, "add single value into a record instead of username/password")
	addCmd.Flags().BoolVarP(&generatePasswordFlag, "generate", "g", false, "auto generate random password")
	addCmd.Flags().StringVarP(&addUsernameFlag, "username", "u", "", "username (skips username prompt)")
	addCmd.Flags().StringVar(&addPasswordFlag, "password", "", "password (skips password prompt)")
	addCmd.Flags().StringVar(&addValueFlag, "value", "", "value for --single records (skips value prompt)")
}

var addCmd = &cobra.Command{
	Use: `add [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to provide it`,
	Short: "Add new record with secrets",
	Long:  `Add username/password or a value that will be stored in a record with provided name`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if singleValueFlag && generatePasswordFlag {
			menuPrintf("Flags %s and %s cannot be used together. %s works only for passwords.\n",
				color.InCyan("--single"),
				color.InCyan("--generate"),
				color.InCyan("--generate"))
			return errExit
		}

		usernameSet := cmd.Flags().Changed("username")
		passwordSet := cmd.Flags().Changed("password")
		valueSet := cmd.Flags().Changed("value")

		if singleValueFlag && (usernameSet || passwordSet) {
			menuPrintf("Flags %s and %s cannot be combined with %s.\n",
				color.InCyan("--username"),
				color.InCyan("--password"),
				color.InCyan("--single"))
			return errExit
		}
		if valueSet && !singleValueFlag {
			menuPrintf("Flag %s requires %s.\n", color.InCyan("--value"), color.InCyan("--single"))
			return errExit
		}
		if passwordSet && generatePasswordFlag {
			menuPrintf("Flags %s and %s cannot be used together.\n",
				color.InCyan("--password"),
				color.InCyan("--generate"))
			return errExit
		}

		store, err := storage.GetOrCreateForMutate()
		if errors.Is(err, prompt.ErrPromptCancelled) {
			return nil
		}
		if errors.Is(err, storage.ErrForkUndecryptable) {
			printForkUndecryptable()
			return errExit
		}
		if err != nil {
			menuPrintln(err.Error())
			return nil
		}

		recordName, err := getRecordName(args)
		if errors.Is(err, prompt.ErrPromptCancelled) {
			return nil
		}
		if err != nil {
			menuPrintln(err.Error())
			return nil
		}

		if strings.ToLower(recordName) == "main" {
			menuPrintf("Name %s is reserved. %s command uses it for changing main password\n", color.InGreen("main"), color.InCyan("change"))
			return nil
		}

		if store.Exists(recordName) {
			menuPrintf("Record with name %s already exists\n", color.InGreen(recordName))
			return nil
		}

		if singleValueFlag {
			recordValue, err := getOrPromptValue(valueSet)
			if errors.Is(err, prompt.ErrPromptCancelled) {
				return nil
			}
			if err != nil {
				menuPrintln(err.Error())
				return nil
			}
			store.AddRecord(&storage.Record{Name: recordName, Value: recordValue})
		} else {
			recordUsername, err := getOrPromptUsername(usernameSet)
			if errors.Is(err, prompt.ErrPromptCancelled) {
				return nil
			}
			if err != nil {
				menuPrintln(err.Error())
				return nil
			}
			recordPassword, err := getOrPromptPassword(passwordSet)
			if errors.Is(err, prompt.ErrPromptCancelled) {
				return nil
			}
			if err != nil {
				menuPrintln(err.Error())
				return nil
			}
			store.AddRecord(&storage.Record{Name: recordName, Username: recordUsername, Password: recordPassword})
		}

		if err := store.Save(); err != nil {
			menuPrintln(err.Error())
			return nil
		}

		if singleValueFlag {
			menuPrintf("Value set successfully in %s record\n", color.InGreen(recordName))
		} else {
			menuPrintf("Username/password set successfully in %s record\n", color.InGreen(recordName))
		}

		storage.GitCommit("added new record")
		return nil
	},
}

func getRecordName(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return prompt.PromptForName("Record name")
}

func getOrPromptUsername(flagSet bool) (string, error) {
	if flagSet {
		return addUsernameFlag, nil
	}
	return prompt.PromptForName("Username")
}

func getOrPromptPassword(flagSet bool) (string, error) {
	if flagSet {
		return addPasswordFlag, nil
	}
	return getOrGenerateRecordPassword()
}

func getOrPromptValue(flagSet bool) (string, error) {
	if flagSet {
		return addValueFlag, nil
	}
	return prompt.PromptForName("Value")
}

func getOrGenerateRecordPassword() (string, error) {
	if generatePasswordFlag {
		return passgen.Generate(16, 4, 6, false, true)
	}
	return prompt.PromptForRecordPassword()
}
