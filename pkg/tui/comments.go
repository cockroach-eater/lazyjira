package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

// replyQuoteLines is how much of the quoted comment a reply carries. Long
// comments are cut so the popup opens on the text being written, not on the
// quote.
const replyQuoteLines = 4

// startNewComment opens the comment popup for the issue on screen.
func (a *App) startNewComment() (tea.Model, tea.Cmd, bool) {
	cur := a.currentIssue()
	if cur == nil || a.side != sideRight || a.detailView.ActiveTab() != views.TabComments {
		return a, nil, true
	}
	a.editContext = editCtx{kind: editCommentNew, issueKey: cur.Key}
	a.textModal.Show("New comment on "+cur.Key, "")
	return a, nil, true
}

// startReplyComment opens the comment popup prefilled with a quote of the
// selected comment. Jira has no threaded replies, so a reply is a normal
// comment that quotes and mentions the person being answered.
func (a *App) startReplyComment() (tea.Model, tea.Cmd, bool) {
	cur := a.currentIssue()
	if cur == nil || a.side != sideRight || a.detailView.ActiveTab() != views.TabComments {
		return a, nil, true
	}
	cmt := a.detailView.SelectedComment()
	if cmt == nil {
		return a, nil, true
	}
	a.editContext = editCtx{kind: editCommentNew, issueKey: cur.Key}
	a.textModal.Show("Reply on "+cur.Key, replyPrefill(cmt))
	return a, nil, true
}

// replyPrefill builds the quoted body of a reply: an @-mention of the author
// followed by their comment as a markdown blockquote.
func replyPrefill(cmt *jira.Comment) string {
	var b strings.Builder
	if cmt.Author != nil && cmt.Author.DisplayName != "" {
		b.WriteString("@" + mentionToken(cmt.Author.DisplayName) + "\n")
	}

	lines := strings.Split(strings.TrimRight(cmt.Body, "\n"), "\n")
	truncated := false
	if len(lines) > replyQuoteLines {
		lines = lines[:replyQuoteLines]
		truncated = true
	}
	for _, line := range lines {
		b.WriteString("> " + line + "\n")
	}
	if truncated {
		b.WriteString("> …\n")
	}
	b.WriteString("\n")
	return b.String()
}

// mentionToken renders a display name as a single @-token. The mention scanner
// only reads one word, and turns underscores back into spaces when matching,
// so "Alice Chen" has to be written "Alice_Chen" to resolve as a whole name.
func mentionToken(displayName string) string {
	return strings.Join(strings.Fields(displayName), "_")
}

// handleTextAreaConfirmed submits what was typed in the comment popup through
// the same pipeline the $EDITOR flow uses, so mentions and ADF conversion
// behave identically.
func (a *App) handleTextAreaConfirmed(text string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(text) == "" {
		a.editContext = editCtx{}
		return a, nil
	}
	*a.logFlag = true
	return a, a.applyEdit(text)
}

// handleTextAreaHandoff reopens the popup's content in $EDITOR, keeping the
// edit context so the result still lands on the right comment.
func (a *App) handleTextAreaHandoff(text string) (tea.Model, tea.Cmd) {
	return a, launchEditor(text, ".md")
}
