package prmpt

import (
	"errors"
	"fmt"
	"os"

	"github.com/TwiN/go-color"
	"github.com/cqroot/prompt"
	"github.com/cqroot/prompt/input"
	"golang.org/x/term"
)

var (
	passwordsDontMatchMsg = "Passwords don't match, try again"
	errMainPassLen        = errors.New("main password must be at least 4 characters long")
	errRequired           = errors.New("input required")
	errInvalidYesNo       = errors.New("input must be one of the following: y, yes, n, no")
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

func YesOrNo(question string) bool {
	fmt.Printf("%s (y/n)\n", question)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(buf); err != nil {
			fmt.Println("Error reading key. Please try again.")
			continue
		}

		switch buf[0] {
		case 'y':
			return true
		case 'n':
			return false
		case 0x03: // Ctrl-C — raw mode disables SIGINT, so handle explicitly
			term.Restore(fd, oldState)
			os.Exit(1)
		}
	}
}

func PromptForMainPassChange() (string, error) {
	// when changing password by default ensure if new is correct (first true arg)
	return promptForMainPass(true, true)
}

func PromptForMainPass(ensure bool) (string, error) {
	return promptForMainPass(ensure, false)
}

func promptForMainPass(ensure bool, mainPassChange bool) (string, error) {
	// Env-var override for tests/scripting. Bypasses ensure/double-confirm
	// since the env var is inherently authoritative.
	envVar := "PSW_MAIN_PASSWORD"
	if mainPassChange {
		envVar = "PSW_NEW_MAIN_PASSWORD"
	}
	if envPass := os.Getenv(envVar); envPass != "" {
		if err := validateMainPassLen(envPass); err != nil {
			return "", fmt.Errorf("%s: %w", envVar, err)
		}
		return envPass, nil
	}

	mainPass := "*"
	repeatMainPass := ""

	getFirstAskText := func(mainPassChange bool) string {
		if mainPassChange {
			return "New main password"
		} else {
			return "Main password"
		}
	}

	getRepeatAskText := func(mainPassChange bool) string {
		if mainPassChange {
			return "Repeat new main password"
		} else {
			return "Repeat main password"
		}
	}

	var err error
	for mainPass != repeatMainPass || errors.Is(err, errMainPassLen) { // ask until passwords match and is valid length
		mainPass, err = prompt.New().Ask(getFirstAskText(mainPassChange)).
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateMainPassLen))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}

			if errors.Is(err, errMainPassLen) {
				fmt.Println(color.InYellow(errMainPassLen.Error()))
				continue
			}

			return "", err
		}

		if !ensure {
			return mainPass, nil
		}

		repeatMainPass, err = prompt.New().Ask(getRepeatAskText(mainPassChange)).
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateMainPassLen))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}

			if errors.Is(err, errMainPassLen) {
				fmt.Println(color.InYellow(errMainPassLen.Error()))
				continue
			}

			return "", err
		}

		if mainPass != repeatMainPass {
			fmt.Println(color.InYellow(passwordsDontMatchMsg))
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
