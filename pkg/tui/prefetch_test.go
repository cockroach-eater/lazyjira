package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// TestPrefetchRelated_IncludesParent ensures the parent key is fetched along
// with subtasks and links so that navigating up the hierarchy feels the same
// as navigating into sub-tasks.
func TestPrefetchRelated_IncludesParent(t *testing.T) {
	const parentKey = "PARENT-1"

	var got string
	fake := &jiratest.FakeClient{T: t}
	fake.SearchIssuesFunc = func(_ context.Context, jql string, _, _ int) (*jira.SearchResult, error) {
		got = jql
		return &jira.SearchResult{}, nil
	}
	a := newAppWithFake(t, fake)

	issue := &jira.Issue{
		Key:      mainKey,
		Parent:   &jira.Issue{Key: parentKey},
		Subtasks: []jira.Issue{{Key: subKey1}},
	}

	cmd := a.prefetchRelated(issue)
	if cmd == nil {
		t.Fatal("expected non-nil prefetch cmd")
	}
	cmd()

	if !strings.Contains(got, parentKey) {
		t.Errorf("SearchIssues JQL %q does not contain parent %q", got, parentKey)
	}
	if !strings.Contains(got, subKey1) {
		t.Errorf("SearchIssues JQL %q does not contain subtask %q", got, subKey1)
	}
}

// TestIssueSelectedMsg_PrefetchesRelated ensures selecting a task in the list
// warms the cache with its subtasks, links and parent so navigating into them
// feels instant.
func TestIssueSelectedMsg_PrefetchesRelated(t *testing.T) {
	var got string
	fake := &jiratest.FakeClient{T: t}
	fake.SearchIssuesFunc = func(_ context.Context, jql string, _, _ int) (*jira.SearchResult, error) {
		got = jql
		return &jira.SearchResult{}, nil
	}
	a := newAppWithFake(t, fake)

	issue := &jira.Issue{
		Key:      mainKey,
		Subtasks: []jira.Issue{{Key: subKey1}},
	}

	_, cmd := a.Update(views.IssueSelectedMsg{Issue: issue})
	if cmd == nil {
		t.Fatal("expected a prefetch cmd from IssueSelectedMsg, got nil")
	}
	cmd()

	if !strings.Contains(got, subKey1) {
		t.Errorf("SearchIssues JQL %q does not contain %q", got, subKey1)
	}
}

// TestPreviewSelectedIssue_ReturnsPrefetchCmd ensures the helper used on tab
// switches and focus returns also warms the cache for the now-selected task.
func TestPreviewSelectedIssue_ReturnsPrefetchCmd(t *testing.T) {
	var got string
	fake := &jiratest.FakeClient{T: t}
	fake.SearchIssuesFunc = func(_ context.Context, jql string, _, _ int) (*jira.SearchResult, error) {
		got = jql
		return &jira.SearchResult{}, nil
	}
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{
		Key:      mainKey,
		Subtasks: []jira.Issue{{Key: subKey1}},
	}})

	cmd := a.previewSelectedIssue()
	if cmd == nil {
		t.Fatal("expected a prefetch cmd from previewSelectedIssue, got nil")
	}
	cmd()

	if !strings.Contains(got, subKey1) {
		t.Errorf("SearchIssues JQL %q does not contain %q", got, subKey1)
	}
}
