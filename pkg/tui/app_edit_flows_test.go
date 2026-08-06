package tui

import (
	"context"
	"errors"
	"maps"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

type failingConverter struct{}

func (failingConverter) ToMarkdown(any) (string, any, error) {
	return "", nil, errors.New("to markdown failed")
}

func (failingConverter) FromMarkdown(string, any) (any, error) {
	return nil, errors.New("from markdown failed")
}

func editFlowApp(t *testing.T, fake *jiratest.FakeClient) *App {
	t.Helper()
	app := newAppWithFake(t, fake)
	app.keymap = DefaultKeymap()
	app.width = 120
	app.height = 40
	app.converter = BuiltinConverter{}
	return app
}

func selectInfoField(t *testing.T, app *App, issue *jira.Issue, fieldCfg config.FieldConfig) *jira.Issue {
	t.Helper()
	app.cfg.Fields = []config.FieldConfig{fieldCfg}
	app.infoPanel.SetFields(app.cfg.Fields)
	app.infoPanel.SetIssue(issue)
	app.infoPanel.Cursor = 0
	if app.infoPanel.SelectedInfoField() == nil {
		t.Fatalf("no info field selected for config %+v", fieldCfg)
	}
	return issue
}

func TestEditInfoField_Dispatch(t *testing.T) {
	t.Parallel()

	storyIssue := func() *jira.Issue {
		return &jira.Issue{
			Key:        testKey,
			IssueType:  &jira.IssueType{ID: "10001", Name: "Story"},
			Labels:     []string{"backend"},
			Components: []jira.Component{{ID: "1", Name: "API"}},
		}
	}

	cases := []struct {
		name     string
		fieldCfg config.FieldConfig
		setup    func(app *App, fake *jiratest.FakeClient)
		wantCmd  bool
		assert   func(t *testing.T, app *App)
	}{
		{
			name:     "issuetype with project id fetches types",
			fieldCfg: config.FieldConfig{ID: "issuetype"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectID = "10000"
				fake.GetIssueTypesFunc = func(context.Context, string) ([]jira.IssueType, error) { return nil, nil }
			},
			wantCmd: true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.onSelect == nil {
					t.Error("onSelect callback should be installed")
				}
			},
		},
		{
			name:     "issuetype without project id is noop",
			fieldCfg: config.FieldConfig{ID: "issuetype"},
		},
		{
			name:     "sprint with board fetches sprints",
			fieldCfg: config.FieldConfig{ID: "sprint"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.boardID = 7
				fake.GetSprintsFunc = func(context.Context, int) ([]jira.Sprint, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:     "sprint without board surfaces error",
			fieldCfg: config.FieldConfig{ID: "sprint"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.statusPanel.ErrorMessage() == "" {
					t.Error("missing board should surface an error")
				}
			},
		},
		{
			name:     "custom single select fetches field options",
			fieldCfg: config.FieldConfig{ID: "customfield_1", Name: "Team", Type: "select"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				fake.GetCreateMetaFunc = func(context.Context, string, string) ([]jira.CreateMetaField, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:     "person field with cached users shows modal",
			fieldCfg: config.FieldConfig{ID: "assignee"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				app.usersCache[testProject] = []jira.User{{AccountID: "u1", DisplayName: "Ann"}}
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
			},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("assignee modal should be visible")
				}
			},
		},
		{
			name:     "person field without cache fetches users",
			fieldCfg: config.FieldConfig{ID: "assignee"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				fake.GetUsersFunc = func(context.Context, string) ([]jira.User, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:     "labels fetches labels and installs checklist callback",
			fieldCfg: config.FieldConfig{ID: "labels"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				fake.GetLabelsFunc = func(context.Context) ([]string, error) { return nil, nil }
			},
			wantCmd: true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.onChecklist == nil {
					t.Error("onChecklist callback should be installed")
				}
			},
		},
		{
			name:     "components fetches components and installs checklist callback",
			fieldCfg: config.FieldConfig{ID: "components"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				fake.GetComponentsFunc = func(context.Context, string) ([]jira.Component, error) { return nil, nil }
			},
			wantCmd: true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.onChecklist == nil {
					t.Error("onChecklist callback should be installed")
				}
			},
		},
		{
			name:     "custom multiselect fetches field options",
			fieldCfg: config.FieldConfig{ID: "customfield_1", Name: "Tags", Type: "multiselect"},
			setup: func(app *App, fake *jiratest.FakeClient) {
				fake.GetCreateMetaFunc = func(context.Context, string, string) ([]jira.CreateMetaField, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:     "custom single text fetches field options",
			fieldCfg: config.FieldConfig{ID: "customfield_1", Name: "Notes", Type: "text"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.inputModal.IsVisible() {
					t.Error("input modal should be visible for text custom field")
				}
			},
		},
		{
			name:     "multi text launches editor",
			fieldCfg: config.FieldConfig{ID: "customfield_1", Name: "Spec", Type: "textarea"},
			wantCmd:  true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.editContext.kind != editFieldText {
					t.Errorf("editContext kind = %v, want editFieldText", app.editContext.kind)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &jiratest.FakeClient{T: t}
			app := editFlowApp(t, fake)
			issue := selectInfoField(t, app, storyIssue(), tc.fieldCfg)
			if tc.setup != nil {
				tc.setup(app, fake)
			}

			_, cmd := app.editInfoField(issue)

			if tc.wantCmd && cmd == nil {
				t.Error("expected non-nil cmd")
			}
			if tc.assert != nil {
				tc.assert(t, app)
			}
		})
	}
}

func TestEditInfoField_LabelsChecklistCallbackUpdatesIssue(t *testing.T) {
	t.Parallel()
	fake := &jiratest.FakeClient{T: t}
	fake.GetLabelsFunc = func(context.Context) ([]string, error) { return nil, nil }
	fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
	app := editFlowApp(t, fake)
	issue := selectInfoField(t, app, &jira.Issue{Key: testKey, IssueType: &jira.IssueType{ID: "10001"}, Labels: []string{"old"}}, config.FieldConfig{ID: "labels"})
	app.issueCache[testKey] = issue

	_, _ = app.editInfoField(issue)
	cmd := app.onChecklist(components.ChecklistConfirmedMsg{Selected: []components.ModalItem{{ID: "backend"}, {ID: "infra"}}})
	cmd()

	if len(fake.UpdateIssueCalls) != 1 {
		t.Fatalf("UpdateIssue calls = %d, want 1", len(fake.UpdateIssueCalls))
	}
	want := map[string]any{"labels": []string{"backend", "infra"}}
	got := fake.UpdateIssueCalls[0].Fields
	labels, ok := got["labels"].([]string)
	if !ok || len(labels) != 2 || labels[0] != "backend" {
		t.Errorf("UpdateIssue fields = %v, want %v", got, want)
	}
}

func TestEditInfoField_ComponentsChecklistCallbackUpdatesIssue(t *testing.T) {
	t.Parallel()
	fake := &jiratest.FakeClient{T: t}
	fake.GetComponentsFunc = func(context.Context, string) ([]jira.Component, error) { return nil, nil }
	fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
	app := editFlowApp(t, fake)
	app.projectKey = testProject
	issue := selectInfoField(t, app, &jira.Issue{Key: testKey, IssueType: &jira.IssueType{ID: "10001"}, Components: []jira.Component{{ID: "9", Name: "Old"}}}, config.FieldConfig{ID: "components"})

	_, _ = app.editInfoField(issue)
	cmd := app.onChecklist(components.ChecklistConfirmedMsg{Selected: []components.ModalItem{{ID: "10"}}})
	cmd()

	if len(fake.UpdateIssueCalls) != 1 {
		t.Fatalf("UpdateIssue calls = %d, want 1", len(fake.UpdateIssueCalls))
	}
	comps, ok := fake.UpdateIssueCalls[0].Fields["components"].([]map[string]string)
	if !ok || len(comps) != 1 || comps[0]["id"] != "10" {
		t.Errorf("UpdateIssue fields = %v", fake.UpdateIssueCalls[0].Fields)
	}
}

func TestOptimisticFieldUpdate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fieldID string
		value   any
		assert  func(t *testing.T, cached *jira.Issue)
	}{
		{
			name:    "summary string",
			fieldID: "summary",
			value:   "new title",
			assert: func(t *testing.T, cached *jira.Issue) {
				t.Helper()
				if cached.Summary != "new title" {
					t.Errorf("Summary = %q", cached.Summary)
				}
			},
		},
		{
			name:    "description string",
			fieldID: fldDescription,
			value:   "new body",
			assert: func(t *testing.T, cached *jira.Issue) {
				t.Helper()
				if cached.Description != "new body" {
					t.Errorf("Description = %q", cached.Description)
				}
			},
		},
		{
			name:    "builtin field via registry",
			fieldID: fldPriority,
			value:   &jira.Priority{ID: "1", Name: "High"},
			assert: func(t *testing.T, cached *jira.Issue) {
				t.Helper()
				if cached.Priority == nil || cached.Priority.Name != "High" {
					t.Errorf("Priority = %v", cached.Priority)
				}
			},
		},
		{
			name:    "custom field creates map",
			fieldID: "customfield_1",
			value:   "5",
			assert: func(t *testing.T, cached *jira.Issue) {
				t.Helper()
				if !maps.Equal(map[string]any{"customfield_1": "5"}, cached.CustomFields) {
					t.Errorf("CustomFields = %v", cached.CustomFields)
				}
			},
		},
		{
			name:    "unknown field leaves issue untouched",
			fieldID: "mystery",
			value:   "x",
			assert: func(t *testing.T, cached *jira.Issue) {
				t.Helper()
				if cached.CustomFields != nil {
					t.Errorf("CustomFields = %v, want nil", cached.CustomFields)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newAppWithFake(t, &jiratest.FakeClient{T: t})
			app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
			app.issueCache[testKey] = &jira.Issue{Key: testKey}

			app.optimisticFieldUpdate(testKey, tc.fieldID, tc.value)

			tc.assert(t, app.issueCache[testKey])
		})
	}
}

func TestOptimisticFieldUpdate_NoCacheEntryIsNoop(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})

	app.optimisticFieldUpdate(testKey, "summary", "x")

	if _, ok := app.issueCache[testKey]; ok {
		t.Error("update without a cache entry must not create one")
	}
}

func TestApplyEdit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		kind     editKind
		isCloud  bool
		assert   func(t *testing.T, fake *jiratest.FakeClient)
		wantsCmd bool
	}{
		{
			name:     "description on server sends markdown",
			kind:     editDesc,
			wantsCmd: true,
			assert: func(t *testing.T, fake *jiratest.FakeClient) {
				t.Helper()
				if len(fake.UpdateIssueCalls) != 1 {
					t.Fatalf("UpdateIssue calls = %d", len(fake.UpdateIssueCalls))
				}
				if body, ok := fake.UpdateIssueCalls[0].Fields[fldDescription].(string); !ok || body != "hello" {
					t.Errorf("description body = %v, want plain string", fake.UpdateIssueCalls[0].Fields[fldDescription])
				}
			},
		},
		{
			name:     "description on cloud converts to ADF",
			kind:     editDesc,
			isCloud:  true,
			wantsCmd: true,
			assert: func(t *testing.T, fake *jiratest.FakeClient) {
				t.Helper()
				if len(fake.UpdateIssueCalls) != 1 {
					t.Fatalf("UpdateIssue calls = %d", len(fake.UpdateIssueCalls))
				}
				if _, isString := fake.UpdateIssueCalls[0].Fields[fldDescription].(string); isString {
					t.Error("cloud description should be ADF, not a string")
				}
			},
		},
		{
			name:     "new comment posts comment",
			kind:     editCommentNew,
			wantsCmd: true,
			assert: func(t *testing.T, fake *jiratest.FakeClient) {
				t.Helper()
				if len(fake.AddCommentCalls) != 1 || fake.AddCommentCalls[0].Key != testKey {
					t.Errorf("AddComment calls = %+v", fake.AddCommentCalls)
				}
			},
		},
		{
			name:     "comment edit updates comment",
			kind:     editCommentMod,
			wantsCmd: true,
			assert: func(t *testing.T, fake *jiratest.FakeClient) {
				t.Helper()
				if len(fake.UpdateCommentCalls) != 1 || fake.UpdateCommentCalls[0].CommentID != "9" {
					t.Errorf("UpdateComment calls = %+v", fake.UpdateCommentCalls)
				}
			},
		},
		{
			name:     "field text always sends markdown",
			kind:     editFieldText,
			isCloud:  true,
			wantsCmd: true,
			assert: func(t *testing.T, fake *jiratest.FakeClient) {
				t.Helper()
				if len(fake.UpdateIssueCalls) != 1 {
					t.Fatalf("UpdateIssue calls = %d", len(fake.UpdateIssueCalls))
				}
				if body, ok := fake.UpdateIssueCalls[0].Fields["customfield_1"].(string); !ok || body != "hello" {
					t.Errorf("field body = %v, want plain string", fake.UpdateIssueCalls[0].Fields["customfield_1"])
				}
			},
		},
		{
			name: "unknown kind is noop",
			kind: editNone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &jiratest.FakeClient{T: t}
			fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
			fake.AddCommentFunc = func(context.Context, string, any) (*jira.Comment, error) { return nil, nil }
			fake.UpdateCommentFunc = func(context.Context, string, string, any) error { return nil }
			app := editFlowApp(t, fake)
			app.isCloud = tc.isCloud
			app.editContext = editCtx{kind: tc.kind, issueKey: testKey, commentID: "9", fieldID: "customfield_1"}

			cmd := app.applyEdit("hello")

			if tc.wantsCmd != (cmd != nil) {
				t.Fatalf("cmd != nil is %v, want %v", cmd != nil, tc.wantsCmd)
			}
			if app.editContext.kind != editNone {
				t.Error("editContext should reset")
			}
			if cmd != nil {
				cmd()
			}
			if tc.assert != nil {
				tc.assert(t, fake)
			}
		})
	}
}

func TestApplyEdit_ConverterErrorSurfacesInStatus(t *testing.T) {
	t.Parallel()
	app := editFlowApp(t, &jiratest.FakeClient{T: t})
	app.isCloud = true
	app.converter = failingConverter{}
	app.editContext = editCtx{kind: editDesc, issueKey: testKey}

	cmd := app.applyEdit("hello")

	if cmd != nil {
		t.Error("conversion failure must not dispatch an update")
	}
	if app.statusPanel.ErrorMessage() == "" {
		t.Error("conversion failure should surface in status panel")
	}
}

func TestMakePersonSelectCallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		isCloud bool
		item    components.ModalItem
		want    any
	}{
		{
			name: "empty selection clears the field",
			item: components.ModalItem{ID: "", Label: "None"},
			want: nil,
		},
		{
			name:    "cloud uses accountId",
			isCloud: true,
			item:    components.ModalItem{ID: "u1", Label: "Ann"},
			want:    map[string]string{fldAccountID: "u1"},
		},
		{
			name: "server uses name",
			item: components.ModalItem{ID: "ann", Label: "Ann"},
			want: map[string]string{fldName: "ann"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &jiratest.FakeClient{T: t}
			fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
			app := editFlowApp(t, fake)
			app.isCloud = tc.isCloud
			app.issueCache[testKey] = &jira.Issue{Key: testKey, Assignee: &jira.User{DisplayName: "Old"}}
			app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})

			callback := app.makePersonSelectCallback(testKey, fldAssignee)
			cmd := callback(tc.item)
			cmd()

			if len(fake.UpdateIssueCalls) != 1 {
				t.Fatalf("UpdateIssue calls = %d, want 1", len(fake.UpdateIssueCalls))
			}
			got := fake.UpdateIssueCalls[0].Fields[fldAssignee]
			if tc.want == nil {
				if got != nil {
					t.Errorf("assignee value = %v, want nil", got)
				}
				if app.issueCache[testKey].Assignee != nil {
					t.Error("optimistic update should clear the assignee")
				}
				return
			}
			gotMap, ok := got.(map[string]string)
			if !ok || !maps.Equal(gotMap, tc.want.(map[string]string)) {
				t.Errorf("assignee value = %v, want %v", got, tc.want)
			}
			if app.issueCache[testKey].Assignee == nil || app.issueCache[testKey].Assignee.DisplayName != "Ann" {
				t.Errorf("optimistic assignee = %v", app.issueCache[testKey].Assignee)
			}
		})
	}
}

func TestHandleCustomFieldOptions_MoreBranches(t *testing.T) {
	t.Parallel()

	t.Run("caches all fields from response", func(t *testing.T) {
		t.Parallel()
		app := editFlowApp(t, &jiratest.FakeClient{T: t})
		msg := customFieldOptionsMsg{
			issueKey:    testKey,
			fieldID:     "customfield_1",
			fieldName:   "Team",
			options:     []jira.CreateMetaValue{{ID: "1", Name: "Core"}},
			allFields:   []jira.CreateMetaField{{FieldID: "customfield_1"}},
			issueTypeID: "10001",
			projectKey:  testProject,
		}

		_, _ = app.handleCustomFieldOptions(msg)

		if _, ok := app.createMetaCache[testProject+":10001"]; !ok {
			t.Error("create meta should be cached")
		}
	})

	t.Run("field not found with editor preference launches editor", func(t *testing.T) {
		t.Parallel()
		app := editFlowApp(t, &jiratest.FakeClient{T: t})

		_, cmd := app.handleCustomFieldOptions(customFieldOptionsMsg{
			issueKey: testKey, fieldID: "customfield_1", fieldNotFound: true, useEditor: true,
		})

		if cmd == nil {
			t.Fatal("expected editor launch cmd")
		}
		if app.editContext.kind != editFieldText {
			t.Errorf("editContext kind = %v, want editFieldText", app.editContext.kind)
		}
	})

	t.Run("person schema with cached users shows modal", func(t *testing.T) {
		t.Parallel()
		app := editFlowApp(t, &jiratest.FakeClient{T: t})
		app.projectKey = testProject
		app.usersCache[testProject] = []jira.User{{AccountID: "u1", DisplayName: "Ann"}}
		app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})

		_, _ = app.handleCustomFieldOptions(customFieldOptionsMsg{
			issueKey: testKey, fieldID: "customfield_1", schemaType: schemaUser,
		})

		if !app.modal.IsVisible() {
			t.Error("user picker modal should be visible")
		}
	})

	t.Run("no options with editor preference launches editor", func(t *testing.T) {
		t.Parallel()
		app := editFlowApp(t, &jiratest.FakeClient{T: t})

		_, cmd := app.handleCustomFieldOptions(customFieldOptionsMsg{
			issueKey: testKey, fieldID: "customfield_1", useEditor: true,
		})

		if cmd == nil {
			t.Fatal("expected editor launch cmd")
		}
	})

	t.Run("multiselect options show preselected checklist", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.UpdateIssueFunc = func(context.Context, string, map[string]any) error { return nil }
		app := editFlowApp(t, fake)
		app.issueCache[testKey] = &jira.Issue{
			Key: testKey,
			CustomFields: map[string]any{
				"customfield_1": []any{map[string]any{"id": "1"}},
			},
		}

		_, _ = app.handleCustomFieldOptions(customFieldOptionsMsg{
			issueKey:  testKey,
			fieldID:   "customfield_1",
			fieldName: "Tags",
			fieldType: views.FieldMultiSelect,
			options:   []jira.CreateMetaValue{{ID: "1", Name: "One"}, {ID: "2", Name: "Two"}},
		})

		if !app.modal.IsVisible() || !app.modal.IsChecklist() {
			t.Fatal("checklist modal should be visible")
		}
		cmd := app.onChecklist(components.ChecklistConfirmedMsg{Selected: []components.ModalItem{{ID: "2"}}})
		cmd()
		vals, ok := fake.UpdateIssueCalls[0].Fields["customfield_1"].([]map[string]string)
		if !ok || len(vals) != 1 || vals[0]["id"] != "2" {
			t.Errorf("UpdateIssue fields = %v", fake.UpdateIssueCalls[0].Fields)
		}
	})
}

func TestFetchCustomFieldOptionsForEdit_EditorBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		fieldCfg config.FieldConfig
		wantKind editKind
	}{
		{
			name:     "textarea type launches editor",
			fieldCfg: config.FieldConfig{ID: "customfield_1", Name: "Spec", Type: "textarea"},
			wantKind: editFieldText,
		},
		{
			name:     "multiline text launches editor",
			fieldCfg: config.FieldConfig{ID: "customfield_1", Name: "Spec", Type: "text", Multiline: true},
			wantKind: editFieldText,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := editFlowApp(t, &jiratest.FakeClient{T: t})
			app.cfg.Fields = []config.FieldConfig{tc.fieldCfg}
			issue := &jira.Issue{Key: testKey, IssueType: &jira.IssueType{ID: "10001"}}
			field := &views.InfoField{Name: tc.fieldCfg.Name, FieldID: tc.fieldCfg.ID, Type: views.FieldMultiText}

			_, cmd := app.fetchCustomFieldOptionsForEdit(issue, field)

			if cmd == nil {
				t.Fatal("expected editor launch cmd")
			}
			if app.editContext.kind != tc.wantKind {
				t.Errorf("editContext kind = %v, want %v", app.editContext.kind, tc.wantKind)
			}
		})
	}
}

func TestFetchActiveTab(t *testing.T) {
	t.Parallel()

	t.Run("jql tab fetches by stored query", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.SearchIssuesFunc = func(context.Context, string, int, int) (*jira.SearchResult, error) {
			return &jira.SearchResult{}, nil
		}
		app := editFlowApp(t, fake)
		app.issuesList.AddJQLTab("project = X")

		cmd := app.fetchActiveTab()

		if cmd == nil {
			t.Fatal("expected fetch cmd for JQL tab")
		}
		cmd()
		if len(fake.SearchIssuesCalls) != 1 || fake.SearchIssuesCalls[0].JQL != "project = X" {
			t.Errorf("SearchIssues calls = %+v", fake.SearchIssuesCalls)
		}
	})

	t.Run("jql tab with empty query is noop", func(t *testing.T) {
		t.Parallel()
		app := editFlowApp(t, &jiratest.FakeClient{T: t})
		app.issuesList.AddJQLTab("")

		if cmd := app.fetchActiveTab(); cmd != nil {
			t.Error("expected nil cmd for empty JQL tab")
		}
	})

	t.Run("empty tab jql is noop", func(t *testing.T) {
		t.Parallel()
		app := editFlowApp(t, &jiratest.FakeClient{T: t})
		app.projectKey = testProject
		app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: ""}})

		if cmd := app.fetchActiveTab(); cmd != nil {
			t.Error("expected nil cmd when tab has no JQL")
		}
	})
}

func TestView_BottomBarVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(t *testing.T, app *App)
	}{
		{
			name:  "active search bar",
			setup: func(_ *testing.T, app *App) { app.searchBar.Activate() },
		},
		{
			name:  "jql modal visible",
			setup: func(_ *testing.T, app *App) { app.jqlModal.Show("", nil) },
		},
		{
			name: "modal searching",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.modal.Show("Pick", []components.ModalItem{{ID: "1", Label: "A"}})
				app.modal, _ = app.modal.Update(runeKey('/'))
				if !app.modal.IsSearching() {
					t.Fatal("modal should be searching")
				}
			},
		},
		{
			name: "help search input",
			setup: func(_ *testing.T, app *App) {
				app.showHelp = true
				app.helpSearching = true
			},
		},
		{
			name: "create form filtering",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.createForm = formWithFields([]components.CreateFormField{{FieldID: "customfield_1", Name: "Team"}})
				_, _ = app.createForm.Intercept(tea.KeyMsg{Type: tea.KeyTab})
				_, _ = app.createForm.Intercept(tea.KeyMsg{Type: tea.KeyTab})
				_, _ = app.createForm.Intercept(runeKey('/'))
				if !app.createForm.IsFiltering() {
					t.Fatal("create form should be filtering")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := routingApp(t)
			tc.setup(t, app)

			if app.View() == "" {
				t.Error("View should render content")
			}
		})
	}
}
