package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
)

func TestHandleIssuesLoaded(t *testing.T) {
	t.Parallel()

	t.Run("active tab populates list and marks online", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.statusPanel.SetError("stale")

		_, cmd := app.handleIssuesLoaded(issuesLoadedMsg{tab: 0, issues: []jira.Issue{{Key: testKey}}})

		if sel := app.issuesList.SelectedIssue(); sel == nil || sel.Key != testKey {
			t.Errorf("selected issue = %v, want %s", sel, testKey)
		}
		if app.statusPanel.ErrorMessage() != "" {
			t.Error("error should be cleared on successful load")
		}
		if cmd == nil {
			t.Error("expected prefetch/preview commands")
		}
	})

	t.Run("git detected key selects matching issue", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		app := newAppWithFake(t, fake)
		app.projectKey = testProject
		app.gitDetectedKey = testKey

		_, _ = app.handleIssuesLoaded(issuesLoadedMsg{tab: 0, issues: []jira.Issue{{Key: testKey}}})

		if app.gitDetectedKey != "" {
			t.Error("gitDetectedKey should clear once the issue is selected")
		}
	})
}

func TestHandleIssueUpdated(t *testing.T) {
	t.Parallel()

	t.Run("non parent field refetches", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		_, cmd := app.handleIssueUpdated(issueUpdatedMsg{issueKey: testKey, field: testSummary})
		if cmd == nil {
			t.Error("expected refetch command")
		}
	})

	t.Run("parent field invalidates tab cache", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		_, cmd := app.handleIssueUpdated(issueUpdatedMsg{issueKey: testKey, field: "parent"})
		if cmd == nil {
			t.Error("expected refetch command after parent change")
		}
	})
}

func TestHandleIssueCreated(t *testing.T) {
	t.Parallel()

	t.Run("created issue hides form and resets context", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{{FieldID: "summary"}})
		app.createCtx = createCtx{projectKey: testProject}

		_, cmd := app.handleIssueCreated(issueCreatedMsg{issue: &jira.Issue{Key: "PLAT-99"}})

		if app.createForm.IsVisible() {
			t.Error("create form should hide after creation")
		}
		if app.createCtx.projectKey != "" {
			t.Error("createCtx should reset")
		}
		if app.gitDetectedKey != "PLAT-99" {
			t.Errorf("gitDetectedKey = %q, want PLAT-99 for auto-select", app.gitDetectedKey)
		}
		if cmd == nil {
			t.Error("expected refresh commands")
		}
	})

	t.Run("nil issue still hides form", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = formWithFields([]components.CreateFormField{{FieldID: "summary"}})

		_, _ = app.handleIssueCreated(issueCreatedMsg{})

		if app.createForm.IsVisible() {
			t.Error("create form should hide even without a created issue")
		}
	})
}

func TestHandleCreateFormTypeSelected(t *testing.T) {
	t.Parallel()

	t.Run("uncached type fetches create meta", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetCreateMetaFunc = func(context.Context, string, string) ([]jira.CreateMetaField, error) {
			return nil, nil
		}
		app := newAppWithFake(t, fake)
		app.createForm = components.NewCreateForm(nil)
		app.createCtx = createCtx{projectKey: testProject}

		_, cmd := app.handleCreateFormTypeSelected(components.CreateFormTypeSelectedMsg{TypeID: "10001", TypeName: "Story"})

		if cmd == nil {
			t.Fatal("expected fetch create meta command")
		}
		if app.createCtx.issueTypeID != "10001" {
			t.Errorf("issueTypeID = %q, want 10001", app.createCtx.issueTypeID)
		}
	})

	t.Run("cached type builds form directly", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createForm = components.NewCreateForm(nil)
		app.usersCache = map[string][]jira.User{testProject: {}}
		app.createCtx = createCtx{projectKey: testProject}
		app.createMetaCache[testProject+":10001"] = []jira.CreateMetaField{
			{FieldID: "summary", Name: "Summary", Required: true, Schema: jira.CreateMetaSchema{Type: "string", System: "summary"}},
		}

		_, _ = app.handleCreateFormTypeSelected(components.CreateFormTypeSelectedMsg{TypeID: "10001", TypeName: "Story"})

		if !app.createForm.IsVisible() {
			t.Error("create form should show for cached meta")
		}
	})
}

func TestHandleCreateMetaLoaded(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.createForm = components.NewCreateForm(nil)
	app.usersCache = map[string][]jira.User{testProject: {}}
	app.createCtx = createCtx{projectKey: testProject, issueTypeID: "10001", issueTypeName: "Story"}

	_, _ = app.handleCreateMetaLoaded(createMetaLoadedMsg{fields: []jira.CreateMetaField{
		{FieldID: "summary", Name: "Summary", Required: true, Schema: jira.CreateMetaSchema{Type: "string", System: "summary"}},
	}})

	if !app.createForm.IsVisible() {
		t.Error("create form should be visible")
	}
	if _, ok := app.createMetaCache[testProject+":10001"]; !ok {
		t.Error("create meta should be cached")
	}
}

func TestHandleCreateMetaLoaded_DuplicatePrefill(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.createForm = components.NewCreateForm(nil)
	app.usersCache = map[string][]jira.User{testProject: {}}
	app.createCtx = createCtx{
		projectKey:    testProject,
		issueTypeID:   "10001",
		issueTypeName: "Story",
		duplicateFrom: &jira.Issue{Summary: "Original"},
	}

	_, _ = app.handleCreateMetaLoaded(createMetaLoadedMsg{fields: []jira.CreateMetaField{
		{FieldID: "summary", Name: "Summary", Required: true, Schema: jira.CreateMetaSchema{Type: "string", System: "summary"}},
	}})

	if !app.createForm.IsVisible() {
		t.Error("create form should be visible after duplicate prefill")
	}
}

func TestBuildCreateFields(t *testing.T) {
	t.Parallel()

	t.Run("synthesizes summary and description when absent", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})

		fields := app.buildCreateFields(nil)

		if len(fields) < 2 {
			t.Fatalf("fields = %d, want at least summary and description", len(fields))
		}
		testkit.AssertEqual(t, "first field", fields[0].FieldID, "summary")
		testkit.AssertEqual(t, "second field", fields[1].FieldID, "description")
	})

	t.Run("includes known and supported custom fields", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})

		fields := app.buildCreateFields([]jira.CreateMetaField{
			{FieldID: fldPriority, Name: "Priority", Schema: jira.CreateMetaSchema{Type: fldPriority, System: fldPriority}},
			{FieldID: "customfield_5", Name: "Points", Schema: jira.CreateMetaSchema{Type: "number"}},
			{FieldID: "issuelinks", Name: "Linked", Schema: jira.CreateMetaSchema{Type: schemaArray}},
		})

		ids := make(map[string]bool)
		for _, field := range fields {
			ids[field.FieldID] = true
		}
		if !ids[fldPriority] {
			t.Error("priority should be included")
		}
		if !ids["customfield_5"] {
			t.Error("supported custom field should be included")
		}
		if ids["issuelinks"] {
			t.Error("issuelinks should be skipped")
		}
	})
}

func TestMetaToFormField(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})

	tests := []struct {
		name   string
		schema jira.CreateMetaSchema
		want   int
	}{
		{"description is multitext", jira.CreateMetaSchema{System: "description"}, components.CFFieldMultiText},
		{"priority is single select", jira.CreateMetaSchema{System: fldPriority}, components.CFFieldSingleSelect},
		{"assignee is person", jira.CreateMetaSchema{System: fldAssignee}, components.CFFieldPerson},
		{"labels is multiselect", jira.CreateMetaSchema{System: fldLabels}, components.CFFieldMultiSelect},
		{"option type is single select", jira.CreateMetaSchema{Type: "option"}, components.CFFieldSingleSelect},
		{"array of option is multiselect", jira.CreateMetaSchema{Type: schemaArray, Items: "option"}, components.CFFieldMultiSelect},
		{"user type is person", jira.CreateMetaSchema{Type: schemaUser}, components.CFFieldPerson},
		{"plain string is single text", jira.CreateMetaSchema{Type: "string"}, components.CFFieldSingleText},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			field := app.metaToFormField(jira.CreateMetaField{FieldID: "f", Name: "F", Schema: testCase.schema})
			testkit.AssertEqual(t, "field type", field.Type, testCase.want)
		})
	}
}

func TestMetaToFormField_OptionalGetsNonePlaceholder(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})

	field := app.metaToFormField(jira.CreateMetaField{
		FieldID: "customfield_5",
		Name:    "Points",
		Schema:  jira.CreateMetaSchema{Type: "string"},
	})

	testkit.AssertEqual(t, "placeholder", field.DisplayValue, "None")
}

func TestMetaToFormField_PrefillsReporterWithCurrentUser_Cloud(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.isCloud = true
	app.currentUser = &jira.User{AccountID: "acc-1", DisplayName: "Alice"}

	field := app.metaToFormField(jira.CreateMetaField{
		FieldID: "reporter", Name: "Reporter", Required: true,
		Schema: jira.CreateMetaSchema{Type: schemaUser, System: "reporter"},
	})

	testkit.AssertEqual(t, "display", field.DisplayValue, "Alice")
	v, ok := field.Value.(map[string]string)
	if !ok || v[fldAccountID] != "acc-1" {
		t.Errorf("reporter Value = %v, want {accountId: acc-1}", field.Value)
	}
}

func TestMetaToFormField_PrefillsReporter_ServerUsesNameKey(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.isCloud = false
	app.currentUser = &jira.User{AccountID: "bob", DisplayName: "Bob"}

	field := app.metaToFormField(jira.CreateMetaField{
		FieldID: "reporter", Name: "Reporter",
		Schema: jira.CreateMetaSchema{Type: schemaUser, System: "reporter"},
	})

	v, ok := field.Value.(map[string]string)
	if !ok || v[fldName] != "bob" {
		t.Errorf("reporter Value = %v, want {name: bob}", field.Value)
	}
}

func TestMetaToFormField_ReporterEmptyWithoutCurrentUser(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.currentUser = nil

	field := app.metaToFormField(jira.CreateMetaField{
		FieldID: "reporter", Name: "Reporter",
		Schema: jira.CreateMetaSchema{Type: schemaUser, System: "reporter"},
	})

	if field.Value != nil {
		t.Errorf("reporter should stay empty without a current user, got %v", field.Value)
	}
}

func TestMetaToFormField_AssigneeNotPrefilled(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.isCloud = true
	app.currentUser = &jira.User{AccountID: "acc-1", DisplayName: "Alice"}

	field := app.metaToFormField(jira.CreateMetaField{
		FieldID: fldAssignee, Name: "Assignee",
		Schema: jira.CreateMetaSchema{Type: schemaUser, System: fldAssignee},
	})

	if field.Value != nil {
		t.Errorf("assignee must not be auto-prefilled, got %v", field.Value)
	}
}

func TestApplyDuplicatePrefill(t *testing.T) {
	t.Parallel()

	fields := []components.CreateFormField{
		{FieldID: "summary"},
		{FieldID: "description"},
		{FieldID: fldPriority},
		{FieldID: fldAssignee},
		{FieldID: fldLabels},
		{FieldID: fldComponents},
		{FieldID: fldSprint},
		{FieldID: "customfield_5"},
	}
	source := &jira.Issue{
		Summary:     "Original",
		Description: "Body",
		Priority:    &jira.Priority{ID: "2", Name: "High"},
		Assignee:    &jira.User{AccountID: "u1", DisplayName: "Ann"},
		Labels:      []string{"backend"},
		Components:  []jira.Component{{ID: "10", Name: "core"}},
		Sprint:      &jira.Sprint{ID: 7, Name: "Sprint 7"},
		CustomFields: map[string]any{
			"customfield_5": "8",
		},
	}

	applyDuplicatePrefill(fields, source, true)

	testkit.AssertEqual(t, "summary", fields[0].DisplayValue, "Copy of Original")
	testkit.AssertEqual(t, "description", fields[1].DisplayValue, "Body")
	testkit.AssertEqual(t, "priority", fields[2].DisplayValue, "High")
	testkit.AssertEqual(t, "assignee", fields[3].DisplayValue, "Ann")
	testkit.AssertEqual(t, "labels", fields[4].DisplayValue, "backend")
	testkit.AssertEqual(t, "components", fields[5].DisplayValue, "core")
	testkit.AssertEqual(t, "sprint", fields[6].DisplayValue, "Sprint 7")
	testkit.AssertEqual(t, "custom", fields[7].DisplayValue, "8")
}

func TestHandleIssueTypesLoaded(t *testing.T) {
	t.Parallel()

	t.Run("create intent filters subtasks and installs handler", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.createCtx = createCtx{intent: true}

		_, _ = app.handleIssueTypesLoaded(issueTypesLoadedMsg{issueTypes: []jira.IssueType{
			{ID: "1", Name: "Story"},
			{ID: "2", Name: "Sub-task", Subtask: true},
		}})

		if !app.modal.IsVisible() {
			t.Error("issue type modal should be visible")
		}
		if app.onSelect == nil {
			t.Error("onSelect should be set for create intent")
		}
		if app.createCtx.intent {
			t.Error("intent should be cleared")
		}
	})

	t.Run("plain load shows all types", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})

		_, _ = app.handleIssueTypesLoaded(issueTypesLoadedMsg{issueTypes: []jira.IssueType{{ID: "1", Name: "Story"}}})

		if !app.modal.IsVisible() {
			t.Error("issue type modal should be visible")
		}
	})
}

func TestHandleProjectsLoaded(t *testing.T) {
	t.Parallel()

	t.Run("empty project key adopts first project", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.demoMode = true

		_, _ = app.handleProjectsLoaded(projectsLoadedMsg{projects: []jira.Project{{Key: testProject, ID: "1"}}})

		testkit.AssertEqual(t, "projectKey", app.projectKey, testProject)
	})

	t.Run("existing project key is preserved", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.demoMode = true
		app.projectKey = testProject

		_, cmd := app.handleProjectsLoaded(projectsLoadedMsg{projects: []jira.Project{{Key: "OPS", ID: "2"}}})

		testkit.AssertEqual(t, "projectKey", app.projectKey, testProject)
		if cmd != nil {
			t.Error("expected nil cmd when project key already set")
		}
	})

	t.Run("config-preset key resolves ID from loaded list", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.demoMode = true
		app.projectKey = testProject // simulates projects: config seeding the key with no ID

		_, cmd := app.handleProjectsLoaded(projectsLoadedMsg{projects: []jira.Project{{Key: testProject, ID: "10005"}}})

		testkit.AssertEqual(t, "projectID", app.projectID, "10005")
		if cmd != nil {
			t.Error("expected nil cmd when project key already set")
		}
	})

	t.Run("config-preset key with no match leaves ID empty", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.demoMode = true
		app.projectKey = testProject

		_, _ = app.handleProjectsLoaded(projectsLoadedMsg{projects: []jira.Project{{Key: "OPS", ID: "2"}}})

		testkit.AssertEqual(t, "projectID", app.projectID, "")
	})
}

func TestPrefetchChildrenDetails(t *testing.T) {
	t.Parallel()

	t.Run("uncached keys produce a batch command", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		cmd := app.prefetchChildrenDetails([]jira.Issue{{Key: "A-1"}, {Key: ""}})
		if cmd == nil {
			t.Error("expected batch prefetch command")
		}
	})

	t.Run("all cached produces no command", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.issueCache["A-1"] = &jira.Issue{Key: "A-1"}
		cmd := app.prefetchChildrenDetails([]jira.Issue{{Key: "A-1"}})
		if cmd != nil {
			t.Error("expected nil cmd when everything is cached")
		}
	})
}

func TestHandlePrioritiesLoaded_InstallsCallback(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})

	_, _ = app.handlePrioritiesLoaded(prioritiesLoadedMsg{priorities: []jira.Priority{{ID: "1", Name: "High"}}})

	if !app.modal.IsVisible() {
		t.Error("priority modal should be visible")
	}
	if app.onSelect == nil {
		t.Fatal("onSelect should be installed")
	}
	if cmd := app.onSelect(components.ModalItem{ID: "1", Label: "High"}); cmd == nil {
		t.Error("priority callback should issue an update command")
	}
}

func TestHandleUsersLoaded_CreateSentinel(t *testing.T) {
	t.Parallel()

	t.Run("checklist callback shows user checklist", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.usersCache = map[string][]jira.User{}
		app.onChecklist = func([]components.ModalItem) tea.Cmd { return nil }

		_, _ = app.handleUsersLoaded(usersLoadedMsg{
			users:    []jira.User{{AccountID: "u1", DisplayName: "Ann"}},
			issueKey: createUsersSentinel,
		})

		if !app.modal.IsVisible() {
			t.Error("user checklist should be visible")
		}
	})

	t.Run("no checklist falls back to picker", func(t *testing.T) {
		t.Parallel()
		app := newAppWithFake(t, &jiratest.FakeClient{T: t})
		app.usersCache = map[string][]jira.User{}

		_, _ = app.handleUsersLoaded(usersLoadedMsg{
			users:    []jira.User{{AccountID: "u1", DisplayName: "Ann"}},
			issueKey: createUsersSentinel,
		})

		if !app.modal.IsVisible() {
			t.Error("user picker should be visible")
		}
	})
}
