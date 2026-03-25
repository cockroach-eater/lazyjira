package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/git"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// handleModalSelected dispatches modal selection via the onSelect callback.
func (a *App) handleModalSelected(msg components.ModalSelectedMsg) (tea.Model, tea.Cmd) {
	fn := a.onSelect
	a.onSelect = nil
	if fn != nil {
		return a, fn(msg.Item)
	}
	return a, nil
}

// handleChecklistConfirmed dispatches checklist selection result.
func (a *App) handleChecklistConfirmed(msg components.ChecklistConfirmedMsg) (tea.Model, tea.Cmd) {
	if fn := a.onChecklist; fn != nil {
		a.onChecklist = nil
		return a, fn(msg.Selected)
	}
	return a, nil
}

// handleModalCancelled clears modal callbacks.
func (a *App) handleModalCancelled() (tea.Model, tea.Cmd) {
	a.onSelect = nil
	a.onChecklist = nil
	return a, nil
}

// handleEditorFinished processes $EDITOR exit and shows diff view.
func (a *App) handleEditorFinished(msg editorFinishedMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tea.EnableMouseCellMotion}
	content, changed, err := readAndCheckEditor(msg)
	if err != nil {
		cleanupEditor(msg.tempPath)
		a.editTempPath = ""
		a.err = err
		return a, tea.Batch(cmds...)
	}
	if !changed {
		cleanupEditor(msg.tempPath)
		a.editTempPath = ""
		return a, tea.Batch(cmds...)
	}
	a.editTempPath = msg.tempPath
	a.diffView.Show("Confirm changes", msg.original, content)
	return a, tea.Batch(cmds...)
}

// handleDiffConfirmed applies the approved edit.
func (a *App) handleDiffConfirmed(msg components.DiffConfirmedMsg) (tea.Model, tea.Cmd) {
	cleanupEditor(a.editTempPath)
	a.editTempPath = ""
	return a, a.applyEdit(msg.Content)
}

// handleDiffCancelled discards the edit.
func (a *App) handleDiffCancelled() (tea.Model, tea.Cmd) {
	cleanupEditor(a.editTempPath)
	a.editTempPath = ""
	return a, nil
}

// handleInputConfirmed processes text input results (summary, field, branch).
func (a *App) handleInputConfirmed(msg components.InputConfirmedMsg) (tea.Model, tea.Cmd) {
	ctx := a.editContext
	a.editContext = editCtx{}
	switch ctx.kind { //nolint:exhaustive // only InputModal-based kinds handled here
	case editSummary:
		if msg.Text != "" {
			return a, updateIssueField(a.client, ctx.issueKey, "summary", msg.Text)
		}
	case editField:
		if msg.Text != "" {
			return a, updateIssueField(a.client, ctx.issueKey, ctx.fieldID, msg.Text)
		}
	case editBranch:
		if msg.Text != "" {
			switch {
			case git.BranchExists(a.gitRepoPath, msg.Text):
				return a, gitCheckoutBranch(a.gitRepoPath, msg.Text)
			case strings.Contains(msg.Text, "/"):
				return a, gitCheckoutTracking(a.gitRepoPath, msg.Text)
			default:
				return a, gitCreateBranch(a.gitRepoPath, msg.Text)
			}
		}
	}
	return a, nil
}

// handleInputCancelled clears edit context.
func (a *App) handleInputCancelled() (tea.Model, tea.Cmd) {
	a.editContext = editCtx{}
	return a, nil
}

// handleExpandBlock shows expanded content in a read-only modal.
func (a *App) handleExpandBlock(msg views.ExpandBlockMsg) (tea.Model, tea.Cmd) {
	items := make([]components.ModalItem, 0, len(msg.Lines))
	for _, line := range msg.Lines {
		items = append(items, components.ModalItem{ID: "", Label: line})
	}
	a.modal.SetSize(a.width, a.height-1)
	a.modal.ShowReadOnly(msg.Title, items)
	return a, nil
}
