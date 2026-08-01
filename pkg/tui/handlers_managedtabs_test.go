package tui

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
)

// newSaveTabApp builds an app focused on a transient JQL tab in project ABC.
func newSaveTabApp(t *testing.T, jql string) *App {
	t.Helper()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	app.issuesList.SetSavedTabs(nil)
	app.savedTabs = nil
	app.issuesList.RebuildTabs(testProjectABC)
	app.issuesList.AddJQLTab(jql)
	return app
}

func TestSaveTab_appendsAndPersists(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newSaveTabApp(t, "project = ABC AND status = Open")

	if _, _, ok := app.handleTabAction(ActManageTab); !ok {
		t.Fatal("ActManageTab not dispatched")
	}
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: "My Bugs"})

	saved := config.LoadSavedTabs()
	if len(saved) != 1 {
		t.Fatalf("LoadSavedTabs len = %d, want 1: %+v", len(saved), saved)
	}
	got := saved[0]
	if got.Name != "My Bugs" || got.JQL != "project = ABC AND status = Open" || got.Project != testProjectABC {
		t.Errorf("persisted tab = %+v", got)
	}
}

func TestSaveTab_emptyJQL_rejected(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newSaveTabApp(t, "")

	app.handleTabAction(ActManageTab)
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: "Empty"})

	if len(app.savedTabs) != 0 {
		t.Errorf("empty-JQL tab must be rejected, savedTabs = %+v", app.savedTabs)
	}
	if got := config.LoadSavedTabs(); len(got) != 0 {
		t.Errorf("nothing should persist, got %+v", got)
	}
}

func TestSaveTab_appearsAsManagedTab(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newSaveTabApp(t, "project = ABC")

	app.handleTabAction(ActManageTab)
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: "Saved"})

	if !app.issuesList.IsManagedTab() || app.issuesList.ActiveTab().Name != "Saved" {
		t.Errorf("after save, active tab should be managed Saved, got managed=%v name=%q",
			app.issuesList.IsManagedTab(), app.issuesList.ActiveTab().Name)
	}
	if names := visibleTabNames(app); !slices.Contains(names, "Saved") {
		t.Errorf("Saved must appear in tab bar, got %v", names)
	}
}

func TestSaveTab_duplicateName_rejectedIdempotent(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newSaveTabApp(t, "project = ABC")
	app.savedTabs = []config.ManagedTab{{Name: "Dup", JQL: "old", Project: testProjectABC}}
	app.issuesList.SetSavedTabs(app.savedTabs)
	app.issuesList.AddJQLTab("project = ABC")

	app.handleTabAction(ActManageTab)
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: "Dup"})

	if len(app.savedTabs) != 1 {
		t.Errorf("duplicate name must not be appended (idempotent), savedTabs = %+v", app.savedTabs)
	}
	if app.helpBar.StatusMsg() == "" {
		t.Error("duplicate name must surface a warning")
	}
}

func TestSaveTab_persistError_surfaced(t *testing.T) {
	// A config dir that cannot be created (a file stands where the dir parent
	// must be) forces SaveSavedTabs to fail.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LAZYJIRA_CONFIG_DIR", filepath.Join(blocker, "nested"))
	app := newSaveTabApp(t, "project = ABC")

	app.handleTabAction(ActManageTab)
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: "Boom"})

	if app.helpBar.StatusMsg() == "" {
		t.Error("persist error must be surfaced via help bar")
	}
}

func TestManageTab_dispatchedNotFallthrough(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})

	// On a config tab (not JQL) the action must still be handled, never
	// forwarded to the panel as a raw keypress.
	if _, _, ok := app.handleTabAction(ActManageTab); !ok {
		t.Error("ActManageTab must be dispatched even on a non-JQL tab")
	}
}

func TestDeleteManagedTab_removesFromStore(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	app.savedTabs = []config.ManagedTab{{Name: testManagedName, JQL: "b", Project: testProjectABC}}
	app.issuesList.SetSavedTabs(app.savedTabs)
	app.issuesList.RebuildTabs(testProjectABC)
	app.issuesList.SetTabIndex(0) // managed Bugs

	if _, _, ok := app.handleTabAction(ActDeleteManagedTab); !ok {
		t.Fatal("ActDeleteManagedTab not dispatched")
	}
	app.handleModalSelected(components.ModalSelectedMsg{Item: components.ModalItem{ID: "yes"}})

	if len(app.savedTabs) != 0 {
		t.Errorf("deleted tab must be removed, savedTabs = %+v", app.savedTabs)
	}
	if got := config.LoadSavedTabs(); len(got) != 0 {
		t.Errorf("removal must persist, got %+v", got)
	}
}

func TestDeleteManagedTab_unPromoteRevealsConfig(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: testManagedName, JQL: "cfg"}})
	app.savedTabs = []config.ManagedTab{{Name: testManagedName, JQL: "managed", Project: testProjectABC}}
	app.issuesList.SetSavedTabs(app.savedTabs)
	app.issuesList.RebuildTabs(testProjectABC)
	app.issuesList.SetTabIndex(0) // managed Bugs shadows config Bugs

	app.handleTabAction(ActDeleteManagedTab)
	app.handleModalSelected(components.ModalSelectedMsg{Item: components.ModalItem{ID: "yes"}})

	if !slices.Contains(visibleTabNames(app), testManagedName) {
		t.Errorf("config Bugs must reappear after un-promote, got %v", visibleTabNames(app))
	}
	if app.issuesList.IsManagedTab() && app.issuesList.ActiveTab().Name == testManagedName {
		t.Error("Bugs should now be the config tab, not managed")
	}
}

func TestDeleteManagedTab_noopOnConfigTab(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	app.savedTabs = []config.ManagedTab{{Name: testManagedName, JQL: "b", Project: testProjectABC}}
	app.issuesList.SetSavedTabs(app.savedTabs)
	app.issuesList.RebuildTabs(testProjectABC)
	app.issuesList.SetTabIndex(app.issuesList.HomeTabIndex()) // config tab

	if _, _, ok := app.handleTabAction(ActDeleteManagedTab); !ok {
		t.Error("ActDeleteManagedTab must be dispatched (handled), even on config tab")
	}
	// No confirm modal callback should have been registered.
	if app.onSelect != nil {
		t.Error("no delete confirmation expected on a config tab")
	}
	if len(app.savedTabs) != 1 {
		t.Errorf("config-tab delete must be a no-op, savedTabs = %+v", app.savedTabs)
	}
}

// At startup the issue list must be assembled for the pre-selected project, so
// a project-bound managed tab is visible and shadows its config namesake. The
// original bug left issuesList.projectKey == "" while a.projectKey was the
// first project: the config tab stayed visible/promotable and the dup-check
// (which uses a.projectKey) then warned on promote.
func TestStartup_managedTabShadowsConfigForInitialProject(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	if err := config.SaveSavedTabs([]config.ManagedTab{
		{Name: "MyTab", JQL: "project = ABC", Project: testProjectABC},
	}); err != nil {
		t.Fatal(err)
	}
	fake := &jiratest.FakeClient{T: t}
	fake.SetOnRequestFunc = func(_ func(jira.RequestLog)) {}
	fake.DiscoverFieldsFunc = func(_ context.Context) error { return nil }
	cfg := &config.Config{
		Jira:      config.JiraConfig{Host: "example.atlassian.net", Email: "t@example.com"},
		Projects:  []config.ProjectConfig{{Key: testProjectABC}},
		IssueTabs: []config.IssueTabConfig{{Name: "MyTab", JQL: "project = ABC"}},
	}

	app := NewApp(cfg, fake)

	names := visibleTabNames(app)
	if len(names) != 1 || names[0] != "MyTab" {
		t.Fatalf("visible tabs = %v, want exactly one MyTab (config shadowed)", names)
	}
	if !app.issuesList.IsManagedTab() {
		t.Errorf("startup MyTab should be the managed tab, not the config one")
	}
}

func TestPromote_thenManagedAndEditable(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	fake := &jiratest.FakeClient{T: t}
	fake.SetOnRequestFunc = func(_ func(jira.RequestLog)) {}
	fake.DiscoverFieldsFunc = func(_ context.Context) error { return nil }
	cfg := &config.Config{
		Jira:      config.JiraConfig{Host: "h", Email: "e"},
		Projects:  []config.ProjectConfig{{Key: testProjectABC}},
		IssueTabs: []config.IssueTabConfig{{Name: "MyTab", JQL: "project = ABC AND status = Open"}},
	}
	app := NewApp(cfg, fake)
	app.keymap = DefaultKeymap()

	if app.issuesList.IsManagedTab() {
		t.Fatal("setup: startup tab should be config")
	}

	app.handleTabAction(ActManageTab)
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: "MyTab"})

	if app.helpBar.StatusMsg() != "" {
		t.Errorf("promote must not warn, got %q", app.helpBar.StatusMsg())
	}
	if !app.issuesList.IsManagedTab() {
		t.Errorf("after promote, active tab should be managed; visible=%v", visibleTabNames(app))
	}

	app.handleTabAction(ActJQLSearch)
	if got := app.jqlModal.InputValue(); got != "project = ABC AND status = Open" {
		t.Errorf("s prefill = %q, want the managed tab's query", got)
	}
}

// Real-world startup: cfg.Projects is empty, so the project is learned from the
// server via handleProjectsLoaded. That handler must sync the tab list's
// project filter, otherwise managed tabs stay hidden and their config namesakes
// remain visible/promotable (the desync behind the recurring promote warning).
func TestProjectsLoaded_syncsManagedTabFilter(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	// projectKey is "" (no project known yet); the list assembled for "".
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "MyTab", JQL: "project = ABC"}})
	app.savedTabs = []config.ManagedTab{{Name: "MyTab", JQL: "project = ABC", Project: testProjectABC}}
	app.issuesList.SetSavedTabs(app.savedTabs)
	if app.issuesList.IsManagedTab() {
		t.Fatal("precondition: managed MyTab must be hidden while projectKey is empty")
	}

	app.handleProjectsLoaded(projectsLoadedMsg{projects: []jira.Project{{Key: testProjectABC}}})

	if app.projectKey != testProjectABC {
		t.Fatalf("projectKey = %q, want %q", app.projectKey, testProjectABC)
	}
	names := visibleTabNames(app)
	count := 0
	for _, n := range names {
		if n == "MyTab" {
			count++
		}
	}
	if count != 1 || !app.issuesList.IsManagedTab() {
		t.Errorf("managed MyTab must shadow config MyTab after projects load: visible=%v managed=%v",
			names, app.issuesList.IsManagedTab())
	}
}

func TestDeleteManagedTab_dispatchedNotFallthrough(t *testing.T) {
	t.Parallel()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})

	if _, _, ok := app.handleTabAction(ActDeleteManagedTab); !ok {
		t.Error("ActDeleteManagedTab must be dispatched, not fall through")
	}
}

func TestPromote_copiesConfigToStore(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	mr := 25
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: testManagedName, JQL: "type = Bug", MaxResults: &mr}})
	app.issuesList.SetSavedTabs(nil)
	app.savedTabs = nil
	app.issuesList.RebuildTabs(testProjectABC)
	app.issuesList.SetTabIndex(app.issuesList.HomeTabIndex())

	if _, _, ok := app.handleTabAction(ActManageTab); !ok {
		t.Fatal("ActManageTab not dispatched on config tab")
	}
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: testManagedName})

	saved := config.LoadSavedTabs()
	if len(saved) != 1 {
		t.Fatalf("LoadSavedTabs len = %d, want 1: %+v", len(saved), saved)
	}
	got := saved[0]
	if got.Name != testManagedName || got.JQL != "type = Bug" || got.Project != testProjectABC {
		t.Errorf("promoted tab = %+v", got)
	}
	if got.MaxResults == nil || *got.MaxResults != 25 {
		t.Errorf("MaxResults not copied: %v", got.MaxResults)
	}
}

func TestPromote_shadowsOriginal(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = testProjectABC
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: testManagedName, JQL: "cfg"}})
	app.issuesList.SetSavedTabs(nil)
	app.savedTabs = nil
	app.issuesList.RebuildTabs(testProjectABC)
	app.issuesList.SetTabIndex(app.issuesList.HomeTabIndex())

	app.handleTabAction(ActManageTab)
	app.handleInputConfirmed(components.InputConfirmedMsg{Text: testManagedName})

	if !app.issuesList.IsManagedTab() || app.issuesList.ActiveTab().Name != testManagedName {
		t.Errorf("after promote, active Bugs must be managed, got managed=%v name=%q",
			app.issuesList.IsManagedTab(), app.issuesList.ActiveTab().Name)
	}
	count := 0
	for _, n := range visibleTabNames(app) {
		if n == testManagedName {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Bugs must appear once after promote (config original shadowed), got %d in %v",
			count, visibleTabNames(app))
	}
}

// newReorderApp builds an app with the given managed store, focused on the
// issues list of the given project.
func newReorderApp(t *testing.T, tabs []config.ManagedTab, project string) *App {
	t.Helper()
	app := newAppWithFake(t, &jiratest.FakeClient{T: t})
	app.demoMode = true
	app.projectKey = project
	app.issuesList.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	app.savedTabs = tabs
	app.issuesList.SetSavedTabs(tabs)
	app.issuesList.RebuildTabs(project)
	return app
}

func TestReorder_swapsManagedOrder(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newReorderApp(t, []config.ManagedTab{
		{Name: "A", JQL: "a", Project: testProjectABC},
		{Name: "B", JQL: "b", Project: testProjectABC},
	}, testProjectABC)
	app.issuesList.SetTabIndex(0) // managed A

	if _, _, ok := app.handleTabAction(ActReorderTabRight); !ok {
		t.Fatal("ActReorderTabRight not dispatched")
	}

	if app.savedTabs[0].Name != "B" || app.savedTabs[1].Name != "A" {
		t.Errorf("after move-right, store order = %+v", app.savedTabs)
	}
	if got := config.LoadSavedTabs(); len(got) != 2 || got[0].Name != "B" || got[1].Name != "A" {
		t.Errorf("reorder must persist, got %+v", got)
	}
	if app.issuesList.ActiveTab().Name != "A" {
		t.Errorf("active tab must follow the moved entry, got %q", app.issuesList.ActiveTab().Name)
	}
}

func TestReorder_respectsProjectFilter(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newReorderApp(t, []config.ManagedTab{
		{Name: "Foreign", JQL: "f", Project: "XYZ"},    // store 0, hidden under ABC
		{Name: "A", JQL: "a", Project: testProjectABC}, // store 1, visible pos 0
		{Name: "B", JQL: "b", Project: testProjectABC}, // store 2, visible pos 1
	}, testProjectABC)
	app.issuesList.SetTabIndex(0) // managed A
	if !app.issuesList.IsManagedTab() || app.issuesList.ActiveTab().Name != "A" {
		t.Fatalf("setup: want active managed A, got %q", app.issuesList.ActiveTab().Name)
	}

	app.handleTabAction(ActReorderTabRight)

	if app.savedTabs[0].Name != "Foreign" {
		t.Errorf("foreign-project entry must be untouched, store = %+v", app.savedTabs)
	}
	if app.savedTabs[1].Name != "B" || app.savedTabs[2].Name != "A" {
		t.Errorf("only visible managed entries swapped, store = %+v", app.savedTabs)
	}
}

func TestReorder_noopAtEdges(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())
	app := newReorderApp(t, []config.ManagedTab{
		{Name: "A", JQL: "a", Project: testProjectABC},
		{Name: "B", JQL: "b", Project: testProjectABC},
	}, testProjectABC)
	app.issuesList.SetTabIndex(0) // managed A at the left edge

	if _, _, ok := app.handleTabAction(ActReorderTabLeft); !ok {
		t.Fatal("ActReorderTabLeft not dispatched")
	}

	if app.savedTabs[0].Name != "A" || app.savedTabs[1].Name != "B" {
		t.Errorf("move-left at left edge must be a no-op, store = %+v", app.savedTabs)
	}
}
