package jira

import (
	"fmt"
	"strings"
	"time"
)

// JiraTime handles Jira's timestamp format which uses +0300 instead of +03:00
type JiraTime struct {
	time.Time
}

func (jt *JiraTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}

	// Try standard formats first.
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
		time.RFC3339Nano,
	}

	var err error
	for _, f := range formats {
		var t time.Time
		t, err = time.Parse(f, s)
		if err == nil {
			jt.Time = t
			return nil
		}
	}

	return fmt.Errorf("cannot parse Jira time %q: %w", s, err)
}

type Issue struct {
	ID             string           `json:"id"`
	Key            string           `json:"key"`
	Summary        string           `json:"-"`
	Description    string           `json:"-"`
	DescriptionADF any              `json:"-"` // raw ADF document from API
	Status         *Status          `json:"-"`
	Priority       *Priority        `json:"-"`
	Assignee       *User            `json:"-"`
	Reporter       *User            `json:"-"`
	Labels         []string         `json:"-"`
	Components     []Component      `json:"-"`
	Sprint         *Sprint          `json:"-"`
	IssueType      *IssueType       `json:"-"`
	Parent         *Issue           `json:"-"`
	Created        time.Time        `json:"-"`
	Updated        time.Time        `json:"-"`
	Subtasks       []Issue          `json:"-"`
	IssueLinks     []IssueLink      `json:"-"`
	Comments       []Comment        `json:"-"`
	Changelog      []ChangelogEntry `json:"-"`
	Transitions    []Transition     `json:"-"`
	TimeTracking   *TimeTracking    `json:"-"`
	CustomFields   map[string]any   `json:"-"`
}

// TimeTracking holds the Jira timetracking field. Values are Jira duration
// strings ("2w 3d 4h"), not durations: the unit lengths are per-instance
// config (a "day" may be 8h), so they are never parsed into time.Duration.
type TimeTracking struct {
	OriginalEstimate  string `json:"originalEstimate"`
	RemainingEstimate string `json:"remainingEstimate"`
	TimeSpent         string `json:"timeSpent"`
}

// IsZero reports whether no estimate or spent time is recorded.
func (t *TimeTracking) IsZero() bool {
	if t == nil {
		return true
	}
	return t.OriginalEstimate == "" && t.RemainingEstimate == "" && t.TimeSpent == ""
}

// ChangelogEntry represents a single change in issue history
type ChangelogEntry struct {
	Author  *User
	Created time.Time
	Items   []ChangeItem
}

// ChangeItem is one field change within a changelog entry
type ChangeItem struct {
	Field      string
	FromString string
	ToString   string
}

type Status struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CategoryKey string `json:"-"`
}

type Priority struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
}

type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	AvatarURL   string `json:"-"`
	Active      bool   `json:"active"`
}

type Sprint struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	State     string    `json:"state"`
	StartDate *JiraTime `json:"startDate"`
	EndDate   *JiraTime `json:"endDate"`
	Goal      string    `json:"goal"`
}

type Board struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	ProjectKey string `json:"-"`
}

type Project struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	AvatarURL string `json:"-"`
	Lead      *User  `json:"lead"`
}

type Comment struct {
	ID      string    `json:"id"`
	Author  *User     `json:"-"`
	Body    string    `json:"-"`
	BodyADF any       `json:"-"` // raw ADF document from API
	Created time.Time `json:"-"`
	Updated time.Time `json:"-"`
}

type Transition struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	To   *Status `json:"to"`
}

type IssueLink struct {
	ID           string         `json:"id"`
	Type         *IssueLinkType `json:"type"`
	InwardIssue  *Issue         `json:"-"`
	OutwardIssue *Issue         `json:"-"`
}

type IssueLinkType struct {
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

type IssueType struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	IconURL        string `json:"iconUrl"`
	Subtask        bool   `json:"subtask"`
	HierarchyLevel int    `json:"hierarchyLevel"`
}

// CanHaveParent reports whether this issue type may carry a parent link.
// Subtask=true and Standard (hierarchyLevel<1) → yes. Epic and higher
// (level>=1) → no, because the parent slot is only writable in projects
// with Premium hierarchy. DC instances without hierarchyLevel deliver 0
// (Standard) — the server validates on edit.
func (t *IssueType) CanHaveParent() bool {
	if t == nil {
		return false
	}
	if t.Subtask {
		return true
	}
	return t.HierarchyLevel < 1
}

type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SearchResult struct {
	Issues     []Issue `json:"issues"`
	Total      int     `json:"total"`
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
}

type CreateMetaField struct {
	FieldID       string
	Name          string
	Required      bool
	Schema        CreateMetaSchema
	AllowedValues []CreateMetaValue
}

type CreateMetaSchema struct {
	Type   string // "string", "array", "priority", "user", "option", etc
	System string // "priority", "assignee", etc
	Custom string // plugin key for custom fields
	Items  string // item type for arrays ("option", "user", "string", etc)
}

type CreateMetaValue struct {
	ID   string
	Name string
}
