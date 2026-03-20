package components

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

// ScrollInfo provides data for rendering a scrollbar in the right border.
type ScrollInfo struct {
	Total   int // total items/lines
	Visible int // visible items/lines
	Offset  int // scroll offset (first visible item index)
}

// RenderCollapsedBar draws a single-line collapsed panel: ╶─[title]───footer─╴
func RenderCollapsedBar(title, footer string, width int, focused bool) string {
	borderColor := theme.ColorNone
	if focused {
		borderColor = theme.ColorGreen
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	var styledTitle string
	if focused {
		styledTitle = theme.Default.Title.Render(title)
	} else {
		styledTitle = borderStyle.Render(title)
	}

	titleLen := lipgloss.Width(styledTitle)

	if footer == "" {
		// ╶─ title ───╴  = 2 + titleLen + padding + 1 = width
		padding := max(width-3-titleLen, 0)
		return borderStyle.Render("╶─") + styledTitle + borderStyle.Render(strings.Repeat("─", padding)+"╴")
	}

	styledFooter := borderStyle.Render(footer)
	footerLen := lipgloss.Width(styledFooter)
	// ╶─ title ── footer ─╴  = 2 + titleLen + padding + footerLen + 2 = width
	padding := max(width-4-titleLen-footerLen, 0)
	return borderStyle.Render("╶─") + styledTitle +
		borderStyle.Render(strings.Repeat("─", padding)) +
		styledFooter + borderStyle.Render("─╴")
}

// RenderPanel draws a bordered panel with title in the top border.
func RenderPanel(title, content string, width, innerHeight int, focused bool) string {
	return RenderPanelFull(title, "", content, width, innerHeight, focused, nil)
}

// RenderPanelFull draws a panel with title, footer, and optional scrollbar.
func RenderPanelFull(title, footer, content string, width, innerHeight int, focused bool, scroll *ScrollInfo) string {
	th := theme.Default

	borderColor := theme.ColorNone
	if focused {
		borderColor = theme.ColorGreen
	}

	var styledTitle string
	if focused {
		styledTitle = th.Title.Render(title)
	} else {
		styledTitle = lipgloss.NewStyle().Foreground(borderColor).Render(title)
	}

	contentWidth := max(width-2, 1)

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Top border.
	titleLen := lipgloss.Width(styledTitle)
	topPadding := max(contentWidth-titleLen-1, 0)
	topLine := borderStyle.Render("╭─") +
		styledTitle +
		borderStyle.Render(strings.Repeat("─", topPadding)+"╮")

	// Content lines.
	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}

	// Compute scrollbar — same algorithm as lazygit's gocui.
	showScroll := scroll != nil && scroll.Total > scroll.Visible && innerHeight > 0
	var thumbStart, thumbEnd int
	if showScroll {
		listSize := scroll.Total
		pageSize := scroll.Visible
		position := scroll.Offset
		scrollArea := innerHeight

		// Thumb height proportional to visible/total.
		thumbH := max(int(float64(pageSize)/float64(listSize)*float64(scrollArea)), 1)

		// Thumb position — snap to bottom at end.
		maxPos := listSize - pageSize
		switch {
		case maxPos <= 0:
			thumbStart = 0
		case position >= maxPos:
			thumbStart = scrollArea - thumbH
		default:
			thumbStart = int(math.Ceil(float64(position) / float64(maxPos) * float64(scrollArea-thumbH-1)))
		}

		thumbEnd = min(thumbStart+thumbH, scrollArea)
	}

	borderVert := borderStyle.Render("│")
	thumbChar := borderStyle.Render("▐")
	var body strings.Builder
	for i, line := range lines {
		rendered := line
		lineWidth := lipgloss.Width(rendered)
		if lineWidth < contentWidth {
			rendered += strings.Repeat(" ", contentWidth-lineWidth)
		}
		// Right border: scrollbar or normal.
		rightBorder := borderVert
		if showScroll && i >= thumbStart && i < thumbEnd {
			rightBorder = thumbChar
		}
		body.WriteString(borderVert + rendered + rightBorder + "\n")
	}

	// Bottom border.
	var bottomLine string
	if footer != "" {
		styledFooter := borderStyle.Render(footer)
		footerLen := lipgloss.Width(styledFooter)
		padding := max(contentWidth-footerLen-1, 0)
		bottomLine = borderStyle.Render("╰"+strings.Repeat("─", padding)) +
			styledFooter +
			borderStyle.Render("─╯")
	} else {
		bottomLine = borderStyle.Render("╰" + strings.Repeat("─", contentWidth) + "╯")
	}

	return topLine + "\n" + body.String() + bottomLine
}
