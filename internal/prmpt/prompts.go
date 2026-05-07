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
	errRequired           = errors.New("input required")
)

func validateRequired(content string) error {
	if len(content) < 1 {
		return errRequired
	}

	return nil
}

func YesOrNo(question string) bool {
	fmt.Printf("%s (y/n)\n", question)

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Non-TTY stdin: default to "no" for safety.
		return false
	}
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
		case 0x03: // Ctrl-C; raw mode disables SIGINT
			term.Restore(fd, oldState) // os.Exit skips defer
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
		return envPass, nil
	}

	mainPass := "*"
	repeatMainPass := ""

	askText := "Main password"
	repeatText := "Repeat main password"
	if mainPassChange {
		askText = "New main password"
		repeatText = "Repeat new main password"
	}

	for mainPass != repeatMainPass {
		var err error
		mainPass, err = prompt.New().Ask(askText).
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateRequired))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
			}
			return "", err
		}

		if !ensure {
			return mainPass, nil
		}

		repeatMainPass, err = prompt.New().Ask(repeatText).
			Input("", input.WithEchoMode(input.EchoPassword), input.WithValidateFunc(validateRequired))
		if err != nil {
			if errors.Is(err, prompt.ErrUserQuit) {
				os.Exit(1)
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
