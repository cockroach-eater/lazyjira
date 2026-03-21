package tui

import (
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// Binding represents a single keybinding with context.
type Binding struct {
	Key         string
	Description string
}

func (a *App) bind(action Action, desc string) Binding {
	return Binding{a.keymap.Keys(action), desc}
}

// ContextBindings returns keybindings for the current focus context.
func (a *App) ContextBindings() []Binding {
	km := a.keymap
	global := []Binding{
		{km.Keys(ActQuit), "quit"},
		{km.Keys(ActSwitchPanel), "switch left/right panels"},
		{km.Keys(ActFocusStatus), "focus status panel"},
		{km.Keys(ActFocusIssues), "focus issues panel"},
		{km.Keys(ActFocusProj), "focus projects panel"},
		{km.Keys(ActSearch), "search / filter current list"},
		{km.Keys(ActRefresh), "refresh data from Jira"},
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
			a.bind(ActEditPriority, "change priority"),
			a.bind(ActEditAssignee, "change assignee"),
			a.bind(ActBrowser, "open issue in browser"),
			a.bind(ActURLPicker, "open URL picker"),
			Binding{"[/]", "switch tab"},
		)

	case a.side == sideLeft && a.leftFocus == focusProjects:
		return append(global,
			Binding{"j/k", "navigate up/down"},
			Binding{"g/G", "go to top/bottom"},
			Binding{"ctrl+d/u", "half-page down/up"},
			a.bind(ActSelect, "select project and load issues"),
			a.bind(ActOpen, "preview project"),
			a.bind(ActFocusRight, "switch to detail panel"),
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
			Binding{"[/]", "previous/next tab"},
			a.bind(ActFocusLeft, "back to left panel"),
			a.bind(ActInfoTab, "jump to info tab"),
			a.bind(ActEditPriority, "change priority"),
			a.bind(ActEditAssignee, "change assignee"),
			a.bind(ActBrowser, "open in browser"),
			a.bind(ActURLPicker, "open URL picker"),
		)
		if a.detailView.ActiveTab() == views.TabComments {
			bindings = append(bindings,
				a.bind(ActEdit, "edit comment"),
				a.bind(ActAddComment, "new comment"),
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
	// Overlay-specific hints take priority over panel hints.
	switch {
	case a.showHelp:
		return []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
			{Key: "esc", Description: "close"},
		}
	case a.diffView.IsVisible():
		return []components.HelpItem{
			{Key: "enter", Description: "confirm"},
			{Key: "esc", Description: "cancel"},
			{Key: "j/k", Description: "scroll"},
		}
	case a.inputModal.IsVisible():
		return []components.HelpItem{
			{Key: "enter", Description: "confirm"},
			{Key: "esc", Description: "cancel"},
		}
	case a.modal.IsVisible() && a.modal.IsChecklist():
		return []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
			{Key: "space", Description: "toggle"},
			{Key: "/", Description: "search"},
			{Key: "enter", Description: "confirm"},
			{Key: "esc", Description: "cancel"},
		}
	case a.modal.IsVisible():
		return []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
			{Key: "/", Description: "search"},
			{Key: "enter", Description: "select"},
			{Key: "esc", Description: "cancel"},
		}
	}

	km := a.keymap
	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		return []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
			{Key: km.Keys(ActSelect), Description: "select"},
			{Key: km.Keys(ActEdit), Description: "edit"},
			{Key: km.Keys(ActComments), Description: "comments"},
			{Key: km.Keys(ActTransition), Description: "transition"},
			{Key: km.Keys(ActEditPriority), Description: "priority"},
			{Key: km.Keys(ActEditAssignee), Description: "assignee"},
			{Key: km.Keys(ActHelp), Description: "help"},
		}
	case a.side == sideLeft && a.leftFocus == focusProjects:
		return []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
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
			{Key: "j/k", Description: "scroll"},
			{Key: "[/]", Description: "tabs"},
		}
		switch a.detailView.ActiveTab() {
		case views.TabComments:
			items = append(items,
				components.HelpItem{Key: km.Keys(ActEdit), Description: "edit comment"},
				components.HelpItem{Key: km.Keys(ActAddComment), Description: "new comment"},
			)
		case views.TabInfo:
			items = append(items,
				components.HelpItem{Key: km.Keys(ActEdit), Description: "edit field"},
			)
		default:
			items = append(items,
				components.HelpItem{Key: km.Keys(ActEdit), Description: "edit"},
			)
		}
		items = append(items,
			components.HelpItem{Key: km.Keys(ActEditPriority), Description: "priority"},
			components.HelpItem{Key: km.Keys(ActEditAssignee), Description: "assignee"},
			components.HelpItem{Key: km.Keys(ActFocusLeft), Description: "back"},
			components.HelpItem{Key: km.Keys(ActHelp), Description: "help"},
		)
		return items
	}
	return nil
}
