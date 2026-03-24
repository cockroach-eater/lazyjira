//go:build demo

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DemoServer serves a fake Jira REST API v3 backed by in-memory demo data.
// Use with a real jira.Client for full-stack demo and testing.
type DemoServer struct {
	data     *DemoClient
	listener net.Listener
	URL      string
}

// NewDemoServer starts an HTTP server on a random port and returns the server.
// Call Close() when done.
func NewDemoServer() (*DemoServer, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	s := &DemoServer{
		data:     NewDemoClient(),
		listener: ln,
		URL:      fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/", s.handle)

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	return s, nil
}

// Close stops the demo server.
func (s *DemoServer) Close() error {
	return s.listener.Close()
}

func (s *DemoServer) handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/rest/api/3")

	switch {
	case path == "/project/search":
		s.handleProjects(w)
	case path == "/search/jql":
		s.handleSearch(w, r)
	case strings.HasSuffix(path, "/transitions"):
		key := extractKeyFromPath(path, "/transitions")
		if r.Method == http.MethodPost {
			s.handleDoTransition(w, r, key)
		} else {
			s.handleGetTransitions(w, key)
		}
	case strings.Contains(path, "/comment/"):
		// PUT /issue/{key}/comment/{id}
		s.handleUpdateComment(w, r, path)
	case strings.HasSuffix(path, "/comment"):
		key := extractKeyFromPath(path, "/comment")
		if r.Method == http.MethodPost {
			s.handleAddComment(w, r, key)
		} else {
			s.handleComments(w, key)
		}
	case strings.HasSuffix(path, "/changelog"):
		key := extractKeyFromPath(path, "/changelog")
		s.handleChangelog(w, key)
	case strings.HasPrefix(path, "/issue/"):
		key := strings.TrimPrefix(path, "/issue/")
		if r.Method == http.MethodPut {
			s.handleUpdateIssue(w, r, key)
		} else {
			s.handleIssue(w, key)
		}
	case path == "/priority":
		s.handlePriorities(w)
	case strings.HasPrefix(path, "/user/assignable/search"):
		s.handleUsers(w)
	case path == "/label":
		s.handleLabels(w)
	case strings.HasSuffix(path, "/components") && strings.HasPrefix(path, "/project/"):
		key := strings.TrimPrefix(path, "/project/")
		key = strings.TrimSuffix(key, "/components")
		s.handleComponents(w, key)
	case path == "/issuetype/project":
		s.handleIssueTypes(w, r)
	case path == "/jql/autocompletedata":
		s.handleAutocompleteData(w)
	case strings.HasPrefix(path, "/jql/autocompletedata/suggestions"):
		s.handleAutocompleteSuggestions(w, r)
	default:
		http.NotFound(w, r)
	}
}

func extractKeyFromPath(path, suffix string) string {
	s := strings.TrimPrefix(path, "/issue/")
	return strings.TrimSuffix(s, suffix)
}

// --- Handlers ---

func (s *DemoServer) handleProjects(w http.ResponseWriter) {
	projects := make([]any, len(s.data.projects))
	for i, p := range s.data.projects {
		proj := map[string]any{
			"id":         p.ID,
			"key":        p.Key,
			"name":       p.Name,
			"avatarUrls": map[string]string{"48x48": p.AvatarURL},
		}
		if p.Lead != nil {
			proj["lead"] = userToJSON(p.Lead)
		}
		projects[i] = proj
	}
	writeJSON(w, map[string]any{"values": projects})
}

func (s *DemoServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	jql := r.URL.Query().Get("jql")
	startAt := parseIntParam(r.URL.Query().Get("startAt"), 0)
	maxResults := parseIntParam(r.URL.Query().Get("maxResults"), 50)

	result, err := s.data.SearchIssues(context.Background(), jql, startAt, maxResults)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	issues := make([]any, len(result.Issues))
	for i := range result.Issues {
		issues[i] = issueToJSON(&result.Issues[i])
	}

	writeJSON(w, map[string]any{
		"issues":     issues,
		"total":      result.Total,
		"maxResults": result.MaxResults,
		"startAt":    result.StartAt,
	})
}

func (s *DemoServer) handleIssue(w http.ResponseWriter, key string) {
	iss, ok := s.data.issueIndex[key]
	if !ok {
		http.Error(w, fmt.Sprintf("issue %s not found", key), http.StatusNotFound)
		return
	}
	writeJSON(w, issueToJSON(iss))
}

func (s *DemoServer) handleComments(w http.ResponseWriter, key string) {
	comments := s.data.comments[key]
	result := make([]any, len(comments))
	for i := range comments {
		result[i] = commentToJSON(&comments[i])
	}
	writeJSON(w, map[string]any{"comments": result})
}

func (s *DemoServer) handleChangelog(w http.ResponseWriter, key string) {
	entries := s.data.changelog[key]
	result := make([]any, len(entries))
	for i, e := range entries {
		items := make([]any, len(e.Items))
		for j, item := range e.Items {
			items[j] = map[string]any{
				"field":      item.Field,
				"fromString": item.FromString,
				"toString":   item.ToString,
			}
		}
		entry := map[string]any{
			"created": formatJiraTime(e.Created),
			"items":   items,
		}
		if e.Author != nil {
			entry["author"] = userToJSON(e.Author)
		}
		result[i] = entry
	}
	writeJSON(w, map[string]any{"values": result})
}

func (s *DemoServer) handleGetTransitions(w http.ResponseWriter, key string) {
	iss, ok := s.data.issueIndex[key]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	transitions := transitionsForStatus(iss.Status.Name)
	result := make([]any, len(transitions))
	for i, t := range transitions {
		tr := map[string]any{
			"id":   t.ID,
			"name": t.Name,
		}
		if t.To != nil {
			tr["to"] = map[string]any{
				"id":              t.To.ID,
				"name":            t.To.Name,
				"description":     t.To.Description,
				"statusCategory":  map[string]string{"key": t.To.CategoryKey},
			}
		}
		result[i] = tr
	}
	writeJSON(w, map[string]any{"transitions": result})
}

func (s *DemoServer) handleDoTransition(w http.ResponseWriter, r *http.Request, key string) {
	var body struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.data.DoTransition(context.Background(), key, body.Transition.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *DemoServer) handleUpdateIssue(w http.ResponseWriter, r *http.Request, key string) {
	iss, ok := s.data.issueIndex[key]
	if !ok {
		http.Error(w, fmt.Sprintf("issue %s not found", key), http.StatusNotFound)
		return
	}
	var body struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if summary, ok := body.Fields["summary"].(string); ok {
		iss.Summary = summary
	}
	if desc, ok := body.Fields["description"]; ok && desc != nil {
		iss.DescriptionADF = desc
		iss.Description = extractADFText(desc)
	}
	if p, ok := body.Fields["priority"].(map[string]any); ok {
		if iss.Priority == nil {
			iss.Priority = &Priority{}
		}
		if id, ok := p["id"].(string); ok {
			iss.Priority.ID = id
		}
	}
	if v, exists := body.Fields["assignee"]; exists {
		if v == nil {
			iss.Assignee = nil
		} else if m, ok := v.(map[string]any); ok {
			if id, ok := m["accountId"].(string); ok {
				iss.Assignee = &User{AccountID: id}
			}
		}
	}
	if labels, ok := body.Fields["labels"].([]any); ok {
		iss.Labels = make([]string, 0, len(labels))
		for _, l := range labels {
			if s, ok := l.(string); ok {
				iss.Labels = append(iss.Labels, s)
			}
		}
	}
	if comps, ok := body.Fields["components"].([]any); ok {
		demoComps, _ := s.data.GetComponents(context.Background(), "")
		nameMap := make(map[string]string)
		for _, dc := range demoComps {
			nameMap[dc.ID] = dc.Name
		}
		iss.Components = make([]Component, 0, len(comps))
		for _, c := range comps {
			if m, ok := c.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					iss.Components = append(iss.Components, Component{ID: id, Name: nameMap[id]})
				}
			}
		}
	}
	if it, ok := body.Fields["issuetype"].(map[string]any); ok {
		if iss.IssueType == nil {
			iss.IssueType = &IssueType{}
		}
		if id, ok := it["id"].(string); ok {
			iss.IssueType.ID = id
		}
	}
	iss.Updated = time.Now()
	w.WriteHeader(http.StatusNoContent)
}

func (s *DemoServer) handleUpdateComment(w http.ResponseWriter, r *http.Request, path string) {
	// path: /issue/{key}/comment/{id}
	parts := strings.Split(strings.TrimPrefix(path, "/issue/"), "/")
	if len(parts) < 3 || r.Method != http.MethodPut {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	key := parts[0]
	commentID := parts[2]
	var body struct {
		Body any `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.data.UpdateComment(context.Background(), key, commentID, body.Body); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *DemoServer) handleAddComment(w http.ResponseWriter, r *http.Request, key string) {
	var body struct {
		Body any `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	comment, err := s.data.AddComment(context.Background(), key, body.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, commentToJSON(comment))
}

func (s *DemoServer) handlePriorities(w http.ResponseWriter) {
	priorities, _ := s.data.GetPriorities(context.Background())
	result := make([]any, len(priorities))
	for i, p := range priorities {
		result[i] = map[string]any{"id": p.ID, "name": p.Name, "iconUrl": p.IconURL}
	}
	writeJSON(w, result)
}

func (s *DemoServer) handleUsers(w http.ResponseWriter) {
	users, _ := s.data.GetUsers(context.Background(), "")
	result := make([]any, len(users))
	for i, u := range users {
		result[i] = userToJSON(&u)
	}
	writeJSON(w, result)
}

func (s *DemoServer) handleLabels(w http.ResponseWriter) {
	labels, _ := s.data.GetLabels(context.Background())
	writeJSON(w, map[string]any{"values": labels})
}

func (s *DemoServer) handleComponents(w http.ResponseWriter, projectKey string) {
	components, _ := s.data.GetComponents(context.Background(), projectKey)
	result := make([]any, len(components))
	for i, c := range components {
		result[i] = map[string]any{"id": c.ID, "name": c.Name}
	}
	writeJSON(w, result)
}

func (s *DemoServer) handleIssueTypes(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	types, _ := s.data.GetIssueTypes(context.Background(), projectID)
	result := make([]any, len(types))
	for i, t := range types {
		result[i] = map[string]any{"id": t.ID, "name": t.Name}
	}
	writeJSON(w, result)
}

func (s *DemoServer) handleAutocompleteData(w http.ResponseWriter) {
	fields := []map[string]any{
		{"value": "status", "displayName": "Status", "operators": []string{"=", "!=", "in", "not in", "was", "was in", "was not in", "was not", "changed"}},
		{"value": "assignee", "displayName": "Assignee", "operators": []string{"=", "!=", "in", "not in", "was", "was in", "was not in", "was not", "changed"}},
		{"value": "priority", "displayName": "Priority", "operators": []string{"=", "!=", "in", "not in", ">", ">=", "<", "<="}},
		{"value": "project", "displayName": "Project", "operators": []string{"=", "!=", "in", "not in"}},
		{"value": "issuetype", "displayName": "Issue Type", "operators": []string{"=", "!=", "in", "not in"}},
		{"value": "summary", "displayName": "Summary", "operators": []string{"~", "!~", "is", "is not"}},
		{"value": "description", "displayName": "Description", "operators": []string{"~", "!~", "is", "is not"}},
		{"value": "reporter", "displayName": "Reporter", "operators": []string{"=", "!=", "in", "not in", "was", "was in", "was not in", "was not", "changed"}},
		{"value": "created", "displayName": "Created", "operators": []string{"=", "!=", ">", ">=", "<", "<="}},
		{"value": "updated", "displayName": "Updated", "operators": []string{"=", "!=", ">", ">=", "<", "<="}},
		{"value": "labels", "displayName": "Labels", "operators": []string{"=", "!=", "in", "not in"}},
		{"value": "component", "displayName": "Component", "operators": []string{"=", "!=", "in", "not in"}},
	}
	writeJSON(w, map[string]any{"visibleFieldNames": fields})
}

func (s *DemoServer) handleAutocompleteSuggestions(w http.ResponseWriter, r *http.Request) {
	fieldName := r.URL.Query().Get("fieldName")
	suggestions := map[string][]map[string]string{
		"status":    {{"value": "Open", "displayName": "Open"}, {"value": "In Progress", "displayName": "In Progress"}, {"value": "Done", "displayName": "Done"}, {"value": "To Do", "displayName": "To Do"}, {"value": "In Review", "displayName": "In Review"}},
		"priority":  {{"value": "Highest", "displayName": "Highest"}, {"value": "High", "displayName": "High"}, {"value": "Medium", "displayName": "Medium"}, {"value": "Low", "displayName": "Low"}, {"value": "Lowest", "displayName": "Lowest"}},
		"issuetype": {{"value": "Bug", "displayName": "Bug"}, {"value": "Story", "displayName": "Story"}, {"value": "Task", "displayName": "Task"}, {"value": "Epic", "displayName": "Epic"}, {"value": "Sub-task", "displayName": "Sub-task"}},
	}

	results := make([]map[string]string, 0)
	if vals, ok := suggestions[fieldName]; ok {
		fieldValue := r.URL.Query().Get("fieldValue")
		for _, v := range vals {
			if fieldValue == "" || strings.Contains(strings.ToLower(v["displayName"]), strings.ToLower(fieldValue)) {
				results = append(results, v)
			}
		}
	}
	writeJSON(w, map[string]any{"results": results})
}

// --- JSON serialization helpers ---

func issueToJSON(iss *Issue) map[string]any {
	labels := iss.Labels
	if labels == nil {
		labels = []string{}
	}

	fields := map[string]any{
		"summary": iss.Summary,
		"description": descriptionADF(iss),
		"labels":      labels,
		"components":  componentsToJSON(iss.Components),
		"created":     formatJiraTime(iss.Created),
		"updated":     formatJiraTime(iss.Updated),
		"subtasks":    subtasksToJSON(iss.Subtasks),
		"issuelinks":  issueLinksToJSON(iss.IssueLinks),
	}

	if iss.Status != nil {
		fields["status"] = statusToJSON(iss.Status)
	}
	if iss.Priority != nil {
		fields["priority"] = map[string]any{
			"id": iss.Priority.ID, "name": iss.Priority.Name, "iconUrl": iss.Priority.IconURL,
		}
	}
	if iss.Assignee != nil {
		fields["assignee"] = userToJSON(iss.Assignee)
	}
	if iss.Reporter != nil {
		fields["reporter"] = userToJSON(iss.Reporter)
	}
	if iss.Sprint != nil {
		fields["sprint"] = map[string]any{
			"id": iss.Sprint.ID, "name": iss.Sprint.Name, "state": iss.Sprint.State,
		}
	}
	if iss.IssueType != nil {
		fields["issuetype"] = map[string]any{
			"id": iss.IssueType.ID, "name": iss.IssueType.Name,
			"iconUrl": iss.IssueType.IconURL, "subtask": iss.IssueType.Subtask,
		}
	}

	return map[string]any{
		"id":     iss.ID,
		"key":    iss.Key,
		"fields": fields,
	}
}

func commentToJSON(c *Comment) map[string]any {
	m := map[string]any{
		"id":      c.ID,
		"body":    commentBodyADF(c),
		"created": formatJiraTime(c.Created),
		"updated": formatJiraTime(c.Updated),
	}
	if c.Author != nil {
		m["author"] = userToJSON(c.Author)
	}
	return m
}

func statusToJSON(s *Status) map[string]any {
	return map[string]any{
		"id":   s.ID,
		"name": s.Name,
		"statusCategory": map[string]string{
			"key": s.CategoryKey,
		},
	}
}

func userToJSON(u *User) map[string]any {
	return map[string]any{
		"accountId":    u.AccountID,
		"displayName":  u.DisplayName,
		"emailAddress": u.Email,
		"active":       u.Active,
		"avatarUrls":   map[string]string{"48x48": u.AvatarURL},
	}
}

func componentsToJSON(components []Component) []any {
	result := make([]any, len(components))
	for i, c := range components {
		result[i] = map[string]any{"id": c.ID, "name": c.Name}
	}
	return result
}

func subtasksToJSON(subtasks []Issue) []any {
	result := make([]any, len(subtasks))
	for i := range subtasks {
		result[i] = issueToJSON(&subtasks[i])
	}
	return result
}

func issueLinksToJSON(links []IssueLink) []any {
	result := make([]any, len(links))
	for i, link := range links {
		l := map[string]any{
			"id": link.ID,
		}
		if link.Type != nil {
			l["type"] = map[string]any{
				"name": link.Type.Name, "inward": link.Type.Inward, "outward": link.Type.Outward,
			}
		}
		if link.InwardIssue != nil {
			l["inwardIssue"] = issueToJSON(link.InwardIssue)
		}
		if link.OutwardIssue != nil {
			l["outwardIssue"] = issueToJSON(link.OutwardIssue)
		}
		result[i] = l
	}
	return result
}

// descriptionADF returns raw ADF if available, otherwise converts plain text.
func descriptionADF(iss *Issue) any {
	if iss.DescriptionADF != nil {
		return iss.DescriptionADF
	}
	return textToADF(iss.Description)
}

// commentBodyADF returns raw ADF if available, otherwise converts plain text.
func commentBodyADF(c *Comment) any {
	if c.BodyADF != nil {
		return c.BodyADF
	}
	return textToADF(c.Body)
}

// textToADF converts plain text to Atlassian Document Format.
func textToADF(text string) any {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	content := make([]any, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			content = append(content, map[string]any{
				"type":    "paragraph",
				"content": []any{},
			})
			continue
		}
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": line},
			},
		})
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

func formatJiraTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04:05.000-0700")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
