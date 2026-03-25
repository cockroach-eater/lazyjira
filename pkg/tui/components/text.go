package components

import "github.com/charmbracelet/lipgloss"

// TruncateEnd truncates s to fit within maxWidth display columns,
// appending "…" if needed. Respects multi-byte UTF-8 characters.
func TruncateEnd(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate)+1 <= maxWidth { // +1 for "…"
			return candidate + "…"
		}
	}
	return "…"
}

// TruncateMiddle truncates keeping start and end visible: "abcdef...xyz"
// Uses display width (not byte count) so multi-byte chars like → are handled correctly.
func TruncateMiddle(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth < 5 {
		runes := []rune(s)
		if len(runes) > maxWidth {
			return string(runes[:maxWidth])
		}
		return s
	}
	runes := []rune(s)
	ellipsis := "..."
	ellipsisW := 3
	budget := maxWidth - ellipsisW
	startBudget := (budget + 1) / 2 // start gets the extra column on odd budget
	endBudget := budget - startBudget

	// Build start: runes from the beginning.
	var start []rune
	w := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		if w+rw > startBudget {
			break
		}
		start = append(start, r)
		w += rw
	}

	// Build end: runes from the end.
	var end []rune
	w = 0
	for i := len(runes) - 1; i >= 0; i-- {
		rw := lipgloss.Width(string(runes[i]))
		if w+rw > endBudget {
			break
		}
		end = append([]rune{runes[i]}, end...)
		w += rw
	}

	return string(start) + ellipsis + string(end)
}

// Truncate truncates s to max bytes, appending "…" if needed.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) > maxLen {
		return s[:maxLen-1] + "…"
	}
	return s
}

// PanelDimensions computes usable content width and inner height from total panel dimensions.
func PanelDimensions(width, height int) (contentWidth, innerHeight int) {
	return max(width-2, 10), max(height-2, 1)
}
