package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

type IssuesLoadedMsg struct{ Issues []jira.Issue }
type IssueSelectedMsg struct{ Issue *jira.Issue }

// TabSwitchedMsg is sent when the user switches issue tabs.
type TabSwitchedMsg struct {
	Tab config.IssueTabConfig
}

const statusOpen = "○"

type IssuesList struct {
	components.ListBase
	issues      []jira.Issue
	allIssues   []jira.Issue
	filter      string
	tabs        []config.IssueTabConfig
	tab         int // active tab index
	tabCache    map[int][]jira.Issue // per-tab cached issues
	userEmail   string
	activeKey   string // the issue currently being viewed
	keyColWidth int
	fields      []string
	theme       *theme.Theme
}

func NewIssuesList() *IssuesList {
	return &IssuesList{theme: theme.Default}
}

func (m *IssuesList) SetFields(fields []string)             { m.fields = fields }
func (m *IssuesList) SetTabs(tabs []config.IssueTabConfig)  { m.tabs = tabs }
func (m *IssuesList) SetUserEmail(email string)              { m.userEmail = email }
func (m *IssuesList) ActiveTab() config.IssueTabConfig {
	if m.tab >= 0 && m.tab < len(m.tabs) {
		return m.tabs[m.tab]
	}
	return config.IssueTabConfig{}
}
func (m *IssuesList) SetActiveKey(key string) {
	m.activeKey = key
	m.applyFilterKeepCursor()
}
func (m *IssuesList) ClearActiveKey() { m.activeKey = "" }

func (m *IssuesList) NextTab() {
	if len(m.tabs) == 0 {
		return
	}
	m.tab = (m.tab + 1) % len(m.tabs)
	m.loadFromCache()
}

func (m *IssuesList) PrevTab() {
	if len(m.tabs) == 0 {
		return
	}
	m.tab = (m.tab + len(m.tabs) - 1) % len(m.tabs)
	m.loadFromCache()
}

// loadFromCache switches display to cached data for current tab, if available.
func (m *IssuesList) loadFromCache() {
	if m.tabCache != nil {
		if cached, ok := m.tabCache[m.tab]; ok {
			m.allIssues = cached
			m.updateKeyColWidth(cached)
			m.applyFilter()
			return
		}
	}
	// No cache — clear list, will be populated by fetch.
	m.allIssues = nil
	m.applyFilter()
}

func (m *IssuesList) GetTabIndex() int { return m.tab }

// SetTabIndex switches to the given tab and loads from cache if available.
func (m *IssuesList) SetTabIndex(idx int) {
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	m.tab = idx
	m.loadFromCache()
}

func (m *IssuesList) SetIssues(issues []jira.Issue) {
	// Remember current selection to preserve position.
	var selectedKey string
	if sel := m.SelectedIssue(); sel != nil {
		selectedKey = sel.Key
	}

	// Store in per-tab cache.
	if m.tabCache == nil {
		m.tabCache = make(map[int][]jira.Issue)
	}
	m.tabCache[m.tab] = issues

	m.allIssues = issues
	m.updateKeyColWidth(issues)
	m.applyFilter()

	// Restore cursor position.
	if selectedKey != "" {
		m.SelectByKey(selectedKey)
	}
}

func (m *IssuesList) updateKeyColWidth(issues []jira.Issue) {
	m.keyColWidth = 0
	for _, issue := range issues {
		if w := lipgloss.Width(issue.Key); w > m.keyColWidth {
			m.keyColWidth = w
		}
	}
}

// HasCachedTab returns true if the current tab has cached data.
func (m *IssuesList) HasCachedTab() bool {
	if m.tabCache == nil {
		return false
	}
	_, ok := m.tabCache[m.tab]
	return ok
}

// SetIssuesForTab stores issues in the cache for a specific tab without updating the display.
func (m *IssuesList) SetIssuesForTab(tab int, issues []jira.Issue) {
	if m.tabCache == nil {
		m.tabCache = make(map[int][]jira.Issue)
	}
	m.tabCache[tab] = issues
}

// InvalidateTabCache clears all cached tab data (e.g. on project switch).
func (m *IssuesList) InvalidateTabCache() {
	m.tabCache = nil
}

func (m *IssuesList) SetFilter(query string) {
	m.filter = query
	m.applyFilter()
}

// ClearFilter removes the search filter but keeps tab filter. Cursor preserved via SelectByKey.
func (m *IssuesList) ClearFilter() {
	m.filter = ""
	m.applyFilterKeepCursor()
}

func (m *IssuesList) applyFilterKeepCursor() {
	prevKey := ""
	if sel := m.SelectedIssue(); sel != nil {
		prevKey = sel.Key
	}
	m.applyFilter()
	if prevKey != "" {
		m.SelectByKey(prevKey)
	}
}

// SelectByKey moves cursor to the issue with the given key. Returns true if found.
func (m *IssuesList) SelectByKey(key string) bool {
	for i, issue := range m.issues {
		if issue.Key == key {
			m.Cursor = i
			m.AdjustOffset()
			return true
		}
	}
	return false
}

func (m *IssuesList) applyFilter() {
	// With config-driven tabs, all tab filtering is server-side via JQL.
	// Only apply text search filter client-side.
	source := m.allIssues

	if m.filter == "" {
		m.issues = source
	} else {
		q := strings.ToLower(m.filter)
		var filtered []jira.Issue
		for _, issue := range source {
			haystack := strings.ToLower(issue.Key + " " + issue.Summary)
			if issue.Assignee != nil {
				haystack += " " + strings.ToLower(issue.Assignee.DisplayName)
			}
			if strings.Contains(haystack, q) {
				filtered = append(filtered, issue)
			}
		}
		m.issues = filtered
	}
	// Pin active (selected) issue to top of the list.
	if m.activeKey != "" {
		for i, issue := range m.issues {
			if issue.Key == m.activeKey {
				if i > 0 {
					m.issues[0], m.issues[i] = m.issues[i], m.issues[0]
				}
				break
			}
		}
	}

	m.Cursor = 0
	m.Offset = 0
	m.SetItemCount(len(m.issues))
}

// ContentHeight returns natural height: items + 2 borders. Min 7 before data loads.
func (m *IssuesList) ContentHeight() int {
	return m.ListBase.ContentHeight(7)
}

func (m *IssuesList) SelectedIssue() *jira.Issue {
	if len(m.issues) == 0 || m.Cursor < 0 || m.Cursor >= len(m.issues) {
		return nil
	}
	return &m.issues[m.Cursor]
}

func (m *IssuesList) Init() tea.Cmd { return nil }

func (m *IssuesList) Update(msg tea.Msg) (*IssuesList, tea.Cmd) {
	if !m.Focused {
		return m, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		if m.KeyNav(msg.String()) {
			return m, func() tea.Msg {
				return IssueSelectedMsg{Issue: m.SelectedIssue()}
			}
		}
	}
	return m, nil
}

func (m *IssuesList) View() string {
	if m.Height <= 1 {
		footer := ""
		if n := len(m.issues); n > 0 {
			footer = fmt.Sprintf("%d of %d", m.Cursor+1, n)
		}
		return components.RenderCollapsedBar(m.buildTitle(), footer, m.Width, m.Focused)
	}

	contentWidth, _ := components.PanelDimensions(m.Width, m.Height)
	visible := m.VisibleRows()

	var rows []string
	end := min(m.Offset+visible, len(m.issues))
	for i := m.Offset; i < end; i++ {
		rows = append(rows, m.renderIssueRow(m.issues[i], contentWidth, i == m.Cursor))
	}

	content := strings.Join(rows, "\n")
	title := m.buildTitle()
	footer := ""
	if len(m.issues) > 0 {
		footer = fmt.Sprintf("%d of %d", m.Cursor+1, len(m.issues))
	}
	scroll := &components.ScrollInfo{Total: len(m.issues), Visible: visible, Offset: m.Offset}
	return components.RenderPanelFull(title, footer, content, m.Width, visible, m.Focused, scroll)
}

// ClickTabAt handles clicks on the title bar to switch tabs.
// Returns true if the tab actually changed.
func (m *IssuesList) ClickTabAt(x int) bool {
	if len(m.tabs) == 0 {
		return false
	}
	prefix := 4 // "[2] "
	sepW := 3   // " - "
	pos := prefix
	for i, t := range m.tabs {
		labelW := len(t.Name)
		var zoneEnd int
		if i < len(m.tabs)-1 {
			zoneEnd = pos + labelW + sepW
		} else {
			zoneEnd = pos + labelW + 10 // generous zone for last tab
		}
		if x >= pos && x < zoneEnd {
			if m.tab != i {
				m.tab = i
				m.applyFilter()
				return true
			}
			return false
		}
		pos = zoneEnd
	}
	return false
}

func (m *IssuesList) buildTitle() string {
	activeStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	sep := lipgloss.NewStyle().Foreground(theme.ColorGray).Render(" - ")

	if len(m.tabs) == 0 {
		return "[2] Issues"
	}

	var parts []string
	for i, t := range m.tabs {
		if i == m.tab {
			parts = append(parts, activeStyle.Render(t.Name))
		} else {
			parts = append(parts, inactiveStyle.Render(t.Name))
		}
	}
	return "[2] " + strings.Join(parts, sep)
}

func (m *IssuesList) renderIssueRow(issue jira.Issue, width int, selected bool) string {
	fields := m.fields
	if len(fields) == 0 {
		fields = []string{"key", "status", "summary"}
	}

	// Calculate fixed column widths.
	// Line format: marker(1) + field1 + " " + field2 + " " + ...
	fixedWidth := 1 // active marker char
	if len(fields) > 1 {
		fixedWidth += len(fields) - 1 // spaces between fields
	}
	for _, f := range fields {
		switch f {
		case "key":
			fixedWidth += m.keyColWidth
		case "status":
			fixedWidth += 1 // single emoji char
		case "priority":
			fixedWidth += 8
		case "assignee":
			fixedWidth += 12
		case "type":
			fixedWidth += 10
		case "updated":
			fixedWidth += 8
		case "summary":
			// elastic — calculated after
		}
	}
	summaryWidth := max(width-fixedWidth, 5)

	active := issue.Key == m.activeKey
	markerChar := " "
	if active {
		markerChar = "*"
	}

	var parts []string
	for _, f := range fields {
		switch f {
		case "key":
			parts = append(parts, padRight(issue.Key, m.keyColWidth))
		case "summary":
			parts = append(parts, components.TruncateEnd(issue.Summary, summaryWidth))
		case "status":
			if selected {
				parts = append(parts, statusEmojiPlain(issue.Status))
			} else {
				parts = append(parts, statusEmoji(issue.Status))
			}
		case "priority":
			name := ""
			if issue.Priority != nil {
				name = issue.Priority.Name
			}
			parts = append(parts, padRight(components.TruncateEnd(name, 8), 8))
		case "assignee":
			name := ""
			if issue.Assignee != nil {
				name = issue.Assignee.DisplayName
			}
			parts = append(parts, padRight(components.TruncateEnd(name, 12), 12))
		case "type":
			name := ""
			if issue.IssueType != nil {
				name = issue.IssueType.Name
			}
			parts = append(parts, padRight(components.TruncateEnd(name, 10), 10))
		case "updated":
			parts = append(parts, padRight(issueTimeAgo(issue.Updated), 8))
		}
	}
	line := markerChar + strings.Join(parts, " ")

	if selected && m.Focused {
		return m.theme.SelectedItem.Width(width).Render(line)
	}
	if active {
		coloredMarker := lipgloss.NewStyle().Foreground(theme.ColorGreen).Render(markerChar)
		rest := strings.Join(parts, " ")
		return m.theme.NormalItem.Width(width).Render(coloredMarker + rest)
	}
	return m.theme.NormalItem.Width(width).Render(line)
}

// padRight pads s with spaces to width w, using visible (ANSI-aware) width.
func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// issueTimeAgo returns a short relative time string for the given timestamp.
func issueTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}

// statusEmojiPlain returns uncolored status char for selected rows.
func statusEmojiPlain(status *jira.Status) string {
	if status == nil {
		return statusOpen
	}
	switch status.CategoryKey {
	case "done":
		return "✓"
	case "indeterminate":
		return "→"
	default:
		return statusOpen
	}
}

func statusEmoji(status *jira.Status) string {
	if status == nil {
		return statusOpen
	}
	switch status.CategoryKey {
	case "done":
		return theme.StatusColor("done").Render("✓")
	case "indeterminate":
		return theme.StatusColor("indeterminate").Render("→")
	default:
		return theme.StatusColor("new").Render("○")
	}
}
