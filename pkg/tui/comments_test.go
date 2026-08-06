package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

func newCommentApp(t *testing.T) (*App, *jira.Issue) {
	t.Helper()
	app := newTestApp()
	app.client = &jiratest.FakeClient{T: t}
	app.infoPanel = views.NewInfoPanel()
	app.statusPanel = views.NewStatusPanel(testProject, "", "")
	app.keymap = DefaultKeymap()
	app.issueCache = map[string]*jira.Issue{}
	logFlag := false
	app.logFlag = &logFlag

	issue := &jira.Issue{
		Key:     testKey,
		Summary: testSummary,
		Comments: []jira.Comment{
			{ID: "1", Author: &jira.User{DisplayName: "Alice"}, Body: "First thought"},
			{ID: "2", Author: &jira.User{DisplayName: "Bob"}, Body: "line one\nline two"},
		},
	}
	app.issuesList.SetIssues([]jira.Issue{*issue})
	app.issueCache[testKey] = issue
	app.previewKey = testKey
	app.side = sideRight
	app.detailView.SetSize(80, 24)
	app.detailView.SetIssue(issue)
	app.detailView.SetActiveTab(views.TabComments)
	return app, issue
}

func TestNewCommentOpensPopup(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)

	_, cmd, handled := app.startNewComment()

	if !handled {
		t.Fatal("the action was not handled")
	}
	if cmd != nil {
		t.Error("opening the popup should not run a command")
	}
	if !app.textModal.IsVisible() {
		t.Fatal("the popup is not visible")
	}
	if got := app.textModal.Text(); got != "" {
		t.Errorf("prefill = %q, want an empty buffer", got)
	}
	if app.editContext.kind != editCommentNew || app.editContext.issueKey != testKey {
		t.Errorf("editContext = %+v", app.editContext)
	}
}

func TestNewCommentIgnoredOutsideCommentsTab(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)
	app.detailView.SetActiveTab(views.TabDetails)

	app.startNewComment()

	if app.textModal.IsVisible() {
		t.Error("the popup opened while the Details tab was active")
	}
}

func TestReplyQuotesSelectedComment(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)

	_, _, handled := app.startReplyComment()

	if !handled {
		t.Fatal("the action was not handled")
	}
	got := app.textModal.Text()
	if !strings.HasPrefix(got, "@Alice\n") {
		t.Errorf("reply = %q, want it to mention the comment author", got)
	}
	if strings.Contains(got, "@Alice ") {
		t.Errorf("reply = %q: a mention token must not contain spaces", got)
	}
	if !strings.Contains(got, "> First thought") {
		t.Errorf("reply = %q, want the comment quoted", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("reply = %q, want a blank line for the answer", got)
	}
}

func TestReplyWithoutSelectedCommentIsNoOp(t *testing.T) {
	t.Parallel()
	app, issue := newCommentApp(t)
	issue.Comments = nil
	app.detailView.SetIssue(issue)
	app.detailView.SetActiveTab(views.TabComments)

	app.startReplyComment()

	if app.textModal.IsVisible() {
		t.Error("a reply popup opened with no comment selected")
	}
}

func TestReplyPrefillTruncatesLongComments(t *testing.T) {
	t.Parallel()
	cmt := &jira.Comment{
		Author: &jira.User{DisplayName: "Carol"},
		Body:   "a\nb\nc\nd\ne\nf",
	}

	got := replyPrefill(cmt)

	if strings.Count(got, "\n> ") < replyQuoteLines-1 {
		t.Errorf("prefill = %q, want %d quoted lines", got, replyQuoteLines)
	}
	if !strings.Contains(got, "> …") {
		t.Errorf("prefill = %q, want an ellipsis marking the cut", got)
	}
	if strings.Contains(got, "> f") {
		t.Errorf("prefill = %q, want the tail dropped", got)
	}
}

// A display name with spaces has to be written as a single underscore token,
// or the mention scanner resolves only the first word and leaves the rest as
// literal text ("@Alice Chen" became "@Alice Chen Chen").
func TestReplyPrefillJoinsMultiWordNames(t *testing.T) {
	t.Parallel()
	got := replyPrefill(&jira.Comment{
		Author: &jira.User{DisplayName: "Alice Chen"},
		Body:   "hi",
	})

	if !strings.HasPrefix(got, "@Alice_Chen\n") {
		t.Errorf("prefill = %q, want @Alice_Chen", got)
	}
}

func TestMentionTokenCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	if got := mentionToken("  Ana   Maria  Ruiz "); got != "Ana_Maria_Ruiz" {
		t.Errorf("mentionToken = %q", got)
	}
}

func TestReplyPrefillWithoutAuthor(t *testing.T) {
	t.Parallel()
	got := replyPrefill(&jira.Comment{Body: "orphan"})

	if strings.HasPrefix(got, "@") {
		t.Errorf("prefill = %q, want no mention when the author is unknown", got)
	}
	if !strings.Contains(got, "> orphan") {
		t.Errorf("prefill = %q, want the body quoted", got)
	}
}

func TestTextAreaConfirmedSubmitsComment(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)
	app.converter = BuiltinConverter{}

	fake := &jiratest.FakeClient{T: t}
	fake.AddCommentFunc = func(_ context.Context, key string, body any) (*jira.Comment, error) {
		return &jira.Comment{ID: "9", Body: body.(string)}, nil
	}
	app.client = fake
	app.startNewComment()

	_, cmd := app.handleTextAreaConfirmed("looks good to me")
	if cmd == nil {
		t.Fatal("confirming the popup did not submit the comment")
	}
	if msg, ok := cmd().(errorMsg); ok {
		t.Fatalf("submit failed: %v", msg.err)
	}

	if len(fake.AddCommentCalls) != 1 {
		t.Fatalf("AddComment calls = %d, want 1", len(fake.AddCommentCalls))
	}
	call := fake.AddCommentCalls[0]
	if call.Key != testKey {
		t.Errorf("comment posted on %q, want %s", call.Key, testKey)
	}
	if body, _ := call.Body.(string); body != "looks good to me" {
		t.Errorf("body = %v, want the typed text", call.Body)
	}
}

func TestTextAreaConfirmedIgnoresBlankText(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)
	app.startNewComment()

	_, cmd := app.handleTextAreaConfirmed("   \n  ")

	if cmd != nil {
		t.Error("a blank comment was submitted")
	}
	if app.editContext.kind != editNone {
		t.Errorf("editContext kind = %v, want it cleared", app.editContext.kind)
	}
}

func TestTextAreaCancelClearsEditContext(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)
	app.startNewComment()

	app.Update(components.TextAreaCancelledMsg{})

	if app.editContext.kind != editNone {
		t.Errorf("editContext kind = %v, want it cleared on cancel", app.editContext.kind)
	}
}

func TestTextAreaHandoffKeepsEditContext(t *testing.T) {
	t.Parallel()
	app, _ := newCommentApp(t)
	app.startNewComment()

	_, cmd := app.handleTextAreaHandoff("draft body")

	if cmd == nil {
		t.Fatal("the handoff did not launch the editor")
	}
	if app.editContext.kind != editCommentNew {
		t.Errorf("editContext kind = %v, want it preserved across the handoff", app.editContext.kind)
	}
}
