package tui

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

func TestHandleKeyMsg_Dispatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		key         tea.KeyMsg
		setup       func(app *App, fake *jiratest.FakeClient)
		wantHandled bool
		assert      func(t *testing.T, app *App, cmd tea.Cmd)
	}{
		{
			name:        "quit returns quit command",
			key:         runeKey('q'),
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil {
					t.Fatal("expected quit cmd")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Error("expected tea.QuitMsg")
				}
			},
		},
		{
			name:        "help opens help overlay",
			key:         runeKey('?'),
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if !app.showHelp {
					t.Error("help overlay should open")
				}
			},
		},
		{
			name:        "help mode routes keys to help handler",
			key:         runeKey('j'),
			setup:       func(app *App, fake *jiratest.FakeClient) { app.showHelp = true },
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if app.helpCursor != 1 {
					t.Errorf("helpCursor = %d, want 1", app.helpCursor)
				}
			},
		},
		{
			name:        "search activates search bar",
			key:         runeKey('/'),
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if !app.searchBar.IsActive() {
					t.Error("search bar should activate")
				}
			},
		},
		{
			name: "custom command takes precedence and reports missing selection",
			key:  runeKey('z'),
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.customCmds = []config.ResolvedCustomCommand{{
					Key:      "z",
					Name:     "zap",
					Scopes:   config.ScopeIssue,
					Contexts: []config.Context{config.CtxIssues},
					Template: parseTmpl(t, "echo {{.Key}}"),
				}}
			},
			wantHandled: true,
		},
		{
			name:        "select opens issue detail",
			key:         tea.KeyMsg{Type: tea.KeySpace},
			setup:       seedSelectedIssue,
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if app.side != sideRight {
					t.Error("select should focus detail")
				}
			},
		},
		{
			name:        "open opens issue detail",
			key:         tea.KeyMsg{Type: tea.KeyEnter},
			setup:       seedSelectedIssue,
			wantHandled: true,
		},
		{
			name:        "url picker without issue is noop",
			key:         runeKey('u'),
			wantHandled: true,
		},
		{
			name:        "edit without issue is noop",
			key:         runeKey('e'),
			wantHandled: true,
		},
		{
			name:        "create branch without repo reports error",
			key:         runeKey('b'),
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if app.statusPanel.ErrorMessage() == "" {
					t.Error("missing repo should surface an error")
				}
			},
		},
		{
			name:        "show parent without parent is noop",
			key:         tea.KeyMsg{Type: tea.KeyBackspace},
			wantHandled: true,
		},
		{
			name: "show parent fetches parent issue",
			key:  tea.KeyMsg{Type: tea.KeyBackspace},
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey, Parent: &jira.Issue{Key: mainKey}}})
				fake.GetIssueFunc = func(context.Context, string) (*jira.Issue, error) { return &jira.Issue{Key: mainKey}, nil }
			},
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil {
					t.Fatal("expected parent fetch cmd")
				}
			},
		},
		{
			name:        "focus action switches panels",
			key:         tea.KeyMsg{Type: tea.KeyTab},
			wantHandled: true,
			assert: func(t *testing.T, app *App, cmd tea.Cmd) {
				t.Helper()
				if app.side != sideRight {
					t.Error("tab should switch side")
				}
			},
		},
		{
			name:        "tab action switches issue tabs",
			key:         runeKey(']'),
			setup:       func(app *App, fake *jiratest.FakeClient) { app.issuesList.SetTabs(nil) },
			wantHandled: true,
		},
		{
			name:        "copy url without issue is noop",
			key:         runeKey('y'),
			wantHandled: true,
		},
		{
			name:        "detail scroll fallthrough on issues focus",
			key:         runeKey('J'),
			wantHandled: true,
		},
		{
			name: "unbound key is not handled",
			key:  tea.KeyMsg{Type: tea.KeyF1},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			fake := &jiratest.FakeClient{T: t}
			app := newAppWithFake(t, fake)
			app.keymap = DefaultKeymap()
			app.width = 120
			app.height = 40
			app.layoutPanels()
			if testCase.setup != nil {
				testCase.setup(app, fake)
			}

			m, cmd := app.handleKeyMsg(testCase.key)

			if testCase.wantHandled != (m != nil) {
				t.Fatalf("handled = %v, want %v", m != nil, testCase.wantHandled)
			}
			if testCase.assert != nil {
				testCase.assert(t, app, cmd)
			}
		})
	}
}

func seedSelectedIssue(app *App, fake *jiratest.FakeClient) {
	stubFullIssueFetch(fake, &jira.Issue{Key: testKey})
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
}

func TestHandleDetailScroll_HalfPages(t *testing.T) {
	t.Parallel()
	app := focusApp(t)

	_, _, downHandled := app.handleDetailScroll(tea.KeyMsg{Type: tea.KeyCtrlF})
	_, _, upHandled := app.handleDetailScroll(tea.KeyMsg{Type: tea.KeyCtrlB})

	if !downHandled || !upHandled {
		t.Errorf("half page scroll handled = (%v,%v), want both", downHandled, upHandled)
	}
}

func TestHandleHelpKeys_HalfPageNavigation(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.showHelp = true

	_, _ = app.handleHelpKeys(tea.KeyMsg{Type: tea.KeyCtrlD})
	if app.helpCursor == 0 {
		t.Error("ctrl+d should move the help cursor down")
	}

	_, _ = app.handleHelpKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	if app.helpCursor != 0 {
		t.Errorf("ctrl+u should move back to top, cursor = %d", app.helpCursor)
	}
}

func TestHandleHelpKeys_DelegatesToSearchWhenSearching(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.showHelp = true
	app.helpSearching = true

	_, _ = app.handleHelpKeys(runeKey('q'))

	if app.helpFilter != "q" {
		t.Errorf("helpFilter = %q, want typed text routed to the search input", app.helpFilter)
	}
}

func TestHandleHelpSearchKey_UpMovesSelection(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.showHelp = true
	app.helpSearching = true
	app.helpCursor = 2

	_, _ = app.handleHelpSearchKey(tea.KeyMsg{Type: tea.KeyUp})

	if app.helpCursor != 1 {
		t.Errorf("helpCursor = %d, want 1", app.helpCursor)
	}
}

func TestHandleHelpSearchKey_TypingUpdatesFilterAndResetsCursor(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.showHelp = true
	app.helpSearching = true
	app.helpCursor = 3

	_, _ = app.handleHelpSearchKey(runeKey('e'))

	if app.helpFilter != "e" {
		t.Errorf("helpFilter = %q, want e", app.helpFilter)
	}
	if app.helpCursor != 0 {
		t.Errorf("helpCursor = %d, want 0", app.helpCursor)
	}
}

func TestHandleFocusAction_FullCycles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		side   focusSide
		focus  focusPanel
		action Action
		want   focusPanel
	}{
		{"right from status", sideLeft, focusStatus, ActFocusRight, focusIssues},
		{"right from info", sideLeft, focusInfo, ActFocusRight, focusProjects},
		{"right from projects wraps to status", sideLeft, focusProjects, ActFocusRight, focusStatus},
		{"left from status wraps to projects", sideLeft, focusStatus, ActFocusLeft, focusProjects},
		{"left from projects", sideLeft, focusProjects, ActFocusLeft, focusInfo},
		{"left from info", sideLeft, focusInfo, ActFocusLeft, focusIssues},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			app := focusApp(t)
			app.side = testCase.side
			app.leftFocus = testCase.focus

			_, _, handled := app.handleFocusAction(testCase.action)

			if !handled {
				t.Fatal("focus action should be handled")
			}
			if app.leftFocus != testCase.want {
				t.Errorf("leftFocus = %v, want %v", app.leftFocus, testCase.want)
			}
		})
	}
}

func TestHandleFocusAction_LeftFromRightSideReturnsLeft(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.side = sideRight

	_, _, handled := app.handleFocusAction(ActFocusLeft)

	if !handled || app.side != sideLeft {
		t.Errorf("handled=%v side=%v, want left", handled, app.side)
	}
}

func TestHandleFocusAction_LeftFromInfoSubTabResetsPreview(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.side = sideLeft
	app.leftFocus = focusInfo
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey, Subtasks: []jira.Issue{{Key: subKey1}}}})
	app.infoPanel.SetIssue(&jira.Issue{Key: testKey, Subtasks: []jira.Issue{{Key: subKey1}}})
	app.infoPanel.SetActiveTab(views.InfoTabSubtasks)

	_, cmd, handled := app.handleFocusAction(ActFocusLeft)

	if !handled || app.leftFocus != focusIssues {
		t.Fatalf("handled=%v focus=%v, want issues", handled, app.leftFocus)
	}
	if cmd == nil {
		t.Error("expected preview reset cmd when leaving sub tab")
	}
}

func TestHandleFocusAction_FocusIssuesShowsCachedIssue(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.side = sideRight
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
	app.issueCache[testKey] = &jira.Issue{Key: testKey, Summary: "cached"}

	_, _, handled := app.handleFocusAction(ActFocusIssues)

	if !handled {
		t.Fatal("ActFocusIssues should be handled")
	}
	if got := app.detailView.IssueKey(); got != testKey {
		t.Errorf("detail issue = %q, want %s", got, testKey)
	}
}

func TestHandleTabAction_MoreBranches(t *testing.T) {
	t.Parallel()

	t.Run("prev tab on detail side", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideRight

		_, _, handled := app.handleTabAction(ActPrevTab)

		if !handled {
			t.Error("ActPrevTab should be handled on detail side")
		}
	})

	t.Run("prev and next tab on info panel", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusInfo
		app.infoPanel.SetIssue(&jira.Issue{Key: testKey})

		_, _, nextHandled := app.handleTabAction(ActNextTab)
		_, _, prevHandled := app.handleTabAction(ActPrevTab)

		if !nextHandled || !prevHandled {
			t.Errorf("info tab switches handled = (%v,%v), want both", nextHandled, prevHandled)
		}
	})

	t.Run("next tab with cached issues previews instead of fetching", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.projectKey = testProject
		app.issuesList.SetTabs([]config.IssueTabConfig{
			{Name: "All", JQL: "project = X"},
			{Name: "Mine", JQL: "assignee = currentUser()"},
		})
		app.issuesList.SetIssuesForTab(1, []jira.Issue{{Key: testKey}})
		app.side = sideLeft
		app.leftFocus = focusIssues

		_, _, handled := app.handleTabAction(ActNextTab)

		if !handled {
			t.Fatal("ActNextTab should be handled")
		}
		if len(app.issuesList.CurrentIssues()) != 1 {
			t.Error("cached tab content should be restored without a fetch")
		}
	})

	t.Run("close jql tab on regular tab is noop", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusIssues

		_, cmd, handled := app.handleTabAction(ActCloseJQLTab)

		if !handled || cmd != nil {
			t.Errorf("handled=%v cmd=%v, want handled with nil cmd", handled, cmd)
		}
	})

	t.Run("jql search opens modal and loads autocomplete", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		fake.GetJQLAutocompleteDataFunc = func(context.Context) ([]jira.AutocompleteField, error) { return nil, nil }
		app := newAppWithFake(t, fake)
		app.keymap = DefaultKeymap()
		app.projectKey = testProject

		_, cmd, handled := app.handleTabAction(ActJQLSearch)

		if !handled {
			t.Fatal("ActJQLSearch should be handled")
		}
		if !app.jqlModal.IsVisible() {
			t.Error("JQL modal should be visible")
		}
		if cmd == nil {
			t.Error("expected autocomplete fetch cmd")
		}
	})
}

func TestHandleIssueAction_MoreBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		action  Action
		setup   func(app *App, fake *jiratest.FakeClient)
		wantCmd bool
		assert  func(t *testing.T, app *App)
	}{
		{
			name:   "transition on wrong focus is noop",
			action: ActTransition,
			setup:  func(app *App, fake *jiratest.FakeClient) { app.side = sideRight },
		},
		{
			name:   "transition with issue fetches transitions",
			action: ActTransition,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
				fake.GetTransitionsFunc = func(context.Context, string) ([]jira.Transition, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:   "comments without issue is noop",
			action: ActComments,
		},
		{
			name:   "duplicate issue from issues panel fetches types",
			action: ActDuplicateIssue,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				app.projectID = "10000"
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
				fake.GetIssueTypesFunc = func(context.Context, string) ([]jira.IssueType, error) { return nil, nil }
			},
			wantCmd: true,
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.createCtx.duplicateFrom == nil {
					t.Error("duplicate source should be recorded")
				}
			},
		},
		{
			name:   "duplicate issue on wrong focus is noop",
			action: ActDuplicateIssue,
			setup:  func(app *App, fake *jiratest.FakeClient) { app.side = sideRight },
		},
		{
			name:   "create issue action starts create flow",
			action: ActCreateIssue,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				app.projectID = "10000"
				fake.GetIssueTypesFunc = func(context.Context, string) ([]jira.IssueType, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:   "new on issues panel starts create flow",
			action: ActNew,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.projectKey = testProject
				app.projectID = "10000"
				fake.GetIssueTypesFunc = func(context.Context, string) ([]jira.IssueType, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:   "new on comments tab opens the comment popup",
			action: ActNew,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.side = sideRight
				issue := &jira.Issue{Key: testKey}
				app.issuesList.SetIssues([]jira.Issue{*issue})
				app.previewKey = testKey
				app.issueCache[testKey] = issue
				app.detailView.SetIssue(issue)
				app.detailView.SetActiveTab(views.TabComments)
			},
			assert: func(t *testing.T, app *App) {
				t.Helper()
				if app.editContext.kind != editCommentNew {
					t.Errorf("editContext kind = %v, want editCommentNew", app.editContext.kind)
				}
				if !app.textModal.IsVisible() {
					t.Error("the comment popup was not shown")
				}
			},
		},
		{
			name:   "new outside comments tab is noop",
			action: ActNew,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.side = sideRight
				issue := &jira.Issue{Key: testKey}
				app.previewKey = testKey
				app.issueCache[testKey] = issue
				app.detailView.SetIssue(issue)
				app.detailView.SetActiveTab(views.TabDetails)
			},
		},
		{
			name:   "priority with issue fetches priorities",
			action: ActPriority,
			setup: func(app *App, fake *jiratest.FakeClient) {
				app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
				fake.GetPrioritiesFunc = func(context.Context) ([]jira.Priority, error) { return nil, nil }
			},
			wantCmd: true,
		},
		{
			name:   "priority without issue is noop",
			action: ActPriority,
		},
		{
			name:   "assignee with cached users shows modal",
			action: ActAssignee,
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
			name:   "assignee without issue is noop",
			action: ActAssignee,
		},
		{
			name:    "refresh all refetches active tab",
			action:  ActRefreshAll,
			setup:   seedTabWithJQL,
			wantCmd: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			fake := &jiratest.FakeClient{T: t}
			app := newAppWithFake(t, fake)
			app.keymap = DefaultKeymap()
			app.width = 120
			app.height = 40
			app.layoutPanels()
			if testCase.setup != nil {
				testCase.setup(app, fake)
			}

			_, cmd, handled := app.handleIssueAction(testCase.action)

			if !handled {
				t.Fatal("action should be handled")
			}
			if testCase.wantCmd && cmd == nil {
				t.Error("expected non-nil cmd")
			}
			if testCase.assert != nil {
				testCase.assert(t, app)
			}
		})
	}
}

func seedTabWithJQL(app *App, fake *jiratest.FakeClient) {
	app.projectKey = testProject
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "project = {{.ProjectKey}}"}})
	fake.SearchIssuesFunc = func(context.Context, string, int, int) (*jira.SearchResult, error) {
		return &jira.SearchResult{}, nil
	}
}

func TestStartDuplicateIssue_RequiresProjectKey(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.projectKey = ""
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})

	_, cmd := app.startDuplicateIssue()

	if cmd != nil {
		t.Error("expected nil cmd without project key")
	}
}

func TestHandleActionSelect_MoreBranches(t *testing.T) {
	t.Parallel()

	t.Run("issues with subtasks walks into children", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusIssues
		app.issuesList.SetIssues([]jira.Issue{{Key: testKey, Subtasks: []jira.Issue{{Key: subKey1}}}})

		_, _ = app.handleActionSelect()

		if !app.issuesList.IsHierarchyTab() {
			t.Error("select on parent should open hierarchy tab")
		}
	})

	t.Run("info fields tab edits selected field", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusInfo
		issue := jira.Issue{Key: testKey, Parent: &jira.Issue{Key: mainKey}, IssueType: &jira.IssueType{ID: "10001"}}
		app.issuesList.SetIssues([]jira.Issue{issue})
		app.cfg.Fields = []config.FieldConfig{{ID: "parent"}}
		app.infoPanel.SetFields(app.cfg.Fields)
		app.infoPanel.SetIssue(&issue)
		app.infoPanel.Cursor = 0

		_, _ = app.handleActionSelect()

		if !app.inputModal.IsVisible() {
			t.Error("editing the parent field should open the input modal")
		}
	})

	t.Run("info fields tab without selection is noop", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusInfo

		_, cmd := app.handleActionSelect()

		if cmd != nil {
			t.Error("expected nil cmd without selection")
		}
	})

	t.Run("info links tab navigates to linked issue", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		stubFullIssueFetch(fake, &jira.Issue{Key: mainKey})
		app := newAppWithFake(t, fake)
		app.keymap = DefaultKeymap()
		app.width = 120
		app.height = 40
		app.layoutPanels()
		app.side = sideLeft
		app.leftFocus = focusInfo
		app.infoPanel.SetIssue(&jira.Issue{
			Key:        testKey,
			IssueLinks: []jira.IssueLink{{Type: &jira.IssueLinkType{Outward: "relates to"}, OutwardIssue: &jira.Issue{Key: mainKey}}},
		})
		app.infoPanel.SetActiveTab(views.InfoTabLinks)

		_, cmd := app.handleActionSelect()

		if cmd == nil {
			t.Error("expected fetch cmd for linked issue")
		}
		if app.leftFocus != focusIssues {
			t.Errorf("leftFocus = %v, want issues", app.leftFocus)
		}
	})

	t.Run("detail side is not handled", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideRight

		m, _ := app.handleActionSelect()

		if m != nil {
			t.Error("select on detail side should fall through")
		}
	})
}

func TestHandleActionOpen_MoreBranches(t *testing.T) {
	t.Parallel()

	t.Run("info fields tab edits selected field", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusInfo
		issue := jira.Issue{Key: testKey, Parent: &jira.Issue{Key: mainKey}, IssueType: &jira.IssueType{ID: "10001"}}
		app.issuesList.SetIssues([]jira.Issue{issue})
		app.cfg.Fields = []config.FieldConfig{{ID: "parent"}}
		app.infoPanel.SetFields(app.cfg.Fields)
		app.infoPanel.SetIssue(&issue)
		app.infoPanel.Cursor = 0

		_, _ = app.handleActionOpen()

		if !app.inputModal.IsVisible() {
			t.Error("editing the parent field should open the input modal")
		}
	})

	t.Run("info fields tab without selection is noop", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft
		app.leftFocus = focusInfo

		_, cmd := app.handleActionOpen()

		if cmd != nil {
			t.Error("expected nil cmd without selection")
		}
	})

	t.Run("info links tab opens linked issue detail", func(t *testing.T) {
		t.Parallel()
		fake := &jiratest.FakeClient{T: t}
		stubFullIssueFetch(fake, &jira.Issue{Key: mainKey})
		app := newAppWithFake(t, fake)
		app.keymap = DefaultKeymap()
		app.width = 120
		app.height = 40
		app.layoutPanels()
		app.side = sideLeft
		app.leftFocus = focusInfo
		app.infoPanel.SetIssue(&jira.Issue{
			Key:        testKey,
			IssueLinks: []jira.IssueLink{{Type: &jira.IssueLinkType{Outward: "relates to"}, OutwardIssue: &jira.Issue{Key: mainKey}}},
		})
		app.infoPanel.SetActiveTab(views.InfoTabLinks)

		_, cmd := app.handleActionOpen()

		if cmd == nil {
			t.Error("expected fetch cmd for linked issue")
		}
		if app.side != sideRight {
			t.Errorf("side = %v, want right", app.side)
		}
	})

	t.Run("detail side is not handled", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideRight

		m, _ := app.handleActionOpen()

		if m != nil {
			t.Error("open on detail side should fall through")
		}
	})
}

func TestOpenIssueDetail_NoSelectionIsNoop(t *testing.T) {
	t.Parallel()
	app := focusApp(t)

	_, cmd := app.openIssueDetail()

	if cmd != nil {
		t.Error("expected nil cmd without selection")
	}
}

func TestNavigateToLinkedIssue_SwitchesToTabContainingIssue(t *testing.T) {
	t.Parallel()
	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: mainKey})
	app := newAppWithFake(t, fake)
	app.keymap = DefaultKeymap()
	app.width = 120
	app.height = 40
	app.layoutPanels()
	app.issuesList.SetTabs([]config.IssueTabConfig{
		{Name: "All", JQL: "project = X"},
		{Name: "Mine", JQL: "assignee = currentUser()"},
	})
	app.issuesList.SetIssuesForTab(0, []jira.Issue{{Key: mainKey}})
	app.issuesList.SetIssuesForTab(1, []jira.Issue{{Key: testKey}})
	app.issuesList.SetTabIndex(1)
	app.infoPanel.SetIssue(&jira.Issue{
		Key:        testKey,
		IssueLinks: []jira.IssueLink{{Type: &jira.IssueLinkType{Outward: "relates to"}, OutwardIssue: &jira.Issue{Key: mainKey}}},
	})
	app.infoPanel.SetActiveTab(views.InfoTabLinks)

	_, _ = app.navigateToLinkedIssue()

	if app.issuesList.GetTabIndex() != 0 {
		t.Errorf("tab index = %d, want 0 (tab containing the linked issue)", app.issuesList.GetTabIndex())
	}
}

func TestHandleActionURLPicker_InternalLinkNavigates(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	issue := &jira.Issue{
		Key:         testKey,
		Description: "see example.atlassian.net/browse/ABC-1 and https://other.example.com/doc",
	}
	app.issuesList.SetIssues([]jira.Issue{*issue, {Key: mainKey}})
	app.previewKey = testKey
	app.issueCache[testKey] = issue

	_, _ = app.handleActionURLPicker()

	if !app.modal.IsVisible() {
		t.Fatal("URL picker modal should be visible")
	}
	if app.onSelect == nil {
		t.Fatal("onSelect should be installed")
	}
	cmd := app.onSelect(components.ModalItem{ID: "example.atlassian.net/browse/" + mainKey, Internal: true})
	if cmd != nil {
		t.Error("internal navigation should not produce a cmd")
	}
}

func TestHandleActionEdit_ConversionFailures(t *testing.T) {
	t.Parallel()

	t.Run("comment ADF conversion error surfaces", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.converter = failingConverter{}
		app.side = sideRight
		issue := &jira.Issue{
			Key:      testKey,
			Comments: []jira.Comment{{ID: "9", BodyADF: map[string]any{"type": "doc"}}},
		}
		app.issuesList.SetIssues([]jira.Issue{*issue})
		app.previewKey = testKey
		app.issueCache[testKey] = issue
		app.detailView.SetIssue(issue)
		app.detailView.SetActiveTab(views.TabComments)

		_, cmd := app.handleActionEdit()

		if cmd != nil {
			t.Error("conversion failure must not launch the editor")
		}
		if app.statusPanel.ErrorMessage() == "" {
			t.Error("conversion failure should surface in status panel")
		}
	})

	t.Run("description ADF conversion error surfaces", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.converter = failingConverter{}
		app.side = sideRight
		issue := &jira.Issue{Key: testKey, DescriptionADF: map[string]any{"type": "doc"}}
		app.issuesList.SetIssues([]jira.Issue{*issue})
		app.previewKey = testKey
		app.issueCache[testKey] = issue
		app.detailView.SetIssue(issue)

		_, cmd := app.handleActionEdit()

		if cmd != nil {
			t.Error("conversion failure must not launch the editor")
		}
		if app.statusPanel.ErrorMessage() == "" {
			t.Error("conversion failure should surface in status panel")
		}
	})

	t.Run("comments tab without selected comment is noop", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideRight
		issue := &jira.Issue{Key: testKey}
		app.previewKey = testKey
		app.issueCache[testKey] = issue
		app.detailView.SetIssue(issue)
		app.detailView.SetActiveTab(views.TabComments)

		_, cmd := app.handleActionEdit()

		if cmd != nil {
			t.Error("expected nil cmd without a selected comment")
		}
	})

	t.Run("comment ADF converts and launches editor", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.converter = BuiltinConverter{}
		app.side = sideRight
		adf := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{},
		}
		issue := &jira.Issue{Key: testKey, Comments: []jira.Comment{{ID: "9", BodyADF: adf}}}
		app.issuesList.SetIssues([]jira.Issue{*issue})
		app.previewKey = testKey
		app.issueCache[testKey] = issue
		app.detailView.SetIssue(issue)
		app.detailView.SetActiveTab(views.TabComments)

		_, cmd := app.handleActionEdit()

		if cmd == nil {
			t.Error("expected editor launch cmd")
		}
		if app.editContext.kind != editCommentMod {
			t.Errorf("editContext kind = %v, want editCommentMod", app.editContext.kind)
		}
	})
}

func gitTestRepo(t *testing.T, branches ...string) string {
	t.Helper()
	dir := t.TempDir()
	commands := make([][]string, 0, 5+len(branches))
	commands = append(commands,
		[]string{"init", "-q", "-b", "main"},
		[]string{"config", "user.email", "test@example.com"},
		[]string{"config", "user.name", "test"},
		[]string{"config", "commit.gpgsign", "false"},
		[]string{"commit", "--allow-empty", "-q", "-m", "init"},
	)
	for _, branch := range branches {
		commands = append(commands, []string{"branch", branch})
	}
	for _, args := range commands {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed: %v (%s)", args, err, output)
		}
	}
	if _, err := exec.CommandContext(t.Context(), "git", "-C", dir, "rev-parse", "--git-dir").CombinedOutput(); err != nil {
		t.Skip("git unavailable in test environment")
	}
	return filepath.Clean(dir)
}

func TestHandleActionCreateBranch_FullTemplate(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.side = sideLeft
	app.leftFocus = focusIssues
	app.gitRepoPath = gitTestRepo(t, testKey+"-existing")
	app.cfg.Git = config.GitConfig{
		AsciiOnly: true,
		BranchFormat: []config.BranchFormatRule{
			{When: config.BranchFormatCondition{Type: "Story"}, Template: "feat/{{.Key}}-{{.Summary}}"},
		},
	}
	issue := &jira.Issue{
		Key:       testKey,
		Summary:   "Add lögin",
		IssueType: &jira.IssueType{Name: "Story"},
		Parent:    &jira.Issue{Key: mainKey},
	}
	app.issuesList.SetIssues([]jira.Issue{*issue})
	app.previewKey = testKey
	app.issueCache[testKey] = issue

	_, _ = app.handleActionCreateBranch()

	if !app.inputModal.IsVisible() {
		t.Fatal("input modal should be visible")
	}
	if app.editContext.kind != editBranch {
		t.Errorf("editContext kind = %v, want editBranch", app.editContext.kind)
	}
	if !app.inputModal.HasHints() {
		t.Error("existing branch with the issue key should populate hints")
	}
}

func TestHandleActionCreateBranch_NoIssueIsNoop(t *testing.T) {
	t.Parallel()
	app := focusApp(t)
	app.side = sideLeft
	app.leftFocus = focusIssues
	app.gitRepoPath = t.TempDir()

	_, cmd := app.handleActionCreateBranch()

	if cmd != nil || app.inputModal.IsVisible() {
		t.Error("expected noop without an issue")
	}
}
