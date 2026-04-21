package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// customCommandFinishedMsg is sent when a custom command exits.
// output is populated for background commands; suspended commands write
// directly to the terminal.
type customCommandFinishedMsg struct {
	err     error
	output  string
	refresh bool
}

// issueScopeData holds template variables for issue-scoped commands.
type issueScopeData struct {
	Key         string
	ProjectKey  string
	ParentKey   string
	Summary     string
	Type        string
	Status      string
	Assignee    string
	Priority    string
	URL         string
	GitBranch   string
	GitRepoPath string
	JiraHost    string
}

// projectScopeData holds template variables for project-scoped commands.
type projectScopeData struct {
	ProjectKey  string
	ProjectName string
	JiraHost    string
	GitBranch   string
	GitRepoPath string
}

// commentScopeData holds template variables for comment-scoped commands.
type commentScopeData struct {
	CommentID     string
	CommentAuthor string
	CommentBody   string
}

// detailCommentsScopeData is used for commands active in the detail.comments
// context, exposing both Issue and Comment fields at the top level. Issue and
// Comment field names do not collide, so struct embedding is unambiguous.
type detailCommentsScopeData struct {
	issueScopeData
	commentScopeData
}

// initCustomCommands resolves the custom commands from config at startup.
// Resolution errors are surfaced on the status panel; the slice is left empty.
func (a *App) initCustomCommands() {
	resolved, err := a.cfg.ResolveCustomCommands()
	if err != nil {
		a.statusPanel.SetError(fmt.Sprintf("custom commands: %v", err))
		a.customCmds = nil
		return
	}
	a.customCmds = resolved
}

// activeContexts returns the contexts that currently apply, ordered by
// descending specificity. First match wins in handleCustomCommand.
func (a *App) activeContexts() []config.Context {
	var out []config.Context
	switch a.side {
	case sideRight:
		switch a.detailView.Mode() {
		case views.ModeIssue:
			if a.detailView.ActiveTab() == views.TabComments {
				out = append(out, config.CtxDetailComments)
			}
			out = append(out, config.CtxDetail)
		case views.ModeProject:
			out = append(out, config.CtxProjects)
		case views.ModeSplash:
			// no custom command contexts on splash
		}
	case sideLeft:
		switch a.leftFocus {
		case focusIssues:
			out = append(out, config.CtxIssues)
		case focusInfo:
			out = append(out, config.CtxInfo)
		case focusProjects:
			out = append(out, config.CtxProjects)
		case focusStatus:
			// none
		}
	}
	return out
}

// handleCustomCommand checks if keyStr matches a custom command bound to an
// active context and executes it. Returns (model, cmd, true) if handled.
func (a *App) handleCustomCommand(keyStr string) (tea.Model, tea.Cmd, bool) {
	for _, ctx := range a.activeContexts() {
		for _, rc := range a.customCmds {
			if rc.Key != keyStr {
				continue
			}
			if !rc.HasContext(ctx) {
				continue
			}
			data, ok := a.buildCommandData(rc)
			if !ok {
				a.helpBar.SetStatusMsg(fmt.Sprintf("%s: no %s selected", rc.Name, scopeNoun(rc.Scopes)))
				return a, nil, true
			}
			return a, a.executeCustomCommand(rc, data), true
		}
	}
	return nil, nil, false
}

// scopeNoun returns a human-readable noun for the selection a scope requires.
func scopeNoun(s config.ScopeMask) string {
	switch s {
	case config.ScopeIssue:
		return "issue"
	case config.ScopeProject:
		return "project"
	case config.ScopeIssue | config.ScopeComment:
		return "comment"
	default:
		return "selection"
	}
}

// buildCommandData returns the template data for a resolved command.
// The second return is false when a required selection is missing.
func (a *App) buildCommandData(rc config.ResolvedCustomCommand) (any, bool) {
	switch rc.Scopes {
	case config.ScopeIssue:
		if a.currentIssue() == nil {
			return nil, false
		}
		return a.buildIssueScopeData(), true
	case config.ScopeProject:
		p := a.projectList.SelectedProject()
		if p == nil {
			return nil, false
		}
		return a.buildProjectScopeData(p), true
	case config.ScopeComment:
		cmt := a.detailView.SelectedComment()
		if cmt == nil {
			return nil, false
		}
		return a.buildCommentScopeData(cmt), true
	case config.ScopeIssue | config.ScopeComment:
		if a.currentIssue() == nil {
			return nil, false
		}
		cmt := a.detailView.SelectedComment()
		if cmt == nil {
			return nil, false
		}
		return detailCommentsScopeData{
			issueScopeData:   a.buildIssueScopeData(),
			commentScopeData: a.buildCommentScopeData(cmt),
		}, true
	}
	return nil, false
}

func (a *App) buildIssueScopeData() issueScopeData {
	sel := a.currentIssue()
	data := issueScopeData{
		GitBranch:   a.gitBranch,
		GitRepoPath: a.gitRepoPath,
		JiraHost:    a.cfg.Jira.Host,
	}
	if sel == nil {
		return data
	}

	data.Key = sel.Key
	data.Summary = sel.Summary

	if parts := strings.SplitN(sel.Key, "-", 2); len(parts) == 2 {
		data.ProjectKey = parts[0]
	}

	if sel.Parent != nil {
		data.ParentKey = sel.Parent.Key
	}
	if sel.IssueType != nil {
		data.Type = sel.IssueType.Name
	}
	if sel.Status != nil {
		data.Status = sel.Status.Name
	}
	if sel.Assignee != nil {
		data.Assignee = sel.Assignee.DisplayName
	}
	if sel.Priority != nil {
		data.Priority = sel.Priority.Name
	}

	if a.cfg.Jira.Host != "" {
		data.URL = fmt.Sprintf("https://%s/browse/%s", a.cfg.Jira.Host, sel.Key)
	}

	return data
}

func (a *App) buildProjectScopeData(p *jira.Project) projectScopeData {
	return projectScopeData{
		ProjectKey:  p.Key,
		ProjectName: p.Name,
		JiraHost:    a.cfg.Jira.Host,
		GitBranch:   a.gitBranch,
		GitRepoPath: a.gitRepoPath,
	}
}

func (a *App) buildCommentScopeData(c *jira.Comment) commentScopeData {
	data := commentScopeData{
		CommentID:   c.ID,
		CommentBody: c.Body,
	}
	if c.Author != nil {
		data.CommentAuthor = c.Author.DisplayName
	}
	return data
}

func (a *App) executeCustomCommand(rc config.ResolvedCustomCommand, data any) tea.Cmd {
	var buf bytes.Buffer
	if err := rc.Template.Execute(&buf, data); err != nil {
		return func() tea.Msg {
			return customCommandFinishedMsg{err: fmt.Errorf("template error: %w", err)}
		}
	}
	cmdStr := buf.String()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	refresh := rc.Refresh

	if rc.ShouldSuspend() {
		c := exec.CommandContext(a.ctx, shell, "-c", cmdStr) //nolint:gosec // user-configured custom commands are intentionally arbitrary shell commands
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return tea.ExecProcess(c, func(err error) tea.Msg {
			return customCommandFinishedMsg{err: err, refresh: refresh}
		})
	}

	// Background execution: capture output and report via the tea message loop.
	// The WaitGroup ensures Shutdown blocks until this goroutine finishes, so
	// the child process is not orphaned on app exit.
	a.cmdWg.Add(1)
	return func() tea.Msg {
		defer a.cmdWg.Done()
		c := exec.CommandContext(a.ctx, shell, "-c", cmdStr) //nolint:gosec // user-configured custom commands are intentionally arbitrary shell commands
		c.Cancel = func() error {
			return c.Process.Signal(syscall.SIGTERM)
		}
		c.WaitDelay = 3 * time.Second
		out, err := c.CombinedOutput()
		return customCommandFinishedMsg{err: err, output: string(out), refresh: refresh}
	}
}

func (a *App) handleCustomCommandFinished(msg customCommandFinishedMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{tea.EnableMouseCellMotion}
	if msg.err != nil {
		errText := msg.err.Error()
		if tail := lastNonEmptyLine(msg.output); tail != "" {
			errText = tail + ": " + errText
		}
		a.statusPanel.SetError(errText)
		return a, tea.Batch(cmds...)
	}
	if tail := lastNonEmptyLine(msg.output); tail != "" {
		a.helpBar.SetStatusMsg(tail)
	}
	// Refresh the previewed issue only when the command declares refresh: true.
	if msg.refresh {
		if cur := a.currentIssue(); cur != nil {
			delete(a.issueCache, cur.Key)
			cmds = append(cmds, fetchIssueDetail(a.client, cur.Key))
		}
	}
	return a, tea.Batch(cmds...)
}

// lastNonEmptyLine returns the last non-empty line of s, trimmed. Used to
// surface a single concise message from arbitrary command output.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// allUIContexts is the universe of UI contexts for shadowing analysis.
var allUIContexts = []config.Context{
	config.CtxIssues,
	config.CtxInfo,
	config.CtxProjects,
	config.CtxDetail,
	config.CtxDetailComments,
}

// quitReachableWarning returns a warning string if every keybinding for
// ActQuit is shadowed by custom commands across every UI context, leaving
// the user no way to quit. Empty string means quit is still reachable.
func quitReachableWarning(km Keymap, cmds []config.ResolvedCustomCommand) string {
	keys := km[ActQuit]
	if len(keys) == 0 {
		return ""
	}
	for _, key := range keys {
		for _, ctx := range allUIContexts {
			if !keyShadowsContext(key, ctx, cmds) {
				return ""
			}
		}
	}
	return fmt.Sprintf(
		"essential action %q is unreachable: keys %v are shadowed by custom commands in every UI context",
		ActQuit, keys,
	)
}

func keyShadowsContext(key string, ctx config.Context, cmds []config.ResolvedCustomCommand) bool {
	for _, rc := range cmds {
		if rc.Key == key && rc.HasContext(ctx) {
			return true
		}
	}
	return false
}

// customCommandHelpItems returns HelpItem entries for the help bar in the given context.
func (a *App) customCommandHelpItems(ctx config.Context) []components.HelpItem {
	items := make([]components.HelpItem, 0)
	for _, rc := range a.customCmds {
		if rc.HasContext(ctx) {
			items = append(items, components.HelpItem{Key: rc.Key, Description: rc.Name})
		}
	}
	return items
}

// customCommandBindings returns Binding entries for the help overlay in the given context.
func (a *App) customCommandBindings(ctx config.Context) []Binding {
	bindings := make([]Binding, 0)
	for _, rc := range a.customCmds {
		if rc.HasContext(ctx) {
			bindings = append(bindings, Binding{
				Key:         rc.Key,
				Description: rc.Name,
			})
		}
	}
	return bindings
}
