package tui

import (
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
)

func focusApp(t *testing.T) *App {
	t.Helper()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.keymap = DefaultKeymap()
	app.width = 120
	app.height = 40
	return app
}

func TestHandleFocusAction(t *testing.T) {
	t.Parallel()

	t.Run("switch panel toggles side", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideLeft

		_, _, ok := app.handleFocusAction(ActSwitchPanel)

		if !ok || app.side != sideRight {
			t.Errorf("ok=%v side=%v, want ok side=right", ok, app.side)
		}
	})

	t.Run("focus detail jumps right", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		_, _, ok := app.handleFocusAction(ActFocusDetail)
		if !ok || app.side != sideRight {
			t.Errorf("ok=%v side=%v, want right", ok, app.side)
		}
	})

	t.Run("direct panel focus actions", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			action Action
			want   focusPanel
		}{
			{ActFocusStatus, focusStatus},
			{ActFocusIssues, focusIssues},
			{ActFocusInfo, focusInfo},
			{ActFocusProj, focusProjects},
		}
		for _, testCase := range cases {
			app := focusApp(t)
			app.side = sideRight
			_, _, ok := app.handleFocusAction(testCase.action)
			if !ok || app.side != sideLeft || app.leftFocus != testCase.want {
				t.Errorf("action %v -> ok=%v side=%v focus=%v, want left/%v", testCase.action, ok, app.side, app.leftFocus, testCase.want)
			}
		}
	})

	t.Run("non focus action is not handled", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		if _, _, ok := app.handleFocusAction(ActQuit); ok {
			t.Error("ActQuit should not be a focus action")
		}
	})
}

func TestHandleTabAction(t *testing.T) {
	t.Parallel()

	twoTabs := func(app *App) {
		app.projectKey = testProject
		app.issuesList.SetTabs([]config.IssueTabConfig{
			{Name: "All", JQL: "project = {{.ProjectKey}}"},
			{Name: "Mine", JQL: "assignee = currentUser()"},
		})
	}

	t.Run("next tab on issues advances", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		twoTabs(app)
		app.side = sideLeft
		app.leftFocus = focusIssues

		_, cmd, ok := app.handleTabAction(ActNextTab)

		if !ok {
			t.Fatal("ActNextTab should be handled")
		}
		if app.issuesList.GetTabIndex() != 1 {
			t.Errorf("tab index = %d, want 1", app.issuesList.GetTabIndex())
		}
		if cmd == nil {
			t.Error("expected fetch command for uncached tab")
		}
	})

	t.Run("prev tab on issues wraps", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		twoTabs(app)
		app.side = sideLeft
		app.leftFocus = focusIssues

		_, _, _ = app.handleTabAction(ActPrevTab)

		if app.issuesList.GetTabIndex() != 1 {
			t.Errorf("tab index = %d, want 1 after wrapping back", app.issuesList.GetTabIndex())
		}
	})

	t.Run("next tab on detail switches detail tab", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		app.side = sideRight

		_, _, ok := app.handleTabAction(ActNextTab)

		if !ok {
			t.Error("ActNextTab should be handled on the detail side")
		}
	})

	t.Run("close jql tab removes it", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		twoTabs(app)
		app.side = sideLeft
		app.leftFocus = focusIssues
		app.issuesList.AddJQLTab("project = X")

		_, _, ok := app.handleTabAction(ActCloseJQLTab)

		if !ok {
			t.Fatal("ActCloseJQLTab should be handled")
		}
		if app.issuesList.HasJQLTab() {
			t.Error("JQL tab should be removed")
		}
	})

	t.Run("non tab action is not handled", func(t *testing.T) {
		t.Parallel()
		app := focusApp(t)
		if _, _, ok := app.handleTabAction(ActQuit); ok {
			t.Error("ActQuit should not be a tab action")
		}
	})
}

func TestHandleIssueAction_Comments(t *testing.T) {
	t.Parallel()
	fake := &jiratest.FakeClient{T: t}
	app := newAppWithFake(t, fake)
	app.keymap = DefaultKeymap()
	app.width = 120
	app.height = 40
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
	app.previewKey = testKey

	_, cmd, ok := app.handleIssueAction(ActComments)

	if !ok {
		t.Fatal("ActComments should be handled")
	}
	if app.side != sideRight {
		t.Error("comments action should switch to detail side")
	}
	if cmd == nil {
		t.Error("expected fetch command for uncached issue")
	}
}

func TestHandleIssueAction_CopyKey(t *testing.T) {
	t.Parallel()
	var captured string
	original := runExternalCommand
	t.Cleanup(func() { runExternalCommand = original })
	runExternalCommand = func(input string, waitForExit bool, name string, args ...string) {
		captured = input
	}

	app := focusApp(t)
	app.issuesList.SetIssues([]jira.Issue{{Key: testKey}})
	app.previewKey = testKey
	app.issueCache[testKey] = &jira.Issue{Key: testKey}

	_, _, ok := app.handleIssueAction(ActCopyKey)

	if !ok {
		t.Fatal("ActCopyKey should be handled")
	}
	testkit.AssertEqual(t, "clipboard payload", captured, testKey)
	if !strings.Contains(app.helpBar.View(), testKey) {
		t.Errorf("help bar should show status message containing %q, got %q", testKey, app.helpBar.View())
	}
}

func TestHandleIssueAction_CopyKey_NoIssueIsNoop(t *testing.T) {
	t.Parallel()
	var called bool
	original := runExternalCommand
	t.Cleanup(func() { runExternalCommand = original })
	runExternalCommand = func(input string, waitForExit bool, name string, args ...string) {
		called = true
	}

	app := focusApp(t)

	_, _, ok := app.handleIssueAction(ActCopyKey)

	if !ok {
		t.Fatal("ActCopyKey should report handled even with no issue")
	}
	if called {
		t.Error("clipboard should not be touched when no issue is selected")
	}
}
