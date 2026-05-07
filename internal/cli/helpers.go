package cli

import (
	"errors"
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/ylniss/psw/internal/strg"
)

// resolveRecordName returns the name of the record selected by the user.
// When exact is set, args[0] must be present and match a record name
// exactly; on miss, prints an error and returns errExit (exit 1). Otherwise
// falls back to substring + interactive selection. If the user cancels the
// picker, returns ("", nil) so the caller exits silently.
func resolveRecordName(storage *strg.Storage, args []string, exact bool) (string, error) {
	if exact {
		if len(args) == 0 {
			fmt.Println("--exact requires a record name argument")
			return "", errExit
		}
		name := args[0]
		if !storage.Exists(name) {
			fmt.Printf("Record %s was not found\n", color.InGreen(name))
			return "", errExit
		}
		return name, nil
	}

	names := storage.GetNames()
	if len(args) > 0 {
		names = storage.GetNamesWithPart(args[0])
	}
	name, err := strg.GetRecordNameInteractive(names)
	if errors.Is(err, strg.ErrPickerCancelled) {
		return "", nil
	}
	return name, err
}
