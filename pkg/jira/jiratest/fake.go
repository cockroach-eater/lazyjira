// Package jiratest provides a FakeClient that implements jira.ClientInterface
// for use in unit tests. By default every method calls t.Fatalf, so unexpected
// calls surface immediately with a clear message. Tests opt-in by assigning a
// *Func field for each method they expect to be called.
package jiratest

import (
	"context"
	"testing"

	"github.com/textfuel/lazyjira/pkg/jira"
)

// Call-record structs. One per method that takes arguments worth asserting.

type GetIssueCall struct {
	Ctx context.Context
	Key string
}

type SearchIssuesCall struct {
	Ctx        context.Context
	JQL        string
	StartAt    int
	MaxResults int
}

type GetTransitionsCall struct {
	Ctx context.Context
	Key string
}

type DoTransitionCall struct {
	Ctx          context.Context
	Key          string
	TransitionID string
}

type AddCommentCall struct {
	Ctx  context.Context
	Key  string
	Body any
}

type UpdateCommentCall struct {
	Ctx       context.Context
	Key       string
	CommentID string
	Body      any
}

type AssignIssueCall struct {
	Ctx       context.Context
	Key       string
	AccountID string
}

type GetBoardIssuesCall struct {
	Ctx     context.Context
	BoardID int
	JQL     string
}

type UpdateIssueCall struct {
	Ctx    context.Context
	Key    string
	Fields map[string]any
}

type CreateIssueCall struct {
	Ctx    context.Context
	Fields map[string]any
}

type GetCreateMetaCall struct {
	Ctx         context.Context
	ProjectKey  string
	IssueTypeID string
}

type GetCommentsCall struct {
	Ctx context.Context
	Key string
}

type GetUsersCall struct {
	Ctx        context.Context
	ProjectKey string
}

type GetSprintsCall struct {
	Ctx     context.Context
	BoardID int
}

type MoveToSprintCall struct {
	Ctx      context.Context
	SprintID int
	Key      string
}

type GetChangelogCall struct {
	Ctx context.Context
	Key string
}

type GetComponentsCall struct {
	Ctx        context.Context
	ProjectKey string
}

type GetIssueTypesCall struct {
	Ctx       context.Context
	ProjectID string
}

type GetJQLAutocompleteSuggestionsCall struct {
	Ctx        context.Context
	FieldName  string
	FieldValue string
}

// FakeClient is a strict-fail implementation of jira.ClientInterface.
//
// Usage:
//
//	fake := &jiratest.FakeClient{T: t}
//	fake.GetIssueFunc = func(ctx context.Context, key string) (*jira.Issue, error) {
//	    return &jira.Issue{Key: key}, nil
//	}
//	// ... use fake as jira.ClientInterface ...
//	if len(fake.GetIssueCalls) != 1 || fake.GetIssueCalls[0].Key != "ABC-1" {
//	    t.Errorf("unexpected calls: %+v", fake.GetIssueCalls)
//	}
type FakeClient struct {
	T *testing.T

	// Function-field overrides. Default (nil) -> t.Fatalf on call.
	GetIssueFunc                      func(ctx context.Context, key string) (*jira.Issue, error)
	SearchIssuesFunc                  func(ctx context.Context, jql string, startAt, maxResults int) (*jira.SearchResult, error)
	GetMyIssuesFunc                   func(ctx context.Context) ([]jira.Issue, error)
	GetTransitionsFunc                func(ctx context.Context, key string) ([]jira.Transition, error)
	DoTransitionFunc                  func(ctx context.Context, key, transitionID string) error
	AddCommentFunc                    func(ctx context.Context, key string, body any) (*jira.Comment, error)
	UpdateCommentFunc                 func(ctx context.Context, key, commentID string, body any) error
	AssignIssueFunc                   func(ctx context.Context, key, accountID string) error
	GetProjectsFunc                   func(ctx context.Context) ([]jira.Project, error)
	GetBoardsFunc                     func(ctx context.Context) ([]jira.Board, error)
	GetBoardIssuesFunc                func(ctx context.Context, boardID int, jql string) ([]jira.Issue, error)
	UpdateIssueFunc                   func(ctx context.Context, key string, fields map[string]any) error
	GetPrioritiesFunc                 func(ctx context.Context) ([]jira.Priority, error)
	CreateIssueFunc                   func(ctx context.Context, fields map[string]any) (*jira.Issue, error)
	GetCreateMetaFunc                 func(ctx context.Context, projectKey, issueTypeID string) ([]jira.CreateMetaField, error)
	GetCommentsFunc                   func(ctx context.Context, key string) ([]jira.Comment, error)
	GetMyselfFunc                     func(ctx context.Context) (*jira.User, error)
	GetUsersFunc                      func(ctx context.Context, projectKey string) ([]jira.User, error)
	GetSprintsFunc                    func(ctx context.Context, boardID int) ([]jira.Sprint, error)
	MoveToSprintFunc                  func(ctx context.Context, sprintID int, key string) error
	GetChangelogFunc                  func(ctx context.Context, key string) ([]jira.ChangelogEntry, error)
	GetLabelsFunc                     func(ctx context.Context) ([]string, error)
	GetComponentsFunc                 func(ctx context.Context, projectKey string) ([]jira.Component, error)
	GetIssueTypesFunc                 func(ctx context.Context, projectID string) ([]jira.IssueType, error)
	GetJQLAutocompleteDataFunc        func(ctx context.Context) ([]jira.AutocompleteField, error)
	GetJQLAutocompleteSuggestionsFunc func(ctx context.Context, fieldName, fieldValue string) ([]jira.AutocompleteSuggestion, error)
	SetOnRequestFunc                  func(fn func(jira.RequestLog))
	SetCustomFieldsFunc               func(ids []string)

	// Call recorders (populated before the *Func is invoked).
	GetIssueCalls                      []GetIssueCall
	SearchIssuesCalls                  []SearchIssuesCall
	GetMyIssuesCalls                   []context.Context
	GetTransitionsCalls                []GetTransitionsCall
	DoTransitionCalls                  []DoTransitionCall
	AddCommentCalls                    []AddCommentCall
	UpdateCommentCalls                 []UpdateCommentCall
	AssignIssueCalls                   []AssignIssueCall
	GetProjectsCalls                   []context.Context
	GetBoardsCalls                     []context.Context
	GetBoardIssuesCalls                []GetBoardIssuesCall
	UpdateIssueCalls                   []UpdateIssueCall
	GetPrioritiesCalls                 []context.Context
	CreateIssueCalls                   []CreateIssueCall
	GetCreateMetaCalls                 []GetCreateMetaCall
	GetCommentsCalls                   []GetCommentsCall
	GetMyselfCalls                     []context.Context
	GetUsersCalls                      []GetUsersCall
	GetSprintsCalls                    []GetSprintsCall
	MoveToSprintCalls                  []MoveToSprintCall
	GetChangelogCalls                  []GetChangelogCall
	GetLabelsCalls                     []context.Context
	GetComponentsCalls                 []GetComponentsCall
	GetIssueTypesCalls                 []GetIssueTypesCall
	GetJQLAutocompleteDataCalls        []context.Context
	GetJQLAutocompleteSuggestionsCalls []GetJQLAutocompleteSuggestionsCall
	SetOnRequestCalls                  int
	SetCustomFieldsCalls               [][]string
}

func (f *FakeClient) fatal(name string) {
	f.T.Helper()
	f.T.Fatalf("jiratest.FakeClient: unexpected call to %s (no *Func configured)", name)
}

// --- jira.ClientInterface implementation ---

func (f *FakeClient) GetIssue(ctx context.Context, key string) (*jira.Issue, error) {
	f.GetIssueCalls = append(f.GetIssueCalls, GetIssueCall{Ctx: ctx, Key: key})
	if f.GetIssueFunc == nil {
		f.fatal("GetIssue")
		return nil, nil
	}
	return f.GetIssueFunc(ctx, key)
}

func (f *FakeClient) SearchIssues(ctx context.Context, jql string, startAt, maxResults int) (*jira.SearchResult, error) {
	f.SearchIssuesCalls = append(f.SearchIssuesCalls, SearchIssuesCall{Ctx: ctx, JQL: jql, StartAt: startAt, MaxResults: maxResults})
	if f.SearchIssuesFunc == nil {
		f.fatal("SearchIssues")
		return nil, nil
	}
	return f.SearchIssuesFunc(ctx, jql, startAt, maxResults)
}

func (f *FakeClient) GetMyIssues(ctx context.Context) ([]jira.Issue, error) {
	f.GetMyIssuesCalls = append(f.GetMyIssuesCalls, ctx)
	if f.GetMyIssuesFunc == nil {
		f.fatal("GetMyIssues")
		return nil, nil
	}
	return f.GetMyIssuesFunc(ctx)
}

func (f *FakeClient) GetTransitions(ctx context.Context, key string) ([]jira.Transition, error) {
	f.GetTransitionsCalls = append(f.GetTransitionsCalls, GetTransitionsCall{Ctx: ctx, Key: key})
	if f.GetTransitionsFunc == nil {
		f.fatal("GetTransitions")
		return nil, nil
	}
	return f.GetTransitionsFunc(ctx, key)
}

func (f *FakeClient) DoTransition(ctx context.Context, key, transitionID string) error {
	f.DoTransitionCalls = append(f.DoTransitionCalls, DoTransitionCall{Ctx: ctx, Key: key, TransitionID: transitionID})
	if f.DoTransitionFunc == nil {
		f.fatal("DoTransition")
		return nil
	}
	return f.DoTransitionFunc(ctx, key, transitionID)
}

func (f *FakeClient) AddComment(ctx context.Context, key string, body any) (*jira.Comment, error) {
	f.AddCommentCalls = append(f.AddCommentCalls, AddCommentCall{Ctx: ctx, Key: key, Body: body})
	if f.AddCommentFunc == nil {
		f.fatal("AddComment")
		return nil, nil
	}
	return f.AddCommentFunc(ctx, key, body)
}

func (f *FakeClient) UpdateComment(ctx context.Context, key, commentID string, body any) error {
	f.UpdateCommentCalls = append(f.UpdateCommentCalls, UpdateCommentCall{Ctx: ctx, Key: key, CommentID: commentID, Body: body})
	if f.UpdateCommentFunc == nil {
		f.fatal("UpdateComment")
		return nil
	}
	return f.UpdateCommentFunc(ctx, key, commentID, body)
}

func (f *FakeClient) AssignIssue(ctx context.Context, key, accountID string) error {
	f.AssignIssueCalls = append(f.AssignIssueCalls, AssignIssueCall{Ctx: ctx, Key: key, AccountID: accountID})
	if f.AssignIssueFunc == nil {
		f.fatal("AssignIssue")
		return nil
	}
	return f.AssignIssueFunc(ctx, key, accountID)
}

func (f *FakeClient) GetProjects(ctx context.Context) ([]jira.Project, error) {
	f.GetProjectsCalls = append(f.GetProjectsCalls, ctx)
	if f.GetProjectsFunc == nil {
		f.fatal("GetProjects")
		return nil, nil
	}
	return f.GetProjectsFunc(ctx)
}

func (f *FakeClient) GetBoards(ctx context.Context) ([]jira.Board, error) {
	f.GetBoardsCalls = append(f.GetBoardsCalls, ctx)
	if f.GetBoardsFunc == nil {
		f.fatal("GetBoards")
		return nil, nil
	}
	return f.GetBoardsFunc(ctx)
}

func (f *FakeClient) GetBoardIssues(ctx context.Context, boardID int, jql string) ([]jira.Issue, error) {
	f.GetBoardIssuesCalls = append(f.GetBoardIssuesCalls, GetBoardIssuesCall{Ctx: ctx, BoardID: boardID, JQL: jql})
	if f.GetBoardIssuesFunc == nil {
		f.fatal("GetBoardIssues")
		return nil, nil
	}
	return f.GetBoardIssuesFunc(ctx, boardID, jql)
}

func (f *FakeClient) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	f.UpdateIssueCalls = append(f.UpdateIssueCalls, UpdateIssueCall{Ctx: ctx, Key: key, Fields: fields})
	if f.UpdateIssueFunc == nil {
		f.fatal("UpdateIssue")
		return nil
	}
	return f.UpdateIssueFunc(ctx, key, fields)
}

func (f *FakeClient) GetPriorities(ctx context.Context) ([]jira.Priority, error) {
	f.GetPrioritiesCalls = append(f.GetPrioritiesCalls, ctx)
	if f.GetPrioritiesFunc == nil {
		f.fatal("GetPriorities")
		return nil, nil
	}
	return f.GetPrioritiesFunc(ctx)
}

func (f *FakeClient) CreateIssue(ctx context.Context, fields map[string]any) (*jira.Issue, error) {
	f.CreateIssueCalls = append(f.CreateIssueCalls, CreateIssueCall{Ctx: ctx, Fields: fields})
	if f.CreateIssueFunc == nil {
		f.fatal("CreateIssue")
		return nil, nil
	}
	return f.CreateIssueFunc(ctx, fields)
}

func (f *FakeClient) GetCreateMeta(ctx context.Context, projectKey, issueTypeID string) ([]jira.CreateMetaField, error) {
	f.GetCreateMetaCalls = append(f.GetCreateMetaCalls, GetCreateMetaCall{Ctx: ctx, ProjectKey: projectKey, IssueTypeID: issueTypeID})
	if f.GetCreateMetaFunc == nil {
		f.fatal("GetCreateMeta")
		return nil, nil
	}
	return f.GetCreateMetaFunc(ctx, projectKey, issueTypeID)
}

func (f *FakeClient) GetComments(ctx context.Context, key string) ([]jira.Comment, error) {
	f.GetCommentsCalls = append(f.GetCommentsCalls, GetCommentsCall{Ctx: ctx, Key: key})
	if f.GetCommentsFunc == nil {
		f.fatal("GetComments")
		return nil, nil
	}
	return f.GetCommentsFunc(ctx, key)
}

func (f *FakeClient) GetMyself(ctx context.Context) (*jira.User, error) {
	f.GetMyselfCalls = append(f.GetMyselfCalls, ctx)
	if f.GetMyselfFunc == nil {
		f.fatal("GetMyself")
		return nil, nil
	}
	return f.GetMyselfFunc(ctx)
}

func (f *FakeClient) GetUsers(ctx context.Context, projectKey string) ([]jira.User, error) {
	f.GetUsersCalls = append(f.GetUsersCalls, GetUsersCall{Ctx: ctx, ProjectKey: projectKey})
	if f.GetUsersFunc == nil {
		f.fatal("GetUsers")
		return nil, nil
	}
	return f.GetUsersFunc(ctx, projectKey)
}

func (f *FakeClient) GetSprints(ctx context.Context, boardID int) ([]jira.Sprint, error) {
	f.GetSprintsCalls = append(f.GetSprintsCalls, GetSprintsCall{Ctx: ctx, BoardID: boardID})
	if f.GetSprintsFunc == nil {
		f.fatal("GetSprints")
		return nil, nil
	}
	return f.GetSprintsFunc(ctx, boardID)
}

func (f *FakeClient) MoveToSprint(ctx context.Context, sprintID int, key string) error {
	f.MoveToSprintCalls = append(f.MoveToSprintCalls, MoveToSprintCall{Ctx: ctx, SprintID: sprintID, Key: key})
	if f.MoveToSprintFunc == nil {
		f.fatal("MoveToSprint")
		return nil
	}
	return f.MoveToSprintFunc(ctx, sprintID, key)
}

func (f *FakeClient) GetChangelog(ctx context.Context, key string) ([]jira.ChangelogEntry, error) {
	f.GetChangelogCalls = append(f.GetChangelogCalls, GetChangelogCall{Ctx: ctx, Key: key})
	if f.GetChangelogFunc == nil {
		f.fatal("GetChangelog")
		return nil, nil
	}
	return f.GetChangelogFunc(ctx, key)
}

func (f *FakeClient) GetLabels(ctx context.Context) ([]string, error) {
	f.GetLabelsCalls = append(f.GetLabelsCalls, ctx)
	if f.GetLabelsFunc == nil {
		f.fatal("GetLabels")
		return nil, nil
	}
	return f.GetLabelsFunc(ctx)
}

func (f *FakeClient) GetComponents(ctx context.Context, projectKey string) ([]jira.Component, error) {
	f.GetComponentsCalls = append(f.GetComponentsCalls, GetComponentsCall{Ctx: ctx, ProjectKey: projectKey})
	if f.GetComponentsFunc == nil {
		f.fatal("GetComponents")
		return nil, nil
	}
	return f.GetComponentsFunc(ctx, projectKey)
}

func (f *FakeClient) GetIssueTypes(ctx context.Context, projectID string) ([]jira.IssueType, error) {
	f.GetIssueTypesCalls = append(f.GetIssueTypesCalls, GetIssueTypesCall{Ctx: ctx, ProjectID: projectID})
	if f.GetIssueTypesFunc == nil {
		f.fatal("GetIssueTypes")
		return nil, nil
	}
	return f.GetIssueTypesFunc(ctx, projectID)
}

func (f *FakeClient) GetJQLAutocompleteData(ctx context.Context) ([]jira.AutocompleteField, error) {
	f.GetJQLAutocompleteDataCalls = append(f.GetJQLAutocompleteDataCalls, ctx)
	if f.GetJQLAutocompleteDataFunc == nil {
		f.fatal("GetJQLAutocompleteData")
		return nil, nil
	}
	return f.GetJQLAutocompleteDataFunc(ctx)
}

func (f *FakeClient) GetJQLAutocompleteSuggestions(ctx context.Context, fieldName, fieldValue string) ([]jira.AutocompleteSuggestion, error) {
	f.GetJQLAutocompleteSuggestionsCalls = append(f.GetJQLAutocompleteSuggestionsCalls, GetJQLAutocompleteSuggestionsCall{Ctx: ctx, FieldName: fieldName, FieldValue: fieldValue})
	if f.GetJQLAutocompleteSuggestionsFunc == nil {
		f.fatal("GetJQLAutocompleteSuggestions")
		return nil, nil
	}
	return f.GetJQLAutocompleteSuggestionsFunc(ctx, fieldName, fieldValue)
}

func (f *FakeClient) SetOnRequest(fn func(jira.RequestLog)) {
	f.SetOnRequestCalls++
	if f.SetOnRequestFunc == nil {
		f.fatal("SetOnRequest")
		return
	}
	f.SetOnRequestFunc(fn)
}

func (f *FakeClient) SetCustomFields(ids []string) {
	f.SetCustomFieldsCalls = append(f.SetCustomFieldsCalls, ids)
	if f.SetCustomFieldsFunc == nil {
		f.fatal("SetCustomFields")
		return
	}
	f.SetCustomFieldsFunc(ids)
}

var _ jira.ClientInterface = (*FakeClient)(nil)
