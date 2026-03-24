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

// JQL search messages.
type jqlSearchResultMsg struct {
	issues []jira.Issue
	jql    string
}
type jqlSearchErrorMsg struct{ err string }

// JQL autocomplete messages.
type jqlFieldsLoadedMsg struct{ fields []jira.AutocompleteField }
type jqlSuggestionsMsg struct{ suggestions []jira.AutocompleteSuggestion }

func fetchJQLSearch(client jira.ClientInterface, jql string) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchIssues(context.Background(), jql, 0, 50)
		if err != nil {
			return jqlSearchErrorMsg{err: err.Error()}
		}
		return jqlSearchResultMsg{issues: result.Issues, jql: jql}
	}
}

func fetchJQLAutocompleteData(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		fields, err := client.GetJQLAutocompleteData(context.Background())
		if err != nil {
			return nil
		}
		return jqlFieldsLoadedMsg{fields: fields}
	}
}

func fetchJQLSuggestions(client jira.ClientInterface, fieldName, partial string) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := client.GetJQLAutocompleteSuggestions(context.Background(), fieldName, partial)
		if err != nil {
			return nil
		}
		return jqlSuggestionsMsg{suggestions: suggestions}
	}
}

type issueUpdatedMsg struct{ issueKey string }
type commentAddedMsg struct{ issueKey string }
type commentUpdatedMsg struct{ issueKey string }
type prioritiesLoadedMsg struct{ priorities []jira.Priority }
type usersLoadedMsg struct {
	users    []jira.User
	issueKey string
}
type labelsLoadedMsg struct{ labels []string }
type componentsLoadedMsg struct{ components []jira.Component }
type issueTypesLoadedMsg struct{ issueTypes []jira.IssueType }

func updateIssueField(client jira.ClientInterface, issueKey, field string, value any) tea.Cmd {
	return func() tea.Msg {
		fields := map[string]any{field: value}
		err := client.UpdateIssue(context.Background(), issueKey, fields)
		if err != nil {
			return errorMsg{err: err}
		}
		return issueUpdatedMsg{issueKey: issueKey}
	}
}

func addComment(client jira.ClientInterface, issueKey string, body any) tea.Cmd {
	return func() tea.Msg {
		_, err := client.AddComment(context.Background(), issueKey, body)
		if err != nil {
			return errorMsg{err: err}
		}
		return commentAddedMsg{issueKey: issueKey}
	}
}

func updateComment(client jira.ClientInterface, issueKey, commentID string, body any) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateComment(context.Background(), issueKey, commentID, body)
		if err != nil {
			return errorMsg{err: err}
		}
		return commentUpdatedMsg{issueKey: issueKey}
	}
}

func fetchPriorities(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		priorities, err := client.GetPriorities(context.Background())
		if err != nil {
			return errorMsg{err: err}
		}
		return prioritiesLoadedMsg{priorities: priorities}
	}
}

func fetchUsers(client jira.ClientInterface, projectKey, issueKey string) tea.Cmd {
	return func() tea.Msg {
		users, err := client.GetUsers(context.Background(), projectKey)
		if err != nil {
			return errorMsg{err: err}
		}
		return usersLoadedMsg{users: users, issueKey: issueKey}
	}
}

func fetchLabels(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		labels, err := client.GetLabels(context.Background())
		if err != nil {
			return errorMsg{err: err}
		}
		return labelsLoadedMsg{labels: labels}
	}
}

func fetchComponents(client jira.ClientInterface, projectKey string) tea.Cmd {
	return func() tea.Msg {
		comps, err := client.GetComponents(context.Background(), projectKey)
		if err != nil {
			return errorMsg{err: err}
		}
		return componentsLoadedMsg{components: comps}
	}
}

func fetchIssueTypes(client jira.ClientInterface, projectID string) tea.Cmd {
	return func() tea.Msg {
		types, err := client.GetIssueTypes(context.Background(), projectID)
		if err != nil {
			return errorMsg{err: err}
		}
		return issueTypesLoadedMsg{issueTypes: types}
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
