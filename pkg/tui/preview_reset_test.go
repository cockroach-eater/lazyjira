package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// setupInfoFocusedOnSubtabs sets up an App with:
//   - mainKey as the list selection
//   - previewKey set to subKey (simulating a prior sub-issue preview)
//   - InfoPanel focused, active tab = tab
//   - InfoPanel loaded with a non-empty issue so it has content
func setupInfoFocused(t *testing.T, tab views.InfoPanelTab) *App {
	t.Helper()
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)

	mainIssue := jira.Issue{
		Key:     mainKey,
		Summary: "main issue",
		Subtasks: []jira.Issue{
			{Key: subKey1, Summary: "sub one"},
		},
		IssueLinks: []jira.IssueLink{
			{
				Type:         &jira.IssueLinkType{Name: "Blocks", Outward: "blocks", Inward: "is blocked by"},
				OutwardIssue: &jira.Issue{Key: "LNK-1", Summary: "linked"},
			},
		},
	}
	a.issuesList.SetIssues([]jira.Issue{mainIssue})
	a.infoPanel.SetIssue(&mainIssue)

	// Advance infopanel to the requested tab
	for a.infoPanel.ActiveTab() != tab {
		a.infoPanel.NextTab()
	}

	// Simulate that user moved cursor in Sub/Lnk tab and preview was set to a sub-key
	a.previewKey = subKey1

	a.side = sideLeft
	a.leftFocus = focusInfo
	a.infoPanel.Focused = true
	a.keymap = DefaultKeymap()
	a.infoPanel.ResolveNav = DefaultKeymap().MatchNav
	return a
}

// TestEscFromSubTab_ResetsPreviewToMainIssue verifies that the Esc/FocusLeft
// action while the InfoPanel has focus and the Sub tab is active resets
// previewKey to the list selection's key.
func TestEscFromSubTab_ResetsPreviewToMainIssue(t *testing.T) {
	a := setupInfoFocused(t, views.InfoTabSubtasks)
	if a.previewKey != subKey1 {
		t.Fatalf("precondition: previewKey = %q, want SUB-1", a.previewKey)
	}

	a.handleFocusAction(ActFocusLeft)

	if got := a.previewKey; got != mainKey {
		t.Errorf("previewKey = %q after FocusLeft from Sub tab, want %q", got, mainKey)
	}
}

// TestEscFromLnkTab_ResetsPreviewToMainIssue verifies the same reset when the
// Lnk tab is active.
func TestEscFromLnkTab_ResetsPreviewToMainIssue(t *testing.T) {
	a := setupInfoFocused(t, views.InfoTabLinks)
	a.previewKey = "LNK-1"

	a.handleFocusAction(ActFocusLeft)

	if got := a.previewKey; got != mainKey {
		t.Errorf("previewKey = %q after FocusLeft from Lnk tab, want %q", got, mainKey)
	}
}

// TestInfoPanelTabSwitchToFields_ResetsPreviewKey verifies that switching the
// InfoPanel tab to Fields resets previewKey to the current list selection.
func TestInfoPanelTabSwitchToFields_ResetsPreviewKey(t *testing.T) {
	// Start on Sub tab with previewKey pointing at a sub-issue
	a := setupInfoFocused(t, views.InfoTabSubtasks)
	a.previewKey = subKey1

	// Switch from Sub -> Lnk -> Fields (NextTab cycles Fields->Lnk->Sub;
	// we need to tab until we reach Fields)
	// visibleTabs order: Fields, Links, Subtasks (indices 0,1,2)
	// PrevTab from Subtasks -> Links -> Fields
	a.handleTabAction(ActPrevTab) // Sub -> Lnk
	a.handleTabAction(ActPrevTab) // Lnk -> Fields

	if got := a.previewKey; got != mainKey {
		t.Errorf("previewKey = %q after switching to Fields tab, want %q", got, mainKey)
	}
}

// TestEmptySubList_NoPreviewDispatch verifies that switching to the Sub tab on
// an issue with no subtasks does not produce a PreviewRequestMsg.
func TestEmptySubList_NoPreviewDispatch(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	// No GetIssueFunc set — any call would t.Fatalf via FakeClient.
	a := newAppWithFake(t, fake)

	issueNoSubs := &jira.Issue{Key: mainKey, Summary: "no subs"}
	a.issuesList.SetIssues([]jira.Issue{*issueNoSubs})
	a.infoPanel.SetIssue(issueNoSubs)
	a.previewKey = mainKey
	a.side = sideLeft
	a.leftFocus = focusInfo
	a.infoPanel.Focused = true
	a.keymap = DefaultKeymap()

	// Ensure we start on Fields tab
	if a.infoPanel.ActiveTab() != views.InfoTabFields {
		t.Fatal("precondition: expected InfoTabFields as default tab")
	}

	// Switch to Sub tab via NextTab actions (Fields->Links->Subtasks)
	a.handleTabAction(ActNextTab) // Fields -> Links
	a.handleTabAction(ActNextTab) // Links -> Sub

	if a.infoPanel.ActiveTab() != views.InfoTabSubtasks {
		t.Fatal("expected InfoTabSubtasks after two NextTab actions")
	}

	// Now simulate cursor movement in Sub tab — should produce no PreviewRequestMsg
	// because there are no subtasks. The InfoPanel.Update handles cursor movement.
	navKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	a.infoPanel.ResolveNav = DefaultKeymap().MatchNav
	_, cmd := a.infoPanel.Update(navKey)
	if cmd == nil {
		return // nil is acceptable: no preview dispatch
	}
	msg := cmd()
	if _, ok := msg.(views.PreviewRequestMsg); ok {
		t.Error("empty Sub list must not dispatch PreviewRequestMsg on cursor move")
	}
}
