package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/cockroach-eater/lazyjira/pkg/tui/theme"
)

// RenderPanel draws a bordered panel with a title in the top border
// and an optional footer in the bottom border (like lazygit's "1 of 25").
func RenderPanel(title, content string, width, innerHeight int, focused bool) string {
	return RenderPanelWithFooter(title, "", content, width, innerHeight, focused)
}

// RenderPanelWithFooter draws a panel with title (top border) and footer (bottom-right border).
func RenderPanelWithFooter(title, footer, content string, width, innerHeight int, focused bool) string {
	th := theme.DefaultTheme()

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

	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Top border: ╭Title──────────╮
	titleLen := lipgloss.Width(styledTitle)
	topPadding := contentWidth - titleLen
	if topPadding < 0 {
		topPadding = 0
	}
	topLine := borderStyle.Render("╭") +
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

	borderVert := borderStyle.Render("│")
	var body strings.Builder
	for _, line := range lines {
		rendered := line
		lineWidth := lipgloss.Width(rendered)
		if lineWidth < contentWidth {
			rendered += strings.Repeat(" ", contentWidth-lineWidth)
		}
		body.WriteString(borderVert + rendered + borderVert + "\n")
	}

	// Bottom border: ╰──────────1 of 25─╯
	var bottomLine string
	if footer != "" {
		styledFooter := borderStyle.Render(footer)
		footerLen := lipgloss.Width(styledFooter)
		padding := contentWidth - footerLen
		if padding < 0 {
			padding = 0
		}
		bottomLine = borderStyle.Render("╰"+strings.Repeat("─", padding)) +
			styledFooter +
			borderStyle.Render("╯")
	} else {
		bottomLine = borderStyle.Render("╰" + strings.Repeat("─", contentWidth) + "╯")
	}

	return topLine + "\n" + body.String() + bottomLine
}
