package views

import (
	"slices"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
)

func TestSetTabs_assemblesImmediately(t *testing.T) {
	t.Parallel()
	cfgs := []config.IssueTabConfig{
		{Name: "All", JQL: "x"},
		{Name: "Mine", JQL: "y"},
	}
	m := NewIssuesList()
	m.SetTabs(cfgs)

	if len(m.tabs) != 2 {
		t.Fatalf("SetTabs must assemble immediately: len(tabs) = %d, want 2", len(m.tabs))
	}
	if got := m.ActiveTab(); got != cfgs[0] {
		t.Errorf("ActiveTab() = %+v, want %+v", got, cfgs[0])
	}
}

func TestRebuildTabs_managedBeforeConfig(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{{Name: "Bugs", JQL: "type=Bug", Project: "ABC"}})

	m.RebuildTabs("ABC")

	if len(m.tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2", len(m.tabs))
	}
	if m.tabs[0].kind != tabKindManaged || m.tabs[0].cfg.Name != "Bugs" {
		t.Errorf("tab 0 = %+v, want managed Bugs first", m.tabs[0])
	}
	if m.tabs[1].kind != tabKindConfig || m.tabs[1].cfg.Name != "All" {
		t.Errorf("tab 1 = %+v, want config All", m.tabs[1])
	}
}

func TestRebuildTabs_filtersByProject(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs(nil)
	m.SetSavedTabs([]config.ManagedTab{
		{Name: "ABC-Bugs", JQL: "a", Project: "ABC"},
		{Name: "XYZ-Bugs", JQL: "b", Project: "XYZ"},
		{Name: "Global", JQL: "c", Project: ""},
	})

	m.RebuildTabs("ABC")

	names := tabNames(m)
	if !slices.Contains(names, "ABC-Bugs") || !slices.Contains(names, "Global") {
		t.Errorf("visible tabs = %v, want ABC-Bugs and Global", names)
	}
	if slices.Contains(names, "XYZ-Bugs") {
		t.Errorf("visible tabs = %v, want XYZ-Bugs hidden under project ABC", names)
	}
}

func TestRebuildTabs_shadowsConfigByName(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "Bugs", JQL: "cfg"}, {Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{{Name: "Bugs", JQL: "managed", Project: "ABC"}})

	m.RebuildTabs("ABC")

	bugsCount := 0
	for _, tb := range m.tabs {
		if tb.cfg.Name == "Bugs" {
			bugsCount++
			if tb.kind != tabKindManaged {
				t.Errorf("visible Bugs tab kind = %d, want managed (config shadowed)", tb.kind)
			}
		}
	}
	if bugsCount != 1 {
		t.Errorf("Bugs tab count = %d, want 1 (config shadowed by managed)", bugsCount)
	}
}

func TestRebuildTabs_setsStoreIdx(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs(nil)
	m.SetSavedTabs([]config.ManagedTab{
		{Name: "XYZ-only", JQL: "a", Project: "XYZ"}, // store index 0, filtered out under ABC
		{Name: "ABC-Bugs", JQL: "b", Project: "ABC"}, // store index 1, visible
	})

	m.RebuildTabs("ABC")

	if len(m.tabs) != 1 {
		t.Fatalf("len(tabs) = %d, want 1 visible", len(m.tabs))
	}
	if m.tabs[0].storeIdx != 1 {
		t.Errorf("storeIdx = %d, want 1 (position in store, not visible slice)", m.tabs[0].storeIdx)
	}
}

func TestVisibleManagedStoreIndices(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{
		{Name: "XYZ-only", JQL: "a", Project: "XYZ"}, // store 0, filtered out under ABC
		{Name: "ABC-Bugs", JQL: "b", Project: "ABC"}, // store 1, visible
		{Name: "Global", JQL: "g", Project: ""},      // store 2, visible
	})
	m.RebuildTabs("ABC")

	if got, want := m.VisibleManagedStoreIndices(), []int{1, 2}; !slices.Equal(got, want) {
		t.Errorf("VisibleManagedStoreIndices() = %v, want %v", got, want)
	}
}

func TestHomeTabIndex_skipsManaged(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{{Name: "Bugs", JQL: "b", Project: "ABC"}})
	m.RebuildTabs("ABC")

	if got := m.HomeTabIndex(); got != 1 {
		t.Errorf("HomeTabIndex() = %d, want 1 (first config tab, managed at 0)", got)
	}
}

func TestInjectIssue_targetsHomeNotManaged(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{{Name: "Bugs", JQL: "b", Project: "ABC"}})
	m.RebuildTabs("ABC")

	m.InjectIssue(jira.Issue{Key: "PLAT-1"})

	m.SetTabIndex(m.HomeTabIndex())
	if got := len(m.CurrentIssues()); got != 1 {
		t.Errorf("home tab CurrentIssues = %d, want 1 (issue injected here)", got)
	}
	m.SetTabIndex(0) // managed tab
	if got := len(m.CurrentIssues()); got != 0 {
		t.Errorf("managed tab CurrentIssues = %d, want 0 (must not receive injection)", got)
	}
}

// homeAndManaged builds a bar of one managed tab (index 0) and one config tab
// (index 1), so "go home" and "go to index 0" are distinguishable.
func homeAndManaged(t *testing.T) *IssuesList {
	t.Helper()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{{Name: "Bugs", JQL: "b", Project: "ABC"}})
	m.RebuildTabs("ABC")
	return m
}

func TestRemoveHierarchyTab_returnsToHomeNotManaged(t *testing.T) {
	t.Parallel()
	m := homeAndManaged(t)
	m.AddHierarchyTab("Children", []jira.Issue{{Key: "CHILD-1"}})

	m.RemoveHierarchyTab()

	if got := m.GetTabIndex(); got != m.HomeTabIndex() {
		t.Errorf("tab after close = %d, want %d (home, not the managed tab at 0)", got, m.HomeTabIndex())
	}
}

func TestInvalidateTabCache_clampsToHomeNotManaged(t *testing.T) {
	t.Parallel()
	m := homeAndManaged(t)
	m.AddJQLTab("project = ABC") // transient tab, becomes active

	m.InvalidateTabCache() // drops it, leaving m.tab out of range

	if got := m.GetTabIndex(); got != m.HomeTabIndex() {
		t.Errorf("tab after invalidate = %d, want %d (home, not the managed tab at 0)", got, m.HomeTabIndex())
	}
}

func TestRebuildTabs_clampsToHomeNotManaged(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{
		{Name: "ABC-Bugs", JQL: "b", Project: "ABC"}, // visible under ABC only
		{Name: "Global", JQL: "g", Project: ""},      // visible everywhere
	})
	m.RebuildTabs("ABC")
	m.SetTabIndex(2) // the config tab, last under ABC

	m.RebuildTabs("XYZ") // one managed tab fewer, so index 2 no longer exists

	if got := m.GetTabIndex(); got != m.HomeTabIndex() {
		t.Errorf("tab after project switch = %d, want %d (home, not the managed tab at 0)", got, m.HomeTabIndex())
	}
}

func tabNames(m *IssuesList) []string {
	names := make([]string, len(m.tabs))
	for i, tb := range m.tabs {
		names[i] = tb.cfg.Name
	}
	return names
}
