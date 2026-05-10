// Package clipclean spawns the clipclean helper. Resolves the binary
// next to psw first so a minimal PATH (niri spawn-sh, systemd user units)
// doesn't break clipboard auto-clear.
package clipclean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Spawn starts clipclean in the background with the clipboard timeout in seconds.
func Spawn(timeoutSec int) error {
	return exec.Command(resolve(), fmt.Sprint(timeoutSec)).Start()
}

func resolve() string {
	if exe, err := os.Executable(); err == nil {
		siblingPath := filepath.Join(filepath.Dir(exe), "clipclean")
		if _, err := os.Stat(siblingPath); err == nil {
			return siblingPath
		}
	}
	return "clipclean"
}
