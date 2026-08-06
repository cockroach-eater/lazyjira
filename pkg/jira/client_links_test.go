package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
)

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
