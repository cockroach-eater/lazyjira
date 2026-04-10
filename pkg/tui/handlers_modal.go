package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/git"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// handleModalSelected dispatches modal selection via the onSelect callback
func (a *App) handleModalSelected(msg components.ModalSelectedMsg) (tea.Model, tea.Cmd) {
	a.createForm.Resume()
	fn := a.onSelect
	a.onSelect = nil
	if fn != nil {
		return a, fn(msg.Item)
	}
	return a, nil
}

// handleChecklistConfirmed dispatches checklist selection result
func (a *App) handleChecklistConfirmed(msg components.ChecklistConfirmedMsg) (tea.Model, tea.Cmd) {
	a.createForm.Resume()
	if fn := a.onChecklist; fn != nil {
		a.onChecklist = nil
		return a, fn(msg.Selected)
	}
	return a, nil
}

// handleModalCancelled clears modal callbacks
func (a *App) handleModalCancelled() (tea.Model, tea.Cmd) {
	a.createForm.Resume()
	if !a.createForm.IsVisible() {
		a.createCtx = createCtx{}
	}
	a.onSelect = nil
	a.onChecklist = nil
	return a, nil
}

// handleEditorFinished processes $EDITOR exit and shows diff view
func (a *App) handleEditorFinished(msg editorFinishedMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tea.EnableMouseCellMotion}
	content, changed, err := readAndCheckEditor(msg)

	// create form description: skip diff view, update field directly
	if a.editContext.kind == editCreateDesc {
		idx := a.editContext.fieldIndex
		a.editContext = editCtx{}
		cleanupEditor(msg.tempPath)
		a.editTempPath = ""
		if err != nil {
			a.statusPanel.SetError(err.Error())
			return a, tea.Batch(cmds...)
		}
		content = strings.TrimSpace(content)
		if changed && content != "" {
			var val any = content
			if a.isCloud {
				val = views.MarkdownToADF(content)
			}
			a.createForm.SetFieldValue(idx, val, content)
		}
		return a, tea.Batch(cmds...)
	}

	if err != nil {
		cleanupEditor(msg.tempPath)
		a.editTempPath = ""
		a.statusPanel.SetError(err.Error())
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
	switch ctx.kind { //nolint:exhaustive
	case editCreateField:
		a.createForm.Resume()
		if msg.Text != "" {
			a.createForm.SetFieldValue(ctx.fieldIndex, msg.Text, msg.Text)
		}
		return a, nil
	case editSummary:
		if msg.Text != "" {
			a.optimisticFieldUpdate(ctx.issueKey, "summary", msg.Text)
			return a, updateIssueField(a.client, ctx.issueKey, "summary", msg.Text)
		}
	case editField:
		if msg.Text != "" {
			a.optimisticFieldUpdate(ctx.issueKey, ctx.fieldID, msg.Text)
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

// handleInputCancelled clears edit context
func (a *App) handleInputCancelled() (tea.Model, tea.Cmd) {
	if a.editContext.kind == editCreateField {
		a.createForm.Resume()
	}
	a.editContext = editCtx{}
	return a, nil
}

const createUsersSentinel = "__create__"

// handleCreateFormEditText opens InputModal for a create form text field
func (a *App) handleCreateFormEditText(msg components.CreateFormEditTextMsg) (tea.Model, tea.Cmd) {
	field := a.createForm.FieldAt(msg.FieldIndex)
	if field == nil {
		return a, nil
	}
	a.createForm.Pause()
	a.inputModal.Show("Edit "+field.Name, field.DisplayValue)
	a.editContext = editCtx{kind: editCreateField, fieldIndex: msg.FieldIndex}
	return a, nil
}

// handleCreateFormEditExternal opens $EDITOR for description
func (a *App) handleCreateFormEditExternal(msg components.CreateFormEditExternalMsg) (tea.Model, tea.Cmd) {
	field := a.createForm.FieldAt(msg.FieldIndex)
	if field == nil {
		return a, nil
	}
	a.editContext = editCtx{kind: editCreateDesc, fieldIndex: msg.FieldIndex}
	content := ""
	if field.Value != nil {
		if _, isStr := field.Value.(string); !isStr {
			content = views.ADFToMarkdown(field.Value)
		}
	}
	if content == "" && field.DisplayValue != "" {
		content = field.DisplayValue
	}
	return a, launchEditor(content, ".md")
}

// handleCreateFormPicker opens selection modal for a create form field
func (a *App) handleCreateFormPicker(msg components.CreateFormPickerMsg) (tea.Model, tea.Cmd) {
	field := a.createForm.FieldAt(msg.FieldIndex)
	if field == nil {
		return a, nil
	}
	idx := msg.FieldIndex

	a.createForm.Pause()

	// for assignee/person fields, fetch users if no items provided
	if field.Type == components.CFFieldPerson && len(msg.Items) == 0 {
		a.onSelect = func(item components.ModalItem) tea.Cmd {
			display := item.Label
			var val any
			if item.ID == "" {
				val = nil
			} else {
				key := fldName
				if a.isCloud {
					key = fldAccountID
				}
				val = map[string]string{key: item.ID}
			}
			a.createForm.SetFieldValue(idx, val, display)
			return nil
		}
		if cached, ok := a.usersCache[a.projectKey]; ok {
			return a.showCreateUserPicker(cached)
		}
		return a, fetchUsers(a.client, a.projectKey, createUsersSentinel)
	}

	items := msg.Items
	if len(items) == 0 {
		// fetch based on field type
		switch field.FieldID {
		case fldPriority:
			a.onSelect = func(item components.ModalItem) tea.Cmd {
				a.createForm.SetFieldValue(idx, map[string]string{"id": item.ID}, item.Label)
				return nil
			}
			return a, fetchPriorities(a.client)
		case fldSprint:
			if a.boardID != 0 {
				a.onSelect = func(item components.ModalItem) tea.Cmd {
					sprintID, _ := strconv.Atoi(item.ID)
					a.createForm.SetFieldValue(idx, sprintID, item.Label)
					return nil
				}
				return a, fetchSprints(a.client, a.boardID)
			}
			a.createForm.Resume()
			return a, nil
		default:
			// no items and no known fetch, resume form
			a.createForm.Resume()
			return a, nil
		}
	}

	a.onSelect = func(item components.ModalItem) tea.Cmd {
		a.createForm.SetFieldValue(idx, map[string]string{"id": item.ID}, item.Label)
		return nil
	}
	a.modal.Show("Select "+field.Name, items)
	return a, nil
}

// showCreateUserPicker shows user picker for create form assignee
func (a *App) showCreateUserPicker(users []jira.User) (tea.Model, tea.Cmd) {
	a.modal.Show("Select Assignee", a.buildUserItems(users))
	return a, nil
}

// handleCreateFormChecklist opens checklist modal for multi-select
func (a *App) handleCreateFormChecklist(msg components.CreateFormChecklistMsg) (tea.Model, tea.Cmd) {
	field := a.createForm.FieldAt(msg.FieldIndex)
	if field == nil {
		return a, nil
	}
	a.createForm.Pause()
	idx := msg.FieldIndex

	items := msg.Items
	if len(items) == 0 {
		switch field.FieldID {
		case fldLabels:
			a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
				labels := make([]string, 0, len(selected))
				for _, item := range selected {
					labels = append(labels, item.ID)
				}
				a.createForm.SetFieldValue(idx, labels, strings.Join(labels, ", "))
				return nil
			}
			return a, fetchLabels(a.client)
		case fldComponents:
			a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
				comps := make([]map[string]string, 0, len(selected))
				names := make([]string, 0, len(selected))
				for _, item := range selected {
					comps = append(comps, map[string]string{"id": item.ID})
					names = append(names, item.Label)
				}
				a.createForm.SetFieldValue(idx, comps, strings.Join(names, ", "))
				return nil
			}
			return a, fetchComponents(a.client, a.projectKey)
		default:
			// multi-user custom field (Requestor, etc)
			if field.SchemaItems == schemaUser {
				return a.handleCreateFormUserChecklist(field, idx)
			}
			// no items and no known fetch, resume form
			a.createForm.Resume()
			return a, nil
		}
	}

	a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
		names := make([]string, 0, len(selected))
		ids := make([]map[string]string, 0, len(selected))
		for _, item := range selected {
			names = append(names, item.Label)
			ids = append(ids, map[string]string{"id": item.ID})
		}
		a.createForm.SetFieldValue(idx, ids, strings.Join(names, ", "))
		return nil
	}
	a.modal.ShowChecklist("Select "+field.Name, items, nil)
	return a, nil
}

// buildUserItems builds a ModalItem list: me (if available), None, then everyone else
func (a *App) buildUserItems(users []jira.User) []components.ModalItem {
	myID := ""
	if a.currentUser != nil {
		myID = a.currentUser.AccountID
	}
	items := make([]components.ModalItem, 0, len(users)+2)
	if a.currentUser != nil {
		items = append(items, components.ModalItem{ID: myID, Label: a.currentUser.DisplayName + " (me)"})
	}
	items = append(items, components.ModalItem{ID: "", Label: "None"})
	for _, u := range users {
		if u.AccountID == myID {
			continue
		}
		items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName})
	}
	return items
}

// handleCreateFormUserChecklist shows a user checklist for multi-user custom fields
func (a *App) handleCreateFormUserChecklist(field *components.CreateFormField, idx int) (tea.Model, tea.Cmd) {
	key := fldName
	if a.isCloud {
		key = fldAccountID
	}
	a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
		users := make([]map[string]string, 0, len(selected))
		names := make([]string, 0, len(selected))
		for _, item := range selected {
			users = append(users, map[string]string{key: item.ID})
			names = append(names, item.Label)
		}
		a.createForm.SetFieldValue(idx, users, strings.Join(names, ", "))
		return nil
	}
	if cached, ok := a.usersCache[a.projectKey]; ok {
		a.modal.ShowChecklist("Select "+field.Name, a.buildUserItems(cached), nil)
		return a, nil
	}
	return a, fetchUsers(a.client, a.projectKey, createUsersSentinel)
}

// handleCreateFormSubmit sends create issue request
func (a *App) handleCreateFormSubmit(msg components.CreateFormSubmitMsg) (tea.Model, tea.Cmd) {
	msg.Fields["project"] = map[string]string{"key": a.createCtx.projectKey}
	msg.Fields["issuetype"] = map[string]string{"id": a.createCtx.issueTypeID}
	a.createForm.SetLoading(true)
	*a.logFlag = true
	return a, createIssue(a.client, msg.Fields)
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
