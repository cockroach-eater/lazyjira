package tui

import (
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// Binding represents a single keybinding with context
type Binding struct {
	Key         string
	Description string
}

func (a *App) bind(action Action, desc string) Binding {
	return Binding{a.keymap.Keys(action), desc}
}

// ContextBindings returns keybindings for the current focus context
func (a *App) ContextBindings() []Binding {
	km := a.keymap
	global := []Binding{
		{km.Keys(ActQuit), "quit"},
		{km.Keys(ActSwitchPanel), "switch left/right panels"},
		{km.Keys(ActFocusStatus), "focus status panel"},
		{km.Keys(ActFocusIssues), "focus issues panel"},
		{km.Keys(ActFocusInfo), "focus info panel"},
		{km.Keys(ActFocusProj), "focus projects panel"},
		{km.Keys(ActSearch), "search / filter current list"},
		{km.Keys(ActRefresh), "refresh data from Jira"},
		a.bind(ActJQLSearch, "JQL search"),
		{km.Keys(ActHelp), "show all keybindings"},
	}

	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		return append(global,
			Binding{"j/k", "navigate up/down"},
			Binding{"g/G", "go to top/bottom"},
			Binding{"ctrl+d/u", "half-page down/up"},
			a.bind(ActSelect, "select issue (mark active + open)"),
			a.bind(ActOpen, "open issue detail"),
			a.bind(ActFocusRight, "open issue detail"),
			a.bind(ActTransition, "transition issue status"),
			a.bind(ActEdit, "edit summary"),
			a.bind(ActComments, "go to comments"),
			a.bind(ActPriority, "change priority"),
			a.bind(ActAssignee, "change assignee"),
			a.bind(ActBrowser, "open issue in browser"),
			a.bind(ActURLPicker, "open URL picker"),
			a.bind(ActCreateBranch, "create branch"),
			a.bind(ActNew, "create issue"),
			a.bind(ActDuplicateIssue, "duplicate issue"),
			a.bind(ActCloseJQLTab, "close JQL tab"),
			Binding{"[]", "switch tab"},
		)

	case a.side == sideLeft && a.leftFocus == focusInfo:
		return append(global,
			Binding{"j/k", "navigate up/down"},
			Binding{"g/G", "go to top/bottom"},
			Binding{"ctrl+d/u", "half-page down/up"},
			Binding{"[]", "switch tab (Info/Lnk/Sub)"},
			a.bind(ActEdit, "edit field"),
			a.bind(ActTransition, "transition issue status"),
			a.bind(ActPriority, "change priority"),
			a.bind(ActAssignee, "change assignee"),
			a.bind(ActBrowser, "open issue in browser"),
			a.bind(ActURLPicker, "open URL picker"),
			a.bind(ActFocusRight, "next panel"),
			a.bind(ActFocusLeft, "previous panel"),
		)

	case a.side == sideLeft && a.leftFocus == focusProjects:
		return append(global,
			Binding{"j/k", "navigate up/down"},
			Binding{"g/G", "go to top/bottom"},
			Binding{"ctrl+d/u", "half-page down/up"},
			a.bind(ActSelect, "select project and load issues"),
			a.bind(ActOpen, "preview project"),
			a.bind(ActFocusRight, "next panel"),
			a.bind(ActFocusLeft, "previous panel"),
		)

	case a.side == sideLeft && a.leftFocus == focusStatus:
		return append(global,
			a.bind(ActFocusRight, "switch to detail panel"),
		)

	case a.side == sideRight:
		bindings := make([]Binding, len(global))
		copy(bindings, global)
		bindings = append(bindings,
			Binding{"j/k", "scroll up/down"},
			Binding{"ctrl+d/u", "half-page down/up"},
			Binding{"[]", "previous/next tab"},
			a.bind(ActFocusLeft, "back to left panel"),
			a.bind(ActInfoTab, "focus info panel"),
			a.bind(ActPriority, "change priority"),
			a.bind(ActAssignee, "change assignee"),
			a.bind(ActBrowser, "open in browser"),
			a.bind(ActURLPicker, "open URL picker"),
		)
		if a.detailView.ActiveTab() == views.TabComments {
			bindings = append(bindings,
				a.bind(ActEdit, "edit comment"),
				a.bind(ActNew, "new comment"),
			)
		} else {
			bindings = append(bindings,
				a.bind(ActEdit, "edit description"),
			)
		}
		return bindings
	}

	return global
}

func (a *App) helpBarItems() []components.HelpItem {
	// Overlay-specific hints take priority over panel hints
	switch {
	case a.createForm.IsVisible():
		items := []components.HelpItem{
			{Key: "tab", Description: "next panel"},
			{Key: "enter", Description: "submit"},
			{Key: "esc", Description: "cancel"},
		}
		switch a.createForm.FocusedPanel() { //nolint:exhaustive
		case components.CreatePanelFields:
			items = append(items,
				components.HelpItem{Key: "e", Description: "edit"},
				components.HelpItem{Key: "/", Description: "filter"},
			)
		case components.CreatePanelDescription:
			items = append(items, components.HelpItem{Key: "e", Description: "edit in $EDITOR"})
		}
		return items
	case a.jqlModal.IsVisible():
		return []components.HelpItem{
			{Key: "enter", Description: "search"},
			{Key: "tab", Description: "switch focus"},
			{Key: "esc", Description: "cancel"},
		}
	case a.showHelp:
		return []components.HelpItem{
			{Key: "esc", Description: "close"},
		}
	case a.diffView.IsVisible():
		return []components.HelpItem{
			{Key: "enter", Description: "confirm"},
			{Key: "esc", Description: "cancel"},
		}
	case a.inputModal.IsVisible():
		items := []components.HelpItem{
			{Key: "enter", Description: "confirm"},
			{Key: "esc", Description: "cancel"},
		}
		if a.inputModal.HasHints() {
			items = append(items, components.HelpItem{Key: "tab", Description: "existing branches"})
		}
		return items
	case a.modal.IsVisible() && a.modal.IsChecklist():
		return []components.HelpItem{
			{Key: "space", Description: "toggle"},
			{Key: "/", Description: "search"},
			{Key: "enter", Description: "confirm"},
			{Key: "esc", Description: "cancel"},
		}
	case a.modal.IsVisible():
		return []components.HelpItem{
			{Key: "/", Description: "search"},
			{Key: "enter", Description: "select"},
			{Key: "esc", Description: "cancel"},
		}
	}

	km := a.keymap
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		items := []components.HelpItem{
			{Key: km.Keys(ActSelect), Description: "select"},
		}
		if a.issuesList.IsJQLTab() {
			items = append(items, components.HelpItem{Key: km.Keys(ActCloseJQLTab), Description: "close JQL"})
		}
		items = append(items,
			components.HelpItem{Key: km.Keys(ActEdit), Description: "edit"},
			components.HelpItem{Key: km.Keys(ActTransition), Description: "transition"},
			components.HelpItem{Key: km.Keys(ActPriority), Description: "priority"},
			components.HelpItem{Key: km.Keys(ActCreateBranch), Description: "branch"},
			components.HelpItem{Key: km.Keys(ActNew), Description: "create"},
			components.HelpItem{Key: km.Keys(ActJQLSearch), Description: "JQL search"},
			components.HelpItem{Key: km.Keys(ActHelp), Description: "help"},
		)
		return items
	case a.side == sideLeft && a.leftFocus == focusInfo:
		return []components.HelpItem{
			{Key: km.Keys(ActEdit), Description: "edit"},
			{Key: km.Keys(ActTransition), Description: "transition"},
			{Key: km.Keys(ActPriority), Description: "priority"},
			{Key: km.Keys(ActAssignee), Description: "assignee"},
			{Key: km.Keys(ActHelp), Description: "help"},
		}
	case a.side == sideLeft && a.leftFocus == focusProjects:
		return []components.HelpItem{
			{Key: km.Keys(ActSelect), Description: "select"},
			{Key: km.Keys(ActOpen), Description: "preview"},
			{Key: km.Keys(ActHelp), Description: "help"},
		}
	case a.side == sideLeft && a.leftFocus == focusStatus:
		return []components.HelpItem{
			{Key: km.Keys(ActSwitchPanel) + "/" + km.Keys(ActFocusRight), Description: "detail"},
			{Key: km.Keys(ActHelp), Description: "help"},
		}
	case a.side == sideRight:
		items := []components.HelpItem{
			{Key: "[]", Description: "tabs"},
		}
		switch a.detailView.ActiveTab() {
		case views.TabComments:
			items = append(items,
				components.HelpItem{Key: km.Keys(ActEdit), Description: "edit comment"},
				components.HelpItem{Key: km.Keys(ActNew), Description: "new comment"},
			)
		default:
			items = append(items,
				components.HelpItem{Key: km.Keys(ActEdit), Description: "edit"},
			)
		}
		items = append(items,
			components.HelpItem{Key: km.Keys(ActPriority), Description: "priority"},
			components.HelpItem{Key: km.Keys(ActAssignee), Description: "assignee"},
			components.HelpItem{Key: km.Keys(ActFocusLeft), Description: "back"},
			components.HelpItem{Key: km.Keys(ActHelp), Description: "help"},
		)
		return items
	}
	return nil
}
