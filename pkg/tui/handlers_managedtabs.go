package tui

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
)

// startManageTab opens the manage prompt for the active tab: save a transient
// JQL tab, or promote a config tab into the managed store. The query and page
// size are captured into editCtx so the single editTabName confirm path is
// source-agnostic.
func (a *App) startManageTab() (tea.Model, tea.Cmd) {
	if a.side != sideLeft || a.leftFocus != focusIssues {
		return a, nil
	}
	switch {
	case a.issuesList.IsJQLTab():
		a.inputModal.Show("Save tab", "")
		a.editContext = editCtx{kind: editTabName, tabJQL: a.issuesList.JQLQuery()}
	case a.issuesList.IsConfigTab():
		cfg := a.issuesList.ActiveTab()
		a.inputModal.Show("Promote tab", cfg.Name)
		a.editContext = editCtx{kind: editTabName, tabJQL: cfg.JQL, tabMaxResults: cfg.MaxResults}
	}
	return a, nil
}

// saveManagedTab persists jql as a named managed tab bound to the current
// project, then jumps to the new tab and fetches it. An empty name or query is
// rejected; a name already used by a managed tab in this project is warned about
// but still saved (shadowing semantics match Promote). Promote never triggers
// the warning: a config tab is only promotable while no same-named managed tab
// shadows it.
func (a *App) saveManagedTab(name, jql string, maxResults *int) tea.Cmd {
	if name == "" || jql == "" {
		return nil
	}
	// Idempotent: a managed tab with this name already visible in the project is
	// not duplicated. Jump to it instead; to change its query, edit in place (s).
	for _, mt := range a.savedTabs {
		if mt.Name == name && (mt.Project == "" || mt.Project == a.projectKey) {
			a.helpBar.SetStatusMsg("a tab named " + name + " already exists")
			a.jumpToManagedTab(name)
			return nil
		}
	}
	a.savedTabs = append(a.savedTabs, config.ManagedTab{Name: name, JQL: normalizeJQL(jql), Project: a.projectKey, MaxResults: maxResults})
	if err := config.SaveSavedTabs(a.savedTabs); err != nil {
		a.helpBar.SetStatusMsg("save tabs: " + err.Error())
	}
	a.issuesList.SetSavedTabs(a.savedTabs)
	a.jumpToManagedTab(name)
	return a.fetchActiveTab()
}

// reorderManagedTab swaps the active managed tab with its visible managed
// neighbor in direction dir (-1 left, +1 right), within the project-filtered
// managed section. No-op on non-managed tabs and at the section edges. The
// active tab follows the moved entry.
func (a *App) reorderManagedTab(dir int) tea.Cmd {
	if !a.issuesList.IsManagedTab() {
		return nil
	}
	visible := a.issuesList.VisibleManagedStoreIndices()
	pos := slices.Index(visible, a.issuesList.ActiveManagedStoreIdx())
	target := pos + dir
	if pos < 0 || target < 0 || target >= len(visible) {
		return nil
	}
	name := a.issuesList.ActiveTab().Name
	i, j := visible[pos], visible[target]
	a.savedTabs[i], a.savedTabs[j] = a.savedTabs[j], a.savedTabs[i]
	if err := config.SaveSavedTabs(a.savedTabs); err != nil {
		a.helpBar.SetStatusMsg("save tabs: " + err.Error())
	}
	a.issuesList.SetSavedTabs(a.savedTabs)
	a.jumpToManagedTab(name)
	return a.fetchActiveTab()
}

// startDeleteManagedTab asks for confirmation before removing the active managed
// tab from the store. No-op on config or transient tabs.
func (a *App) startDeleteManagedTab() (tea.Model, tea.Cmd) {
	if a.side != sideLeft || a.leftFocus != focusIssues {
		return a, nil
	}
	if !a.issuesList.IsManagedTab() {
		return a, nil
	}
	storeIdx := a.issuesList.ActiveManagedStoreIdx()
	a.onSelect = func(item components.ModalItem) tea.Cmd {
		if item.ID == "yes" {
			return a.deleteManagedTab(storeIdx)
		}
		return nil
	}
	a.modal.Show("Delete tab?", []components.ModalItem{
		{ID: "yes", Label: "Yes"},
		{ID: "no", Label: "No"},
	})
	return a, nil
}

// deleteManagedTab removes the store entry at storeIdx, persists, and rebuilds.
// A shadowed config tab of the same name reappears (un-promote).
func (a *App) deleteManagedTab(storeIdx int) tea.Cmd {
	if storeIdx < 0 || storeIdx >= len(a.savedTabs) {
		return nil
	}
	a.savedTabs = append(a.savedTabs[:storeIdx], a.savedTabs[storeIdx+1:]...)
	if err := config.SaveSavedTabs(a.savedTabs); err != nil {
		a.helpBar.SetStatusMsg("save tabs: " + err.Error())
	}
	a.issuesList.SetSavedTabs(a.savedTabs)
	return a.fetchActiveTab()
}

// jumpToManagedTab moves the active tab to the managed tab named name, if
// visible. It cycles the visible bar via the public tab API.
func (a *App) jumpToManagedTab(name string) {
	il := a.issuesList
	start := il.GetTabIndex()
	for {
		if il.IsManagedTab() && il.ActiveTab().Name == name {
			return
		}
		il.NextTab()
		if il.GetTabIndex() == start {
			return
		}
	}
}
