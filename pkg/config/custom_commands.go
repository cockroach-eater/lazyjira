package config

import (
	"fmt"
	"slices"
	"strings"
	"text/template"
)

// Context identifies a UI state in which a custom command may fire.
type Context string

const (
	CtxIssues         Context = "issues"
	CtxInfo           Context = "info"
	CtxProjects       Context = "projects"
	CtxDetail         Context = "detail"
	CtxDetailComments Context = "detail.comments"
)

// DefaultCommandContexts is applied when a command omits `contexts:`.
var DefaultCommandContexts = []Context{CtxIssues, CtxInfo, CtxDetail}

// ScopeMask is a bitmask of data scopes a command expects.
type ScopeMask uint8

const (
	ScopeIssue ScopeMask = 1 << iota
	ScopeProject
	ScopeComment
)

var contextScopes = map[Context]ScopeMask{
	CtxIssues:         ScopeIssue,
	CtxInfo:           ScopeIssue,
	CtxProjects:       ScopeProject,
	CtxDetail:         ScopeIssue,
	CtxDetailComments: ScopeIssue | ScopeComment,
}

// ResolvedCustomCommand is a validated, pre-computed custom command.
type ResolvedCustomCommand struct {
	Key      string
	Name     string
	Command  string
	Suspend  *bool
	Refresh  bool
	Contexts []Context
	Scopes   ScopeMask
	Template *template.Template
}

// ShouldSuspend mirrors CustomCommandConfig.ShouldSuspend.
func (r ResolvedCustomCommand) ShouldSuspend() bool {
	return r.Suspend == nil || *r.Suspend
}

// HasContext reports whether the command is bound to the given context.
func (r ResolvedCustomCommand) HasContext(c Context) bool {
	return slices.Contains(r.Contexts, c)
}

func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

var commandFuncMap = template.FuncMap{
	"shellescape": shellescape,
}

// ResolveCustomCommands validates the flat CustomCommands list and returns
// pre-computed entries with typed contexts, scope mask and parsed template.
func (c *Config) ResolveCustomCommands() ([]ResolvedCustomCommand, error) {
	out := make([]ResolvedCustomCommand, 0, len(c.CustomCommands))
	type keyCtx struct {
		key string
		ctx Context
	}
	seen := make(map[keyCtx]bool)

	for i, entry := range c.CustomCommands {
		if entry.Key == "" || entry.Name == "" || entry.Command == "" {
			return nil, fmt.Errorf("customCommands[%d] (%q): key, name and command must all be non-empty", i, entry.Name)
		}

		var ctxs []Context
		if len(entry.Contexts) == 0 {
			ctxs = append(ctxs, DefaultCommandContexts...)
		} else {
			for _, raw := range entry.Contexts {
				ctx := Context(raw)
				if _, ok := contextScopes[ctx]; !ok {
					return nil, fmt.Errorf("customCommands[%d] (%q): unknown context %q", i, entry.Key, raw)
				}
				ctxs = append(ctxs, ctx)
			}
		}

		var scopes ScopeMask
		for _, ctx := range ctxs {
			scopes |= contextScopes[ctx]
			k := keyCtx{entry.Key, ctx}
			if seen[k] {
				return nil, fmt.Errorf("customCommands: duplicate key %q in context %q", entry.Key, ctx)
			}
			seen[k] = true
		}

		tmpl, err := template.New("customCommand").
			Option("missingkey=error").
			Funcs(commandFuncMap).
			Parse(entry.Command)
		if err != nil {
			return nil, fmt.Errorf("customCommands[%d] (%q): template parse error: %w", i, entry.Key, err)
		}

		out = append(out, ResolvedCustomCommand{
			Key:      entry.Key,
			Name:     entry.Name,
			Command:  entry.Command,
			Suspend:  entry.Suspend,
			Refresh:  entry.Refresh,
			Contexts: ctxs,
			Scopes:   scopes,
			Template: tmpl,
		})
	}
	return out, nil
}
