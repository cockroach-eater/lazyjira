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

func (d *DemoClient) AddComment(_ context.Context, _ string, _ string) (*Comment, error) {
	return nil, nil
}
func (d *DemoClient) AssignIssue(_ context.Context, _ string, _ string) error { return nil }
func (d *DemoClient) GetBoards(_ context.Context) ([]Board, error)            { return nil, nil }
func (d *DemoClient) GetBoardIssues(_ context.Context, _ int, _ string) ([]Issue, error) {
	return nil, nil
}
func (d *DemoClient) UpdateIssue(_ context.Context, _ string, _ map[string]any) error { return nil }
func (d *DemoClient) CreateIssue(_ context.Context, _, _, _, _ string) (*Issue, error) {
	return nil, nil
}
func (d *DemoClient) GetUsers(_ context.Context, _ string) ([]User, error) { return nil, nil }
func (d *DemoClient) GetSprints(_ context.Context, _ int) ([]Sprint, error) {
	return nil, nil
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
			Description: "When rate limit is exceeded, the API returns 500 Internal Server Error.\nShould return 429 Too Many Requests with Retry-After header.\n\nSee RFC 6585 https://datatracker.ietf.org/doc/html/rfc6585#section-4\nRelated PR: https://github.com/acme/platform/pull/847\nGrafana dashboard: https://grafana.internal/d/api-errors",
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
		{ID: "c10", Author: dave, Body: "Reproduced in staging — sending 50 req/s to /api/users, after threshold we get 500 with generic error body. No Retry-After header at all. Logs: https://kibana.internal/app/discover#/plat-3-repro", Created: now.Add(-3 * day), Updated: now.Add(-3 * day)},
		{ID: "c16", Author: demo, Body: "Found the root cause. The middleware catches RateLimitExceeded but re-throws as InternalError. The error handler in error_handler.go doesn't have a case for rate limit errors, so it falls through to the default 500.", Created: now.Add(-2 * day), Updated: now.Add(-2 * day)},
		{ID: "c17", Author: bob, Body: "Good catch. Make sure the Retry-After header uses the actual reset time from the rate limiter, not a hardcoded value. Also add X-RateLimit-Remaining for client-side backoff. See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After for spec.", Created: now.Add(-36 * time.Hour), Updated: now.Add(-36 * time.Hour)},
		{ID: "c18", Author: demo, Body: "Done — PR is up https://github.com/acme/platform/pull/847 — added all three headers. Integration tests pass on staging.", Created: now.Add(-12 * time.Hour), Updated: now.Add(-12 * time.Hour)},
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
