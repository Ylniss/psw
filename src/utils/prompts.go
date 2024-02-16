package utils

import (
	"errors"
	"os"

	"github.com/cqroot/prompt"
	"github.com/cqroot/prompt/input"
)

var ErrPasswordsDontMatch = errors.New("Passwords don't match, try again")

func validateMainPassLen(content string) error {
	if len(content) < 4 {
		return errors.New("Main password must be at least 4 characters long")
	}

	return nil
}

func PromptForMainPass(ensure bool) (string, error) {
	mainPass, err := prompt.New().Ask("Main password").
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

	repeatMainPass, err := prompt.New().Ask("Repeat main password").
		Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateMainPassLen))
	if err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(1)
		}
		return "", err
	}

	if mainPass != repeatMainPass {
		return "", ErrPasswordsDontMatch
	}

	return mainPass, nil
}

func PromptForRecordPass() (string, error) {
	recordPass, err := prompt.New().Ask("Password").
		Input("", input.WithEchoMode(input.EchoPassword))
	if err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(1)
		}
		return "", err
	}

	repeatRecordPass, err := prompt.New().Ask("Repeat password").
		Input("", input.WithEchoMode(input.EchoPassword))
	if err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(1)
		}
		return "", err
	}

	if recordPass != repeatRecordPass {
		return "", ErrPasswordsDontMatch
	}

	return recordPass, nil
}

func PromptForName(promptText string) (string, error) {
	password, err := prompt.New().Ask(promptText).Input("")
	if err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(1)
		}
		return "", err
	}
	return password, nil
}
