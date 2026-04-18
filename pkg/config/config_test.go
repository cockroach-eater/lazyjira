package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_TLSEnvVars(t *testing.T) {
	t.Setenv("CONFIG_DIR", t.TempDir())
	t.Setenv("JIRA_SERVER_TYPE", "server")
	t.Setenv("JIRA_TLS_CERT", "/tmp/cert.pem")
	t.Setenv("JIRA_TLS_KEY", "/tmp/key.pem")
	t.Setenv("JIRA_TLS_CA", "/tmp/ca.pem")
	t.Setenv("JIRA_TLS_INSECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Jira.ServerType != "server" {
		t.Errorf("ServerType = %q, want server", cfg.Jira.ServerType)
	}
	if cfg.Jira.TLS.CertFile != "/tmp/cert.pem" {
		t.Errorf("CertFile = %q", cfg.Jira.TLS.CertFile)
	}
	if cfg.Jira.TLS.KeyFile != "/tmp/key.pem" {
		t.Errorf("KeyFile = %q", cfg.Jira.TLS.KeyFile)
	}
	if cfg.Jira.TLS.CAFile != "/tmp/ca.pem" {
		t.Errorf("CAFile = %q", cfg.Jira.TLS.CAFile)
	}
	if !cfg.Jira.TLS.Insecure {
		t.Error("Insecure should be true")
	}
}

func TestLoad_CustomCommandRefreshFromYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CONFIG_DIR", dir)

	cfgYAML := `customCommands:
  - key: "y"
    name: "copy"
    command: "echo {{.Key}}"
    suspend: false
  - key: "w"
    name: "log work"
    command: "echo {{.Key}}"
    refresh: true
`
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CustomCommands[0].Refresh {
		t.Error("first command: Refresh should default to false")
	}
	if !cfg.CustomCommands[1].Refresh {
		t.Error("second command: Refresh should be true")
	}
}

func TestLoad_InvalidCustomCommandTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CONFIG_DIR", dir)

	cfgYAML := `customCommands:
  - key: "y"
    name: "broken"
    command: "echo {{.Unclosed"
`
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid template, got nil")
	}
	if !strings.Contains(err.Error(), "template parse error") {
		t.Errorf("error = %q, want it to mention template parse error", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestResolveMaxResults(t *testing.T) {
	tests := []struct {
		name   string
		global *int
		tab    IssueTabConfig
		want   int
	}{
		{"all unset → default", nil, IssueTabConfig{}, DefaultMaxResults},
		{"global only", ptr(25), IssueTabConfig{}, 25},
		{"tab overrides global", ptr(25), IssueTabConfig{MaxResults: ptr(75)}, 75},
		{"negative global ignored", ptr(-5), IssueTabConfig{}, DefaultMaxResults},
		{"zero tab falls back to global", ptr(40), IssueTabConfig{MaxResults: ptr(0)}, 40},
		{"large global not clamped", ptr(500), IssueTabConfig{}, 500},
		{"large tab override not clamped", ptr(50), IssueTabConfig{MaxResults: ptr(1000)}, 1000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{MaxResults: tc.global}
			if got := c.ResolveMaxResults(tc.tab); got != tc.want {
				t.Errorf("ResolveMaxResults = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestResolveGlobalMaxResults(t *testing.T) {
	tests := []struct {
		name   string
		global *int
		want   int
	}{
		{"nil → default", nil, DefaultMaxResults},
		{"zero → default", ptr(0), DefaultMaxResults},
		{"negative → default", ptr(-1), DefaultMaxResults},
		{"set", ptr(125), 125},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{MaxResults: tc.global}
			if got := c.ResolveGlobalMaxResults(); got != tc.want {
				t.Errorf("ResolveGlobalMaxResults = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDefaultConfig_MaxResults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxResults != nil {
		t.Errorf("default MaxResults should be nil (unset), got %d", *cfg.MaxResults)
	}
	if got := cfg.ResolveGlobalMaxResults(); got != DefaultMaxResults {
		t.Errorf("ResolveGlobalMaxResults on defaults = %d, want %d", got, DefaultMaxResults)
	}
}
