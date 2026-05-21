package menu

import (
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
	left := (termWidth - contentColumnWidth(termWidth)) / 2
	if left < 0 {
		return 0
	}
	return left
}

var defaultHeaderColor = lipgloss.Color("6")

// menuActions is the ordered list of buttons. Index maps directly to:
//   - the [N] hotkey (1-based)
//   - the cell in menuCells (same index)
//   - the dispatch name in newAction
//
// Keep the three in sync when adding entries.
var menuActions = []string{"get", "add", "change", "remove", "settings", "rollback"}

// menuCells lays out the actions in a column-major 3x2 grid:
//
//	[1] get      [3] change    [5] settings
//	[2] add      [4] remove    [6] rollback
var menuCells = [...]struct{ row, col int }{
	{0, 0}, // [1] get
	{1, 0}, // [2] add
	{0, 1}, // [3] change
	{1, 1}, // [4] remove
	{0, 2}, // [5] settings
	{1, 2}, // [6] rollback
}

const menuGridCols = 3
const menuGridRows = 2

// menuCellLookup[row][col] = menuActions index, or -1 if empty.
// Inverse of menuCells, built at init.
var menuCellLookup = func() [menuGridRows][menuGridCols]int {
	var m [menuGridRows][menuGridCols]int
	for r := 0; r < menuGridRows; r++ {
		for c := 0; c < menuGridCols; c++ {
			m[r][c] = -1
		}
	}
	for i, cell := range menuCells {
		m[cell.row][cell.col] = i
	}
	return m
}()

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
	buttonPrefixWidth    = 2
)

const footerHelp = "←→/hjkl/tab nav · 1-6 jump · enter/space run · esc/q quit"

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

// indentToContent indents content to contentColumnLeft.
// No wrap; use for short single-line content.
func indentToContent(termWidth int, content string) string {
	if termWidth == 0 {
		return content
	}
	return lipgloss.NewStyle().MarginLeft(contentColumnLeft(termWidth)).Render(content)
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
	extra := (column - contentWidth) / 2
	if extra < 0 {
		extra = 0
	}
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

// minLeadingSpaces returns the smallest leading-space count across non-blank
// lines.
func minLeadingSpaces(content string) int {
	best := -1
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
		if best == -1 || n < best {
			best = n
		}
	}
	if best < 0 {
		return 0
	}
	return best
}
