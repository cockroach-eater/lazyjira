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
	case strings.HasSuffix(path, "/comment"):
		key := extractKeyFromPath(path, "/comment")
		s.handleComments(w, key)
	case strings.HasSuffix(path, "/changelog"):
		key := extractKeyFromPath(path, "/changelog")
		s.handleChangelog(w, key)
	case strings.HasPrefix(path, "/issue/"):
		key := strings.TrimPrefix(path, "/issue/")
		s.handleIssue(w, key)
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

// --- JSON serialization helpers ---

func issueToJSON(iss *Issue) map[string]any {
	labels := iss.Labels
	if labels == nil {
		labels = []string{}
	}

	fields := map[string]any{
		"summary":     iss.Summary,
		"description": textToADF(iss.Description),
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
		"body":    textToADF(c.Body),
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
