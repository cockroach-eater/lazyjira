package tui

import (
	"context"
	"testing"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// TestPreviewDebounce_RapidMovement verifies that when two PreviewRequestMsgs
// arrive in rapid succession only the second one causes a GetIssue call.
//
// We simulate debounce deterministically without real timers: dispatch both
// PreviewRequestMsgs first so the epoch advances to 2, then dispatch a
// synthetic previewDebounceMsg for epoch=1 (stale) followed by one for
// epoch=2 (fresh). Only the fresh one must trigger a GetIssue call.
func TestPreviewDebounce_RapidMovement(t *testing.T) {
	const key2 = "SUB-2"
	key1 := subKey1

	fake := &jiratest.FakeClient{T: t}
	// Configure for key2 only; any unexpected GetIssue for key1 would t.Fatalf.
	fake.GetIssueFunc = func(_ context.Context, key string) (*jira.Issue, error) {
		if key != key2 {
			t.Errorf("unexpected GetIssue call for key %q (expected %q only)", key, key2)
		}
		return &jira.Issue{Key: key}, nil
	}
	fake.GetCommentsFunc = func(_ context.Context, _ string) ([]jira.Comment, error) {
		return nil, nil
	}
	fake.GetChangelogFunc = func(_ context.Context, _ string) ([]jira.ChangelogEntry, error) {
		return nil, nil
	}

	a := newAppWithFake(t, fake)

	// Two rapid PreviewRequestMsgs advance the epoch to 2.
	_, _ = a.Update(views.PreviewRequestMsg{Key: key1})
	_, _ = a.Update(views.PreviewRequestMsg{Key: key2})

	if a.previewEpoch != 2 {
		t.Fatalf("previewEpoch = %d after two msgs, want 2", a.previewEpoch)
	}

	// Stale debounce tick for epoch=1: must be a no-op.
	_, fetchCmd := a.Update(previewDebounceMsg{key: key1, epoch: 1})
	if fetchCmd != nil {
		fetchCmd() // execute to surface any unexpected GetIssue call
		if len(fake.GetIssueCalls) > 0 {
			t.Errorf("stale debounce tick caused %d GetIssue call(s), want 0", len(fake.GetIssueCalls))
		}
	}

	before := len(fake.GetIssueCalls)

	// Fresh debounce tick for epoch=2: must trigger one GetIssue call.
	_, fetchCmd2 := a.Update(previewDebounceMsg{key: key2, epoch: 2})
	if fetchCmd2 == nil {
		t.Fatal("expected fetch cmd from fresh debounce tick, got nil")
	}
	fetchCmd2()

	after := len(fake.GetIssueCalls)
	if got := after - before; got != 1 {
		t.Errorf("fresh debounce tick caused %d GetIssue call(s), want 1", got)
	}
	if after > 0 && fake.GetIssueCalls[after-1].Key != key2 {
		t.Errorf("GetIssue called with key %q, want %q", fake.GetIssueCalls[after-1].Key, key2)
	}
}

// TestPreviewDebounce_Lapse verifies that when two PreviewRequestMsgs are
// separated by enough time (simulated by dispatching the debounce for the
// first before the second PreviewRequestMsg arrives), both result in a
// GetIssue call.
func TestPreviewDebounce_Lapse(t *testing.T) {
	const key2 = "SUB-2"
	key1 := subKey1

	var issueCalls []string
	fake := &jiratest.FakeClient{T: t}
	fake.GetIssueFunc = func(_ context.Context, key string) (*jira.Issue, error) {
		issueCalls = append(issueCalls, key)
		return &jira.Issue{Key: key}, nil
	}
	fake.GetCommentsFunc = func(_ context.Context, _ string) ([]jira.Comment, error) {
		return nil, nil
	}
	fake.GetChangelogFunc = func(_ context.Context, _ string) ([]jira.ChangelogEntry, error) {
		return nil, nil
	}

	a := newAppWithFake(t, fake)

	// First PreviewRequestMsg - epoch becomes 1.
	_, _ = a.Update(views.PreviewRequestMsg{Key: key1})

	// Debounce fires before the second msg (lapse): epoch=1 still matches.
	_, fetchCmd1 := a.Update(previewDebounceMsg{key: key1, epoch: 1})
	if fetchCmd1 == nil {
		t.Fatal("expected fetch cmd from first debounce tick, got nil")
	}
	fetchCmd1()

	// Second PreviewRequestMsg - epoch becomes 2.
	_, _ = a.Update(views.PreviewRequestMsg{Key: key2})

	// Debounce fires for epoch=2, still matches.
	_, fetchCmd2 := a.Update(previewDebounceMsg{key: key2, epoch: 2})
	if fetchCmd2 == nil {
		t.Fatal("expected fetch cmd from second debounce tick, got nil")
	}
	fetchCmd2()

	if len(issueCalls) != 2 {
		t.Fatalf("expected 2 GetIssue calls, got %d: %v", len(issueCalls), issueCalls)
	}
	if issueCalls[0] != key1 {
		t.Errorf("first GetIssue call key = %q, want %q", issueCalls[0], key1)
	}
	if issueCalls[1] != key2 {
		t.Errorf("second GetIssue call key = %q, want %q", issueCalls[1], key2)
	}
}

// TestPreviewStaleResponse_DroppedWhenEpochAdvanced verifies that a detail
// response carrying an old epoch does not update infoPanel or issueCache.
func TestPreviewStaleResponse_DroppedWhenEpochAdvanced(t *testing.T) {
	const key2 = "SUB-2"
	key1 := subKey1

	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: key2, Summary: "current"})

	a := newAppWithFake(t, fake)
	// Pre-load InfoPanel with a main issue so a stale response that wrongly
	// resets it would flip IssueKey away from mainKey.
	main := &jira.Issue{Key: mainKey, Summary: "main"}
	a.issuesList.SetIssues([]jira.Issue{*main})
	a.infoPanel.SetIssue(main)

	// Simulate previewEpoch at 2 with previewKey = key2 (user is on key2).
	a.previewKey = key2
	a.previewEpoch = 2

	// Stale response for epoch=1 arrives.
	_, _ = a.Update(previewDetailLoadedMsg{
		issue: &jira.Issue{Key: key1, Summary: "stale"},
		epoch: 1,
	})

	// The stale response must not touch the cache or the panels.
	if got := a.infoPanel.IssueKey(); got != mainKey {
		t.Errorf("infoPanel.IssueKey() = %q, want %q (must stay with main)", got, mainKey)
	}
	if got := a.detailView.IssueKey(); got == key1 {
		t.Errorf("detailView updated with stale key %q, want no update", key1)
	}
	if _, ok := a.issueCache[key1]; ok {
		t.Errorf("issueCache populated with stale key %q, want absent", key1)
	}

	// Fresh response for epoch=2 arrives.
	_, _ = a.Update(previewDetailLoadedMsg{
		issue: &jira.Issue{Key: key2, Summary: "current"},
		epoch: 2,
	})

	// A preview response only updates the DetailView + cache, not the
	// InfoPanel (which belongs to the main list issue).
	if got := a.detailView.IssueKey(); got != key2 {
		t.Errorf("detailView.IssueKey() = %q after fresh response, want %q", got, key2)
	}
	if _, ok := a.issueCache[key2]; !ok {
		t.Errorf("issueCache missing key %q after fresh response", key2)
	}
}

// TestNonPreviewFetch_NotAffectedByDebounce verifies that ActRefresh works
// normally regardless of previewEpoch value.
func TestNonPreviewFetch_NotAffectedByDebounce(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: mainKey, Summary: "main"})

	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	a.previewKey = mainKey
	a.previewEpoch = 99

	_, cmd, handled := a.handleIssueAction(ActRefresh)
	if !handled {
		t.Fatal("ActRefresh was not handled")
	}
	if cmd == nil {
		t.Fatal("expected cmd from ActRefresh, got nil")
	}
	msg := cmd()

	loaded, ok := msg.(issueDetailLoadedMsg)
	if !ok {
		t.Fatalf("ActRefresh produced %T, want issueDetailLoadedMsg", msg)
	}
	if loaded.issue == nil || loaded.issue.Key != mainKey {
		t.Errorf("issueDetailLoadedMsg.issue.Key = %q, want %q", loaded.issue.Key, mainKey)
	}

	// issueDetailLoadedMsg (non-preview) must update views without epoch check.
	_, _ = a.handleIssueDetailLoaded(loaded)
	if got := a.infoPanel.IssueKey(); got != mainKey {
		t.Errorf("infoPanel.IssueKey() = %q after ActRefresh, want %q", got, mainKey)
	}
}
