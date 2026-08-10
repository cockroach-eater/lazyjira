package tui

import (
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

func appForKeybindings(t *testing.T) *App {
	t.Helper()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.keymap = DefaultKeymap()
	app.width = 120
	app.height = 40
	return app
}

func TestContextBindings_ContainsQuit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(*App)
	}{
		{
			name:  "issues focus",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusIssues },
		},
		{
			name:  "info focus",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusInfo },
		},
		{
			name:  "projects focus",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusProjects },
		},
		{
			name:  "status focus",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusStatus },
		},
		{
			name:  "detail side",
			setup: func(app *App) { app.side = sideRight },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			app := appForKeybindings(t)
			testCase.setup(app)
			bindings := app.ContextBindings()
			if len(bindings) == 0 {
				t.Fatal("expected non-empty bindings")
			}
			found := false
			for _, binding := range bindings {
				if binding.Description == string(ActQuit) {
					found = true
					break
				}
			}
			if !found {
				t.Error("quit binding missing from context bindings")
			}
		})
	}
}

func TestContextBindings_DetailCommentsIncludesEdit(t *testing.T) {
	t.Parallel()
	app := appForKeybindings(t)
	app.side = sideRight
	app.detailView.SetIssue(&jira.Issue{Key: testKey})
	app.detailView.SetActiveTab(views.TabComments)

	bindings := app.ContextBindings()

	found := false
	for _, binding := range bindings {
		if binding.Description == "edit comment" {
			found = true
			break
		}
	}
	if !found {
		t.Error("edit comment binding missing when on comments tab")
	}
}

func TestContextBindings_IssuesIncludesCopyKey(t *testing.T) {
	t.Parallel()
	app := appForKeybindings(t)
	app.side = sideLeft
	app.leftFocus = focusIssues

	bindings := app.ContextBindings()

	found := false
	for _, binding := range bindings {
		if binding.Description == "copy issue key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("copy issue key binding missing from issues context")
	}
}

func TestHelpBarItems_NotEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(*App)
	}{
		{
			name:  "issues panel",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusIssues },
		},
		{
			name:  "info panel",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusInfo },
		},
		{
			name:  "projects panel",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusProjects },
		},
		{
			name:  "status panel",
			setup: func(app *App) { app.side = sideLeft; app.leftFocus = focusStatus },
		},
		{
			name:  "detail right panel",
			setup: func(app *App) { app.side = sideRight },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			app := appForKeybindings(t)
			testCase.setup(app)
			items := app.helpBarItems()
			if len(items) == 0 {
				t.Error("expected non-empty help bar items")
			}
		})
	}
}

func TestNavBindings_HasSixEntries(t *testing.T) {
	t.Parallel()
	app := appForKeybindings(t)
	bindings := app.navBindings()
	if len(bindings) != 6 {
		t.Errorf("navBindings len = %d, want 6", len(bindings))
	}
}

func TestDetailScrollBindings_HasFourEntries(t *testing.T) {
	t.Parallel()
	app := appForKeybindings(t)
	bindings := app.detailScrollBindings()
	if len(bindings) != 4 {
		t.Errorf("detailScrollBindings len = %d, want 4", len(bindings))
	}
}

func TestBind_ReturnsBindingWithDescription(t *testing.T) {
	t.Parallel()
	app := appForKeybindings(t)
	b := app.bind(ActQuit, "quit the app")
	if b.Description != "quit the app" {
		t.Errorf("description = %q, want %q", b.Description, "quit the app")
	}
	if b.Key == "" {
		t.Error("key should not be empty for ActQuit")
	}
}
