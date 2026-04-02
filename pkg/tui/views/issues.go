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

// TabSwitchedMsg is sent when the user switches issue tabs
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
	tab         int
	tabCache    map[int][]jira.Issue
	userEmail   string
	activeKey   string
	keyColWidth int
	fields      []string
	theme       *theme.Theme
	jqlQuery    string
	jqlTabIdx   int
}

func NewIssuesList() *IssuesList {
	return &IssuesList{theme: theme.Default, jqlTabIdx: -1}
}

func (m *IssuesList) SetFields(fields []string)            { m.fields = fields }
func (m *IssuesList) SetTabs(tabs []config.IssueTabConfig) { m.tabs = tabs }
func (m *IssuesList) SetUserEmail(email string)            { m.userEmail = email }
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

// AddJQLTab creates or replaces the JQL tab with the given query
func (m *IssuesList) AddJQLTab(jql string) {
	if m.jqlTabIdx >= 0 {
		m.jqlQuery = jql
		m.tab = m.jqlTabIdx
		return
	}
	m.tabs = append(m.tabs, config.IssueTabConfig{Name: "JQL", JQL: ""})
	m.jqlTabIdx = len(m.tabs) - 1
	m.jqlQuery = jql
	m.tab = m.jqlTabIdx
	m.loadFromCache()
}

// RemoveJQLTab removes the JQL tab and switches to tab 0
func (m *IssuesList) RemoveJQLTab() {
	if m.jqlTabIdx < 0 {
		return
	}
	m.tabs = m.tabs[:m.jqlTabIdx]
	if m.tabCache != nil {
		delete(m.tabCache, m.jqlTabIdx)
	}
	m.jqlTabIdx = -1
	m.jqlQuery = ""
	m.tab = 0
	m.loadFromCache()
}

// HasJQLTab returns true if a JQL tab currently exists
func (m *IssuesList) HasJQLTab() bool {
	return m.jqlTabIdx >= 0
}

// IsJQLTab returns true if the currently active tab is the JQL tab
func (m *IssuesList) IsJQLTab() bool {
	return m.jqlTabIdx >= 0 && m.tab == m.jqlTabIdx
}

// JQLQuery returns the raw JQL query for the JQL tab
func (m *IssuesList) JQLQuery() string {
	return m.jqlQuery
}

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

func (m *IssuesList) loadFromCache() {
	if m.tabCache != nil {
		if cached, ok := m.tabCache[m.tab]; ok {
			m.allIssues = cached
			m.updateKeyColWidth(cached)
			m.applyFilter()
			return
		}
	}
	m.allIssues = nil
	m.applyFilter()
}

func (m *IssuesList) GetTabIndex() int { return m.tab }

// SetTabIndex switches to the given tab and loads from cache if available
func (m *IssuesList) SetTabIndex(idx int) {
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	m.tab = idx
	m.loadFromCache()
}

func (m *IssuesList) SetIssues(issues []jira.Issue) {
	var selectedKey string
	if sel := m.SelectedIssue(); sel != nil {
		selectedKey = sel.Key
	}

	if m.tabCache == nil {
		m.tabCache = make(map[int][]jira.Issue)
	}
	m.tabCache[m.tab] = issues

	m.allIssues = issues
	m.updateKeyColWidth(issues)
	m.applyFilter()

	if selectedKey != "" {
		m.SelectByKey(selectedKey)
	}
}

// PatchIssue updates a single issue in the current list and tab cache by key
func (m *IssuesList) PatchIssue(updated *jira.Issue) {
	patch := func(issues []jira.Issue) {
		for i, iss := range issues {
			if iss.Key == updated.Key {
				issues[i].Summary = updated.Summary
				issues[i].Status = updated.Status
				issues[i].Priority = updated.Priority
				issues[i].Assignee = updated.Assignee
				issues[i].IssueType = updated.IssueType
				issues[i].Updated = updated.Updated
				return
			}
		}
	}
	patch(m.allIssues)
	if m.tabCache != nil {
		if cached, ok := m.tabCache[m.tab]; ok {
			patch(cached)
		}
	}
	m.applyFilterKeepCursor()
}

func (m *IssuesList) updateKeyColWidth(issues []jira.Issue) {
	m.keyColWidth = 0
	for _, issue := range issues {
		if w := lipgloss.Width(issue.Key); w > m.keyColWidth {
			m.keyColWidth = w
		}
	}
}

// HasCachedTab returns true if the current tab has cached data
func (m *IssuesList) HasCachedTab() bool {
	if m.tabCache == nil {
		return false
	}
	_, ok := m.tabCache[m.tab]
	return ok
}

// SetIssuesForTab stores issues in the cache for a specific tab without updating the display
func (m *IssuesList) SetIssuesForTab(tab int, issues []jira.Issue) {
	if m.tabCache == nil {
		m.tabCache = make(map[int][]jira.Issue)
	}
	m.tabCache[tab] = issues
}

// InvalidateTabCache clears all cached tab data
func (m *IssuesList) InvalidateTabCache() {
	m.tabCache = nil
	if m.jqlTabIdx >= 0 {
		m.tabs = m.tabs[:m.jqlTabIdx]
		m.jqlTabIdx = -1
		m.jqlQuery = ""
		if m.tab >= len(m.tabs) {
			m.tab = 0
		}
	}
}

func (m *IssuesList) SetFilter(query string) {
	m.filter = query
	m.applyFilter()
}

// ClearFilter removes the search filter and preserves cursor position
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

// FindInAnyTab checks all tab caches for the given key and returns (tabIndex, true) if found
func (m *IssuesList) FindInAnyTab(key string) (int, bool) {
	for _, issue := range m.issues {
		if issue.Key == key {
			return m.tab, true
		}
	}
	for tab, issues := range m.tabCache {
		if tab == m.tab {
			continue
		}
		for _, issue := range issues {
			if issue.Key == key {
				return tab, true
			}
		}
	}
	return -1, false
}

// InjectIssue adds an issue to tab 0 cache if not already present
func (m *IssuesList) InjectIssue(issue jira.Issue) {
	if m.tabCache == nil {
		m.tabCache = make(map[int][]jira.Issue)
	}
	cached := m.tabCache[0]
	for _, iss := range cached {
		if iss.Key == issue.Key {
			return
		}
	}
	m.tabCache[0] = append([]jira.Issue{issue}, cached...)
}

// SelectByKey moves cursor to the issue with the given key and returns true if found
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

// ContentHeight returns natural height of items plus 2 borders with a minimum of 7
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

// ClickTabAt handles clicks on the title bar to switch tabs and returns true if the tab changed
func (m *IssuesList) ClickTabAt(x int) bool {
	if len(m.tabs) == 0 {
		return false
	}
	prefix := 4
	sepW := 3
	pos := prefix
	for i, t := range m.tabs {
		labelW := len(t.Name)
		var zoneEnd int
		if i < len(m.tabs)-1 {
			zoneEnd = pos + labelW + sepW
		} else {
			zoneEnd = pos + labelW + 10
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
		fields = []string{"key", fieldStatus, "summary"}
	}

	fixedWidth := 1
	if len(fields) > 1 {
		fixedWidth += len(fields) - 1
	}
	for _, f := range fields {
		switch f {
		case "key":
			fixedWidth += m.keyColWidth
		case fieldStatus:
			fixedWidth += 1
		case "priority":
			fixedWidth += 8
		case "assignee":
			fixedWidth += 12
		case "type":
			fixedWidth += 10
		case "updated":
			fixedWidth += 8
		case "summary":
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
		case fieldStatus:
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

// padRight pads s with spaces to width w using visible ANSI-aware width
func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// issueTimeAgo returns a short relative time string for the given timestamp
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

// statusEmojiPlain returns uncolored status char for selected rows
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
