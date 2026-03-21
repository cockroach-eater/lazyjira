//go:build demo

package jira

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DemoClient implements ClientInterface with in-memory fake data for demo mode.
type DemoClient struct {
	projects   []Project
	issues     map[string][]*Issue  // projectKey → issues
	issueIndex map[string]*Issue    // issueKey → issue
	comments   map[string][]Comment // issueKey → comments
	changelog  map[string][]ChangelogEntry
	onRequest  func(RequestLog)
}

// Compile-time check.
var _ ClientInterface = (*DemoClient)(nil)

// NewDemoClient creates a DemoClient populated with realistic fake data.
func NewDemoClient() *DemoClient {
	d := &DemoClient{
		issues:     make(map[string][]*Issue),
		issueIndex: make(map[string]*Issue),
		comments:   make(map[string][]Comment),
		changelog:  make(map[string][]ChangelogEntry),
	}
	d.initDemoData()
	return d
}

func (d *DemoClient) SetOnRequest(fn func(RequestLog)) { d.onRequest = fn }
func (d *DemoClient) SetCustomFields(_ []string)       {}

func (d *DemoClient) logRequest(method, path string) {
	if d.onRequest != nil {
		d.onRequest(RequestLog{
			Method:  method,
			Path:    path,
			Status:  200,
			Elapsed: 12 * time.Millisecond,
		})
	}
}

func (d *DemoClient) GetProjects(_ context.Context) ([]Project, error) {
	d.logRequest("GET", "/project/search")
	return d.projects, nil
}

var projectKeyRe = regexp.MustCompile(`(?i)project\s*=\s*"?(\w+)"?`)
var assigneeCurrentRe = regexp.MustCompile(`(?i)assignee\s*=\s*currentUser\(\)`)

func (d *DemoClient) SearchIssues(_ context.Context, jql string, startAt, maxResults int) (*SearchResult, error) {
	d.logRequest("GET", "/search/jql?jql="+jql)

	m := projectKeyRe.FindStringSubmatch(jql)
	if m == nil {
		return &SearchResult{}, nil
	}
	projectKey := strings.ToUpper(m[1])
	all := d.issues[projectKey]

	// Filter by assignee=currentUser() → "demo@lazyjira.dev" user
	var filtered []*Issue
	if assigneeCurrentRe.MatchString(jql) {
		for _, iss := range all {
			if iss.Assignee != nil && iss.Assignee.Email == "demo@lazyjira.dev" {
				filtered = append(filtered, iss)
			}
		}
	} else {
		filtered = all
	}

	total := len(filtered)
	if startAt >= total {
		return &SearchResult{Total: total, MaxResults: maxResults, StartAt: startAt}, nil
	}
	end := min(startAt+maxResults, total)
	issues := make([]Issue, end-startAt)
	for i, iss := range filtered[startAt:end] {
		issues[i] = *iss
	}
	return &SearchResult{Issues: issues, Total: total, MaxResults: maxResults, StartAt: startAt}, nil
}

func (d *DemoClient) GetIssue(_ context.Context, issueKey string) (*Issue, error) {
	d.logRequest("GET", "/issue/"+issueKey)
	iss, ok := d.issueIndex[issueKey]
	if !ok {
		return nil, fmt.Errorf("issue %s not found", issueKey)
	}
	cp := *iss
	return &cp, nil
}

func (d *DemoClient) GetComments(_ context.Context, issueKey string) ([]Comment, error) {
	d.logRequest("GET", "/issue/"+issueKey+"/comment")
	return d.comments[issueKey], nil
}

func (d *DemoClient) GetChangelog(_ context.Context, issueKey string) ([]ChangelogEntry, error) {
	d.logRequest("GET", "/issue/"+issueKey+"/changelog")
	return d.changelog[issueKey], nil
}

func (d *DemoClient) GetTransitions(_ context.Context, issueKey string) ([]Transition, error) {
	d.logRequest("GET", "/issue/"+issueKey+"/transitions")
	iss, ok := d.issueIndex[issueKey]
	if !ok {
		return nil, fmt.Errorf("issue %s not found", issueKey)
	}
	return transitionsForStatus(iss.Status.Name), nil
}

func transitionsForStatus(status string) []Transition {
	switch status {
	case "To Do":
		return []Transition{
			{ID: "11", Name: "Start Progress", To: &Status{ID: "3", Name: "In Progress", Description: "Work has begun on this issue", CategoryKey: "indeterminate"}},
		}
	case "In Progress":
		return []Transition{
			{ID: "21", Name: "Submit Review", To: &Status{ID: "4", Name: "In Review", Description: "Code is ready for peer review", CategoryKey: "indeterminate"}},
			{ID: "31", Name: "Mark Done", To: &Status{ID: "5", Name: "Done", Description: "Issue is resolved and verified", CategoryKey: "done"}},
		}
	case "In Review":
		return []Transition{
			{ID: "41", Name: "Approve", To: &Status{ID: "5", Name: "Done", Description: "Review passed — issue is complete", CategoryKey: "done"}},
			{ID: "51", Name: "Request Changes", To: &Status{ID: "3", Name: "In Progress", Description: "Changes requested by reviewer, needs rework", CategoryKey: "indeterminate"}},
		}
	case "Done":
		return []Transition{
			{ID: "61", Name: "Reopen", To: &Status{ID: "1", Name: "To Do", Description: "Issue needs additional work or was not fully resolved", CategoryKey: "new"}},
		}
	}
	return nil
}

func (d *DemoClient) DoTransition(_ context.Context, issueKey, transitionID string) error {
	d.logRequest("POST", "/issue/"+issueKey+"/transitions")
	iss, ok := d.issueIndex[issueKey]
	if !ok {
		return fmt.Errorf("issue %s not found", issueKey)
	}
	transitions := transitionsForStatus(iss.Status.Name)
	for _, t := range transitions {
		if t.ID == transitionID {
			oldStatus := iss.Status.Name
			iss.Status = t.To
			iss.Updated = time.Now()
			// Append live changelog entry.
			d.changelog[issueKey] = append(d.changelog[issueKey], ChangelogEntry{
				Author:  &User{AccountID: "u0", DisplayName: "Demo User", Email: "demo@lazyjira.dev", Active: true},
				Created: time.Now(),
				Items:   []ChangeItem{{Field: "status", FromString: oldStatus, ToString: t.To.Name}},
			})
			return nil
		}
	}
	return fmt.Errorf("transition %s not valid for issue %s", transitionID, issueKey)
}

func (d *DemoClient) GetMyIssues(ctx context.Context) ([]Issue, error) {
	result, err := d.SearchIssues(ctx, "project = SHOP AND assignee=currentUser() ORDER BY priority DESC", 0, 50)
	if err != nil {
		return nil, err
	}
	return result.Issues, nil
}

func (d *DemoClient) AddComment(_ context.Context, issueKey string, body any) (*Comment, error) {
	d.logRequest("POST", "/issue/"+issueKey+"/comment")
	id := fmt.Sprintf("cmt-%d", time.Now().UnixNano())
	c := Comment{
		ID:      id,
		Author:  &User{AccountID: "u0", DisplayName: "Demo User", Email: "demo@lazyjira.dev", Active: true},
		Created: time.Now(),
		Updated: time.Now(),
	}
	if body != nil {
		c.BodyADF = body
		c.Body = extractADFText(body)
	}
	d.comments[issueKey] = append(d.comments[issueKey], c)
	return &c, nil
}

func (d *DemoClient) UpdateComment(_ context.Context, issueKey, commentID string, body any) error {
	d.logRequest("PUT", "/issue/"+issueKey+"/comment/"+commentID)
	comments := d.comments[issueKey]
	for i := range comments {
		if comments[i].ID == commentID {
			if body != nil {
				comments[i].BodyADF = body
				comments[i].Body = extractADFText(body)
			}
			comments[i].Updated = time.Now()
			d.comments[issueKey] = comments
			return nil
		}
	}
	return fmt.Errorf("comment %s not found on %s", commentID, issueKey)
}
func (d *DemoClient) AssignIssue(_ context.Context, _ string, _ string) error { return nil }
func (d *DemoClient) GetBoards(_ context.Context) ([]Board, error)            { return nil, nil }
func (d *DemoClient) GetBoardIssues(_ context.Context, _ int, _ string) ([]Issue, error) {
	return nil, nil
}
func (d *DemoClient) UpdateIssue(_ context.Context, issueKey string, fields map[string]any) error {
	d.logRequest("PUT", "/issue/"+issueKey)
	iss, ok := d.issueIndex[issueKey]
	if !ok {
		return fmt.Errorf("issue %s not found", issueKey)
	}
	if summary, ok := fields["summary"].(string); ok {
		iss.Summary = summary
	}
	if desc, ok := fields["description"]; ok && desc != nil {
		iss.DescriptionADF = desc
		iss.Description = extractADFText(desc)
	}
	if p, ok := fields["priority"].(map[string]string); ok {
		if iss.Priority == nil {
			iss.Priority = &Priority{}
		}
		iss.Priority.ID = p["id"]
	}
	if v, ok := fields["assignee"]; ok {
		if v == nil {
			iss.Assignee = nil
		} else if m, ok := v.(map[string]string); ok {
			iss.Assignee = &User{AccountID: m["accountId"]}
		}
	}
	if labels, ok := fields["labels"].([]string); ok {
		iss.Labels = labels
	}
	if comps, ok := fields["components"].([]map[string]string); ok {
		demoComps, _ := d.GetComponents(context.Background(), "")
		nameMap := make(map[string]string)
		for _, dc := range demoComps {
			nameMap[dc.ID] = dc.Name
		}
		iss.Components = make([]Component, len(comps))
		for i, c := range comps {
			id := c["id"]
			iss.Components[i] = Component{ID: id, Name: nameMap[id]}
		}
	}
	if it, ok := fields["issuetype"].(map[string]string); ok {
		if iss.IssueType == nil {
			iss.IssueType = &IssueType{}
		}
		iss.IssueType.ID = it["id"]
	}
	iss.Updated = time.Now()
	return nil
}
func (d *DemoClient) CreateIssue(_ context.Context, _, _, _, _ string) (*Issue, error) {
	return nil, nil
}
func (d *DemoClient) GetUsers(_ context.Context, _ string) ([]User, error) {
	d.logRequest("GET", "/user/assignable/search")
	return []User{
		{AccountID: "u0", DisplayName: "Demo User", Email: "demo@lazyjira.dev", Active: true},
		{AccountID: "u1", DisplayName: "Alice Chen", Email: "alice@example.com", Active: true},
		{AccountID: "u2", DisplayName: "Bob Martinez", Email: "bob@example.com", Active: true},
		{AccountID: "u3", DisplayName: "Carol Kim", Email: "carol@example.com", Active: true},
		{AccountID: "u4", DisplayName: "Dave Patel", Email: "dave@example.com", Active: true},
		{AccountID: "u5", DisplayName: "Eve Johnson", Email: "eve@example.com", Active: true},
	}, nil
}

func (d *DemoClient) GetPriorities(_ context.Context) ([]Priority, error) {
	d.logRequest("GET", "/priority")
	return []Priority{
		{ID: "1", Name: "Critical"},
		{ID: "2", Name: "High"},
		{ID: "3", Name: "Medium"},
		{ID: "4", Name: "Low"},
	}, nil
}
func (d *DemoClient) GetSprints(_ context.Context, _ int) ([]Sprint, error) {
	return nil, nil
}

func (d *DemoClient) GetLabels(_ context.Context) ([]string, error) {
	d.logRequest("GET", "/label")
	return []string{
		"api", "auth", "backend", "bug", "checkout", "core", "database",
		"devops", "docs", "feature", "frontend", "infrastructure",
		"integration", "ios", "mobile", "monitoring", "notifications",
		"observability", "offline", "payments", "performance", "search",
		"security", "theme", "ui", "ux",
	}, nil
}

func (d *DemoClient) GetComponents(_ context.Context, _ string) ([]Component, error) {
	d.logRequest("GET", "/project/components")
	return []Component{
		{ID: "1", Name: "API"},
		{ID: "2", Name: "Frontend"},
		{ID: "3", Name: "Backend"},
	}, nil
}

func (d *DemoClient) GetIssueTypes(_ context.Context, _ string) ([]IssueType, error) {
	d.logRequest("GET", "/issuetype/project")
	return []IssueType{
		{ID: "1", Name: "Bug"},
		{ID: "2", Name: "Story"},
		{ID: "3", Name: "Task"},
		{ID: "4", Name: "Epic"},
	}, nil
}

// --- Demo data ---

func (d *DemoClient) initDemoData() {
	now := time.Now()
	day := 24 * time.Hour

	// Users
	alice := &User{AccountID: "u1", DisplayName: "Alice Chen", Email: "alice@example.com", Active: true}
	bob := &User{AccountID: "u2", DisplayName: "Bob Martinez", Email: "bob@example.com", Active: true}
	carol := &User{AccountID: "u3", DisplayName: "Carol Kim", Email: "carol@example.com", Active: true}
	dave := &User{AccountID: "u4", DisplayName: "Dave Patel", Email: "dave@example.com", Active: true}
	eve := &User{AccountID: "u5", DisplayName: "Eve Johnson", Email: "eve@example.com", Active: true}
	// Demo user (currentUser)
	demo := &User{AccountID: "u0", DisplayName: "Demo User", Email: "demo@lazyjira.dev", Active: true}

	// Statuses
	todo := &Status{ID: "1", Name: "To Do", CategoryKey: "new"}
	inProgress := &Status{ID: "3", Name: "In Progress", CategoryKey: "indeterminate"}
	inReview := &Status{ID: "4", Name: "In Review", CategoryKey: "indeterminate"}
	done := &Status{ID: "5", Name: "Done", CategoryKey: "done"}

	// Priorities
	critical := &Priority{ID: "1", Name: "Critical"}
	high := &Priority{ID: "2", Name: "High"}
	medium := &Priority{ID: "3", Name: "Medium"}
	low := &Priority{ID: "4", Name: "Low"}

	// Issue types
	story := &IssueType{ID: "10001", Name: "Story"}
	bug := &IssueType{ID: "10002", Name: "Bug"}
	task := &IssueType{ID: "10003", Name: "Task"}

	// Sprint
	sprint1 := &Sprint{ID: 1, Name: "Sprint 23", State: "active"}

	// Projects
	d.projects = []Project{
		{ID: "1", Key: "SHOP", Name: "Online Shop", Lead: alice},
		{ID: "2", Key: "PLAT", Name: "Platform Services", Lead: bob},
		{ID: "3", Key: "MOBI", Name: "Mobile App", Lead: carol},
	}

	// --- SHOP issues ---
	shopIssues := []*Issue{
		{
			ID: "101", Key: "SHOP-1", Summary: "Implement shopping cart persistence",
			Description: "Cart items should survive page reload and browser restart.\nUse localStorage for guest users, server-side for logged-in users.\nHandle merge conflicts when guest logs in with existing cart.",
			Status: inProgress, Priority: high, Assignee: demo, Reporter: alice,
			IssueType: story, Sprint: sprint1,
			Labels: []string{"frontend", "ux"}, Components: []Component{{ID: "c1", Name: "Cart"}},
			Created: now.Add(-10 * day), Updated: now.Add(-1 * day),
		},
		{
			ID: "102", Key: "SHOP-2", Summary: "Fix checkout total not updating on quantity change",
			Description: "When user changes quantity in cart, the total price doesn't recalculate.\nReproducible on Chrome and Firefox. Safari seems fine.\nRegression from the discount code feature.",
			Status: inReview, Priority: critical, Assignee: bob, Reporter: dave,
			IssueType: bug, Sprint: sprint1,
			Labels: []string{"bug", "checkout"}, Components: []Component{{ID: "c2", Name: "Checkout"}},
			Created: now.Add(-5 * day), Updated: now.Add(-6 * time.Hour),
		},
		{
			ID: "103", Key: "SHOP-3", Summary: "Add product search with Elasticsearch",
			Description: "Replace the current SQL LIKE search with Elasticsearch.\nSupport typo tolerance, faceted search, and autocomplete.\nIndex should update within 5 seconds of product changes.",
			Status: todo, Priority: high, Assignee: carol, Reporter: alice,
			IssueType: story,
			Labels: []string{"backend", "search"}, Components: []Component{{ID: "c3", Name: "Search"}},
			Created: now.Add(-14 * day), Updated: now.Add(-3 * day),
		},
		{
			ID: "104", Key: "SHOP-4", Summary: "Set up CI/CD pipeline for staging",
			Description: "Configure GitHub Actions to deploy to staging on every merge to main.\nInclude database migrations, smoke tests, and Slack notification.",
			Status: done, Priority: medium, Assignee: eve, Reporter: bob,
			IssueType: task,
			Labels: []string{"devops"}, Components: []Component{{ID: "c4", Name: "Infrastructure"}},
			Created: now.Add(-20 * day), Updated: now.Add(-7 * day),
		},
		{
			ID: "105", Key: "SHOP-5", Summary: "Product image zoom on hover",
			Description: "Implement a smooth zoom effect when hovering over product images.\nShould work on both desktop and mobile (pinch to zoom).",
			Status: inProgress, Priority: medium, Assignee: carol, Reporter: alice,
			IssueType: story, Sprint: sprint1,
			Labels: []string{"frontend", "ux"},
			Created: now.Add(-8 * day), Updated: now.Add(-2 * day),
		},
		{
			ID: "106", Key: "SHOP-6", Summary: "Order confirmation email not sent for PayPal orders",
			Description: "Customers paying via PayPal don't receive confirmation emails.\nStripe and bank transfer work fine. Issue started after the payment gateway update.",
			Status: todo, Priority: critical, Assignee: demo, Reporter: dave,
			IssueType: bug,
			Labels: []string{"bug", "payments"}, Components: []Component{{ID: "c5", Name: "Payments"}},
			Created: now.Add(-2 * day), Updated: now.Add(-1 * day),
		},
		{
			ID: "107", Key: "SHOP-7", Summary: "Implement wishlist feature",
			Description: "Users should be able to save items to a wishlist.\nWishlist should be shareable via link.\nItems should show if they're on sale.",
			Status: todo, Priority: low, Assignee: nil, Reporter: alice,
			IssueType: story,
			Labels: []string{"feature"},
			Created: now.Add(-30 * day), Updated: now.Add(-15 * day),
		},
		{
			ID: "108", Key: "SHOP-8", Summary: "Optimize product listing page load time",
			Description: "Product listing takes 3.2s to load. Target is under 1s.\nProfile and optimize database queries, add pagination, lazy load images.",
			Status: inProgress, Priority: high, Assignee: bob, Reporter: eve,
			IssueType: task, Sprint: sprint1,
			Labels: []string{"performance"},
			Created: now.Add(-6 * day), Updated: now.Add(-12 * time.Hour),
		},
		{
			ID: "109", Key: "SHOP-9", Summary: "Add discount code validation",
			Description: "Validate discount codes in real-time as the user types.\nShow remaining uses and expiry date. Prevent stacking of incompatible codes.",
			Status: inReview, Priority: medium, Assignee: demo, Reporter: alice,
			IssueType: story, Sprint: sprint1,
			Labels: []string{"checkout", "frontend"},
			Created: now.Add(-12 * day), Updated: now.Add(-1 * day),
		},
		{
			ID: "110", Key: "SHOP-10", Summary: "Write API documentation for partner integrations",
			Description: "Document all public API endpoints with request/response examples.\nUse OpenAPI 3.0 spec. Include authentication guide.",
			Status: todo, Priority: low, Assignee: bob, Reporter: alice,
			IssueType: task,
			Labels: []string{"docs"},
			Created: now.Add(-25 * day), Updated: now.Add(-20 * day),
		},
		{
			ID: "111", Key: "SHOP-11", Summary: "Mobile responsive checkout flow",
			Description: "Checkout flow breaks on screens under 375px.\nButtons overlap, form fields are too narrow. Needs complete mobile redesign.",
			Status: done, Priority: high, Assignee: carol, Reporter: dave,
			IssueType: bug,
			Labels: []string{"mobile", "checkout"},
			Created: now.Add(-18 * day), Updated: now.Add(-10 * day),
		},
		{
			ID: "112", Key: "SHOP-12", Summary: "Inventory sync with warehouse system",
			Description: "Real-time inventory sync between the shop and warehouse management system.\nUse webhook-based updates. Handle race conditions for limited stock items.",
			Status: todo, Priority: high, Assignee: eve, Reporter: bob,
			IssueType: story,
			Labels: []string{"backend", "integration"},
			Created: now.Add(-4 * day), Updated: now.Add(-3 * day),
		},
	}

	// Subtasks for SHOP-1
	shopIssues[0].Subtasks = []Issue{
		{Key: "SHOP-1a", Summary: "Implement localStorage adapter", Status: done, IssueType: task},
		{Key: "SHOP-1b", Summary: "Server-side cart merge logic", Status: inProgress, IssueType: task},
	}

	// Issue links
	shopIssues[1].IssueLinks = []IssueLink{
		{
			ID:   "lnk1",
			Type: &IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
			OutwardIssue: &Issue{Key: "SHOP-4", Summary: "Set up CI/CD pipeline for staging", Status: done},
		},
	}
	shopIssues[5].IssueLinks = []IssueLink{
		{
			ID:   "lnk2",
			Type: &IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
			InwardIssue: &Issue{Key: "SHOP-2", Summary: "Fix checkout total not updating on quantity change", Status: inReview},
		},
	}

	// --- PLAT issues ---
	platIssues := []*Issue{
		{
			ID: "201", Key: "PLAT-1", Summary: "Migrate auth service to OAuth 2.0",
			Description: "Replace legacy session-based auth with OAuth 2.0 + PKCE.\nSupport Google, GitHub, and SAML SSO providers.\nMaintain backward compatibility during migration.",
			Status: inProgress, Priority: critical, Assignee: bob, Reporter: bob,
			IssueType: story, Sprint: sprint1,
			Labels: []string{"auth", "security"},
			Created: now.Add(-15 * day), Updated: now.Add(-1 * day),
		},
		{
			ID: "202", Key: "PLAT-2", Summary: "Set up distributed tracing with OpenTelemetry",
			Description: "Instrument all services with OpenTelemetry.\nSet up Jaeger for trace visualization. Add custom spans for database queries.",
			Status: todo, Priority: high, Assignee: eve, Reporter: bob,
			IssueType: task,
			Labels: []string{"observability"},
			Created: now.Add(-10 * day), Updated: now.Add(-5 * day),
		},
		{
			ID: "203", Key: "PLAT-3", Summary: "Rate limiter returns 500 instead of 429",
			Description:    extractADFText(plat3ADF()),
			DescriptionADF: plat3ADF(),
			Status: inReview, Priority: high, Assignee: demo, Reporter: dave,
			IssueType: bug,
			Labels: []string{"bug", "api"},
			Created: now.Add(-3 * day), Updated: now.Add(-8 * time.Hour),
		},
		{
			ID: "204", Key: "PLAT-4", Summary: "Implement service mesh with Consul",
			Description: "Replace manual service discovery with Consul Connect.\nEnable mTLS between services. Set up traffic splitting for canary deploys.",
			Status: todo, Priority: medium, Assignee: nil, Reporter: bob,
			IssueType: story,
			Labels: []string{"infrastructure"},
			Created: now.Add(-22 * day), Updated: now.Add(-18 * day),
		},
		{
			ID: "205", Key: "PLAT-5", Summary: "Database connection pool exhaustion under load",
			Description: "Under sustained load (>1000 RPS), connection pool fills up and queries time out.\nNeed to implement connection pooling with PgBouncer and query optimization.",
			Status: inProgress, Priority: critical, Assignee: bob, Reporter: eve,
			IssueType: bug, Sprint: sprint1,
			Labels: []string{"database", "performance"},
			Created: now.Add(-4 * day), Updated: now.Add(-6 * time.Hour),
		},
		{
			ID: "206", Key: "PLAT-6", Summary: "Add Prometheus metrics for all API endpoints",
			Description: "Expose request duration, error rate, and throughput metrics.\nCreate Grafana dashboards for each service.",
			Status: done, Priority: medium, Assignee: eve, Reporter: bob,
			IssueType: task,
			Labels: []string{"observability", "monitoring"},
			Created: now.Add(-30 * day), Updated: now.Add(-12 * day),
		},
		{
			ID: "207", Key: "PLAT-7", Summary: "Implement API versioning strategy",
			Description: "Design and implement API versioning using URL path (/v1/, /v2/).\nDocument migration guide for consumers. Set up automated deprecation notices.",
			Status: todo, Priority: low, Assignee: demo, Reporter: alice,
			IssueType: task,
			Labels: []string{"api", "docs"},
			Created: now.Add(-16 * day), Updated: now.Add(-14 * day),
		},
		{
			ID: "208", Key: "PLAT-8", Summary: "Centralized configuration management",
			Description: "Move from per-service config files to centralized config with Vault.\nSupport dynamic config reloading without restarts.",
			Status: todo, Priority: medium, Assignee: eve, Reporter: bob,
			IssueType: story,
			Labels: []string{"infrastructure", "devops"},
			Created: now.Add(-8 * day), Updated: now.Add(-6 * day),
		},
	}

	platIssues[0].Subtasks = []Issue{
		{Key: "PLAT-1a", Summary: "OAuth provider integration", Status: done, IssueType: task},
		{Key: "PLAT-1b", Summary: "Token refresh flow", Status: inProgress, IssueType: task},
		{Key: "PLAT-1c", Summary: "Migration script for existing sessions", Status: todo, IssueType: task},
	}

	// PLAT-3 subtasks
	platIssues[2].Subtasks = []Issue{
		{Key: "PLAT-3a", Summary: "Add RateLimitError type to error enum", Status: done, IssueType: task},
		{Key: "PLAT-3b", Summary: "Map 429 in error handler + Retry-After header", Status: done, IssueType: task},
		{Key: "PLAT-3c", Summary: "Integration tests for rate limit responses", Status: inReview, IssueType: task},
	}
	// PLAT-3 issue links
	platIssues[2].IssueLinks = []IssueLink{
		{
			ID:   "lnk3",
			Type: &IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
			OutwardIssue: &Issue{Key: "PLAT-1", Summary: "Migrate auth service to OAuth 2.0", Status: inProgress},
		},
		{
			ID:   "lnk4",
			Type: &IssueLinkType{Name: "Relates", Inward: "relates to", Outward: "relates to"},
			OutwardIssue: &Issue{Key: "PLAT-5", Summary: "Database connection pool exhaustion under load", Status: inProgress},
		},
	}

	// --- MOBI issues ---
	mobiIssues := []*Issue{
		{
			ID: "301", Key: "MOBI-1", Summary: "Implement offline mode for product browsing",
			Description: "Cache product catalog for offline access.\nSync changes when back online. Show clear offline indicator.",
			Status: inProgress, Priority: high, Assignee: carol, Reporter: carol,
			IssueType: story, Sprint: sprint1,
			Labels: []string{"offline", "core"},
			Created: now.Add(-12 * day), Updated: now.Add(-2 * day),
		},
		{
			ID: "302", Key: "MOBI-2", Summary: "Push notifications for order status updates",
			Description: "Send push notifications when order status changes.\nSupport both iOS and Android. Allow users to customize notification preferences.",
			Status: todo, Priority: medium, Assignee: demo, Reporter: carol,
			IssueType: story,
			Labels: []string{"notifications"},
			Created: now.Add(-9 * day), Updated: now.Add(-5 * day),
		},
		{
			ID: "303", Key: "MOBI-3", Summary: "App crashes on iOS 17 when opening camera for barcode scan",
			Description: "App crashes immediately when accessing camera on iOS 17.2+.\nPermission dialog appears but app terminates before user can respond.\nWorks fine on iOS 16.",
			Status: inReview, Priority: critical, Assignee: carol, Reporter: dave,
			IssueType: bug,
			Labels: []string{"ios", "crash"},
			Created: now.Add(-2 * day), Updated: now.Add(-4 * time.Hour),
		},
		{
			ID: "304", Key: "MOBI-4", Summary: "Biometric authentication for checkout",
			Description: "Add Face ID / Touch ID / fingerprint confirmation before placing orders.\nFallback to PIN code. Remember preference per device.",
			Status: todo, Priority: medium, Assignee: nil, Reporter: alice,
			IssueType: story,
			Labels: []string{"security", "checkout"},
			Created: now.Add(-20 * day), Updated: now.Add(-15 * day),
		},
		{
			ID: "305", Key: "MOBI-5", Summary: "Reduce app binary size from 85MB to under 50MB",
			Description: "Audit dependencies, remove unused assets, enable app thinning.\nSplit by architecture. Defer loading of non-critical modules.",
			Status: inProgress, Priority: low, Assignee: carol, Reporter: eve,
			IssueType: task,
			Labels: []string{"performance", "build"},
			Created: now.Add(-7 * day), Updated: now.Add(-3 * day),
		},
		{
			ID: "306", Key: "MOBI-6", Summary: "Dark mode support",
			Description: "Implement full dark mode theme following platform guidelines.\nRespect system setting. Allow manual toggle in app settings.",
			Status: done, Priority: low, Assignee: carol, Reporter: carol,
			IssueType: story,
			Labels: []string{"ui", "theme"},
			Created: now.Add(-25 * day), Updated: now.Add(-8 * day),
		},
	}

	// Register all issues
	for _, iss := range shopIssues {
		d.addIssue("SHOP", iss)
	}
	for _, iss := range platIssues {
		d.addIssue("PLAT", iss)
	}
	for _, iss := range mobiIssues {
		d.addIssue("MOBI", iss)
	}

	// Comments
	d.comments["SHOP-1"] = []Comment{
		{ID: "c1", Author: alice, Body: "Should we also persist the cart for anonymous users? We could use a session cookie as fallback.", Created: now.Add(-9 * day), Updated: now.Add(-9 * day)},
		{ID: "c2", Author: demo, Body: "Good point. I'll use localStorage for anonymous and merge when they sign in. Added SHOP-1b subtask for the merge logic.", Created: now.Add(-8 * day), Updated: now.Add(-8 * day)},
		{ID: "c3", Author: bob, Body: "Make sure to handle the edge case where the same product exists in both carts with different options (size, color).", Created: now.Add(-7 * day), Updated: now.Add(-7 * day)},
	}
	d.comments["SHOP-2"] = []Comment{
		{ID: "c4", Author: dave, Body: "Reproduced consistently. The event listener on quantity input fires but updateTotal() uses stale DOM values.", Created: now.Add(-4 * day), Updated: now.Add(-4 * day)},
		{ID: "c5", Author: bob, Body: "Found it — the discount code feature added a debounce that delays the price recalculation. Fixing now.", Created: now.Add(-3 * day), Updated: now.Add(-3 * day)},
	}
	d.comments["SHOP-6"] = []Comment{
		{ID: "c6", Author: eve, Body: "Checked the logs — PayPal webhook is returning success but our handler isn't triggering the email service. Looks like a missing event mapping.", Created: now.Add(-1 * day), Updated: now.Add(-1 * day)},
	}
	d.comments["PLAT-1"] = []Comment{
		{ID: "c7", Author: bob, Body: "OAuth provider integration is done. Moving on to token refresh flow. The tricky part is handling concurrent refresh requests.", Created: now.Add(-5 * day), Updated: now.Add(-5 * day)},
		{ID: "c8", Author: alice, Body: "Are we supporting refresh token rotation? It's recommended by the OAuth 2.1 draft.", Created: now.Add(-4 * day), Updated: now.Add(-4 * day)},
		{ID: "c9", Author: bob, Body: "Yes, implementing rotation with replay detection. If a stolen refresh token is used, all tokens for that session get revoked.", Created: now.Add(-3 * day), Updated: now.Add(-3 * day)},
	}
	d.comments["PLAT-3"] = []Comment{
		{ID: "c10", Author: dave,
			Body: extractADFText(plat3Comment1ADF()), BodyADF: plat3Comment1ADF(),
			Created: now.Add(-3 * day), Updated: now.Add(-3 * day)},
		{ID: "c16", Author: demo,
			Body: extractADFText(plat3Comment2ADF()), BodyADF: plat3Comment2ADF(),
			Created: now.Add(-2 * day), Updated: now.Add(-2 * day)},
		{ID: "c17", Author: bob,
			Body: extractADFText(plat3Comment3ADF()), BodyADF: plat3Comment3ADF(),
			Created: now.Add(-36 * time.Hour), Updated: now.Add(-36 * time.Hour)},
		{ID: "c18", Author: demo,
			Body: extractADFText(plat3Comment4ADF()), BodyADF: plat3Comment4ADF(),
			Created: now.Add(-12 * time.Hour), Updated: now.Add(-12 * time.Hour)},
	}
	d.comments["PLAT-5"] = []Comment{
		{ID: "c11", Author: eve, Body: "Added PgBouncer to the staging environment. Connection usage dropped from 200 to 15 under the same load. Preparing production rollout.", Created: now.Add(-2 * day), Updated: now.Add(-2 * day)},
		{ID: "c12", Author: bob, Body: "Also found two N+1 queries in the order service that were burning 40% of connections. Fixed in the same PR.", Created: now.Add(-1 * day), Updated: now.Add(-1 * day)},
	}
	d.comments["MOBI-1"] = []Comment{
		{ID: "c13", Author: carol, Body: "Using Core Data for local cache. Sync strategy: last-write-wins for simple fields, merge for arrays (cart items).", Created: now.Add(-8 * day), Updated: now.Add(-8 * day)},
	}
	d.comments["MOBI-3"] = []Comment{
		{ID: "c14", Author: dave, Body: "Crash log points to NSCameraUsageDescription missing for the new entitlement structure in iOS 17. The key is present but needs to be under a different dict in the updated Info.plist format.", Created: now.Add(-1 * day), Updated: now.Add(-1 * day)},
		{ID: "c15", Author: carol, Body: "Fixed and verified on iOS 17.2, 17.3, and 17.4 beta. Also added a pre-check that gracefully handles missing permissions.", Created: now.Add(-4 * time.Hour), Updated: now.Add(-4 * time.Hour)},
	}

	// Changelog
	d.changelog["SHOP-1"] = []ChangelogEntry{
		{Author: alice, Created: now.Add(-10 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: demo, Created: now.Add(-8 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
		{Author: alice, Created: now.Add(-7 * day), Items: []ChangeItem{{Field: "priority", FromString: "Medium", ToString: "High"}}},
		{Author: alice, Created: now.Add(-6 * day), Items: []ChangeItem{{Field: "description",
			FromString: "Cart items should survive page reload and browser restart.\nUse localStorage for guest users, server-side for logged-in users.",
			ToString:   "Cart items should survive page reload and browser restart.\nUse localStorage for guest users, server-side for logged-in users.\nHandle merge conflicts when guest logs in with existing cart.\n\nAcceptance Criteria:\n- Guest cart persists across browser sessions using localStorage\n- Logged-in user cart is stored server-side in Redis with 30-day TTL\n- When guest with items logs in, show merge dialog if conflicts exist\n- Merge strategies: keep guest, keep server, combine (sum quantities)\n- Cart items include: product ID, variant, quantity, added timestamp\n- Maximum 50 items per cart, show warning at 45\n- Cart sync runs on page load and every 60 seconds for logged-in users\n\nTechnical Notes:\n- localStorage key: 'lazycart_v2_items' (JSON array)\n- Redis key pattern: 'cart:{userId}' with HASH type\n- Merge conflict detection: compare by productId+variantId\n- Use optimistic locking for concurrent cart updates\n- Fallback to cookie-based storage if localStorage is unavailable",
		}}},
	}
	d.changelog["SHOP-2"] = []ChangelogEntry{
		{Author: dave, Created: now.Add(-5 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: dave, Created: now.Add(-5 * day), Items: []ChangeItem{{Field: "priority", FromString: "High", ToString: "Critical"}}},
		{Author: bob, Created: now.Add(-3 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
		{Author: bob, Created: now.Add(-1 * day), Items: []ChangeItem{{Field: "status", FromString: "In Progress", ToString: "In Review"}}},
	}
	d.changelog["SHOP-9"] = []ChangelogEntry{
		{Author: demo, Created: now.Add(-10 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
		{Author: demo, Created: now.Add(-2 * day), Items: []ChangeItem{{Field: "status", FromString: "In Progress", ToString: "In Review"}}},
		{Author: alice, Created: now.Add(-2 * day), Items: []ChangeItem{{Field: "assignee", FromString: "Carol Kim", ToString: "Demo User"}}},
	}
	d.changelog["PLAT-1"] = []ChangelogEntry{
		{Author: bob, Created: now.Add(-15 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: bob, Created: now.Add(-14 * day), Items: []ChangeItem{{Field: "description",
			FromString: "Replace legacy session-based auth with OAuth 2.0.",
			ToString:   "Replace legacy session-based auth with OAuth 2.0 + PKCE.\nSupport Google, GitHub, and SAML SSO providers.\nMaintain backward compatibility during migration.\n\nMigration Plan:\n1. Deploy OAuth endpoints alongside existing session auth\n2. New logins use OAuth, existing sessions remain valid\n3. Background job converts active sessions to OAuth tokens (2 week window)\n4. After migration window, disable session auth endpoints\n5. Remove session tables from database\n\nSecurity Requirements:\n- PKCE required for all public clients (mobile, SPA)\n- Refresh token rotation with replay detection\n- Access token lifetime: 15 minutes\n- Refresh token lifetime: 30 days (sliding window)\n- Rate limit: 10 failed auth attempts per minute per IP\n- All tokens stored as bcrypt hashes, never plaintext\n- Revocation endpoint must invalidate within 30 seconds\n\nSSO Configuration:\n- Google: OpenID Connect discovery, scopes: openid profile email\n- GitHub: OAuth 2.0 with user:email scope\n- SAML: Support both IdP-initiated and SP-initiated flows\n- Attribute mapping configurable per tenant\n- JIT provisioning with default role assignment",
		}}},
		{Author: bob, Created: now.Add(-12 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
	}
	d.changelog["PLAT-3"] = []ChangelogEntry{
		{Author: dave, Created: now.Add(-3 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: dave, Created: now.Add(-3 * day), Items: []ChangeItem{{Field: "priority", FromString: "Medium", ToString: "High"}}},
		{Author: demo, Created: now.Add(-2 * day), Items: []ChangeItem{{Field: "assignee", FromString: "Dave Patel", ToString: "Demo User"}}},
		{Author: demo, Created: now.Add(-2 * day), Items: []ChangeItem{{Field: "description",
			FromString: "When rate limit is exceeded, the API returns 500 Internal Server Error.\nShould return 429 Too Many Requests with Retry-After header.",
			ToString:   "When rate limit is exceeded, the API returns 500 Internal Server Error.\nShould return 429 Too Many Requests with Retry-After header.\n\nRoot Cause:\nThe rate limiter middleware catches the RateLimitExceeded exception but\nre-throws it as a generic InternalError. The error mapping in\nerror_handler.go only has explicit cases for AuthError, ValidationError,\nand NotFoundError — everything else falls through to 500.\n\nFix:\n1. Add RateLimitError to the error type enum in pkg/errors/types.go\n2. Map RateLimitError → 429 in error_handler.go\n3. Set Retry-After header from the limiter's reset timestamp\n4. Add X-RateLimit-Remaining and X-RateLimit-Limit headers\n5. Return JSON body: {\"error\": \"rate_limit_exceeded\", \"retry_after\": N}\n\nTesting:\n- Unit test: verify 429 status and headers when limit exceeded\n- Integration test: hit endpoint 100 times, confirm 429 after threshold\n- Load test: confirm Retry-After values are accurate under sustained load\n- Verify existing 500 errors for real server errors are not affected\n\nRollout:\n- Deploy behind feature flag rate_limiter_v2\n- Enable on staging, soak for 24h\n- Enable on production with 10% traffic, then 50%, then 100%\n- Monitor error rate dashboard for false positives",
		}}},
		{Author: demo, Created: now.Add(-36 * time.Hour), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
		{Author: demo, Created: now.Add(-12 * time.Hour), Items: []ChangeItem{{Field: "labels", FromString: "bug", ToString: "bug, api"}}},
		{Author: demo, Created: now.Add(-8 * time.Hour), Items: []ChangeItem{{Field: "status", FromString: "In Progress", ToString: "In Review"}}},
	}
	d.changelog["PLAT-5"] = []ChangelogEntry{
		{Author: eve, Created: now.Add(-4 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: eve, Created: now.Add(-4 * day), Items: []ChangeItem{{Field: "priority", FromString: "High", ToString: "Critical"}}},
		{Author: bob, Created: now.Add(-3 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
		{Author: bob, Created: now.Add(-3 * day), Items: []ChangeItem{{Field: "assignee", FromString: "Eve Johnson", ToString: "Bob Martinez"}}},
	}
	d.changelog["SHOP-3"] = []ChangelogEntry{
		{Author: alice, Created: now.Add(-14 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: carol, Created: now.Add(-10 * day), Items: []ChangeItem{{Field: "description",
			FromString: "Replace the current SQL LIKE search with Elasticsearch.",
			ToString:   "Replace the current SQL LIKE search with Elasticsearch.\nSupport typo tolerance, faceted search, and autocomplete.\nIndex should update within 5 seconds of product changes.\n\nSearch Features:\n- Full-text search across name, description, SKU, and tags\n- Fuzzy matching with edit distance 2 for typo tolerance\n- Faceted filtering: category, price range, brand, rating, availability\n- Autocomplete suggestions with product thumbnails\n- Recent searches per user (stored client-side)\n- Search analytics: top queries, zero-result queries, click-through rate\n\nIndexing Strategy:\n- Elasticsearch 8.x with 2 shards, 1 replica\n- Index mapping: keyword fields for filters, text fields with custom analyzer\n- Custom analyzer: lowercase, asciifolding, edge_ngram (2-15) for autocomplete\n- Sync via CDC (Change Data Capture) from PostgreSQL using Debezium\n- Bulk reindex job runs nightly, incremental updates via CDC during the day\n- Index aliases for zero-downtime reindexing\n\nAPI Design:\n- GET /api/search?q=...&category=...&minPrice=...&maxPrice=...\n- GET /api/search/suggest?q=... (autocomplete)\n- Response includes: hits, facets, total count, took_ms\n- Pagination via search_after (no deep pagination)\n- Cache frequent queries in Redis with 60s TTL",
		}}},
	}
	d.changelog["MOBI-1"] = []ChangelogEntry{
		{Author: carol, Created: now.Add(-12 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: carol, Created: now.Add(-11 * day), Items: []ChangeItem{{Field: "description",
			FromString: "Cache product catalog for offline access.",
			ToString:   "Cache product catalog for offline access.\nSync changes when back online. Show clear offline indicator.\n\nOffline Storage:\n- Product catalog: Core Data (iOS) / Room (Android)\n- Images: disk cache with LRU eviction, max 200MB\n- Cart: local-first, sync on reconnect with conflict resolution\n- User preferences: UserDefaults / SharedPreferences\n\nSync Protocol:\n- On app launch: check connectivity, fetch delta since last sync timestamp\n- Delta endpoint: GET /api/catalog/delta?since=<timestamp>\n- Response: created, updated, deleted product IDs with full data\n- Conflict resolution: server wins for catalog data, merge for cart\n- Background sync every 15 minutes when online (iOS BGTaskScheduler)\n- Manual pull-to-refresh triggers immediate full sync\n\nUI Behavior:\n- Offline banner: yellow bar at top \"You're offline — showing cached data\"\n- Stale data indicator: gray timestamp on products older than 24h\n- Disable actions that require network: checkout, reviews, wishlists\n- Show cached product count in offline banner\n- Graceful degradation: search works on cached data only\n- Queue actions (add to cart, wishlist) for replay when back online",
		}}},
		{Author: carol, Created: now.Add(-10 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
	}
	d.changelog["MOBI-3"] = []ChangelogEntry{
		{Author: dave, Created: now.Add(-2 * day), Items: []ChangeItem{{Field: "status", FromString: "", ToString: "To Do"}}},
		{Author: carol, Created: now.Add(-1 * day), Items: []ChangeItem{{Field: "status", FromString: "To Do", ToString: "In Progress"}}},
		{Author: carol, Created: now.Add(-4 * time.Hour), Items: []ChangeItem{{Field: "status", FromString: "In Progress", ToString: "In Review"}}},
	}
}

func (d *DemoClient) addIssue(projectKey string, iss *Issue) {
	d.issues[projectKey] = append(d.issues[projectKey], iss)
	d.issueIndex[iss.Key] = iss
}

// --- ADF helpers for demo comments ---

func adfDoc(content ...any) any {
	return map[string]any{"type": "doc", "version": float64(1), "content": content}
}

func adfPara(content ...any) map[string]any {
	return map[string]any{"type": "paragraph", "content": content}
}

func adfText(t string) map[string]any {
	return map[string]any{"type": "text", "text": t}
}

func adfBold(t string) map[string]any {
	return map[string]any{"type": "text", "text": t, "marks": []any{map[string]any{"type": "strong"}}}
}

func adfCode(t string) map[string]any {
	return map[string]any{"type": "text", "text": t, "marks": []any{map[string]any{"type": "code"}}}
}

func adfLink(t, href string) map[string]any {
	return map[string]any{"type": "text", "text": t, "marks": []any{
		map[string]any{"type": "link", "attrs": map[string]any{"href": href}},
	}}
}

func adfCodeBlock(lang, body string) map[string]any {
	return map[string]any{
		"type": "codeBlock", "attrs": map[string]any{"language": lang},
		"content": []any{adfText(body)},
	}
}

func plat3Comment1ADF() any {
	return adfDoc(
		adfPara(
			adfText("Reproduced in staging — sending "),
			adfBold("50 req/s"),
			adfText(" to "),
			adfCode("/api/users"),
			adfText(", after threshold we get "),
			adfBold("500"),
			adfText(" with generic error body. No "),
			adfCode("Retry-After"),
			adfText(" header at all."),
		),
		adfPara(
			adfText("Response body:"),
		),
		adfCodeBlock("json", `{
  "error": "internal_server_error",
  "message": "An unexpected error occurred"
}`),
		adfPara(
			adfText("Logs: "),
			adfLink("Kibana — plat-3-repro", "https://kibana.internal/app/discover#/plat-3-repro"),
		),
	)
}

func plat3Comment2ADF() any {
	return adfDoc(
		adfPara(
			adfText("Found the root cause. The middleware catches "),
			adfCode("RateLimitExceeded"),
			adfText(" but re-throws as "),
			adfCode("InternalError"),
			adfText(". The error handler in "),
			adfCode("error_handler.go"),
			adfText(" doesn't have a case for rate limit errors, so it falls through to the default 500."),
		),
		adfPara(
			adfText("The fix is in "),
			adfCode("mapErrorToStatus()"),
			adfText(":"),
		),
		adfCodeBlock("go", `case *errors.RateLimitError:
    w.Header().Set("Retry-After", strconv.Itoa(e.RetryAfter))
    w.Header().Set("X-RateLimit-Remaining", "0")
    writeJSON(w, 429, errorResponse{
        Error:   "rate_limit_exceeded",
        Message: e.Error(),
    })`),
	)
}

func plat3Comment3ADF() any {
	return adfDoc(
		adfPara(
			adfText("Good catch. Make sure the "),
			adfCode("Retry-After"),
			adfText(" header uses the "),
			adfBold("actual reset time"),
			adfText(" from the rate limiter, not a hardcoded value. Also add "),
			adfCode("X-RateLimit-Remaining"),
			adfText(" for client-side backoff."),
		),
		adfPara(
			adfText("See "),
			adfLink("MDN — Retry-After", "https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After"),
			adfText(" for spec."),
		),
	)
}

func plat3Comment4ADF() any {
	return adfDoc(
		adfPara(
			adfText("Done — PR is up "),
			adfLink("acme/platform#847", "https://github.com/acme/platform/pull/847"),
			adfText(" — added all three headers. Integration tests pass on staging."),
		),
		adfPara(
			adfText("Response now:"),
		),
		adfCodeBlock("http", `HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
Content-Type: application/json

{"error": "rate_limit_exceeded", "retry_after": 30}`),
	)
}

// plat3ADF returns a rich ADF document for the PLAT-3 demo issue.
// Showcases headings, bold, code blocks, lists, blockquote, rule, inline code, links.
//
//nolint:funlen // demo data generator
func plat3ADF() any {
	text := func(t string) map[string]any {
		return map[string]any{"type": "text", "text": t}
	}
	bold := func(t string) map[string]any {
		return map[string]any{"type": "text", "text": t, "marks": []any{
			map[string]any{"type": "strong"},
		}}
	}
	code := func(t string) map[string]any {
		return map[string]any{"type": "text", "text": t, "marks": []any{
			map[string]any{"type": "code"},
		}}
	}
	italic := func(t string) map[string]any {
		return map[string]any{"type": "text", "text": t, "marks": []any{
			map[string]any{"type": "em"},
		}}
	}
	link := func(t, href string) map[string]any {
		return map[string]any{"type": "text", "text": t, "marks": []any{
			map[string]any{"type": "link", "attrs": map[string]any{"href": href}},
		}}
	}
	heading := func(level int, t string) map[string]any {
		return map[string]any{
			"type":    "heading",
			"attrs":   map[string]any{"level": float64(level)},
			"content": []any{text(t)},
		}
	}
	para := func(content ...any) map[string]any {
		return map[string]any{"type": "paragraph", "content": content}
	}
	bullet := func(items ...any) map[string]any {
		listItems := make([]any, len(items))
		for i, item := range items {
			listItems[i] = map[string]any{
				"type": "listItem",
				"content": []any{
					item,
				},
			}
		}
		return map[string]any{"type": "bulletList", "content": listItems}
	}
	ordered := func(items ...any) map[string]any {
		listItems := make([]any, len(items))
		for i, item := range items {
			listItems[i] = map[string]any{
				"type": "listItem",
				"content": []any{
					item,
				},
			}
		}
		return map[string]any{"type": "orderedList", "content": listItems}
	}
	codeBlock := func(lang, body string) map[string]any {
		return map[string]any{
			"type":    "codeBlock",
			"attrs":   map[string]any{"language": lang},
			"content": []any{text(body)},
		}
	}
	blockquote := func(content ...any) map[string]any {
		return map[string]any{"type": "blockquote", "content": content}
	}
	rule := map[string]any{"type": "rule"}

	return map[string]any{
		"type":    "doc",
		"version": float64(1),
		"content": []any{
			// Problem
			heading(2, "Problem"),
			para(
				text("When the rate limit is exceeded, the API returns "),
				bold("500 Internal Server Error"),
				text(" instead of the correct "),
				code("429 Too Many Requests"),
				text(" response. Clients cannot distinguish rate limiting from real server errors."),
			),

			// Root Cause
			heading(2, "Root Cause"),
			para(
				text("The rate limiter middleware catches "),
				code("RateLimitExceeded"),
				text(" but re-throws it as a generic "),
				code("InternalError"),
				text(". The error mapping in "),
				code("error_handler.go"),
				text(" only handles:"),
			),
			bullet(
				para(code("AuthError"), text(" → 401")),
				para(code("ValidationError"), text(" → 400")),
				para(code("NotFoundError"), text(" → 404")),
			),
			para(
				italic("Everything else falls through to 500."),
			),

			rule,

			// Fix
			heading(2, "Fix"),
			ordered(
				para(text("Add "), code("RateLimitError"), text(" to the error type enum in "), code("pkg/errors/types.go")),
				para(text("Map "), code("RateLimitError"), text(" → "), bold("429"), text(" in "), code("error_handler.go")),
				para(text("Set "), code("Retry-After"), text(" header from the limiter's reset timestamp")),
				para(text("Add "), code("X-RateLimit-Remaining"), text(" and "), code("X-RateLimit-Limit"), text(" headers")),
			),

			// Response format
			heading(3, "Expected response format"),
			codeBlock("json", `{
  "error": "rate_limit_exceeded",
  "message": "API rate limit exceeded",
  "retry_after": 30
}`),

			// Testing
			heading(2, "Testing"),
			bullet(
				para(bold("Unit test:"), text(" verify 429 status and headers when limit exceeded")),
				para(bold("Integration test:"), text(" hit endpoint 100 times, confirm 429 after threshold")),
				para(bold("Load test:"), text(" confirm Retry-After values are accurate under sustained load")),
			),

			// Blockquote
			blockquote(
				para(
					text("See "), link("RFC 6585 §4", "https://datatracker.ietf.org/doc/html/rfc6585#section-4"),
					text(" for the 429 status code specification."),
				),
			),

			rule,

			// References
			heading(3, "References"),
			bullet(
				para(text("PR: "), link("acme/platform#847", "https://github.com/acme/platform/pull/847")),
				para(text("Dashboard: "), link("Grafana API Errors", "https://grafana.internal/d/api-errors")),
			),
		},
	}
}
