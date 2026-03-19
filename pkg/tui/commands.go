package tui

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
)

func fetchIssuesByJQL(client jira.ClientInterface, jql string, tab int) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchIssues(context.Background(), jql, 0, 50)
		if err != nil {
			return errorMsg{err: err}
		}
		return issuesLoadedMsg{issues: result.Issues, tab: tab}
	}
}

// resolveTabJQL applies template variables to a tab's JQL string.
func resolveTabJQL(tab config.IssueTabConfig, projectKey, email string) string {
	tmpl, err := template.New("jql").Parse(tab.JQL)
	if err != nil {
		return fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey)
	}
	data := struct {
		ProjectKey string
		UserEmail  string
	}{ProjectKey: projectKey, UserEmail: email}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey)
	}
	return buf.String()
}

// fetchFullIssue fetches issue + comments + changelog, returning the given message type.
func fetchFullIssue(client jira.ClientInterface, key string, mkMsg func(*jira.Issue) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(context.Background(), key)
		if err != nil {
			return mkMsg(nil)
		}
		comments, err := client.GetComments(context.Background(), key)
		if err == nil {
			issue.Comments = comments
		}
		changelog, err := client.GetChangelog(context.Background(), key)
		if err == nil {
			issue.Changelog = changelog
		}
		return mkMsg(issue)
	}
}

func fetchIssueDetail(client jira.ClientInterface, key string) tea.Cmd {
	return fetchFullIssue(client, key, func(issue *jira.Issue) tea.Msg {
		if issue == nil {
			return errorMsg{err: fmt.Errorf("failed to fetch issue %s", key)}
		}
		return issueDetailLoadedMsg{issue: issue}
	})
}

func fetchProjects(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		projects, err := client.GetProjects(context.Background())
		if err != nil {
			return errorMsg{err: err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

func prefetchIssue(client jira.ClientInterface, key string) tea.Cmd {
	return fetchFullIssue(client, key, func(issue *jira.Issue) tea.Msg {
		if issue == nil {
			return nil // silent fail for prefetch
		}
		return issuePrefetchedMsg{issue: issue}
	})
}

func fetchTransitions(client jira.ClientInterface, issueKey string) tea.Cmd {
	return func() tea.Msg {
		transitions, err := client.GetTransitions(context.Background(), issueKey)
		if err != nil {
			return errorMsg{err: err}
		}
		return transitionsLoadedMsg{issueKey: issueKey, transitions: transitions}
	}
}

func doTransition(client jira.ClientInterface, key, transitionID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DoTransition(context.Background(), key, transitionID)
		if err != nil {
			return errorMsg{err: err}
		}
		return transitionDoneMsg{}
	}
}

func saveLastProject(projectKey string) {
	creds, err := config.LoadCredentials()
	if err != nil || creds == nil {
		return
	}
	creds.LastProject = projectKey
	_ = config.SaveCredentials(creds)
}
