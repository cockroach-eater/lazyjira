package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveCustomCommands_Defaults(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "y", Name: "Copy", Command: "echo {{.Key}}"},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("len = %d, want 1", len(resolved))
	}
	got := resolved[0]
	if len(got.Contexts) != len(DefaultCommandContexts) {
		t.Errorf("Contexts = %v, want %v", got.Contexts, DefaultCommandContexts)
	}
	if got.Scopes != ScopeIssue {
		t.Errorf("Scopes = %d, want %d", got.Scopes, ScopeIssue)
	}
}

func TestResolveCustomCommands_SingleContextProjects(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "n", Name: "Notes", Command: "echo {{.ProjectKey}}", Contexts: []string{"projects"}},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resolved[0]
	if got.Scopes != ScopeProject {
		t.Errorf("Scopes = %d, want %d", got.Scopes, ScopeProject)
	}
}

func TestResolveCustomCommands_DetailComments(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "c", Name: "Comment", Command: "echo {{.Key}}-{{.CommentID}}", Contexts: []string{"detail.comments"}},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resolved[0]
	if got.Scopes != ScopeIssue|ScopeComment {
		t.Errorf("Scopes = %d, want %d", got.Scopes, ScopeIssue|ScopeComment)
	}
}

func TestResolveCustomCommands_MixedScopes(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "x", Name: "Mixed", Command: "echo x", Contexts: []string{"issues", "detail.comments"}},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved[0].Scopes != ScopeIssue|ScopeComment {
		t.Errorf("Scopes = %d, want %d", resolved[0].Scopes, ScopeIssue|ScopeComment)
	}
}

func TestResolveCustomCommands_EmptyFieldsError(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  CustomCommandConfig
	}{
		{"empty key", CustomCommandConfig{Name: "x", Command: "echo"}},
		{"empty name", CustomCommandConfig{Key: "a", Command: "echo"}},
		{"empty command", CustomCommandConfig{Key: "a", Name: "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{CustomCommands: []CustomCommandConfig{tc.cmd}}
			if _, err := cfg.ResolveCustomCommands(); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestResolveCustomCommands_UnknownContextError(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "a", Name: "x", Command: "echo", Contexts: []string{"bogus"}},
		},
	}
	_, err := cfg.ResolveCustomCommands()
	if err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected unknown context error, got %v", err)
	}
}

func TestResolveCustomCommands_DuplicateKeySameContextError(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "a", Name: "first", Command: "echo 1", Contexts: []string{"issues"}},
			{Key: "a", Name: "second", Command: "echo 2", Contexts: []string{"issues"}},
		},
	}
	if _, err := cfg.ResolveCustomCommands(); err == nil {
		t.Error("expected duplicate key error")
	}
}

func TestResolveCustomCommands_DuplicateKeyDisjointContextsAllowed(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "a", Name: "first", Command: "echo 1", Contexts: []string{"issues"}},
			{Key: "a", Name: "second", Command: "echo 2", Contexts: []string{"projects"}},
		},
	}
	if _, err := cfg.ResolveCustomCommands(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveCustomCommands_RefreshDefault(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "y", Name: "Copy", Command: "echo {{.Key}}"},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved[0].Refresh {
		t.Error("Refresh should default to false")
	}
}

func TestResolveCustomCommands_RefreshTrue(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "w", Name: "Log work", Command: "echo {{.Key}}", Refresh: true},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved[0].Refresh {
		t.Error("Refresh should be true when set")
	}
}

func TestResolveCustomCommands_InvalidTemplateError(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "a", Name: "x", Command: "echo {{.Unclosed"},
		},
	}
	if _, err := cfg.ResolveCustomCommands(); err == nil {
		t.Error("expected template parse error")
	}
}

func TestResolveCustomCommands_ShellescapeFunc(t *testing.T) {
	cfg := &Config{
		CustomCommands: []CustomCommandConfig{
			{Key: "s", Name: "Shellescape", Command: "echo {{.Key | shellescape}}"},
		},
	}
	resolved, err := cfg.ResolveCustomCommands()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	data := struct{ Key string }{Key: "what's up"}
	if err := resolved[0].Template.Execute(&buf, data); err != nil {
		t.Fatalf("template execute error: %v", err)
	}

	want := `echo 'what'\''s up'`
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
