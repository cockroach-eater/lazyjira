package views

import (
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
)

func TestSetTabs_wrapsConfigAsKindConfig(t *testing.T) {
	t.Parallel()
	cfgs := []config.IssueTabConfig{
		{Name: "All", JQL: "x"},
		{Name: "Mine", JQL: "y"},
	}
	m := NewIssuesList()
	m.SetTabs(cfgs)

	if len(m.tabs) != len(cfgs) {
		t.Fatalf("len(tabs) = %d, want %d", len(m.tabs), len(cfgs))
	}
	for i, tb := range m.tabs {
		if tb.kind != tabKindConfig {
			t.Errorf("tab %d kind = %d, want tabKindConfig", i, tb.kind)
		}
		if tb.storeIdx != -1 {
			t.Errorf("tab %d storeIdx = %d, want -1 (not managed)", i, tb.storeIdx)
		}
	}
	if got := m.ActiveTab(); got != cfgs[0] {
		t.Errorf("ActiveTab() = %+v, want %+v", got, cfgs[0])
	}
}

func TestTransientTabs_addRemoveByKind(t *testing.T) {
	t.Parallel()
	m := listWithTabs(config.IssueTabConfig{Name: "All", JQL: "x"})

	m.AddJQLTab("project = FOO")
	if !m.IsJQLTab() {
		t.Fatal("active tab should be the JQL tab after AddJQLTab")
	}
	m.AddHierarchyTab("Child", []jira.Issue{{Key: "C-1"}})
	if !m.IsHierarchyTab() {
		t.Fatal("active tab should be the hierarchy tab after AddHierarchyTab")
	}
	if !m.HasJQLTab() {
		t.Fatal("JQL tab should coexist with hierarchy tab")
	}
	if len(m.tabs) != 3 {
		t.Fatalf("len(tabs) = %d, want 3 (config + JQL + hierarchy)", len(m.tabs))
	}

	m.InvalidateTabCache()

	if m.HasJQLTab() || m.HasHierarchyTab() {
		t.Error("InvalidateTabCache should drop transient tabs")
	}
	if len(m.tabs) != 1 || m.tabs[0].kind != tabKindConfig {
		t.Errorf("config tab should survive: tabs=%d", len(m.tabs))
	}
}

func TestRemoveJQLTab_returnsToOrigin(t *testing.T) {
	t.Parallel()
	m := listWithTabs(
		config.IssueTabConfig{Name: "All", JQL: "x"},
		config.IssueTabConfig{Name: "Mine", JQL: "y"},
	)
	m.SetTabIndex(1) // user is on "Mine" when opening JQL search

	m.AddJQLTab("project = FOO")
	if !m.IsJQLTab() {
		t.Fatal("expected JQL tab active after AddJQLTab")
	}

	m.RemoveJQLTab()
	if got := m.GetTabIndex(); got != 1 {
		t.Errorf("after closing JQL tab, GetTabIndex() = %d, want 1 (origin tab)", got)
	}
}

func TestTabCache_loadedDistinctFromEmpty(t *testing.T) {
	t.Parallel()
	m := listWithTabs(config.IssueTabConfig{Name: "All", JQL: "x"})

	if m.HasCachedTab() {
		t.Error("HasCachedTab() = true before any load, want false")
	}

	m.SetIssues([]jira.Issue{}) // loaded, but zero hits
	if !m.HasCachedTab() {
		t.Error("HasCachedTab() = false after SetIssues with empty slice, want true (loaded != empty)")
	}

	m.SetIssues([]jira.Issue{{Key: "PLAT-1", Summary: "old"}})
	m.PatchIssue(&jira.Issue{Key: "PLAT-1", Summary: "new"})
	if got := m.CurrentIssues()[0].Summary; got != "new" {
		t.Errorf("PatchIssue summary = %q, want new", got)
	}
}

func TestSetIssuesForTab_staleIndex_noop(t *testing.T) {
	t.Parallel()
	m := listWithTabs(config.IssueTabConfig{Name: "All", JQL: "x"})

	// Index beyond len(tabs) must not panic (stale async fetch after rebuild).
	m.SetIssuesForTab(99, []jira.Issue{{Key: "PLAT-9"}})

	if _, ok := m.FindInAnyTab("PLAT-9"); ok {
		t.Error("issues from an out-of-bounds tab index must not be stored")
	}
}
