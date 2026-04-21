package tui

import (
	"context"
	"testing"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

const mainKey = "ABC-1"
const subKey1 = "SUB-1"

// newAppWithFake augments newTestApp() with a FakeClient, a non-nil logFlag,
// an InfoPanel, a StatusPanel, and an empty issueCache — enough to flow a
// message through App.Update without NPEs.
func newAppWithFake(t *testing.T, fake *jiratest.FakeClient) *App {
	t.Helper()
	a := newTestApp()
	a.client = fake
	logFlag := false
	a.logFlag = &logFlag
	a.infoPanel = views.NewInfoPanel()
	a.statusPanel = views.NewStatusPanel("", "", "")
	a.logPanel = views.NewLogPanel()
	a.issueCache = map[string]*jira.Issue{}
	return a
}

// stubFullIssueFetch wires the three methods that fetchFullIssue calls.
// Only GetIssue returns a real issue; Comments + Changelog are empty.
func stubFullIssueFetch(fake *jiratest.FakeClient, issue *jira.Issue) {
	fake.GetIssueFunc = func(_ context.Context, _ string) (*jira.Issue, error) {
		return issue, nil
	}
	fake.GetCommentsFunc = func(_ context.Context, _ string) ([]jira.Comment, error) {
		return nil, nil
	}
	fake.GetChangelogFunc = func(_ context.Context, _ string) ([]jira.ChangelogEntry, error) {
		return nil, nil
	}
}

// TestActRefresh_FetchesPreviewedIssue documents the happy path: after the
// usual navigation sets previewKey to the displayed issue, pressing refresh
// re-fetches that issue.
func TestActRefresh_FetchesPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: mainKey, Summary: "updated"})

	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	a.previewKey = mainKey

	_, cmd, handled := a.handleIssueAction(ActRefresh)
	if !handled {
		t.Fatal("ActRefresh was not handled")
	}
	if cmd == nil {
		t.Fatal("expected tea.Cmd, got nil")
	}
	msg := cmd()

	if len(fake.GetIssueCalls) != 1 {
		t.Fatalf("expected 1 GetIssue call, got %d: %+v", len(fake.GetIssueCalls), fake.GetIssueCalls)
	}
	if got := fake.GetIssueCalls[0].Key; got != mainKey {
		t.Errorf("GetIssue called with key %q, want %q", got, mainKey)
	}

	loaded, ok := msg.(issueDetailLoadedMsg)
	if !ok {
		t.Fatalf("expected issueDetailLoadedMsg, got %T", msg)
	}
	if loaded.issue == nil || loaded.issue.Key != mainKey {
		t.Errorf("loaded.issue = %+v, want Key=ABC-1", loaded.issue)
	}
}

// TestIssueSelectedMsg_UpdatesPreviewKey pins down the invariant that the
// previewKey follows whatever issue the user has selected in the list.
func TestIssueSelectedMsg_UpdatesPreviewKey(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)

	_, _ = a.Update(views.IssueSelectedMsg{Issue: &jira.Issue{Key: mainKey}})

	if got := a.previewKey; got != mainKey {
		t.Errorf("previewKey = %q, want %q", got, mainKey)
	}
}

// TestPreviewSelectedIssue_UpdatesPreviewKey covers the helper that syncs the
// preview to the current list selection (called on tab switches and after
// issues load). It must keep previewKey aligned.
func TestPreviewSelectedIssue_UpdatesPreviewKey(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: "XYZ-9"}})

	a.previewSelectedIssue()

	if got := a.previewKey; got != "XYZ-9" {
		t.Errorf("previewKey = %q, want %q", got, "XYZ-9")
	}
}

// TestHandleIssueDetailLoaded_RoutesByPreviewKey ensures a detail response for
// the previewed sub-issue updates the DetailView (which follows previewKey)
// but leaves the InfoPanel untouched (the InfoPanel belongs to the main list
// issue and must keep its tab/cursor).
func TestHandleIssueDetailLoaded_RoutesByPreviewKey(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	main := &jira.Issue{Key: mainKey, Summary: "main"}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)
	a.previewKey = subKey1

	_, _ = a.handleIssueDetailLoaded(issueDetailLoadedMsg{
		issue: &jira.Issue{Key: subKey1, Summary: "fresh"},
	})

	if got := a.detailView.IssueKey(); got != subKey1 {
		t.Errorf("detailView.IssueKey() = %q, want %q (DetailView follows previewKey)", got, subKey1)
	}
	if got := a.infoPanel.IssueKey(); got != mainKey {
		t.Errorf("infoPanel.IssueKey() = %q, want %q (InfoPanel stays on main issue)", got, mainKey)
	}
}

// TestActRefresh_NoFetchWhenPreviewKeyEmpty pins the invariant that previewKey
// is the single source of truth: with no preview active, refresh is a no-op
// even if the list has a selection. This removes the implicit fallback to
// the list cursor.
func TestActRefresh_NoFetchWhenPreviewKeyEmpty(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	// No *Func set — any call would t.Fatalf.
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	// previewKey intentionally left empty.

	_, cmd, handled := a.handleIssueAction(ActRefresh)
	if !handled {
		t.Fatal("ActRefresh was not handled")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd (no fetch), got non-nil")
	}
	if len(fake.GetIssueCalls) != 0 {
		t.Errorf("expected 0 GetIssue calls, got %d: %+v", len(fake.GetIssueCalls), fake.GetIssueCalls)
	}
}

// TestActRefresh_UsesPreviewKey_WhenSet documents the desired behavior:
// when a preview (e.g. a sub-issue selected in the info panel) is active,
// the refresh action must re-fetch that preview's issue, not the main list
// selection.
func TestActRefresh_UsesPreviewKey_WhenSet(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: "ABC-2", Summary: "sub-item"})

	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	a.previewKey = "ABC-2"

	_, cmd, handled := a.handleIssueAction(ActRefresh)
	if !handled {
		t.Fatal("ActRefresh was not handled")
	}
	if cmd == nil {
		t.Fatal("expected tea.Cmd, got nil")
	}
	cmd()

	if len(fake.GetIssueCalls) != 1 {
		t.Fatalf("expected 1 GetIssue call, got %d: %+v", len(fake.GetIssueCalls), fake.GetIssueCalls)
	}
	if got := fake.GetIssueCalls[0].Key; got != "ABC-2" {
		t.Errorf("GetIssue called with key %q, want %q (preview key)", got, "ABC-2")
	}
}

// TestActRefresh_InvalidatesCacheBeforeFetch ensures that ActRefresh removes the
// existing cache entry for the previewed issue synchronously, before the fetch
// cmd is dispatched. Any cache read between the keypress and the response must
// see a miss, never stale data.
func TestActRefresh_InvalidatesCacheBeforeFetch(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: mainKey, Summary: "fresh"})

	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	a.previewKey = mainKey
	// Pre-populate the cache with a stale entry.
	stale := &jira.Issue{Key: mainKey, Summary: "stale"}
	a.issueCache[mainKey] = stale

	_, _, handled := a.handleIssueAction(ActRefresh)
	if !handled {
		t.Fatal("ActRefresh was not handled")
	}

	// The cache entry must be gone synchronously — before cmd is executed.
	if _, ok := a.issueCache[mainKey]; ok {
		t.Errorf("issueCache[%q] still present after ActRefresh; expected cache invalidation", mainKey)
	}
}
