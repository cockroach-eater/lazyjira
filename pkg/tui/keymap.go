package tui

import (
	"slices"

	"github.com/textfuel/lazyjira/pkg/config"
)

// Action represents a user-triggerable action
type Action string

// Actions each can be remapped to different keys via config
const (
	ActQuit        Action = "quit"
	ActHelp        Action = "help"
	ActSearch      Action = "search"
	ActSwitchPanel Action = "switchPanel"
	ActFocusRight  Action = "focusRight"
	ActFocusLeft   Action = "focusLeft"
	ActSelect      Action = "select"      // primary: mark active + open
	ActOpen        Action = "open"        // secondary: open/preview without marking
	ActPrevTab     Action = "prevTab"
	ActNextTab     Action = "nextTab"
	ActFocusDetail Action = "focusDetail"
	ActFocusStatus Action = "focusStatus"
	ActFocusIssues Action = "focusIssues"
	ActFocusInfo   Action = "focusInfo"
	ActFocusProj   Action = "focusProjects"
	ActCopyURL     Action = "copyURL"
	ActBrowser     Action = "browser"
	ActURLPicker   Action = "urlPicker"
	ActTransition  Action = "transition"
	ActRefresh     Action = "refresh"
	ActRefreshAll  Action = "refreshAll"
	ActInfoTab     Action = "infoTab" // legacy: now focuses Info panel
	ActEdit        Action = "edit"
	ActComments    Action = "comments"
	ActNew Action = "new"
	ActPriority Action = "editPriority"
	ActAssignee Action = "editAssignee"
	ActJQLSearch    Action = "jqlSearch"
	ActCloseJQLTab  Action = "closeJQLTab"
	ActCreateBranch Action = "createBranch"
	ActCreateIssue    Action = "createIssue"
	ActDuplicateIssue Action = "duplicateIssue"
)

// Keymap maps actions to key strings. Multiple keys can trigger the same action
type Keymap map[Action][]string

// DefaultKeymap returns the default key bindings
func DefaultKeymap() Keymap {
	return Keymap{
		ActQuit:        {"q", "ctrl+c"},
		ActHelp:        {"?"},
		ActSearch:      {"/"},
		ActSwitchPanel: {"tab"},
		ActFocusRight:  {"l", "right"},
		ActFocusLeft:   {"h", "left", "esc"},
		ActSelect:      {" "},
		ActOpen:        {"enter"},
		ActPrevTab:     {"["},
		ActNextTab:     {"]"},
		ActFocusDetail: {"0"},
		ActFocusStatus: {"1"},
		ActFocusIssues: {"2"},
		ActFocusInfo:   {"3"},
		ActFocusProj:   {"4"},
		ActCopyURL:     {"y"},
		ActBrowser:     {"o"},
		ActURLPicker:   {"u"},
		ActTransition:  {"t"},
		ActRefresh:     {"r"},
		ActRefreshAll:  {"R"},
		ActInfoTab:         {"i"},
		ActEdit:        {"e"},
		ActComments:    {"c"},
		ActNew:  {"n"},
		ActPriority: {"p"},
		ActAssignee: {"a"},
		ActJQLSearch:    {"s"},
		ActCloseJQLTab:  {"x"},
		ActCreateBranch:    {"b"},
		ActDuplicateIssue: {"ctrl+n"},
	}
}

// KeymapFromConfig builds a Keymap starting from defaults and overriding
// with any non-empty values from the user's keybinding config.
func KeymapFromConfig(kcfg config.KeybindingConfig) Keymap {
	km := DefaultKeymap()
	set := func(action Action, val string) {
		if val != "" {
			km[action] = []string{val}
		}
	}
	// Universal
	set(ActQuit, kcfg.Universal.Quit)
	set(ActHelp, kcfg.Universal.Help)
	set(ActSearch, kcfg.Universal.Search)
	set(ActSwitchPanel, kcfg.Universal.SwitchPanel)
	set(ActRefresh, kcfg.Universal.Refresh)
	set(ActRefreshAll, kcfg.Universal.RefreshAll)
	set(ActPrevTab, kcfg.Universal.PrevTab)
	set(ActNextTab, kcfg.Universal.NextTab)
	set(ActFocusDetail, kcfg.Universal.FocusDetail)
	set(ActFocusStatus, kcfg.Universal.FocusStatus)
	set(ActFocusIssues, kcfg.Universal.FocusIssues)
	set(ActFocusInfo, kcfg.Universal.FocusInfo)
	set(ActFocusProj, kcfg.Universal.FocusProj)
	set(ActJQLSearch, kcfg.Universal.JQLSearch)
	// Issues (Select, Open, FocusRight are shared with Projects panel)
	set(ActSelect, kcfg.Issues.Select)
	set(ActOpen, kcfg.Issues.Open)
	set(ActFocusRight, kcfg.Issues.FocusRight)
	set(ActTransition, kcfg.Issues.Transition)
	set(ActBrowser, kcfg.Issues.Browser)
	set(ActURLPicker, kcfg.Issues.URLPicker)
	set(ActCopyURL, kcfg.Issues.CopyURL)
	set(ActCloseJQLTab, kcfg.Issues.CloseJQLTab)
	set(ActCreateBranch, kcfg.Issues.CreateBranch)
	set(ActCreateIssue, kcfg.Issues.CreateIssue)
	// Detail
	set(ActFocusLeft, kcfg.Detail.FocusLeft)
	set(ActInfoTab, kcfg.Detail.InfoTab)
	return km
}

// Match returns the action for the given key, or "" if none
func (km Keymap) Match(key string) Action {
	for action, keys := range km {
		if slices.Contains(keys, key) {
			return action
		}
	}
	return ""
}

// Keys returns the first key bound to the action (for display in help)
func (km Keymap) Keys(action Action) string {
	if keys, ok := km[action]; ok && len(keys) > 0 {
		k := keys[0]
		if k == " " {
			return "space"
		}
		return k
	}
	return ""
}
