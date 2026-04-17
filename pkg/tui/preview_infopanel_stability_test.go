package tui

import (
	"testing"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// TestPreviewDetailLoaded_DoesNotMutateInfoPanel pins the invariant that a
// preview response must not overwrite the InfoPanel. The InfoPanel belongs to
// the main issue selected in the list, its active tab and cursor reflect the
// user's navigation context. Only the DetailView on the right may follow the
// previewed issue.
func TestPreviewDetailLoaded_DoesNotMutateInfoPanel(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)

	// Main issue selected in the list; InfoPanel belongs to it.
	main := &jira.Issue{Key: mainKey, Summary: "main"}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)

	// User has navigated into the Sub tab on the main issue.
	a.previewKey = subKey1
	a.previewEpoch = 1

	// The sub-issue's fetch response arrives.
	_, _ = a.Update(previewDetailLoadedMsg{
		issue: &jira.Issue{Key: subKey1, Summary: "sub"},
		epoch: 1,
	})

	if got := a.infoPanel.IssueKey(); got != mainKey {
		t.Errorf("infoPanel.IssueKey() = %q after preview response, want %q (InfoPanel must stay on main issue)", got, mainKey)
	}
}

// TestIssueDetailLoaded_DoesNotMutateInfoPanelForSubPreview covers the
// ActRefresh path: when the user is previewing a sub-issue and presses refresh,
// the detail response comes through the non-preview channel. It still must not
// overwrite the InfoPanel.
func TestIssueDetailLoaded_DoesNotMutateInfoPanelForSubPreview(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)

	main := &jira.Issue{Key: mainKey, Summary: "main"}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)

	// Previewing a sub-issue.
	a.previewKey = subKey1

	_, _ = a.handleIssueDetailLoaded(issueDetailLoadedMsg{
		issue: &jira.Issue{Key: subKey1, Summary: "sub-fresh"},
	})

	if got := a.infoPanel.IssueKey(); got != mainKey {
		t.Errorf("infoPanel.IssueKey() = %q after sub-issue refresh, want %q", got, mainKey)
	}
}

// TestShowCachedIssue_DoesNotMutateInfoPanelForForeignKey ensures navigating
// to a linked issue (not the main list selection) does not reset the InfoPanel.
// showCachedIssue is called from navigateToLinkedIssue and friends with a key
// that does not match the current list cursor; before the fix it overwrote
// the InfoPanel and reset its tab/cursor.
func TestShowCachedIssue_DoesNotMutateInfoPanelForForeignKey(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	main := &jira.Issue{Key: mainKey, Summary: "main"}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)
	a.issueCache[subKey1] = &jira.Issue{Key: subKey1, Summary: "sub"}

	a.showCachedIssue(subKey1)

	if got := a.infoPanel.IssueKey(); got != mainKey {
		t.Errorf("infoPanel.IssueKey() = %q after showCachedIssue(%q), want %q",
			got, subKey1, mainKey)
	}
}

// TestPreviewRequestMsg_CacheHit_UpdatesDetailViewImmediately pins the
// invariant that a preview request for an already-cached issue is served
// synchronously from cache. No fetch, no debounce delay, matching the
// feel of scrolling through the main issue list.
func TestPreviewRequestMsg_CacheHit_UpdatesDetailViewImmediately(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	// No *Func set: any HTTP path would t.Fatalf.
	a := newAppWithFake(t, fake)

	cached := &jira.Issue{Key: subKey1, Summary: "cached sub"}
	a.issueCache[subKey1] = cached

	_, _ = a.Update(views.PreviewRequestMsg{Key: subKey1})

	if got := a.detailView.IssueKey(); got != subKey1 {
		t.Errorf("detailView.IssueKey() = %q, want %q (cache hit should update synchronously)", got, subKey1)
	}
	if a.previewKey != subKey1 {
		t.Errorf("previewKey = %q, want %q", a.previewKey, subKey1)
	}
}

// TestTabSwitchToSubtasks_DispatchesPreviewRequest verifies that entering the
// Subtasks tab immediately previews the first subtask without requiring an
// extra cursor move.
func TestTabSwitchToSubtasks_DispatchesPreviewRequest(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	main := &jira.Issue{
		Key:      mainKey,
		Subtasks: []jira.Issue{{Key: subKey1}},
	}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)
	a.side = sideLeft
	a.leftFocus = focusInfo

	// Cycle: Fields -> Links (no links here) -> Subtasks.
	_, _, _ = a.handleTabAction(ActNextTab)
	_, cmd, handled := a.handleTabAction(ActNextTab)
	if !handled {
		t.Fatal("ActNextTab not handled")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd on sub-tab entry, got nil")
	}
	msg := cmd()
	pr, ok := msg.(views.PreviewRequestMsg)
	if !ok {
		t.Fatalf("expected PreviewRequestMsg, got %T", msg)
	}
	if pr.Key != subKey1 {
		t.Errorf("PreviewRequestMsg.Key = %q, want %q", pr.Key, subKey1)
	}
}

// TestTabSwitchToLinks_DispatchesPreviewRequest covers the same behaviour for
// the Links tab.
func TestTabSwitchToLinks_DispatchesPreviewRequest(t *testing.T) {
	const linkKey = "LNK-1"

	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	main := &jira.Issue{
		Key: mainKey,
		IssueLinks: []jira.IssueLink{{
			Type:         &jira.IssueLinkType{Name: "relates to"},
			OutwardIssue: &jira.Issue{Key: linkKey},
		}},
	}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)
	a.side = sideLeft
	a.leftFocus = focusInfo

	// One NextTab advances from Fields to Links.
	_, cmd, handled := a.handleTabAction(ActNextTab)
	if !handled {
		t.Fatal("ActNextTab not handled")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd on link-tab entry, got nil")
	}
	msg := cmd()
	pr, ok := msg.(views.PreviewRequestMsg)
	if !ok {
		t.Fatalf("expected PreviewRequestMsg, got %T", msg)
	}
	if pr.Key != linkKey {
		t.Errorf("PreviewRequestMsg.Key = %q, want %q", pr.Key, linkKey)
	}
}

// TestTabSwitchToSubtasks_EmptyListNoDispatch guards the empty-list edge case:
// when the main issue has no subtasks, entering the Subtasks tab must not fire
// a PreviewRequestMsg.
func TestTabSwitchToSubtasks_EmptyListNoDispatch(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	main := &jira.Issue{Key: mainKey} // no subtasks, no links
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)
	a.side = sideLeft
	a.leftFocus = focusInfo

	// Cycle to Links (empty) then Subtasks (empty).
	_, cmd1, _ := a.handleTabAction(ActNextTab)
	if cmd1 != nil {
		if msg := cmd1(); msg != nil {
			if _, ok := msg.(views.PreviewRequestMsg); ok {
				t.Errorf("empty Links tab dispatched PreviewRequestMsg, want no dispatch")
			}
		}
	}
	_, cmd2, _ := a.handleTabAction(ActNextTab)
	if cmd2 != nil {
		if msg := cmd2(); msg != nil {
			if _, ok := msg.(views.PreviewRequestMsg); ok {
				t.Errorf("empty Subtasks tab dispatched PreviewRequestMsg, want no dispatch")
			}
		}
	}
}
