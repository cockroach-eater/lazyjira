package views

import (
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
)

func fieldByID(fields []InfoField, id string) *InfoField {
	for i := range fields {
		if fields[i].FieldID == id {
			return &fields[i]
		}
	}
	return nil
}

func TestEstimateFieldShowsOriginalAndSpent(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:          "P-1",
		TimeTracking: &jira.TimeTracking{OriginalEstimate: "2d", TimeSpent: "4h"},
	}

	got := fieldByID(buildInfoFields(issue, nil, extraInfoFields{}), "timetracking")

	if got == nil {
		t.Fatal("no Estimate row was built")
	}
	if got.Name != "Estimate" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Value != "2d (spent 4h)" {
		t.Errorf("Value = %q", got.Value)
	}
}

func TestEstimateFieldFallsBackToNone(t *testing.T) {
	t.Parallel()
	got := fieldByID(buildInfoFields(&jira.Issue{Key: "P-1"}, nil, extraInfoFields{}), "timetracking")

	if got == nil {
		t.Fatal("the Estimate row should still be listed when unset")
	}
	if got.Value != noneLabelUpper {
		t.Errorf("Value = %q, want %s", got.Value, noneLabelUpper)
	}
}

func TestEstimateFieldOmitsSpentWhenAbsent(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{TimeTracking: &jira.TimeTracking{OriginalEstimate: "3h"}}

	got := fieldByID(buildInfoFields(issue, nil, extraInfoFields{}), "timetracking")

	if got.Value != "3h" {
		t.Errorf("Value = %q, want no spent suffix", got.Value)
	}
}

func TestEstimateEditValueIsTheRawEstimate(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		TimeTracking: &jira.TimeTracking{OriginalEstimate: "2d", TimeSpent: "4h"},
	}

	// The display carries the spent time; the input must not.
	if got := EditValueForField(issue, "timetracking", "2d (spent 4h)"); got != "2d" {
		t.Errorf("EditValueForField = %q, want the plain estimate", got)
	}
}

func TestSetBuiltinFieldValueAppliesEstimate(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{}

	if !SetBuiltinFieldValue(issue, "timetracking", "1w") {
		t.Fatal("SetBuiltinFieldValue did not handle timetracking")
	}
	if issue.TimeTracking == nil || issue.TimeTracking.OriginalEstimate != "1w" {
		t.Errorf("TimeTracking = %+v", issue.TimeTracking)
	}

	SetBuiltinFieldValue(issue, "timetracking", "")
	if issue.TimeTracking != nil {
		t.Errorf("TimeTracking = %+v, want cleared", issue.TimeTracking)
	}
}

func TestReviewerFieldSitsAfterAssignee(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:          "P-1",
		Assignee:     &jira.User{DisplayName: "Alice"},
		CustomFields: map[string]any{"customfield_101": map[string]any{"displayName": "Bob"}},
	}

	fields := buildInfoFields(issue, nil, extraInfoFields{reviewerID: "customfield_101"})

	var assigneeIdx, reviewerIdx = -1, -1
	for i, f := range fields {
		switch f.FieldID {
		case fieldAssignee:
			assigneeIdx = i
		case "customfield_101":
			reviewerIdx = i
		}
	}
	if reviewerIdx < 0 {
		t.Fatal("no Reviewer row was built")
	}
	if reviewerIdx != assigneeIdx+1 {
		t.Errorf("Reviewer at %d, Assignee at %d: it should follow the assignee", reviewerIdx, assigneeIdx)
	}
	if fields[reviewerIdx].Name != "Reviewer" || fields[reviewerIdx].Type != FieldPerson {
		t.Errorf("Reviewer row = %+v, want a person field", fields[reviewerIdx])
	}
	if !strings.Contains(fields[reviewerIdx].Value, "Bob") {
		t.Errorf("Reviewer value = %q", fields[reviewerIdx].Value)
	}
}

func TestReviewerFieldFallsBackToNone(t *testing.T) {
	t.Parallel()
	fields := buildInfoFields(&jira.Issue{Key: "P-1"}, nil, extraInfoFields{reviewerID: "customfield_101"})

	got := fieldByID(fields, "customfield_101")
	if got == nil {
		t.Fatal("no Reviewer row was built")
	}
	if got.Value != noneLabelUpper {
		t.Errorf("Value = %q, want %s for an unset reviewer", got.Value, noneLabelUpper)
	}
}

func TestReviewerFieldNotDuplicatedWhenAlreadyConfigured(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{Key: "P-1", CustomFields: map[string]any{"customfield_101": "Bob"}}
	cfgFields := []config.FieldConfig{
		{ID: "status"},
		{ID: "customfield_101", Name: "Code reviewer"},
	}

	fields := buildInfoFields(issue, cfgFields, extraInfoFields{reviewerID: "customfield_101"})

	count := 0
	for _, f := range fields {
		if f.FieldID == "customfield_101" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the reviewer field appears %d times, want the user's own entry kept", count)
	}
	if got := fieldByID(fields, "customfield_101"); got.Name != "Code reviewer" {
		t.Errorf("Name = %q, want the configured label to win", got.Name)
	}
}

func TestNoReviewerRowWhenUnconfigured(t *testing.T) {
	t.Parallel()
	fields := buildInfoFields(&jira.Issue{Key: "P-1"}, nil, extraInfoFields{})

	for _, f := range fields {
		if f.Name == "Reviewer" {
			t.Errorf("a Reviewer row appeared with no reviewerField configured: %+v", f)
		}
	}
}

func TestBranchRowOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{Key: "P-1"}

	if got := fieldByID(buildInfoFields(issue, nil, extraInfoFields{}), BranchFieldID); got != nil {
		t.Errorf("a Branch row appeared outside a git repository: %+v", got)
	}

	fields := buildInfoFields(issue, nil, extraInfoFields{showBranch: true, branch: "feat/p-1-thing"})
	got := fieldByID(fields, BranchFieldID)
	if got == nil {
		t.Fatal("no Branch row was built")
	}
	if got.Value != "feat/p-1-thing" {
		t.Errorf("Value = %q", got.Value)
	}
}

func TestBranchRowFallsBackToNone(t *testing.T) {
	t.Parallel()
	fields := buildInfoFields(&jira.Issue{Key: "P-1"}, nil, extraInfoFields{showBranch: true})

	if got := fieldByID(fields, BranchFieldID); got.Value != noneLabelUpper {
		t.Errorf("Value = %q, want %s when no branch matches", got.Value, noneLabelUpper)
	}
}

func TestInfoPanelSetBranchIgnoresStaleAnswers(t *testing.T) {
	t.Parallel()
	p := NewInfoPanel()
	p.EnableBranchField(true)
	p.SetIssue(&jira.Issue{Key: "P-2"})

	p.SetBranch("P-1", "feat/p-1") // answer for the issue we already left

	if got := fieldByID(p.Fields(), BranchFieldID); got.Value != noneLabelUpper {
		t.Errorf("Value = %q, want the stale branch dropped", got.Value)
	}

	p.SetBranch("P-2", "feat/p-2")
	if got := fieldByID(p.Fields(), BranchFieldID); got.Value != "feat/p-2" {
		t.Errorf("Value = %q", got.Value)
	}
}

func TestInfoPanelBranchClearedOnIssueChange(t *testing.T) {
	t.Parallel()
	p := NewInfoPanel()
	p.EnableBranchField(true)
	p.SetIssue(&jira.Issue{Key: "P-1"})
	p.SetBranch("P-1", "feat/p-1")

	p.SetIssue(&jira.Issue{Key: "P-2"})

	if got := fieldByID(p.Fields(), BranchFieldID); got.Value != noneLabelUpper {
		t.Errorf("Value = %q, want the previous issue's branch gone", got.Value)
	}
}
