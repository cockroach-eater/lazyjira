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
	const subKey = "SUB-1"

	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: subKey, Summary: "sub issue"})

	a := newAppWithFake(t, fake)

	_, _ = a.Update(views.PreviewRequestMsg{Key: subKey})

	if got := a.previewKey; got != subKey {
		t.Errorf("previewKey = %q, want %q", got, subKey)
	}
}

// TestPreviewRequestMsg_CmdCallsGetIssue verifies the returned Cmd
// triggers a GetIssue call with the correct key.
func TestPreviewRequestMsg_CmdCallsGetIssue(t *testing.T) {
	const subKey = "SUB-1"

	fake := &jiratest.FakeClient{T: t}
	stubFullIssueFetch(fake, &jira.Issue{Key: subKey, Summary: "sub issue"})

	a := newAppWithFake(t, fake)

	_, cmd := a.Update(views.PreviewRequestMsg{Key: subKey})
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from PreviewRequestMsg handler, got nil")
	}

	cmd() // execute the command to trigger the fake calls

	if len(fake.GetIssueCalls) != 1 {
		t.Fatalf("expected 1 GetIssue call, got %d: %+v", len(fake.GetIssueCalls), fake.GetIssueCalls)
	}
	if got := fake.GetIssueCalls[0].Key; got != subKey {
		t.Errorf("GetIssue called with key %q, want %q", got, subKey)
	}
}
