package views

import (
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
)

func findField(fields []InfoField, id string) (InfoField, bool) {
	for _, f := range fields {
		if f.FieldID == id {
			return f, true
		}
	}
	return InfoField{}, false
}

func TestParentField_Present(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key: "PROJ-2",
		Parent: &jira.Issue{
			Key:     "PROJ-1",
			Summary: "epic summary",
		},
	}

	fields := buildInfoFields(issue, nil, extraInfoFields{})
	f, ok := findField(fields, "parent")
	if !ok {
		t.Fatalf("expected 'parent' field in default info fields, got: %+v", fields)
	}
	if f.Name != "Parent" {
		t.Errorf("Name = %q, want %q", f.Name, "Parent")
	}
	if f.Value != "[PROJ-1] epic summary" {
		t.Errorf("Value = %q, want %q", f.Value, "[PROJ-1] epic summary")
	}
	if f.Type != FieldSingleText {
		t.Errorf("Type = %v, want FieldSingleText", f.Type)
	}
}

func TestParentField_AbsentWhenNil(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{Key: "PROJ-2"}

	fields := buildInfoFields(issue, nil, extraInfoFields{})
	if _, ok := findField(fields, "parent"); ok {
		t.Errorf("expected no 'parent' field when Issue.Parent and IssueType are nil, got one")
	}
}

func TestParentField_NoneForSubtaskWithoutParent(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:       "PROJ-2",
		IssueType: &jira.IssueType{Name: "Sub-task", Subtask: true},
	}
	fields := buildInfoFields(issue, nil, extraInfoFields{})
	f, ok := findField(fields, "parent")
	if !ok {
		t.Fatalf("expected 'parent' field for subtask without parent, got: %+v", fields)
	}
	if f.Value != noneLabelUpper {
		t.Errorf("Value = %q, want %q", f.Value, noneLabelUpper)
	}
}

func TestParentField_NoneForStandardWithoutParent(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:       "PROJ-2",
		IssueType: &jira.IssueType{Name: "Story", HierarchyLevel: 0},
	}
	fields := buildInfoFields(issue, nil, extraInfoFields{})
	f, ok := findField(fields, "parent")
	if !ok {
		t.Fatalf("expected 'parent' field for standard issue without parent, got: %+v", fields)
	}
	if f.Value != noneLabelUpper {
		t.Errorf("Value = %q, want %q", f.Value, noneLabelUpper)
	}
}

func TestParentField_HiddenForEpicWithoutParent(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:       "PROJ-2",
		IssueType: &jira.IssueType{Name: "Epic", HierarchyLevel: 1},
	}
	fields := buildInfoFields(issue, nil, extraInfoFields{})
	if _, ok := findField(fields, "parent"); ok {
		t.Errorf("expected no 'parent' field for epic (level 1) without parent, got one")
	}
}

func TestParentField_SetReplacesIssue(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:    "PROJ-2",
		Parent: &jira.Issue{Key: "PROJ-1", Summary: "epic"},
	}
	if !SetBuiltinFieldValue(issue, "parent", &jira.Issue{Key: "OTHER-1"}) {
		t.Fatalf("SetBuiltinFieldValue('parent', ...) returned false; expected true")
	}
	if issue.Parent == nil || issue.Parent.Key != "OTHER-1" {
		t.Errorf("Parent.Key = %v, want OTHER-1", issue.Parent)
	}
}

func TestParentField_SetNilUnsets(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:    "PROJ-2",
		Parent: &jira.Issue{Key: "PROJ-1", Summary: "epic"},
	}
	if !SetBuiltinFieldValue(issue, "parent", nil) {
		t.Fatalf("SetBuiltinFieldValue('parent', nil) returned false; expected true")
	}
	if issue.Parent != nil {
		t.Errorf("Parent = %+v, want nil", issue.Parent)
	}
}

func TestEditValueForField_ParentReturnsKeyOnly(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:    "PROJ-2",
		Parent: &jira.Issue{Key: "PROJ-1", Summary: "epic summary"},
	}
	got := EditValueForField(issue, "parent", "PROJ-1 -- epic summary")
	if got != "PROJ-1" {
		t.Errorf("EditValueForField('parent', ...) = %q, want %q", got, "PROJ-1")
	}
}

func TestEditValueForField_ParentNilReturnsEmpty(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{Key: "PROJ-2"}
	got := EditValueForField(issue, "parent", "")
	if got != "" {
		t.Errorf("EditValueForField('parent', empty) = %q, want empty", got)
	}
}

func TestEditValueForField_FallbackToDisplay(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{Key: "PROJ-2"}
	if got := EditValueForField(issue, "summary", "current summary"); got != "current summary" {
		t.Errorf("fallback display: got %q, want %q", got, "current summary")
	}
	if got := EditValueForField(issue, "summary", noneLabelUpper); got != "" {
		t.Errorf("fallback None: got %q, want empty", got)
	}
}

func TestParentField_KeyOnlyDisplay(t *testing.T) {
	t.Parallel()
	issue := &jira.Issue{
		Key:    "PROJ-2",
		Parent: &jira.Issue{Key: "PROJ-1"},
	}
	fields := buildInfoFields(issue, nil, extraInfoFields{})
	f, ok := findField(fields, "parent")
	if !ok {
		t.Fatalf("expected 'parent' field, got: %+v", fields)
	}
	if f.Value != "PROJ-1" {
		t.Errorf("Value = %q, want %q", f.Value, "PROJ-1")
	}
}

func TestParentField_LongSummaryTruncated(t *testing.T) {
	t.Parallel()
	longSummary := strings.Repeat("x", 200)
	issue := &jira.Issue{
		Key:    "PROJ-2",
		Parent: &jira.Issue{Key: "PROJ-1", Summary: longSummary},
	}

	styled, plain := renderInfoRowPairs(issue, nil, extraInfoFields{}, nil, 30)
	_ = styled

	var parentRow string
	for _, row := range plain {
		if strings.Contains(row, "Parent:") {
			parentRow = row
			break
		}
	}
	if parentRow == "" {
		t.Fatalf("expected Parent row in rendered output, got: %v", plain)
	}
	if !strings.HasSuffix(parentRow, "…") {
		t.Errorf("expected truncated Parent row to end with '…', got %q", parentRow)
	}
	if strings.Contains(parentRow, longSummary) {
		t.Errorf("expected long summary to be truncated, but full summary appears in row: %q", parentRow)
	}
}
