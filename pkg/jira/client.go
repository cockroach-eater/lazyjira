package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ClientInterface interface {
	GetIssue(ctx context.Context, issueKey string) (*Issue, error)
	SearchIssues(ctx context.Context, jql string, startAt, maxResults int) (*SearchResult, error)
	GetMyIssues(ctx context.Context) ([]Issue, error)
	GetTransitions(ctx context.Context, issueKey string) ([]Transition, error)
	DoTransition(ctx context.Context, issueKey, transitionID string) error
	AddComment(ctx context.Context, issueKey, body string) (*Comment, error)
	AssignIssue(ctx context.Context, issueKey, accountID string) error
	GetProjects(ctx context.Context) ([]Project, error)
	GetBoards(ctx context.Context) ([]Board, error)
	GetBoardIssues(ctx context.Context, boardID int, jql string) ([]Issue, error)
	UpdateIssue(ctx context.Context, issueKey string, fields map[string]any) error
	CreateIssue(ctx context.Context, projectKey, issueTypeID, summary, description string) (*Issue, error)
	GetComments(ctx context.Context, issueKey string) ([]Comment, error)
	GetUsers(ctx context.Context, projectKey string) ([]User, error)
	GetSprints(ctx context.Context, boardID int) ([]Sprint, error)
	GetChangelog(ctx context.Context, issueKey string) ([]ChangelogEntry, error)
	SetOnRequest(fn func(RequestLog))
	SetCustomFields(ids []string)
}

// RequestLog contains info about a completed API request.
type RequestLog struct {
	Method  string
	Path    string
	Status  int
	Elapsed time.Duration
}

type Client struct {
	baseURL        string
	authHeader     string
	httpClient     *http.Client
	dryRun         bool
	logger         io.Writer
	onRequest      func(RequestLog) // callback for TUI log panel
	customFieldIDs []string
}

// SetCustomFields sets the list of custom field IDs to fetch from the API.
func (c *Client) SetCustomFields(ids []string) { c.customFieldIDs = ids }

// Compile-time check that Client implements ClientInterface.
var _ ClientInterface = (*Client)(nil)

func NewClient(host, email, token string) *Client {
	host = strings.TrimRight(host, "/")
	credentials := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
	return &Client{
		baseURL:    host + "/rest/api/3",
		authHeader: "Basic " + credentials,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewOAuthClient creates a client using OAuth bearer token.
// baseURL is https://api.atlassian.com/ex/jira/{cloudId}/rest/api/3
func NewOAuthClient(cloudID, accessToken string) *Client {
	return &Client{
		baseURL:    "https://api.atlassian.com/ex/jira/" + cloudID + "/rest/api/3",
		authHeader: "Bearer " + accessToken,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// BaseURL returns the API base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// AuthHeader returns the Authorization header value.
func (c *Client) AuthHeader() string { return c.authHeader }

// SetDryRun enables dry-run mode: GET requests work normally, write operations (POST/PUT/DELETE) are skipped.
func (c *Client) SetDryRun(v bool) { c.dryRun = v }

// SetLogger sets a writer for request logging.
func (c *Client) SetLogger(w io.Writer) { c.logger = w }

// SetOnRequest sets a callback for each completed request (for TUI log panel).
func (c *Client) SetOnRequest(fn func(RequestLog)) { c.onRequest = fn }

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	start := time.Now()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	fullURL := c.baseURL + path

	// Log request.
	c.log("%s %s %s\n", start.Format("15:04:05"), method, fullURL)

	// Dry-run: skip write operations.
	if c.dryRun && method != http.MethodGet {
		c.log("  [DRY-RUN] skipped write operation\n")
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log("  ERROR: %v\n", err)
		return fmt.Errorf("execute request %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	elapsed := time.Since(start)
	c.log("  -> %d (%s) %d bytes\n", resp.StatusCode, elapsed.Round(time.Millisecond), len(respBody))

	if c.onRequest != nil {
		c.onRequest(RequestLog{
			Method:  method,
			Path:    path,
			Status:  resp.StatusCode,
			Elapsed: elapsed,
		})
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Log error response body for debugging.
		c.log("  BODY: %s\n", string(respBody))
		return fmt.Errorf("request %s %s returned status %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response for %s %s: %w", method, path, err)
		}
	}

	return nil
}

func (c *Client) log(format string, args ...any) {
	if c.logger != nil {
		fmt.Fprintf(c.logger, format, args...)
	}
}

func (c *Client) GetIssue(ctx context.Context, issueKey string) (*Issue, error) {
	var raw issueResponse
	err := c.do(ctx, http.MethodGet, "/issue/"+issueKey, nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", issueKey, err)
	}
	issue := raw.toIssue()
	return &issue, nil
}

func (c *Client) SearchIssues(ctx context.Context, jql string, startAt, maxResults int) (*SearchResult, error) {
	fields := "summary,description,status,priority,assignee,reporter,labels,components,sprint,issuetype,created,updated,subtasks,issuelinks"
	if len(c.customFieldIDs) > 0 {
		fields += "," + strings.Join(c.customFieldIDs, ",")
	}
	path := fmt.Sprintf("/search/jql?jql=%s&startAt=%d&maxResults=%d&fields=%s",
		url.QueryEscape(jql), startAt, maxResults, fields)

	var raw searchResponse
	err := c.do(ctx, http.MethodGet, path, nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}

	result := &SearchResult{
		Total:      raw.Total,
		MaxResults: raw.MaxResults,
		StartAt:    raw.StartAt,
		Issues:     make([]Issue, len(raw.Issues)),
	}
	for i, ri := range raw.Issues {
		result.Issues[i] = ri.toIssue()
	}
	return result, nil
}

func (c *Client) GetMyIssues(ctx context.Context) ([]Issue, error) {
	jql := "assignee=currentUser() ORDER BY priority DESC, updated DESC"
	result, err := c.SearchIssues(ctx, jql, 0, 50)
	if err != nil {
		return nil, fmt.Errorf("get my issues: %w", err)
	}
	return result.Issues, nil
}

func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]Transition, error) {
	var raw struct {
		Transitions []Transition `json:"transitions"`
	}
	err := c.do(ctx, http.MethodGet, "/issue/"+issueKey+"/transitions", nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("get transitions for %s: %w", issueKey, err)
	}
	return raw.Transitions, nil
}

func (c *Client) DoTransition(ctx context.Context, issueKey, transitionID string) error {
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	err := c.do(ctx, http.MethodPost, "/issue/"+issueKey+"/transitions", body, nil)
	if err != nil {
		return fmt.Errorf("do transition %s on %s: %w", transitionID, issueKey, err)
	}
	return nil
}

func (c *Client) AddComment(ctx context.Context, issueKey, body string) (*Comment, error) {
	reqBody := map[string]any{
		"body": body,
	}
	var raw commentResponse
	err := c.do(ctx, http.MethodPost, "/issue/"+issueKey+"/comment", reqBody, &raw)
	if err != nil {
		return nil, fmt.Errorf("add comment to %s: %w", issueKey, err)
	}
	comment := raw.toComment()
	return &comment, nil
}

func (c *Client) AssignIssue(ctx context.Context, issueKey, accountID string) error {
	body := map[string]string{"accountId": accountID}
	err := c.do(ctx, http.MethodPut, "/issue/"+issueKey+"/assignee", body, nil)
	if err != nil {
		return fmt.Errorf("assign issue %s: %w", issueKey, err)
	}
	return nil
}

func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	var raw []projectResponse
	err := c.do(ctx, http.MethodGet, "/project/search?maxResults=100", nil, &struct {
		Values *[]projectResponse `json:"values"`
	}{Values: &raw})
	if err != nil {
		return nil, fmt.Errorf("get projects: %w", err)
	}
	projects := make([]Project, len(raw))
	for i, rp := range raw {
		projects[i] = rp.toProject()
	}
	return projects, nil
}

// doAgile executes a GET request against the Jira Agile REST API.
func (c *Client) doAgile(ctx context.Context, path string, result any) error {
	origBase := c.baseURL
	c.baseURL = strings.Replace(c.baseURL, "/rest/api/3", "/rest/agile/1.0", 1)
	err := c.do(ctx, http.MethodGet, path, nil, result)
	c.baseURL = origBase
	return err
}

func (c *Client) GetBoards(ctx context.Context) ([]Board, error) {
	var raw struct {
		Values []boardResponse `json:"values"`
	}
	err := c.doAgile(ctx, "/board?maxResults=100", &raw)
	if err != nil {
		return nil, fmt.Errorf("get boards: %w", err)
	}
	boards := make([]Board, len(raw.Values))
	for i, rb := range raw.Values {
		boards[i] = rb.toBoard()
	}
	return boards, nil
}

func (c *Client) GetBoardIssues(ctx context.Context, boardID int, jql string) ([]Issue, error) {
	path := fmt.Sprintf("/board/%d/issue?maxResults=50", boardID)
	if jql != "" {
		path += "&jql=" + jql
	}

	var raw searchResponse
	err := c.doAgile(ctx, path, &raw)
	if err != nil {
		return nil, fmt.Errorf("get board %d issues: %w", boardID, err)
	}
	issues := make([]Issue, len(raw.Issues))
	for i, ri := range raw.Issues {
		issues[i] = ri.toIssue()
	}
	return issues, nil
}

func (c *Client) UpdateIssue(ctx context.Context, issueKey string, fields map[string]any) error {
	body := map[string]any{"fields": fields}
	err := c.do(ctx, http.MethodPut, "/issue/"+issueKey, body, nil)
	if err != nil {
		return fmt.Errorf("update issue %s: %w", issueKey, err)
	}
	return nil
}

func (c *Client) CreateIssue(ctx context.Context, projectKey, issueTypeID, summary, description string) (*Issue, error) {
	body := map[string]any{
		"fields": map[string]any{
			"project":     map[string]string{"key": projectKey},
			"issuetype":   map[string]string{"id": issueTypeID},
			"summary":     summary,
			"description": description,
		},
	}
	var raw issueResponse
	err := c.do(ctx, http.MethodPost, "/issue", body, &raw)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}
	issue := raw.toIssue()
	return &issue, nil
}

func (c *Client) GetComments(ctx context.Context, issueKey string) ([]Comment, error) {
	var raw struct {
		Comments []commentResponse `json:"comments"`
	}
	err := c.do(ctx, http.MethodGet, "/issue/"+issueKey+"/comment", nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("get comments for %s: %w", issueKey, err)
	}
	comments := make([]Comment, len(raw.Comments))
	for i, rc := range raw.Comments {
		comments[i] = rc.toComment()
	}
	return comments, nil
}

func (c *Client) GetChangelog(ctx context.Context, issueKey string) ([]ChangelogEntry, error) {
	var raw struct {
		Values []changelogResponse `json:"values"`
	}
	err := c.do(ctx, http.MethodGet, "/issue/"+issueKey+"/changelog?maxResults=100", nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("get changelog for %s: %w", issueKey, err)
	}
	entries := make([]ChangelogEntry, len(raw.Values))
	for i, rc := range raw.Values {
		entries[i] = rc.toChangelogEntry()
	}
	return entries, nil
}

func (c *Client) GetUsers(ctx context.Context, projectKey string) ([]User, error) {
	var raw []userResponse
	err := c.do(ctx, http.MethodGet, "/user/assignable/search?project="+projectKey+"&maxResults=100", nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("get users for project %s: %w", projectKey, err)
	}
	users := make([]User, len(raw))
	for i, ru := range raw {
		users[i] = ru.toUser()
	}
	return users, nil
}

func (c *Client) GetSprints(ctx context.Context, boardID int) ([]Sprint, error) {
	var raw struct {
		Values []Sprint `json:"values"`
	}
	path := fmt.Sprintf("/board/%d/sprint?maxResults=50", boardID)
	err := c.doAgile(ctx, path, &raw)
	if err != nil {
		return nil, fmt.Errorf("get sprints for board %d: %w", boardID, err)
	}
	return raw.Values, nil
}

// Internal response types for proper JSON unmarshalling from Jira REST API v3.

type issueResponse struct {
	ID     string              `json:"id"`
	Key    string              `json:"key"`
	Fields issueFieldsResponse `json:"fields"`
}

type issueFieldsResponse struct {
	Summary     string              `json:"summary"`
	Description any                 `json:"description"`
	Status      *statusResponse     `json:"status"`
	Priority    *Priority           `json:"priority"`
	Assignee    *userResponse       `json:"assignee"`
	Reporter    *userResponse       `json:"reporter"`
	Labels      []string            `json:"labels"`
	Components  []Component         `json:"components"`
	Sprint      *Sprint             `json:"sprint"`
	IssueType   *IssueType          `json:"issuetype"`
	Created     JiraTime            `json:"created"`
	Updated     JiraTime            `json:"updated"`
	Subtasks    []issueResponse     `json:"subtasks"`
	IssueLinks  []issueLinkResponse `json:"issuelinks"`
	RawExtra    map[string]json.RawMessage `json:"-"`
}

func (f *issueFieldsResponse) UnmarshalJSON(data []byte) error {
	type Alias issueFieldsResponse
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*f = issueFieldsResponse(alias)

	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}
	f.RawExtra = make(map[string]json.RawMessage)
	for k, v := range allFields {
		if strings.HasPrefix(k, "customfield_") {
			f.RawExtra[k] = v
		}
	}
	return nil
}

type statusResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory struct {
		Key string `json:"key"`
	} `json:"statusCategory"`
}

func (r *statusResponse) toStatus() *Status {
	return &Status{
		ID:          r.ID,
		Name:        r.Name,
		CategoryKey: r.StatusCategory.Key,
	}
}

func (r *issueResponse) toIssue() Issue {
	issue := Issue{
		ID:         r.ID,
		Key:        r.Key,
		Summary:    r.Fields.Summary,
		Priority:   r.Fields.Priority,
		Labels:     r.Fields.Labels,
		Components: r.Fields.Components,
		Sprint:     r.Fields.Sprint,
		IssueType:  r.Fields.IssueType,
		Created:    r.Fields.Created.Time,
		Updated:    r.Fields.Updated.Time,
	}

	if r.Fields.Status != nil {
		issue.Status = r.Fields.Status.toStatus()
	}

	// Description in API v3 is ADF (JSON document); extract plain text.
	if r.Fields.Description != nil {
		issue.Description = extractADFText(r.Fields.Description)
	}

	if r.Fields.Assignee != nil {
		u := r.Fields.Assignee.toUser()
		issue.Assignee = &u
	}
	if r.Fields.Reporter != nil {
		u := r.Fields.Reporter.toUser()
		issue.Reporter = &u
	}

	issue.Subtasks = make([]Issue, len(r.Fields.Subtasks))
	for i, sub := range r.Fields.Subtasks {
		issue.Subtasks[i] = sub.toIssue()
	}

	issue.IssueLinks = make([]IssueLink, len(r.Fields.IssueLinks))
	for i, link := range r.Fields.IssueLinks {
		issue.IssueLinks[i] = link.toIssueLink()
	}

	if len(r.Fields.RawExtra) > 0 {
		issue.CustomFields = make(map[string]any)
		for k, raw := range r.Fields.RawExtra {
			var val any
			if err := json.Unmarshal(raw, &val); err == nil && val != nil {
				issue.CustomFields[k] = val
			}
		}
	}

	return issue
}

// extractADFText recursively extracts plain text from Atlassian Document Format.
//
//nolint:gocognit // ADF parser complexity is inherent to the format
func extractADFText(v any) string {
	switch node := v.(type) {
	case map[string]any:
		nodeType, _ := node["type"].(string)

		switch nodeType {
		case "text":
			if text, ok := node["text"].(string); ok {
				return text
			}
		case "mention":
			// {"type":"mention","attrs":{"text":"@Name","id":"..."}}
			// Wrap in markers so TUI can color the full name including spaces.
			if attrs, ok := node["attrs"].(map[string]any); ok {
				if text, ok := attrs["text"].(string); ok {
					return "\x00MENTION:" + text + "\x00"
				}
			}
		case "emoji":
			if attrs, ok := node["attrs"].(map[string]any); ok {
				if shortName, ok := attrs["shortName"].(string); ok {
					return shortName
				}
			}
		case "hardBreak":
			return "\n"
		case "inlineCard":
			// {"type":"inlineCard","attrs":{"url":"..."}}
			if attrs, ok := node["attrs"].(map[string]any); ok {
				if url, ok := attrs["url"].(string); ok {
					return url
				}
			}
		case "listItem":
			// Render list items with bullet.
			if content, ok := node["content"].([]any); ok {
				var parts []string
				for _, child := range content {
					if text := extractADFText(child); text != "" {
						parts = append(parts, text)
					}
				}
				return "• " + strings.Join(parts, "")
			}
		}

		// Recurse into content array for container nodes.
		if content, ok := node["content"].([]any); ok {
			var parts []string
			for _, child := range content {
				if text := extractADFText(child); text != "" {
					parts = append(parts, text)
				}
			}
			sep := ""
			if nodeType == "paragraph" || nodeType == "heading" || nodeType == "bulletList" || nodeType == "orderedList" || nodeType == "codeBlock" || nodeType == "blockquote" {
				sep = "\n"
			}
			return strings.Join(parts, "") + sep
		}

	case []any:
		var parts []string
		for _, child := range node {
			if text := extractADFText(child); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")

	case string:
		return node
	}
	return ""
}

type issueLinkResponse struct {
	ID           string         `json:"id"`
	Type         *IssueLinkType `json:"type"`
	InwardIssue  *issueResponse `json:"inwardIssue"`
	OutwardIssue *issueResponse `json:"outwardIssue"`
}

func (r *issueLinkResponse) toIssueLink() IssueLink {
	link := IssueLink{
		ID:   r.ID,
		Type: r.Type,
	}
	if r.InwardIssue != nil {
		i := r.InwardIssue.toIssue()
		link.InwardIssue = &i
	}
	if r.OutwardIssue != nil {
		o := r.OutwardIssue.toIssue()
		link.OutwardIssue = &o
	}
	return link
}

type userResponse struct {
	AccountID   string          `json:"accountId"`
	DisplayName string          `json:"displayName"`
	Email       string          `json:"emailAddress"`
	AvatarURLs  json.RawMessage `json:"avatarUrls"`
	Active      bool            `json:"active"`
}

func (r *userResponse) toUser() User {
	u := User{
		AccountID:   r.AccountID,
		DisplayName: r.DisplayName,
		Email:       r.Email,
		Active:      r.Active,
	}
	// Extract 48x48 avatar URL from the avatarUrls map.
	var avatars map[string]string
	if err := json.Unmarshal(r.AvatarURLs, &avatars); err == nil {
		u.AvatarURL = avatars["48x48"]
	}
	return u
}

type commentResponse struct {
	ID      string        `json:"id"`
	Author  *userResponse `json:"author"`
	Body    any   `json:"body"`
	Created JiraTime      `json:"created"`
	Updated JiraTime      `json:"updated"`
}

func (r *commentResponse) toComment() Comment {
	c := Comment{
		ID:      r.ID,
		Created: r.Created.Time,
		Updated: r.Updated.Time,
	}
	if r.Author != nil {
		u := r.Author.toUser()
		c.Author = &u
	}
	// Body in API v3 is ADF; extract plain text.
	if r.Body != nil {
		c.Body = extractADFText(r.Body)
	}
	return c
}

type projectResponse struct {
	ID         string          `json:"id"`
	Key        string          `json:"key"`
	Name       string          `json:"name"`
	AvatarURLs json.RawMessage `json:"avatarUrls"`
	Lead       *userResponse   `json:"lead"`
}

func (r *projectResponse) toProject() Project {
	p := Project{
		ID:   r.ID,
		Key:  r.Key,
		Name: r.Name,
	}
	var avatars map[string]string
	if err := json.Unmarshal(r.AvatarURLs, &avatars); err == nil {
		p.AvatarURL = avatars["48x48"]
	}
	if r.Lead != nil {
		u := r.Lead.toUser()
		p.Lead = &u
	}
	return p
}

type boardResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location *struct {
		ProjectKey string `json:"projectKey"`
	} `json:"location"`
}

func (r *boardResponse) toBoard() Board {
	b := Board{
		ID:   r.ID,
		Name: r.Name,
		Type: r.Type,
	}
	if r.Location != nil {
		b.ProjectKey = r.Location.ProjectKey
	}
	return b
}

type searchResponse struct {
	Issues     []issueResponse `json:"issues"`
	Total      int             `json:"total"`
	MaxResults int             `json:"maxResults"`
	StartAt    int             `json:"startAt"`
}

type changelogResponse struct {
	Author  *userResponse `json:"author"`
	Created JiraTime      `json:"created"`
	Items   []struct {
		Field      string `json:"field"`
		FromString string `json:"fromString"`
		ToString   string `json:"toString"`
	} `json:"items"`
}

func (r *changelogResponse) toChangelogEntry() ChangelogEntry {
	e := ChangelogEntry{
		Created: r.Created.Time,
	}
	if r.Author != nil {
		u := r.Author.toUser()
		e.Author = &u
	}
	for _, item := range r.Items {
		e.Items = append(e.Items, ChangeItem{
			Field:      item.Field,
			FromString: item.FromString,
			ToString:   item.ToString,
		})
	}
	return e
}
