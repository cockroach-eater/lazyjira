package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/git"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// handleKeyMsg dispatches keyboard actions.
// Returns (nil, nil) if the key was not handled and should be forwarded to the focused panel
func (a *App) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.helpBar.SetStatusMsg("")

	if a.showHelp {
		return a.handleHelpKeys(msg)
	}

	action := a.keymap.Match(msg.String())

	switch action { //nolint:exhaustive
	case ActQuit:
		return a, tea.Quit
	case ActHelp:
		a.showHelp = true
		a.helpCursor = 0
		a.helpFilter = ""
		a.helpSearching = false
		return a, nil
	case ActSearch:
		a.searchBar.Activate()
		return a, nil
	case ActSelect:
		return a.handleActionSelect()
	case ActOpen:
		return a.handleActionOpen()
	case ActURLPicker:
		return a.handleActionURLPicker()
	case ActEdit:
		return a.handleActionEdit()
	case ActCreateBranch:
		return a.handleActionCreateBranch()
	}

	if m, cmd, ok := a.handleFocusAction(action); ok {
		return m, cmd
	}
	if m, cmd, ok := a.handleTabAction(action); ok {
		return m, cmd
	}
	if m, cmd, ok := a.handleIssueAction(action); ok {
		return m, cmd
	}

	if a.side == sideLeft && (a.leftFocus == focusIssues || a.leftFocus == focusInfo) {
		if m, cmd, ok := a.handleDetailScroll(msg); ok {
			return m, cmd
		}
	}

	return nil, nil
}

func (a *App) handleDetailScroll(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch a.keymap.Match(msg.String()) { //nolint:exhaustive
	case ActDetailScrollDown:
		a.detailView.ScrollBy(1)
		return a, nil, true
	case ActDetailScrollUp:
		a.detailView.ScrollBy(-1)
		return a, nil, true
	case ActDetailHalfDown:
		a.detailView.ScrollBy(a.detailView.VisibleRows() / 2)
		return a, nil, true
	case ActDetailHalfUp:
		a.detailView.ScrollBy(-a.detailView.VisibleRows() / 2)
		return a, nil, true
	}
	return nil, nil, false
}

func (a *App) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.helpSearching {
		return a.handleHelpSearchKey(msg)
	}

	bindings := a.filteredHelpBindings()
	key := msg.String()

	if key == "/" {
		a.helpSearching = true
		a.helpSearch.SetValue("")
		a.helpSearch.SetWidth(a.width - 5)
		return a, nil
	}
	if key == "esc" || key == "q" || key == "?" {
		a.showHelp = false
		a.helpFilter = ""
		a.helpSearching = false
		return a, nil
	}

	switch a.keymap.MatchNav(key) {
	case components.NavNone:
	case components.NavDown:
		if a.helpCursor < len(bindings)-1 {
			a.helpCursor++
		}
	case components.NavUp:
		if a.helpCursor > 0 {
			a.helpCursor--
		}
	case components.NavTop:
		a.helpCursor = 0
	case components.NavBottom:
		if len(bindings) > 0 {
			a.helpCursor = len(bindings) - 1
		}
	case components.NavHalfDown:
		halfPage := max(1, min(len(bindings), a.height-8)/2)
		a.helpCursor += halfPage
		if a.helpCursor >= len(bindings) {
			a.helpCursor = len(bindings) - 1
		}
	case components.NavHalfUp:
		halfPage := max(1, min(len(bindings), a.height-8)/2)
		a.helpCursor -= halfPage
		if a.helpCursor < 0 {
			a.helpCursor = 0
		}
	}
	return a, nil
}

func (a *App) handleHelpSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := a.filteredHelpBindings()
	switch msg.String() {
	case "enter":
		a.helpConfirmSearch()
	case "esc":
		a.helpSearching = false
		a.helpFilter = ""
		a.helpCursor = 0
	case "down", components.KeyCtrlJ:
		if a.helpCursor < len(filtered)-1 {
			a.helpCursor++
		}
	case "up", components.KeyCtrlK:
		if a.helpCursor > 0 {
			a.helpCursor--
		}
	default:
		updated, changed := a.helpSearch.Update(msg)
		a.helpSearch = updated
		if changed {
			a.helpFilter = a.helpSearch.Value()
			a.helpCursor = 0
		}
	}
	return a, nil
}

func (a *App) helpConfirmSearch() {
	var matchedBinding *Binding
	filtered := a.filteredHelpBindings()
	if a.helpCursor >= 0 && a.helpCursor < len(filtered) {
		b := filtered[a.helpCursor]
		matchedBinding = &b
	}
	a.helpSearching = false
	a.helpFilter = ""
	a.helpSearch.SetValue("")
	all := a.ContextBindings()
	a.helpCursor = 0
	if matchedBinding != nil {
		for i, b := range all {
			if b.Key == matchedBinding.Key && b.Description == matchedBinding.Description {
				a.helpCursor = i
				break
			}
		}
	}
}

func (a *App) filteredHelpBindings() []Binding {
	bindings := a.ContextBindings()
	if a.helpFilter == "" {
		return bindings
	}
	query := strings.ToLower(a.helpFilter)
	var result []Binding
	for _, b := range bindings {
		if strings.Contains(strings.ToLower(b.Key), query) ||
			strings.Contains(strings.ToLower(b.Description), query) {
			result = append(result, b)
		}
	}
	return result
}

func (a *App) handleFocusAction(action Action) (tea.Model, tea.Cmd, bool) {
	switch action { //nolint:exhaustive
	case ActSwitchPanel:
		if a.side == sideLeft {
			a.side = sideRight
		} else {
			a.side = sideLeft
		}
		a.updateFocusState()
		return a, nil, true

	case ActFocusRight:
		if a.side == sideLeft {
			switch a.leftFocus {
			case focusStatus:
				a.leftFocus = focusIssues
			case focusIssues:
				a.leftFocus = focusInfo
			case focusInfo:
				a.leftFocus = focusProjects
			case focusProjects:
				a.leftFocus = focusStatus
			}
			a.updateFocusState()
			return a, nil, true
		}

	case ActFocusLeft:
		if a.side == sideLeft {
			switch a.leftFocus {
			case focusStatus:
				a.leftFocus = focusProjects
			case focusIssues:
				a.leftFocus = focusStatus
			case focusInfo:
				a.leftFocus = focusIssues
			case focusProjects:
				a.leftFocus = focusInfo
			}
			a.updateFocusState()
			return a, nil, true
		}
		if a.side == sideRight {
			a.side = sideLeft
			a.updateFocusState()
			return a, nil, true
		}

	case ActFocusDetail:
		a.side = sideRight
		a.updateFocusState()
		return a, nil, true

	case ActFocusStatus:
		a.side = sideLeft
		a.leftFocus = focusStatus
		a.splashInfo.Project = a.projectKey
		a.detailView.SetSplash(a.splashInfo)
		a.updateFocusState()
		return a, nil, true

	case ActFocusIssues:
		a.side = sideLeft
		a.leftFocus = focusIssues
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			a.showCachedIssue(sel.Key)
		}
		a.updateFocusState()
		return a, nil, true

	case ActFocusInfo, ActInfoTab:
		a.side = sideLeft
		a.leftFocus = focusInfo
		a.updateFocusState()
		return a, nil, true

	case ActFocusProj:
		a.side = sideLeft
		a.leftFocus = focusProjects
		a.updateFocusState()
		return a, nil, true
	}
	return nil, nil, false
}

func (a *App) handleTabAction(action Action) (tea.Model, tea.Cmd, bool) {
	switch action { //nolint:exhaustive
	case ActPrevTab:
		switch {
		case a.side == sideRight:
			a.detailView.PrevTab()
		case a.side == sideLeft && a.leftFocus == focusIssues:
			a.issuesList.PrevTab()
			if !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab(), true
			}
			a.previewSelectedIssue()
		case a.side == sideLeft && a.leftFocus == focusInfo:
			a.infoPanel.PrevTab()
		}
		return a, nil, true

	case ActNextTab:
		switch {
		case a.side == sideRight:
			a.detailView.NextTab()
		case a.side == sideLeft && a.leftFocus == focusIssues:
			a.issuesList.NextTab()
			if !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab(), true
			}
			a.previewSelectedIssue()
		case a.side == sideLeft && a.leftFocus == focusInfo:
			a.infoPanel.NextTab()
		}
		return a, nil, true

	case ActCloseJQLTab:
		if a.side == sideLeft && a.leftFocus == focusIssues && a.issuesList.IsJQLTab() {
			a.issuesList.RemoveJQLTab()
			if !a.issuesList.HasCachedTab() {
				return a, a.fetchActiveTab(), true
			}
		}
		return a, nil, true

	case ActJQLSearch:
		history := LoadJQLHistory()
		prefill := ""
		if a.projectKey != "" {
			prefill = "project = " + a.projectKey + " AND "
		}
		a.jqlModal.Show(prefill, history)
		var cmds []tea.Cmd
		if a.jqlFields == nil {
			cmds = append(cmds, fetchJQLAutocompleteData(a.client))
		}
		return a, tea.Batch(cmds...), true
	}
	return nil, nil, false
}

func (a *App) handleIssueAction(action Action) (tea.Model, tea.Cmd, bool) {
	switch action { //nolint:exhaustive
	case ActCopyURL:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			copyToClipboard(a.cfg.Jira.Host + "/browse/" + sel.Key)
		}
		return a, nil, true

	case ActBrowser:
		if sel := a.issuesList.SelectedIssue(); sel != nil && (a.leftFocus == focusIssues || a.side == sideRight) {
			openBrowser(a.cfg.Jira.Host + "/browse/" + sel.Key)
		}
		return a, nil, true

	case ActTransition:
		if a.side == sideLeft && (a.leftFocus == focusIssues || a.leftFocus == focusInfo) {
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				*a.logFlag = true
				return a, fetchTransitions(a.client, sel.Key), true
			}
		}
		return a, nil, true

	case ActComments:
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return a, nil, true
		}
		a.side = sideRight
		a.detailView.SetActiveTab(views.TabComments)
		a.updateFocusState()
		if _, ok := a.issueCache[sel.Key]; !ok {
			return a, fetchIssueDetail(a.client, sel.Key), true
		}
		return a, nil, true

	case ActDuplicateIssue:
		if a.side == sideLeft && a.leftFocus == focusIssues {
			m, cmd := a.startDuplicateIssue()
			return m, cmd, true
		}
		return a, nil, true

	case ActCreateIssue:
		m, cmd := a.startCreateIssue()
		return m, cmd, true

	case ActNew:
		if a.side == sideLeft && a.leftFocus == focusIssues && a.projectKey != "" {
			m, cmd := a.startCreateIssue()
			return m, cmd, true
		}
		sel := a.issuesList.SelectedIssue()
		if sel == nil || a.side != sideRight || a.detailView.ActiveTab() != views.TabComments {
			return a, nil, true
		}
		a.editContext = editCtx{kind: editCommentNew, issueKey: sel.Key}
		return a, launchEditor("", ".md"), true

	case ActPriority:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			*a.logFlag = true
			return a, fetchPriorities(a.client), true
		}
		return a, nil, true

	case ActAssignee:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			*a.logFlag = true
			a.onSelect = a.makePersonSelectCallback(sel.Key, "assignee")
			if cached, ok := a.usersCache[a.projectKey]; ok {
				m, cmd := a.handleUsersLoaded(usersLoadedMsg{users: cached, issueKey: sel.Key})
				return m, cmd, true
			}
			return a, fetchUsers(a.client, a.projectKey, sel.Key), true
		}
		return a, nil, true

	case ActRefresh:
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			*a.logFlag = true
			return a, fetchIssueDetail(a.client, sel.Key), true
		}
		return a, nil, true

	case ActRefreshAll:
		return a, a.fetchActiveTab(), true
	}
	return nil, nil, false
}

func (a *App) startDuplicateIssue() (tea.Model, tea.Cmd) {
	sel := a.issuesList.SelectedIssue()
	if sel == nil || a.projectKey == "" {
		return a, nil
	}
	source := sel
	if cached, ok := a.issueCache[sel.Key]; ok {
		source = cached
	}
	a.createCtx = createCtx{
		intent:        true,
		projectKey:    a.projectKey,
		projectID:     a.projectID,
		duplicateFrom: source,
	}
	*a.logFlag = true
	return a, fetchIssueTypes(a.client, a.projectID)
}

func (a *App) startCreateIssue() (tea.Model, tea.Cmd) {
	if a.projectKey == "" {
		return a, nil
	}
	a.createCtx = createCtx{
		intent:     true,
		projectKey: a.projectKey,
		projectID:  a.projectID,
	}
	*a.logFlag = true
	return a, fetchIssueTypes(a.client, a.projectID)
}

func (a *App) handleActionSelect() (tea.Model, tea.Cmd) {
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		return a.openIssueDetail()
	case a.side == sideLeft && a.leftFocus == focusInfo:
		if a.infoPanel.ActiveTab() == views.InfoTabFields {
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return a.editInfoField(sel)
			}
			return a, nil
		}
		return a.navigateToLinkedIssue()
	case a.side == sideLeft && a.leftFocus == focusProjects:
		return a.openProject()
	}
	return nil, nil
}

func (a *App) handleActionOpen() (tea.Model, tea.Cmd) {
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		return a.openIssueDetail()
	case a.side == sideLeft && a.leftFocus == focusInfo:
		if a.infoPanel.ActiveTab() == views.InfoTabFields {
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return a.editInfoField(sel)
			}
			return a, nil
		}
		return a.openLinkedIssueDetail()
	case a.side == sideLeft && a.leftFocus == focusProjects:
		return a.openProject()
	}
	return nil, nil
}

func (a *App) openIssueDetail() (tea.Model, tea.Cmd) {
	if sel := a.issuesList.SelectedIssue(); sel != nil {
		a.side = sideRight
		a.updateFocusState()
		return a, fetchIssueDetail(a.client, sel.Key)
	}
	return a, nil
}

func (a *App) openProject() (tea.Model, tea.Cmd) {
	if p := a.projectList.SelectedProject(); p != nil {
		prefetch := a.selectProject(p)
		a.leftFocus = focusIssues
		a.updateFocusState()
		return a, tea.Batch(a.fetchActiveTab(), prefetch)
	}
	return a, nil
}

func (a *App) openLinkedIssueDetail() (tea.Model, tea.Cmd) {
	key := a.infoPanelSelectedKey()
	if key == "" {
		return a, nil
	}
	a.side = sideRight
	a.updateFocusState()
	a.showCachedIssue(key)
	return a, fetchIssueDetail(a.client, key)
}

func (a *App) navigateToLinkedIssue() (tea.Model, tea.Cmd) {
	key := a.infoPanelSelectedKey()
	if key == "" {
		return a, nil
	}
	if tab, found := a.issuesList.FindInAnyTab(key); found {
		if tab != a.issuesList.GetTabIndex() {
			a.issuesList.SetTabIndex(tab)
		}
	} else if cached, ok := a.issueCache[key]; ok {
		a.issuesList.InjectIssue(*cached)
		a.issuesList.SetTabIndex(0)
	}
	a.issuesList.SelectByKey(key)
	a.leftFocus = focusIssues
	a.updateFocusState()
	a.showCachedIssue(key)
	return a, fetchIssueDetail(a.client, key)
}

// infoPanelSelectedKey returns the issue key under cursor in Lnk/Sub tabs, or ""
func (a *App) infoPanelSelectedKey() string {
	if key := a.infoPanel.SelectedLinkKey(); key != "" {
		return key
	}
	return a.infoPanel.SelectedSubtaskKey()
}

// handleActionURLPicker shows the URL picker modal
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

// handleActionEdit dispatches context-aware editing
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
	if a.side == sideLeft && a.leftFocus == focusInfo {
		return a.editInfoField(sel)
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

// handleActionCreateBranch opens the branch creation input
func (a *App) handleActionCreateBranch() (tea.Model, tea.Cmd) {
	if a.side != sideLeft || a.leftFocus != focusIssues {
		return a, nil
	}
	if a.gitRepoPath == "" {
		a.statusPanel.SetError("not a git repository")
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
	parentKey := ""
	if sel.Parent != nil {
		parentKey = sel.Parent.Key
	}
	data := git.BranchTemplateData{
		Key:        sel.Key,
		ProjectKey: projKey,
		Number:     number,
		Summary:    git.SanitizeSummary(sel.Summary, a.cfg.Git.AsciiOnly),
		Type:       typeName,
		ParentKey:  parentKey,
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
