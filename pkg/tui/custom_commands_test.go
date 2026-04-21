package tui

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

func newTestApp() *App {
	return &App{
		cfg:         &config.Config{Jira: config.JiraConfig{Host: "example.atlassian.net"}},
		issuesList:  views.NewIssuesList(),
		projectList: views.NewProjectList(),
		detailView:  views.NewDetailView(),
		side:        sideLeft,
		leftFocus:   focusIssues,
	}
}

func TestActiveContexts(t *testing.T) {
	cases := []struct {
		name  string
		setup func(a *App)
		want  []config.Context
	}{
		{
			"left issues",
			func(a *App) { a.side = sideLeft; a.leftFocus = focusIssues },
			[]config.Context{config.CtxIssues},
		},
		{
			"left info",
			func(a *App) { a.side = sideLeft; a.leftFocus = focusInfo },
			[]config.Context{config.CtxInfo},
		},
		{
			"left projects",
			func(a *App) { a.side = sideLeft; a.leftFocus = focusProjects },
			[]config.Context{config.CtxProjects},
		},
		{
			"left status",
			func(a *App) { a.side = sideLeft; a.leftFocus = focusStatus },
			nil,
		},
		{
			"right detail details",
			func(a *App) {
				a.side = sideRight
				a.detailView.SetIssue(&jira.Issue{Key: "X-1"})
				a.detailView.SetActiveTab(views.TabDetails)
			},
			[]config.Context{config.CtxDetail},
		},
		{
			"right detail comments",
			func(a *App) {
				a.side = sideRight
				a.detailView.SetIssue(&jira.Issue{Key: "X-1"})
				a.detailView.SetActiveTab(views.TabComments)
			},
			[]config.Context{config.CtxDetailComments, config.CtxDetail},
		},
		{
			"right project mode",
			func(a *App) {
				a.side = sideRight
				a.detailView.SetProject(&jira.Project{Key: "P"})
			},
			[]config.Context{config.CtxProjects},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			tc.setup(a)
			got := a.activeContexts()
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d (%v), want %d (%v)", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func parseTmpl(t *testing.T, s string) *template.Template {
	t.Helper()
	tmpl, err := template.New("t").Option("missingkey=error").Parse(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tmpl
}

func TestBuildCommandData_SingleScopeFlat(t *testing.T) {
	a := newTestApp()
	a.issuesList.SetIssues([]jira.Issue{{Key: "ABC-1", Summary: "hi"}})
	rc := config.ResolvedCustomCommand{
		Key:      "y",
		Scopes:   config.ScopeIssue,
		Contexts: []config.Context{config.CtxIssues},
		Template: parseTmpl(t, "{{.Key}}|{{.Summary}}"),
	}
	data, ok := a.buildCommandData(rc)
	if !ok {
		t.Fatal("expected ok")
	}
	var buf bytes.Buffer
	if err := rc.Template.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := buf.String(); got != "ABC-1|hi" {
		t.Errorf("got %q", got)
	}
}

func TestBuildCommandData_DetailCommentsFlat(t *testing.T) {
	a := newTestApp()
	issue := jira.Issue{Key: "ABC-1", Comments: []jira.Comment{{ID: "10", Body: "hello"}}}
	a.issuesList.SetIssues([]jira.Issue{issue})
	a.detailView.SetIssue(&issue)
	a.detailView.SetActiveTab(views.TabComments)

	rc := config.ResolvedCustomCommand{
		Key:      "c",
		Scopes:   config.ScopeIssue | config.ScopeComment,
		Contexts: []config.Context{config.CtxDetailComments},
		Template: parseTmpl(t, "{{.Key}}-{{.CommentID}}"),
	}
	data, ok := a.buildCommandData(rc)
	if !ok {
		t.Fatal("expected ok")
	}
	var buf bytes.Buffer
	if err := rc.Template.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := buf.String(); got != "ABC-1-10" {
		t.Errorf("got %q", got)
	}
}

func TestBuildCommandData_SharedFieldsProjectScope(t *testing.T) {
	a := newTestApp()
	a.gitBranch = "feature/x"
	a.gitRepoPath = "/tmp/repo"
	a.projectList.SetProjects([]jira.Project{{Key: "P", Name: "Proj"}})
	a.side = sideLeft
	a.leftFocus = focusProjects

	rc := config.ResolvedCustomCommand{
		Key:      "p",
		Scopes:   config.ScopeProject,
		Contexts: []config.Context{config.CtxProjects},
		Template: parseTmpl(t, "{{.ProjectKey}}|{{.JiraHost}}|{{.GitBranch}}|{{.GitRepoPath}}"),
	}
	data, ok := a.buildCommandData(rc)
	if !ok {
		t.Fatal("expected ok")
	}
	var buf bytes.Buffer
	if err := rc.Template.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, want := buf.String(), "P|example.atlassian.net|feature/x|/tmp/repo"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildCommandData_SharedFieldsDetailComments(t *testing.T) {
	a := newTestApp()
	a.gitBranch = "feature/y"
	a.gitRepoPath = "/tmp/repo2"
	issue := jira.Issue{Key: "ABC-1", Comments: []jira.Comment{{ID: "10", Body: "hello"}}}
	a.issuesList.SetIssues([]jira.Issue{issue})
	a.detailView.SetIssue(&issue)
	a.detailView.SetActiveTab(views.TabComments)

	rc := config.ResolvedCustomCommand{
		Key:      "c",
		Scopes:   config.ScopeIssue | config.ScopeComment,
		Contexts: []config.Context{config.CtxDetailComments},
		Template: parseTmpl(t, "{{.Key}}|{{.CommentID}}|{{.JiraHost}}|{{.GitBranch}}|{{.GitRepoPath}}"),
	}
	data, ok := a.buildCommandData(rc)
	if !ok {
		t.Fatal("expected ok")
	}
	var buf bytes.Buffer
	if err := rc.Template.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, want := buf.String(), "ABC-1|10|example.atlassian.net|feature/y|/tmp/repo2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildCommandData_MissingSelectionSwallows(t *testing.T) {
	a := newTestApp()
	a.side = sideLeft
	a.leftFocus = focusProjects
	rc := config.ResolvedCustomCommand{
		Key:      "n",
		Scopes:   config.ScopeProject,
		Contexts: []config.Context{config.CtxProjects},
		Template: parseTmpl(t, "{{.ProjectKey}}"),
	}
	if _, ok := a.buildCommandData(rc); ok {
		t.Error("expected ok=false with no selected project")
	}
}

func TestHandleCustomCommand_SpecificityDispatch(t *testing.T) {
	a := newTestApp()
	issue := jira.Issue{Key: "ABC-1", Comments: []jira.Comment{{ID: "9", Body: "b"}}}
	a.issuesList.SetIssues([]jira.Issue{issue})
	a.detailView.SetIssue(&issue)
	a.detailView.SetActiveTab(views.TabComments)
	a.side = sideRight

	detailCmd := config.ResolvedCustomCommand{
		Key: "x", Name: "detail-one", Scopes: config.ScopeIssue,
		Contexts: []config.Context{config.CtxDetail},
		Template: parseTmpl(t, "echo detail"),
	}
	commentsCmd := config.ResolvedCustomCommand{
		Key: "x", Name: "comments-one", Scopes: config.ScopeIssue | config.ScopeComment,
		Contexts: []config.Context{config.CtxDetailComments},
		Template: parseTmpl(t, "echo comments"),
	}
	a.customCmds = []config.ResolvedCustomCommand{detailCmd, commentsCmd}

	// The specificity-ordered contexts should dispatch to detail.comments first.
	ctxs := a.activeContexts()
	if len(ctxs) == 0 || ctxs[0] != config.CtxDetailComments {
		t.Fatalf("activeContexts = %v, want detail.comments first", ctxs)
	}
	// Walk the dispatch logic manually to assert selection without exec.
	var chosen string
	for _, ctx := range ctxs {
		for _, rc := range a.customCmds {
			if rc.Key == "x" && rc.HasContext(ctx) {
				chosen = rc.Name
				goto done
			}
		}
	}
done:
	if chosen != "comments-one" {
		t.Errorf("chose %q, want comments-one", chosen)
	}
}

func TestQuitReachableWarning(t *testing.T) {
	allCtxs := []config.Context{
		config.CtxIssues,
		config.CtxInfo,
		config.CtxProjects,
		config.CtxDetail,
		config.CtxDetailComments,
	}
	defaultKm := Keymap{ActQuit: {"q", "ctrl+c"}}

	cases := []struct {
		name     string
		km       Keymap
		cmds     []config.ResolvedCustomCommand
		wantWarn bool
	}{
		{
			name:     "no custom commands",
			km:       defaultKm,
			cmds:     nil,
			wantWarn: false,
		},
		{
			name: "custom command on unrelated key",
			km:   defaultKm,
			cmds: []config.ResolvedCustomCommand{
				{Key: "y", Contexts: allCtxs},
			},
			wantWarn: false,
		},
		{
			name: "shadows q in issues only",
			km:   defaultKm,
			cmds: []config.ResolvedCustomCommand{
				{Key: "q", Contexts: []config.Context{config.CtxIssues}},
			},
			wantWarn: false,
		},
		{
			name: "shadows q and ctrl+c everywhere",
			km:   defaultKm,
			cmds: []config.ResolvedCustomCommand{
				{Key: "q", Contexts: allCtxs},
				{Key: "ctrl+c", Contexts: allCtxs},
			},
			wantWarn: true,
		},
		{
			name: "user remapped quit to alt+q, q shadowed everywhere",
			km:   Keymap{ActQuit: {"alt+q"}},
			cmds: []config.ResolvedCustomCommand{
				{Key: "q", Contexts: allCtxs},
			},
			wantWarn: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warning := quitReachableWarning(tc.km, tc.cmds)
			got := warning != ""
			if got != tc.wantWarn {
				t.Errorf("got warning=%q, want warn=%v", warning, tc.wantWarn)
			}
		})
	}
}
