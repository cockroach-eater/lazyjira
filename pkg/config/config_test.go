package config

import "testing"

func TestLoad_TLSEnvVars(t *testing.T) {
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
