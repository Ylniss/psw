package prmpt

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/TwiN/go-color"
	"github.com/eiannone/keyboard"
	"github.com/samber/lo"

	"github.com/cqroot/prompt"
	"github.com/cqroot/prompt/input"
)

var (
	passwordsDontMatchMsg = "Passwords don't match, try again"
	errMainPassLen        = errors.New("Main password must be at least 4 characters long")
	errRequired           = errors.New("Input required")
	errInvalidYesNo       = errors.New("Input must be one of the following: y, yes, n, no")
)

func validateMainPassLen(content string) error {
	if len(content) < 4 {
		return errMainPassLen
	}

	return nil
}

func validateRequired(content string) error {
	if len(content) < 1 {
		return errRequired
	}

	return nil
}

func validateYesOrNoInput(content string) error {
	valid := []string{"y", "yes", "n", "no"}
	if !lo.Contains(valid, strings.ToLower(content)) {
		return errInvalidYesNo
	}

	return nil
}

func YesOrNo(question string) bool {
	fmt.Printf("%s (y/n)\n", question)

	if err := keyboard.Open(); err != nil {
		panic(err)
	}
	defer keyboard.Close()

	for {
		char, _, err := keyboard.GetSingleKey()
		if err != nil {
			fmt.Println("Error reading key. Please try again.")
			continue
		}

		if char == 'y' {
			return true
		} else if char == 'n' {
			return false
		}
	}
}

func PromptForMainPass(ensure bool) (string, error) {
	mainPass := "*"
	repeatMainPass := ""

	var err error
	for mainPass != repeatMainPass { // ask until passwords match todo: and is valid length
		mainPass, err = prompt.New().Ask("Main password").
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateMainPassLen))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}
			return "", err
		}

		if !ensure {
			return mainPass, nil
		}

		repeatMainPass, err = prompt.New().Ask("Repeat main password").
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateMainPassLen))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}
			return "", err
		}

		if mainPass != repeatMainPass {
			fmt.Println(color.InCyan(passwordsDontMatchMsg))
		}
	}
	return mainPass, nil
}

func PromptForRecordPass() (string, error) {
	recordPass := "*"
	repeatRecordPass := ""

	for recordPass != repeatRecordPass { // ask until passwords match
		var err error
		recordPass, err = prompt.New().Ask("Password").
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateRequired))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}
			return "", err
		}

		repeatRecordPass, err = prompt.New().Ask("Repeat password").
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateRequired))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}
			return "", err
		}

		if recordPass != repeatRecordPass {
			fmt.Println(color.InCyan(passwordsDontMatchMsg))
		}
	}

	return recordPass, nil
}

func PromptForName(promptText string) (string, error) {
	password, err := prompt.New().Ask(promptText).Input("", input.WithValidateFunc(validateRequired))
	if err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(1)
		}
		return "", err
	}
	return password, nil
}