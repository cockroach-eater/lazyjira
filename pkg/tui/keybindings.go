package tui

import "github.com/cockroach-eater/lazyjira/pkg/tui/components"

// Binding represents a single keybinding with context.
type Binding struct {
	Key         string
	Description string
}

// ContextBindings returns keybindings for the current focus context.
// Used both for the help bar (short) and the ? popup (full list).
func (a *App) ContextBindings() []Binding {
	// Global bindings always available.
	global := []Binding{
		{"q", "quit"},
		{"tab", "switch left/right panels"},
		{"1", "focus status panel"},
		{"2", "focus issues panel"},
		{"3", "focus projects panel"},
		{"/", "search / filter current list"},
		{"r", "refresh data from Jira"},
		{"?", "show all keybindings"},
	}

	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		return append(global,
			Binding{"j/k", "navigate up/down"},
			Binding{"g/G", "go to top/bottom"},
			Binding{"ctrl+d/u", "half-page down/up"},
			Binding{"enter", "open issue detail (right panel)"},
			Binding{"l", "open issue detail (right panel)"},
			Binding{"t", "transition issue status"},
			Binding{"o", "open issue in browser"},
			Binding{"u", "open URL picker"},
			Binding{"[/]", "switch All/Assigned"},
		)

	case a.side == sideLeft && a.leftFocus == focusProjects:
		return append(global,
			Binding{"j/k", "navigate up/down"},
			Binding{"enter", "select project and load issues"},
			Binding{"l", "switch to detail panel"},
		)

	case a.side == sideLeft && a.leftFocus == focusStatus:
		return append(global,
			Binding{"l", "switch to detail panel"},
		)

	case a.side == sideRight:
		return append(global,
			Binding{"j/k", "scroll up/down"},
			Binding{"ctrl+d/u", "half-page down/up"},
			Binding{"[/]", "previous/next tab"},
			Binding{"h", "back to left panel"},
			Binding{"i", "jump to info tab"},
			Binding{"o", "open in browser"},
			Binding{"u", "open URL picker"},
		)
	}

	return global
}

// HelpBarItems returns a short subset for the bottom help bar.
func (a *App) helpBarItems() []components.HelpItem {
	var items []components.HelpItem

	switch {
	case a.side == sideLeft && a.leftFocus == focusIssues:
		items = []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
			{Key: "enter/l", Description: "open"},
			{Key: "t", Description: "transition"},
			{Key: "/", Description: "search"},
			{Key: "?", Description: "help"},
		}
	case a.side == sideLeft && a.leftFocus == focusProjects:
		items = []components.HelpItem{
			{Key: "j/k", Description: "navigate"},
			{Key: "enter", Description: "select"},
			{Key: "/", Description: "search"},
			{Key: "?", Description: "help"},
		}
	case a.side == sideLeft && a.leftFocus == focusStatus:
		items = []components.HelpItem{
			{Key: "tab/l", Description: "detail"},
			{Key: "?", Description: "help"},
		}
	case a.side == sideRight:
		items = []components.HelpItem{
			{Key: "j/k", Description: "scroll"},
			{Key: "[/]", Description: "tabs"},
			{Key: "h", Description: "back"},
			{Key: "i", Description: "info"},
			{Key: "?", Description: "help"},
		}
	}

	return items
}

// HelpPopupContent renders all keybindings for the ? popup.
func (a *App) HelpPopupContent(width int) string {
	bindings := a.ContextBindings()
	maxKey := 0
	for _, b := range bindings {
		if len(b.Key) > maxKey {
			maxKey = len(b.Key)
		}
	}

	var lines string
	for _, b := range bindings {
		lines += " " + a.helpKeyStyle(b.Key, maxKey) + "  " + b.Description + "\n"
	}
	return lines
}

func (a *App) helpKeyStyle(key string, maxW int) string {
	padded := key
	for len(padded) < maxW {
		padded += " "
	}
	return padded
}
