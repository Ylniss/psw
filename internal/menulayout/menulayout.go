// Package menulayout holds the indent/width column for `psw menu`'s dispatched
// subcommand. One source of truth — don't mirror this state elsewhere.
//
// Not concurrent-safe. One runMenu invocation at a time, no nesting.
package menulayout

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	currentIndent int
	currentWidth  int
)

// Set / Clear bracket the dispatched subcommand. Only called from runMenu.
func Set(indent, width int) { currentIndent, currentWidth = indent, width }
func Clear()                { currentIndent, currentWidth = 0, 0 }
func Indent() int           { return currentIndent }

// Render indents text and wraps to width. Pass-through when neither is set.
// Trailing newline preserved.
func Render(text string) string { return applyStyle(text, true) }

// RenderIndent indents text without wrapping. For TUI views where wrap would
// mangle a textinput.
func RenderIndent(text string) string { return applyStyle(text, false) }

// applyStyle is the shared core. The strip-restore is load-bearing: lipgloss
// treats a trailing \n as a hard break and emits a phantom blank line padded
// with margin/width spaces. Strip before, restore after.
func applyStyle(text string, applyWrap bool) string {
	wrapsToWidth := applyWrap && currentWidth > 0
	if currentIndent == 0 && !wrapsToWidth {
		return text
	}
	hasTrailingNewline := strings.HasSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\n")
	style := lipgloss.NewStyle()
	if wrapsToWidth {
		style = style.Width(currentWidth)
	}
	if currentIndent > 0 {
		style = style.MarginLeft(currentIndent)
	}
	rendered := style.Render(text)
	if hasTrailingNewline {
		rendered += "\n"
	}
	return rendered
}
