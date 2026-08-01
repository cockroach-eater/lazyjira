package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/navstack"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

type IssuesLoadedMsg struct{ Issues []jira.Issue }
type IssueSelectedMsg struct{ Issue *jira.Issue }

// TabSwitchedMsg is sent when the user switches issue tabs
type TabSwitchedMsg struct {
	Tab config.IssueTabConfig
}

const statusOpen = "○"

// tabKind distinguishes the source/lifecycle of a tab in the visible bar.
type tabKind int

const (
	tabKindConfig    tabKind = iota // from config.yml issueTabs
	tabKindManaged                  // from saved_tabs.yml (Phase 2)
	tabKindJQLSearch                // transient JQL search
	tabKindHierarchy                // transient child/parent
)

// tab is one entry in the visible tab bar. It carries its kind and, for
// managed tabs, the store identity (storeIdx) as fields rather than relying
// on index position.
type tab struct {
	kind     tabKind
	cfg      config.IssueTabConfig // Name, JQL, MaxResults
	storeIdx int                   // index in savedTabs; -1 when not managed
	issues   []jira.Issue          // loaded issues (was tabCache[idx])
	loaded   bool                  // distinguishes "not loaded" from "empty"
}

type IssuesList struct {
	components.ListBase
	issues           []jira.Issue
	allIssues        []jira.Issue
	filter           string
	tabs             []tab
	tab              int
	configTabs       []config.IssueTabConfig // raw config.yml tabs, before assembly
	savedTabs        []config.ManagedTab     // raw managed store, before assembly
	projectKey       string                  // current project, for managed-tab filtering
	tabEpoch         int                     // bumped on every reassembly; guards stale tab fetches
	jqlOriginTab     int                     // tab to return to when the JQL tab is closed
	userEmail        string
	keyColWidth      int
	fields           []string
	theme            *theme.Theme
	typeIcons        map[string]string
	typeIconCols     int
	statusIcons      map[string]string
	statusIconCols   int
	priorityIcons    map[string]string
	priorityIconCols int
	jqlQuery         string
	hierarchyTitle   string
	hierarchyStack   *navstack.NavStack
}

func NewIssuesList() *IssuesList {
	return &IssuesList{theme: theme.Default}
}

// jqlTabIndex returns the index of the transient JQL tab, or (-1, false).
func (m *IssuesList) jqlTabIndex() (int, bool) {
	for i := range m.tabs {
		if m.tabs[i].kind == tabKindJQLSearch {
			return i, true
		}
	}
	return -1, false
}

// hierarchyTabIndex returns the index of the transient hierarchy tab, or (-1, false).
func (m *IssuesList) hierarchyTabIndex() (int, bool) {
	for i := range m.tabs {
		if m.tabs[i].kind == tabKindHierarchy {
			return i, true
		}
	}
	return -1, false
}

func (m *IssuesList) SetFields(fields []string) { m.fields = fields }
func (m *IssuesList) SetTypeIcons(icons map[string]string) {
	m.typeIcons = icons
	max := 0
	for _, icon := range icons {
		if w := lipgloss.Width(icon); w > max {
			max = w
		}
	}
	m.typeIconCols = max
}
func (m *IssuesList) SetStatusIcons(icons map[string]string) {
	m.statusIcons = icons
	max := 0
	for _, icon := range icons {
		if w := lipgloss.Width(icon); w > max {
			max = w
		}
	}
	m.statusIconCols = max
}
func (m *IssuesList) SetPriorityIcons(icons map[string]string) {
	m.priorityIcons = icons
	max := 0
	for _, icon := range icons {
		if w := lipgloss.Width(icon); w > max {
			max = w
		}
	}
	m.priorityIconCols = max
}

// SetTabs sets the raw config.yml tabs and reassembles the visible bar. It
// assembles immediately (rather than deferring to the next external trigger)
// so m.tabs is never empty between this setter and a project switch.
func (m *IssuesList) SetTabs(tabs []config.IssueTabConfig) {
	m.configTabs = tabs
	m.RebuildTabs(m.projectKey)
}

// SetSavedTabs sets the raw managed-tab store and reassembles the visible bar.
func (m *IssuesList) SetSavedTabs(tabs []config.ManagedTab) {
	m.savedTabs = tabs
	m.RebuildTabs(m.projectKey)
}

// RebuildTabs assembles the visible tab bar for projectKey: managed tabs first
// (filtered to project=="" or ==projectKey, in store order), then config tabs
// whose name is not shadowed by a managed tab. Transient tabs are not
// re-injected. Each managed tab carries its store index for later mutation.
func (m *IssuesList) RebuildTabs(projectKey string) {
	m.projectKey = projectKey
	m.tabEpoch++

	var assembled []tab
	shadow := make(map[string]bool)
	for i, mt := range m.savedTabs {
		if mt.Project != "" && mt.Project != projectKey {
			continue
		}
		assembled = append(assembled, tab{
			kind:     tabKindManaged,
			cfg:      config.IssueTabConfig{Name: mt.Name, JQL: mt.JQL, MaxResults: mt.MaxResults},
			storeIdx: i,
		})
		shadow[mt.Name] = true
	}
	for _, ct := range m.configTabs {
		if shadow[ct.Name] {
			continue
		}
		assembled = append(assembled, tab{kind: tabKindConfig, cfg: ct, storeIdx: -1})
	}

	m.tabs = assembled
	if m.tab < 0 || m.tab >= len(m.tabs) {
		m.tab = m.HomeTabIndex()
	}
	m.loadFromCache()
}
func (m *IssuesList) SetUserEmail(email string) { m.userEmail = email }
func (m *IssuesList) ActiveTab() config.IssueTabConfig {
	if m.tab >= 0 && m.tab < len(m.tabs) {
		return m.tabs[m.tab].cfg
	}
	return config.IssueTabConfig{}
}

// AddJQLTab creates or replaces the JQL tab with the given query. On creation
// it records the current tab as the origin to return to when the tab is closed.
func (m *IssuesList) AddJQLTab(jql string) {
	if idx, ok := m.jqlTabIndex(); ok {
		m.jqlQuery = jql
		m.tab = idx
		return
	}
	m.jqlOriginTab = m.tab
	m.tabs = append(m.tabs, tab{kind: tabKindJQLSearch, cfg: config.IssueTabConfig{Name: "JQL"}, storeIdx: -1})
	m.jqlQuery = jql
	m.tab = len(m.tabs) - 1
	m.loadFromCache()
}

// RemoveJQLTab removes the JQL tab and returns to the origin tab it was opened
// from, falling back to the home tab if that origin is no longer valid.
func (m *IssuesList) RemoveJQLTab() {
	idx, ok := m.jqlTabIndex()
	if !ok {
		return
	}
	m.tabs = m.tabs[:idx]
	m.jqlQuery = ""
	if m.jqlOriginTab >= 0 && m.jqlOriginTab < len(m.tabs) {
		m.tab = m.jqlOriginTab
	} else {
		m.tab = m.HomeTabIndex()
	}
	m.loadFromCache()
}

// HasJQLTab returns true if a JQL tab currently exists
func (m *IssuesList) HasJQLTab() bool {
	_, ok := m.jqlTabIndex()
	return ok
}

// IsJQLTab returns true if the currently active tab is the JQL tab
func (m *IssuesList) IsJQLTab() bool {
	idx, ok := m.jqlTabIndex()
	return ok && m.tab == idx
}

// JQLQuery returns the raw JQL query for the JQL tab
func (m *IssuesList) JQLQuery() string {
	return m.jqlQuery
}

// Appends and focuses a hierarchy tab. If one already exists, behaves
// like ReplaceHierarchyTabContent and returns the existing index.
func (m *IssuesList) AddHierarchyTab(title string, issues []jira.Issue) int {
	if idx, ok := m.hierarchyTabIndex(); ok {
		m.ReplaceHierarchyTabContent(title, issues)
		return idx
	}
	m.tabs = append(m.tabs, tab{kind: tabKindHierarchy, cfg: config.IssueTabConfig{Name: title}, storeIdx: -1})
	idx := len(m.tabs) - 1
	m.hierarchyTitle = title
	m.hierarchyStack = navstack.NewNavStack()
	m.tabs[idx].issues = issues
	m.tabs[idx].loaded = true
	m.tab = idx
	m.loadFromCache()
	return idx
}

// Replaces the hierarchy tab's title and issue list while keeping the
// tab index and stack stable. No-op if no hierarchy tab.
func (m *IssuesList) ReplaceHierarchyTabContent(title string, issues []jira.Issue) {
	idx, ok := m.hierarchyTabIndex()
	if !ok {
		return
	}
	m.hierarchyTitle = title
	m.tabs[idx].cfg = config.IssueTabConfig{Name: title}
	m.tabs[idx].issues = issues
	m.tabs[idx].loaded = true
	if m.tab == idx {
		m.loadFromCache()
	}
}

// Removes the hierarchy tab, drops its stack, and switches to tab 0.
// No-op if no hierarchy tab.
func (m *IssuesList) RemoveHierarchyTab() {
	idx, ok := m.hierarchyTabIndex()
	if !ok {
		return
	}
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	m.hierarchyTitle = ""
	m.hierarchyStack = nil
	m.tab = m.HomeTabIndex()
	m.loadFromCache()
}

func (m *IssuesList) HasHierarchyTab() bool {
	_, ok := m.hierarchyTabIndex()
	return ok
}

func (m *IssuesList) IsHierarchyTab() bool {
	idx, ok := m.hierarchyTabIndex()
	return ok && m.tab == idx
}

// IsManagedTab reports whether the active tab originates from the managed store.
func (m *IssuesList) IsManagedTab() bool {
	return m.tab >= 0 && m.tab < len(m.tabs) && m.tabs[m.tab].kind == tabKindManaged
}

// IsConfigTab reports whether the active tab originates from config.yml.
func (m *IssuesList) IsConfigTab() bool {
	return m.tab >= 0 && m.tab < len(m.tabs) && m.tabs[m.tab].kind == tabKindConfig
}

// ActiveManagedStoreIdx returns the active tab's index in the managed store, or
// -1 when the active tab is not managed.
func (m *IssuesList) ActiveManagedStoreIdx() int {
	if m.IsManagedTab() {
		return m.tabs[m.tab].storeIdx
	}
	return -1
}

// VisibleManagedStoreIndices returns the store indices of the currently visible
// managed tabs, in visible (left-to-right) order. Reorder maps a visible
// neighbor back to its store entry through this.
func (m *IssuesList) VisibleManagedStoreIndices() []int {
	var idxs []int
	for i := range m.tabs {
		if m.tabs[i].kind == tabKindManaged {
			idxs = append(idxs, m.tabs[i].storeIdx)
		}
	}
	return idxs
}

// The current hierarchy tab title ("Children"/"Parent"/"Link").
func (m *IssuesList) HierarchyTitle() string {
	return m.hierarchyTitle
}

// The NavStack associated with the hierarchy tab, or nil if no
// hierarchy tab exists.
func (m *IssuesList) HierarchyStack() *navstack.NavStack {
	return m.hierarchyStack
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
	if m.tab >= 0 && m.tab < len(m.tabs) && m.tabs[m.tab].loaded {
		cached := m.tabs[m.tab].issues
		m.allIssues = cached
		m.updateKeyColWidth(cached)
		m.applyFilter()
		return
	}
	m.allIssues = nil
	m.applyFilter()
}

func (m *IssuesList) GetTabIndex() int { return m.tab }

// TabEpoch returns the current reassembly generation. A tab fetch captures it
// at issue time; a result whose epoch no longer matches targets a stale tab
// layout and must be discarded.
func (m *IssuesList) TabEpoch() int { return m.tabEpoch }

// HomeTabIndex returns the index of the first config tab (the primary issue
// list), or 0 if none exists. With managed tabs prepended, index 0 is no
// longer necessarily the home list.
func (m *IssuesList) HomeTabIndex() int {
	for i := range m.tabs {
		if m.tabs[i].kind == tabKindConfig {
			return i
		}
	}
	return 0
}

func (m *IssuesList) CurrentIssues() []jira.Issue { return m.allIssues }

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

	if m.tab >= 0 && m.tab < len(m.tabs) {
		m.tabs[m.tab].issues = issues
		m.tabs[m.tab].loaded = true
	}

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
		for i := range issues {
			if issues[i].Key == updated.Key {
				PatchIssueFields(&issues[i], updated)
				return
			}
		}
	}
	patch(m.allIssues)
	if m.tab >= 0 && m.tab < len(m.tabs) && m.tabs[m.tab].loaded {
		patch(m.tabs[m.tab].issues)
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

// HasCachedTab returns true if the current tab has loaded data
func (m *IssuesList) HasCachedTab() bool {
	return m.tab >= 0 && m.tab < len(m.tabs) && m.tabs[m.tab].loaded
}

// SetIssuesForTab stores issues for a specific tab without updating the display.
// Out-of-bounds indices are ignored (stale async fetch after a tab rebuild).
func (m *IssuesList) SetIssuesForTab(tab int, issues []jira.Issue) {
	if tab < 0 || tab >= len(m.tabs) {
		return
	}
	m.tabs[tab].issues = issues
	m.tabs[tab].loaded = true
}

// InvalidateTabCache drops loaded issues from persistent tabs and removes
// transient tabs (JQL and Hierarchy). Surviving tabs reset to not-loaded so
// the next access refetches.
func (m *IssuesList) InvalidateTabCache() {
	kept := m.tabs[:0:0]
	for _, t := range m.tabs {
		if t.kind == tabKindJQLSearch || t.kind == tabKindHierarchy {
			continue
		}
		t.issues = nil
		t.loaded = false
		kept = append(kept, t)
	}
	m.tabs = kept
	m.jqlQuery = ""
	m.hierarchyTitle = ""
	m.hierarchyStack = nil
	if m.tab >= len(m.tabs) {
		m.tab = m.HomeTabIndex()
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
	for i := range m.tabs {
		if i == m.tab {
			continue
		}
		for _, issue := range m.tabs[i].issues {
			if issue.Key == key {
				return i, true
			}
		}
	}
	return -1, false
}

// InjectIssue adds an issue to the home tab's cache if not already present
func (m *IssuesList) InjectIssue(issue jira.Issue) {
	if len(m.tabs) == 0 {
		return
	}
	h := m.HomeTabIndex()
	cached := m.tabs[h].issues
	for _, iss := range cached {
		if iss.Key == issue.Key {
			return
		}
	}
	m.tabs[h].issues = append([]jira.Issue{issue}, cached...)
	m.tabs[h].loaded = true
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
	contentWidth, _ := components.PanelDimensions(m.Width, m.Height)
	// Top border: ╭─ {title} {padding}╮
	// contentWidth spans from after ╭─ to the right border; -1 reserves the ╮.
	maxTitleW := contentWidth - 1

	if m.Height <= 1 {
		footer := ""
		if n := len(m.issues); n > 0 {
			footer = fmt.Sprintf("%d of %d", m.Cursor+1, n)
		}
		return components.RenderCollapsedBar(m.buildTitle(maxTitleW), footer, m.Width, m.Focused)
	}

	visible := m.VisibleRows()

	var rows []string
	end := min(m.Offset+visible, len(m.issues))
	for i := m.Offset; i < end; i++ {
		rows = append(rows, m.renderIssueRow(m.issues[i], contentWidth, i == m.Cursor))
	}

	content := strings.Join(rows, "\n")
	title := m.buildTitle(maxTitleW)
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
		labelW := len(t.cfg.Name)
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

// buildTitle builds the tab bar title for the issues panel. When all tabs fit
// within maxTitleW the full list is shown. When they don't, a contiguous
// sliding window that always contains the active tab is displayed, preserving
// the original tab order. The window expands leftward first (showing context
// before the active tab), then fills any remaining budget rightward.
func (m *IssuesList) buildTitle(maxTitleW int) string {
	activeStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	sep := lipgloss.NewStyle().Foreground(theme.ColorGray).Render(" - ")
	sepW := lipgloss.Width(sep)

	if len(m.tabs) == 0 {
		return "[2] Issues"
	}

	labels := make([]string, len(m.tabs))
	labelW := make([]int, len(m.tabs))
	for i, t := range m.tabs {
		if i == m.tab {
			labels[i] = activeStyle.Render(t.cfg.Name)
		} else {
			labels[i] = inactiveStyle.Render(t.cfg.Name)
		}
		labelW[i] = lipgloss.Width(labels[i])
	}

	const prefix = "[2] "
	prefixW := lipgloss.Width(prefix)

	fullTitle := prefix + strings.Join(labels, sep)
	if maxTitleW <= 0 || lipgloss.Width(fullTitle) <= maxTitleW {
		return fullTitle
	}

	// Overflow: find a contiguous sliding window that always contains the
	// active tab. Expand left first (preserving context), then fill right.
	budget := maxTitleW - prefixW

	// If the active tab label alone exceeds the budget, truncate it so the
	// title never returns wider than maxTitleW regardless of label length.
	if budget > 0 && labelW[m.tab] > budget {
		truncated := components.TruncateEnd(m.tabs[m.tab].cfg.Name, budget)
		labels[m.tab] = activeStyle.Render(truncated)
		labelW[m.tab] = lipgloss.Width(labels[m.tab])
	}

	start := m.tab
	end := m.tab
	used := labelW[m.tab]

	for start > 0 {
		cost := labelW[start-1] + sepW
		if used+cost > budget {
			break
		}
		start--
		used += cost
	}
	for end < len(m.tabs)-1 {
		cost := labelW[end+1] + sepW
		if used+cost > budget {
			break
		}
		end++
		used += cost
	}

	var parts []string
	for i := start; i <= end; i++ {
		parts = append(parts, labels[i])
	}
	return prefix + strings.Join(parts, sep)
}

func (m *IssuesList) renderIssueRow(issue jira.Issue, width int, selected bool) string {
	fields := m.fields
	if len(fields) == 0 {
		fields = []string{"key", fieldStatus, "summary"}
	}

	currTypeIcon := typeIcon(m.typeIcons, issue.IssueType)
	currStatusIcon := statusIcon(m.statusIcons, issue.Status)
	currPriorityIcon := priorityIcon(m.priorityIcons, issue.Priority)

	fixedWidth := 1
	if len(fields) > 1 {
		fixedWidth += len(fields) - 1
	}
	for _, f := range fields {
		switch f {
		case "key":
			fixedWidth += m.keyColWidth
		case fieldStatus:
			fixedWidth += max(1, m.statusIconCols)
		case "priority":
			if currPriorityIcon != "" {
				fixedWidth += m.priorityIconCols
			} else {
				fixedWidth += 8
			}
		case "assignee":
			fixedWidth += 12
		case "type":
			if currTypeIcon != "" {
				fixedWidth += m.typeIconCols
			} else {
				fixedWidth += 10
			}
		case "updated":
			fixedWidth += 8
		case "summary":
		}
	}
	summaryWidth := max(width-fixedWidth, 5)

	var parts []string
	for _, f := range fields {
		switch f {
		case "key":
			parts = append(parts, padRight(issue.Key, m.keyColWidth))
		case "summary":
			parts = append(parts, padRight(components.TruncateEnd(issue.Summary, summaryWidth), summaryWidth))
		case fieldStatus:
			if currStatusIcon != "" {
				parts = append(parts, padRight(currStatusIcon, m.statusIconCols))
			} else {
				if selected {
					parts = append(parts, padRight(statusEmojiPlain(issue.Status), m.statusIconCols))
				} else {
					parts = append(parts, padRight(statusEmoji(issue.Status), m.statusIconCols))
				}
			}
		case "priority":
			if currPriorityIcon != "" {
				parts = append(parts, padRight(currPriorityIcon, m.priorityIconCols))
			} else {
				name := ""
				if issue.Priority != nil {
					name = issue.Priority.Name
				}
				parts = append(parts, padRight(components.TruncateEnd(name, 8), 8))
			}
		case "assignee":
			name := ""
			if issue.Assignee != nil {
				name = issue.Assignee.DisplayName
			}
			parts = append(parts, padRight(components.TruncateEnd(name, 12), 12))
		case "type":
			if currTypeIcon != "" {
				parts = append(parts, padRight(currTypeIcon, m.typeIconCols))
			} else {
				name := ""
				if issue.IssueType != nil {
					name = issue.IssueType.Name
				}
				parts = append(parts, padRight(components.TruncateEnd(name, 10), 10))
			}
		case "updated":
			parts = append(parts, padRight(issueTimeAgo(issue.Updated), 8))
		}
	}
	line := " " + strings.Join(parts, " ")
	if ansi.StringWidth(line) > width {
		line = ansi.Truncate(line, width, "")
	}

	if selected && m.Focused {
		return m.theme.SelectedItem.Width(width).Render(line)
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

// typeIcon returns the configured icon for the given issue type, or empty string if none.
func typeIcon(icons map[string]string, issueType *jira.IssueType) string {
	if issueType == nil {
		return ""
	}
	icon, ok := icons[issueType.Name]
	if !ok || icon == "" {
		return ""
	}
	return icon
}

// statusIcon returns the configured icon for the given status, or empty string if none.
func statusIcon(icons map[string]string, status *jira.Status) string {
	if status == nil {
		return ""
	}
	icon, ok := icons[status.Name]
	if !ok || icon == "" {
		return ""
	}
	return icon
}

// priorityIcon returns the configured icon for the given priority, or empty string if none.
func priorityIcon(icons map[string]string, priority *jira.Priority) string {
	if priority == nil {
		return ""
	}
	icon, ok := icons[priority.Name]
	if !ok || icon == "" {
		return ""
	}
	return icon
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
