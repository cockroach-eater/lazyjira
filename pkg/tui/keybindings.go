package tui

import (
	"slices"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

// Binding represents a single keybinding with context
type Binding struct {
	Key         string
	Description string
}

func (a *App) bind(action Action, desc string) Binding {
	return Binding{a.keymap.Keys(action), desc}
}

func (a *App) navBindings() []Binding {
	return []Binding{
		a.bind(ActNavDown, "navigate down"),
		a.bind(ActNavUp, "navigate up"),
		a.bind(ActNavTop, "go to top"),
		a.bind(ActNavBottom, "go to bottom"),
		a.bind(ActNavHalfDown, "half-page down"),
		a.bind(ActNavHalfUp, "half-page up"),
	}
}

func (a *App) detailScrollBindings() []Binding {
	return []Binding{
		a.bind(ActDetailScrollDown, "scroll detail down"),
		a.bind(ActDetailScrollUp, "scroll detail up"),
		a.bind(ActDetailHalfDown, "half-page detail down"),
		a.bind(ActDetailHalfUp, "half-page detail up"),
	}
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
		bindings := slices.Concat(global, a.navBindings(), a.detailScrollBindings())
		bindings = append(bindings,
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
		// Tab-management keys depend on the active tab kind so the reference
		// never advertises an action that does nothing on the current tab.
		switch {
		case a.issuesList.IsJQLTab():
			bindings = append(bindings, a.bind(ActManageTab, "save as managed tab"))
		case a.issuesList.IsConfigTab():
			bindings = append(bindings, a.bind(ActManageTab, "promote to managed tab"))
		case a.issuesList.IsManagedTab():
			bindings = append(bindings,
				a.bind(ActJQLSearch, "edit query"),
				a.bind(ActDeleteManagedTab, "delete managed tab"),
				a.bind(ActReorderTabLeft, "move tab left"),
				a.bind(ActReorderTabRight, "move tab right"),
			)
		}
		bindings = append(bindings, a.customCommandBindings(config.CtxIssues)...)
		return bindings

	case a.side == sideLeft && a.leftFocus == focusInfo:
		bindings := slices.Concat(global, a.navBindings(), a.detailScrollBindings())
		bindings = append(bindings,
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
		bindings = append(bindings, a.customCommandBindings(config.CtxInfo)...)
		return bindings

	case a.side == sideLeft && a.leftFocus == focusProjects:
		bindings := slices.Concat(global, a.navBindings())
		bindings = append(bindings,
			a.bind(ActSelect, "select project and load issues"),
			a.bind(ActFocusRight, "next panel"),
			a.bind(ActFocusLeft, "previous panel"),
		)
		bindings = append(bindings, a.customCommandBindings(config.CtxProjects)...)
		return bindings

	case a.side == sideLeft && a.leftFocus == focusStatus:
		return append(global,
			a.bind(ActFocusRight, "switch to detail panel"),
		)

	case a.side == sideRight:
		bindings := slices.Concat(global, a.navBindings())
		bindings = append(bindings,
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
		bindings = append(bindings, a.customCommandBindings(config.CtxDetail)...)
		if a.detailView.ActiveTab() == views.TabComments {
			bindings = append(bindings, a.customCommandBindings(config.CtxDetailComments)...)
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
		// Enter and Ctrl+S do different things depending on whether the modal is
		// editing an existing managed tab (Enter overwrites it) or running an
		// ad-hoc search (Enter searches); label them by what they actually do.
		enterDesc, saveDesc := "search", "save tab"
		if a.editingManagedTab >= 0 {
			enterDesc, saveDesc = "update tab", "save as new tab"
		}
		return []components.HelpItem{
			{Key: "enter", Description: enterDesc},
			{Key: "tab", Description: "switch focus"},
			{Key: "ctrl+s", Description: saveDesc},
			{Key: "esc", Description: "cancel"},
		}
	case a.showHelp:
		return []components.HelpItem{
			{Key: "/", Description: "filter"},
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
		var items []components.HelpItem
		if a.issuesList.IsJQLTab() {
			items = append(items, components.HelpItem{Key: km.Keys(ActCloseJQLTab), Description: "close JQL"})
		}
		switch {
		case a.issuesList.IsJQLTab():
			items = append(items, components.HelpItem{Key: km.Keys(ActManageTab), Description: "save tab"})
		case a.issuesList.IsConfigTab():
			items = append(items, components.HelpItem{Key: km.Keys(ActManageTab), Description: "promote tab"})
		case a.issuesList.IsManagedTab():
			items = append(items,
				components.HelpItem{Key: km.Keys(ActJQLSearch), Description: "edit query"},
				components.HelpItem{Key: km.Keys(ActDeleteManagedTab), Description: "delete tab"},
				components.HelpItem{Key: km.Keys(ActReorderTabLeft) + km.Keys(ActReorderTabRight), Description: "move tab"},
			)
		}
		cur := a.currentIssue()
		enterDesc := "detail"
		if cur != nil {
			children, resolved := a.childrenForList(cur)
			if resolved && len(children) > 0 {
				enterDesc = "children"
			}
		}
		items = append(items, components.HelpItem{Key: km.Keys(ActSelect), Description: enterDesc})
		if cur != nil && cur.Parent != nil && cur.Parent.Key != "" {
			items = append(items, components.HelpItem{Key: km.Keys(ActShowParent), Description: "parent"})
		}
		items = append(items,
			components.HelpItem{Key: km.Keys(ActEdit), Description: "edit"},
			components.HelpItem{Key: km.Keys(ActTransition), Description: "transition"},
			components.HelpItem{Key: km.Keys(ActPriority), Description: "priority"},
			components.HelpItem{Key: km.Keys(ActCreateBranch), Description: "branch"},
			components.HelpItem{Key: km.Keys(ActNew), Description: "create"},
		)
		// On a managed tab the JQL key edits that tab's query (shown above);
		// elsewhere it starts a fresh search.
		if !a.issuesList.IsManagedTab() {
			items = append(items, components.HelpItem{Key: km.Keys(ActJQLSearch), Description: "JQL search"})
		}
		if a.canCreateSubtask() {
			items = append(items, components.HelpItem{Key: km.Keys(ActCreateSubtask), Description: "subtask"})
		}
		items = append(items, a.customCommandHelpItems(config.CtxIssues)...)
		items = append(items, components.HelpItem{Key: km.Keys(ActHelp), Description: "help"})
		return items
	case a.side == sideLeft && a.leftFocus == focusInfo:
		items := make([]components.HelpItem, 0, 6)
		items = append(items,
			components.HelpItem{Key: km.Keys(ActEdit), Description: "edit"},
			components.HelpItem{Key: km.Keys(ActTransition), Description: "transition"},
			components.HelpItem{Key: km.Keys(ActPriority), Description: "priority"},
			components.HelpItem{Key: km.Keys(ActAssignee), Description: "assignee"},
		)
		items = append(items, a.customCommandHelpItems(config.CtxInfo)...)
		items = append(items, components.HelpItem{Key: km.Keys(ActHelp), Description: "help"})
		return items
	case a.side == sideLeft && a.leftFocus == focusProjects:
		items := make([]components.HelpItem, 0, 4)
		items = append(items,
			components.HelpItem{Key: km.Keys(ActSelect), Description: "select"},
			components.HelpItem{Key: km.Keys(ActOpen), Description: "preview"},
		)
		items = append(items, a.customCommandHelpItems(config.CtxProjects)...)
		items = append(items, components.HelpItem{Key: km.Keys(ActHelp), Description: "help"})
		return items
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
		)
		items = append(items, a.customCommandHelpItems(config.CtxDetail)...)
		if a.detailView.ActiveTab() == views.TabComments {
			items = append(items, a.customCommandHelpItems(config.CtxDetailComments)...)
		}
		items = append(items, components.HelpItem{Key: km.Keys(ActHelp), Description: "help"})
		return items
	}
	return nil
}
