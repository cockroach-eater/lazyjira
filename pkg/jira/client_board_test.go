package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
)

const projectStatusesBody = `[
  {"name":"Task","statuses":[
    {"id":"3","name":"In Progress","statusCategory":{"key":"indeterminate"}},
    {"id":"1","name":"To Do","statusCategory":{"key":"new"}}
  ]},
  {"name":"Bug","statuses":[
    {"id":"5","name":"Done","statusCategory":{"key":"done"}},
    {"id":"1","name":"To Do","statusCategory":{"key":"new"}}
  ]}
]`

func TestGetProjectStatusesDeduplicatesAndOrdersByCategory(t *testing.T) {
	t.Parallel()
	client, recorded := newRecordingClient(t, cloudOpts(), testkit.StubResponse{
		Status: http.StatusOK, Body: projectStatusesBody,
	})

	statuses, err := client.GetProjectStatuses(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("GetProjectStatuses: %v", err)
	}

	if recorded.Path != "/rest/api/3/project/PLAT/statuses" {
		t.Errorf("path = %q", recorded.Path)
	}

	names := make([]string, 0, len(statuses))
	for _, s := range statuses {
		names = append(names, s.Name)
	}
	want := []string{"To Do", "In Progress", "Done"}
	if len(names) != len(want) {
		t.Fatalf("statuses = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("statuses = %v, want %v", names, want)
			break
		}
	}
	if statuses[0].CategoryKey != "new" {
		t.Errorf("CategoryKey = %q, want new", statuses[0].CategoryKey)
	}
}

func TestGetProjectStatusesWrapsAPIError(t *testing.T) {
	t.Parallel()
	client, _ := newRecordingClient(t, cloudOpts(), testkit.StubResponse{
		Status: http.StatusNotFound, Body: `{"errorMessages":["No project could be found"]}`,
	})

	if _, err := client.GetProjectStatuses(context.Background(), errorTestProjectKey); err == nil {
		t.Fatal("expected error for missing project")
	} else if !strings.Contains(err.Error(), errorTestProjectKey) {
		t.Errorf("error %q does not name the project", err)
	}
}

func TestGetIssueLinkTypes(t *testing.T) {
	t.Parallel()
	client, recorded := newRecordingClient(t, cloudOpts(), testkit.StubResponse{
		Status: http.StatusOK,
		Body:   `{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"}]}`,
	})

	types, err := client.GetIssueLinkTypes(context.Background())
	if err != nil {
		t.Fatalf("GetIssueLinkTypes: %v", err)
	}
	if recorded.Path != "/rest/api/3/issueLinkType" {
		t.Errorf("path = %q", recorded.Path)
	}
	if len(types) != 1 || types[0].Name != "Blocks" || types[0].Outward != "blocks" {
		t.Errorf("types = %+v", types)
	}
}

func TestCreateIssueLinkSendsDirectionalPayload(t *testing.T) {
	t.Parallel()
	client, recorded := newRecordingClient(t, cloudOpts(), testkit.StubResponse{Status: http.StatusCreated})

	if err := client.CreateIssueLink(context.Background(), "Blocks", "PLAT-2", "PLAT-1"); err != nil {
		t.Fatalf("CreateIssueLink: %v", err)
	}

	if recorded.Method != http.MethodPost || recorded.Path != "/rest/api/3/issueLink" {
		t.Errorf("%s %s", recorded.Method, recorded.Path)
	}

	var body struct {
		Type         struct{ Name string } `json:"type"`
		InwardIssue  struct{ Key string }  `json:"inwardIssue"`
		OutwardIssue struct{ Key string }  `json:"outwardIssue"`
	}
	if err := json.Unmarshal(recorded.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Type.Name != "Blocks" {
		t.Errorf("type = %q", body.Type.Name)
	}
	if body.InwardIssue.Key != "PLAT-2" || body.OutwardIssue.Key != "PLAT-1" {
		t.Errorf("inward = %q, outward = %q", body.InwardIssue.Key, body.OutwardIssue.Key)
	}
}

func TestDeleteIssueLink(t *testing.T) {
	t.Parallel()
	client, recorded := newRecordingClient(t, cloudOpts(), testkit.StubResponse{Status: http.StatusNoContent})

	if err := client.DeleteIssueLink(context.Background(), "10042"); err != nil {
		t.Fatalf("DeleteIssueLink: %v", err)
	}
	if recorded.Method != http.MethodDelete || recorded.Path != "/rest/api/3/issueLink/10042" {
		t.Errorf("%s %s", recorded.Method, recorded.Path)
	}
}

func TestDeleteIssuePassesDeleteSubtasks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		subtasks bool
		want     string
	}{
		{"without subtasks", false, "false"},
		{"with subtasks", true, "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, recorded := newRecordingClient(t, cloudOpts(), testkit.StubResponse{Status: http.StatusNoContent})

			if err := client.DeleteIssue(context.Background(), errorTestIssueKey, tc.subtasks); err != nil {
				t.Fatalf("DeleteIssue: %v", err)
			}
			if recorded.Method != http.MethodDelete {
				t.Errorf("method = %s", recorded.Method)
			}
			if recorded.Path != "/rest/api/3/issue/"+errorTestIssueKey {
				t.Errorf("path = %q", recorded.Path)
			}
			if got := recorded.Query.Get("deleteSubtasks"); got != tc.want {
				t.Errorf("deleteSubtasks = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetIssueParsesTimeTracking(t *testing.T) {
	t.Parallel()
	client, _ := newRecordingClient(t, cloudOpts(), testkit.StubResponse{
		Status: http.StatusOK,
		Body: `{"id":"1","key":"PLAT-1","fields":{"summary":"s","timetracking":
		  {"originalEstimate":"2d","remainingEstimate":"4h","timeSpent":"1d 4h"}}}`,
	})

	issue, err := client.GetIssue(context.Background(), "PLAT-1")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.TimeTracking == nil {
		t.Fatal("TimeTracking is nil")
	}
	if issue.TimeTracking.OriginalEstimate != "2d" || issue.TimeTracking.TimeSpent != "1d 4h" {
		t.Errorf("TimeTracking = %+v", issue.TimeTracking)
	}
}

func TestGetIssueLeavesTimeTrackingNilWhenEmpty(t *testing.T) {
	t.Parallel()
	client, _ := newRecordingClient(t, cloudOpts(), testkit.StubResponse{
		Status: http.StatusOK,
		Body:   `{"id":"1","key":"PLAT-1","fields":{"summary":"s","timetracking":{}}}`,
	})

	issue, err := client.GetIssue(context.Background(), "PLAT-1")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.TimeTracking != nil {
		t.Errorf("TimeTracking = %+v, want nil for an empty object", issue.TimeTracking)
	}
}

func TestSearchIssuesRequestsTimeTrackingField(t *testing.T) {
	t.Parallel()
	client, recorded := newRecordingClient(t, cloudOpts(), testkit.StubResponse{
		Status: http.StatusOK, Body: `{"issues":[],"total":0,"maxResults":50,"startAt":0}`,
	})

	if _, err := client.SearchIssues(context.Background(), "project = PLAT", 0, 50); err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	if !strings.Contains(recorded.Query.Get("fields"), "timetracking") {
		t.Errorf("fields = %q, want it to include timetracking", recorded.Query.Get("fields"))
	}
}
