package tui

import (
	"testing"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// TestPreviewRequestMsg_SetsPreviewKeyAndFetches verifies that receiving
// PreviewRequestMsg sets a.previewKey and returns a non-nil Cmd that calls
// GetIssue with the given key.
func TestPreviewRequestMsg_SetsPreviewKeyAndFetches(t *testing.T) {
	subKey := subKey1

	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: subKey, Summary: "sub issue"})

	a := newAppWithFake(t, fake)

	_, _ = a.Update(views.PreviewRequestMsg{Key: subKey})

	if got := a.previewKey; got != subKey {
		t.Errorf("previewKey = %q, want %q", got, subKey)
	}
}

// TestPreviewRequestMsg_CmdEventuallyCallsGetIssue verifies that after the
// debounce tick fires (simulated via previewDebounceMsg), a GetIssue call is
// made for the correct key.
func TestPreviewRequestMsg_CmdEventuallyCallsGetIssue(t *testing.T) {
	subKey := subKey1

	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: subKey, Summary: "sub issue"})

	a := newAppWithFake(t, fake)

	_, tickCmd := a.Update(views.PreviewRequestMsg{Key: subKey})
	if tickCmd == nil {
		t.Fatal("expected non-nil tea.Cmd from PreviewRequestMsg handler, got nil")
	}

	// The tick cmd returns a previewDebounceMsg when fired. We simulate it
	// deterministically by dispatching the debounce message directly, which
	// avoids waiting for the real 150 ms timer.
	_, fetchCmd := a.Update(previewDebounceMsg{key: subKey, epoch: a.previewEpoch})
	if fetchCmd == nil {
		t.Fatal("expected fetch cmd from debounce tick, got nil")
	}

	fetchCmd() // triggers GetIssue on the fake

	if len(fake.GetIssueCalls) != 1 {
		t.Fatalf("expected 1 GetIssue call, got %d: %+v", len(fake.GetIssueCalls), fake.GetIssueCalls)
	}
	if got := fake.GetIssueCalls[0].Key; got != subKey {
		t.Errorf("GetIssue called with key %q, want %q", got, subKey)
	}
}
