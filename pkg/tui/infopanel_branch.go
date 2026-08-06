package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// maybeFetchBranch looks up the git branch of issueKey for the Info panel,
// serving the cache when it already knows the answer.
func (a *App) maybeFetchBranch(issueKey string) tea.Cmd {
	if a.gitRepoPath == "" || issueKey == "" {
		return nil
	}
	if branch, ok := a.branchCache[issueKey]; ok {
		a.infoPanel.SetBranch(issueKey, branch)
		return nil
	}
	return fetchIssueBranch(a.gitRepoPath, issueKey)
}

func (a *App) handleBranchLoaded(msg branchLoadedMsg) (tea.Model, tea.Cmd) {
	a.branchCache[msg.issueKey] = msg.branch
	a.infoPanel.SetBranch(msg.issueKey, msg.branch)
	return a, nil
}
