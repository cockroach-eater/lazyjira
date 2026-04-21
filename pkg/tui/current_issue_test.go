package tui

import (
	"context"
	"testing"
	"text/template"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// setupPreviewedSub creates an app with MAIN selected in the list and SUB-1
// cached as the preview. Action handlers should target SUB-1.
func setupPreviewedSub(t *testing.T, fake *jiratest.FakeClient, sub *jira.Issue) *App {
	t.Helper()
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey, Summary: "main summary"}})
	a.previewKey = subKey1
	a.issueCache[subKey1] = sub
	return a
}

// TestEditAction_TargetsPreviewedIssue verifies that pressing the edit action
// while previewing a sub-issue edits the sub's summary, not the main issue.
func TestEditAction_TargetsPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey, Summary: "main summary"}})
	a.previewKey = subKey1
	a.issueCache[subKey1] = &jira.Issue{Key: subKey1, Summary: "sub summary"}
	a.side = sideLeft
	a.leftFocus = focusIssues

	_, _ = a.handleActionEdit()

	if got := a.editContext.issueKey; got != subKey1 {
		t.Errorf("editContext.issueKey = %q, want %q", got, subKey1)
	}
}

// TestCustomCommand_TargetsPreviewedIssue verifies that a custom command's
// template is rendered with the previewed issue's key, not the list cursor.
func TestCustomCommand_TargetsPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey, Summary: "main"}})
	a.previewKey = subKey1
	a.issueCache[subKey1] = &jira.Issue{Key: subKey1, Summary: "sub"}

	tmpl, err := template.New("t").Option("missingkey=error").Parse("{{.Key}}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rc := config.ResolvedCustomCommand{
		Key:      "x",
		Scopes:   config.ScopeIssue,
		Contexts: []config.Context{config.CtxIssues},
		Template: tmpl,
	}

	data, ok := a.buildCommandData(rc)
	if !ok {
		t.Fatal("buildCommandData returned ok=false")
	}
	scope, ok := data.(issueScopeData)
	if !ok {
		t.Fatalf("buildCommandData returned %T, want issueScopeData", data)
	}
	if scope.Key != subKey1 {
		t.Errorf("scope.Key = %q, want %q", scope.Key, subKey1)
	}
}

// TestCurrentIssue_StubWhenPreviewKeyUncached covers the race between a
// preview request firing and the fetch populating the cache. During that
// window the user might press an issue-scoped key; the action must target
// the preview key (shown on screen) rather than the list cursor.
func TestCurrentIssue_StubWhenPreviewKeyUncached(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	a.previewKey = subKey1 // no cache entry yet

	cur := a.currentIssue()
	if cur == nil {
		t.Fatal("currentIssue() returned nil with previewKey set")
	}
	if cur.Key != subKey1 {
		t.Errorf("currentIssue().Key = %q, want %q (stub for previewKey)", cur.Key, subKey1)
	}
}

// TestCurrentIssue_FallsBackToListWhenNoPreview covers the steady-state:
// before any preview is active, issue-scoped actions operate on the list
// selection.
func TestCurrentIssue_FallsBackToListWhenNoPreview(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := newAppWithFake(t, fake)
	a.issuesList.SetIssues([]jira.Issue{{Key: mainKey}})
	// previewKey intentionally empty

	cur := a.currentIssue()
	if cur == nil {
		t.Fatal("currentIssue() returned nil with list selection present")
	}
	if cur.Key != mainKey {
		t.Errorf("currentIssue().Key = %q, want %q", cur.Key, mainKey)
	}
}

// TestEditAction_OnInfoSubTab_EditsPreviewedIssueSummary: pressing edit while
// the info panel has focus on the Sub or Lnk tab edits the summary of the
// issue currently previewed, not the main list issue.
func TestEditAction_OnInfoSubTab_EditsPreviewedIssueSummary(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1, Summary: "sub summary"})
	a.side = sideLeft
	a.leftFocus = focusInfo
	// Main issue has a subtask so the Sub tab has an item.
	main := &jira.Issue{Key: mainKey, Subtasks: []jira.Issue{{Key: subKey1}}}
	a.infoPanel.SetIssue(main)
	a.infoPanel.NextTab() // Fields -> Links
	a.infoPanel.NextTab() // Links  -> Subtasks

	_, _ = a.handleActionEdit()

	if got := a.editContext.issueKey; got != subKey1 {
		t.Errorf("editContext.issueKey = %q, want %q", got, subKey1)
	}
	if a.editContext.kind != editSummary {
		t.Errorf("editContext.kind = %v, want editSummary", a.editContext.kind)
	}
}

// TestEditAction_Description_TargetsPreviewedIssue: pressing edit from the
// detail view (side=right, Details tab) edits the previewed issue's
// description.
func TestEditAction_Description_TargetsPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1, Description: "sub desc"})
	a.side = sideRight
	a.detailView.SetActiveTab(views.TabDetails)

	_, _ = a.handleActionEdit()

	if got := a.editContext.issueKey; got != subKey1 {
		t.Errorf("editContext.issueKey = %q, want %q", got, subKey1)
	}
	if a.editContext.kind != editDesc {
		t.Errorf("editContext.kind = %v, want editDesc", a.editContext.kind)
	}
}

// TestTransitionAction_TargetsPreviewedIssue: ActTransition fetches
// transitions for the previewed issue.
func TestTransitionAction_TargetsPreviewedIssue(t *testing.T) {
	var calledKey string
	fake := &jiratest.FakeClient{T: t}
	fake.GetTransitionsFunc = func(_ context.Context, key string) ([]jira.Transition, error) {
		calledKey = key
		return nil, nil
	}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1})

	_, cmd, handled := a.handleIssueAction(ActTransition)
	if !handled {
		t.Fatal("ActTransition not handled")
	}
	if cmd == nil {
		t.Fatal("expected cmd, got nil")
	}
	cmd()

	if calledKey != subKey1 {
		t.Errorf("GetTransitions called with %q, want %q", calledKey, subKey1)
	}
}

// TestAssigneeAction_TargetsPreviewedIssue: ActAssignee fetches users
// scoped to the previewed issue.
func TestAssigneeAction_TargetsPreviewedIssue(t *testing.T) {
	var calledProject string
	fake := &jiratest.FakeClient{T: t}
	fake.GetUsersFunc = func(_ context.Context, projectKey string) ([]jira.User, error) {
		calledProject = projectKey
		return nil, nil
	}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1})
	a.projectKey = "SUB"

	_, cmd, handled := a.handleIssueAction(ActAssignee)
	if !handled {
		t.Fatal("ActAssignee not handled")
	}
	if cmd == nil {
		t.Fatal("expected cmd, got nil")
	}
	cmd()

	if a.onSelect == nil {
		t.Error("onSelect was not installed (handler needs a previewed issue)")
	}
	if calledProject != "SUB" {
		t.Errorf("GetUsers called for project %q, want SUB", calledProject)
	}
}

// TestCommentsAction_TargetsPreviewedIssue: ActComments opens the detail
// view's Comments tab and shows the previewed issue.
func TestCommentsAction_TargetsPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1, Summary: "cached"})
	a.detailView.SetIssue(a.issueCache[subKey1])

	_, cmd, handled := a.handleIssueAction(ActComments)
	if !handled {
		t.Fatal("ActComments not handled")
	}
	if cmd != nil {
		// No-op since the previewed issue is cached.
		cmd()
	}
	if a.detailView.ActiveTab() != views.TabComments {
		t.Errorf("detailView tab = %v, want Comments", a.detailView.ActiveTab())
	}
	if a.detailView.IssueKey() != subKey1 {
		t.Errorf("detailView.IssueKey() = %q, want %q", a.detailView.IssueKey(), subKey1)
	}
}

// TestNewCommentAction_TargetsPreviewedIssue: ActNew in the Comments tab
// opens the editor bound to the previewed issue.
func TestNewCommentAction_TargetsPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1})
	a.side = sideRight
	a.detailView.SetActiveTab(views.TabComments)

	_, _, handled := a.handleIssueAction(ActNew)
	if !handled {
		t.Fatal("ActNew not handled")
	}
	if got := a.editContext.issueKey; got != subKey1 {
		t.Errorf("editContext.issueKey = %q, want %q", got, subKey1)
	}
	if a.editContext.kind != editCommentNew {
		t.Errorf("editContext.kind = %v, want editCommentNew", a.editContext.kind)
	}
}

// TestDuplicateIssueAction_TargetsPreviewedIssue: starting a duplicate uses
// the previewed issue as the source.
func TestDuplicateIssueAction_TargetsPreviewedIssue(t *testing.T) {
	fake := &jiratest.FakeClient{T: t}
	fake.GetIssueTypesFunc = func(_ context.Context, _ string) ([]jira.IssueType, error) {
		return nil, nil
	}
	a := setupPreviewedSub(t, fake, &jira.Issue{Key: subKey1, Summary: "sub summary"})
	a.side = sideLeft
	a.leftFocus = focusIssues
	a.projectKey = "SUB"

	_, _, handled := a.handleIssueAction(ActDuplicateIssue)
	if !handled {
		t.Fatal("ActDuplicateIssue not handled")
	}
	if a.createCtx.duplicateFrom == nil {
		t.Fatal("duplicateFrom not set")
	}
	if got := a.createCtx.duplicateFrom.Key; got != subKey1 {
		t.Errorf("duplicateFrom.Key = %q, want %q", got, subKey1)
	}
}
