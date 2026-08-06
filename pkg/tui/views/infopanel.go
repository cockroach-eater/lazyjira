package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

// PreviewRequestMsg is dispatched by InfoPanel when the cursor moves to a
// different issue in the Sub or Lnk tab, requesting a detail preview fetch.
type PreviewRequestMsg struct{ Key string }

// ChildrenRequestMsg is dispatched when the Sub tab becomes active for a
// Cloud issue and the children for its key have not been loaded yet. The
// app handles it by calling jira.Client.GetChildren and feeding the result
// back via InfoPanel.SetChildren / SetChildrenError.
type ChildrenRequestMsg struct{ Key string }

// InfoPanelTab identifies a tab within the Info panel
type InfoPanelTab int

const (
	InfoTabFields InfoPanelTab = iota
	InfoTabLinks
	InfoTabSubtasks
)

type InfoPanel struct {
	components.ListBase
	issue  *jira.Issue
	fields []config.FieldConfig
	// reviewerField is the custom field id configured as the reviewer, or
	// empty. Jira has no standard reviewer field, so it cannot be a builtin.
	reviewerField string
	// branch is the local git branch matching branchFor. It does not come
	// from Jira, so the app feeds it in with SetBranch.
	branch    string
	branchFor string
	// showBranch stays false outside a git repository, where the row would
	// be permanently empty.
	showBranch      bool
	filter          string
	activeTab       InfoPanelTab
	theme           *theme.Theme
	filteredIndices []int

	// cloud toggles the Cloud parent-link Children path. When true, the
	// Sub tab renders children loaded via SetChildren; when false, it
	// falls back to issue.Subtasks.
	cloud bool
	// Children state for the Cloud path. childrenForKey identifies which
	// issue's children currently sit in children. childrenLoaded is the
	// "we have an answer" flag (an empty slice still counts as loaded).
	// childrenError carries the last fetch error, if any.
	childrenForKey string
	children       []jira.Issue
	childrenLoaded bool
	childrenError  string

	typeIcons map[string]string
}

func (p *InfoPanel) SetTypeIcons(icons map[string]string) { p.typeIcons = icons }

func NewInfoPanel() *InfoPanel {
	return &InfoPanel{theme: theme.Default}
}

func (p *InfoPanel) SetIssue(issue *jira.Issue) {
	prevKey := ""
	if p.issue != nil {
		prevKey = p.issue.Key
	}
	p.issue = issue
	if issue == nil || issue.Key != prevKey {
		p.Cursor = 0
		p.Offset = 0
		p.resetChildren()
	}
	p.syncItemCount()
}

// SetCloud toggles the Cloud parent-link Children rendering path.
func (p *InfoPanel) SetCloud(cloud bool) { p.cloud = cloud }

// SetChildren stores children for parentKey. If parentKey doesn't match the
// currently displayed issue, the call is dropped (stale response).
func (p *InfoPanel) SetChildren(parentKey string, issues []jira.Issue) {
	if p.issue == nil || p.issue.Key != parentKey {
		return
	}
	p.childrenForKey = parentKey
	p.children = issues
	p.childrenLoaded = true
	p.childrenError = ""
	p.syncItemCount()
}

// SetChildrenError records a fetch error for parentKey. Stale errors (key
// mismatch) are dropped.
func (p *InfoPanel) SetChildrenError(parentKey, msg string) {
	if p.issue == nil || p.issue.Key != parentKey {
		return
	}
	p.childrenForKey = parentKey
	p.children = nil
	p.childrenLoaded = false
	p.childrenError = msg
	p.syncItemCount()
}

// MaybeChildrenRequest returns a Cmd that emits ChildrenRequestMsg when the
// Sub tab is active on a Cloud issue and the children have neither been
// loaded nor failed for the current key. Returns nil otherwise.
func (p *InfoPanel) MaybeChildrenRequest() tea.Cmd {
	if !p.cloud || p.issue == nil {
		return nil
	}
	if p.activeTab != InfoTabSubtasks {
		return nil
	}
	if p.childrenForKey == p.issue.Key && (p.childrenLoaded || p.childrenError != "") {
		return nil
	}
	key := p.issue.Key
	return func() tea.Msg { return ChildrenRequestMsg{Key: key} }
}

// Children exposes the loaded Cloud children for tests and callers that need
// the resolved sub-tab list.
func (p *InfoPanel) Children() []jira.Issue { return p.children }

func (p *InfoPanel) resetChildren() {
	p.childrenForKey = ""
	p.children = nil
	p.childrenLoaded = false
	p.childrenError = ""
}

// IssueKey returns the key of the currently displayed issue
func (p *InfoPanel) IssueKey() string {
	if p.issue != nil {
		return p.issue.Key
	}
	return ""
}

func (p *InfoPanel) SetFields(fields []config.FieldConfig) {
	p.fields = fields
}

// SetReviewerField configures the custom field shown as Reviewer.
func (p *InfoPanel) SetReviewerField(fieldID string) {
	p.reviewerField = fieldID
}

// EnableBranchField makes the Branch row appear. Only meaningful inside a git
// repository.
func (p *InfoPanel) EnableBranchField(enabled bool) {
	p.showBranch = enabled
	p.syncItemCount()
}

// SetBranch records the branch found for issueKey. A stale answer (the cursor
// already moved on) is dropped.
func (p *InfoPanel) SetBranch(issueKey, branch string) {
	if p.issue == nil || p.issue.Key != issueKey {
		return
	}
	p.branchFor = issueKey
	p.branch = branch
	p.syncItemCount()
}

// extraFields bundles the non-Jira rows for the issue on screen.
func (p *InfoPanel) extraFields() extraInfoFields {
	return extraInfoFields{
		reviewerID: p.reviewerField,
		branch:     p.branchValue(),
		showBranch: p.showBranch,
	}
}

// branchValue is the Branch row content for the issue on screen.
func (p *InfoPanel) branchValue() string {
	if p.issue == nil || p.branchFor != p.issue.Key {
		return ""
	}
	return p.branch
}

func (p *InfoPanel) SetFilter(query string) {
	p.filter = query
	p.filteredIndices = nil
}

// ClearFilter removes the search filter preserving cursor on the same element
func (p *InfoPanel) ClearFilter() {
	origIdx := p.resolveOriginalIndex()
	p.filter = ""
	p.filteredIndices = nil
	if origIdx >= 0 {
		p.Cursor = origIdx
		p.AdjustOffset()
	}
}

func (p *InfoPanel) Issue() *jira.Issue {
	return p.issue
}

func (p *InfoPanel) ActiveTab() InfoPanelTab {
	return p.activeTab
}

// Does not reset Cursor/Offset; callers must do so themselves if needed.
func (p *InfoPanel) SetActiveTab(tab InfoPanelTab) {
	p.activeTab = tab
	p.syncItemCount()
}

func (p *InfoPanel) Fields() []InfoField {
	return buildInfoFields(p.issue, p.fields, p.extraFields())
}

func (p *InfoPanel) SelectedInfoField() *InfoField {
	if p.issue == nil || p.activeTab != InfoTabFields {
		return nil
	}
	fields := p.Fields()
	idx := p.resolveOriginalIndex()
	if idx >= 0 && idx < len(fields) {
		return &fields[idx]
	}
	return nil
}

func (p *InfoPanel) resolveOriginalIndex() int {
	if p.filteredIndices != nil {
		if p.Cursor >= 0 && p.Cursor < len(p.filteredIndices) {
			return p.filteredIndices[p.Cursor]
		}
		return -1
	}
	return p.Cursor
}

// SelectedLinkKey returns the issue key of the selected link
func (p *InfoPanel) SelectedLinkKey() string {
	if iss := p.SelectedLinkIssue(); iss != nil {
		return iss.Key
	}
	return ""
}

// SelectedLinkIssue returns the linked issue currently under cursor (with
// whatever fields the parent payload carried, typically key + summary +
// status), or nil when no link is selected.
func (p *InfoPanel) SelectedLinkIssue() *jira.Issue {
	_, issue := p.selectedLink()
	return issue
}

// SelectedLink returns the whole link under the cursor, which carries the id
// needed to delete it.
func (p *InfoPanel) SelectedLink() *jira.IssueLink {
	link, _ := p.selectedLink()
	return link
}

// selectedLink walks the rendered link rows -- one per direction, since a link
// can name an issue on either side -- and returns the row under the cursor.
func (p *InfoPanel) selectedLink() (*jira.IssueLink, *jira.Issue) {
	if p.issue == nil || p.activeTab != InfoTabLinks {
		return nil, nil
	}
	target := p.resolveOriginalIndex()
	if target < 0 {
		return nil, nil
	}
	idx := 0
	for i := range p.issue.IssueLinks {
		link := &p.issue.IssueLinks[i]
		if link.Type == nil {
			continue
		}
		if link.OutwardIssue != nil {
			if idx == target {
				return link, link.OutwardIssue
			}
			idx++
		}
		if link.InwardIssue != nil {
			if idx == target {
				return link, link.InwardIssue
			}
			idx++
		}
	}
	return nil, nil
}

// SelectedSubtaskKey returns the issue key of the selected subtask
func (p *InfoPanel) SelectedSubtaskKey() string {
	if iss := p.SelectedSubtaskIssue(); iss != nil {
		return iss.Key
	}
	return ""
}

// SelectedSubtaskIssue returns the subtask under cursor (with whatever
// fields the parent payload carried, typically key + summary + status),
// or nil when no subtask is selected.
func (p *InfoPanel) SelectedSubtaskIssue() *jira.Issue {
	if p.issue == nil || p.activeTab != InfoTabSubtasks {
		return nil
	}
	idx := p.resolveOriginalIndex()
	subs := p.subtaskList()
	if idx >= 0 && idx < len(subs) {
		return &subs[idx]
	}
	return nil
}

// subtaskList returns the list backing the Sub tab: Cloud children when
// loaded, otherwise the legacy Subtasks slice.
func (p *InfoPanel) subtaskList() []jira.Issue {
	if p.cloud && p.childrenLoaded && p.childrenForKey != "" && p.issue != nil && p.childrenForKey == p.issue.Key {
		return p.children
	}
	if p.issue == nil {
		return nil
	}
	return p.issue.Subtasks
}

func (p *InfoPanel) ContentHeight() int {
	return p.ListBase.ContentHeight(3)
}

func (p *InfoPanel) NextTab() {
	tabs := p.visibleTabs()
	for i, t := range tabs {
		if t == p.activeTab {
			p.activeTab = tabs[(i+1)%len(tabs)]
			p.Cursor = 0
			p.Offset = 0
			p.syncItemCount()
			return
		}
	}
}

func (p *InfoPanel) PrevTab() {
	tabs := p.visibleTabs()
	for i, t := range tabs {
		if t == p.activeTab {
			p.activeTab = tabs[(i+len(tabs)-1)%len(tabs)]
			p.Cursor = 0
			p.Offset = 0
			p.syncItemCount()
			return
		}
	}
}

func (p *InfoPanel) visibleTabs() []InfoPanelTab {
	return []InfoPanelTab{InfoTabFields, InfoTabLinks, InfoTabSubtasks}
}

func (p *InfoPanel) syncItemCount() {
	p.SetItemCount(p.tabItemCount())
}

func (p *InfoPanel) tabItemCount() int {
	if p.issue == nil {
		return 0
	}
	switch p.activeTab {
	case InfoTabFields:
		return infoFieldCount(p.issue, p.fields, p.extraFields())
	case InfoTabLinks:
		count := 0
		for _, link := range p.issue.IssueLinks {
			if link.Type == nil {
				continue
			}
			if link.OutwardIssue != nil {
				count++
			}
			if link.InwardIssue != nil {
				count++
			}
		}
		return count
	case InfoTabSubtasks:
		return len(p.subtaskList())
	}
	return 0
}

func (p *InfoPanel) Init() tea.Cmd { return nil }

func (p *InfoPanel) Update(msg tea.Msg) (*InfoPanel, tea.Cmd) {
	if !p.Focused {
		return p, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		moved := p.KeyNav(msg.String())
		if moved {
			switch p.activeTab {
			case InfoTabSubtasks:
				if key := p.SelectedSubtaskKey(); key != "" {
					return p, func() tea.Msg { return PreviewRequestMsg{Key: key} }
				}
			case InfoTabLinks:
				if key := p.SelectedLinkKey(); key != "" {
					return p, func() tea.Msg { return PreviewRequestMsg{Key: key} }
				}
			case InfoTabFields:
				// no preview dispatch on cursor move in fields tab
			}
		}
	}
	return p, nil
}

func (p *InfoPanel) View() string {
	if p.Height <= 1 {
		footer := ""
		if count := p.tabItemCount(); count > 0 {
			footer = fmt.Sprintf("%d of %d", p.Cursor+1, count)
		}
		return components.RenderCollapsedBar("[3] Info", footer, p.Width, p.Focused)
	}

	contentWidth, innerHeight := components.PanelDimensions(p.Width, p.Height)

	if p.issue == nil {
		placeholder := lipgloss.NewStyle().Foreground(theme.ColorGray).Render("No issue selected")
		return components.RenderPanel("[3] Info", placeholder, p.Width, innerHeight, p.Focused)
	}

	title := p.buildTitle()

	styled, plain := p.renderTabRows(contentWidth)

	if p.filter != "" {
		q := strings.ToLower(p.filter)
		var fs, fp []string
		var indices []int
		for i, row := range plain {
			if strings.Contains(strings.ToLower(row), q) {
				fs = append(fs, styled[i])
				fp = append(fp, row)
				indices = append(indices, i)
			}
		}
		styled, plain = fs, fp
		p.filteredIndices = indices
	} else {
		p.filteredIndices = nil
	}

	if p.Cursor >= len(plain) {
		p.Cursor = len(plain) - 1
	}
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	p.SetItemCount(len(plain))
	p.AdjustOffset()

	var rendered []string
	end := min(p.Offset+innerHeight, len(plain))
	for i := p.Offset; i < end; i++ {
		if i == p.Cursor && p.Focused {
			rendered = append(rendered, p.theme.SelectedItem.Width(contentWidth).Render(plain[i]))
		} else {
			rendered = append(rendered, p.theme.NormalItem.Width(contentWidth).Render(styled[i]))
		}
	}

	content := strings.Join(rendered, "\n")
	footer := ""
	if len(plain) > 0 {
		footer = fmt.Sprintf("%d of %d", p.Cursor+1, len(plain))
	}
	scroll := &components.ScrollInfo{Total: len(plain), Visible: innerHeight, Offset: p.Offset}
	return components.RenderPanelFull(title, footer, content, p.Width, innerHeight, p.Focused, scroll)
}

func (p *InfoPanel) buildTitle() string {
	tabs := p.visibleTabs()
	if len(tabs) <= 1 {
		return "[3] Info"
	}

	activeStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	sepStyle := lipgloss.NewStyle().Foreground(theme.ColorGray)
	sep := sepStyle.Render(" - ")

	var parts []string
	for _, t := range tabs {
		label := infoPanelTabLabel(t)
		if t == p.activeTab {
			parts = append(parts, activeStyle.Render(label))
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
	}

	return "[3] " + strings.Join(parts, sep)
}

// ClickTabAt switches tab based on x position in the title bar
func (p *InfoPanel) ClickTabAt(x int) {
	tabs := p.visibleTabs()
	if len(tabs) <= 1 {
		return
	}
	prefix := len("[3] ")
	sepW := 3

	pos := prefix
	for _, t := range tabs {
		label := infoPanelTabLabel(t)
		labelW := len(label)
		if x >= pos && x < pos+labelW+sepW {
			if t != p.activeTab {
				p.activeTab = t
				p.Cursor = 0
				p.Offset = 0
				p.syncItemCount()
			}
			return
		}
		pos += labelW + sepW
	}
}

func infoPanelTabLabel(tab InfoPanelTab) string {
	switch tab {
	case InfoTabFields:
		return "Info"
	case InfoTabLinks:
		return "Lnk"
	case InfoTabSubtasks:
		return "Sub"
	}
	return ""
}

func (p *InfoPanel) renderTabRows(width int) (styled, plain []string) {
	switch p.activeTab {
	case InfoTabFields:
		return p.renderFieldRowPairs()
	case InfoTabLinks:
		return p.renderLinkRowPairs(width)
	case InfoTabSubtasks:
		return p.renderSubtaskRowPairs(width)
	}
	return nil, nil
}

func (p *InfoPanel) renderFieldRowPairs() (styled, plain []string) {
	return renderInfoRowPairs(p.issue, p.fields, p.extraFields(), p.theme, p.Width-2)
}

func (p *InfoPanel) renderLinkRowPairs(width int) (styled, plain []string) {
	keyStyle := p.theme.KeyStyle
	valStyle := p.theme.ValueStyle
	for _, link := range p.issue.IssueLinks {
		if link.Type == nil {
			continue
		}
		if link.OutwardIssue != nil {
			s := " " + keyStyle.Render(link.Type.Outward) + " " +
				valStyle.Render(fmt.Sprintf("%s: %s", link.OutwardIssue.Key, link.OutwardIssue.Summary))
			pl := fmt.Sprintf(" %s %s: %s", link.Type.Outward, link.OutwardIssue.Key, link.OutwardIssue.Summary)
			styled = append(styled, components.TruncateEnd(s, width-1))
			plain = append(plain, components.TruncateEnd(pl, width-1))
		}
		if link.InwardIssue != nil {
			s := " " + keyStyle.Render(link.Type.Inward) + " " +
				valStyle.Render(fmt.Sprintf("%s: %s", link.InwardIssue.Key, link.InwardIssue.Summary))
			pl := fmt.Sprintf(" %s %s: %s", link.Type.Inward, link.InwardIssue.Key, link.InwardIssue.Summary)
			styled = append(styled, components.TruncateEnd(s, width-1))
			plain = append(plain, components.TruncateEnd(pl, width-1))
		}
	}
	return
}

func (p *InfoPanel) renderSubtaskRowPairs(width int) (styled, plain []string) {
	subs := p.subtaskList()
	if p.cloud && p.childrenForKey == p.issue.Key {
		if p.childrenError != "" {
			pl := " Failed to load children: " + p.childrenError
			s := lipgloss.NewStyle().Foreground(theme.ColorRed).Render(pl)
			return []string{components.TruncateEnd(s, width-1)}, []string{components.TruncateEnd(pl, width-1)}
		}
		if p.childrenLoaded && len(subs) == 0 {
			pl := " No children"
			s := lipgloss.NewStyle().Foreground(theme.ColorGray).Render(pl)
			return []string{components.TruncateEnd(s, width-1)}, []string{components.TruncateEnd(pl, width-1)}
		}
	}
	styled = make([]string, 0, len(subs))
	plain = make([]string, 0, len(subs))
	for _, sub := range subs {
		emoji := statusEmoji(sub.Status)
		emojiPlain := statusEmojiPlain(sub.Status)
		marker := p.issueTypeMarker(sub.IssueType)
		s := fmt.Sprintf(" %s %s%s: %s", emoji, marker, sub.Key, sub.Summary)
		pl := fmt.Sprintf(" %s %s%s: %s", emojiPlain, marker, sub.Key, sub.Summary)
		styled = append(styled, components.TruncateEnd(s, width-1))
		plain = append(plain, components.TruncateEnd(pl, width-1))
	}
	return
}

func (p *InfoPanel) issueTypeMarker(t *jira.IssueType) string {
	if t == nil || t.Name == "" {
		return ""
	}
	if icon := typeIcon(p.typeIcons, t); icon != "" {
		return icon + " "
	}
	return "[" + t.Name + "] "
}
