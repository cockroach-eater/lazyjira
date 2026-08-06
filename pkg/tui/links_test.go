package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

const (
	linkKey1 = testProject + "-1"
	linkKey2 = testProject + "-2"
)

var blocksType = &jira.IssueLinkType{
	ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks",
}

// newLinkApp returns an app whose Info panel shows an issue carrying one link
// and one subtask.
func newLinkApp(t *testing.T) *App {
	t.Helper()
	app := newTestApp()
	app.client = &jiratest.FakeClient{T: t}
	app.infoPanel = views.NewInfoPanel()
	app.statusPanel = views.NewStatusPanel(testProject, "", "")
	app.keymap = DefaultKeymap()
	app.issueCache = map[string]*jira.Issue{}
	app.childrenCache = map[string][]jira.Issue{}
	logFlag := false
	app.logFlag = &logFlag

	issue := &jira.Issue{
		Key:     linkKey1,
		Summary: testSummary,
		IssueLinks: []jira.IssueLink{
			{ID: "500", Type: blocksType, OutwardIssue: &jira.Issue{Key: linkKey2, Summary: "Blocked one"}},
		},
		Subtasks: []jira.Issue{{Key: testProject + "-9", Summary: "A subtask"}},
	}
	app.issuesList.SetIssues([]jira.Issue{*issue, {Key: linkKey2, Summary: "Blocked one"}})
	app.issueCache[linkKey1] = issue
	app.previewKey = linkKey1
	app.infoPanel.SetIssue(issue)
	app.side = sideLeft
	app.leftFocus = focusInfo
	return app
}

func TestLinkTypesModalListsBothDirections(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)

	app.handleIssueLinkTypesLoaded(issueLinkTypesLoadedMsg{types: []jira.IssueLinkType{*blocksType}})

	if !app.modal.IsVisible() {
		t.Fatal("the link type modal was not shown")
	}
	items := app.modal.Items()
	if len(items) != 2 {
		t.Fatalf("items = %d, want one per direction: %+v", len(items), items)
	}
	if items[0].Label != "blocks" || items[1].Label != "is blocked by" {
		t.Errorf("labels = %q, %q", items[0].Label, items[1].Label)
	}
}

func TestLinkTypesModalCollapsesSymmetricTypes(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	relates := jira.IssueLinkType{Name: "Relates", Inward: "relates to", Outward: "relates to"}

	app.handleIssueLinkTypesLoaded(issueLinkTypesLoadedMsg{types: []jira.IssueLinkType{relates}})

	if got := len(app.modal.Items()); got != 1 {
		t.Errorf("items = %d, want a symmetric type listed once", got)
	}
}

func TestLinkTypesEmptyReportsAnError(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.onSelect = func(components.ModalItem) tea.Cmd { return nil }

	app.handleIssueLinkTypesLoaded(issueLinkTypesLoadedMsg{})

	if app.modal.IsVisible() {
		t.Error("an empty type list should not open a modal")
	}
	if app.onSelect != nil {
		t.Error("the pending callback was left dangling")
	}
}

func TestLinkTypeSelectionOpensTargetPrompt(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)

	app.makeLinkTypeCallback(linkKey1)(components.ModalItem{ID: "Blocks|out", Label: "blocks"})

	if !app.inputModal.IsVisible() {
		t.Fatal("the target prompt was not shown")
	}
	if app.editContext.kind != editLinkTarget || app.editContext.fieldID != "Blocks|out" {
		t.Errorf("editContext = %+v", app.editContext)
	}
	if !app.inputModal.HasHints() {
		t.Error("the prompt offers no candidate keys from the list")
	}
}

func TestLinkCandidatesExcludeTheIssueItself(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.issuesList.SetIssues([]jira.Issue{{Key: linkKey1}, {Key: linkKey2}})

	got := app.linkCandidateKeys(linkKey1)

	if len(got) != 1 || got[0] != linkKey2 {
		t.Errorf("candidates = %v, want everything but the issue itself", got)
	}
}

func TestApplyLinkTargetOutwardDirection(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	fake := &jiratest.FakeClient{T: t}
	fake.CreateIssueLinkFunc = func(context.Context, string, string, string) error { return nil }
	app.client = fake

	ctx := editCtx{kind: editLinkTarget, issueKey: linkKey1, fieldID: "Blocks|out"}
	cmd := app.applyLinkTarget(ctx, linkKey2)
	if cmd == nil {
		t.Fatal("no command was produced")
	}
	cmd()

	call := fake.CreateIssueLinkCalls[0]
	if call.TypeName != "Blocks" {
		t.Errorf("type = %q", call.TypeName)
	}
	// "PLAT-1 blocks PLAT-2": the current issue is the outward side.
	if call.OutwardKey != linkKey1 || call.InwardKey != linkKey2 {
		t.Errorf("outward = %q, inward = %q", call.OutwardKey, call.InwardKey)
	}
}

func TestApplyLinkTargetInwardDirectionSwapsSides(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	fake := &jiratest.FakeClient{T: t}
	fake.CreateIssueLinkFunc = func(context.Context, string, string, string) error { return nil }
	app.client = fake

	ctx := editCtx{kind: editLinkTarget, issueKey: linkKey1, fieldID: "Blocks|in"}
	app.applyLinkTarget(ctx, linkKey2)()

	call := fake.CreateIssueLinkCalls[0]
	// "PLAT-1 is blocked by PLAT-2": the current issue is now the inward side.
	if call.InwardKey != linkKey1 || call.OutwardKey != linkKey2 {
		t.Errorf("inward = %q, outward = %q", call.InwardKey, call.OutwardKey)
	}
}

func TestApplyLinkTargetRejectsMalformedKey(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)

	ctx := editCtx{kind: editLinkTarget, issueKey: linkKey1, fieldID: "Blocks|out"}
	cmd := app.applyLinkTarget(ctx, "not a key")
	if cmd == nil {
		t.Fatal("a malformed key produced no result")
	}
	msg, ok := cmd().(errorMsg)
	if !ok {
		t.Fatalf("message = %T, want errorMsg", cmd())
	}
	if !strings.Contains(msg.err.Error(), "invalid issue key") {
		t.Errorf("error = %v", msg.err)
	}
}

func TestApplyLinkTargetIgnoresEmptyInput(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)

	ctx := editCtx{kind: editLinkTarget, issueKey: linkKey1, fieldID: "Blocks|out"}
	if cmd := app.applyLinkTarget(ctx, "  "); cmd != nil {
		t.Error("an empty target created a link")
	}
}

func TestDeleteLinkAsksBeforeDeleting(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	fake := &jiratest.FakeClient{T: t}
	fake.DeleteIssueLinkFunc = func(context.Context, string) error { return nil }
	app.client = fake
	app.infoPanel.SetActiveTab(views.InfoTabLinks)

	app.startDeleteSelection()

	if !app.modal.IsVisible() {
		t.Fatal("no confirmation was shown")
	}
	if len(fake.DeleteIssueLinkCalls) != 0 {
		t.Fatal("the link was deleted before the user confirmed")
	}
	if title := app.modal.Title(); !strings.Contains(title, linkKey2) {
		t.Errorf("prompt = %q, want it to name the linked issue", title)
	}

	app.onSelect(components.ModalItem{ID: confirmYes})()

	if len(fake.DeleteIssueLinkCalls) != 1 || fake.DeleteIssueLinkCalls[0].LinkID != "500" {
		t.Errorf("DeleteIssueLink calls = %+v", fake.DeleteIssueLinkCalls)
	}
}

func TestDeclinedConfirmationDeletesNothing(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.infoPanel.SetActiveTab(views.InfoTabLinks)
	app.startDeleteSelection()

	if cmd := app.onSelect(components.ModalItem{ID: "__no__"}); cmd != nil {
		t.Error("answering No still ran the delete")
	}
}

func TestDeleteSubtaskAsksBeforeDeleting(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	fake := &jiratest.FakeClient{T: t}
	fake.DeleteIssueFunc = func(context.Context, string, bool) error { return nil }
	app.client = fake
	app.infoPanel.SetActiveTab(views.InfoTabSubtasks)

	app.startDeleteSelection()
	if !app.modal.IsVisible() {
		t.Fatal("no confirmation was shown")
	}
	app.onSelect(components.ModalItem{ID: confirmYes})()

	if len(fake.DeleteIssueCalls) != 1 {
		t.Fatalf("DeleteIssue calls = %d", len(fake.DeleteIssueCalls))
	}
	if got := fake.DeleteIssueCalls[0].Key; got != testProject+"-9" {
		t.Errorf("deleted %q", got)
	}
}

func TestDeleteIgnoredOnTheFieldsTab(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.infoPanel.SetActiveTab(views.InfoTabFields)

	app.startDeleteSelection()

	if app.modal.IsVisible() {
		t.Error("the Fields tab has nothing to delete")
	}
}

func TestDeleteIgnoredOutsideTheInfoPanel(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.leftFocus = focusIssues
	app.infoPanel.SetActiveTab(views.InfoTabLinks)

	app.startDeleteSelection()

	if app.modal.IsVisible() {
		t.Error("delete fired with the Info panel unfocused")
	}
}

func TestLinkChangedRefetchesTheIssue(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	fake := &jiratest.FakeClient{T: t}
	fake.GetIssueFunc = func(_ context.Context, key string) (*jira.Issue, error) {
		return &jira.Issue{Key: key}, nil
	}
	fake.GetCommentsFunc = func(context.Context, string) ([]jira.Comment, error) { return nil, nil }
	fake.GetChangelogFunc = func(context.Context, string) ([]jira.ChangelogEntry, error) { return nil, nil }
	app.client = fake

	_, cmd := app.handleIssueLinkChanged(issueLinkChangedMsg{issueKey: linkKey1})

	if _, stale := app.issueCache[linkKey1]; stale {
		t.Error("the stale issue is still cached; the refetch would be served from it")
	}
	if cmd == nil {
		t.Fatal("no refetch was scheduled")
	}
	cmd()
	if len(fake.GetIssueCalls) == 0 {
		t.Error("the issue was not refetched")
	}
}

func TestIssueDeletedClearsCachesAndReloads(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	sub := testProject + "-9"
	app.issueCache[sub] = &jira.Issue{Key: sub}
	app.childrenCache[linkKey1] = []jira.Issue{{Key: sub}}
	app.previewKey = sub

	app.handleIssueDeleted(issueDeletedMsg{issueKey: sub, parentKey: linkKey1})

	if _, stale := app.issueCache[sub]; stale {
		t.Error("the deleted issue is still cached")
	}
	if _, stale := app.childrenCache[linkKey1]; stale {
		t.Error("the parent's children cache was not invalidated")
	}
	if app.previewKey != "" {
		t.Errorf("previewKey = %q, want it cleared for a deleted issue", app.previewKey)
	}
}

// The prompt must repeat the phrasing of the Lnk row it acts on, which pairs
// outwardIssue with the outward phrase.
func TestLinkedIssueRelationMatchesTheRenderedRow(t *testing.T) {
	t.Parallel()
	outward := &jira.IssueLink{Type: blocksType, OutwardIssue: &jira.Issue{Key: linkKey2}}
	if got := linkedIssueRelation(outward); got != "blocks" {
		t.Errorf("relation = %q", got)
	}

	inward := &jira.IssueLink{Type: blocksType, InwardIssue: &jira.Issue{Key: linkKey2}}
	if got := linkedIssueRelation(inward); got != "is blocked by" {
		t.Errorf("relation = %q", got)
	}
	if got := linkedIssueRelation(nil); got != "" {
		t.Errorf("relation = %q, want empty for a nil link", got)
	}
}

// Entering the Lnk or Sub tab previews the related issue in the detail panel,
// so currentIssue points at the neighbour. A new link must still attach to the
// issue whose links are on screen.
func TestLinkAttachesToTheIssueTheInfoPanelShows(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.infoPanel.SetActiveTab(views.InfoTabLinks)
	app.previewKey = linkKey2
	app.issueCache[linkKey2] = &jira.Issue{Key: linkKey2}

	if got := app.linkSubject(); got != linkKey1 {
		t.Errorf("link subject = %q, want the issue the Info panel is showing (%s)", got, linkKey1)
	}
}

func TestLinkSubjectIsTheOpenIssueElsewhere(t *testing.T) {
	t.Parallel()
	app := newLinkApp(t)
	app.leftFocus = focusIssues
	app.previewKey = linkKey2
	app.issueCache[linkKey2] = &jira.Issue{Key: linkKey2}

	if got := app.linkSubject(); got != linkKey2 {
		t.Errorf("link subject = %q, want the issue on screen (%s)", got, linkKey2)
	}
}
