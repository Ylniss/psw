package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/TwiN/go-color"
	"github.com/ylniss/psw/internal/storage"
)

// printForkUndecryptable prints the red error shown when storage was
// re-encrypted under a different main password.
func printForkUndecryptable() {
	fmt.Println(color.InRed(storage.ForkUndecryptableUserMessage))
}

// resolveRecordName returns the name of the record selected by the user.
// When exact is set, args[0] must match a record name or an entry in extras
// exactly; on miss, prints an error and returns errExit (exit 1). Otherwise
// falls back to substring + interactive selection. extras (e.g. "main-password"
// for `change`) appear after real records in the picker and survive
// substring filtering by the same predicate. If the user cancels the picker,
// returns ("", nil) so the caller exits silently.
func resolveRecordName(store *storage.Storage, args []string, exact bool, extras []string) (string, error) {
	if exact {
		if len(args) == 0 {
			fmt.Println("--exact requires a record name argument")
			return "", errExit
		}
		name := args[0]
		if store.Exists(name) || slices.Contains(extras, name) {
			return name, nil
		}
		fmt.Printf("Record %s was not found\n", color.InGreen(name))
		return "", errExit
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
