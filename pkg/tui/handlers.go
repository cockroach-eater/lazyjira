package tui

import (
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/git"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// handleResize handles terminal resize events.
func (a *App) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height
	a.overlays.SetSize(msg.Width, msg.Height)
	a.layoutPanels()
	a.helpBar.SetWidth(msg.Width)
	a.searchBar.SetWidth(msg.Width)
	return a, nil
}

// handleSearchChanged filters the active panel by query.
func (a *App) handleSearchChanged(msg components.SearchChangedMsg) (tea.Model, tea.Cmd) {
	if a.side == sideLeft && a.leftFocus == focusIssues {
		a.issuesList.SetFilter(msg.Query)
	} else if a.side == sideLeft && a.leftFocus == focusProjects {
		a.projectList.SetFilter(msg.Query)
	}
	return a, nil
}

// handleSearchConfirmed finalizes search: selects the filtered item and loads data.
func (a *App) handleSearchConfirmed() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if a.side == sideLeft && a.leftFocus == focusIssues {
		selectedIssue := a.issuesList.SelectedIssue()
		a.issuesList.ClearFilter()
		if selectedIssue != nil {
			a.issuesList.SelectByKey(selectedIssue.Key)
			cmds = append(cmds, fetchIssueDetail(a.client, selectedIssue.Key))
		}
	} else if a.side == sideLeft && a.leftFocus == focusProjects {
		if p := a.projectList.SelectedProject(); p != nil {
			a.selectProject(p)
			cmds = append(cmds, a.fetchActiveTab())
		}
		a.projectList.SetFilter("")
	}
	return a, tea.Batch(cmds...)
}

// handleSearchCancelled clears search filter.
func (a *App) handleSearchCancelled() (tea.Model, tea.Cmd) {
	a.issuesList.SetFilter("")
	a.projectList.SetFilter("")
	return a, nil
}

// handleAutoFetch re-fetches issues on a timer.
func (a *App) handleAutoFetch() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if cmd := a.fetchActiveTab(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return autoFetchTickMsg{}
	}))
	return a, tea.Batch(cmds...)
}

// handleIssuesLoaded processes newly fetched issues.
func (a *App) handleIssuesLoaded(msg issuesLoadedMsg) (tea.Model, tea.Cmd) {
	a.err = nil
	*a.logFlag = false
	a.statusPanel.SetOnline(true)
	a.issuesList.SetIssuesForTab(msg.tab, msg.issues)

	var cmds []tea.Cmd
	if msg.tab == a.issuesList.GetTabIndex() {
		a.issuesList.SetIssues(msg.issues)
		for _, issue := range msg.issues {
			cmds = append(cmds, prefetchIssue(a.client, issue.Key))
		}
	}
	// Auto-detect: select issue from git branch.
	if a.gitDetectedKey != "" {
		detectedKey := a.gitDetectedKey
		projectKey := strings.SplitN(detectedKey, "-", 2)[0]
		if !strings.EqualFold(projectKey, a.projectKey) {
			projects := a.projectList.AllProjects()
			for _, p := range projects {
				if strings.EqualFold(p.Key, projectKey) {
					a.selectProject(&p)
					cmds = append(cmds, a.fetchActiveTab())
					return a, tea.Batch(cmds...)
				}
			}
			a.gitDetectedKey = ""
		} else {
			a.gitDetectedKey = ""
			a.navigateToIssue(detectedKey)
		}
	}
	return a, tea.Batch(cmds...)
}

// handleIssueDetailLoaded updates the detail view with full issue data.
func (a *App) handleIssueDetailLoaded(msg issueDetailLoadedMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = false
	if msg.issue != nil {
		a.issueCache[msg.issue.Key] = msg.issue
	}
	if a.leftFocus == focusIssues || a.side == sideRight {
		a.detailView.SetIssue(msg.issue)
	} else {
		a.detailView.UpdateIssueData(msg.issue)
	}
	return a, nil
}

// handleIssuePrefetched caches prefetched issue data.
func (a *App) handleIssuePrefetched(msg issuePrefetchedMsg) (tea.Model, tea.Cmd) {
	if msg.issue != nil {
		a.issueCache[msg.issue.Key] = msg.issue
		if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == msg.issue.Key {
			if a.leftFocus == focusIssues || a.side == sideRight {
				a.detailView.SetIssue(msg.issue)
			}
		}
	}
	return a, nil
}

// handleTransitionDone re-fetches data after a transition.
func (a *App) handleTransitionDone() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if cmd := a.fetchActiveTab(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sel := a.issuesList.SelectedIssue(); sel != nil {
		cmds = append(cmds, fetchIssueDetail(a.client, sel.Key))
	}
	return a, tea.Batch(cmds...)
}

// handleTransitionsLoaded shows the transition picker modal.
func (a *App) handleTransitionsLoaded(msg transitionsLoadedMsg) (tea.Model, tea.Cmd) {
	if len(msg.transitions) == 0 {
		return a, nil
	}
	var items []components.ModalItem
	for _, t := range msg.transitions {
		label := t.Name
		hint := ""
		if t.To != nil {
			label += " → " + t.To.Name
			hint = t.To.Description
		}
		items = append(items, components.ModalItem{ID: t.ID, Label: label, Hint: hint})
	}
	a.onSelect = nil // default: transition/URL handler
	a.modal.Show("Transition: "+msg.issueKey, items)
	return a, nil
}

// handlePrioritiesLoaded shows the priority picker modal.
func (a *App) handlePrioritiesLoaded(msg prioritiesLoadedMsg) (tea.Model, tea.Cmd) {
	if len(msg.priorities) == 0 {
		return a, nil
	}
	var items []components.ModalItem
	for _, p := range msg.priorities {
		items = append(items, components.ModalItem{ID: p.ID, Label: p.Name})
	}
	a.onSelect = func(item components.ModalItem) tea.Cmd {
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			return updateIssueField(a.client, sel.Key, "priority", map[string]string{"id": item.ID})
		}
		return nil
	}
	a.modal.Show("Priority", items)
	return a, nil
}

// handleUsersLoaded shows the assignee/reporter picker modal.
func (a *App) handleUsersLoaded(msg usersLoadedMsg) (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	var items []components.ModalItem
	items = append(items, components.ModalItem{ID: "", Label: "Unassigned"})
	email := a.cfg.Jira.Email
	for _, u := range msg.users {
		if u.Email == email {
			items = append(items, components.ModalItem{ID: u.AccountID, Label: "→ " + u.DisplayName})
			break
		}
	}
	for _, u := range msg.users {
		if u.Email != email {
			items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName})
		}
	}
	// onSelect callback set at fetch time (ActEditAssignee or editInfoField).
	a.modal.Show("Assignee: "+sel.Key, items)
	return a, nil
}

// handleLabelsLoaded shows the labels checklist modal.
func (a *App) handleLabelsLoaded(msg labelsLoadedMsg) (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	cached := sel
	if c, ok := a.issueCache[sel.Key]; ok {
		cached = c
	}
	selected := make(map[string]bool)
	for _, l := range cached.Labels {
		selected[l] = true
	}
	var items []components.ModalItem
	for _, l := range msg.labels {
		items = append(items, components.ModalItem{ID: l, Label: l})
	}
	a.modal.ShowChecklist("Labels: "+sel.Key, items, selected)
	return a, nil
}

// handleComponentsLoaded shows the components checklist modal.
func (a *App) handleComponentsLoaded(msg componentsLoadedMsg) (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	cached := sel
	if c, ok := a.issueCache[sel.Key]; ok {
		cached = c
	}
	selected := make(map[string]bool)
	for _, c := range cached.Components {
		selected[c.ID] = true
	}
	var items []components.ModalItem
	for _, c := range msg.components {
		items = append(items, components.ModalItem{ID: c.ID, Label: c.Name})
	}
	a.modal.ShowChecklist("Components: "+sel.Key, items, selected)
	return a, nil
}

// handleIssueTypesLoaded shows the issue type picker modal.
func (a *App) handleIssueTypesLoaded(msg issueTypesLoadedMsg) (tea.Model, tea.Cmd) {
	items := make([]components.ModalItem, 0, len(msg.issueTypes))
	for _, t := range msg.issueTypes {
		items = append(items, components.ModalItem{ID: t.ID, Label: t.Name})
	}
	a.modal.Show("Issue Type", items)
	return a, nil
}

// handleModalSelected dispatches modal selection result.
func (a *App) handleModalSelected(msg components.ModalSelectedMsg) (tea.Model, tea.Cmd) {
	if fn := a.onSelect; fn != nil {
		a.onSelect = nil
		return a, fn(msg.Item)
	}
	// Default: transition or URL picker.
	id := msg.Item.ID
	if strings.HasPrefix(id, "http") {
		if issueKey := a.extractIssueKey(id); issueKey != "" {
			a.navigateToIssue(issueKey)
			return a, nil
		}
		openBrowser(id)
	} else {
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			return a, doTransition(a.client, sel.Key, id)
		}
	}
	return a, nil
}

// handleChecklistConfirmed dispatches checklist confirmation result.
func (a *App) handleChecklistConfirmed(msg components.ChecklistConfirmedMsg) (tea.Model, tea.Cmd) {
	if fn := a.onChecklist; fn != nil {
		a.onChecklist = nil
		return a, fn(msg.Selected)
	}
	return a, nil
}

// handleModalCancelled clears modal callbacks.
func (a *App) handleModalCancelled() (tea.Model, tea.Cmd) {
	a.onSelect = nil
	a.onChecklist = nil
	return a, nil
}

// handleEditorFinished processes $EDITOR exit and shows diff view.
func (a *App) handleEditorFinished(msg editorFinishedMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tea.EnableMouseCellMotion}
	content, changed, err := readAndCheckEditor(msg)
	if err != nil {
		cleanupEditor(msg.tempPath)
		a.editTempPath = ""
		a.err = err
		return a, tea.Batch(cmds...)
	}
	if !changed {
		cleanupEditor(msg.tempPath)
		a.editTempPath = ""
		return a, tea.Batch(cmds...)
	}
	a.editTempPath = msg.tempPath
	a.diffView.Show("Confirm changes", msg.original, content)
	return a, tea.Batch(cmds...)
}

// handleDiffConfirmed applies the approved edit.
func (a *App) handleDiffConfirmed(msg components.DiffConfirmedMsg) (tea.Model, tea.Cmd) {
	cleanupEditor(a.editTempPath)
	a.editTempPath = ""
	return a, a.applyEdit(msg.Content)
}

// handleDiffCancelled discards the edit.
func (a *App) handleDiffCancelled() (tea.Model, tea.Cmd) {
	cleanupEditor(a.editTempPath)
	a.editTempPath = ""
	return a, nil
}

// handleInputConfirmed processes text input results (summary, field, branch).
func (a *App) handleInputConfirmed(msg components.InputConfirmedMsg) (tea.Model, tea.Cmd) {
	ctx := a.editContext
	a.editContext = editCtx{}
	switch ctx.kind { //nolint:exhaustive // only InputModal-based kinds handled here
	case editSummary:
		if msg.Text != "" {
			return a, updateIssueField(a.client, ctx.issueKey, "summary", msg.Text)
		}
	case editField:
		if msg.Text != "" {
			return a, updateIssueField(a.client, ctx.issueKey, ctx.fieldID, msg.Text)
		}
	case editBranch:
		if msg.Text != "" {
			switch {
			case git.BranchExists(a.gitRepoPath, msg.Text):
				return a, gitCheckoutBranch(a.gitRepoPath, msg.Text)
			case strings.Contains(msg.Text, "/"):
				return a, gitCheckoutTracking(a.gitRepoPath, msg.Text)
			default:
				return a, gitCreateBranch(a.gitRepoPath, msg.Text)
			}
		}
	}
	return a, nil
}

// handleInputCancelled clears edit context.
func (a *App) handleInputCancelled() (tea.Model, tea.Cmd) {
	a.editContext = editCtx{}
	return a, nil
}

// handleIssueUpdated re-fetches issue data after an update.
func (a *App) handleIssueUpdated(msg issueUpdatedMsg) (tea.Model, tea.Cmd) {
	return a, tea.Batch(
		fetchIssueDetail(a.client, msg.issueKey),
		a.fetchActiveTab(),
	)
}

// handleExpandBlock shows expanded content in a read-only modal.
func (a *App) handleExpandBlock(msg views.ExpandBlockMsg) (tea.Model, tea.Cmd) {
	items := make([]components.ModalItem, 0, len(msg.Lines))
	for _, line := range msg.Lines {
		items = append(items, components.ModalItem{ID: "", Label: line})
	}
	a.modal.SetSize(a.width, a.height-1)
	a.modal.ShowReadOnly(msg.Title, items)
	return a, nil
}

// handleJQLSubmit starts a JQL search.
func (a *App) handleJQLSubmit(msg components.JQLSubmitMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = true
	a.jqlModal.SetLoading(true)
	return a, fetchJQLSearch(a.client, msg.Query)
}

// handleJQLSearchResult processes JQL search results.
func (a *App) handleJQLSearchResult(msg jqlSearchResultMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = false
	a.jqlModal.Hide()
	a.issuesList.AddJQLTab(msg.jql)
	a.issuesList.SetIssues(msg.issues)
	history := LoadJQLHistory()
	history = AddToHistory(history, msg.jql)
	_ = SaveJQLHistory(history)
	a.side = sideLeft
	a.leftFocus = focusIssues
	a.updateFocusState()
	cmds := make([]tea.Cmd, 0, len(msg.issues))
	for _, issue := range msg.issues {
		cmds = append(cmds, prefetchIssue(a.client, issue.Key))
	}
	return a, tea.Batch(cmds...)
}

// handleJQLSearchError shows error in JQL modal.
func (a *App) handleJQLSearchError(msg jqlSearchErrorMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = false
	a.jqlModal.SetError(msg.err)
	return a, nil
}

// handleJQLInputChanged processes JQL autocomplete context.
func (a *App) handleJQLInputChanged(msg components.JQLInputChangedMsg) (tea.Model, tea.Cmd) {
	ctx := parseJQLContext(msg.Text, msg.CursorPos)
	a.jqlModal.SetPartialLen(ctx.PartialLen)
	switch ctx.Mode {
	case jqlCtxField:
		if a.jqlFields != nil {
			suggestions := matchFieldSuggestions(a.jqlFields, ctx.Partial)
			if len(suggestions) > 0 {
				a.jqlModal.SetSuggestions(suggestions)
			} else {
				a.jqlModal.SetHistory(LoadJQLHistory())
			}
		}
	case jqlCtxValue:
		a.jqlModal.SetACLoading(true)
		return a, fetchJQLSuggestions(a.client, ctx.FieldName, ctx.Partial)
	default:
		a.jqlModal.SetHistory(LoadJQLHistory())
	}
	return a, nil
}

// handleJQLFieldsLoaded caches JQL autocomplete field data.
func (a *App) handleJQLFieldsLoaded(msg jqlFieldsLoadedMsg) (tea.Model, tea.Cmd) {
	a.jqlFields = msg.fields
	return a, nil
}

// handleJQLSuggestions updates JQL suggestion list.
func (a *App) handleJQLSuggestions(msg jqlSuggestionsMsg) (tea.Model, tea.Cmd) {
	if a.jqlModal.IsVisible() {
		items := make([]string, 0, len(msg.suggestions))
		for _, s := range msg.suggestions {
			items = append(items, s.Value)
		}
		if len(items) > 0 {
			a.jqlModal.SetSuggestions(items)
		} else {
			a.jqlModal.SetHistory(LoadJQLHistory())
		}
	}
	return a, nil
}

// handleProjectsLoaded processes the project list from API.
func (a *App) handleProjectsLoaded(msg projectsLoadedMsg) (tea.Model, tea.Cmd) {
	projects := msg.projects
	if !a.demoMode {
		if creds, err := config.LoadCredentials(); err == nil && creds != nil && creds.LastProject != "" {
			for i, p := range projects {
				if p.Key == creds.LastProject {
					projects[0], projects[i] = projects[i], projects[0]
					break
				}
			}
		}
	}
	a.projectList.SetProjects(projects)
	if a.projectKey == "" && len(projects) > 0 {
		a.projectKey = projects[0].Key
		a.projectID = projects[0].ID
		a.statusPanel.SetProject(a.projectKey)
		a.projectList.SetActiveKey(a.projectKey)
		return a, a.fetchActiveTab()
	}
	return a, nil
}

// handleKeyMsg dispatches keyboard actions.
//
// handleKeyMsg dispatches keyboard actions.
// Returns (nil, nil) if the key was not handled and should be forwarded to the focused panel.
//
//nolint:gocognit // action dispatch with context-dependent behavior
func (a *App) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.helpBar.SetStatusMsg("")

	// Help popup: j/k navigate, esc/q close, other keys ignored.
	if a.showHelp {
		bindings := a.ContextBindings()
		switch msg.String() {
		case "j", "down":
			if a.helpCursor < len(bindings)-1 {
				a.helpCursor++
			}
		case "k", "up":
			if a.helpCursor > 0 {
				a.helpCursor--
			}
		case "esc", "q", "?":
			a.showHelp = false
		}
		return a, nil
	}

	var cmds []tea.Cmd
	action := a.keymap.Match(msg.String())
	switch action {
	case ActQuit:
		return a, tea.Quit

	case ActHelp:
		a.showHelp = true
		a.helpCursor = 0
		return a, nil

	case ActSearch:
		a.searchBar.Activate()
		return a, nil

	case ActSwitchPanel:
		if a.side == sideLeft {
			a.side = sideRight
		} else {
			a.side = sideLeft
		}
		a.updateFocusState()
		return a, nil

	case ActFocusRight:
		if a.side == sideLeft {
			a.side = sideRight
			a.updateFocusState()
			return a, nil
		}

	case ActFocusLeft:
		if a.side == sideRight {
			a.side = sideLeft
			a.updateFocusState()
			return a, nil
		}

	case ActSelect:
		return a.handleActionSelect()

	case ActOpen:
		return a.handleActionOpen()

	case ActPrevTab:
		if a.side == sideRight {
			a.detailView.PrevTab()
		} else if a.side == sideLeft && a.leftFocus == focusIssues {
			a.issuesList.PrevTab()
			if !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab()
			}
		}
		return a, nil
	case ActNextTab:
		if a.side == sideRight {
			a.detailView.NextTab()
		} else if a.side == sideLeft && a.leftFocus == focusIssues {
			a.issuesList.NextTab()
			if !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab()
			}
		}
		return a, nil

	case ActFocusDetail:
		a.side = sideRight
		a.updateFocusState()
		return a, nil

	case ActFocusStatus:
		a.side = sideLeft
		a.leftFocus = focusStatus
		a.splashInfo.Project = a.projectKey
		a.detailView.SetSplash(a.splashInfo)
		a.updateFocusState()
		return a, nil
	case ActFocusIssues:
		a.side = sideLeft
		a.leftFocus = focusIssues
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			a.showCachedIssue(sel.Key)
		}
		a.updateFocusState()
		return a, nil
	case ActFocusProj:
		a.side = sideLeft
		a.leftFocus = focusProjects
		a.updateFocusState()
		return a, nil

	case ActCopyURL:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			copyToClipboard(a.cfg.Jira.Host + "/browse/" + sel.Key)
		}
		return a, nil

	case ActBrowser:
		if sel := a.issuesList.SelectedIssue(); sel != nil && (a.leftFocus == focusIssues || a.side == sideRight) {
			openBrowser(a.cfg.Jira.Host + "/browse/" + sel.Key)
		}
		return a, nil

	case ActURLPicker:
		return a.handleActionURLPicker()

	case ActTransition:
		if a.side == sideLeft && a.leftFocus == focusIssues {
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				*a.logFlag = true
				return a, fetchTransitions(a.client, sel.Key)
			}
		}
		return a, nil

	case ActComments:
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return a, nil
		}
		a.side = sideRight
		a.detailView.SetActiveTab(views.TabComments)
		a.updateFocusState()
		if _, ok := a.issueCache[sel.Key]; !ok {
			return a, fetchIssueDetail(a.client, sel.Key)
		}
		return a, nil

	case ActAddComment:
		sel := a.issuesList.SelectedIssue()
		if sel == nil || a.side != sideRight || a.detailView.ActiveTab() != views.TabComments {
			return a, nil
		}
		a.editContext = editCtx{kind: editCommentNew, issueKey: sel.Key}
		return a, launchEditor("", ".md")

	case ActEdit:
		return a.handleActionEdit()

	case ActEditPriority:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			*a.logFlag = true
			return a, fetchPriorities(a.client)
		}
		return a, nil

	case ActEditAssignee:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			*a.logFlag = true
			a.onSelect = a.makePersonSelectCallback("assignee")
			return a, fetchUsers(a.client, a.projectKey, sel.Key)
		}
		return a, nil

	case ActCreateBranch:
		return a.handleActionCreateBranch()

	case ActJQLSearch:
		history := LoadJQLHistory()
		prefill := ""
		if a.projectKey != "" {
			prefill = "project = " + a.projectKey + " AND "
		}
		a.jqlModal.Show(prefill, history)
		if a.jqlFields == nil {
			cmds = append(cmds, fetchJQLAutocompleteData(a.client))
		}
		return a, tea.Batch(cmds...)

	case ActCloseJQLTab:
		if a.side == sideLeft && a.leftFocus == focusIssues && a.issuesList.IsJQLTab() {
			a.issuesList.RemoveJQLTab()
			if !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab()
			}
		}
		return a, nil

	case ActRefresh:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			*a.logFlag = true
			return a, fetchIssueDetail(a.client, sel.Key)
		}
		return a, nil

	case ActRefreshAll:
		return a, a.fetchActiveTab()

	default:
		// ActInfoTab and nav keys are handled by the focused panel below.
		return nil, nil
	}
	return a, nil
}

// handleActionSelect handles space/enter on the left panel.
// Returns nil model on sideRight so the key falls through to the detail panel.
func (a *App) handleActionSelect() (tea.Model, tea.Cmd) {
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			a.issuesList.SetActiveKey(sel.Key)
			a.side = sideRight
			a.updateFocusState()
			return a, fetchIssueDetail(a.client, sel.Key)
		}
		return a, nil
	case a.side == sideLeft && a.leftFocus == focusProjects:
		if p := a.projectList.SelectedProject(); p != nil {
			a.selectProject(p)
			a.leftFocus = focusIssues
			a.updateFocusState()
			return a, a.fetchActiveTab()
		}
		return a, nil
	}
	// sideRight: let detail view handle expand via its Update.
	return nil, nil
}

// handleActionOpen handles l/enter to preview without full select.
// Returns nil model on sideRight so the key falls through to the detail panel.
func (a *App) handleActionOpen() (tea.Model, tea.Cmd) {
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			a.side = sideRight
			a.updateFocusState()
			return a, fetchIssueDetail(a.client, sel.Key)
		}
		return a, nil
	case a.side == sideLeft && a.leftFocus == focusProjects:
		if p := a.projectList.SelectedProject(); p != nil {
			a.detailView.SetProject(p)
		}
		return a, nil
	}
	// sideRight: let detail view handle expand via its Update.
	return nil, nil
}

// handleActionURLPicker shows the URL picker modal.
func (a *App) handleActionURLPicker() (tea.Model, tea.Cmd) {
	if sel := a.issuesList.SelectedIssue(); sel != nil {
		cached := sel
		if c, ok := a.issueCache[sel.Key]; ok {
			cached = c
		}
		groups := views.ExtractURLs(cached, a.cfg.Jira.Host)
		if len(groups) > 0 {
			var items []components.ModalItem
			for i, g := range groups {
				if i > 0 || len(groups) > 1 {
					items = append(items, components.ModalItem{Label: g.Section, Separator: true})
				}
				for _, u := range g.URLs {
					display := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
					if key := a.extractIssueKey(u); key != "" {
						items = append(items, components.ModalItem{ID: u, Label: key + " " + display, Internal: true})
					} else {
						items = append(items, components.ModalItem{ID: u, Label: display})
					}
				}
			}
			a.modal.Show("URLs", items)
		}
	}
	return a, nil
}

// handleActionEdit dispatches context-aware editing.
func (a *App) handleActionEdit() (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	if a.side == sideLeft && a.leftFocus == focusIssues {
		a.inputModal.Show("Edit Summary", sel.Summary)
		a.editContext = editCtx{kind: editSummary, issueKey: sel.Key}
		return a, nil
	}
	if a.side == sideRight && a.detailView.ActiveTab() == views.TabComments {
		cmt := a.detailView.SelectedComment()
		if cmt == nil {
			return a, nil
		}
		md := ""
		if cmt.BodyADF != nil {
			md = views.ADFToMarkdown(cmt.BodyADF)
		} else if cmt.Body != "" {
			md = cmt.Body
		}
		a.editContext = editCtx{kind: editCommentMod, issueKey: sel.Key, commentID: cmt.ID}
		return a, launchEditor(md, ".md")
	}
	if a.side == sideRight && a.detailView.ActiveTab() == views.TabInfo {
		return a.editInfoField(sel)
	}
	// Default: edit description.
	cached := sel
	if c, ok := a.issueCache[sel.Key]; ok {
		cached = c
	}
	md := ""
	if cached.DescriptionADF != nil {
		md = views.ADFToMarkdown(cached.DescriptionADF)
	} else if cached.Description != "" {
		md = cached.Description
	}
	a.editContext = editCtx{kind: editDesc, issueKey: sel.Key}
	return a, launchEditor(md, ".md")
}

// handleActionCreateBranch opens the branch creation input.
func (a *App) handleActionCreateBranch() (tea.Model, tea.Cmd) {
	if a.side != sideLeft || a.leftFocus != focusIssues {
		return a, nil
	}
	if a.gitRepoPath == "" {
		a.err = errors.New("not a git repository")
		return a, nil
	}
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	parts := strings.SplitN(sel.Key, "-", 2)
	projKey := parts[0]
	number := ""
	if len(parts) > 1 {
		number = parts[1]
	}
	typeName := ""
	if sel.IssueType != nil {
		typeName = sel.IssueType.Name
	}
	data := git.BranchTemplateData{
		Key:        sel.Key,
		ProjectKey: projKey,
		Number:     number,
		Summary:    git.SanitizeSummary(sel.Summary, a.cfg.Git.AsciiOnly),
		Type:       typeName,
	}
	tmplStr := ""
	for _, r := range a.cfg.Git.BranchFormat {
		if r.When.Type == "*" || strings.EqualFold(r.When.Type, typeName) {
			tmplStr = r.Template
			break
		}
	}
	name := git.GenerateBranchName(data, tmplStr, a.cfg.Git.AsciiOnly)
	a.inputModal.Show("Create branch", name)
	a.editContext = editCtx{kind: editBranch}
	if result, err := git.SearchBranches(a.gitRepoPath, sel.Key); err == nil {
		hints := make([]string, 0, len(result.Local)+len(result.Remote))
		hints = append(hints, result.Local...)
		hints = append(hints, result.Remote...)
		if len(hints) > 0 {
			a.inputModal.SetHints(hints)
		}
	}
	return a, nil
}

// selectProject sets the active project, clearing issue state.
func (a *App) selectProject(p *jira.Project) {
	a.projectKey = p.Key
	a.projectID = p.ID
	a.statusPanel.SetProject(p.Key)
	a.projectList.SetActiveKey(p.Key)
	a.issuesList.ClearActiveKey()
	a.issuesList.InvalidateTabCache()
	a.issueCache = make(map[string]*jira.Issue)
	if !a.demoMode {
		go saveLastProject(p.Key)
	}
}

// routeToPanel forwards input to the focused panel.
func (a *App) routeToPanel(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	if a.side == sideLeft {
		switch a.leftFocus {
		case focusIssues:
			updated, cmd := a.issuesList.Update(msg)
			a.issuesList = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case focusProjects:
			updated, cmd := a.projectList.Update(msg)
			a.projectList = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case focusStatus:
			updated, cmd := a.statusPanel.Update(msg)
			a.statusPanel = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	} else {
		updated, cmd := a.detailView.Update(msg)
		a.detailView = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}
