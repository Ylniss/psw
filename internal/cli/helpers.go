package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/TwiN/go-color"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

// printForkUndecryptable prints the fork-undecryptable banner in red.
func printForkUndecryptable() {
	fmt.Println(color.InRed(storage.ForkUndecryptableUserMessage))
}

// handleCmdErr classifies err for a RunE. Returns (true, retval) → caller
// returns retval; (false, nil) → caller continues. Any user-facing print is
// done before return. Cases:
//
//	nil                          → (false, nil)
//	prompt.ErrPromptCancelled    → (true, nil)         silent exit
//	errSilentExit                → (true, errSilentExit) already-printed sentinel
//	storage.ErrForkUndecryptable → (true, errSilentExit) banner printed
//	anything else                → (true, nil)         err printed
func handleCmdErr(err error) (stop bool, ret error) {
	if err == nil {
		return false, nil
	}
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return true, nil
	}
	if errors.Is(err, errSilentExit) {
		return true, errSilentExit
	}
	if errors.Is(err, storage.ErrForkUndecryptable) {
		printForkUndecryptable()
		return true, errSilentExit
	}
	fmt.Println(err.Error())
	return true, nil
}

// handlePromptErr handles a prompt error in a helper that returns bool.
// Returns true to abort.
func handlePromptErr(err error) (abort bool) {
	if err == nil {
		return false
	}
	if !errors.Is(err, prompt.ErrPromptCancelled) {
		fmt.Println(err.Error())
	}
	return true
}

// resolveRecordName returns the record selected by the user. With exact,
// args[0] must match a name or extras entry — otherwise prints and returns
// errSilentExit. Without exact, falls back to substring match + interactive
// picker. extras (e.g. "main-password") render after real records and pass
// the same substring filter. Cancel returns ("", nil).
func resolveRecordName(store *storage.Storage, args []string, exact bool, extras []string) (string, error) {
	if exact {
		if len(args) == 0 {
			fmt.Println("--exact requires a record name argument")
			return "", errSilentExit
		}
		name := args[0]
		if store.Exists(name) || slices.Contains(extras, name) {
			return name, nil
		}
		fmt.Printf("Record %s was not found\n", color.InGreen(name))
		return "", errSilentExit
	}

	names := store.GetNames()
	if len(args) > 0 {
		names = store.GetNamesWithPart(args[0])
		extras = filterExtrasByPart(extras, args[0])
	}
	name, err := storage.GetRecordNameInteractive(names, extras)
	if errors.Is(err, storage.ErrPickerCancelled) {
		return "", nil
	}
	return name, err
}

func filterExtrasByPart(extras []string, part string) []string {
	lp := strings.ToLower(part)
	var matched []string
	for _, e := range extras {
		if strings.Contains(strings.ToLower(e), lp) {
			matched = append(matched, e)
		}
	}
	return matched
}
