package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Credentials stores Jira connection info.
type Credentials struct {
	Host        string `json:"host"`
	Email       string `json:"email"`
	Token       string `json:"token"`
	LastProject string `json:"last_project,omitempty"`
}

// AuthPath returns the path to auth.json in the config directory.
func AuthPath() string {
	return filepath.Join(ConfigDir(), "auth.json")
}

// LoadCredentials reads saved credentials. Returns nil if not found.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(AuthPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// SaveCredentials writes credentials to auth.json with 0600 permissions.
func SaveCredentials(creds *Credentials) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(AuthPath(), data, 0o600)
}

// ClearCredentials removes auth.json.
func ClearCredentials() error {
	err := os.Remove(AuthPath())
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
