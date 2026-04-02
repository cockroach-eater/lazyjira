package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type panelID int

const (
	panelStatus panelID = iota
	panelIssues
	panelInfo
	panelProjects
	panelDetail
	panelLog
)

func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	x, y := msg.X, msg.Y
	panel, relY := a.hitTest(x, y)

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		return a.mouseScroll(panel, -3)
	case msg.Button == tea.MouseButtonWheelDown:
		return a.mouseScroll(panel, 3)
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		return a.mouseClick(panel, relY, x)
	}
	return a, nil
}

// hitTest determines which panel the coordinates fall in and the relative Y.
func (a *App) hitTest(x, y int) (panelID, int) {
	if a.isVerticalLayout() {
		// Vertical: all stacked.
		top := 0
		if y < top+a.panelStatusH {
			return panelStatus, y - top
		}
		top += a.panelStatusH
		if y < top+a.panelIssuesH {
			return panelIssues, y - top
		}
		top += a.panelIssuesH
		if y < top+a.panelInfoH {
			return panelInfo, y - top
		}
		top += a.panelInfoH
		if y < top+a.panelProjectsH {
			return panelProjects, y - top
		}
		top += a.panelProjectsH
		if y < top+a.panelDetailH {
			return panelDetail, y - top
		}
		return panelLog, y - top - a.panelDetailH
	}

	// Horizontal layout.
	if x < a.panelSideW {
		top := 0
		if y < top+a.panelStatusH {
			return panelStatus, y - top
		}
		top += a.panelStatusH
		if y < top+a.panelIssuesH {
			return panelIssues, y - top
		}
		top += a.panelIssuesH
		if y < top+a.panelInfoH {
			return panelInfo, y - top
		}
		top += a.panelInfoH
		return panelProjects, y - top
	}

	// Right side.
	if y < a.panelDetailH {
		return panelDetail, y
	}
	return panelLog, y - a.panelDetailH
}

func (a *App) mouseScroll(panel panelID, delta int) (tea.Model, tea.Cmd) {
	switch panel { //nolint:exhaustive
	case panelIssues:
		if a.side != sideLeft || a.leftFocus != focusIssues {
			a.side = sideLeft
			a.leftFocus = focusIssues
			a.updateFocusState()
		}
		if delta > 0 {
			a.issuesList.ScrollBy(1)
		} else {
			a.issuesList.ScrollBy(-1)
		}
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			a.showCachedIssue(sel.Key)
		}
	case panelInfo:
		if a.side != sideLeft || a.leftFocus != focusInfo {
			a.side = sideLeft
			a.leftFocus = focusInfo
			a.updateFocusState()
		}
		if delta > 0 {
			a.infoPanel.ScrollBy(1)
		} else {
			a.infoPanel.ScrollBy(-1)
		}
	case panelProjects:
		if a.side != sideLeft || a.leftFocus != focusProjects {
			a.side = sideLeft
			a.leftFocus = focusProjects
			a.updateFocusState()
		}
		if delta > 0 {
			a.projectList.ScrollBy(1)
		} else {
			a.projectList.ScrollBy(-1)
		}
		if p := a.projectList.SelectedProject(); p != nil {
			a.detailView.SetProject(p)
		}
	case panelDetail:
		if a.side != sideRight {
			a.side = sideRight
			a.updateFocusState()
		}
		if delta > 0 {
			a.detailView.ScrollBy(1)
		} else {
			a.detailView.ScrollBy(-1)
		}
	}
	return a, nil
}

func (a *App) mouseClick(panel panelID, relY int, x int) (tea.Model, tea.Cmd) {
	switch panel { //nolint:exhaustive
	case panelStatus:
		a.side = sideLeft
		a.leftFocus = focusStatus
		a.splashInfo.Project = a.projectKey
		a.detailView.SetSplash(a.splashInfo)
		a.updateFocusState()

	case panelIssues:
		a.side = sideLeft
		a.leftFocus = focusIssues
		a.updateFocusState()
		if relY == 0 {
			// Title bar — tab click.
			if a.issuesList.ClickTabAt(x) && !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab()
			}
		} else if dbl := a.issuesList.ClickAt(relY); dbl {
			// Double-click → open issue (same as Enter).
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				a.issuesList.SetActiveKey(sel.Key)
				a.side = sideRight
				a.updateFocusState()
				return a, fetchIssueDetail(a.client, sel.Key)
			}
		} else if sel := a.issuesList.SelectedIssue(); sel != nil {
			a.showCachedIssue(sel.Key)
		}

	case panelInfo:
		a.side = sideLeft
		a.leftFocus = focusInfo
		a.updateFocusState()
		if relY == 0 {
			a.infoPanel.ClickTabAt(x)
		} else {
			a.infoPanel.ClickAt(relY)
		}

	case panelProjects:
		a.side = sideLeft
		a.leftFocus = focusProjects
		a.updateFocusState()
		if dbl := a.projectList.ClickAt(relY); dbl {
			// Double-click → select project (same as Enter).
			if p := a.projectList.SelectedProject(); p != nil {
				prefetch := a.selectProject(p)
				a.leftFocus = focusIssues
				a.updateFocusState()
				return a, tea.Batch(a.fetchActiveTab(), prefetch)
			}
		} else if p := a.projectList.SelectedProject(); p != nil {
			a.detailView.SetProject(p)
		}

	case panelDetail:
		a.side = sideRight
		a.updateFocusState()
		if relY == 0 {
			// Title bar → tab click.
			relX := x
			if !a.isVerticalLayout() {
				relX = x - a.panelSideW
			}
			a.detailView.ClickTab(relX)
		} else {
			if cmd := a.detailView.ClickItem(relY); cmd != nil {
				return a, cmd
			}
		}
	}
	return a, nil
}
