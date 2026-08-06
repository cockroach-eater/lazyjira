package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

// meLabel prefixes the current user in the board filter so it is pickable
// without knowing one's own display name by heart.
const meLabel = "(Me) "

// boardFilterSentinel marks a user fetch whose result feeds the board filter
// rather than the assignee picker.
const boardFilterSentinel = "__board_filter__"

// everyoneFilterID is the row that clears the board filter. Without it the
// only way back to "show everything" would be to untick every name.
const everyoneFilterID = "__everyone__"

// boardVisible reports whether the right panel is showing the project board.
// While it is, list navigation must not push issues into the detail panel:
// the board owns it until a task is opened with enter.
func (a *App) boardVisible() bool {
	return !a.taskOpen && a.detailView.Mode() == views.ModeKanban
}

// projectBoards are the agile boards of the active project, in the order the
// Agile API reported them.
func (a *App) projectBoards() []jira.Board {
	var out []jira.Board
	for _, b := range a.boards {
		if b.ProjectKey == a.projectKey {
			out = append(out, b)
		}
	}
	return out
}

// showBoard puts the active project's board on the right panel. A project can
// have several agile boards, each with its own columns and its own issues; one
// with none falls back to a board laid out from the project statuses.
func (a *App) showBoard() tea.Cmd {
	if a.projectKey == "" {
		return nil
	}
	a.taskOpen = false
	a.detailView.ShowKanban(a.projectKey)

	boards := a.projectBoards()
	if a.activeBoard >= len(boards) {
		a.activeBoard = len(boards) - 1
	}
	if a.activeBoard < 0 && len(boards) > 0 {
		a.activeBoard = 0
	}

	view := a.detailView.Kanban()
	view.SetAssigneeFilter(a.boardFilters[a.projectKey], a.boardFilterLabels[a.projectKey])
	view.SetBoardTabs(boardNames(boards), a.activeBoard)

	cmds := []tea.Cmd{}
	if _, ok := a.projectStatuses[a.projectKey]; !ok {
		cmds = append(cmds, fetchProjectStatuses(a.client, a.projectKey))
	}
	if cmd := a.applyBoardLayout(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sel := a.issuesList.SelectedIssue(); sel != nil {
		view.SelectByKey(sel.Key)
	}
	return tea.Batch(cmds...)
}

// boardScope is the set of issue keys the issue list is currently showing, so
// that narrowing the list -- by tab or by the search filter -- narrows the
// board with it. Returns nil while the list has nothing loaded, which would
// otherwise blank the board before its first fetch lands.
func (a *App) boardScope() map[string]bool {
	visible := a.issuesList.VisibleIssues()
	if len(visible) == 0 && len(a.issuesList.CurrentIssues()) == 0 {
		return nil
	}
	keys := make(map[string]bool, len(visible))
	for _, issue := range visible {
		keys[issue.Key] = true
	}
	return keys
}

// applyBoardLayout feeds the view from the active agile board, falling back to
// the project statuses and the active list tab while the board is unknown or
// when the project has no board at all. Returns a fetch when the board's data
// is not cached yet.
func (a *App) applyBoardLayout() tea.Cmd {
	view := a.detailView.Kanban()
	boards := a.projectBoards()
	view.SetScope(a.boardScope())

	if a.activeBoard < 0 || a.activeBoard >= len(boards) {
		view.SetData(a.projectStatuses[a.projectKey], a.issuesList.CurrentIssues())
		return nil
	}

	boardID := boards[a.activeBoard].ID
	issues, haveIssues := a.boardIssues[boardID]
	columns, haveColumns := a.boardColumns[boardID]
	if !haveIssues {
		// Show the list issues under the project statuses until the board
		// answers, rather than blanking the panel.
		view.SetData(a.projectStatuses[a.projectKey], a.issuesList.CurrentIssues())
		return fetchBoardData(a.client, boardID)
	}

	view.SetIssues(issues)
	if haveColumns && len(columns) > 0 {
		view.SetBoardLayout(columns, a.projectStatuses[a.projectKey])
	} else {
		view.SetStatuses(a.projectStatuses[a.projectKey])
	}
	return nil
}

func boardNames(boards []jira.Board) []string {
	if len(boards) < 2 {
		// A single board adds nothing to the title.
		return nil
	}
	names := make([]string, 0, len(boards))
	for _, b := range boards {
		names = append(names, b.Name)
	}
	return names
}

// cycleBoard moves to the next or previous board of the project.
func (a *App) cycleBoard(delta int) tea.Cmd {
	boards := a.projectBoards()
	if len(boards) < 2 {
		return nil
	}
	a.activeBoard = (a.activeBoard + delta + len(boards)) % len(boards)
	a.detailView.Kanban().SetBoardTabs(boardNames(boards), a.activeBoard)
	return a.applyBoardLayout()
}

// handleBoardDataLoaded caches an agile board's layout and cards.
func (a *App) handleBoardDataLoaded(msg boardDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Not fatal: the board keeps the status-derived layout it is showing.
		a.statusPanel.SetError("board " + strconv.Itoa(msg.boardID) + ": " + msg.err.Error())
		return a, nil
	}
	a.boardColumns[msg.boardID] = msg.columns
	a.boardIssues[msg.boardID] = msg.issues

	if a.boardVisible() {
		if boards := a.projectBoards(); a.activeBoard >= 0 && a.activeBoard < len(boards) &&
			boards[a.activeBoard].ID == msg.boardID {
			return a, a.applyBoardLayout()
		}
	}
	return a, nil
}

// refreshBoard re-feeds the board from whatever source it is using. It is a
// no-op unless the board is the visible panel.
func (a *App) refreshBoard() {
	if !a.boardVisible() {
		return
	}
	a.applyBoardLayout()
}

// handleProjectStatusesLoaded caches a project's workflow statuses. A failed
// request is not surfaced as an error: the board falls back to the statuses
// present in the loaded issues, which is a usable board.
func (a *App) handleProjectStatusesLoaded(msg projectStatusesLoadedMsg) (tea.Model, tea.Cmd) {
	if len(msg.statuses) == 0 {
		return a, nil
	}
	a.projectStatuses[msg.projectKey] = msg.statuses
	if a.boardVisible() && msg.projectKey == a.projectKey {
		a.applyBoardLayout()
	}
	return a, nil
}

// handleBoardKeys drives the board while the right panel has focus. It returns
// handled=false for anything it does not own, so the global bindings keep
// working.
//
// key is needed because the same action covers both "move a column left" and
// "leave the panel": h and left steer the board, esc backs out of it.
func (a *App) handleBoardKeys(action Action, key string) (tea.Model, tea.Cmd, bool) {
	board := a.detailView.Kanban()

	switch action { //nolint:exhaustive
	case ActNavUp, ActDetailScrollUp:
		board.Move(0, -1)
	case ActNavDown, ActDetailScrollDown:
		board.Move(0, 1)
	case ActFocusLeft:
		if key == keyEsc {
			return a, nil, false
		}
		board.Move(-1, 0)
	case ActFocusRight:
		board.Move(1, 0)
	case ActOpen, ActSelect:
		return a.openBoardCard()
	case ActFilterAssignees:
		return a.openBoardFilter()
	case ActPrevTab, ActNextTab:
		// Only claim the tab keys when there is more than one board to move
		// between; otherwise they keep switching the issue list's tabs.
		if len(a.projectBoards()) < 2 {
			return a, nil, false
		}
		delta := 1
		if action == ActPrevTab {
			delta = -1
		}
		return a, a.cycleBoard(delta), true
	default:
		return a, nil, false
	}

	// Keep the list cursor on the card the board highlights, so leaving the
	// board lands on the same issue.
	if sel := board.SelectedIssue(); sel != nil {
		a.issuesList.SelectByKey(sel.Key)
		a.infoPanel.SetIssue(sel)
	}
	return a, nil, true
}

// openBoardCard opens the highlighted card as a regular task view.
func (a *App) openBoardCard() (tea.Model, tea.Cmd, bool) {
	sel := a.detailView.Kanban().SelectedIssue()
	if sel == nil {
		return a, nil, true
	}
	a.taskOpen = true
	a.issuesList.SelectByKey(sel.Key)
	a.infoPanel.SetIssue(sel)
	a.showCachedIssue(sel.Key)
	if a.detailView.Mode() == views.ModeKanban {
		// Nothing cached yet: show what the board knows until the fetch lands.
		a.detailView.SetIssue(sel)
	}
	return a, fetchIssueDetail(a.client, sel.Key), true
}

// openBoardFilter asks for the set of assignees the board should show. The
// user list is fetched (and cached) exactly like the assignee picker does.
func (a *App) openBoardFilter() (tea.Model, tea.Cmd, bool) {
	a.editContext = editCtx{kind: editBoardFilter}
	a.onChecklist = a.applyBoardFilter

	if users, ok := a.usersCache[a.projectKey]; ok {
		a.showBoardFilterModal(users)
		return a, nil, true
	}
	*a.logFlag = true
	return a, fetchUsers(a.client, a.projectKey, boardFilterSentinel), true
}

// showBoardFilterModal lists the assignable users, with the current user and
// the unassigned bucket pinned on top, preselecting the active filter.
func (a *App) showBoardFilterModal(users []jira.User) {
	selected := a.boardFilters[a.projectKey]

	items := []components.ModalItem{
		{ID: everyoneFilterID, Label: "(Everyone — clear the filter)"},
		{ID: views.UnassignedFilterID, Label: "(Unassigned)"},
	}
	if a.currentUser != nil {
		items = append(items, components.ModalItem{
			ID:    a.currentUser.AccountID,
			Label: meLabel + a.currentUser.DisplayName,
		})
	}
	for _, u := range users {
		if a.currentUser != nil && u.AccountID == a.currentUser.AccountID {
			continue
		}
		items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName})
	}

	a.modal.ShowChecklist("Show issues assigned to", items, selected)
}

// applyBoardFilter stores the chosen assignees for the active project.
//
// Confirming without ticking anything falls back to the highlighted row:
// people reach for enter on the person they want, and an empty selection
// silently meaning "everyone" reads as a filter that does nothing. Clearing
// is done explicitly through the "everyone" row.
func (a *App) applyBoardFilter(confirmed components.ChecklistConfirmedMsg) tea.Cmd {
	items := confirmed.Selected
	switch {
	case confirmed.Cursor.ID == everyoneFilterID:
		// Landing on that row is an explicit "show everything", and it has to
		// beat the ticks left over from the filter being replaced.
		items = nil
	case len(items) == 0 && confirmed.Cursor.ID != "":
		items = []components.ModalItem{confirmed.Cursor}
	}

	ids := make(map[string]bool, len(items))
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.ID == everyoneFilterID {
			ids, names = make(map[string]bool), nil
			break
		}
		ids[item.ID] = true
		names = append(names, strings.TrimPrefix(item.Label, meLabel))
	}

	label := boardFilterLabel(names)
	a.boardFilters[a.projectKey] = ids
	a.boardFilterLabels[a.projectKey] = label
	a.detailView.Kanban().SetAssigneeFilter(ids, label)
	return nil
}

// boardFilterLabel keeps the panel title short: names while they fit, a count
// beyond that.
func boardFilterLabel(names []string) string {
	switch {
	case len(names) == 0:
		return ""
	case len(names) <= 2:
		return strings.Join(names, ", ")
	default:
		return strconv.Itoa(len(names)) + " assignees"
	}
}
