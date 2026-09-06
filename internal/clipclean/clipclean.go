// Package clipclean spawns the clipclean helper. Resolves the binary
// next to psw first so a minimal PATH (niri spawn-sh, systemd user units)
// doesn't break clipboard auto-clear.
package clipclean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
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

// CopyAndSchedule copies secret to the clipboard and schedules clipclean to
// wipe it after timeoutSec. A helper that won't start clears the clipboard
// right away — never leave a secret behind with nothing to clean it up.
func CopyAndSchedule(secret string, timeoutSec int) error {
	if err := clipboard.WriteAll(secret); err != nil {
		return fmt.Errorf("Failed to copy value to clipboard: %w", err)
	}
	if err := Spawn(timeoutSec); err != nil {
		_ = clipboard.WriteAll("")
		return fmt.Errorf("Couldn't start clipboard cleanup: %w (clipboard cleared)", err)
	}
	return nil
}

// Run is the helper body: sleeps out the timeout in args[1], then clears the
// clipboard if it still holds what was copied. args is the raw os.Args.
func Run(args []string) {
	if len(args) < 2 {
		return // wrong number of arguments
	}
	timeoutSec, err := strconv.Atoi(args[1])
	if err != nil {
		return // timeout in incorrect format
	}
	if timeoutSec <= 0 {
		return // non-positive timeout: nothing to wait for
	}

	originalClip, err := clipboard.ReadAll()
	if err != nil {
		return // failed to read clipboard
	}

	time.Sleep(time.Duration(timeoutSec) * time.Second)

	currentClip, err := clipboard.ReadAll()
	if err != nil {
		return // failed to read clipboard
	}

	if currentClip == originalClip {
		if err := clipboard.WriteAll(""); err != nil {
			fmt.Fprintln(os.Stderr, "clipclean: failed to clear clipboard:", err)
		}
	}
}
