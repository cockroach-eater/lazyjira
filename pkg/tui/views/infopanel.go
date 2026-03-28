package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

// InfoPanelTab identifies a tab within the Info panel.
type InfoPanelTab int

const (
	InfoTabFields   InfoPanelTab = iota // field key-value pairs
	InfoTabLinks                        // issue links
	InfoTabSubtasks                     // subtasks
)

type InfoPanel struct {
	components.ListBase
	issue           *jira.Issue
	customFields    []config.CustomFieldConfig
	filter          string
	activeTab       InfoPanelTab
	theme           *theme.Theme
	filteredIndices []int // maps filtered cursor position → original data index; nil when no filter
}

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
		p.activeTab = InfoTabFields
	}
	p.syncItemCount()
}

// IssueKey returns the key of the currently displayed issue, or "".
func (p *InfoPanel) IssueKey() string {
	if p.issue != nil {
		return p.issue.Key
	}
	return ""
}

func (p *InfoPanel) SetCustomFields(fields []config.CustomFieldConfig) {
	p.customFields = fields
}

func (p *InfoPanel) SetFilter(query string) {
	p.filter = query
	p.filteredIndices = nil
}

// ClearFilter removes the search filter, preserving cursor on the same element.
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

func (p *InfoPanel) Fields() []InfoField {
	return buildInfoFields(p.issue, p.customFields)
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

// resolveOriginalIndex maps the current cursor to the original data index,
// accounting for active filter. Returns -1 if out of bounds.
func (p *InfoPanel) resolveOriginalIndex() int {
	if p.filteredIndices != nil {
		if p.Cursor >= 0 && p.Cursor < len(p.filteredIndices) {
			return p.filteredIndices[p.Cursor]
		}
		return -1
	}
	return p.Cursor
}

// SelectedLinkKey returns the issue key of the selected link, or "".
func (p *InfoPanel) SelectedLinkKey() string {
	if p.issue == nil || p.activeTab != InfoTabLinks {
		return ""
	}
	target := p.resolveOriginalIndex()
	if target < 0 {
		return ""
	}
	// Walk links matching renderLinkRowPairs order (one row per direction).
	idx := 0
	for _, link := range p.issue.IssueLinks {
		if link.Type == nil {
			continue
		}
		if link.OutwardIssue != nil {
			if idx == target {
				return link.OutwardIssue.Key
			}
			idx++
		}
		if link.InwardIssue != nil {
			if idx == target {
				return link.InwardIssue.Key
			}
			idx++
		}
	}
	return ""
}

// SelectedSubtaskKey returns the issue key of the selected subtask, or "".
func (p *InfoPanel) SelectedSubtaskKey() string {
	if p.issue == nil || p.activeTab != InfoTabSubtasks {
		return ""
	}
	idx := p.resolveOriginalIndex()
	if idx >= 0 && idx < len(p.issue.Subtasks) {
		return p.issue.Subtasks[idx].Key
	}
	return ""
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
		return infoFieldCount(p.issue, p.customFields)
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
		return len(p.issue.Subtasks)
	}
	return 0
}

func (p *InfoPanel) Init() tea.Cmd { return nil }

func (p *InfoPanel) Update(msg tea.Msg) (*InfoPanel, tea.Cmd) {
	if !p.Focused {
		return p, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		p.KeyNav(msg.String())
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

	// Apply filter (on plain text to avoid ANSI in search).
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

	// Clamp cursor.
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
			// Plain text for selected row — no inner ANSI to break background.
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

// ClickTabAt switches tab based on x position in the title bar.
func (p *InfoPanel) ClickTabAt(x int) {
	tabs := p.visibleTabs()
	if len(tabs) <= 1 {
		return
	}
	// Title format: "╭─[3] Info - Lnk - Sub─..."
	// Tabs start after "[3] " prefix (border char "╭─" is col 0-1).
	prefix := len("[3] ")
	sepW := 3 // " - "

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

// renderTabRows returns (styled, plain) row pairs for the active tab.
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
	w := p.Width - 2 // panel borders
	styled = renderInfoRows(p.issue, p.customFields, p.theme, w)
	plain = renderInfoRowsPlain(p.issue, p.customFields, w)
	return
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
	styled = make([]string, 0, len(p.issue.Subtasks))
	plain = make([]string, 0, len(p.issue.Subtasks))
	for _, sub := range p.issue.Subtasks {
		emoji := statusEmoji(sub.Status)
		emojiPlain := statusEmojiPlain(sub.Status)
		s := fmt.Sprintf(" %s %s: %s", emoji, sub.Key, sub.Summary)
		pl := fmt.Sprintf(" %s %s: %s", emojiPlain, sub.Key, sub.Summary)
		styled = append(styled, components.TruncateEnd(s, width-1))
		plain = append(plain, components.TruncateEnd(pl, width-1))
	}
	return
}
