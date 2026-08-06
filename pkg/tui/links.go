package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

const (
	// linkOutward marks the direction in which the current issue is the
	// subject of the relation ("this blocks that").
	linkOutward = "out"
	linkInward  = "in"

	confirmYes = "__yes__"
)

// startLinkIssue asks for a link type, then for the issue to link to.
func (a *App) startLinkIssue() (tea.Model, tea.Cmd, bool) {
	subject := a.linkSubject()
	if subject == "" {
		return a, nil, true
	}
	a.onSelect = a.makeLinkTypeCallback(subject)
	*a.logFlag = true
	return a, fetchIssueLinkTypes(a.client), true
}

// linkSubject is the issue a new link attaches to. With the Info panel focused
// that is the issue it is showing: entering its Lnk or Sub tab previews the
// related issue in the detail panel, so currentIssue would attach the link to
// the neighbour the user is merely looking at.
func (a *App) linkSubject() string {
	if a.side == sideLeft && a.leftFocus == focusInfo {
		if key := a.infoPanel.IssueKey(); key != "" {
			return key
		}
	}
	if cur := a.currentIssue(); cur != nil {
		return cur.Key
	}
	return ""
}

// handleIssueLinkTypesLoaded lists each link type once per direction, since
// "blocks" and "is blocked by" are the same type read from opposite ends.
func (a *App) handleIssueLinkTypesLoaded(msg issueLinkTypesLoadedMsg) (tea.Model, tea.Cmd) {
	items := make([]components.ModalItem, 0, len(msg.types)*2)
	for _, t := range msg.types {
		items = append(items, components.ModalItem{
			ID:    t.Name + "|" + linkOutward,
			Label: t.Outward,
			Hint:  t.Name,
		})
		// A symmetric type ("relates to" both ways) needs only one row.
		if !strings.EqualFold(t.Inward, t.Outward) {
			items = append(items, components.ModalItem{
				ID:    t.Name + "|" + linkInward,
				Label: t.Inward,
				Hint:  t.Name,
			})
		}
	}
	if len(items) == 0 {
		a.onSelect = nil
		a.statusPanel.SetError("no issue link types available")
		return a, nil
	}
	a.modal.Show("Link type", items)
	return a, nil
}

// makeLinkTypeCallback turns the chosen link type into the second prompt: the
// key of the issue to link to.
func (a *App) makeLinkTypeCallback(issueKey string) onSelectFunc {
	return func(item components.ModalItem) tea.Cmd {
		if !strings.Contains(item.ID, "|") {
			return nil
		}
		// The chosen type and direction ride along in fieldID until the
		// target key comes back from the input modal.
		a.editContext = editCtx{kind: editLinkTarget, issueKey: issueKey, fieldID: item.ID}
		a.inputModal.Show(item.Label+" …", "")
		a.inputModal.SetHints(a.linkCandidateKeys(issueKey))
		return nil
	}
}

// linkCandidateKeys offers the issues already on screen as link targets, so
// the common case needs no typing.
func (a *App) linkCandidateKeys(exclude string) []string {
	issues := a.issuesList.CurrentIssues()
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue.Key == exclude {
			continue
		}
		keys = append(keys, issue.Key)
	}
	return keys
}

// applyLinkTarget creates the link once the target key is known. The direction
// chosen decides which side of the relation the current issue sits on.
func (a *App) applyLinkTarget(ctx editCtx, target string) tea.Cmd {
	target = strings.ToUpper(strings.TrimSpace(target))
	if target == "" {
		return nil
	}
	if !parentKeyRegex.MatchString(target) {
		return errCmd("invalid issue key %q (expected PROJ-123)", target)
	}
	typeName, direction, ok := strings.Cut(ctx.fieldID, "|")
	if !ok {
		return nil
	}

	inward, outward := target, ctx.issueKey
	if direction == linkInward {
		inward, outward = ctx.issueKey, target
	}
	*a.logFlag = true
	return createIssueLink(a.client, typeName, inward, outward, ctx.issueKey)
}

// startDeleteSelection deletes whatever the cursor is on: a link in the Lnk
// tab, a subtask in the Sub tab. Both ask first.
func (a *App) startDeleteSelection() (tea.Model, tea.Cmd, bool) {
	if a.side != sideLeft || a.leftFocus != focusInfo {
		return a, nil, true
	}
	switch a.infoPanel.ActiveTab() {
	case views.InfoTabLinks:
		return a.confirmDeleteLink()
	case views.InfoTabSubtasks:
		return a.confirmDeleteSubtask()
	case views.InfoTabFields:
	}
	return a, nil, true
}

func (a *App) confirmDeleteLink() (tea.Model, tea.Cmd, bool) {
	link := a.infoPanel.SelectedLink()
	issue := a.infoPanel.SelectedLinkIssue()
	if link == nil || link.ID == "" || issue == nil {
		return a, nil, true
	}
	owner := a.infoPanel.IssueKey()
	linkID := link.ID

	question := "Remove link to " + issue.Key + "?"
	if relation := linkedIssueRelation(link); relation != "" {
		question = "Remove \"" + relation + " " + issue.Key + "\"?"
	}

	a.confirm(question, func() tea.Cmd {
		*a.logFlag = true
		return deleteIssueLink(a.client, linkID, owner)
	})
	return a, nil, true
}

func (a *App) confirmDeleteSubtask() (tea.Model, tea.Cmd, bool) {
	sub := a.infoPanel.SelectedSubtaskIssue()
	if sub == nil {
		return a, nil, true
	}
	parent := a.infoPanel.IssueKey()
	key := sub.Key

	a.confirm("Delete "+key+" permanently?", func() tea.Cmd {
		*a.logFlag = true
		return deleteIssue(a.client, key, parent)
	})
	return a, nil, true
}

// confirm shows a two-option modal and runs onYes only if the user picks Yes.
// Destructive actions never run straight off a keypress.
func (a *App) confirm(question string, onYes func() tea.Cmd) {
	a.onSelect = func(item components.ModalItem) tea.Cmd {
		if item.ID != confirmYes {
			return nil
		}
		return onYes()
	}
	a.modal.Show(question, []components.ModalItem{
		{ID: "__no__", Label: "No, keep it"},
		{ID: confirmYes, Label: "Yes, delete"},
	})
}

// handleIssueLinkChanged refreshes the issue whose links or subtasks changed,
// dropping the stale cache first so the refetch is not served from it.
func (a *App) handleIssueLinkChanged(msg issueLinkChangedMsg) (tea.Model, tea.Cmd) {
	if msg.issueKey == "" {
		return a, nil
	}
	delete(a.issueCache, msg.issueKey)
	delete(a.childrenCache, msg.issueKey)
	return a, fetchIssueDetail(a.client, msg.issueKey)
}

// handleIssueDeleted drops the issue everywhere it is remembered and reloads
// the list it came from.
func (a *App) handleIssueDeleted(msg issueDeletedMsg) (tea.Model, tea.Cmd) {
	delete(a.issueCache, msg.issueKey)
	delete(a.childrenCache, msg.issueKey)
	if msg.issueKey == a.previewKey {
		a.previewKey = ""
	}

	cmds := []tea.Cmd{a.fetchActiveTab()}
	if msg.parentKey != "" && msg.parentKey != msg.issueKey {
		delete(a.issueCache, msg.parentKey)
		delete(a.childrenCache, msg.parentKey)
		cmds = append(cmds, fetchIssueDetail(a.client, msg.parentKey))
	}
	return a, tea.Batch(cmds...)
}

// linkedIssueRelation phrases a link the way the Lnk tab renders it, so a
// confirmation prompt repeats the row the user is looking at. The pairing --
// outwardIssue with the outward phrase, inwardIssue with the inward one --
// matches renderLinkRowPairs.
func linkedIssueRelation(link *jira.IssueLink) string {
	if link == nil || link.Type == nil {
		return ""
	}
	if link.OutwardIssue != nil {
		return link.Type.Outward
	}
	return link.Type.Inward
}
