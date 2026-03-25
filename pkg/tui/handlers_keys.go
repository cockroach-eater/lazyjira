package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/git"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

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
			a.onSelect = func(item components.ModalItem) tea.Cmd {
				id := item.ID
				if strings.HasPrefix(id, "http") {
					if issueKey := a.extractIssueKey(id); issueKey != "" {
						a.navigateToIssue(issueKey)
						return nil
					}
					openBrowser(id)
				}
				return nil
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
