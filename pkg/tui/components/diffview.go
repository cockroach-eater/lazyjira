package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DiffConfirmedMsg is sent when user confirms the diff (Enter).
type DiffConfirmedMsg struct {
	Content string // the new (edited) content
}

// DiffCancelledMsg is sent when user rejects the diff (Esc/q).
type DiffCancelledMsg struct{}

// DiffView shows a unified diff in a scrollable modal with confirm/reject.
type DiffView struct {
	title   string
	lines   []string // pre-rendered diff lines with ANSI colors
	content string   // the new content to return on confirm
	offset  int
	visible bool
	width   int
	height  int
}

func NewDiffView() DiffView {
	return DiffView{}
}

// Show displays the diff view with the given title, diff lines, and new content.
func (d *DiffView) Show(title string, oldText, newText string) {
	d.title = title
	d.lines = computeUnifiedDiff(oldText, newText)
	d.content = newText
	d.offset = 0
	d.visible = true
}

func (d *DiffView) Hide()           { d.visible = false }
func (d *DiffView) IsVisible() bool { return d.visible }
func (d *DiffView) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *DiffView) Update(msg tea.Msg) (DiffView, tea.Cmd) {
	if !d.visible {
		return *d, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			d.visible = false
			content := d.content
			return *d, func() tea.Msg { return DiffConfirmedMsg{Content: content} }
		case "esc", "q":
			d.visible = false
			return *d, func() tea.Msg { return DiffCancelledMsg{} }
		case "j", "down":
			d.offset++
		case "k", "up":
			if d.offset > 0 {
				d.offset--
			}
		case "ctrl+d":
			d.offset += d.visibleH() / 2
		case "ctrl+u":
			d.offset -= d.visibleH() / 2
			if d.offset < 0 {
				d.offset = 0
			}
		}
	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelDown {
			d.offset++
		} else if msg.Button == tea.MouseButtonWheelUp && d.offset > 0 {
			d.offset--
		}
	}
	return *d, nil
}

func (d *DiffView) visibleH() int {
	return max(d.height-4, 3) // borders + title + footer
}

func (d *DiffView) View() string {
	if !d.visible || len(d.lines) == 0 {
		return ""
	}

	// Size: 80% width, fit content height.
	contentW := min(d.width*8/10, d.width-4)
	if contentW < 30 {
		contentW = min(d.width-2, 30)
	}
	visibleH := d.visibleH()

	// Clamp scroll offset.
	maxOffset := max(len(d.lines)-visibleH, 0)
	if d.offset > maxOffset {
		d.offset = maxOffset
	}

	// Slice visible lines.
	end := min(d.offset+visibleH, len(d.lines))
	visible := d.lines[d.offset:end]

	// Truncate lines to fit width.
	innerW := contentW - 2 // borders
	var displayLines []string
	for _, line := range visible {
		if lipgloss.Width(line) > innerW {
			displayLines = append(displayLines, TruncateEnd(line, innerW))
		} else {
			displayLines = append(displayLines, line)
		}
	}

	body := strings.Join(displayLines, "\n")
	footer := fmt.Sprintf("%d lines", len(d.lines))

	return RenderPanelFull(d.title, footer, body, contentW, visibleH, true,
		&ScrollInfo{Total: len(d.lines), Visible: visibleH, Offset: d.offset})
}

// Intercept handles a message if the diff view is visible. Implements Overlay.
func (d *DiffView) Intercept(msg tea.Msg) (tea.Cmd, bool) {
	if !d.visible {
		return nil, false
	}
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		updated, cmd := d.Update(msg)
		*d = updated
		return cmd, true
	}
	return nil, false
}

// Render draws the diff view centered on bg. Implements Overlay.
func (d *DiffView) Render(bg string, w, h int) string {
	if !d.visible {
		return bg
	}
	return centerOverlay(bg, d.View(), w, h)
}

// computeUnifiedDiff generates a simple line-based unified diff.
// Context lines are plain, additions green with +, removals red with -.
func computeUnifiedDiff(oldText, newText string) []string {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	ctxStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Simple LCS-based diff.
	lcs := computeLCS(oldLines, newLines)

	var result []string
	oi, ni, li := 0, 0, 0
	for li < len(lcs) {
		// Lines removed (in old but not in LCS at this position).
		for oi < len(oldLines) && (li >= len(lcs) || oldLines[oi] != lcs[li]) {
			result = append(result, delStyle.Render("- "+oldLines[oi]))
			oi++
		}
		// Lines added (in new but not in LCS at this position).
		for ni < len(newLines) && (li >= len(lcs) || newLines[ni] != lcs[li]) {
			result = append(result, addStyle.Render("+ "+newLines[ni]))
			ni++
		}
		// Context line (in both).
		if li < len(lcs) {
			result = append(result, ctxStyle.Render("  "+lcs[li]))
			oi++
			ni++
			li++
		}
	}
	// Remaining removals.
	for oi < len(oldLines) {
		result = append(result, delStyle.Render("- "+oldLines[oi]))
		oi++
	}
	// Remaining additions.
	for ni < len(newLines) {
		result = append(result, addStyle.Render("+ "+newLines[ni]))
		ni++
	}

	return result
}

// computeLCS returns the Longest Common Subsequence of two string slices.
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	// Build DP table.
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	// Backtrack to find LCS.
	lcs := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		switch {
		case a[i-1] == b[j-1]:
			lcs = append(lcs, a[i-1])
			i--
			j--
		case dp[i-1][j] > dp[i][j-1]:
			i--
		default:
			j--
		}
	}
	// Reverse.
	for l, r := 0, len(lcs)-1; l < r; l, r = l+1, r-1 {
		lcs[l], lcs[r] = lcs[r], lcs[l]
	}
	return lcs
}
