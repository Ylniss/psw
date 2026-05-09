package cli

import (
	"errors"
	"fmt"

	"github.com/TwiN/go-color"
	"github.com/ylniss/psw/internal/menulayout"
	"github.com/ylniss/psw/internal/storage"
)

// menuPrintln is fmt.Println with menu-mode indent + wrap applied.
func menuPrintln(s string) { fmt.Println(menulayout.Render(s)) }

// menuPrintf is fmt.Printf with menu-mode indent + wrap applied.
func menuPrintf(format string, args ...any) {
	fmt.Print(menulayout.Render(fmt.Sprintf(format, args...)))
}

// printForkUndecryptable prints the red error shown when storage was
// re-encrypted under a different main password.
func printForkUndecryptable() {
	menuPrintln(color.InRed("Storage was re-encrypted under a different main password since the last sync. Push from the device that ran 'change main' first, then retry on this device."))
}

// resolveRecordName returns the name of the record selected by the user.
// When exact is set, args[0] must be present and match a record name
// exactly; on miss, prints an error and returns errExit (exit 1). Otherwise
// falls back to substring + interactive selection. If the user cancels the
// picker, returns ("", nil) so the caller exits silently.
func resolveRecordName(store *storage.Storage, args []string, exact bool) (string, error) {
	if exact {
		if len(args) == 0 {
			menuPrintln("--exact requires a record name argument")
			return "", errExit
		}
		name := args[0]
		if !store.Exists(name) {
			menuPrintf("Record %s was not found\n", color.InGreen(name))
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
