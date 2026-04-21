package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/tui/components"
)

// handleJQLSubmit starts a JQL search.
func (a *App) handleJQLSubmit(msg components.JQLSubmitMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = true
	a.jqlModal.SetLoading(true)
	return a, fetchJQLSearch(a.client, msg.Query, a.cfg.ResolveGlobalMaxResults())
}

// handleJQLSearchResult processes JQL search results.
func (a *App) handleJQLSearchResult(msg jqlSearchResultMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = false
	a.jqlModal.Hide()
	a.issuesList.AddJQLTab(msg.jql)
	a.issuesList.SetIssues(msg.issues)
	history := LoadJQLHistory()
	history = AddToHistory(history, msg.jql)
	_ = SaveJQLHistory(history)
	a.side = sideLeft
	a.leftFocus = focusIssues
	a.updateFocusState()
	cmds := make([]tea.Cmd, 0, len(msg.issues))
	for _, issue := range msg.issues {
		cmds = append(cmds, prefetchIssue(a.client, issue.Key))
	}
	return a, tea.Batch(cmds...)
}

// handleJQLSearchError shows error in JQL modal.
func (a *App) handleJQLSearchError(msg jqlSearchErrorMsg) (tea.Model, tea.Cmd) {
	*a.logFlag = false
	a.jqlModal.SetError(msg.err)
	return a, nil
}

// handleJQLInputChanged processes JQL autocomplete context.
func (a *App) handleJQLInputChanged(msg components.JQLInputChangedMsg) (tea.Model, tea.Cmd) {
	ctx := parseJQLContext(msg.Text, msg.CursorPos)
	a.jqlModal.SetPartialLen(ctx.PartialLen)
	switch ctx.Mode {
	case jqlCtxField:
		if a.jqlFields != nil {
			suggestions := matchFieldSuggestions(a.jqlFields, ctx.Partial)
			if len(suggestions) > 0 {
				a.jqlModal.SetSuggestions(suggestions)
			} else {
				a.jqlModal.SetHistory(LoadJQLHistory())
			}
		}
	case jqlCtxValue:
		a.jqlModal.SetACLoading(true)
		return a, fetchJQLSuggestions(a.client, ctx.FieldName, ctx.Partial)
	default:
		a.jqlModal.SetHistory(LoadJQLHistory())
	}
	return a, nil
}

// handleJQLFieldsLoaded caches JQL autocomplete field data.
func (a *App) handleJQLFieldsLoaded(msg jqlFieldsLoadedMsg) (tea.Model, tea.Cmd) {
	a.jqlFields = msg.fields
	return a, nil
}

// handleJQLSuggestions updates JQL suggestion list.
func (a *App) handleJQLSuggestions(msg jqlSuggestionsMsg) (tea.Model, tea.Cmd) {
	if a.jqlModal.IsVisible() {
		items := make([]string, 0, len(msg.suggestions))
		for _, s := range msg.suggestions {
			items = append(items, s.Value)
		}
		if len(items) > 0 {
			a.jqlModal.SetSuggestions(items)
		} else {
			a.jqlModal.SetHistory(LoadJQLHistory())
		}
	}
	return a, nil
}
