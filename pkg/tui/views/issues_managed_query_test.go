package views

import (
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
)

func TestIsManagedTab(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{{Name: "Bugs", JQL: "b", Project: "ABC"}})
	m.RebuildTabs("ABC")

	m.SetTabIndex(0) // managed
	if !m.IsManagedTab() {
		t.Error("tab 0 is managed, IsManagedTab() = false")
	}
	if m.IsConfigTab() {
		t.Error("tab 0 is managed, IsConfigTab() = true")
	}

	m.SetTabIndex(1) // config
	if !m.IsConfigTab() {
		t.Error("tab 1 is config, IsConfigTab() = false")
	}
	if m.IsManagedTab() {
		t.Error("tab 1 is config, IsManagedTab() = true")
	}
}

func TestActiveManagedStoreIdx(t *testing.T) {
	t.Parallel()
	m := NewIssuesList()
	m.SetTabs([]config.IssueTabConfig{{Name: "All", JQL: "x"}})
	m.SetSavedTabs([]config.ManagedTab{
		{Name: "XYZ-only", JQL: "a", Project: "XYZ"}, // store index 0, hidden under ABC
		{Name: "ABC-Bugs", JQL: "b", Project: "ABC"}, // store index 1, visible
	})
	m.RebuildTabs("ABC")

	m.SetTabIndex(0) // managed ABC-Bugs
	if got := m.ActiveManagedStoreIdx(); got != 1 {
		t.Errorf("ActiveManagedStoreIdx() = %d, want 1 (store position, not visible)", got)
	}

	m.SetTabIndex(1) // config All
	if got := m.ActiveManagedStoreIdx(); got != -1 {
		t.Errorf("ActiveManagedStoreIdx() on config tab = %d, want -1", got)
	}
}
