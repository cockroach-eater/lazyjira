package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

func formWithFields(fields []components.CreateFormField) components.CreateForm {
	form := components.NewCreateForm(nil)
	form.ShowForm(fields, "Story", testProject)
	return form
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "edit.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestHandleModalSelected(t *testing.T) {
	t.Parallel()

	t.Run("invokes callback and clears it", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		called := false
		app.onSelect = func(components.ModalItem) tea.Cmd {
			called = true
			return func() tea.Msg { return nil }
		}

		_, cmd := app.handleModalSelected(components.ModalSelectedMsg{Item: components.ModalItem{ID: "1"}})

		if !called {
			t.Error("onSelect callback was not invoked")
		}
		if app.onSelect != nil {
			t.Error("onSelect should be cleared after dispatch")
		}
		if cmd == nil {
			t.Error("expected non-nil cmd from callback")
		}
	})

	t.Run("nil callback is noop", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		_, cmd := app.handleModalSelected(components.ModalSelectedMsg{})
		if cmd != nil {
			t.Error("expected nil cmd with no callback")
		}
	})
}

func TestHandleChecklistConfirmed(t *testing.T) {
	t.Parallel()

	t.Run("invokes callback and clears it", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		called := false
		app.onChecklist = func(components.ChecklistConfirmedMsg) tea.Cmd {
			called = true
			return nil
		}

		_, _ = app.handleChecklistConfirmed(components.ChecklistConfirmedMsg{})

		if !called {
			t.Error("onChecklist callback was not invoked")
		}
		if app.onChecklist != nil {
			t.Error("onChecklist should be cleared after dispatch")
		}
	})

	t.Run("nil callback is noop", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		_, cmd := app.handleChecklistConfirmed(components.ChecklistConfirmedMsg{})
		if cmd != nil {
			t.Error("expected nil cmd with no callback")
		}
	})
}

func TestHandleModalCancelled_ClearsCallbacks(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.onSelect = func(components.ModalItem) tea.Cmd { return nil }
	app.onChecklist = func(components.ChecklistConfirmedMsg) tea.Cmd { return nil }
	app.createCtx = createCtx{projectKey: testProject}

	_, _ = app.handleModalCancelled()

	if app.onSelect != nil || app.onChecklist != nil {
		t.Error("callbacks should be cleared")
	}
	if app.createCtx.projectKey != "" {
		t.Error("createCtx should reset when create form is not visible")
	}
}

func TestHandleDiffConfirmed_CleansUpTempFile(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	path := writeTempFile(t, "body")
	app.editTempPath = path
	app.editContext = editCtx{kind: editNone}

	_, _ = app.handleDiffConfirmed(components.DiffConfirmedMsg{Content: "body"})

	if app.editTempPath != "" {
		t.Error("editTempPath should be cleared")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("temp file should be removed")
	}
}

func TestHandleDiffCancelled_CleansUpTempFile(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	path := writeTempFile(t, "body")
	app.editTempPath = path

	_, cmd := app.handleDiffCancelled()

	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if app.editTempPath != "" {
		t.Error("editTempPath should be cleared")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("temp file should be removed")
	}
}

func TestHandleInputConfirmed(t *testing.T) {
	t.Parallel()

	t.Run("create field sets form value", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: "summary"},
			{FieldID: "customfield_1", Name: "Points"},
		})
		app.editContext = editCtx{kind: editCreateField, fieldIndex: 1}

		_, _ = app.handleInputConfirmed(components.InputConfirmedMsg{Text: "5"})

		if got := app.createForm.FieldAt(1).DisplayValue; got != "5" {
			t.Errorf("DisplayValue = %q, want 5", got)
		}
		if app.editContext.kind != editNone {
			t.Error("editContext should reset")
		}
	})

	t.Run("summary update issues command", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
		app := newAppWithFake(t, fake)
		app.editContext = editCtx{kind: editSummary, issueKey: testKey}

		_, cmd := app.handleInputConfirmed(components.InputConfirmedMsg{Text: "new title"})

		if cmd == nil {
			t.Fatal("expected update command")
		}
		if _, ok := cmd().(issueUpdatedMsg); !ok {
			t.Error("expected issueUpdatedMsg from summary update")
		}
	})

	t.Run("field update issues command", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
		app := newAppWithFake(t, fake)
		app.editContext = editCtx{kind: editField, issueKey: testKey, fieldID: "customfield_1"}

		_, cmd := app.handleInputConfirmed(components.InputConfirmedMsg{Text: "x"})

		if cmd == nil {
			t.Fatal("expected update command")
		}
	})

	t.Run("parent field with invalid key surfaces error", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.editContext = editCtx{kind: editField, issueKey: testKey, fieldID: "parent"}

		_, cmd := app.handleInputConfirmed(components.InputConfirmedMsg{Text: "not a key"})

		if cmd == nil {
			t.Fatal("expected error command")
		}
		if _, ok := cmd().(errorMsg); !ok {
			t.Error("expected errorMsg for invalid parent key")
		}
	})

	t.Run("empty text is noop", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.editContext = editCtx{kind: editSummary, issueKey: testKey}

		_, cmd := app.handleInputConfirmed(components.InputConfirmedMsg{Text: ""})

		if cmd != nil {
			t.Error("empty summary text should not issue a command")
		}
	})
}

func TestHandleInputCancelled_ResetsContext(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.createForm = formWithFields([]components.CreateFormField{{FieldID: "summary"}})
	app.editContext = editCtx{kind: editCreateField, fieldIndex: 0}

	_, _ = app.handleInputCancelled()

	if app.editContext.kind != editNone {
		t.Error("editContext should reset")
	}
}

func TestHandleCreateFormEditText(t *testing.T) {
	t.Parallel()

	t.Run("opens input modal for field", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: "customfield_1", Name: "Points", DisplayValue: "3"},
		})

		_, _ = app.handleCreateFormEditText(components.CreateFormEditTextMsg{FieldIndex: 0})

		if !app.inputModal.IsVisible() {
			t.Error("input modal should be visible")
		}
		if app.editContext.kind != editCreateField {
			t.Error("editContext should be editCreateField")
		}
	})

	t.Run("missing field is noop", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields(nil)

		_, cmd := app.handleCreateFormEditText(components.CreateFormEditTextMsg{FieldIndex: 9})

		if cmd != nil || app.inputModal.IsVisible() {
			t.Error("out of range field index should be a noop")
		}
	})
}

func TestHandleCreateFormPicker(t *testing.T) {
	t.Parallel()

	t.Run("with items shows modal", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: "customfield_1", Name: "Team", Type: components.CFFieldSingleSelect},
		})

		_, _ = app.handleCreateFormPicker(components.CreateFormPickerMsg{
			FieldIndex: 0,
			Items:      []components.ModalItem{{ID: "1", Label: "A"}},
		})

		if !app.modal.IsVisible() {
			t.Error("selection modal should be visible")
		}
		if app.onSelect == nil {
			t.Error("onSelect should be set")
		}
	})

	t.Run("priority field with no items fetches priorities", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetPrioritiesFunc = func(context.Context) ([]jira.Priority, error) { return nil, nil }
		app := newAppWithFake(t, fake)
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: fldPriority, Name: "Priority", Type: components.CFFieldSingleSelect},
		})

		_, cmd := app.handleCreateFormPicker(components.CreateFormPickerMsg{FieldIndex: 0})

		if cmd == nil {
			t.Fatal("expected fetch priorities command")
		}
	})

	t.Run("person field with no cache fetches users", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetUsersFunc = func(context.Context, string) ([]jira.User, error) { return nil, nil }
		app := newAppWithFake(t, fake)
		app.usersCache = map[string][]jira.User{}
		app.projectKey = testProject
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: fldAssignee, Name: "Assignee", Type: components.CFFieldPerson},
		})

		_, cmd := app.handleCreateFormPicker(components.CreateFormPickerMsg{FieldIndex: 0})

		if cmd == nil {
			t.Fatal("expected fetch users command")
		}
		if app.onSelect == nil {
			t.Error("onSelect should be set for person field")
		}
	})

	t.Run("person field with cache shows picker", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.projectKey = testProject
		app.usersCache = map[string][]jira.User{testProject: {{AccountID: "u1", DisplayName: "Ann"}}}
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: fldAssignee, Name: "Assignee", Type: components.CFFieldPerson},
		})

		_, _ = app.handleCreateFormPicker(components.CreateFormPickerMsg{FieldIndex: 0})

		if !app.modal.IsVisible() {
			t.Error("user picker modal should be visible")
		}
	})
}

func TestHandleCreateFormChecklist(t *testing.T) {
	t.Parallel()

	t.Run("with items shows checklist", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: "customfield_1", Name: "Tags", Type: components.CFFieldMultiSelect},
		})

		_, _ = app.handleCreateFormChecklist(components.CreateFormChecklistMsg{
			FieldIndex: 0,
			Items:      []components.ModalItem{{ID: "1", Label: "A"}},
		})

		if !app.modal.IsVisible() {
			t.Error("checklist modal should be visible")
		}
		if app.onChecklist == nil {
			t.Error("onChecklist should be set")
		}
	})

	t.Run("labels field fetches labels", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetLabelsFunc = func(context.Context) ([]string, error) { return nil, nil }
		app := newAppWithFake(t, fake)
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: fldLabels, Name: "Labels", Type: components.CFFieldMultiSelect},
		})

		_, cmd := app.handleCreateFormChecklist(components.CreateFormChecklistMsg{FieldIndex: 0})

		if cmd == nil {
			t.Fatal("expected fetch labels command")
		}
		if app.onChecklist == nil {
			t.Error("onChecklist should be set for labels")
		}
	})

	t.Run("components field fetches components", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetComponentsFunc = func(context.Context, string) ([]jira.Component, error) { return nil, nil }
		app := newAppWithFake(t, fake)
		app.projectKey = testProject
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: fldComponents, Name: "Components", Type: components.CFFieldMultiSelect},
		})

		_, cmd := app.handleCreateFormChecklist(components.CreateFormChecklistMsg{FieldIndex: 0})

		if cmd == nil {
			t.Fatal("expected fetch components command")
		}
	})
}

func TestHandleCreateFormUserChecklist(t *testing.T) {
	t.Parallel()

	t.Run("cached users show checklist", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.projectKey = testProject
		app.usersCache = map[string][]jira.User{testProject: {{AccountID: "u1", DisplayName: "Ann"}}}
		field := &components.CreateFormField{FieldID: "customfield_9", Name: "Reviewers", SchemaItems: schemaUser}

		_, _ = app.handleCreateFormUserChecklist(field, 0)

		if !app.modal.IsVisible() {
			t.Error("user checklist should be visible")
		}
		if app.onChecklist == nil {
			t.Error("onChecklist should be set")
		}
	})

	t.Run("no cache fetches users", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetUsersFunc = func(context.Context, string) ([]jira.User, error) { return nil, nil }
		app := newAppWithFake(t, fake)
		app.usersCache = map[string][]jira.User{}
		app.projectKey = testProject
		field := &components.CreateFormField{FieldID: "customfield_9", Name: "Reviewers", SchemaItems: schemaUser}

		_, cmd := app.handleCreateFormUserChecklist(field, 0)

		if cmd == nil {
			t.Fatal("expected fetch users command")
		}
	})
}

func TestShowCreateUserPicker(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})

	_, _ = app.showCreateUserPicker([]jira.User{{AccountID: "u1", DisplayName: "Ann"}})

	if !app.modal.IsVisible() {
		t.Error("user picker modal should be visible")
	}
}

func TestHandleCreateFormSubmit(t *testing.T) {
	t.Parallel()
	fake := &jiratest.FakeClient{T: t}
	fake.CreateIssueFunc = func(context.Context, map[string]any) (*jira.Issue, error) {
		return &jira.Issue{Key: "PLAT-99"}, nil
	}
	app := newAppWithFake(t, fake)
	app.createCtx = createCtx{projectKey: testProject, issueTypeID: "10001"}

	fields := map[string]any{"summary": "hi"}
	_, cmd := app.handleCreateFormSubmit(components.CreateFormSubmitMsg{Fields: fields})

	if cmd == nil {
		t.Fatal("expected create command")
	}
	project, ok := fields["project"].(map[string]string)
	if !ok || project["key"] != testProject {
		t.Errorf("project field not injected: %v", fields["project"])
	}
	if _, ok := fields["issuetype"]; !ok {
		t.Error("issuetype field not injected")
	}
}

func TestHandleExpandBlock_ShowsReadOnlyModal(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.width = 80
	app.height = 24

	_, _ = app.handleExpandBlock(views.ExpandBlockMsg{Title: "Body", Lines: []string{"a", "b"}})

	if !app.modal.IsVisible() {
		t.Error("expand modal should be visible")
	}
}

func TestHandleEditorFinished(t *testing.T) {
	t.Parallel()

	t.Run("create description updates form field", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{
			{FieldID: "summary"},
			{FieldID: "description"},
		})
		app.editContext = editCtx{kind: editCreateDesc, fieldIndex: 1}
		path := writeTempFile(t, "new body")

		_, _ = app.handleEditorFinished(editorFinishedMsg{original: "old", tempPath: path})

		if got := app.createForm.FieldAt(1).Value; got != "new body" {
			t.Errorf("field value = %v, want new body", got)
		}
		if app.editContext.kind != editNone {
			t.Error("editContext should reset")
		}
	})

	t.Run("changed content opens diff view", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.editContext = editCtx{kind: editField, issueKey: testKey, fieldID: "customfield_1"}
		path := writeTempFile(t, "changed")

		_, _ = app.handleEditorFinished(editorFinishedMsg{original: "orig", tempPath: path})

		if !app.diffView.IsVisible() {
			t.Error("diff view should be visible after a change")
		}
		if app.editTempPath != path {
			t.Error("editTempPath should be retained for diff confirmation")
		}
	})

	t.Run("unchanged content cleans up", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.editContext = editCtx{kind: editField, issueKey: testKey}
		path := writeTempFile(t, "same")

		_, _ = app.handleEditorFinished(editorFinishedMsg{original: "same", tempPath: path})

		if app.diffView.IsVisible() {
			t.Error("diff view should stay hidden when nothing changed")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("temp file should be removed when unchanged")
		}
	})

	t.Run("editor error surfaces in status", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.editContext = editCtx{kind: editField, issueKey: testKey}

		_, _ = app.handleEditorFinished(editorFinishedMsg{err: errNoEditor})

		if app.diffView.IsVisible() {
			t.Error("diff view should not show on editor error")
		}
		if app.statusPanel.ErrorMessage() == "" {
			t.Error("status panel should show error message after editor failure")
		}
	})
}
