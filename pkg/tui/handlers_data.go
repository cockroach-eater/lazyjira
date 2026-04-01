package tui

import (
	"fmt"
	"maps"
	"sort"
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
					if cmd := a.selectProject(&p); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, a.fetchActiveTab())
					a.gitDetectedKey = ""
					return a, tea.Batch(cmds...)
				}
			}
		}
		switch {
		case a.issuesList.SelectByKey(detectedKey):
			a.issuesList.SetActiveKey(detectedKey)
			cmds = append(cmds, fetchIssueDetail(a.client, detectedKey))
			a.gitDetectedKey = ""
		case a.issuesList.GetTabIndex() != 0:
			// not in current tab, try All tab
			a.issuesList.SetTabIndex(0)
			cmds = append(cmds, a.fetchActiveTab())
		default:
			a.gitDetectedKey = ""
		}
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

// handlePrioritiesLoaded shows the priority picker modal
func (a *App) handlePrioritiesLoaded(msg prioritiesLoadedMsg) (tea.Model, tea.Cmd) {
	if len(msg.priorities) == 0 {
		return a, nil
	}
	var items []components.ModalItem
	for _, p := range msg.priorities {
		items = append(items, components.ModalItem{ID: p.ID, Label: p.Name})
	}
	// only set default callback if caller did not set one (e.g. create form sets its own)
	if a.onSelect == nil {
		a.onSelect = func(item components.ModalItem) tea.Cmd {
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return updateIssueField(a.client, sel.Key, fldPriority, map[string]string{"id": item.ID})
			}
			return nil
		}
	}
	a.modal.Show("Priority", items)
	return a, nil
}

// handleUsersLoaded shows the assignee/reporter picker modal.
func (a *App) handleUsersLoaded(msg usersLoadedMsg) (tea.Model, tea.Cmd) {
	// Cache users for this project
	if a.projectKey != "" && len(msg.users) > 0 {
		a.usersCache[a.projectKey] = msg.users
	}
	// Prefetch only caches, no modal
	if msg.issueKey == "" {
		return a, nil
	}
	// create form user picker (single-select or checklist)
	if msg.issueKey == createUsersSentinel {
		if a.onChecklist != nil {
			// multi-user field: show checklist with me at top
			a.modal.ShowChecklist("Select users", a.buildUserItems(msg.users), nil)
			return a, nil
		}
		return a.showCreateUserPicker(msg.users)
	}
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return a, nil
	}
	currentAssigneeID := ""
	if sel.Assignee != nil {
		currentAssigneeID = sel.Assignee.AccountID
	}

	myAccountID := ""
	if a.currentUser != nil {
		myAccountID = a.currentUser.AccountID
	}

	var items []components.ModalItem

	// Put current user first
	if a.currentUser != nil {
		meLabel := a.currentUser.DisplayName + " (me)"
		meFound := false
		for _, u := range msg.users {
			if u.AccountID == myAccountID {
				items = append(items, components.ModalItem{ID: u.AccountID, Label: meLabel, Active: u.AccountID == currentAssigneeID})
				meFound = true
				break
			}
		}
		if !meFound {
			items = append(items, components.ModalItem{ID: a.currentUser.AccountID, Label: meLabel, Active: a.currentUser.AccountID == currentAssigneeID})
		}
	}

	// Then unassigned option
	items = append(items, components.ModalItem{ID: "", Label: "None", Active: currentAssigneeID == ""})

	// Then everyone else, skip current user since already added
	for _, u := range msg.users {
		if u.AccountID == myAccountID {
			continue
		}
		items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName, Active: u.AccountID == currentAssigneeID})
	}

	// onSelect callback set at fetch time (ActAssignee or editInfoField)
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
	if a.onSelect == nil {
		issueKey := sel.Key
		a.onSelect = func(item components.ModalItem) tea.Cmd {
			sprintID, _ := strconv.Atoi(item.ID)
			if sprintID == 0 {
				return updateIssueField(a.client, issueKey, "sprint", nil)
			}
			return moveToSprint(a.client, sprintID, issueKey)
		}
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

// handleIssueTypesLoaded shows the issue type picker modal or create form type picker
func (a *App) handleIssueTypesLoaded(msg issueTypesLoadedMsg) (tea.Model, tea.Cmd) {
	// if creating an issue, show type picker via the standard Modal
	if a.createCtx.intent {
		a.createCtx.intent = false
		items := make([]components.ModalItem, 0, len(msg.issueTypes))
		for _, t := range msg.issueTypes {
			if !t.Subtask {
				items = append(items, components.ModalItem{ID: t.ID, Label: t.Name})
			}
		}
		a.onSelect = func(item components.ModalItem) tea.Cmd {
			return func() tea.Msg {
				return components.CreateFormTypeSelectedMsg{TypeID: item.ID, TypeName: item.Label}
			}
		}
		a.modal.Show("Select issue type", items)
		return a, nil
	}
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

// handleCreateFormTypeSelected fetches create metadata for selected type
func (a *App) handleCreateFormTypeSelected(msg components.CreateFormTypeSelectedMsg) (tea.Model, tea.Cmd) {
	a.createCtx.issueTypeID = msg.TypeID
	a.createCtx.issueTypeName = msg.TypeName
	a.createForm.SetLoading(true)
	*a.logFlag = true
	return a, fetchCreateMeta(a.client, a.createCtx.projectKey, msg.TypeID)
}

// handleCreateMetaLoaded builds form fields from metadata
func (a *App) handleCreateMetaLoaded(msg createMetaLoadedMsg) (tea.Model, tea.Cmd) {
	fields := a.buildCreateFields(msg.fields)

	// prefill from active tab JQL
	if a.cfg.GUI.ShouldPrefillFromTab() {
		tab := a.issuesList.ActiveTab()
		if tab.JQL != "" {
			jql := resolveTabJQL(tab, a.projectKey, a.cfg.Jira.Email)
			prefill := ParseJQLPrefill(jql)
			ApplyPrefill(fields, prefill, a.currentUser, a.isCloud)
		}
	}

	// duplicate mode: prefill from source issue (overrides JQL prefill)
	if src := a.createCtx.duplicateFrom; src != nil {
		applyDuplicatePrefill(fields, src, a.isCloud)
	}

	a.createForm.ShowForm(fields, a.createCtx.issueTypeName, a.createCtx.projectKey)

	// prefetch users in background (not included in createmeta AllowedValues)
	var cmds []tea.Cmd
	if _, ok := a.usersCache[a.projectKey]; !ok {
		cmds = append(cmds, fetchUsers(a.client, a.projectKey, ""))
	}
	if len(cmds) > 0 {
		return a, tea.Batch(cmds...)
	}
	return a, nil
}

// applyDuplicatePrefill copies field values from a source issue to form fields
func applyDuplicatePrefill(fields []components.CreateFormField, src *jira.Issue, isCloud bool) {
	for i := range fields {
		switch fields[i].FieldID {
		case "summary":
			fields[i].DisplayValue = "Copy of " + src.Summary
			fields[i].Value = "Copy of " + src.Summary
		case "description":
			fields[i].DisplayValue = src.Description
			if src.DescriptionADF != nil {
				fields[i].Value = stripADFMedia(src.DescriptionADF)
			} else {
				fields[i].Value = src.Description
			}
		case fldPriority:
			if src.Priority != nil {
				fields[i].DisplayValue = src.Priority.Name
				fields[i].Value = map[string]string{"id": src.Priority.ID}
			}
		case fldAssignee:
			if src.Assignee != nil {
				fields[i].DisplayValue = src.Assignee.DisplayName
				key := fldName
				if isCloud {
					key = fldAccountID
				}
				fields[i].Value = map[string]string{key: src.Assignee.AccountID}
			}
		case fldLabels:
			if len(src.Labels) > 0 {
				fields[i].DisplayValue = strings.Join(src.Labels, ", ")
				fields[i].Value = src.Labels
			}
		case fldComponents:
			if len(src.Components) > 0 {
				comps := make([]map[string]string, 0, len(src.Components))
				names := make([]string, 0, len(src.Components))
				for _, c := range src.Components {
					comps = append(comps, map[string]string{"id": c.ID})
					names = append(names, c.Name)
				}
				fields[i].DisplayValue = strings.Join(names, ", ")
				fields[i].Value = comps
			}
		case fldSprint:
			if src.Sprint != nil {
				fields[i].DisplayValue = src.Sprint.Name
				fields[i].Value = map[string]string{"id": strconv.Itoa(src.Sprint.ID)}
			}
		default:
			// custom fields
			if strings.HasPrefix(fields[i].FieldID, "customfield_") {
				if val, ok := src.CustomFields[fields[i].FieldID]; ok {
					display := formatCustomVal(val)
					if display == "" {
						continue
					}
					fields[i].Value = val
					fields[i].DisplayValue = display
				}
			}
		}
	}
}

// stripADFMedia removes media nodes from ADF that reference source issue attachments
func stripADFMedia(adf any) any {
	doc, ok := adf.(map[string]any)
	if !ok {
		return adf
	}
	content, ok := doc["content"].([]any)
	if !ok {
		return adf
	}
	var filtered []any
	for _, node := range content {
		n, ok := node.(map[string]any)
		if !ok {
			filtered = append(filtered, node)
			continue
		}
		nodeType, _ := n["type"].(string)
		if nodeType == "mediaSingle" || nodeType == "mediaGroup" || nodeType == "media" {
			continue
		}
		filtered = append(filtered, node)
	}
	result := make(map[string]any, len(doc))
	maps.Copy(result, doc)
	result["content"] = filtered
	return result
}

// formatCustomVal converts a custom field value to display string
func formatCustomVal(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case map[string]any:
		if name, ok := val["displayName"].(string); ok {
			return name
		}
		if name, ok := val["value"].(string); ok {
			return name
		}
		if name, ok := val["name"].(string); ok {
			return name
		}
		return ""
	case []any:
		var parts []string
		for _, item := range val {
			parts = append(parts, formatCustomVal(item))
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

// buildCreateFields converts create metadata to form fields
// fields that are set automatically or not user-editable on creation
var skipCreateFields = map[string]bool{
	"project":    true,
	"issuetype":  true,
	"attachment": true,
	"issuelinks": true,
	"parent":     true,
}

// supported schema types for create form fields
var supportedSchemaTypes = map[string]bool{
	"string":   true,
	"array":    true,
	"priority": true,
	"user":     true,
	"option":   true,
	"number":   true,
	"date":     true,
	"datetime": true,
	"timetracking": true,
}

func (a *App) buildCreateFields(meta []jira.CreateMetaField) []components.CreateFormField {
	// ordered known fields shown first
	knownOrder := []string{"summary", "description", fldPriority, fldAssignee, fldLabels, fldComponents, fldSprint}
	metaMap := make(map[string]jira.CreateMetaField)
	for _, f := range meta {
		metaMap[f.FieldID] = f
	}

	added := make(map[string]bool)
	var fields []components.CreateFormField

	for _, fid := range knownOrder {
		mf, ok := metaMap[fid]
		if !ok {
			// always show summary and description even if not in metadata
			switch fid {
			case "summary":
				mf = jira.CreateMetaField{FieldID: "summary", Name: "Summary", Required: true, Schema: jira.CreateMetaSchema{Type: "string", System: "summary"}}
			case "description":
				mf = jira.CreateMetaField{FieldID: "description", Name: "Description", Required: false, Schema: jira.CreateMetaSchema{Type: "string", System: "description"}}
			default:
				continue
			}
		}
		fields = append(fields, a.metaToFormField(mf))
		added[fid] = true
	}

	// apply custom field name overrides from config
	cfgNames := make(map[string]string)
	for _, cf := range a.cfg.CustomFields {
		cfgNames[cf.ID] = cf.Name
	}

	// add remaining fields from API metadata (sorted by required first)
	var remaining []jira.CreateMetaField
	for _, mf := range meta {
		if added[mf.FieldID] || skipCreateFields[mf.FieldID] {
			continue
		}
		if !supportedSchemaTypes[mf.Schema.Type] {
			continue
		}
		remaining = append(remaining, mf)
	}
	// required fields first
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Required != remaining[j].Required {
			return remaining[i].Required
		}
		return false
	})
	for _, mf := range remaining {
		ff := a.metaToFormField(mf)
		if name, ok := cfgNames[mf.FieldID]; ok {
			ff.Name = name
		}
		fields = append(fields, ff)
	}

	return fields
}

const schemaArray = "array"

// metaToFormField converts one CreateMetaField to CreateFormField
func (a *App) metaToFormField(mf jira.CreateMetaField) components.CreateFormField {
	ft := components.CFFieldSingleText
	switch {
	case mf.Schema.System == "description":
		ft = components.CFFieldMultiText
	case mf.Schema.System == fldPriority || mf.Schema.System == "issuetype" || mf.Schema.System == fldSprint:
		ft = components.CFFieldSingleSelect
	case mf.Schema.System == fldAssignee || mf.Schema.System == "reporter":
		ft = components.CFFieldPerson
	case mf.Schema.System == fldLabels:
		ft = components.CFFieldMultiSelect
	case mf.Schema.System == fldComponents:
		ft = components.CFFieldMultiSelect
	case mf.Schema.Type == "option":
		// custom single-select (Size, Project, etc)
		ft = components.CFFieldSingleSelect
	case mf.Schema.Type == schemaArray && mf.Schema.Items == "option":
		// custom multi-select (Tags, Components custom, etc)
		ft = components.CFFieldMultiSelect
	case mf.Schema.Type == schemaArray && mf.Schema.Items == "user":
		// custom multi-user picker (Requestor, etc)
		ft = components.CFFieldMultiSelect
	case mf.Schema.Type == schemaArray && mf.Schema.Items == "string":
		// labels-like arrays
		ft = components.CFFieldMultiSelect
	case mf.Schema.Type == schemaArray:
		ft = components.CFFieldMultiSelect
	case mf.Schema.Type == "user":
		ft = components.CFFieldPerson
	case len(mf.AllowedValues) > 0:
		// has options, treat as select even if type is unknown
		ft = components.CFFieldSingleSelect
	}

	allowed := make([]components.ModalItem, 0, len(mf.AllowedValues))
	for _, v := range mf.AllowedValues {
		allowed = append(allowed, components.ModalItem{ID: v.ID, Label: v.Name})
	}

	ff := components.CreateFormField{
		Name:          mf.Name,
		FieldID:       mf.FieldID,
		Type:          ft,
		Required:      mf.Required,
		AllowedValues: allowed,
		SchemaItems:   mf.Schema.Items,
	}

	// empty optional fields show "None"
	if !mf.Required && ff.DisplayValue == "" && ft != components.CFFieldMultiText {
		ff.DisplayValue = "None"
	}

	return ff
}

// handleIssueCreated closes form and refreshes issue list
func (a *App) handleIssueCreated(msg issueCreatedMsg) (tea.Model, tea.Cmd) {
	a.createForm.Hide()
	a.createCtx = createCtx{}
	if msg.issue != nil {
		a.helpBar.SetStatusMsg("Created " + msg.issue.Key)
		if a.cfg.GUI.ShouldSelectCreatedIssue() {
			a.gitDetectedKey = msg.issue.Key
			// prepare detail view to accept the new issue when fetchIssueDetail returns
			a.issuesList.SetActiveKey(msg.issue.Key)
			a.detailView.SetIssue(nil)
		}
		return a, tea.Batch(a.fetchActiveTab(), fetchIssueDetail(a.client, msg.issue.Key))
	}
	return a, a.fetchActiveTab()
}

// handleIssueUpdated re-fetches issue data after an update.
// Only refreshes the issue detail, not the tab — avoids cursor jumping
// when the edited issue no longer matches the tab JQL (e.g. unassign on "Assigned" tab).
func (a *App) handleIssueUpdated(msg issueUpdatedMsg) (tea.Model, tea.Cmd) {
	return a, fetchIssueDetail(a.client, msg.issueKey)
}
