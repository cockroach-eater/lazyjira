package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/tui/components"
)

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
					a.gitDetectedKey = ""
					return a, tea.Batch(cmds...)
				}
			}
		}
		a.issuesList.SelectByKey(detectedKey)
		if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == detectedKey {
			cmds = append(cmds, fetchIssueDetail(a.client, detectedKey))
		}
		a.gitDetectedKey = ""
	}
	return a, tea.Batch(cmds...)
}

// handleIssueDetailLoaded updates the detail view with full issue data.
func (a *App) handleIssueDetailLoaded(msg issueDetailLoadedMsg) (tea.Model, tea.Cmd) {
	a.err = nil
	*a.logFlag = false
	a.statusPanel.SetOnline(true)
	a.issueCache[msg.issue.Key] = msg.issue
	a.detailView.UpdateIssueData(msg.issue)
	return a, nil
}

// handleIssuePrefetched caches prefetched issue data silently.
func (a *App) handleIssuePrefetched(msg issuePrefetchedMsg) (tea.Model, tea.Cmd) {
	if msg.issue == nil {
		return a, nil
	}
	a.issueCache[msg.issue.Key] = msg.issue
	if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == msg.issue.Key {
		a.detailView.UpdateIssueData(msg.issue)
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

// handleIssueUpdated re-fetches issue data after an update.
func (a *App) handleIssueUpdated(msg issueUpdatedMsg) (tea.Model, tea.Cmd) {
	return a, tea.Batch(
		fetchIssueDetail(a.client, msg.issueKey),
		a.fetchActiveTab(),
	)
}
