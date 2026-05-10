package menu

import (
	imgcolor "image/color"

	"charm.land/lipgloss/v2"
)

// Trailing whitespace OK; lipgloss.Width takes max line width.
const pswHeader = `
 ████████     █████   █████ ███ █████
░░███░░███   ███░░   ░░███ ░███░░███
 ░███ ░███  ░░█████   ░███ ░███ ░███
 ░███ ░███   ░░░░███  ░░███████████
 ░███████    ██████    ░░████░████
 ░███░░░    ░░░░░░      ░░░░ ░░░░
 ░███
 █████
░░░░░                             `

// pswHeaderWidth is the widest line; header hides below this terminal width.
var pswHeaderWidth = lipgloss.Width(pswHeader)

var defaultHeaderColor = lipgloss.Color("6")

func renderHeader(c imgcolor.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render(pswHeader)
}

// indentToHeader left-pads content to align with the centered header.
func indentToHeader(termWidth int, content string) string {
	if termWidth <= pswHeaderWidth {
		return content
	}
	indent := (termWidth - pswHeaderWidth) / 2
	return lipgloss.NewStyle().MarginLeft(indent).Render(content)
}

// wrapToHeader indents AND wraps content to the header column.
func wrapToHeader(termWidth int, content string) string {
	if termWidth <= pswHeaderWidth {
		return lipgloss.NewStyle().Width(termWidth).Render(content)
	}
	indent := (termWidth - pswHeaderWidth) / 2
	return lipgloss.NewStyle().Width(pswHeaderWidth).MarginLeft(indent).Render(content)
}

// centerHorizontally returns content as-is at width 0 (terminal size unknown).
func centerHorizontally(width int, content string) string {
	if width == 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}
