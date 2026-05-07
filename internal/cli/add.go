package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/TwiN/go-color"
	passgen "github.com/sethvargo/go-password/password"
	"github.com/ylniss/psw/internal/prmpt"
	"github.com/ylniss/psw/internal/strg"

	"github.com/spf13/cobra"
)

var (
	singleValFlag bool
	genPassFlag   bool
	addUserFlag   string
	addPassFlag   string
	addValueFlag  string
)

func init() {
	addCmd.Flags().BoolVarP(&singleValFlag, "single", "s", false, "add single value into a record instead of username/password")
	addCmd.Flags().BoolVarP(&genPassFlag, "generate", "g", false, "auto generate random password")
	addCmd.Flags().StringVarP(&addUserFlag, "username", "u", "", "username (skips username prompt)")
	addCmd.Flags().StringVar(&addPassFlag, "password", "", "password (skips password prompt)")
	addCmd.Flags().StringVar(&addValueFlag, "value", "", "value for --single records (skips value prompt)")
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use: `add [name] [flags]

Arguments:
  name    Optional name of the record to get. If omitted, you'll be prompted to provide it`,
	Short: "Add new record with secrets",
	Long:  `Add username/password or a value that will be stored in a record with provided name`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if singleValFlag && genPassFlag {
			fmt.Printf("Flags %s and %s cannot be used together. %s works only for passwords.\n",
				color.InCyan("--single"),
				color.InCyan("--generate"),
				color.InCyan("--generate"))
			return errExit
		}

		userSet := cmd.Flags().Changed("username")
		passSet := cmd.Flags().Changed("password")
		valSet := cmd.Flags().Changed("value")

		if singleValFlag && (userSet || passSet) {
			fmt.Printf("Flags %s and %s cannot be combined with %s.\n",
				color.InCyan("--username"),
				color.InCyan("--password"),
				color.InCyan("--single"))
			return errExit
		}
		if valSet && !singleValFlag {
			fmt.Printf("Flag %s requires %s.\n", color.InCyan("--value"), color.InCyan("--single"))
			return errExit
		}
		if passSet && genPassFlag {
			fmt.Printf("Flags %s and %s cannot be used together.\n",
				color.InCyan("--password"),
				color.InCyan("--generate"))
			return errExit
		}

		storage, err := strg.GetOrCreateIfNotExists()
		if errors.Is(err, prmpt.ErrPromptCancelled) {
			return nil
		}
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		recordName, err := getRecordName(args)
		if errors.Is(err, prmpt.ErrPromptCancelled) {
			return nil
		}
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		if strings.ToLower(recordName) == "main" {
			fmt.Printf("Name %s is reserved. %s command uses it for changing main password\n", color.InGreen("main"), color.InCyan("change"))
			return nil
		}

		if storage.Exists(recordName) {
			fmt.Printf("Record with name %s already exists\n", color.InGreen(recordName))
			return nil
		}

		if singleValFlag {
			recordVal, err := getOrPromptValue(valSet)
			if errors.Is(err, prmpt.ErrPromptCancelled) {
				return nil
			}
			if err != nil {
				fmt.Println(err.Error())
				return nil
			}
			storage.AddRecord(&strg.Record{Name: recordName, Value: recordVal})
		} else {
			recordUser, err := getOrPromptUsername(userSet)
			if errors.Is(err, prmpt.ErrPromptCancelled) {
				return nil
			}
			if err != nil {
				fmt.Println(err.Error())
				return nil
			}
			recordPass, err := getOrPromptPassword(passSet)
			if errors.Is(err, prmpt.ErrPromptCancelled) {
				return nil
			}
			if err != nil {
				fmt.Println(err.Error())
				return nil
			}
			storage.AddRecord(&strg.Record{Name: recordName, User: recordUser, Pass: recordPass})
		}

		if err := storage.Save(); err != nil {
			fmt.Println(err.Error())
			return nil
		}

		if singleValFlag {
			fmt.Printf("Value set successfully in %s record\n", color.InGreen(recordName))
		} else {
			fmt.Printf("Username/password set successfully in %s record\n", color.InGreen(recordName))
		}

		strg.GitCommit("added new record")
		return nil
	},
}

func getRecordName(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return prmpt.PromptForName("Record name")
}

func getOrPromptUsername(flagSet bool) (string, error) {
	if flagSet {
		return addUserFlag, nil
	}
	return prmpt.PromptForName("Username")
}

func getOrPromptPassword(flagSet bool) (string, error) {
	if flagSet {
		return addPassFlag, nil
	}
	return getOrGenerateRecordPass()
}

func getOrPromptValue(flagSet bool) (string, error) {
	if flagSet {
		return addValueFlag, nil
	}
	return prmpt.PromptForName("Value")
}

func getOrGenerateRecordPass() (string, error) {
	if genPassFlag {
		return passgen.Generate(16, 4, 6, false, true)
	}
	return prmpt.PromptForRecordPass()
}
