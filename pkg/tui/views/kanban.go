package views

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

// UnassignedFilterID is the pseudo account id that matches issues with no
// assignee in the board filter.
const UnassignedFilterID = "__unassigned__"

const (
	minColumnWidth  = 18
	cardInnerMargin = 1 // one blank column between cards
)

// KanbanColumn is one rendered board column. A column can collect several
// statuses, which is how agile boards are actually configured.
type KanbanColumn struct {
	Name        string
	StatusIDs   []string
	CategoryKey string
	Issues      []*jira.Issue
}

// holds reports whether statusID belongs in this column.
func (c KanbanColumn) holds(statusID string) bool {
	return slices.Contains(c.StatusIDs, statusID)
}

// KanbanView renders a set of issues laid out in columns.
//
// The layout comes either from an agile board's own configuration or, when the
// project has no board, from its statuses. Filtering is client-side over the
// issues it was given.
type KanbanView struct {
	layout    []KanbanColumn // columns without their issues
	allIssues []jira.Issue
	columns   []KanbanColumn

	boardNames  []string
	activeBoard int

	col, row int   // cursor: column index, card index within the column
	scroll   []int // first visible card per column
	hOffset  int   // first visible column

	assignees   map[string]bool // empty means "everyone"
	filterLabel string

	// scope restricts the board to a set of issue keys, mirroring what the
	// issue list is showing. nil means no restriction; an empty non-nil set
	// legitimately means "the list shows nothing".
	scope map[string]bool

	projectKey    string
	width, height int
}

func NewKanbanView() *KanbanView {
	return &KanbanView{assignees: map[string]bool{}}
}

func (k *KanbanView) SetProjectKey(key string) { k.projectKey = key }
func (k *KanbanView) ProjectKey() string       { return k.projectKey }

func (k *KanbanView) SetSize(width, height int) {
	k.width, k.height = width, height
}

// SetData lays the board out one column per status and replaces the issues.
// Passing nil statuses derives the columns from the issues themselves, which
// is what happens for a project with no reachable status list.
func (k *KanbanView) SetData(statuses []jira.Status, issues []jira.Issue) {
	k.allIssues = issues
	k.SetStatuses(statuses)
}

// SetStatuses replaces the layout with one column per status, keeping the
// issues already loaded.
func (k *KanbanView) SetStatuses(statuses []jira.Status) {
	if len(statuses) == 0 {
		statuses = statusesFromIssues(k.allIssues)
	}
	layout := make([]KanbanColumn, 0, len(statuses))
	for _, s := range statuses {
		layout = append(layout, KanbanColumn{
			Name: s.Name, StatusIDs: []string{s.ID}, CategoryKey: s.CategoryKey,
		})
	}
	k.layout = layout
	k.rebuild()
}

// SetBoardLayout lays the board out with an agile board's own columns. The
// statuses are only consulted to colour each header by workflow category.
func (k *KanbanView) SetBoardLayout(columns []jira.BoardColumn, statuses []jira.Status) {
	category := make(map[string]string, len(statuses))
	for _, s := range statuses {
		category[s.ID] = s.CategoryKey
	}
	for i := range k.allIssues {
		if s := k.allIssues[i].Status; s != nil {
			if _, known := category[s.ID]; !known {
				category[s.ID] = s.CategoryKey
			}
		}
	}

	layout := make([]KanbanColumn, 0, len(columns))
	for _, col := range columns {
		rendered := KanbanColumn{Name: col.Name, StatusIDs: col.StatusIDs}
		for _, id := range col.StatusIDs {
			if key, ok := category[id]; ok && key != "" {
				rendered.CategoryKey = key
				break
			}
		}
		layout = append(layout, rendered)
	}
	k.layout = layout
	k.rebuild()
}

// SetIssues replaces the cards, keeping the current layout.
func (k *KanbanView) SetIssues(issues []jira.Issue) {
	k.allIssues = issues
	k.rebuild()
}

// SetScope restricts the board to the given issue keys, so that narrowing the
// issue list narrows the board with it. Pass nil to lift the restriction.
func (k *KanbanView) SetScope(keys map[string]bool) {
	k.scope = keys
	k.rebuild()
}

// SetBoardTabs names the boards of the project and which one is on screen, so
// the panel title can show them.
func (k *KanbanView) SetBoardTabs(names []string, active int) {
	k.boardNames = names
	k.activeBoard = active
}

// SetAssigneeFilter restricts the board to the given account ids. An empty set
// clears the filter. label is what the panel title shows.
func (k *KanbanView) SetAssigneeFilter(ids map[string]bool, label string) {
	k.assignees = ids
	if ids == nil {
		k.assignees = map[string]bool{}
	}
	k.filterLabel = label
	k.rebuild()
}

func (k *KanbanView) AssigneeFilter() map[string]bool { return k.assignees }
func (k *KanbanView) FilterLabel() string             { return k.filterLabel }
func (k *KanbanView) Columns() []KanbanColumn         { return k.columns }

// statusesFromIssues derives board columns from the issues at hand, preserving
// first-seen order within each workflow category.
func statusesFromIssues(issues []jira.Issue) []jira.Status {
	seen := make(map[string]bool)
	var out []jira.Status
	for i := range issues {
		s := issues[i].Status
		if s == nil || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		out = append(out, *s)
	}
	jira.SortStatuses(out)
	return out
}

func (k *KanbanView) rebuild() {
	k.columns = make([]KanbanColumn, len(k.layout))
	copy(k.columns, k.layout)
	for i := range k.columns {
		k.columns[i].Issues = nil
	}

	for i := range k.allIssues {
		issue := &k.allIssues[i]
		if issue.Status == nil || !k.matches(issue) {
			continue
		}
		for c := range k.columns {
			if k.columns[c].holds(issue.Status.ID) {
				k.columns[c].Issues = append(k.columns[c].Issues, issue)
				break
			}
		}
	}

	k.scroll = make([]int, len(k.columns))
	k.clampCursor()
}

func (k *KanbanView) matches(issue *jira.Issue) bool {
	if k.scope != nil && !k.scope[issue.Key] {
		return false
	}
	if len(k.assignees) == 0 {
		return true
	}
	if issue.Assignee == nil {
		return k.assignees[UnassignedFilterID]
	}
	return k.assignees[issue.Assignee.AccountID]
}

// clampCursor keeps the cursor on an existing card, preferring to stay in the
// current column and falling back to the first column that has any.
func (k *KanbanView) clampCursor() {
	if len(k.columns) == 0 {
		k.col, k.row = 0, 0
		return
	}
	k.col = min(max(k.col, 0), len(k.columns)-1)
	if len(k.columns[k.col].Issues) == 0 {
		for i, column := range k.columns {
			if len(column.Issues) > 0 {
				k.col = i
				break
			}
		}
	}
	k.row = min(max(k.row, 0), max(len(k.columns[k.col].Issues)-1, 0))
}

// Move walks the cursor by whole cards (dy) or columns (dx). Changing column
// keeps the vertical position when the target column is long enough.
func (k *KanbanView) Move(dx, dy int) {
	if len(k.columns) == 0 {
		return
	}
	if dx != 0 {
		k.col = min(max(k.col+dx, 0), len(k.columns)-1)
		k.row = min(k.row, max(len(k.columns[k.col].Issues)-1, 0))
	}
	if dy != 0 {
		k.row = min(max(k.row+dy, 0), max(len(k.columns[k.col].Issues)-1, 0))
	}
}

// SelectByKey moves the cursor to the card holding issueKey, if it is visible
// under the current filter.
func (k *KanbanView) SelectByKey(issueKey string) bool {
	for c, column := range k.columns {
		for r, issue := range column.Issues {
			if issue.Key == issueKey {
				k.col, k.row = c, r
				return true
			}
		}
	}
	return false
}

func (k *KanbanView) SelectedIssue() *jira.Issue {
	if k.col < 0 || k.col >= len(k.columns) {
		return nil
	}
	column := k.columns[k.col]
	if k.row < 0 || k.row >= len(column.Issues) {
		return nil
	}
	return column.Issues[k.row]
}

// Title is the panel heading: the project, its boards as tabs, and the active
// filter.
func (k *KanbanView) Title() string {
	title := "[0] Kanban"
	if k.projectKey != "" {
		title += ": " + k.projectKey
	}
	if len(k.boardNames) > 0 {
		marked := make([]string, 0, len(k.boardNames))
		for i, name := range k.boardNames {
			if i == k.activeBoard {
				name = "[" + name + "]"
			}
			marked = append(marked, name)
		}
		title += " " + strings.Join(marked, " ")
	}
	if k.filterLabel != "" {
		title += " — " + k.filterLabel
	}
	return title
}

// visibleColumnCount is how many columns fit side by side at the current width.
func (k *KanbanView) visibleColumnCount() int {
	if k.width <= 0 || len(k.columns) == 0 {
		return 0
	}
	fit := k.width / (minColumnWidth + cardInnerMargin)
	return min(max(fit, 1), len(k.columns))
}

func (k *KanbanView) View() string {
	if len(k.columns) == 0 {
		return theme.Default.Subtitle.Render("No statuses to show for this project")
	}

	visible := k.visibleColumnCount()
	k.hOffset = min(max(k.hOffset, 0), max(len(k.columns)-visible, 0))
	if k.col < k.hOffset {
		k.hOffset = k.col
	} else if k.col >= k.hOffset+visible {
		k.hOffset = k.col - visible + 1
	}

	colWidth := max((k.width-(visible-1)*cardInnerMargin)/max(visible, 1), minColumnWidth)
	cardHeight := max(k.height-1, 1) // one line goes to the column header

	rendered := make([]string, 0, visible)
	for i := k.hOffset; i < k.hOffset+visible && i < len(k.columns); i++ {
		rendered = append(rendered, k.renderColumn(i, colWidth, cardHeight))
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if len(k.columns) > visible {
		board += "\n" + theme.Default.Subtitle.Render(
			"columns "+strconv.Itoa(k.hOffset+1)+"-"+strconv.Itoa(k.hOffset+visible)+" of "+strconv.Itoa(len(k.columns)))
	}
	return board
}

func (k *KanbanView) renderColumn(index, width, height int) string {
	column := k.columns[index]
	inner := max(width-cardInnerMargin, 1)

	header := components.TruncateEnd(column.Name, max(inner-6, 4))
	header += " (" + strconv.Itoa(len(column.Issues)) + ")"
	headerStyle := theme.StatusColor(column.CategoryKey)
	if index == k.col {
		headerStyle = headerStyle.Bold(true)
	}

	lines := []string{headerStyle.Render(components.TruncateEnd(header, inner))}

	if len(column.Issues) == 0 {
		lines = append(lines, theme.Default.Subtitle.Render(components.TruncateEnd("—", inner)))
	}

	// Three lines of content plus a blank separator; scroll by whole cards so
	// none is cut in half.
	const cardLines = 4
	perScreen := max(height/cardLines, 1)
	if index < len(k.scroll) {
		if index == k.col {
			if k.row < k.scroll[index] {
				k.scroll[index] = k.row
			} else if k.row >= k.scroll[index]+perScreen {
				k.scroll[index] = k.row - perScreen + 1
			}
		}
		start := min(k.scroll[index], max(len(column.Issues)-1, 0))
		for row := start; row < len(column.Issues) && row < start+perScreen; row++ {
			selected := index == k.col && row == k.row
			lines = append(lines, k.renderCard(column.Issues[row], inner, selected)...)
		}
	}

	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).MaxHeight(height + 1).Render(body)
}

// renderCard draws one issue as exactly three lines: key + priority, summary,
// assignee. The selected card is marked with a leading bar because a
// background highlight is unreadable over the per-status colors.
func (k *KanbanView) renderCard(issue *jira.Issue, width int, selected bool) []string {
	marker, textWidth := "  ", width-2
	if selected {
		marker = theme.Default.Title.Render("▌") + " "
	}

	key := issue.Key
	if selected {
		key = theme.Default.Title.Render(key)
	} else {
		key = theme.Default.KeyStyle.Render(key)
	}
	head := key
	if issue.Priority != nil {
		head += " " + theme.PriorityStyled(components.TruncateEnd(issue.Priority.Name, 8))
	}

	summary := components.TruncateEnd(issue.Summary, textWidth)
	if !selected {
		summary = theme.Default.Subtitle.Render(summary)
	}

	who := "unassigned"
	if issue.Assignee != nil {
		who = issue.Assignee.DisplayName
	}
	who = theme.AuthorRender(components.TruncateEnd(who, textWidth))

	return []string{
		marker + components.TruncateEnd(head, textWidth),
		marker + summary,
		marker + who,
		"",
	}
}
