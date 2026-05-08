package cli

import (
	"errors"
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/ylniss/psw/internal/storage"
)

// resolveRecordName returns the name of the record selected by the user.
// When exact is set, args[0] must be present and match a record name
// exactly; on miss, prints an error and returns errExit (exit 1). Otherwise
// falls back to substring + interactive selection. If the user cancels the
// picker, returns ("", nil) so the caller exits silently.
func resolveRecordName(store *storage.Storage, args []string, exact bool) (string, error) {
	if exact {
		if len(args) == 0 {
			fmt.Println("--exact requires a record name argument")
			return "", errExit
		}
		name := args[0]
		if !store.Exists(name) {
			fmt.Printf("Record %s was not found\n", color.InGreen(name))
			return "", errExit
		}
		return name, nil
	}

	names := store.GetNames()
	if len(args) > 0 {
		names = store.GetNamesWithPart(args[0])
	}
	name, err := storage.GetRecordNameInteractive(names)
	if errors.Is(err, storage.ErrPickerCancelled) {
		return "", nil
	}
	return name, err
}
