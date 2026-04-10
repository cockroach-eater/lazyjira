package tui

// isVerticalLayout returns true when terminal is too narrow for side-by-side.
func (a *App) isVerticalLayout() bool {
	return a.width < 80
}

// sideWidth calculates the left panel width, shrinking for narrow terminals.
func (a *App) sideWidth() int {
	if a.isVerticalLayout() {
		return a.width
	}
	sideW := a.cfg.GUI.SidePanelWidth
	if sideW <= 0 {
		sideW = 40
	}
	if a.width < 120 && sideW > a.width*35/100 {
		sideW = a.width * 35 / 100
	}
	if sideW > a.width/2 {
		sideW = a.width / 2
	}
	if sideW < 25 {
		sideW = 25
	}
	return sideW
}

func (a *App) layoutPanels() {
	totalH := a.height - 1

	if a.isVerticalLayout() {
		w := a.width
		logH := 5

		// Priority-based accordion for vertical layout.
		// Non-focused left panels are collapsed (1 line).
		// Detail always gets its target first, then remaining
		// space goes to the focused left panel.
		const (
			detailTarget = 12 // enough for issue description
			projectsCap  = 8  // hard cap for projects
			infoCap      = 8  // hard cap for info
		)

		statusH := 1 // always collapsed in vertical
		issuesH := 1
		infoH := 1
		projectsH := 1
		var detailH int

		avail := totalH - statusH - logH // space for detail + issues + info + projects

		switch {
		case a.side == sideRight:
			// Detail focused: all left panels collapsed.
			detailH = avail - issuesH - infoH - projectsH

		case a.leftFocus == focusIssues:
			// Issues focused: others collapsed.
			remaining := avail - infoH - projectsH
			detailH = min(detailTarget, max(remaining-1, 1))
			issuesH = remaining - detailH

		case a.leftFocus == focusInfo:
			// Info focused: others collapsed.
			remaining := avail - issuesH - projectsH
			detailH = min(detailTarget, max(remaining-1, 1))
			infoH = min(remaining-detailH, infoCap)
			detailH = remaining - infoH

		case a.leftFocus == focusProjects:
			// Projects focused: others collapsed.
			remaining := avail - issuesH - infoH
			detailH = min(detailTarget, max(remaining-1, 1))
			projectsH = min(remaining-detailH, projectsCap)
			detailH = remaining - projectsH

		case a.leftFocus == focusStatus:
			statusH = 3
			avail -= 2 // status took 2 extra
			remaining := avail - infoH - projectsH
			detailH = min(detailTarget, max(remaining-1, 1))
			issuesH = remaining - detailH

		default:
			detailH = avail - issuesH - infoH - projectsH
		}

		if issuesH < 1 {
			issuesH = 1
		}
		if infoH < 1 {
			infoH = 1
		}
		if projectsH < 1 {
			projectsH = 1
		}
		if detailH < 3 {
			detailH = 3
		}

		a.statusPanel.SetSize(w, statusH)
		a.issuesList.SetSize(w, issuesH)
		a.infoPanel.SetSize(w, infoH)
		a.projectList.SetSize(w, projectsH)
		a.detailView.SetSize(w, detailH)
		a.logPanel.SetSize(w, logH)
		a.panelSideW = w
		a.panelStatusH = statusH
		a.panelIssuesH = issuesH
		a.panelInfoH = infoH
		a.panelProjectsH = projectsH
		a.panelDetailH = detailH
		a.panelLogH = logH
		return
	}

	sideW := a.sideWidth()
	mainW := a.width - sideW

	statusH := 3
	remaining := totalH - statusH

	var issuesH, infoH, projectsH int
	minH := 3
	collapsedH := max(a.cfg.GUI.CollapsedPanelHeight, minH)

	switch a.leftFocus {
	case focusProjects:
		// Projects focused: info and issues compact.
		infoH = collapsedH
		issuesNat := max(min(a.issuesList.ContentHeight(), 12), minH)
		issuesH = max(min(issuesNat, remaining/3), minH)
		projectsH = remaining - issuesH - infoH
	case focusInfo:
		// Info focused: issues compact, projects compact.
		projectsH = collapsedH
		issuesNat := max(min(a.issuesList.ContentHeight(), 12), minH)
		issuesH = max(min(issuesNat, remaining/3), minH)
		infoH = remaining - issuesH - projectsH
	default:
		// Issues focused (or status): info and projects compact, issues gets space.
		infoH = collapsedH
		projectsH = collapsedH
		issuesH = remaining - infoH - projectsH
	}

	if issuesH < minH {
		issuesH = minH
	}
	if infoH < minH {
		infoH = minH
	}
	if projectsH < minH {
		projectsH = minH
	}

	a.statusPanel.SetSize(sideW, statusH)
	a.issuesList.SetSize(sideW, issuesH)
	a.infoPanel.SetSize(sideW, infoH)
	a.projectList.SetSize(sideW, projectsH)

	// Right column: log fits content or max 8, detail gets the rest.
	logH := 8
	detailH := max(totalH-logH, 5)

	a.detailView.SetSize(mainW, detailH)
	a.logPanel.SetSize(mainW, logH)

	// Cache sizes for mouse.
	a.panelSideW = sideW
	a.panelStatusH = statusH
	a.panelIssuesH = issuesH
	a.panelInfoH = infoH
	a.panelProjectsH = projectsH
	a.panelDetailH = detailH
	a.panelLogH = logH
}
