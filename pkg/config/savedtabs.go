package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const savedTabsFile = "saved_tabs.yml"

// ManagedTab is a TUI-managed issue tab persisted in saved_tabs.yml. Unlike
// IssueTabConfig (hand-edited templates in config.yml), a managed tab holds a
// concrete JQL query bound to a project (empty Project means global).
type ManagedTab struct {
	Name       string `yaml:"name"`
	JQL        string `yaml:"jql"`
	Project    string `yaml:"project"`
	MaxResults *int   `yaml:"maxResults"`
}

type savedTabsFileFormat struct {
	Tabs []ManagedTab `yaml:"tabs"`
}

// LoadSavedTabs reads managed tabs from ConfigDir()/saved_tabs.yml.
// Returns nil on a missing or unreadable file, mirroring LoadJQLHistory.
func LoadSavedTabs() []ManagedTab {
	path := filepath.Join(ConfigDir(), savedTabsFile)
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil
	}
	var f savedTabsFileFormat
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil
	}
	return f.Tabs
}

// SaveSavedTabs writes managed tabs to ConfigDir()/saved_tabs.yml. It marshals
// only the managed-tabs store, never config.yml, so the hand-edited config is
// left untouched.
func SaveSavedTabs(tabs []ManagedTab) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(savedTabsFileFormat{Tabs: tabs})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, savedTabsFile), data, 0o644)
}
