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

// handleCmdErr maps a non-nil err to the value a RunE must return. Any
// user-facing print is done before return. Call as
// `if err != nil { return handleCmdErr(err) }`. Cases:
//
//	prompt.ErrPromptCancelled    → nil            silent exit
//	errSilentExit                → errSilentExit  already-printed sentinel
//	storage.ErrForkUndecryptable → errSilentExit  banner printed
//	storage.ErrPSW1Unsupported   → errSilentExit  red banner printed
//	anything else                → nil            err printed
func handleCmdErr(err error) error {
	if errors.Is(err, prompt.ErrPromptCancelled) {
		return nil
	}
	if errors.Is(err, errSilentExit) {
		return errSilentExit
	}
	if errors.Is(err, storage.ErrForkUndecryptable) {
		fmt.Println(color.InRed(storage.ForkUndecryptableUserMessage))
		return errSilentExit
	}
	if errors.Is(err, storage.ErrPSW1Unsupported) {
		fmt.Println(color.InRed(err.Error()))
		return errSilentExit
	}
	fmt.Println(err.Error())
	return nil
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
			fmt.Println("--exact needs a record name")
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
	name, err := prompt.GetRecordNameInteractive(names, extras)
	if errors.Is(err, prompt.ErrPickerCancelled) {
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
			fmt.Println("--exact needs at least one record name")
			return nil, errSilentExit
		}
		var missing []string
		for _, name := range args {
			if !store.Exists(name) {
				missing = append(missing, color.InGreen(name))
			}
		}
		if len(missing) > 0 {
			fmt.Printf("Records not found: %s\n", strings.Join(missing, ", "))
			return nil, errSilentExit
		}
		return dedupe(args), nil
	}

	if len(args) >= 2 {
		fmt.Println("passing multiple names requires --exact")
		return nil, errSilentExit
	}

	names := store.GetNames()
	if len(args) == 1 {
		names = store.GetNamesWithPart(args[0])
	}
	picked, err := prompt.GetRecordNamesInteractive(names)
	if errors.Is(err, prompt.ErrPickerCancelled) {
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

// recordType: "value" or "user/pass". Safe for debug logs.
func recordType(r storage.Record) string {
	if len(r.Value) != 0 {
		return "value"
	}
	return "user/pass"
}
