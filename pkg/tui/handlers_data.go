package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
)

// handleIssuesLoaded processes newly fetched issues.
func (a *App) handleIssuesLoaded(msg issuesLoadedMsg) (tea.Model, tea.Cmd) {
	a.statusPanel.SetError("")
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
					a.gitDetectedKey = ""
					return a, tea.Batch(cmds...)
				}
			}
		}
		a.issuesList.SelectByKey(detectedKey)
		if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == detectedKey {
			a.issuesList.SetActiveKey(detectedKey)
			cmds = append(cmds, fetchIssueDetail(a.client, detectedKey))
		}
		a.gitDetectedKey = ""
	}
	return a, tea.Batch(cmds...)
}

// handleIssueDetailLoaded updates the detail view with full issue data.
func (a *App) handleIssueDetailLoaded(msg issueDetailLoadedMsg) (tea.Model, tea.Cmd) {
	a.statusPanel.SetError("")
	*a.logFlag = false
	a.statusPanel.SetOnline(true)
	a.issueCache[msg.issue.Key] = msg.issue
	// Only update detail view if it's showing this issue (don't clobber preview of a different issue).
	if a.detailView.IssueKey() == "" || a.detailView.IssueKey() == msg.issue.Key {
		a.detailView.UpdateIssueData(msg.issue)
	}
	// Keep info panel in sync — only if it's already showing this issue.
	if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == msg.issue.Key {
		if a.infoPanel.IssueKey() == "" || a.infoPanel.IssueKey() == msg.issue.Key {
			a.infoPanel.SetIssue(msg.issue)
		}
	}
	// Update issue in the list so changes appear immediately.
	a.issuesList.PatchIssue(msg.issue)

	// Prefetch linked issues and subtasks for instant preview.
	return a, a.prefetchRelated(msg.issue)
}

// handleIssuePrefetched caches prefetched issue data silently.
func (a *App) handleIssuePrefetched(msg issuePrefetchedMsg) (tea.Model, tea.Cmd) {
	if msg.issue == nil {
		return a, nil
	}
	a.issueCache[msg.issue.Key] = msg.issue
	if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == msg.issue.Key {
		// Only update panels if they're already showing this issue (don't clobber preview).
		if a.detailView.IssueKey() == "" || a.detailView.IssueKey() == msg.issue.Key {
			a.detailView.UpdateIssueData(msg.issue)
		}
		if a.infoPanel.IssueKey() == "" || a.infoPanel.IssueKey() == msg.issue.Key {
			a.infoPanel.SetIssue(msg.issue)
		}
	}
	return a, nil
}

// handleTransitionDone re-fetches data after a transition.
func (a *App) handleTransitionDone() (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	return a, tea.Batch(
		fetchIssueDetail(a.client, sel.Key),
		a.fetchActiveTab(),
	)
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
	issueKey := msg.issueKey
	a.onSelect = func(item components.ModalItem) tea.Cmd {
		return doTransition(a.client, issueKey, item.ID)
	}
	a.modal.Show("Transition: "+issueKey, items)
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
	currentAssigneeID := ""
	if sel.Assignee != nil {
		currentAssigneeID = sel.Assignee.AccountID
	}
	var items []components.ModalItem
	items = append(items, components.ModalItem{ID: "", Label: "Unassigned", Active: currentAssigneeID == ""})
	email := a.cfg.Jira.Email
	for _, u := range msg.users {
		if u.Email == email {
			items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName, Active: u.AccountID == currentAssigneeID})
			break
		}
	}
	for _, u := range msg.users {
		if u.Email != email {
			items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName, Active: u.AccountID == currentAssigneeID})
		}
	}
	// onSelect callback set at fetch time (ActEditAssignee or editInfoField).
	a.modal.Show("Assignee: "+sel.Key, items)
	return a, nil
}

// handleBoardsLoaded caches boards and resolves the board for the current project.
func (a *App) handleBoardsLoaded(msg boardsLoadedMsg) (tea.Model, tea.Cmd) {
	a.boards = msg.boards
	a.resolveBoardID()
	return a, nil
}

func (a *App) resolveBoardID() {
	a.boardID = 0
	for _, b := range a.boards {
		if b.ProjectKey == a.projectKey {
			a.boardID = b.ID
			return
		}
	}
}

// handleSprintsLoaded shows the sprint picker modal.
func (a *App) handleSprintsLoaded(msg sprintsLoadedMsg) (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	currentSprintID := 0
	if sel.Sprint != nil {
		currentSprintID = sel.Sprint.ID
	}
	var items []components.ModalItem
	items = append(items, components.ModalItem{ID: "0", Label: "None", Active: currentSprintID == 0})
	for _, s := range msg.sprints {
		if s.State == "closed" {
			continue
		}
		label := s.Name
		if s.State == "active" {
			label += " (active)"
		}
		items = append(items, components.ModalItem{
			ID:     strconv.Itoa(s.ID),
			Label:  label,
			Active: s.ID == currentSprintID,
		})
	}
	issueKey := sel.Key
	a.onSelect = func(item components.ModalItem) tea.Cmd {
		sprintID, _ := strconv.Atoi(item.ID)
		if sprintID == 0 {
			return updateIssueField(a.client, issueKey, "sprint", nil)
		}
		return moveToSprint(a.client, sprintID, issueKey)
	}
	a.modal.Show("Sprint: "+sel.Key, items)
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
		a.resolveBoardID()
		return a, a.fetchActiveTab()
	}
	return a, nil
}

// prefetchRelated batches a single JQL search for all linked issues and subtasks not yet in cache.
func (a *App) prefetchRelated(issue *jira.Issue) tea.Cmd {
	if issue == nil {
		return nil
	}
	seen := make(map[string]bool)
	var keys []string

	collect := func(key string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		if _, ok := a.issueCache[key]; !ok {
			keys = append(keys, key)
		}
	}

	for _, sub := range issue.Subtasks {
		collect(sub.Key)
	}
	for _, link := range issue.IssueLinks {
		if link.OutwardIssue != nil {
			collect(link.OutwardIssue.Key)
		}
		if link.InwardIssue != nil {
			collect(link.InwardIssue.Key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return batchPrefetch(a.client, keys)
}

// handleBatchPrefetched caches all issues from a batch prefetch.
func (a *App) handleBatchPrefetched(msg batchPrefetchedMsg) (tea.Model, tea.Cmd) {
	for i := range msg.issues {
		a.issueCache[msg.issues[i].Key] = &msg.issues[i]
	}
	return a, nil
}

// handleIssueUpdated re-fetches issue data after an update.
// Only refreshes the issue detail, not the tab — avoids cursor jumping
// when the edited issue no longer matches the tab JQL (e.g. unassign on "Assigned" tab).
func (a *App) handleIssueUpdated(msg issueUpdatedMsg) (tea.Model, tea.Cmd) {
	return a, fetchIssueDetail(a.client, msg.issueKey)
}
