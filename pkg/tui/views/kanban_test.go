package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
)

const (
	kbKey1 = "P-1"
	kbKey2 = "P-2"
	kbKey3 = "P-3"
	kbKey4 = "P-4"
)

var (
	kbTodo       = jira.Status{ID: "1", Name: "To Do", CategoryKey: "new"}
	kbInProgress = jira.Status{ID: "3", Name: "In Progress", CategoryKey: "indeterminate"}
	kbDone       = jira.Status{ID: "5", Name: "Done", CategoryKey: "done"}

	kbAlice = &jira.User{AccountID: "u1", DisplayName: "Alice"}
	kbBob   = &jira.User{AccountID: "u2", DisplayName: "Bob"}
)

func kbStatuses() []jira.Status {
	return []jira.Status{kbTodo, kbInProgress, kbDone}
}

func kbIssues() []jira.Issue {
	status := func(s jira.Status) *jira.Status { return &s }
	return []jira.Issue{
		{Key: kbKey1, Summary: "First", Status: status(kbTodo), Assignee: kbAlice},
		{Key: kbKey2, Summary: "Second", Status: status(kbTodo), Assignee: kbBob},
		{Key: kbKey3, Summary: "Third", Status: status(kbInProgress), Assignee: kbAlice},
		{Key: kbKey4, Summary: "Fourth", Status: status(kbDone)}, // unassigned
	}
}

func newTestKanban() *KanbanView {
	k := NewKanbanView()
	k.SetProjectKey("P")
	k.SetSize(120, 20)
	k.SetData(kbStatuses(), kbIssues())
	return k
}

func columnKeys(k *KanbanView, index int) []string {
	issues := k.Columns()[index].Issues
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, issue.Key)
	}
	return keys
}

func TestKanbanGroupsIssuesByStatus(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	if len(k.Columns()) != 3 {
		t.Fatalf("columns = %d, want 3", len(k.Columns()))
	}
	if got := columnKeys(k, 0); strings.Join(got, ",") != "P-1,P-2" {
		t.Errorf("To Do column = %v", got)
	}
	if got := columnKeys(k, 2); strings.Join(got, ",") != kbKey4 {
		t.Errorf("Done column = %v", got)
	}
}

func TestKanbanKeepsEmptyColumns(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(120, 20)
	k.SetData(kbStatuses(), []jira.Issue{{Key: kbKey1, Status: &jira.Status{ID: "1", Name: "To Do", CategoryKey: "new"}}})

	if len(k.Columns()) != 3 {
		t.Fatalf("columns = %d, want the full status set even when empty", len(k.Columns()))
	}
	if len(k.Columns()[1].Issues) != 0 {
		t.Errorf("In Progress should be empty, got %d", len(k.Columns()[1].Issues))
	}
}

func TestKanbanDerivesColumnsWhenStatusesMissing(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(120, 20)
	k.SetData(nil, kbIssues())

	if len(k.Columns()) != 3 {
		t.Fatalf("columns = %d, want 3 derived from the issues", len(k.Columns()))
	}
	want := []string{"To Do", "In Progress", "Done"}
	for i, name := range want {
		if k.Columns()[i].Name != name {
			t.Errorf("column %d = %q, want %q (category order)", i, k.Columns()[i].Name, name)
		}
	}
}

func TestKanbanAssigneeFilter(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	k.SetAssigneeFilter(map[string]bool{"u1": true}, "Alice")

	if got := columnKeys(k, 0); strings.Join(got, ",") != kbKey1 {
		t.Errorf("To Do column = %v, want only Alice's issue", got)
	}
	if got := columnKeys(k, 2); len(got) != 0 {
		t.Errorf("Done column = %v, want empty (P-4 is unassigned)", got)
	}
	if k.FilterLabel() != "Alice" {
		t.Errorf("FilterLabel = %q", k.FilterLabel())
	}
}

func TestKanbanFilterMatchesUnassigned(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	k.SetAssigneeFilter(map[string]bool{UnassignedFilterID: true}, "Unassigned")

	if got := columnKeys(k, 2); strings.Join(got, ",") != kbKey4 {
		t.Errorf("Done column = %v, want the unassigned issue", got)
	}
	if got := columnKeys(k, 0); len(got) != 0 {
		t.Errorf("To Do column = %v, want empty", got)
	}
}

func TestKanbanEmptyFilterShowsEverything(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	k.SetAssigneeFilter(map[string]bool{"u1": true}, "Alice")
	k.SetAssigneeFilter(nil, "")

	total := 0
	for i := range k.Columns() {
		total += len(k.Columns()[i].Issues)
	}
	if total != 4 {
		t.Errorf("visible issues = %d, want all 4 after clearing the filter", total)
	}
}

func TestKanbanCursorMovement(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	if got := k.SelectedIssue(); got == nil || got.Key != kbKey1 {
		t.Fatalf("initial selection = %v, want P-1", got)
	}
	k.Move(0, 1)
	if got := k.SelectedIssue(); got.Key != kbKey2 {
		t.Errorf("after down = %s, want P-2", got.Key)
	}
	k.Move(1, 0)
	if got := k.SelectedIssue(); got.Key != kbKey3 {
		t.Errorf("after right = %s, want P-3 (row clamped to the shorter column)", got.Key)
	}
	k.Move(1, 0)
	if got := k.SelectedIssue(); got.Key != kbKey4 {
		t.Errorf("after right = %s, want P-4", got.Key)
	}
	k.Move(1, 0) // past the last column
	if got := k.SelectedIssue(); got.Key != kbKey4 {
		t.Errorf("cursor escaped the board: %s", got.Key)
	}
	k.Move(-2, 0)
	if got := k.SelectedIssue(); got.Key != kbKey1 {
		t.Errorf("after two lefts = %s, want P-1", got.Key)
	}
	k.Move(0, -1) // above the first card
	if got := k.SelectedIssue(); got.Key != kbKey1 {
		t.Errorf("cursor escaped upwards: %s", got.Key)
	}
}

func TestKanbanCursorSkipsEmptyColumnOnRebuild(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	k.Move(2, 0) // Done column
	k.SetAssigneeFilter(map[string]bool{"u1": true}, "Alice")

	got := k.SelectedIssue()
	if got == nil {
		t.Fatal("selection is nil after the filter emptied the current column")
	}
	if got.Key != kbKey1 {
		t.Errorf("selection = %s, want the first column that still has cards", got.Key)
	}
}

func TestKanbanSelectedIssueNilWhenBoardEmpty(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(80, 20)
	k.SetData(kbStatuses(), nil)

	if got := k.SelectedIssue(); got != nil {
		t.Errorf("SelectedIssue = %v, want nil on an empty board", got)
	}
	if view := k.View(); view == "" {
		t.Error("View is empty; an empty board should still render its columns")
	}
}

func TestKanbanSelectByKey(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	if !k.SelectByKey(kbKey3) {
		t.Fatal("SelectByKey(P-3) = false")
	}
	if got := k.SelectedIssue(); got.Key != kbKey3 {
		t.Errorf("selection = %s", got.Key)
	}
	if k.SelectByKey("NOPE-1") {
		t.Error("SelectByKey matched an issue that is not on the board")
	}
}

func TestKanbanViewRendersStatusesAndCards(t *testing.T) {
	t.Parallel()
	view := newTestKanban().View()

	for _, want := range []string{"To Do", "In Progress", "Done", kbKey1, "First", "Alice"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "(2)") {
		t.Errorf("view is missing the To Do card count:\n%s", view)
	}
}

func TestKanbanViewNarrowTerminalShowsSubsetOfColumns(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	k.SetSize(40, 20) // room for two columns at most

	view := k.View()
	if !strings.Contains(view, "of 3") {
		t.Errorf("narrow view should report hidden columns:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 40 {
			t.Errorf("line overflows the 40-column panel (%d columns): %q", got, line)
		}
	}
}

func TestKanbanViewScrollsColumnsToFollowCursor(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	k.SetSize(40, 20)

	k.Move(2, 0) // Done is the third column, off-screen at this width
	view := k.View()
	if !strings.Contains(view, "Done") {
		t.Errorf("board did not scroll to the selected column:\n%s", view)
	}
}

func TestKanbanTitleIncludesProjectAndFilter(t *testing.T) {
	t.Parallel()
	k := newTestKanban()
	if got := k.Title(); got != "[0] Kanban: P" {
		t.Errorf("Title = %q", got)
	}
	k.SetAssigneeFilter(map[string]bool{"u1": true}, "Alice")
	if got := k.Title(); !strings.Contains(got, "Alice") {
		t.Errorf("Title = %q, want the filter label", got)
	}
}

func TestBoardLayoutGroupsSeveralStatusesInOneColumn(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(120, 20)
	k.SetData(kbStatuses(), kbIssues())

	k.SetBoardLayout([]jira.BoardColumn{
		{Name: "Backlog", StatusIDs: []string{"1"}},
		{Name: "Doing", StatusIDs: []string{"3", "5"}},
	}, kbStatuses())

	cols := k.Columns()
	if len(cols) != 2 {
		t.Fatalf("columns = %d, want the board's own two", len(cols))
	}
	if cols[0].Name != "Backlog" || len(cols[0].Issues) != 2 {
		t.Errorf("Backlog = %q with %d cards", cols[0].Name, len(cols[0].Issues))
	}
	// In Progress and Done collapse into one column.
	if cols[1].Name != "Doing" || len(cols[1].Issues) != 2 {
		t.Errorf("Doing = %q with %d cards, want the two merged statuses", cols[1].Name, len(cols[1].Issues))
	}
}

func TestBoardLayoutColoursHeadersFromTheStatusCategory(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(120, 20)
	k.SetData(kbStatuses(), kbIssues())

	k.SetBoardLayout([]jira.BoardColumn{{Name: "Shipped", StatusIDs: []string{"5"}}}, kbStatuses())

	if got := k.Columns()[0].CategoryKey; got != "done" {
		t.Errorf("CategoryKey = %q, want the category of the mapped status", got)
	}
}

func TestBoardLayoutDropsIssuesOutsideItsColumns(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(120, 20)
	k.SetData(kbStatuses(), kbIssues())

	// A board that only maps To Do leaves the other issues off the board, as
	// Jira does for issues outside the board's columns.
	k.SetBoardLayout([]jira.BoardColumn{{Name: "Only todo", StatusIDs: []string{"1"}}}, kbStatuses())

	total := 0
	for _, c := range k.Columns() {
		total += len(c.Issues)
	}
	if total != 2 {
		t.Errorf("cards = %d, want only the two mapped ones", total)
	}
}

func TestSetIssuesKeepsTheBoardLayout(t *testing.T) {
	t.Parallel()
	k := NewKanbanView()
	k.SetSize(120, 20)
	k.SetData(kbStatuses(), kbIssues())
	k.SetBoardLayout([]jira.BoardColumn{{Name: "Doing", StatusIDs: []string{"3", "5"}}}, kbStatuses())

	k.SetIssues(kbIssues())

	cols := k.Columns()
	if len(cols) != 1 || cols[0].Name != "Doing" {
		t.Errorf("columns = %+v, want the layout preserved across new cards", cols)
	}
}

func TestBoardTabsAppearInTheTitle(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	k.SetBoardTabs([]string{"Delivery", "Bugs"}, 1)

	got := k.Title()
	if !strings.Contains(got, "Delivery") || !strings.Contains(got, "[Bugs]") {
		t.Errorf("Title = %q, want both boards with the active one marked", got)
	}
}

func TestSingleBoardIsNotShownAsTabs(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	k.SetBoardTabs(nil, 0)

	if got := k.Title(); got != "[0] Kanban: P" {
		t.Errorf("Title = %q, want no tab strip for a single board", got)
	}
}

func TestScopeRestrictsTheBoardToTheGivenKeys(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	k.SetScope(map[string]bool{kbKey1: true, kbKey3: true})

	total := 0
	for _, c := range k.Columns() {
		total += len(c.Issues)
	}
	if total != 2 {
		t.Errorf("cards = %d, want only the two scoped keys", total)
	}
	if got := columnKeys(k, 0); strings.Join(got, ",") != kbKey1 {
		t.Errorf("To Do column = %v", got)
	}
}

// nil means "no restriction"; an empty non-nil set means the list is showing
// nothing, and the board has to agree.
func TestNilScopeIsNotAnEmptyScope(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	k.SetScope(map[string]bool{})
	total := 0
	for _, c := range k.Columns() {
		total += len(c.Issues)
	}
	if total != 0 {
		t.Errorf("cards = %d, want none for an empty scope", total)
	}

	k.SetScope(nil)
	total = 0
	for _, c := range k.Columns() {
		total += len(c.Issues)
	}
	if total != 4 {
		t.Errorf("cards = %d, want every card back once the scope is lifted", total)
	}
}

func TestScopeAndAssigneeFilterCompose(t *testing.T) {
	t.Parallel()
	k := newTestKanban()

	k.SetScope(map[string]bool{kbKey1: true, kbKey2: true})
	k.SetAssigneeFilter(map[string]bool{"u1": true}, "Alice")

	total := 0
	for _, c := range k.Columns() {
		total += len(c.Issues)
	}
	if total != 1 {
		t.Errorf("cards = %d, want the intersection of scope and assignee", total)
	}
}
