package utils

import (
	"errors"

	"github.com/manifoldco/promptui"
)

func PromptForPassword(text string) (string, error) {
	validate := func(input string) error {
		if len(input) < 4 {
			return errors.New("Password must be at least 4 characters long")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    text,
		Mask:     '*',
		Validate: validate,
	}

	return prompt.Run()
}
