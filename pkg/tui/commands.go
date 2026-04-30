package tui

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/git"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

// Git message types
type gitBranchCreatedMsg struct{ name string }
type gitCheckoutDoneMsg struct{ name string }
type gitErrorMsg struct{ err error }

func gitCreateBranch(repoPath, name string) tea.Cmd {
	return func() tea.Msg {
		if err := git.CreateBranch(repoPath, name); err != nil {
			return gitErrorMsg{err: err}
		}
		return gitBranchCreatedMsg{name: name}
	}
}

func gitCheckoutBranch(repoPath, name string) tea.Cmd {
	return func() tea.Msg {
		if err := git.Checkout(repoPath, name); err != nil {
			return gitErrorMsg{err: err}
		}
		return gitCheckoutDoneMsg{name: name}
	}
}

func gitCheckoutTracking(repoPath, remoteBranch string) tea.Cmd {
	return func() tea.Msg {
		if err := git.CheckoutTracking(repoPath, remoteBranch); err != nil {
			return gitErrorMsg{err: err}
		}
		// Extract local name from remote branch (strip remote prefix).
		name := remoteBranch
		if _, after, ok := strings.Cut(remoteBranch, "/"); ok {
			name = after
		}
		return gitCheckoutDoneMsg{name: name}
	}
}

func fetchIssuesByJQL(client jira.ClientInterface, jql string, tab, maxResults int) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchIssues(context.Background(), jql, 0, maxResults)
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
		return fmt.Sprintf("project = \"%s\" ORDER BY updated DESC", projectKey)
	}
	data := struct {
		ProjectKey string
		UserEmail  string
	}{ProjectKey: "\"" + projectKey + "\"", UserEmail: email}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("project = \"%s\" ORDER BY updated DESC", projectKey)
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

// fetchPreviewDetail is like fetchIssueDetail but returns a previewDetailLoadedMsg
// carrying the caller's epoch, so that responses from a superseded preview
// intent can be dropped. See App.previewEpoch.
func fetchPreviewDetail(client jira.ClientInterface, key string, epoch int) tea.Cmd {
	return fetchFullIssue(client, key, func(issue *jira.Issue) tea.Msg {
		if issue == nil {
			return errorMsg{err: fmt.Errorf("failed to fetch issue %s", key)}
		}
		return previewDetailLoadedMsg{issue: issue, epoch: epoch}
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

func batchPrefetch(client jira.ClientInterface, keys []string) tea.Cmd {
	return func() tea.Msg {
		jql := "key in (" + strings.Join(keys, ",") + ")"
		result, err := client.SearchIssues(context.Background(), jql, 0, len(keys))
		if err != nil || result == nil {
			return nil
		}
		return batchPrefetchedMsg{issues: result.Issues}
	}
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

// JQL search messages
type jqlSearchResultMsg struct {
	issues []jira.Issue
	jql    string
}
type jqlSearchErrorMsg struct{ err string }

// JQL autocomplete messages
type jqlFieldsLoadedMsg struct{ fields []jira.AutocompleteField }
type jqlSuggestionsMsg struct{ suggestions []jira.AutocompleteSuggestion }

func fetchJQLSearch(client jira.ClientInterface, jql string, maxResults int) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchIssues(context.Background(), jql, 0, maxResults)
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

type myselfLoadedMsg struct{ user *jira.User }

type fieldsDiscoveredMsg struct{ err error }

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
type createMetaLoadedMsg struct{ fields []jira.CreateMetaField }
type issueCreatedMsg struct{ issue *jira.Issue }
type createErrorMsg struct{ err error }

type customFieldOptionsMsg struct {
	issueKey      string
	fieldID       string
	fieldName     string
	fieldType     views.InfoFieldType
	currentValue  string
	useEditor     bool
	schemaType    string
	schemaItems   string
	options       []jira.CreateMetaValue
	allFields     []jira.CreateMetaField
	issueTypeID   string
	projectKey    string
	fieldNotFound bool
}

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

func fetchCreateMeta(client jira.ClientInterface, projectKey, issueTypeID string) tea.Cmd {
	return func() tea.Msg {
		fields, err := client.GetCreateMeta(context.Background(), projectKey, issueTypeID)
		if err != nil {
			return createErrorMsg{err: err}
		}
		return createMetaLoadedMsg{fields: fields}
	}
}

func fetchCustomFieldOptions(client jira.ClientInterface, projectKey, issueTypeID string, info customFieldOptionsMsg) tea.Cmd {
	return func() tea.Msg {
		meta, err := client.GetCreateMeta(context.Background(), projectKey, issueTypeID)
		if err != nil {
			return errorMsg{err: err}
		}
		info.allFields = meta
		info.issueTypeID = issueTypeID
		info.projectKey = projectKey
		for _, f := range meta {
			if f.FieldID == info.fieldID {
				info.options = f.AllowedValues
				info.schemaType = f.Schema.Type
				info.schemaItems = f.Schema.Items
				return info
			}
		}
		info.fieldNotFound = true
		return info
	}
}

func createIssue(client jira.ClientInterface, fields map[string]any) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.CreateIssue(context.Background(), fields)
		if err != nil {
			return createErrorMsg{err: err}
		}
		return issueCreatedMsg{issue: issue}
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

func fetchMyself(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		user, err := client.GetMyself(context.Background())
		if err != nil {
			return errorMsg{err: err}
		}
		return myselfLoadedMsg{user: user}
	}
}

func fetchFieldDiscovery(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		return fieldsDiscoveredMsg{err: client.DiscoverFields(context.Background())}
	}
}

// prefetchUsersMsg triggers a background users fetch after a short delay
type prefetchUsersMsg struct{ projectKey string }

// prefetchUsers schedules a users fetch after a delay to avoid
// hammering the API when the user switches projects quickly
func prefetchUsers(projectKey string) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		return prefetchUsersMsg{projectKey: projectKey}
	})
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

func fetchBoards(client jira.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		boards, err := client.GetBoards(context.Background())
		if err != nil {
			return nil // silent fail — boards are optional (agile API may be unavailable)
		}
		return boardsLoadedMsg{boards: boards}
	}
}

func fetchSprints(client jira.ClientInterface, boardID int) tea.Cmd {
	return func() tea.Msg {
		sprints, err := client.GetSprints(context.Background(), boardID)
		if err != nil {
			// silently ignore, board may not support sprints
			return sprintsLoadedMsg{sprints: nil}
		}
		return sprintsLoadedMsg{sprints: sprints}
	}
}

func moveToSprint(client jira.ClientInterface, sprintID int, issueKey string) tea.Cmd {
	return func() tea.Msg {
		err := client.MoveToSprint(context.Background(), sprintID, issueKey)
		if err != nil {
			return errorMsg{err: err}
		}
		return issueUpdatedMsg{issueKey: issueKey}
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
