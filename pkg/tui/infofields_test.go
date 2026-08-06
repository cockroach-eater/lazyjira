package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/jira/jiratest"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

// newEstimateApp is an app whose Info panel shows one cached issue.
func newEstimateApp(t *testing.T) (*App, *jira.Issue) {
	t.Helper()
	app := newTestApp()
	app.client = &jiratest.FakeClient{T: t}
	app.infoPanel = views.NewInfoPanel()
	app.statusPanel = views.NewStatusPanel(testProject, "", "")
	app.issueCache = map[string]*jira.Issue{}
	logFlag := false
	app.logFlag = &logFlag

	issue := &jira.Issue{Key: testKey, Summary: testSummary}
	app.issuesList.SetIssues([]jira.Issue{*issue})
	app.issueCache[testKey] = issue
	app.previewKey = testKey
	return app, issue
}

func TestApplyEstimateEditRejectsGarbage(t *testing.T) {
	t.Parallel()
	app, _ := newEstimateApp(t)

	cmd := app.applyEstimateEdit(testKey, "soon")
	if cmd == nil {
		t.Fatal("no result for an invalid estimate")
	}
	msg, ok := cmd().(errorMsg)
	if !ok {
		t.Fatalf("message = %T, want errorMsg", cmd())
	}
	if !strings.Contains(msg.err.Error(), "invalid estimate") {
		t.Errorf("error = %v", msg.err)
	}
}

func TestApplyEstimateEditAcceptsJiraDurations(t *testing.T) {
	t.Parallel()
	app, _ := newEstimateApp(t)
	fake := &jiratest.FakeClient{T: t}
	fake.UpdateIssueFunc = func(_ context.Context, _ string, _ map[string]any) error { return nil }
	app.client = fake

	for _, valid := range []string{"2d", "1w 3d", "90m", "2w 3d 4h"} {
		fake.UpdateIssueCalls = nil
		cmd := app.applyEstimateEdit(testKey, valid)
		if cmd == nil {
			t.Fatalf("%q produced no command", valid)
		}
		if msg, isErr := cmd().(errorMsg); isErr {
			t.Fatalf("%q was rejected: %v", valid, msg.err)
		}
		if len(fake.UpdateIssueCalls) != 1 {
			t.Fatalf("%q: UpdateIssue calls = %d", valid, len(fake.UpdateIssueCalls))
		}
		got := fake.UpdateIssueCalls[0].Fields["timetracking"]
		want := map[string]string{"originalEstimate": valid}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q sent %v, want %v", valid, got, want)
		}
	}
}

func TestApplyEstimateEditClearsOnEmptyInput(t *testing.T) {
	t.Parallel()
	app, issue := newEstimateApp(t)
	issue.TimeTracking = &jira.TimeTracking{OriginalEstimate: "2d"}
	fake := &jiratest.FakeClient{T: t}
	fake.UpdateIssueFunc = func(_ context.Context, _ string, _ map[string]any) error { return nil }
	app.client = fake

	cmd := app.applyEstimateEdit(testKey, "")
	if cmd == nil {
		t.Fatal("clearing the estimate produced no command")
	}
	cmd()

	if issue.TimeTracking != nil {
		t.Errorf("the cached issue still carries %+v", issue.TimeTracking)
	}
	sent := fake.UpdateIssueCalls[0].Fields["timetracking"]
	if fmt.Sprint(sent) != fmt.Sprint(map[string]string{"originalEstimate": ""}) {
		t.Errorf("sent %v, want an empty estimate", sent)
	}
}
