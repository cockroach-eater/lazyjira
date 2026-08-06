package tui

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func routingApp(t *testing.T) *App {
	t.Helper()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.keymap = DefaultKeymap()
	app.width = 120
	app.height = 40
	app.layoutPanels()
	return app
}

type routingCase struct {
	name    string
	setup   func(app *App, fake *jiratest.FakeClient)
	msg     tea.Msg
	wantCmd bool
	assert  func(t *testing.T, app *App)
}

func runRoutingCases(t *testing.T, cases []routingCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &jiratest.FakeClient{T: t}
			app := newAppWithFake(t, fake)
			app.keymap = DefaultKeymap()
			app.width = 120
			app.height = 40
			app.layoutPanels()
			if tc.setup != nil {
				tc.setup(app, fake)
			}

			_, cmd := app.Update(tc.msg)

			if tc.wantCmd && cmd == nil {
				t.Error("expected non-nil cmd")
			}
			if tc.assert != nil {
				tc.assert(t, app)
			}
		})
	}
}

func TestUpdate_RoutesCoreMessages(t *testing.T) {
	t.Parallel()
	runRoutingCases(t, []routingCase{
		{
			name: "window size sets dimensions",
			msg:  tea.WindowSizeMsg{Width: 100, Height: 30},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.width != 100 || app.height != 30 {
					t.Errorf("size = (%d,%d), want (100,30)", app.width, app.height)
				}
			},
		},
		{
			name: "mouse message dispatches to mouse handler",
			msg:  tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 5, Y: 0},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.leftFocus != focusStatus {
					t.Errorf("leftFocus = %v, want focusStatus after status click", app.leftFocus)
				}
			},
		},
		{
			name: "handled key message returns from key handler",
			msg:  runeKey('?'),
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.showHelp {
					t.Error("? key should open help")
				}
			},
		},
		{
			name: "unhandled key message falls through to panel routing",
			msg:  tea.KeyMsg{Type: tea.KeyF1},
		},
		{
			name: "search changed filters issues",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey, Summary: "alpha"}, {Key: mainKey, Summary: "beta"}})
			},
			msg: components.SearchChangedMsg{Query: "alpha"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if sel := app.issuesList.SelectedIssue(); sel == nil || sel.Key != testKey {
					t.Errorf("filter should select the alpha issue, got %v", sel)
				}
			},
		},
		{
			name: "search confirmed without selection",
			msg:  components.SearchConfirmedMsg{},
		},
		{
			name: "search cancelled clears filters",
			msg:  components.SearchCancelledMsg{},
		},
		{
			name:    "auto fetch tick schedules next tick",
			msg:     autoFetchTickMsg{},
			wantCmd: true,
		},
	})
}

func TestUpdate_RoutesDataMessages(t *testing.T) {
	t.Parallel()
	runRoutingCases(t, []routingCase{
		{
			name: "issues loaded fills active tab",
			setup: func(app *App, fake *jiratest.FakeClient) {
				stubFullIssueFetch(fake, &jira.Issue{Key: testKey})
			},
			msg:     issuesLoadedMsg{issues: []jira.Issue{{Key: testKey}}, tab: 0},
			wantCmd: true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if sel := app.issuesList.SelectedIssue(); sel == nil || sel.Key != testKey {
					t.Errorf("issues list selection = %v, want %s", sel, testKey)
				}
			},
		},
		{
			name: "issue detail loaded caches issue",
			msg:  issueDetailLoadedMsg{issue: &jira.Issue{Key: testKey}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if _, ok := app.issueCache[testKey]; !ok {
					t.Error("issue should be cached")
				}
			},
		},
		{
			name: "issue prefetched caches silently",
			msg:  issuePrefetchedMsg{issue: &jira.Issue{Key: testKey}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if _, ok := app.issueCache[testKey]; !ok {
					t.Error("prefetched issue should be cached")
				}
			},
		},
		{
			name: "batch prefetched caches all",
			msg:  batchPrefetchedMsg{issues: []jira.Issue{{Key: testKey}, {Key: mainKey}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if len(app.issueCache) != 2 {
					t.Errorf("cache size = %d, want 2", len(app.issueCache))
				}
			},
		},
		{
			name: "projects loaded in demo mode",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.demoMode = true
			},
			msg: projectsLoadedMsg{projects: []jira.Project{{Key: testProject}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.projectList.SelectedProject() == nil {
					t.Error("project list should be populated")
				}
			},
		},
		{
			name: "transition done without selection is noop",
			msg:  transitionDoneMsg{},
		},
		{
			name: "transitions loaded shows modal",
			msg: transitionsLoadedMsg{issueKey: testKey, transitions: []jira.Transition{
				{ID: "31", Name: "Done", To: &jira.Status{Name: "Done"}},
			}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("transition modal should be visible")
				}
			},
		},
		{
			name: "priorities loaded shows modal",
			msg:  prioritiesLoadedMsg{priorities: []jira.Priority{{ID: "1", Name: "High"}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("priority modal should be visible")
				}
			},
		},
		{
			name: "fields discovered error surfaces in status panel",
			msg:  fieldsDiscoveredMsg{err: errors.New("discover failed")},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.statusPanel.ErrorMessage() != "discover failed" {
					t.Errorf("status error = %q", app.statusPanel.ErrorMessage())
				}
			},
		},
		{
			name: "fields discovered without error",
			msg:  fieldsDiscoveredMsg{},
		},
		{
			name: "boards loaded resolves board id",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
			},
			msg: boardsLoadedMsg{boards: []jira.Board{{ID: 7, ProjectKey: testProject}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.boardID != 7 {
					t.Errorf("boardID = %d, want 7", app.boardID)
				}
			},
		},
		{
			name: "sprints loaded without selection is noop",
			msg:  sprintsLoadedMsg{sprints: []jira.Sprint{{ID: 1, Name: "S1"}}},
		},
		{
			name: "prefetch users for current project fetches",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				fake.GetUsersFunc = func(context.Context, string) ([]jira.User, error) { return nil, nil }
			},
			msg:     prefetchUsersMsg{projectKey: testProject},
			wantCmd: true,
		},
		{
			name: "prefetch users skipped when cached",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				app.usersCache[testProject] = []jira.User{}
			},
			msg: prefetchUsersMsg{projectKey: testProject},
		},
		{
			name: "prefetch users skipped for stale project",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
			},
			msg: prefetchUsersMsg{projectKey: "OTHER"},
		},
		{
			name: "users loaded caches project users",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
			},
			msg: usersLoadedMsg{users: []jira.User{{AccountID: "u1", DisplayName: "Ann"}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if len(app.usersCache[testProject]) != 1 {
					t.Error("users should be cached for project")
				}
			},
		},
		{
			name: "labels loaded without selection is noop",
			msg:  labelsLoadedMsg{labels: []string{"backend"}},
		},
		{
			name: "components loaded without selection is noop",
			msg:  componentsLoadedMsg{components: []jira.Component{{ID: "1", Name: "API"}}},
		},
		{
			name: "issue types loaded shows picker",
			msg:  issueTypesLoadedMsg{issueTypes: []jira.IssueType{{ID: "1", Name: "Story"}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("issue type modal should be visible")
				}
			},
		},
		{
			name: "custom field options not found opens input modal",
			msg:  customFieldOptionsMsg{issueKey: testKey, fieldID: "customfield_1", fieldName: "Points", fieldNotFound: true},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.inputModal.IsVisible() {
					t.Error("input modal should be visible")
				}
			},
		},
	})
}

func TestUpdate_RoutesCreateAndEditMessages(t *testing.T) {
	t.Parallel()
	runRoutingCases(t, []routingCase{
		{
			name: "create meta loaded shows form",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				app.usersCache[testProject] = []jira.User{}
				app.createCtx = createCtx{projectKey: testProject, issueTypeID: "10001", issueTypeName: "Story"}
			},
			msg: createMetaLoadedMsg{fields: []jira.CreateMetaField{}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.createForm.IsVisible() {
					t.Error("create form should be visible")
				}
			},
		},
		{
			name: "issue created hides form and refetches",
			setup: func(app *App, fake *jiratest.FakeClient) {
				stubFullIssueFetch(fake, &jira.Issue{Key: testKey})
			},
			msg:     issueCreatedMsg{issue: &jira.Issue{Key: testKey}},
			wantCmd: true,
		},
		{
			name: "create error surfaces on form",
			msg:  createErrorMsg{err: errors.New("boom")},
		},
		{
			name: "pre-form create error aborts to status",
			setup: func(app *App, _ *jiratest.FakeClient) {
				app.createCtx = createCtx{projectKey: "DSOTEST", parentKey: testKey}
				app.createForm.SetLoading(true)
			},
			msg: createPreFormErrorMsg{err: &jira.APIError{
				Status:   404,
				Messages: []string{"You cannot create issues in this project."},
			}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.createForm.IsVisible() {
					t.Error("form should be hidden after a pre-form error")
				}
				if app.statusPanel.ErrorMessage() == "" {
					t.Error("status panel should carry the pre-form error")
				}
			},
		},
		{
			name:    "issue updated refetches detail",
			setup:   func(app *App, fake *jiratest.FakeClient) { stubFullIssueFetch(fake, &jira.Issue{Key: testKey}) },
			msg:     issueUpdatedMsg{issueKey: testKey, field: "summary"},
			wantCmd: true,
		},
		{
			name:    "comment added refetches detail",
			setup:   func(app *App, fake *jiratest.FakeClient) { stubFullIssueFetch(fake, &jira.Issue{Key: testKey}) },
			msg:     commentAddedMsg{issueKey: testKey},
			wantCmd: true,
		},
		{
			name:    "comment updated refetches detail",
			setup:   func(app *App, fake *jiratest.FakeClient) { stubFullIssueFetch(fake, &jira.Issue{Key: testKey}) },
			msg:     commentUpdatedMsg{issueKey: testKey},
			wantCmd: true,
		},
		{
			name: "create form type selected fetches meta",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.createCtx = createCtx{intent: true, projectKey: testProject}
				fake.GetCreateMetaFunc = func(context.Context, string, string) ([]jira.CreateMetaField, error) { return nil, nil }
			},
			msg:     components.CreateFormTypeSelectedMsg{TypeID: "10001", TypeName: "Story"},
			wantCmd: true,
		},
		{
			name: "create form edit text opens input modal",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.createForm = formWithFields([]components.CreateFormField{{FieldID: "customfield_1", Name: "Points"}})
			},
			msg: components.CreateFormEditTextMsg{FieldIndex: 0},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.inputModal.IsVisible() {
					t.Error("input modal should be visible")
				}
			},
		},
		{
			name: "create form edit external launches editor for missing field",
			msg:  components.CreateFormEditExternalMsg{FieldIndex: 5},
		},
		{
			name: "create form picker with items shows modal",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.createForm = formWithFields([]components.CreateFormField{{FieldID: "customfield_1", Name: "Team", Type: components.CFFieldSingleSelect}})
			},
			msg: components.CreateFormPickerMsg{FieldIndex: 0, Items: []components.ModalItem{{ID: "1", Label: "A"}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("picker modal should be visible")
				}
			},
		},
		{
			name: "create form checklist with items shows modal",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.createForm = formWithFields([]components.CreateFormField{{FieldID: "customfield_1", Name: "Tags", Type: components.CFFieldMultiSelect}})
			},
			msg: components.CreateFormChecklistMsg{FieldIndex: 0, Items: []components.ModalItem{{ID: "1", Label: "A"}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("checklist modal should be visible")
				}
			},
		},
		{
			name: "create form submit creates issue",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.createCtx = createCtx{projectKey: testProject, issueTypeID: "10001"}
				fake.CreateIssueFunc = func(context.Context, map[string]any) (*jira.Issue, error) { return &jira.Issue{Key: testKey}, nil }
			},
			msg:     components.CreateFormSubmitMsg{Fields: map[string]any{"summary": testSummary}},
			wantCmd: true,
		},
		{
			name: "create form cancel resets context",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.createCtx = createCtx{projectKey: testProject}
			},
			msg: components.CreateFormCancelMsg{},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.createCtx.projectKey != "" {
					t.Error("createCtx should be reset")
				}
			},
		},
		{
			name: "modal selected dispatches callback",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.onSelect = func(components.ModalItem) tea.Cmd { return func() tea.Msg { return nil } }
			},
			msg:     components.ModalSelectedMsg{Item: components.ModalItem{ID: "1"}},
			wantCmd: true,
		},
		{
			name: "checklist confirmed dispatches callback",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.onChecklist = func(components.ChecklistConfirmedMsg) tea.Cmd { return func() tea.Msg { return nil } }
			},
			msg:     components.ChecklistConfirmedMsg{},
			wantCmd: true,
		},
		{
			name: "modal cancelled clears callbacks",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.onSelect = func(components.ModalItem) tea.Cmd { return nil }
			},
			msg: components.ModalCancelledMsg{},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.onSelect != nil {
					t.Error("onSelect should be cleared")
				}
			},
		},
		{
			name:    "editor finished with error keeps mouse cmd",
			msg:     editorFinishedMsg{err: errNoEditor},
			wantCmd: true,
		},
		{
			name:    "custom command finished with error",
			msg:     customCommandFinishedMsg{err: errors.New("exit 1")},
			wantCmd: true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.statusPanel.ErrorMessage() == "" {
					t.Error("command error should surface in status panel")
				}
			},
		},
		{
			name: "diff confirmed applies edit",
			msg:  components.DiffConfirmedMsg{Content: "body"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.editTempPath != "" {
					t.Errorf("editTempPath = %q after DiffConfirmedMsg, want empty (cleanup)", app.editTempPath)
				}
			},
		},
		{
			name: "diff cancelled cleans up",
			msg:  components.DiffCancelledMsg{},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.editTempPath != "" {
					t.Errorf("editTempPath = %q after DiffCancelledMsg, want empty (cleanup)", app.editTempPath)
				}
			},
		},
		{
			name: "input confirmed without context is noop",
			msg:  components.InputConfirmedMsg{Text: "x"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.editContext.kind != editNone {
					t.Errorf("editContext.kind = %v after InputConfirmedMsg with no ctx, want editNone", app.editContext.kind)
				}
			},
		},
		{
			name: "input cancelled resets context",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.editContext = editCtx{kind: editSummary, issueKey: testKey}
			},
			msg: components.InputCancelledMsg{},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.editContext.kind != editNone {
					t.Error("editContext should reset")
				}
			},
		},
	})
}

func TestUpdate_RoutesJQLAndNavMessages(t *testing.T) {
	t.Parallel()
	runRoutingCases(t, []routingCase{
		{
			name: "jql submit starts search",
			setup: func(app *App, fake *jiratest.FakeClient) {
				fake.SearchIssuesFunc = func(context.Context, string, int, int) (*jira.SearchResult, error) { return &jira.SearchResult{}, nil }
			},
			msg:     components.JQLSubmitMsg{Query: "project = X"},
			wantCmd: true,
		},
		{
			name: "jql search error sets modal error",
			msg:  jqlSearchErrorMsg{err: "bad jql"},
		},
		{
			name: "jql cancel is noop",
			msg:  components.JQLCancelMsg{},
		},
		{
			name: "jql fields loaded caches fields",
			msg:  jqlFieldsLoadedMsg{fields: []jira.AutocompleteField{{Value: "project"}}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.jqlFields == nil {
					t.Error("jqlFields should be cached")
				}
			},
		},
		{
			name: "jql suggestions when modal hidden is noop",
			msg:  jqlSuggestionsMsg{suggestions: []jira.AutocompleteSuggestion{{Value: "PLAT"}}},
		},
		{
			name: "navigate issue message moves selection",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
			},
			msg: views.NavigateIssueMsg{Key: testKey},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.leftFocus != focusIssues {
					t.Error("navigation should focus issues panel")
				}
			},
		},
		{
			name: "expand block shows read only modal",
			msg:  views.ExpandBlockMsg{Title: "Body", Lines: []string{"a"}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("expand modal should be visible")
				}
			},
		},
		{
			name: "git branch created updates branch",
			msg:  gitBranchCreatedMsg{name: "feat/x"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.gitBranch != "feat/x" {
					t.Errorf("gitBranch = %q", app.gitBranch)
				}
			},
		},
		{
			name: "git checkout done updates branch",
			msg:  gitCheckoutDoneMsg{name: "feat/y"},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.gitBranch != "feat/y" {
					t.Errorf("gitBranch = %q", app.gitBranch)
				}
			},
		},
		{
			name: "git error surfaces in status panel",
			msg:  gitErrorMsg{err: errors.New("git failed")},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.statusPanel.ErrorMessage() != "git failed" {
					t.Errorf("status error = %q", app.statusPanel.ErrorMessage())
				}
			},
		},
		{
			name: "error message shows modal",
			msg:  errorMsg{err: errors.New("api down")},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if !app.modal.IsVisible() {
					t.Error("error modal should be visible")
				}
				if app.statusPanel.ErrorMessage() != "api down" {
					t.Errorf("status error = %q", app.statusPanel.ErrorMessage())
				}
			},
		},
		{
			name: "issue selected nil is noop",
			msg:  views.IssueSelectedMsg{},
		},
		{
			name: "issue selected with cache uses cached issue",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.issueCache[testKey] = &jira.Issue{Key: testKey, Summary: "cached"}
			},
			msg: views.IssueSelectedMsg{Issue: &jira.Issue{Key: testKey}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.infoPanel.Issue() == nil || app.infoPanel.Issue().Summary != "cached" {
					t.Error("info panel should show the cached issue")
				}
			},
		},
	})
}

func TestUpdate_RoutesLifecycleMessages(t *testing.T) {
	t.Parallel()
	runRoutingCases(t, []routingCase{
		{
			name: "children request ignored off cloud",
			msg:  views.ChildrenRequestMsg{Key: testKey},
		},
		{
			name: "children walk request ignored off cloud",
			msg:  childrenWalkRequestMsg{key: testKey},
		},
		{
			name: "stale children loaded dropped",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.childrenEpoch = 5
			},
			msg: childrenLoadedMsg{key: testKey, epoch: 1},
		},
		{
			name: "stale parent loaded dropped",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.parentEpoch = 5
			},
			msg: parentLoadedMsg{childKey: testKey, parent: &jira.Issue{Key: mainKey}, epoch: 1},
		},
		{
			name: "parent loaded with error dropped",
			msg:  parentLoadedMsg{childKey: testKey, err: errors.New("nope")},
		},
		{
			name: "stale preview detail dropped",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.previewEpoch = 5
			},
			msg: previewDetailLoadedMsg{issue: &jira.Issue{Key: testKey}, epoch: 1},
		},
		{
			name: "preview detail with nil issue dropped",
			msg:  previewDetailLoadedMsg{issue: nil, epoch: 0},
		},
		{
			name: "stale preview debounce dropped",
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.previewEpoch = 5
			},
			msg: previewDebounceMsg{key: testKey, epoch: 1},
		},
		{
			name: "project hovered updates detail view",
			msg:  views.ProjectHoveredMsg{Project: &jira.Project{Key: testProject}},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.detailView.Mode() != views.ModeProject {
					t.Error("detail view should switch to project mode")
				}
			},
		},
		{
			name: "project hovered nil is noop",
			msg:  views.ProjectHoveredMsg{},
		},
		{
			name: "unknown message routes to focused panel",
			msg:  struct{ unknown bool }{unknown: true},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.side != sideLeft {
					t.Errorf("side = %v after unknown msg, want sideLeft (focused panel must not change)", app.side)
				}
			},
		},
	})
}

func TestUpdate_ActiveSearchBarConsumesKeys(t *testing.T) {
	t.Parallel()
	app := routingApp(t)
	app.searchBar.Activate()

	_, _ = app.Update(runeKey('a'))

	if got := app.searchBar.Query(); got != "a" {
		t.Errorf("search query = %q, want a", got)
	}
}

func TestUpdate_ActiveSearchBarPassesArrowKeysThrough(t *testing.T) {
	t.Parallel()
	app := routingApp(t)
	app.searchBar.Activate()
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}, {Key: mainKey}})

	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})

	if got := app.searchBar.Query(); got != "" {
		t.Errorf("arrow key must not be typed into the search bar, query = %q", got)
	}
}

func TestUpdate_VisibleOverlayInterceptsKeys(t *testing.T) {
	t.Parallel()
	app := routingApp(t)
	app.modal.Show("Pick", []components.ModalItem{{ID: "1", Label: "A"}})
	app.overlays = components.OverlayStack{&app.modal}

	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if app.modal.IsVisible() {
		t.Error("esc should close the intercepting modal")
	}
}
