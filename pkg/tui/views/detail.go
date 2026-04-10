package views

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

type DetailTab int

const (
	TabDetails DetailTab = iota
	TabComments
	TabHistory
)

// MainMode controls what the right panel displays
type MainMode int

const (
	ModeIssue   MainMode = iota
	ModeSplash
	ModeProject
)

// SplashInfo holds data for the splash/status screen
type SplashInfo struct {
	Version    string
	AuthMethod string
	Host       string
	Email      string
	Project    string
}

const (
	maxBlockLines   = 8 // max visible lines per entry before collapsing
	unknownLabel    = "Unknown"
	noneLabel       = "none"
	noneLabelUpper  = "None"
)

// ExpandBlockMsg is sent when user wants to expand a collapsed block
type ExpandBlockMsg struct {
	Title string
	Lines []string
}

// NavigateIssueMsg is sent when user activates a block linked to a Jira issue
type NavigateIssueMsg struct {
	Key string
}

type DetailView struct {
	issue        *jira.Issue
	project      *jira.Project
	splash       SplashInfo
	mode         MainMode
	activeTab    DetailTab
	scrollY      int
	listCursor   int
	blocks       [][]string
	blockKeys    []string
	dblClick     components.DblClickDetector
	width        int
	height       int
	focused      bool
	theme        *theme.Theme
	ResolveNav   components.NavResolver
}

func NewDetailView() *DetailView {
	return &DetailView{theme: theme.Default, mode: ModeIssue}
}

// IssueKey returns the key of the currently displayed issue, or ""
func (d *DetailView) IssueKey() string {
	if d.issue != nil && d.mode == ModeIssue {
		return d.issue.Key
	}
	return ""
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

// UpdateIssueData stores issue data without changing mode (for background updates)
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
func (d *DetailView) SetFocused(focused bool) {
	if d.focused && !focused {
		// Actually losing focus — reset list cursor.
		d.listCursor = 0
	}
	d.focused = focused
}

func (d *DetailView) ActiveTab() DetailTab { return d.activeTab }

func (d *DetailView) SetActiveTab(tab DetailTab) {
	d.activeTab = tab
	d.scrollY = 0
	d.listCursor = 0
}

func (d *DetailView) SelectedComment() *jira.Comment {
	if d.issue == nil || d.activeTab != TabComments {
		return nil
	}
	if d.listCursor >= 0 && d.listCursor < len(d.issue.Comments) {
		return &d.issue.Comments[d.listCursor]
	}
	return nil
}


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

// ClickTab switches tab based on x position in the title bar
func (d *DetailView) ClickTab(x int) {
	if d.issue == nil {
		return
	}
	labels := d.tabLabels()
	if len(labels) == 0 {
		return
	}

	// Tabs start after "[0] KEY" + " - " (the border char "╭" is col 0).
	prefix := "[0] " + d.issue.Key
	sepW := 3 // " - "
	tabsStart := len(prefix) + sepW

	if x < tabsStart {
		return
	}

	// Each tab owns from its start to the next tab's start (inclusive of separator).
	pos := tabsStart
	for i, tl := range labels {
		labelW := len(tl.label)
		var zoneEnd int
		if i < len(labels)-1 {
			zoneEnd = pos + labelW + sepW
		} else {
			zoneEnd = pos + labelW + 10 // last tab: generous zone
		}
		if x >= pos && x < zoneEnd {
			d.activeTab = tl.tab
			d.scrollY = 0
			d.listCursor = 0
			return
		}
		pos = zoneEnd
	}
}

func (d *DetailView) ScrollBy(delta int) {
	if d.IsListTab() {
		d.listCursor += delta
		if d.listCursor < 0 {
			d.listCursor = 0
		}
		if count := d.listTabItemCount(); d.listCursor >= count {
			d.listCursor = count - 1
		}
		if d.listCursor < 0 {
			d.listCursor = 0
		}
	} else {
		d.scrollY += delta
		if d.scrollY < 0 {
			d.scrollY = 0
		}
	}
}

// ClickItem selects a list item. Double-click on truncated block expands it.
// Returns an ExpandBlockMsg if double-click on truncated block, nil otherwise
func (d *DetailView) ClickItem(relY int) tea.Cmd {
	if !d.IsListTab() || d.issue == nil {
		return nil
	}
	// relY=0 is title bar, relY=1+ is content. Find which block the click falls in.
	// We need to map content line to block index.
	// Simple approach: the clicked line (accounting for scroll) maps to a block.
	clickedLine := d.scrollY + relY - 1 // -1 for title border
	if clickedLine < 0 {
		return nil
	}

	// Walk blocks to find which one contains the clicked line.
	blockWidth := max(d.width-2, 10) - 1 // -1 for list bar prefix
	blocks := d.renderActiveTabBlocks(blockWidth)
	if blocks == nil {
		return nil
	}

	linePos := 0
	for i, block := range blocks {
		displayH := len(block)
		if displayH > maxBlockLines {
			displayH = maxBlockLines + 1
		}
		blockEnd := linePos + displayH
		if clickedLine >= linePos && clickedLine < blockEnd {
			d.listCursor = i
			if d.dblClick.Click(i) && len(block) > maxBlockLines {
				return func() tea.Msg {
					return ExpandBlockMsg{Title: "Details", Lines: block}
				}
			}
			return nil
		}
		linePos = blockEnd + 1
	}
	return nil
}

// listTabItemCount returns the number of items for list-based tabs, 0 for text tabs.
func (d *DetailView) listTabItemCount() int {
	if d.issue == nil {
		return 0
	}
	switch d.activeTab {
	case TabComments:
		return len(d.issue.Comments)
	case TabHistory:
		return len(d.issue.Changelog)
	default:
		return 0
	}
}

func (d *DetailView) IsListTab() bool {
	switch d.activeTab {
	case TabComments, TabHistory:
		return true
	default:
		return false
	}
}

func (d *DetailView) ListCursorUp() {
	if d.listCursor > 0 {
		d.listCursor--
	}
}

func (d *DetailView) ListCursorDown() {
	if count := d.listTabItemCount(); d.listCursor < count-1 {
		d.listCursor++
	}
}

func (d *DetailView) Update(msg tea.Msg) (*DetailView, tea.Cmd) {
	if !d.focused {
		return d, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}
	key := km.String()

	if key == "tab" {
		d.NextTab()
		d.scrollY = 0
		d.listCursor = 0
		return d, nil
	}
	if key == "enter" || key == " " {
		return d.handleActivation()
	}

	nav := components.NavNone
	if d.ResolveNav != nil {
		nav = d.ResolveNav(key)
	}
	switch nav {
	case components.NavNone:
	case components.NavDown:
		d.handleCursorDown()
	case components.NavUp:
		d.handleCursorUp()
	case components.NavHalfDown:
		d.handleHalfPageDown()
	case components.NavHalfUp:
		d.handleHalfPageUp()
	case components.NavTop:
		d.scrollY = 0
		d.listCursor = 0
	case components.NavBottom:
		if count := d.listTabItemCount(); count > 0 {
			d.listCursor = count - 1
		} else {
			d.scrollY = math.MaxInt32
		}
	}
	return d, nil
}

func (d *DetailView) handleCursorDown() {
	if count := d.listTabItemCount(); count > 0 {
		if d.listCursor < count-1 {
			d.listCursor++
		} else {
			d.listCursor = 0
		}
	} else {
		d.scrollY++
	}
}

func (d *DetailView) handleCursorUp() {
	if count := d.listTabItemCount(); count > 0 {
		if d.listCursor > 0 {
			d.listCursor--
		} else {
			d.listCursor = count - 1
		}
	} else if d.scrollY > 0 {
		d.scrollY--
	}
}

func (d *DetailView) handleHalfPageDown() {
	if count := d.listTabItemCount(); count > 0 {
		d.listCursor += d.VisibleRows() / 2
		if d.listCursor >= count {
			d.listCursor = count - 1
		}
	} else {
		d.scrollY += d.VisibleRows() / 2
	}
}

func (d *DetailView) handleHalfPageUp() {
	if d.listTabItemCount() > 0 {
		d.listCursor -= d.VisibleRows() / 2
		if d.listCursor < 0 {
			d.listCursor = 0
		}
	} else {
		d.scrollY -= d.VisibleRows() / 2
		if d.scrollY < 0 {
			d.scrollY = 0
		}
	}
}

func (d *DetailView) handleActivation() (*DetailView, tea.Cmd) {
	if d.IsListTab() && d.listCursor >= 0 && d.listCursor < len(d.blocks) {
		if d.listCursor < len(d.blockKeys) && d.blockKeys[d.listCursor] != "" {
			key := d.blockKeys[d.listCursor]
			return d, func() tea.Msg {
				return NavigateIssueMsg{Key: key}
			}
		}
		block := d.blocks[d.listCursor]
		if len(block) > maxBlockLines {
			return d, func() tea.Msg {
				return ExpandBlockMsg{Title: "Details", Lines: block}
			}
		}
	}
	return d, nil
}

func (d *DetailView) VisibleRows() int {
	return max(d.height-2, 1)
}

func (d *DetailView) View() string {
	contentWidth, innerH := components.PanelDimensions(d.width, d.height)

	if d.mode == ModeSplash {
		return d.renderSplash(contentWidth, innerH)
	}
	if d.mode == ModeProject && d.project != nil {
		return d.renderProjectView(contentWidth, innerH)
	}

	visible := d.VisibleRows()

	if d.issue == nil {
		title := "[0] Detail"
		placeholder := lipgloss.NewStyle().Foreground(theme.ColorGray).Render("Select an issue to view details")
		return components.RenderPanel(title, placeholder, d.width, innerH, d.focused)
	}

	title := d.buildTitle(contentWidth)

	var contentLines []string
	if count := d.listTabItemCount(); count > 0 {
		contentLines = d.renderBlockList(contentWidth, visible)
	} else {
		switch d.activeTab {
		case TabDetails:
			contentLines = d.renderDescription(contentWidth)
		default:
			contentLines = []string{" No content."}
		}
	}

	totalLines := len(contentLines)
	contentLines = d.clampAndSliceScroll(contentLines, visible)

	body := strings.Join(contentLines, "\n")

	footer := ""
	if count := d.listTabItemCount(); count > 0 {
		footer = fmt.Sprintf("%d of %d", d.listCursor+1, count)
	}
	scroll := &components.ScrollInfo{Total: totalLines, Visible: visible, Offset: d.scrollY}
	return components.RenderPanelFull(title, footer, body, d.width, innerH, d.focused, scroll)
}

func (d *DetailView) renderBlockList(contentWidth, visible int) []string {
	blockWidth := contentWidth - 1
	blocks := d.renderActiveTabBlocks(blockWidth)

	if d.listCursor >= len(blocks) {
		d.listCursor = len(blocks) - 1
	}
	if d.listCursor < 0 {
		d.listCursor = 0
	}

	d.blocks = blocks
	d.blockKeys = d.buildBlockKeys(blocks)

	bar := lipgloss.NewStyle().Foreground(theme.ColorBlue).Render("▎")
	ellipsis := lipgloss.NewStyle().Foreground(theme.ColorGray).Render("    ...")
	sep := strings.Repeat("─", blockWidth)

	var lines []string
	for i, block := range blocks {
		displayBlock := block
		truncated := false
		if len(block) > maxBlockLines {
			displayBlock = block[:maxBlockLines]
			truncated = true
		}
		for _, line := range displayBlock {
			if i == d.listCursor && d.focused {
				lines = append(lines, bar+line)
			} else {
				lines = append(lines, " "+line)
			}
		}
		if truncated {
			if i == d.listCursor && d.focused {
				lines = append(lines, bar+ellipsis)
			} else {
				lines = append(lines, " "+ellipsis)
			}
		}
		if i < len(blocks)-1 {
			lines = append(lines, " "+sep)
		}
	}

	d.autoScrollToBlock(blocks, visible)
	return lines
}

func (d *DetailView) autoScrollToBlock(blocks [][]string, visible int) {
	displayBlockHeight := func(block []string) int {
		h := len(block)
		if h > maxBlockLines {
			h = maxBlockLines + 1
		}
		return h
	}
	lineStart := 0
	for i := 0; i < d.listCursor && i < len(blocks); i++ {
		lineStart += displayBlockHeight(blocks[i]) + 1
	}
	margin := 1
	if visible <= 3 {
		margin = 0
	}
	if lineStart-margin < d.scrollY {
		d.scrollY = lineStart - margin
	}
	blockEnd := lineStart + displayBlockHeight(blocks[d.listCursor])
	if blockEnd+margin > d.scrollY+visible {
		d.scrollY = blockEnd + margin - visible
	}
}

func (d *DetailView) clampAndSliceScroll(contentLines []string, visible int) []string {
	maxScroll := max(len(contentLines)-visible, 0)
	if d.scrollY > maxScroll {
		d.scrollY = maxScroll
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
	return scrolled
}

type tabLabel struct {
	tab   DetailTab
	label string
}

func (d *DetailView) tabLabels() []tabLabel {
	var tabs []tabLabel
	tabs = append(tabs, tabLabel{TabDetails, "Body"})
	if d.issue != nil {
		tabs = append(tabs, tabLabel{TabComments, "Cmt"})
	}
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

	prefix := "[0] " + d.issue.Key

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
	// Try rich ADF rendering first.
	if d.issue.DescriptionADF != nil {
		if lines := renderADF(d.issue.DescriptionADF, width-1); len(lines) > 0 {
			result := make([]string, len(lines))
			for i, l := range lines {
				result[i] = " " + l
			}
			return result
		}
	}

	// Fallback: plain text (Server/DC may use Jira wiki markup).
	valStyle := d.theme.ValueStyle
	desc := wikiToPlain(d.issue.Description)
	if desc == "" {
		desc = "(no description)"
	}
	wrapped := wrapText(desc, width-2)
	styled := colorURLsWrapped(wrapped)
	lines := make([]string, 0, len(styled))
	for _, line := range styled {
		lines = append(lines, " "+colorMentions(valStyle.Render(line)))
	}
	return lines
}

var urlStyle = lipgloss.NewStyle().Foreground(theme.ColorCyan).Underline(true)

// colorURLs highlights http/https URLs in a single line with underlined cyan.
func colorURLs(s string) string {
	result := s
	for _, prefix := range []string{"https://", "http://"} {
		for {
			start := strings.Index(result, prefix)
			if start == -1 {
				break
			}
			rest := result[start:]
			end := strings.IndexAny(rest, " \t\n")
			if end == -1 {
				end = len(rest)
			}
			rawURL := rest[:end]
			colored := urlStyle.Render(rawURL)
			result = result[:start] + colored + rest[end:]
		}
	}
	return result
}

// colorURLsWrapped highlights URLs across wrapped lines. If a URL was split
// by wrapText, the continuation on the next line is also highlighted.
func colorURLsWrapped(lines []string) []string {
	result := make([]string, len(lines))
	urlCont := false
	for i, line := range lines {
		if urlCont {
			// Previous line ended mid-URL — highlight continuation.
			end := strings.IndexAny(line, " \t")
			if end == -1 {
				result[i] = urlStyle.Render(line)
				urlCont = true
				continue
			}
			result[i] = urlStyle.Render(line[:end]) + colorURLs(line[end:])
		} else {
			result[i] = colorURLs(line)
		}
		// Check if this line ends mid-URL (URL extends to end of line).
		urlCont = lineEndsInURL(lines[i])
	}
	return result
}

// lineEndsInURL returns true if the raw line ends inside a URL.
func lineEndsInURL(line string) bool {
	lastURL := strings.LastIndex(line, "https://")
	if idx := strings.LastIndex(line, "http://"); idx > lastURL {
		lastURL = idx
	}
	if lastURL == -1 {
		return false
	}
	// If no space after the URL start, it extends to end of line.
	return !strings.ContainsAny(line[lastURL:], " \t")
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
		name, after, found := strings.Cut(rest, suffix)
		if !found {
			break
		}
		colored := theme.AuthorRender(name)
		result = result[:start] + colored + after
	}
	return result
}

// buildBlockKeys returns an issue key per block for navigable tabs (subtasks, links).
func (d *DetailView) buildBlockKeys(blocks [][]string) []string {
	return make([]string, len(blocks))
}

// renderActiveTabBlocks dispatches block rendering to the current tab.
func (d *DetailView) renderActiveTabBlocks(width int) [][]string {
	switch d.activeTab { //nolint:exhaustive
	case TabComments:
		return d.renderCommentBlocks(width)
	case TabHistory:
		return d.renderHistoryBlocks(width)
	}
	return nil
}

// renderEntry renders a single author+time header + content block + separator.

func (d *DetailView) renderHistoryBlocks(width int) [][]string {
	gray := lipgloss.NewStyle().Foreground(theme.ColorGray)
	blocks := make([][]string, 0, len(d.issue.Changelog))

	// Reverse order: newest first.
	for i := len(d.issue.Changelog) - 1; i >= 0; i-- {
		entry := d.issue.Changelog[i]
		author := unknownLabel
		if entry.Author != nil {
			author = entry.Author.DisplayName
		}

		var block []string
		block = append(block, " "+theme.AuthorRender(author)+" "+gray.Render(timeAgo(entry.Created)))

		for _, item := range entry.Items {
			from := cleanWikiMarkup(item.FromString)
			to := cleanWikiMarkup(item.ToString)
			if from == "" {
				from = noneLabel
			}
			if to == "" {
				to = noneLabel
			}

			field := strings.ToLower(item.Field)

			if field == "description" || field == "comment" || field == "environment" {
				block = append(block, "   "+gray.Render(item.Field)+gray.Render(":"))
				block = append(block, renderDiff(from, to, width-4)...)
				continue
			}

			if isMultiSelectField(field) {
				block = append(block, "   "+gray.Render(item.Field)+":")
				block = append(block, renderMultiSelectDiff(from, to)...)
				continue
			}

			// Build plain-text line first, wrap, then apply styling.
			plainLine := fmt.Sprintf("   %s: %s → %s", item.Field, from, to)
			wrapped := wrapText(plainLine, width-2)
			for _, wl := range wrapped {
				// Apply field-name gray.
				wl = strings.Replace(wl, item.Field+":", gray.Render(item.Field)+":", 1)
				// Apply value coloring.
				if field == fieldStatus {
					if from != noneLabel {
						wl = strings.Replace(wl, from, statusNameStyle(from).Render(from), 1)
					}
					if to != noneLabel {
						wl = strings.Replace(wl, to, statusNameStyle(to).Render(to), 1)
					}
				} else if isPersonField(field) {
					if from != noneLabel {
						wl = strings.Replace(wl, from, theme.AuthorRender(from), 1)
					}
					if to != noneLabel {
						wl = strings.Replace(wl, to, theme.AuthorRender(to), 1)
					}
				}
				block = append(block, colorURLs(wl))
			}
		}

		blocks = append(blocks, block)
	}
	return blocks
}

// statusNameStyle returns a color style based on status name heuristics.
func statusNameStyle(name string) lipgloss.Style {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "done") || strings.Contains(lower, "resolved") ||
		strings.Contains(lower, "closed") || strings.Contains(lower, "complete"):
		return lipgloss.NewStyle().Foreground(theme.ColorGreen)
	case strings.Contains(lower, "progress") || strings.Contains(lower, "development") ||
		strings.Contains(lower, "review") || strings.Contains(lower, "testing"):
		return lipgloss.NewStyle().Foreground(theme.ColorYellow)
	case strings.Contains(lower, "todo") || strings.Contains(lower, "open") ||
		strings.Contains(lower, "new") || strings.Contains(lower, "backlog") ||
		strings.Contains(lower, "ready"):
		return lipgloss.NewStyle().Foreground(theme.ColorCyan)
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorWhite)
	}
}

// isPersonField returns true if the field name likely contains a person value.
func isPersonField(field string) bool {
	personFields := []string{
		"assignee", "reviewer", "reporter", "creator", "tester",
		"qa", "developer", "lead", "owner", "approver",
	}
	lower := strings.ToLower(field)
	for _, pf := range personFields {
		if lower == pf || strings.Contains(lower, pf) {
			return true
		}
	}
	return false
}

func isMultiSelectField(field string) bool {
	switch field {
	case "labels", "components", "fix versions", "affects versions", "tags", "component":
		return true
	}
	return false
}

func renderMultiSelectDiff(from, to string) []string {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	parseSet := func(s string) map[string]struct{} {
		m := make(map[string]struct{})
		if s == "" || s == noneLabel {
			return m
		}
		for _, v := range strings.Split(s, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				m[v] = struct{}{}
			}
		}
		return m
	}

	parseList := func(s string) []string {
		if s == "" || s == noneLabel {
			return nil
		}
		var out []string
		for _, v := range strings.Split(s, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				out = append(out, v)
			}
		}
		return out
	}

	fromSet := parseSet(from)
	toSet := parseSet(to)
	fromList := parseList(from)
	toList := parseList(to)

	var lines []string
	// Removed items (in from but not in to), preserve original order.
	for _, v := range fromList {
		if _, ok := toSet[v]; !ok {
			lines = append(lines, red.Render("   - "+v))
		}
	}
	// Added items (in to but not in from), preserve original order.
	for _, v := range toList {
		if _, ok := fromSet[v]; !ok {
			lines = append(lines, green.Render("   + "+v))
		}
	}
	return lines
}

func (d *DetailView) renderCommentBlocks(width int) [][]string {
	gray := lipgloss.NewStyle().Foreground(theme.ColorGray)
	valStyle := d.theme.ValueStyle
	blocks := make([][]string, 0, len(d.issue.Comments))
	for _, c := range d.issue.Comments {
		author := unknownLabel
		if c.Author != nil {
			author = c.Author.DisplayName
		}
		block := []string{" " + theme.AuthorRender(author) + " " + gray.Render(timeAgo(c.Created))}

		// Try rich ADF rendering first.
		var bodyLines []string
		if c.BodyADF != nil {
			bodyLines = renderADF(c.BodyADF, width-1)
		}
		if len(bodyLines) > 0 {
			for _, l := range bodyLines {
				block = append(block, " "+l)
			}
		} else {
			// Fallback: plain text (Server/DC may use wiki markup).
			wrapped := colorURLsWrapped(wrapText(wikiToPlain(c.Body), width-2))
			for _, wl := range wrapped {
				block = append(block, " "+colorMentions(valStyle.Render(wl)))
			}
		}
		blocks = append(blocks, block)
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
	lines = append(lines, gray.Render("  lazyjira "+d.splash.Version))
	lines = append(lines, gray.Render("  (c) 2026 textfuel"))

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
	title := "[0] Project: " + p.Name
	title = components.TruncateEnd(title, contentWidth-2)
	return components.RenderPanel(title, content, d.width, innerH, d.focused)
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

// URLGroup is a named group of URLs for the URL picker
type URLGroup struct {
	Section string
	URLs    []string
}

// ExtractURLs returns URLs found in the issue, grouped by source
func ExtractURLs(issue *jira.Issue, host string) []URLGroup {
	if issue == nil {
		return nil
	}
	seen := make(map[string]bool)
	// Skip the issue's own URL — it's already open.
	seen[host+"/browse/"+issue.Key] = true

	add := func(urls *[]string, u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			*urls = append(*urls, u)
		}
	}

	var groups []URLGroup

	// Body (description): prefer ADF, fallback to plain text.
	var body []string
	if issue.DescriptionADF != nil {
		for _, u := range extractADFURLs(issue.DescriptionADF) {
			add(&body, u)
		}
	} else {
		for _, u := range findURLs(issue.Description) {
			add(&body, u)
		}
	}
	if len(body) > 0 {
		groups = append(groups, URLGroup{"Body", body})
	}

	// Comments: prefer ADF, fallback to plain text.
	var comments []string
	for _, c := range issue.Comments {
		if c.BodyADF != nil {
			for _, u := range extractADFURLs(c.BodyADF) {
				add(&comments, u)
			}
		} else {
			for _, u := range findURLs(c.Body) {
				add(&comments, u)
			}
		}
	}
	if len(comments) > 0 {
		groups = append(groups, URLGroup{"Comments", comments})
	}

	// Linked issues.
	var links []string
	for _, link := range issue.IssueLinks {
		if link.OutwardIssue != nil {
			add(&links, host+"/browse/"+link.OutwardIssue.Key)
		}
		if link.InwardIssue != nil {
			add(&links, host+"/browse/"+link.InwardIssue.Key)
		}
	}
	if len(links) > 0 {
		groups = append(groups, URLGroup{"Links", links})
	}

	// History (changelog).
	var history []string
	for _, entry := range issue.Changelog {
		for _, item := range entry.Items {
			for _, u := range findURLs(item.FromString) {
				add(&history, u)
			}
			for _, u := range findURLs(item.ToString) {
				add(&history, u)
			}
		}
	}
	if len(history) > 0 {
		groups = append(groups, URLGroup{"History", history})
	}

	return groups
}

// findURLs extracts http/https URLs from text.
func findURLs(text string) []string {
	var urls []string
	for _, word := range strings.Fields(text) {
		// Strip surrounding punctuation/brackets.
		word = strings.TrimLeft(word, "([{<\"'")
		word = strings.TrimRight(word, ".,;:!?)]}>\"'")
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			urls = append(urls, word)
		}
	}
	return urls
}

// cleanWikiMarkup strips Jira wiki markup from changelog values
// handles patterns like [~accountid:...], {code:lang}...{code}, [text|url]
func cleanWikiMarkup(s string) string {
	if s == "" {
		return s
	}
	result := strings.ReplaceAll(s, "\r", "")

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
		if lipgloss.Width(paragraph) <= width {
			lines = append(lines, paragraph)
			continue
		}
		runes := []rune(paragraph)
		for len(runes) > 0 {
			// Measure runes until we exceed width.
			w := 0
			cut := 0
			for i, r := range runes {
				rw := lipgloss.Width(string(r))
				if w+rw > width {
					break
				}
				w += rw
				cut = i + 1
			}
			if cut == 0 {
				cut = 1 // at least one rune to avoid infinite loop
			}
			if cut < len(runes) {
				// Back up to last space for word-wrap.
				for j := cut - 1; j > 0; j-- {
					if runes[j] == ' ' {
						cut = j
						break
					}
				}
			}
			lines = append(lines, string(runes[:cut]))
			runes = runes[cut:]
			// Trim leading spaces from next line.
			for len(runes) > 0 && runes[0] == ' ' {
				runes = runes[1:]
			}
		}
	}
	return lines
}

// wikiToPlain converts Jira wiki markup to plain text with basic formatting
// handles *bold*, [text|url], {code}...{code} blocks, and h1 through h6 headings
var (
	wikiLinkRe     = regexp.MustCompile(`\[([^|\]]+)\|([^\]]+)\]`)
	wikiPlainLinkRe = regexp.MustCompile(`\[([^\]|]+)\]`)
	wikiBoldRe     = regexp.MustCompile(`\*([^\n*]+)\*`)
	wikiItalicRe   = regexp.MustCompile(`_([^\n_]+)_`)
	wikiHeadingRe  = regexp.MustCompile(`(?m)^h[1-6]\.\s*`)
)

// wikiToPlain converts Jira wiki markup to readable plain text.
func wikiToPlain(s string) string {
	if s == "" {
		return s
	}
	// Strip carriage returns — Jira Server often sends \r\n which corrupts
	// terminal output (\r moves cursor to column 0 causing text overlap).
	s = strings.ReplaceAll(s, "\r", "")
	s = wikiLinkRe.ReplaceAllString(s, "$1 ($2)")
	s = wikiPlainLinkRe.ReplaceAllString(s, "$1")
	s = wikiBoldRe.ReplaceAllString(s, "$1")
	s = wikiItalicRe.ReplaceAllString(s, "$1")
	s = wikiHeadingRe.ReplaceAllString(s, "")
	for _, tag := range []string{"{code}", "{noformat}", "{quote}"} {
		s = strings.ReplaceAll(s, tag, "")
	}
	return s
}

// RenderDescriptionPreview renders description text for preview in create form
// Cloud converts markdown to ADF then renders richly
// Server strips wiki markup and colors URLs
func RenderDescriptionPreview(text string, width int, isCloud bool) []string {
	if text == "" || width <= 0 {
		return nil
	}
	if isCloud {
		adf := MarkdownToADF(text)
		if lines := renderADF(adf, width); len(lines) > 0 {
			return lines
		}
	}
	plain := wikiToPlain(text)
	wrapped := wrapText(plain, width)
	return colorURLsWrapped(wrapped)
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
