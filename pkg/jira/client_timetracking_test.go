package jira

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
)

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
