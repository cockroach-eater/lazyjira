package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
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
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		a.issuesList.SetFilter(msg.Query)
	case a.side == sideLeft && a.leftFocus == focusInfo:
		a.infoPanel.SetFilter(msg.Query)
	case a.side == sideLeft && a.leftFocus == focusProjects:
		a.projectList.SetFilter(msg.Query)
	}
	return a, nil
}

// handleSearchConfirmed finalizes search: selects the filtered item and loads data.
func (a *App) handleSearchConfirmed() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		selectedIssue := a.issuesList.SelectedIssue()
		a.issuesList.ClearFilter()
		if selectedIssue != nil {
			a.issuesList.SelectByKey(selectedIssue.Key)
			cmds = append(cmds, fetchIssueDetail(a.client, selectedIssue.Key))
		}
	case a.side == sideLeft && a.leftFocus == focusInfo:
		a.infoPanel.ClearFilter()
	case a.side == sideLeft && a.leftFocus == focusProjects:
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
	a.infoPanel.SetFilter("")
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

// selectProject sets the active project, clearing issue state.
func (a *App) selectProject(p *jira.Project) {
	a.projectKey = p.Key
	a.projectID = p.ID
	a.statusPanel.SetProject(p.Key)
	a.projectList.SetActiveKey(p.Key)
	a.issuesList.ClearActiveKey()
	a.issuesList.InvalidateTabCache()
	a.issueCache = make(map[string]*jira.Issue)
	a.infoPanel.SetIssue(nil)
	a.resolveBoardID()
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
		case focusInfo:
			updated, cmd := a.infoPanel.Update(msg)
			a.infoPanel = updated
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
