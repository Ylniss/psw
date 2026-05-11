package menu

import (
	"fmt"
	imgcolor "image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// PSW ASCII header. Trailing whitespace OK; lipgloss.Width takes max line width.
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

// headerPaddingX widens the effective header column on each side. Footer
// content is centered within (pswHeaderWidth + 2*headerPaddingX); set so the
// picker help line (≈71 cells) fits without wrapping at typical widths.
const headerPaddingX = 18

var defaultHeaderColor = lipgloss.Color("6")

var menuActions = []string{"get", "add", "change", "remove"}

var (
	menuButtonStyle = lipgloss.NewStyle().Padding(0, 2)
	menuSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).Padding(0, 2)
	menuHelpStyle   = lipgloss.NewStyle().Faint(true)
	menuErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

const footerHelp = "←→/1-4 select · enter run · esc/q quit"

// actionFrameHeight is the rows above an action sub-view (header + spacers
// + buttons row). Computed from the same shape View() emits so it tracks
// layout changes automatically.
var actionFrameHeight = lipgloss.Height(renderActionFrame())

func renderActionFrame() string {
	var b strings.Builder
	b.WriteString(renderHeader(defaultHeaderColor))
	b.WriteString("\n\n")
	b.WriteString(menuButtonStyle.Render("[1] x"))
	b.WriteString("\n\n")
	return b.String()
}

func renderHeader(c imgcolor.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render(pswHeader)
}

func indentToHeader(termWidth int, content string) string {
	if termWidth <= pswHeaderWidth {
		return content
	}
	indent := (termWidth - pswHeaderWidth) / 2
	return lipgloss.NewStyle().MarginLeft(indent).Render(content)
}

// wrapToHeader indents and wraps to the header column.
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

// indentToFooter centers footer content within (pswHeaderWidth + 2*headerPaddingX),
// capped to termWidth so narrow terminals stay centered.
func indentToFooter(termWidth int, content string) string {
	if termWidth == 0 {
		return content
	}
	contentWidth := lipgloss.Width(content)
	if termWidth <= contentWidth {
		return content
	}
	column := pswHeaderWidth + 2*headerPaddingX
	if column > termWidth {
		column = termWidth
	}
	columnLeft := (termWidth - column) / 2
	extra := (column - contentWidth) / 2
	if extra < 0 {
		extra = 0
	}
	return lipgloss.NewStyle().MarginLeft(columnLeft + extra).Render(content)
}

// alignBlock left-margins content so its first non-space char lands at
// targetCol. Backs out any leading padding the content brings (bubbles/list
// items have 2 cells of left padding; bare prompts and spinners have none
// — both should align under [1]). Trims trailing spaces from each line
// because bubbles/list pads to its full width.
func alignBlock(termWidth, targetCol int, content string) (string, int) {
	trimmed := trimTrailingPerLine(content)
	if termWidth == 0 || termWidth <= targetCol {
		return trimmed, 0
	}
	indent := targetCol - minLeadingSpaces(trimmed)
	if indent < 0 {
		indent = 0
	}
	return lipgloss.NewStyle().MarginLeft(indent).Render(trimmed), indent
}

func trimTrailingPerLine(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// minLeadingSpaces returns the smallest count of leading space characters
// across non-blank lines. Used to back out content's own left padding when
// aligning to a target column.
func minLeadingSpaces(content string) int {
	min := -1
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := 0
		for _, r := range l {
			if r != ' ' {
				break
			}
			n++
		}
		if min == -1 || n < min {
			min = n
		}
	}
	if min < 0 {
		return 0
	}
	return min
}

// buttonsRowWidth measures the rendered buttons row so action sub-views can
// align under [1] regardless of changes to button labels.
func buttonsRowWidth() int {
	buttons := make([]string, len(menuActions))
	for i, a := range menuActions {
		label := fmt.Sprintf("[%d] %s", i+1, a)
		buttons[i] = menuButtonStyle.Render(label)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	return lipgloss.Width(row)
}

// firstButtonLeftCol returns the terminal column where [1]'s leftmost char
// renders: buttons row left edge plus the button style's left padding.
func firstButtonLeftCol(termWidth int) int {
	leftEdge := (termWidth - buttonsRowWidth()) / 2
	if leftEdge < 0 {
		leftEdge = 0
	}
	return leftEdge + menuButtonStyle.GetPaddingLeft()
}
