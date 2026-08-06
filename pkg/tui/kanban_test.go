package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

var errBoardTest = errors.New("board unavailable")

const (
	boardKey1 = testProject + "-1"
	boardKey2 = testProject + "-2"
	aliceName = "Alice"
)

var (
	kbStatusTodo = jira.Status{ID: "1", Name: "To Do", CategoryKey: "new"}
	kbStatusDone = jira.Status{ID: "5", Name: "Done", CategoryKey: "done"}
	kbUserAlice  = &jira.User{AccountID: "u1", DisplayName: aliceName}
	kbUserBob    = &jira.User{AccountID: "u2", DisplayName: "Bob"}
)

func kbBoardIssues() []jira.Issue {
	todo, done := kbStatusTodo, kbStatusDone
	return []jira.Issue{
		{Key: boardKey1, Summary: "First", Status: &todo, Assignee: kbUserAlice},
		{Key: boardKey2, Summary: "Second", Status: &done, Assignee: kbUserBob},
	}
}

// newBoardApp returns an app with a project selected and issues loaded, which
// is the state in which the board is meant to be visible.
func newBoardApp(t *testing.T) *App {
	t.Helper()
	app := newTestApp()
	app.client = &jiratest.FakeClient{T: t}
	app.infoPanel = views.NewInfoPanel()
	app.statusPanel = views.NewStatusPanel(testProject, "", "")
	app.logPanel = views.NewLogPanel()
	app.keymap = DefaultKeymap()
	logFlag := false
	app.logFlag = &logFlag
	app.projectStatuses = map[string][]jira.Status{testProject: {kbStatusTodo, kbStatusDone}}
	app.activeBoard = -1
	app.boardColumns = map[int][]jira.BoardColumn{}
	app.boardIssues = map[int][]jira.Issue{}
	app.boardFilters = map[string]map[string]bool{}
	app.boardFilterLabels = map[string]string{}
	app.usersCache = map[string][]jira.User{}
	app.issueCache = map[string]*jira.Issue{}
	app.projectKey = testProject
	app.issuesList.SetIssues(kbBoardIssues())
	app.detailView.SetSize(100, 20)
	app.showBoard()
	return app
}

func TestShowBoardPutsKanbanOnTheDetailPanel(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	if app.detailView.Mode() != views.ModeKanban {
		t.Fatalf("mode = %v, want ModeKanban", app.detailView.Mode())
	}
	if !app.boardVisible() {
		t.Error("boardVisible = false right after showBoard")
	}
	if got := len(app.detailView.Kanban().Columns()); got != 2 {
		t.Errorf("columns = %d, want 2", got)
	}
}

func TestShowBoardIsNoOpWithoutProject(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.projectKey = ""
	app.detailView.SetIssue(&jira.Issue{Key: testKey})

	if cmd := app.showBoard(); cmd != nil {
		t.Error("showBoard fetched statuses with no project selected")
	}
	if app.detailView.Mode() != views.ModeIssue {
		t.Errorf("mode = %v, want the panel left untouched", app.detailView.Mode())
	}
}

func TestShowBoardFetchesStatusesOnlyWhenNotCached(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	if cmd := app.showBoard(); cmd != nil {
		t.Error("showBoard refetched statuses that were already cached")
	}

	delete(app.projectStatuses, testProject)
	if cmd := app.showBoard(); cmd == nil {
		t.Error("showBoard did not fetch the missing statuses")
	}
}

func TestProjectStatusesLoadedFallsBackOnFailure(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	delete(app.projectStatuses, testProject)
	app.showBoard() // rebuilds the board with statuses derived from the issues

	app.handleProjectStatusesLoaded(projectStatusesLoadedMsg{projectKey: testProject})

	if _, cached := app.projectStatuses[testProject]; cached {
		t.Error("an empty response should not be cached as the status list")
	}
	if got := len(app.detailView.Kanban().Columns()); got != 2 {
		t.Errorf("columns = %d, want 2 derived from the loaded issues", got)
	}
}

func TestProjectStatusesLoadedUpdatesVisibleBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	extra := jira.Status{ID: "3", Name: "In Progress", CategoryKey: "indeterminate"}

	app.handleProjectStatusesLoaded(projectStatusesLoadedMsg{
		projectKey: testProject,
		statuses:   []jira.Status{kbStatusTodo, extra, kbStatusDone},
	})

	if got := len(app.detailView.Kanban().Columns()); got != 3 {
		t.Errorf("columns = %d, want the refreshed status list", got)
	}
}

func TestOpenIssueDetailClosesTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.issuesList.SelectByKey(boardKey1)

	app.openIssueDetail()

	if !app.taskOpen {
		t.Error("taskOpen = false after opening a task")
	}
	if app.detailView.Mode() != views.ModeIssue {
		t.Errorf("mode = %v, want ModeIssue", app.detailView.Mode())
	}
	if app.boardVisible() {
		t.Error("board is still considered visible with a task open")
	}
}

func TestEscFromTaskReturnsToTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.issuesList.SelectByKey(boardKey1)
	app.openIssueDetail()

	_, _, handled := app.handleFocusAction(ActFocusLeft)

	if !handled {
		t.Fatal("ActFocusLeft was not handled from the right panel")
	}
	if app.taskOpen {
		t.Error("taskOpen is still true after closing the task")
	}
	if app.detailView.Mode() != views.ModeKanban {
		t.Errorf("mode = %v, want the board back", app.detailView.Mode())
	}
	if app.side != sideLeft {
		t.Error("focus did not return to the left column")
	}
}

func TestBoardSurvivesListNavigation(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.issueCache[boardKey2] = &jira.Issue{Key: boardKey2, Summary: "Second"}

	app.Update(views.PreviewRequestMsg{Key: boardKey2})

	if app.detailView.Mode() != views.ModeKanban {
		t.Errorf("mode = %v; moving the list cursor must not replace the board", app.detailView.Mode())
	}
	if got := app.detailView.Kanban().SelectedIssue(); got == nil || got.Key != boardKey2 {
		t.Errorf("board cursor = %v, want it to follow the list selection", got)
	}
}

func TestPreviewDetailLoadedDoesNotReplaceTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.previewEpoch = 7

	app.Update(previewDetailLoadedMsg{
		issue: &jira.Issue{Key: boardKey1, Summary: "First", Description: "body"},
		epoch: 7,
	})

	if app.detailView.Mode() != views.ModeKanban {
		t.Errorf("mode = %v; a background fetch must not replace the board", app.detailView.Mode())
	}
	if _, cached := app.issueCache[boardKey1]; !cached {
		t.Error("the fetched issue was not cached")
	}
}

func TestBoardKeysMoveTheCursor(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.side = sideRight

	if got := app.detailView.Kanban().SelectedIssue(); got.Key != boardKey1 {
		t.Fatalf("initial card = %s", got.Key)
	}

	_, _, handled := app.handleBoardKeys(ActFocusRight, "l")
	if !handled {
		t.Fatal("ActFocusRight was not handled by the board")
	}
	if got := app.detailView.Kanban().SelectedIssue(); got.Key != boardKey2 {
		t.Errorf("card after moving right = %s, want PLAT-2", got.Key)
	}
	if got := app.issuesList.SelectedIssue(); got == nil || got.Key != boardKey2 {
		t.Errorf("list selection = %v, want it to follow the board cursor", got)
	}
}

func TestBoardLetsEscThrough(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.side = sideRight

	if _, _, handled := app.handleBoardKeys(ActFocusLeft, "esc"); handled {
		t.Error("the board swallowed esc; it must fall through to the focus handler")
	}
	if _, _, handled := app.handleBoardKeys(ActFocusLeft, "h"); !handled {
		t.Error("h should steer the board columns")
	}
}

func TestBoardIgnoresUnrelatedActions(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.side = sideRight

	if _, _, handled := app.handleBoardKeys(ActRefresh, "r"); handled {
		t.Error("the board claimed an action it does not own")
	}
}

func TestOpenBoardCardOpensTheTask(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.side = sideRight
	app.handleBoardKeys(ActFocusRight, "l") // move to PLAT-2

	_, cmd, handled := app.handleBoardKeys(ActOpen, "enter")

	if !handled || cmd == nil {
		t.Fatal("enter did not open the highlighted card")
	}
	if !app.taskOpen {
		t.Error("taskOpen = false after opening a card")
	}
	if app.detailView.Mode() != views.ModeKanban && app.detailView.IssueKey() != boardKey2 {
		t.Errorf("detail shows %q, want PLAT-2", app.detailView.IssueKey())
	}
}

func TestBoardFilterUsesCachedUsers(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.usersCache[testProject] = []jira.User{*kbUserAlice, *kbUserBob}

	_, cmd, handled := app.openBoardFilter()

	if !handled {
		t.Fatal("the filter action was not handled")
	}
	if cmd != nil {
		t.Error("the filter refetched users that were already cached")
	}
	if !app.modal.IsVisible() {
		t.Error("the assignee checklist was not shown")
	}
}

func TestBoardFilterFetchesUsersWhenMissing(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	_, cmd, _ := app.openBoardFilter()
	if cmd == nil {
		t.Fatal("the filter did not fetch the user list")
	}

	app.handleUsersLoaded(usersLoadedMsg{users: []jira.User{*kbUserAlice}, issueKey: boardFilterSentinel})

	if !app.modal.IsVisible() {
		t.Error("the checklist was not shown once the users arrived")
	}
}

func TestApplyBoardFilterHidesOtherAssignees(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	app.applyBoardFilter(components.ChecklistConfirmedMsg{Selected: []components.ModalItem{{ID: "u1", Label: meLabel + aliceName}}})

	board := app.detailView.Kanban()
	if got := len(board.Columns()[0].Issues); got != 1 {
		t.Errorf("To Do column = %d issues, want Alice's only", got)
	}
	if got := len(board.Columns()[1].Issues); got != 0 {
		t.Errorf("Done column = %d issues, want Bob's hidden", got)
	}
	if got := board.FilterLabel(); got != aliceName {
		t.Errorf("filter label = %q, want the (Me) prefix stripped", got)
	}
}

func TestBoardFilterPersistsPerProject(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.applyBoardFilter(components.ChecklistConfirmedMsg{Selected: []components.ModalItem{{ID: "u1", Label: "Alice"}}})

	app.showBoard() // as happens when coming back to the project

	if got := app.detailView.Kanban().FilterLabel(); got != aliceName {
		t.Errorf("filter label = %q, want the stored filter reapplied", got)
	}
}

func TestBoardFilterLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		names []string
		want  string
	}{
		{nil, ""},
		{[]string{"Alice"}, "Alice"},
		{[]string{"Alice", "Bob"}, "Alice, Bob"},
		{[]string{"Alice", "Bob", "Carol"}, "3 assignees"},
	}
	for _, tc := range cases {
		if got := boardFilterLabel(tc.names); got != tc.want {
			t.Errorf("boardFilterLabel(%v) = %q, want %q", tc.names, got, tc.want)
		}
	}
}

func TestIssuesLoadedRefreshesTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.statusPanel = views.NewStatusPanel(testProject, "", "")
	flag := false
	app.logFlag = &flag

	extra := kbStatusTodo
	app.handleIssuesLoaded(issuesLoadedMsg{
		tab: app.issuesList.GetTabIndex(),
		issues: append(kbBoardIssues(), jira.Issue{
			Key: testProject + "-3", Summary: "Third", Status: &extra, Assignee: kbUserAlice,
		}),
	})

	if got := len(app.detailView.Kanban().Columns()[0].Issues); got != 2 {
		t.Errorf("To Do column = %d issues, want the newly loaded one included", got)
	}
}

// keyMsg builds a key press the way the runtime delivers it.
func keyMsg(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestBoardKeysRoutedFromHandleKeyMsg(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.side = sideRight
	app.keymap = DefaultKeymap()
	app.helpBar = components.HelpBar{}

	app.handleKeyMsg(keyMsg("l"))

	if got := app.detailView.Kanban().SelectedIssue(); got == nil || got.Key != boardKey2 {
		t.Errorf("board cursor = %v; the key was not routed to the board", got)
	}
}

func TestBoardHasItsOwnHelpContext(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.side = sideRight

	var help strings.Builder
	for _, b := range app.ContextBindings() {
		help.WriteString(b.Key + " " + b.Description + "\n")
	}
	joined := help.String()
	for _, want := range []string{"move between cards", "filter by assignee", "open the highlighted task"} {
		if !strings.Contains(joined, want) {
			t.Errorf("board help is missing %q:\n%s", want, joined)
		}
	}

	var barBuf strings.Builder
	for _, item := range app.helpBarItems() {
		barBuf.WriteString(item.Key + ":" + item.Description + " ")
	}
	bar := barBuf.String()
	if !strings.Contains(bar, "filter") {
		t.Errorf("board help bar = %q", bar)
	}
}

// The project is often chosen by handleProjectsLoaded rather than by
// selectProject: at Init there is no project yet, so the board has to be
// raised there too or it never appears on startup.
func TestProjectsLoadedRaisesTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.projectKey = ""
	app.projectID = ""
	app.taskOpen = true
	app.detailView.SetIssue(&jira.Issue{Key: boardKey1})
	// Nothing known about this project yet, as on a cold start.
	delete(app.projectStatuses, testProject)

	_, cmd := app.handleProjectsLoaded(projectsLoadedMsg{
		projects: []jira.Project{{ID: "1", Key: testProject, Name: "Platform"}},
	})

	if app.detailView.Mode() != views.ModeKanban {
		t.Errorf("mode = %v, want the board up once the project is known", app.detailView.Mode())
	}
	if !app.boardVisible() {
		t.Error("boardVisible = false after the project loaded")
	}
	if cmd == nil {
		t.Fatal("the board columns were never fetched")
	}
}

// The board sits on the right, but the cursor is usually still in the issues
// list when the user reaches for the filter.
func TestFilterOpensFromTheLeftPanelToo(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.usersCache[testProject] = []jira.User{*kbUserAlice, *kbUserBob}
	app.side = sideLeft
	app.leftFocus = focusIssues

	app.handleKeyMsg(keyMsg("f"))

	if !app.modal.IsVisible() {
		t.Error("f did nothing with the board on screen and the focus on the left")
	}
}

func TestFilterIgnoredWhenNoBoardIsShown(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.usersCache[testProject] = []jira.User{*kbUserAlice}
	app.taskOpen = true
	app.detailView.SetIssue(&jira.Issue{Key: boardKey1})
	app.side = sideLeft
	app.leftFocus = focusIssues

	app.handleKeyMsg(keyMsg("f"))

	if app.modal.IsVisible() {
		t.Error("the board filter opened with a task on screen")
	}
}

// End-to-end through the real messages: the checklist result has to reach the
// board, not just the callback.
func TestChecklistConfirmFiltersTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.usersCache[testProject] = []jira.User{*kbUserAlice, *kbUserBob}
	app.side = sideRight

	total := func() int {
		n := 0
		for _, c := range app.detailView.Kanban().Columns() {
			n += len(c.Issues)
		}
		return n
	}
	if total() != 2 {
		t.Fatalf("cards = %d, want 2 before filtering", total())
	}

	app.handleKeyMsg(keyMsg("f"))
	app.Update(components.ChecklistConfirmedMsg{
		Selected: []components.ModalItem{{ID: kbUserAlice.AccountID, Label: kbUserAlice.DisplayName}},
	})

	if got := total(); got != 1 {
		t.Errorf("cards = %d, want only Alice's after confirming the checklist", got)
	}
	if got := app.detailView.Kanban().FilterLabel(); got != aliceName {
		t.Errorf("filter label = %q", got)
	}
}

// A checklist needs space to tick a row, but people press enter on the person
// they want. An empty selection then meant "everyone", so the filter appeared
// to do nothing at all.
func TestFilterAppliesTheHighlightedRowWhenNothingIsTicked(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	app.applyBoardFilter(components.ChecklistConfirmedMsg{
		Cursor: components.ModalItem{ID: kbUserAlice.AccountID, Label: kbUserAlice.DisplayName},
	})

	board := app.detailView.Kanban()
	if got := len(board.Columns()[0].Issues); got != 1 {
		t.Errorf("To Do column = %d issues, want only Alice's", got)
	}
	if got := len(board.Columns()[1].Issues); got != 0 {
		t.Errorf("Done column = %d issues, want Bob's hidden", got)
	}
	if got := board.FilterLabel(); got != aliceName {
		t.Errorf("filter label = %q", got)
	}
}

// Ticked rows still win: the cursor is only a fallback.
func TestTickedRowsBeatTheCursor(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	app.applyBoardFilter(components.ChecklistConfirmedMsg{
		Selected: []components.ModalItem{{ID: kbUserBob.AccountID, Label: kbUserBob.DisplayName}},
		Cursor:   components.ModalItem{ID: kbUserAlice.AccountID, Label: kbUserAlice.DisplayName},
	})

	if got := app.detailView.Kanban().FilterLabel(); got != "Bob" {
		t.Errorf("filter label = %q, want the ticked row", got)
	}
}

func TestEveryoneRowClearsTheFilter(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.applyBoardFilter(components.ChecklistConfirmedMsg{
		Selected: []components.ModalItem{{ID: kbUserAlice.AccountID, Label: kbUserAlice.DisplayName}},
	})

	app.applyBoardFilter(components.ChecklistConfirmedMsg{
		Cursor: components.ModalItem{ID: everyoneFilterID, Label: "(Everyone — clear the filter)"},
	})

	total := 0
	for _, c := range app.detailView.Kanban().Columns() {
		total += len(c.Issues)
	}
	if total != 2 {
		t.Errorf("cards = %d, want every card back", total)
	}
	if got := app.detailView.Kanban().FilterLabel(); got != "" {
		t.Errorf("filter label = %q, want it cleared", got)
	}
}

func TestFilterModalOffersTheClearRowFirst(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	app.showBoardFilterModal([]jira.User{*kbUserAlice})

	items := app.modal.Items()
	if len(items) == 0 || items[0].ID != everyoneFilterID {
		t.Fatalf("first row = %+v, want the clear-the-filter row", items)
	}
}

// Reopening the filter shows the active one already ticked, so choosing the
// clear row has to win over those leftovers.
func TestEveryoneRowBeatsLeftoverTicks(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)

	app.applyBoardFilter(components.ChecklistConfirmedMsg{
		Selected: []components.ModalItem{{ID: kbUserAlice.AccountID, Label: kbUserAlice.DisplayName}},
		Cursor:   components.ModalItem{ID: everyoneFilterID, Label: "(Everyone — clear the filter)"},
	})

	total := 0
	for _, c := range app.detailView.Kanban().Columns() {
		total += len(c.Issues)
	}
	if total != 2 {
		t.Errorf("cards = %d, want the filter cleared despite the ticked row", total)
	}
}

func kbBoards() []jira.Board {
	return []jira.Board{
		{ID: 1, Name: "Delivery", Type: "scrum", ProjectKey: testProject},
		{ID: 2, Name: "Bugs", Type: "kanban", ProjectKey: testProject},
		{ID: 9, Name: "Other project", Type: "kanban", ProjectKey: "OTHER"},
	}
}

func TestProjectBoardsExcludesOtherProjects(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()

	got := app.projectBoards()

	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("boards = %+v, want only this project's", got)
	}
}

func TestBoardFetchesItsOwnColumnsAndCards(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()

	cmd := app.showBoard()
	if cmd == nil {
		t.Fatal("the active board's data was never fetched")
	}

	app.handleBoardDataLoaded(boardDataLoadedMsg{
		boardID: 1,
		columns: []jira.BoardColumn{{Name: "Doing", StatusIDs: []string{"1", "5"}}},
		issues:  kbBoardIssues(),
	})

	cols := app.detailView.Kanban().Columns()
	if len(cols) != 1 || cols[0].Name != "Doing" {
		t.Fatalf("columns = %+v, want the board's own layout", cols)
	}
	if len(cols[0].Issues) != 2 {
		t.Errorf("cards = %d, want both statuses collected in the column", len(cols[0].Issues))
	}
}

func TestCycleBoardSwitchesToTheNextOne(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()
	app.handleBoardDataLoaded(boardDataLoadedMsg{boardID: 1, issues: kbBoardIssues()})

	cmd := app.cycleBoard(1)

	if app.activeBoard != 1 {
		t.Fatalf("activeBoard = %d, want the second board", app.activeBoard)
	}
	if cmd == nil {
		t.Error("the second board's data was not fetched")
	}
	if got := app.detailView.Kanban().Title(); !strings.Contains(got, "[Bugs]") {
		t.Errorf("title = %q, want the second board marked active", got)
	}
}

func TestCycleBoardWrapsAround(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()

	app.cycleBoard(-1)

	if app.activeBoard != 1 {
		t.Errorf("activeBoard = %d, want it to wrap to the last board", app.activeBoard)
	}
}

func TestCycleBoardIsNoOpWithASingleBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = []jira.Board{{ID: 1, Name: "Only", ProjectKey: testProject}}
	app.showBoard()

	if cmd := app.cycleBoard(1); cmd != nil {
		t.Error("cycling did something with a single board")
	}
	if app.activeBoard != 0 {
		t.Errorf("activeBoard = %d", app.activeBoard)
	}
}

// A project with no agile board still gets a usable board, laid out from its
// statuses and filled from the active list tab.
func TestProjectWithoutBoardsFallsBackToStatuses(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = []jira.Board{{ID: 9, Name: "Elsewhere", ProjectKey: "OTHER"}}

	app.showBoard()

	if app.activeBoard != -1 {
		t.Errorf("activeBoard = %d, want none selected", app.activeBoard)
	}
	cols := app.detailView.Kanban().Columns()
	if len(cols) != 2 || cols[0].Name != "To Do" {
		t.Fatalf("columns = %+v, want the project statuses", cols)
	}
	if got := app.detailView.Kanban().Title(); strings.Contains(got, "[") && strings.Contains(got, "]") &&
		!strings.HasPrefix(got, "[0] Kanban") {
		t.Errorf("title = %q, want no board tabs", got)
	}
}

func TestSwitchingProjectResetsTheActiveBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()
	app.cycleBoard(1)

	app.selectProject(&jira.Project{Key: "OTHER", ID: "9"})

	if app.activeBoard == 1 {
		t.Error("the previous project's board index carried over")
	}
}

func TestBoardDataErrorKeepsTheBoardUsable(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()

	app.handleBoardDataLoaded(boardDataLoadedMsg{boardID: 1, err: errBoardTest})

	if got := len(app.detailView.Kanban().Columns()); got != 2 {
		t.Errorf("columns = %d, want the status fallback still standing", got)
	}
}

func TestBoardTabKeysCycleBoards(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()
	app.side = sideRight

	app.handleBoardKeys(ActNextTab, "]")

	if app.activeBoard != 1 {
		t.Errorf("activeBoard = %d, want ] to move to the next board", app.activeBoard)
	}
}

// With a single board the tab keys belong to the issue list, as before.
func TestTabKeysFallThroughWithASingleBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = []jira.Board{{ID: 1, Name: "Only", ProjectKey: testProject}}
	app.showBoard()
	app.side = sideRight

	if _, _, handled := app.handleBoardKeys(ActNextTab, "]"); handled {
		t.Error("the board swallowed ] with nothing to cycle between")
	}
}

// The board mirrors what the issue list is showing, so switching to a
// narrower tab or typing a filter narrows the board too.
func TestBoardFollowsTheVisibleList(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()
	app.handleBoardDataLoaded(boardDataLoadedMsg{
		boardID: 1,
		issues:  kbBoardIssues(),
	})

	cards := func() int {
		n := 0
		for _, c := range app.detailView.Kanban().Columns() {
			n += len(c.Issues)
		}
		return n
	}
	if cards() != 2 {
		t.Fatalf("cards = %d, want the whole board", cards())
	}

	app.issuesList.SetFilter(aliceName)
	app.refreshBoard()

	if got := cards(); got != 1 {
		t.Errorf("cards = %d, want the board narrowed with the list", got)
	}
}

// A board card the list is not showing drops off the board, which is the
// trade-off of mirroring the list.
func TestBoardDropsCardsMissingFromTheList(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.boards = kbBoards()
	app.showBoard()

	extra := kbStatusTodo
	app.handleBoardDataLoaded(boardDataLoadedMsg{
		boardID: 1,
		issues: append(kbBoardIssues(), jira.Issue{
			Key: testProject + "-99", Summary: "Only on the board", Status: &extra,
		}),
	})

	for _, c := range app.detailView.Kanban().Columns() {
		for _, iss := range c.Issues {
			if iss.Key == testProject+"-99" {
				t.Error("a card absent from the list stayed on the board")
			}
		}
	}
}

// Before the list has loaded anything the scope must not blank the board.
func TestEmptyListDoesNotBlankTheBoard(t *testing.T) {
	t.Parallel()
	app := newBoardApp(t)
	app.issuesList.SetIssues(nil)
	app.boards = kbBoards()
	app.showBoard()
	app.handleBoardDataLoaded(boardDataLoadedMsg{boardID: 1, issues: kbBoardIssues()})

	total := 0
	for _, c := range app.detailView.Kanban().Columns() {
		total += len(c.Issues)
	}
	if total != 2 {
		t.Errorf("cards = %d, want the board intact while the list is empty", total)
	}
}
