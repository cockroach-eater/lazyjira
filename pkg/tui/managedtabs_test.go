package tui

import (
	"slices"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
)

// visibleTabNames enumerates the issue list's visible tab labels via the public
// tab-navigation API (the underlying slice is unexported in package views).
func visibleTabNames(app *App) []string {
	il := app.issuesList
	start := il.GetTabIndex()
	var names []string
	for {
		names = append(names, il.ActiveTab().Name)
		il.NextTab()
		if il.GetTabIndex() == start {
			break
		}
	}
	return names
}

func TestProjectSwitch_refiltersManaged(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	app.issuesList.SetSavedTabs([]config.ManagedTab{
		{Name: "ABC-Bugs", JQL: "a", Project: testProjectABC},
		{Name: "Global", JQL: "g", Project: ""},
	})

	app.selectProject(&jira.Project{Key: testProjectABC})
	if got := visibleTabNames(app); !slices.Contains(got, "ABC-Bugs") || !slices.Contains(got, "Global") {
		t.Fatalf("under ABC want ABC-Bugs and Global, got %v", got)
	}

	app.selectProject(&jira.Project{Key: "XYZ"})
	got := visibleTabNames(app)
	if slices.Contains(got, "ABC-Bugs") {
		t.Errorf("after switch to XYZ, ABC-Bugs must be hidden, got %v", got)
	}
	if !slices.Contains(got, "Global") {
		t.Errorf("global managed tab must survive project switch, got %v", got)
	}
}

func TestHelpBarItems_managedTabKeysVisible(t *testing.T) {
	t.Parallel()
	descs := func(app *App) []string {
		items := app.helpBarItems()
		out := make([]string, 0, len(items))
		for _, it := range items {
			out = append(out, it.Description)
		}
		return out
	}

	// Transient JQL tab → "save tab"
	jqlApp := appForKeybindings(t)
	jqlApp.side, jqlApp.leftFocus = sideLeft, focusIssues
	jqlApp.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	jqlApp.issuesList.AddJQLTab("project = X")
	if got := descs(jqlApp); !slices.Contains(got, "save tab") {
		t.Errorf("JQL tab help bar missing 'save tab': %v", got)
	}

	// Config tab → "promote tab"
	cfgApp := appForKeybindings(t)
	cfgApp.side, cfgApp.leftFocus = sideLeft, focusIssues
	cfgApp.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	if got := descs(cfgApp); !slices.Contains(got, "promote tab") {
		t.Errorf("config tab help bar missing 'promote tab': %v", got)
	}

	// Managed tab → "delete tab" + "move tab"
	mApp := appForKeybindings(t)
	mApp.side, mApp.leftFocus = sideLeft, focusIssues
	mApp.projectKey = testProjectABC
	mApp.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	mApp.issuesList.SetSavedTabs([]config.ManagedTab{{Name: testManagedName, JQL: "b", Project: testProjectABC}})
	mApp.issuesList.RebuildTabs(testProjectABC)
	mApp.issuesList.SetTabIndex(0) // managed tab
	got := descs(mApp)
	if !slices.Contains(got, "delete tab") || !slices.Contains(got, "move tab") {
		t.Errorf("managed tab help bar missing delete/move: %v", got)
	}
	if !slices.Contains(got, "edit query") {
		t.Errorf("managed tab help bar must show 'edit query' (discoverable s): %v", got)
	}
	if slices.Contains(got, "promote tab") {
		t.Errorf("managed tab must not show 'promote tab': %v", got)
	}
}

func TestTabFetch_staleEpoch_dropped(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.leftFocus = focusProjects // skip preview side effects in the active-tab branch
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})

	staleEpoch := app.issuesList.TabEpoch()
	app.issuesList.RebuildTabs(testProjectABC) // a save/delete-style reassembly bumps the epoch
	freshEpoch := app.issuesList.TabEpoch()
	if freshEpoch == staleEpoch {
		t.Fatal("RebuildTabs must bump the tab epoch")
	}

	app.handleIssuesLoaded(issuesLoadedMsg{tab: 0, issues: []jira.Issue{{Key: "PLAT-1"}}, epoch: staleEpoch})
	if app.issuesList.HasCachedTab() {
		t.Error("stale-epoch fetch result must be dropped, not stored")
	}

	app.handleIssuesLoaded(issuesLoadedMsg{tab: 0, issues: []jira.Issue{{Key: "PLAT-2"}}, epoch: freshEpoch})
	if !app.issuesList.HasCachedTab() {
		t.Error("current-epoch fetch result must be applied")
	}
}
