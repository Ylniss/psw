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

// resolveRecordNames is the multi-select counterpart of resolveRecordName.
// Exact: every arg must match; all missing names are reported in one error
// before any mutation. Non-exact: 0 args → multi-picker; 1 arg → substring
// filter; ≥2 args → rejected. Cancel returns (nil, nil).
func resolveRecordNames(store *storage.Storage, args []string, exact bool) ([]string, error) {
	if exact {
		if len(args) == 0 {
			fmt.Println("--exact requires at least one record name argument")
			return nil, errSilentExit
		}
		var missing []string
		for _, name := range args {
			if !store.Exists(name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			colored := make([]string, len(missing))
			for i, n := range missing {
				colored[i] = color.InGreen(n)
			}
			fmt.Printf("Records not found: %s\n", strings.Join(colored, ", "))
			return nil, errSilentExit
		}
		return dedupe(args), nil
	}

	if len(args) >= 2 {
		fmt.Println("multiple names require --exact")
		return nil, errSilentExit
	}

	names := store.GetNames()
	if len(args) == 1 {
		names = store.GetNamesWithPart(args[0])
	}
	picked, err := storage.GetRecordNamesInteractive(names)
	if errors.Is(err, storage.ErrPickerCancelled) {
		return nil, nil
	}
	return picked, err
}

func dedupe(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}
