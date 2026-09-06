package menu

import (
	imgcolor "image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/awnumar/memguard"
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

// contentPaddingX is the side margin around the header.
// Body-content column = pswHeaderWidth + 2*contentPaddingX.
// Sized so picker help (~71 cells) fits without wrapping.
const contentPaddingX = 8

// contentColumnWidth caps the layout column to the terminal width so narrow
// terminals don't overflow.
func contentColumnWidth(termWidth int) int {
	w := pswHeaderWidth + 2*contentPaddingX
	if termWidth > 0 && termWidth < w {
		return termWidth
	}
	return w
}

// contentColumnLeft is the terminal column where the body-content column starts.
func contentColumnLeft(termWidth int) int {
	return max((termWidth-contentColumnWidth(termWidth))/2, 0)
}

var defaultHeaderColor = lipgloss.Color("6")

// menuEntry is one button: its label and the action it starts. Its index in
// menuEntries is the [N] hotkey (1-based).
type menuEntry struct {
	name string
	new  func(*memguard.Enclave) Action
}

// menuEntries lays out the actions in a column-major 3x2 grid:
//
//	[1] get      [3] change    [5] settings
//	[2] add      [4] remove    [6] rollback
var menuEntries = []menuEntry{
	{name: "get", new: func(p *memguard.Enclave) Action { return NewGetAction(p) }},
	{name: "add", new: func(p *memguard.Enclave) Action { return NewAddAction(p) }},
	{name: "change", new: func(p *memguard.Enclave) Action { return NewChangeAction(p) }},
	{name: "remove", new: func(p *memguard.Enclave) Action { return NewRemoveAction(p) }},
	{name: "settings", new: func(*memguard.Enclave) Action { return NewSettingsAction() }},
	{name: "rollback", new: func(p *memguard.Enclave) Action { return NewRollbackAction(p) }},
}

const menuGridCols = 3
const menuGridRows = 2

// No PaddingRight on button styles: trailing pad on the rightmost button
// would push the visible right edge 2 cells short of the content column.
// Gaps in renderButtons separate buttons instead.
var (
	menuButtonStyle      = lipgloss.NewStyle()
	menuButtonStyleFaint = lipgloss.NewStyle().Faint(true)
	menuSelectStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	menuHelpStyle        = lipgloss.NewStyle().Faint(true)
	menuErrStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// 2-cell indicator slot at the left of every button. Both prefixes are the
// same width so selection doesn't shift cells.
const (
	buttonPrefixSelected = "> "
	buttonPrefixBlank    = "  "
)

// layoutRowSpaceBetween places buttons with equal gaps. Leftmost hugs the
// column's left edge, rightmost its right edge.
func layoutRowSpaceBetween(rendered []string, widths []int, btnIdxs []int, columnWidth int) string {
	n := len(btnIdxs)
	if n == 0 {
		return ""
	}
	sumW := 0
	for _, i := range btnIdxs {
		sumW += widths[i]
	}
	totalGap := max(columnWidth-sumW, 0)
	gaps := make([]int, n-1)
	if n > 1 {
		base := totalGap / (n - 1)
		rem := totalGap % (n - 1)
		for i := range gaps {
			gaps[i] = base
			if i < rem {
				gaps[i]++
			}
		}
	}
	var sb strings.Builder
	for i, btn := range btnIdxs {
		sb.WriteString(rendered[btn])
		if i < len(gaps) {
			sb.WriteString(strings.Repeat(" ", gaps[i]))
		}
	}
	return sb.String()
}

const selectActionHelp = "←→/hjkl/tab nav · 1-6 jump · enter/space run · esc/q quit"

// actionFrameHeight = row count of header + spacers + button rows.
// Computed from renderActionFrame so it tracks layout changes.
var actionFrameHeight = lipgloss.Height(renderActionFrame())

func renderActionFrame() string {
	var b strings.Builder
	b.WriteString(renderHeader(defaultHeaderColor))
	b.WriteString("\n\n")
	b.WriteString(menuButtonStyle.Render(buttonPrefixBlank + "[1] x"))
	b.WriteString("\n")
	b.WriteString(menuButtonStyle.Render(buttonPrefixBlank + "[2] y"))
	b.WriteString("\n\n")
	return b.String()
}

func renderHeader(c imgcolor.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render(pswHeader)
}

// wrapToContent indents and wraps content to the body-content column.
func wrapToContent(termWidth int, content string) string {
	if termWidth == 0 {
		return content
	}
	return lipgloss.NewStyle().
		Width(contentColumnWidth(termWidth)).
		MarginLeft(contentColumnLeft(termWidth)).
		Render(content)
}

// headerColumnLeft is the column where the header starts.
// Password-phase content aligns flush with the header, not the wider body
// column.
func headerColumnLeft(termWidth int) int {
	if termWidth <= pswHeaderWidth {
		return 0
	}
	return (termWidth - pswHeaderWidth) / 2
}

// indentToHeader indents content to the header's left edge (no wrap).
func indentToHeader(termWidth int, content string) string {
	if termWidth == 0 {
		return content
	}
	return lipgloss.NewStyle().MarginLeft(headerColumnLeft(termWidth)).Render(content)
}

// wrapToHeader wraps and indents content to the header column.
func wrapToHeader(termWidth int, content string) string {
	if termWidth == 0 {
		return content
	}
	if termWidth <= pswHeaderWidth {
		return lipgloss.NewStyle().Width(termWidth).Render(content)
	}
	return lipgloss.NewStyle().
		Width(pswHeaderWidth).
		MarginLeft(headerColumnLeft(termWidth)).
		Render(content)
}

// centerHorizontally centers content in width. Returns content unchanged
// at width 0.
func centerHorizontally(width int, content string) string {
	if width == 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

// indentToFooter centers footer content within the body-content column.
func indentToFooter(termWidth int, content string) string {
	if termWidth == 0 {
		return content
	}
	contentWidth := lipgloss.Width(content)
	if termWidth <= contentWidth {
		return content
	}
	column := contentColumnWidth(termWidth)
	extra := max((column-contentWidth)/2, 0)
	return lipgloss.NewStyle().MarginLeft(contentColumnLeft(termWidth) + extra).Render(content)
}

// alignBlock indents content so its first non-space char lands at targetCol.
// Backs out content's own leading padding (bubbles/list items have 2; bare
// prompts have 0). Trims trailing spaces (bubbles/list pads to full width).
func alignBlock(termWidth, targetCol int, content string) (string, int) {
	trimmed := trimTrailingPerLine(content)
	if termWidth == 0 || termWidth <= targetCol {
		return trimmed, 0
	}
	indent := max(targetCol-minLeadingSpaces(trimmed), 0)
	return lipgloss.NewStyle().MarginLeft(indent).Render(trimmed), indent
}

func trimTrailingPerLine(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// minLeadingSpaces returns the smallest leading-space count across non-blank
// lines.
func minLeadingSpaces(content string) int {
	best := -1
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if best == -1 || n < best {
			best = n
		}
	}
	return max(best, 0)
}
