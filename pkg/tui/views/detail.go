package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cockroach-eater/lazyjira/pkg/jira"
	"github.com/cockroach-eater/lazyjira/pkg/tui/components"
	"github.com/cockroach-eater/lazyjira/pkg/tui/theme"
)

type DetailTab int

const (
	TabDetails  DetailTab = iota
	TabSubtasks
	TabComments
	TabLinks
	TabInfo
	TabHistory
	tabCount = 6
)

// MainMode controls what the right panel displays.
type MainMode int

const (
	ModeIssue   MainMode = iota
	ModeSplash
	ModeProject
)

// SplashInfo holds data for the splash/status screen.
type SplashInfo struct {
	AuthMethod string // "API Token" or "OAuth" or "env vars"
	Host       string
	Email      string
	Project    string
}

type DetailView struct {
	issue      *jira.Issue
	project    *jira.Project
	splash     SplashInfo
	mode       MainMode
	activeTab  DetailTab
	scrollY    int
	listCursor int // cursor for list-based tabs (Sub, Cmt, Lnk, Hist)
	width      int
	height     int
	focused    bool
	theme      *theme.Theme
}

func NewDetailView() *DetailView {
	return &DetailView{theme: theme.DefaultTheme(), mode: ModeIssue}
}

func (d *DetailView) SetIssue(issue *jira.Issue) {
	prevKey := ""
	if d.issue != nil {
		prevKey = d.issue.Key
	}
	d.issue = issue
	d.mode = ModeIssue
	// Only reset tab/scroll when switching to a different issue.
	if issue == nil || issue.Key != prevKey {
		d.scrollY = 0
		d.activeTab = TabDetails
	}
}

// UpdateIssueData stores issue data without changing mode (for background updates).
func (d *DetailView) UpdateIssueData(issue *jira.Issue) {
	prevKey := ""
	if d.issue != nil {
		prevKey = d.issue.Key
	}
	d.issue = issue
	if issue != nil && issue.Key != prevKey {
		d.scrollY = 0
		d.activeTab = TabDetails
	}
}

func (d *DetailView) SetProject(project *jira.Project) {
	d.project = project
	d.mode = ModeProject
	d.scrollY = 0
}

func (d *DetailView) SetSplash(info SplashInfo) {
	d.splash = info
	d.mode = ModeSplash
	d.scrollY = 0
}

func (d *DetailView) SetSize(w, h int)       { d.width = w; d.height = h }
func (d *DetailView) SetFocused(focused bool) { d.focused = focused }
func (d *DetailView) Init() tea.Cmd           { return nil }

func (d *DetailView) NextTab() {
	vt := d.visibleTabs()
	for i, t := range vt {
		if t == d.activeTab {
			d.activeTab = vt[(i+1)%len(vt)]
			d.scrollY = 0
			d.listCursor = 0
			return
		}
	}
	if len(vt) > 0 {
		d.activeTab = vt[0]
		d.scrollY = 0
	}
}

func (d *DetailView) PrevTab() {
	vt := d.visibleTabs()
	for i, t := range vt {
		if t == d.activeTab {
			d.activeTab = vt[(i+len(vt)-1)%len(vt)]
			d.scrollY = 0
			d.listCursor = 0
			return
		}
	}
	if len(vt) > 0 {
		d.activeTab = vt[0]
		d.scrollY = 0
	}
}

func (d *DetailView) visibleTabs() []DetailTab {
	labels := d.tabLabels()
	tabs := make([]DetailTab, len(labels))
	for i, l := range labels {
		tabs[i] = l.tab
	}
	return tabs
}

// ClickTab switches tab based on x position in the title bar.
func (d *DetailView) ClickTab(x int) {
	if d.issue == nil {
		return
	}
	// Reconstruct tab labels and their positions in the title.
	type tabPos struct {
		tab   DetailTab
		start int
		end   int
	}
	labels := d.tabLabels()
	prefix := fmt.Sprintf("[0] %s", d.issue.Key)
	sep := " - "
	pos := len(prefix) + len(sep)

	var positions []tabPos
	for i, tl := range labels {
		end := pos + len(tl.label)
		positions = append(positions, tabPos{tab: tl.tab, start: pos, end: end})
		if i < len(labels)-1 {
			pos = end + len(sep)
		}
	}

	for _, p := range positions {
		if x >= p.start && x < p.end {
			d.activeTab = p.tab
			d.scrollY = 0
			return
		}
	}
}

func (d *DetailView) ScrollBy(delta int) {
	d.scrollY += delta
	if d.scrollY < 0 {
		d.scrollY = 0
	}
}

// listTabItemCount returns the number of items for list-based tabs, 0 for text tabs.
func (d *DetailView) listTabItemCount() int {
	if d.issue == nil {
		return 0
	}
	switch d.activeTab {
	case TabSubtasks:
		return len(d.issue.Subtasks)
	case TabComments:
		return len(d.issue.Comments)
	case TabLinks:
		return len(d.issue.IssueLinks)
	case TabHistory:
		return len(d.issue.Changelog)
	case TabInfo:
		return d.infoFieldCount()
	}
	return 0
}

func (d *DetailView) infoFieldCount() int {
	if d.issue == nil {
		return 0
	}
	count := 6 // Status, Priority, Assignee, Reporter, Type, Sprint
	if len(d.issue.Labels) > 0 {
		count++
	}
	if len(d.issue.Components) > 0 {
		count++
	}
	return count
}

func (d *DetailView) isListTab() bool {
	return d.listTabItemCount() > 0
}

func (d *DetailView) Update(msg tea.Msg) (*DetailView, tea.Cmd) {
	if !d.focused {
		return d, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if count := d.listTabItemCount(); count > 0 {
				if d.listCursor < count-1 {
					d.listCursor++
				}
			} else {
				d.scrollY++
			}
		case "k", "up":
			if d.listTabItemCount() > 0 {
				if d.listCursor > 0 {
					d.listCursor--
				}
			} else if d.scrollY > 0 {
				d.scrollY--
			}
		case "tab":
			d.activeTab = (d.activeTab + 1) % tabCount
			d.scrollY = 0
			d.listCursor = 0
		case "i":
			d.activeTab = TabInfo
			d.scrollY = 0
			d.listCursor = 0
		case "ctrl+d":
			if count := d.listTabItemCount(); count > 0 {
				d.listCursor += d.visibleRows() / 2
				if d.listCursor >= count {
					d.listCursor = count - 1
				}
			} else {
				d.scrollY += d.visibleRows() / 2
			}
		case "ctrl+u":
			if d.listTabItemCount() > 0 {
				d.listCursor -= d.visibleRows() / 2
				if d.listCursor < 0 {
					d.listCursor = 0
				}
			} else {
				d.scrollY -= d.visibleRows() / 2
				if d.scrollY < 0 {
					d.scrollY = 0
				}
			}
		}
	}
	return d, nil
}

func (d *DetailView) visibleRows() int {
	// Total height = innerHeight + 2 (borders). Tabs are in the title now.
	rows := d.height - 2
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (d *DetailView) View() string {
	contentWidth := d.width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}
	innerH := d.height - 2
	if innerH < 1 {
		innerH = 1
	}

	// Splash mode.
	if d.mode == ModeSplash {
		return d.renderSplash(contentWidth, innerH)
	}

	// Project mode.
	if d.mode == ModeProject && d.project != nil {
		return d.renderProjectView(contentWidth, innerH)
	}

	visible := d.visibleRows()

	// Issue mode.
	if d.issue == nil {
		title := "[0] Detail"
		placeholder := lipgloss.NewStyle().Foreground(theme.ColorGray).Render("Select an issue to view details")
		return components.RenderPanel(title, placeholder, d.width, innerH, d.focused)
	}

	// Build title: [0] KEY - Tab - Tab - Tab
	title := d.buildTitle(contentWidth)

	// Content: list tabs return blocks per item, text tabs return flat lines.
	var contentLines []string

	if count := d.listTabItemCount(); count > 0 {
		// List tab — render blocks, highlight selected.
		var blocks [][]string
		switch d.activeTab {
		case TabSubtasks:
			blocks = d.renderSubtaskBlocks(contentWidth)
		case TabComments:
			blocks = d.renderCommentBlocks(contentWidth)
		case TabLinks:
			blocks = d.renderLinkBlocks(contentWidth)
		case TabHistory:
			blocks = d.renderHistoryBlocks(contentWidth)
		case TabInfo:
			blocks = d.renderInfoBlocks(contentWidth)
		}

		// Clamp cursor.
		if d.listCursor >= len(blocks) {
			d.listCursor = len(blocks) - 1
		}
		if d.listCursor < 0 {
			d.listCursor = 0
		}

		// Flatten blocks with highlight on selected.
		selectedBg := lipgloss.NewStyle().Background(lipgloss.Color("0")) // subtle highlight
		sep := strings.Repeat("─", contentWidth-2)
		for i, block := range blocks {
			for _, line := range block {
				if i == d.listCursor {
					// Highlight: render with subtle bg.
					contentLines = append(contentLines, selectedBg.Width(contentWidth).Render(line))
				} else {
					contentLines = append(contentLines, line)
				}
			}
			if i < len(blocks)-1 {
				contentLines = append(contentLines, " "+sep)
			}
		}

		// Auto-scroll: find the line range of cursor block and ensure visible.
		lineStart := 0
		for i := 0; i < d.listCursor && i < len(blocks); i++ {
			lineStart += len(blocks[i]) + 1 // +1 for separator
		}
		if lineStart < d.scrollY {
			d.scrollY = lineStart
		}
		blockEnd := lineStart + len(blocks[d.listCursor])
		if blockEnd > d.scrollY+visible {
			d.scrollY = blockEnd - visible
		}
	} else {
		switch d.activeTab {
		case TabDetails:
			contentLines = d.renderDescription(contentWidth)
		default:
			contentLines = []string{" No content."}
		}
	}

	// Apply scroll for text tabs.
	if d.scrollY > len(contentLines) {
		d.scrollY = len(contentLines)
	}
	if d.scrollY < 0 {
		d.scrollY = 0
	}
	scrolled := contentLines
	if d.scrollY < len(scrolled) {
		scrolled = scrolled[d.scrollY:]
	} else {
		scrolled = nil
	}
	if len(scrolled) > visible {
		scrolled = scrolled[:visible]
	}

	body := strings.Join(scrolled, "\n")

	footer := ""
	if count := d.listTabItemCount(); count > 0 {
		footer = fmt.Sprintf("%d of %d", d.listCursor+1, count)
	}
	return components.RenderPanelWithFooter(title, footer, body, d.width, innerH, d.focused)
}

type tabLabel struct {
	tab   DetailTab
	label string
}

func (d *DetailView) tabLabels() []tabLabel {
	var tabs []tabLabel
	tabs = append(tabs, tabLabel{TabDetails, "Body"})
	if d.issue != nil {
		if len(d.issue.Subtasks) > 0 {
			tabs = append(tabs, tabLabel{TabSubtasks, "Sub"})
		}
		if len(d.issue.Comments) > 0 {
			tabs = append(tabs, tabLabel{TabComments, "Cmt"})
		}
		if len(d.issue.IssueLinks) > 0 {
			tabs = append(tabs, tabLabel{TabLinks, "Lnk"})
		}
	}
	tabs = append(tabs, tabLabel{TabInfo, "Info"})
	if d.issue != nil && len(d.issue.Changelog) > 0 {
		tabs = append(tabs, tabLabel{TabHistory, "Hist"})
	}
	return tabs
}

func (d *DetailView) buildTitle(maxWidth int) string {
	tabs := d.tabLabels()

	activeStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	sepStyle := lipgloss.NewStyle().Foreground(theme.ColorGray)

	prefix := fmt.Sprintf("[0] %s", d.issue.Key)

	var tabParts []string
	for _, t := range tabs {
		if t.tab == d.activeTab {
			tabParts = append(tabParts, activeStyle.Render(t.label))
		} else {
			tabParts = append(tabParts, inactiveStyle.Render(t.label))
		}
	}

	sep := sepStyle.Render(" - ")
	return prefix + sep + strings.Join(tabParts, sep)
}

func (d *DetailView) renderDescription(width int) []string {
	valStyle := d.theme.ValueStyle
	var lines []string

	desc := d.issue.Description
	if desc == "" {
		desc = "(no description)"
	}
	for _, line := range wrapText(desc, width-2) {
		lines = append(lines, " "+colorMentions(valStyle.Render(line)))
	}
	return lines
}

// colorMentions replaces \x00MENTION:@Name\x00 markers with colored author names.
func colorMentions(s string) string {
	const prefix = "\x00MENTION:"
	const suffix = "\x00"
	result := s
	for {
		start := strings.Index(result, prefix)
		if start == -1 {
			break
		}
		rest := result[start+len(prefix):]
		end := strings.Index(rest, suffix)
		if end == -1 {
			break
		}
		name := rest[:end]
		colored := theme.AuthorRender(name)
		result = result[:start] + colored + rest[end+len(suffix):]
	}
	return result
}

func (d *DetailView) renderSubtaskBlocks(width int) [][]string {
	var blocks [][]string
	for _, sub := range d.issue.Subtasks {
		emoji := statusEmoji(sub.Status)
		blocks = append(blocks, []string{fmt.Sprintf(" %s %s: %s", emoji, sub.Key, sub.Summary)})
	}
	return blocks
}

func (d *DetailView) renderInfoBlocks(width int) [][]string {
	issue := d.issue
	valStyle := d.theme.ValueStyle
	var blocks [][]string

	statusName := "Unknown"
	if issue.Status != nil {
		statusName = theme.StatusColor(issue.Status.CategoryKey).Render(issue.Status.Name)
	}
	blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Status:", statusName)})

	priorityName := "None"
	if issue.Priority != nil {
		priorityName = d.priorityStyled(issue.Priority.Name)
	}
	blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Priority:", priorityName)})

	assignee := "Unassigned"
	if issue.Assignee != nil {
		assignee = theme.AuthorRender(issue.Assignee.DisplayName)
	}
	blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Assignee:", assignee)})

	reporter := "Unknown"
	if issue.Reporter != nil {
		reporter = theme.AuthorRender(issue.Reporter.DisplayName)
	}
	blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Reporter:", reporter)})

	typeName := "Unknown"
	if issue.IssueType != nil {
		typeName = issue.IssueType.Name
	}
	blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Type:", valStyle.Render(typeName))})

	sprintName := "None"
	if issue.Sprint != nil {
		sprintName = issue.Sprint.Name
	}
	blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Sprint:", valStyle.Render(sprintName))})

	if len(issue.Labels) > 0 {
		blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Labels:", valStyle.Render(strings.Join(issue.Labels, ", ")))})
	}

	if len(issue.Components) > 0 {
		var names []string
		for _, c := range issue.Components {
			names = append(names, c.Name)
		}
		blocks = append(blocks, []string{fmt.Sprintf(" %-11s %s", "Components:", valStyle.Render(strings.Join(names, ", ")))})
	}

	return blocks
}

// renderEntry renders a single author+time header + content block + separator.

func (d *DetailView) renderHistoryBlocks(width int) [][]string {
	gray := lipgloss.NewStyle().Foreground(theme.ColorGray)
	var blocks [][]string

	// Reverse order: newest first.
	for i := len(d.issue.Changelog) - 1; i >= 0; i-- {
		entry := d.issue.Changelog[i]
		author := "Unknown"
		if entry.Author != nil {
			author = entry.Author.DisplayName
		}

		var block []string
		block = append(block, " "+theme.AuthorRender(author)+" "+gray.Render(timeAgo(entry.Created)))

		for _, item := range entry.Items {
			from := cleanWikiMarkup(item.FromString)
			to := cleanWikiMarkup(item.ToString)
			if from == "" {
				from = "none"
			}
			if to == "" {
				to = "none"
			}

			field := strings.ToLower(item.Field)

			if field == "description" || field == "comment" || field == "environment" {
				block = append(block, "   "+gray.Render(item.Field)+gray.Render(":"))
				block = append(block, renderDiff(from, to, width-4)...)
				continue
			}

			if field == "assignee" || field == "reviewer" || field == "reporter" {
				if from != "none" {
					from = theme.AuthorRender(from)
				}
				if to != "none" {
					to = theme.AuthorRender(to)
				}
			}
			changeLine := fmt.Sprintf("   %s: %s → %s", gray.Render(item.Field), from, to)
			for _, wl := range wrapText(changeLine, width-2) {
				block = append(block, wl)
			}
		}

		blocks = append(blocks, block)
	}
	return blocks
}

func (d *DetailView) renderCommentBlocks(width int) [][]string {
	gray := lipgloss.NewStyle().Foreground(theme.ColorGray)
	valStyle := d.theme.ValueStyle
	var blocks [][]string
	for _, c := range d.issue.Comments {
		author := "Unknown"
		if c.Author != nil {
			author = c.Author.DisplayName
		}
		var block []string
		block = append(block, " "+theme.AuthorRender(author)+" "+gray.Render(timeAgo(c.Created)))
		for _, wl := range wrapText(c.Body, width-2) {
			block = append(block, " "+colorMentions(valStyle.Render(wl)))
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func (d *DetailView) renderLinkBlocks(width int) [][]string {
	keyStyle := d.theme.KeyStyle
	valStyle := d.theme.ValueStyle
	var blocks [][]string
	for _, link := range d.issue.IssueLinks {
		if link.Type == nil {
			continue
		}
		var block []string
		if link.OutwardIssue != nil {
			block = append(block, " "+
				keyStyle.Render(link.Type.Outward)+" "+
				valStyle.Render(fmt.Sprintf("%s: %s", link.OutwardIssue.Key, link.OutwardIssue.Summary)))
		}
		if link.InwardIssue != nil {
			block = append(block, " "+
				keyStyle.Render(link.Type.Inward)+" "+
				valStyle.Render(fmt.Sprintf("%s: %s", link.InwardIssue.Key, link.InwardIssue.Summary)))
		}
		if len(block) > 0 {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func (d *DetailView) renderSplash(contentWidth, innerH int) string {
	green := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	gray := lipgloss.NewStyle().Foreground(theme.ColorGray)
	label := lipgloss.NewStyle().Foreground(theme.ColorGreen)
	val := lipgloss.NewStyle()

	ascii := `   _                  _ _
  | |                (_|_)
  | | __ _ _____   _  _ _ _ __ __ _
  | |/ _` + "`" + ` |_  / | | || | | '__/ _` + "`" + ` |
  | | (_| |/ /| |_| || | | | | (_| |
  |_|\__,_/___|\__, || |_|_|  \__,_|
                __/ |/ |
               |___/__/`

	var lines []string
	for _, l := range strings.Split(ascii, "\n") {
		lines = append(lines, green.Render(l))
	}
	lines = append(lines, "")
	lines = append(lines, gray.Render("  lazyjira — Jira TUI"))
	lines = append(lines, gray.Render("  (c) 2026 Andrey Kondratev"))

	// Connection info.
	s := d.splash
	lines = append(lines, "")
	lines = append(lines, "  "+strings.Repeat("─", 30))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %s  %s", label.Render("Auth:"), val.Render(s.AuthMethod)))
	lines = append(lines, fmt.Sprintf("  %s  %s", label.Render("Host:"), val.Render(s.Host)))
	lines = append(lines, fmt.Sprintf("  %s %s", label.Render("Email:"), val.Render(s.Email)))
	if s.Project != "" {
		lines = append(lines, fmt.Sprintf("  %s  %s", label.Render("Project:"), val.Render(s.Project)))
	}

	content := strings.Join(lines, "\n")
	return components.RenderPanel("[0] lazyjira", content, d.width, innerH, d.focused)
}

func (d *DetailView) renderProjectView(contentWidth, innerH int) string {
	p := d.project
	valStyle := d.theme.ValueStyle
	gray := lipgloss.NewStyle().Foreground(theme.ColorGray)

	var lines []string
	lines = append(lines, fmt.Sprintf(" %-11s %s", "Key:", valStyle.Render(p.Key)))
	lines = append(lines, fmt.Sprintf(" %-11s %s", "Name:", valStyle.Render(p.Name)))
	if p.Lead != nil {
		lines = append(lines, fmt.Sprintf(" %-11s %s", "Lead:", theme.AuthorRender(p.Lead.DisplayName)))
	}
	if p.ID != "" {
		lines = append(lines, fmt.Sprintf(" %-11s %s", "ID:", gray.Render(p.ID)))
	}

	content := strings.Join(lines, "\n")
	title := fmt.Sprintf("[0] Project: %s", p.Name)
	title = truncateRunes(title, contentWidth-2)
	return components.RenderPanel(title, content, d.width, innerH, d.focused)
}

func (d *DetailView) priorityStyled(name string) string {
	switch strings.ToLower(name) {
	case "highest", "high", "critical", "blocker":
		return d.theme.PriorityHigh.Render(name)
	case "medium":
		return d.theme.PriorityMedium.Render(name)
	default:
		return d.theme.PriorityLow.Render(name)
	}
}

// renderDiff shows removed lines in red and added lines in green.
func renderDiff(from, to string, maxWidth int) []string {
	redStyle := lipgloss.NewStyle().Foreground(theme.ColorRed)
	greenStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen)

	fromLines := strings.Split(strings.TrimSpace(from), "\n")
	toLines := strings.Split(strings.TrimSpace(to), "\n")

	// Build sets for simple diff.
	fromSet := make(map[string]bool)
	toSet := make(map[string]bool)
	for _, l := range fromLines {
		fromSet[strings.TrimSpace(l)] = true
	}
	for _, l := range toLines {
		toSet[strings.TrimSpace(l)] = true
	}

	var lines []string

	// Show removed lines (in from but not in to).
	for _, l := range fromLines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || trimmed == "none" {
			continue
		}
		if !toSet[trimmed] {
			for _, wl := range wrapText("- "+trimmed, maxWidth) {
				lines = append(lines, "    "+redStyle.Render(wl))
			}
		}
	}

	// Show added lines (in to but not in from).
	for _, l := range toLines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || trimmed == "none" {
			continue
		}
		if !fromSet[trimmed] {
			for _, wl := range wrapText("+ "+trimmed, maxWidth) {
				lines = append(lines, "    "+greenStyle.Render(wl))
			}
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "    "+lipgloss.NewStyle().Foreground(theme.ColorGray).Render("(content changed)"))
	}

	return lines
}

// ExtractURLs returns all URLs found in the issue (description, comments, links).
func ExtractURLs(issue *jira.Issue, host string) []string {
	if issue == nil {
		return nil
	}
	seen := make(map[string]bool)
	var urls []string
	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}

	// Skip the issue's own URL — it's already open.
	selfURL := host + "/browse/" + issue.Key
	seen[selfURL] = true

	// URLs from description.
	for _, u := range findURLs(issue.Description) {
		add(u)
	}

	// URLs from comments.
	for _, c := range issue.Comments {
		for _, u := range findURLs(c.Body) {
			add(u)
		}
	}

	// Linked issue URLs.
	for _, link := range issue.IssueLinks {
		if link.OutwardIssue != nil {
			add(host + "/browse/" + link.OutwardIssue.Key)
		}
		if link.InwardIssue != nil {
			add(host + "/browse/" + link.InwardIssue.Key)
		}
	}

	return urls
}

// findURLs extracts http/https URLs from text.
func findURLs(text string) []string {
	var urls []string
	for _, word := range strings.Fields(text) {
		// Strip trailing punctuation.
		word = strings.TrimRight(word, ".,;:!?)")
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			urls = append(urls, word)
		}
	}
	return urls
}

// cleanWikiMarkup strips Jira wiki markup from changelog values.
// Handles: [~accountid:...], {code:lang}...{code}, [text|url], etc.
func cleanWikiMarkup(s string) string {
	if s == "" {
		return s
	}
	result := s

	// [~accountid:UUID] → replace with @user (unresolved mentions)
	for {
		start := strings.Index(result, "[~accountid:")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "]")
		if end == -1 {
			break
		}
		result = result[:start] + "@user" + result[start+end+1:]
	}

	// {code:lang}...{code} → just the content
	for {
		start := strings.Index(result, "{code")
		if start == -1 {
			break
		}
		// Find closing }
		endOpen := strings.Index(result[start:], "}")
		if endOpen == -1 {
			break
		}
		// Find {code} closing tag
		closeTag := strings.Index(result[start+endOpen+1:], "{code}")
		if closeTag == -1 {
			// No closing tag, just strip the opening
			result = result[:start] + result[start+endOpen+1:]
			continue
		}
		content := result[start+endOpen+1 : start+endOpen+1+closeTag]
		result = result[:start] + strings.TrimSpace(content) + result[start+endOpen+1+closeTag+6:]
	}

	// [text|url] → text
	for {
		start := strings.Index(result, "[")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "]")
		if end == -1 {
			break
		}
		inner := result[start+1 : start+end]
		if pipe := strings.Index(inner, "|"); pipe != -1 {
			inner = inner[:pipe]
		}
		result = result[:start] + inner + result[start+end+1:]
	}

	return strings.TrimSpace(result)
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if len(paragraph) <= width {
			lines = append(lines, paragraph)
			continue
		}
		for len(paragraph) > width {
			cut := width
			for cut > 0 && paragraph[cut] != ' ' {
				cut--
			}
			if cut == 0 {
				cut = width
			}
			lines = append(lines, paragraph[:cut])
			paragraph = strings.TrimLeft(paragraph[cut:], " ")
		}
		if paragraph != "" {
			lines = append(lines, paragraph)
		}
	}
	return lines
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}
