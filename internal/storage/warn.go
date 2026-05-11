package storage

import (
	"fmt"
	"os"

	color "github.com/TwiN/go-color"
)

// WarnSink, when non-nil, receives warning messages instead of stderr.
// Set by hosts that own the screen (e.g. menu mode).
var WarnSink func(string)

// Warn writes a yellow line through WarnSink, or stderr when sink is nil.
func Warn(format string, args ...any) {
	msg := color.InYellow(fmt.Sprintf(format, args...))
	if WarnSink != nil {
		WarnSink(msg)
		return
	}
	fmt.Fprintln(os.Stderr, msg)
}
