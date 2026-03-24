package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Jira         JiraConfig          `yaml:"jira"`
	Projects     []ProjectConfig     `yaml:"projects"`
	GUI          GUIConfig           `yaml:"gui"`
	Keybinding   KeybindingConfig    `yaml:"keybinding"`
	IssueTabs    []IssueTabConfig    `yaml:"issueTabs"`
	Cache        CacheConfig         `yaml:"cache"`
	Refresh      RefreshConfig       `yaml:"refresh"`
	CustomFields []CustomFieldConfig `yaml:"customFields"`
}

type IssueTabConfig struct {
	Name string `yaml:"name"`
	JQL  string `yaml:"jql"` // supports {{.ProjectKey}}, {{.UserEmail}}
}

type CustomFieldConfig struct {
	ID   string `yaml:"id"`   // e.g. "customfield_10015"
	Name string `yaml:"name"` // display name, e.g. "Story Points"
	Type string `yaml:"type"` // "select", "multiselect", "user", "text", "textarea" (default: "text")
}

type KeybindingConfig struct {
	Universal UniversalKeys `yaml:"universal"`
	Issues    IssueKeys     `yaml:"issues"`
	Projects  ProjectKeys   `yaml:"projects"`
	Detail    DetailKeys    `yaml:"detail"`
}

type UniversalKeys struct {
	Quit        string `yaml:"quit"`
	Help        string `yaml:"help"`
	Search      string `yaml:"search"`
	SwitchPanel string `yaml:"switchPanel"`
	Refresh     string `yaml:"refresh"`
	RefreshAll  string `yaml:"refreshAll"`
	PrevTab     string `yaml:"prevTab"`
	NextTab     string `yaml:"nextTab"`
	FocusDetail string `yaml:"focusDetail"`
	FocusStatus string `yaml:"focusStatus"`
	FocusIssues string `yaml:"focusIssues"`
	FocusProj   string `yaml:"focusProjects"`
	JQLSearch   string `yaml:"jqlSearch"`
}

type IssueKeys struct {
	Select     string `yaml:"select"`
	Open       string `yaml:"open"`
	FocusRight string `yaml:"focusRight"`
	Transition string `yaml:"transition"`
	Browser    string `yaml:"browser"`
	URLPicker  string `yaml:"urlPicker"`
	CopyURL    string `yaml:"copyURL"`
	CloseJQLTab string `yaml:"closeJQLTab"`
}

type ProjectKeys struct {
	Select     string `yaml:"select"`
	Open       string `yaml:"open"`
	FocusRight string `yaml:"focusRight"`
}

type DetailKeys struct {
	FocusLeft string `yaml:"focusLeft"`
	InfoTab   string `yaml:"infoTab"`
}

type JiraConfig struct {
	Host  string `yaml:"host"`
	Email string `yaml:"email"`
	Token string `yaml:"-"` // never saved to file
}

type ProjectConfig struct {
	Key     string `yaml:"key"`
	BoardID int    `yaml:"boardId"`
}

type GUIConfig struct {
	Theme           string   `yaml:"theme"`
	Language        string   `yaml:"language"`
	SidePanelWidth  int      `yaml:"sidePanelWidth"`
	ShowIcons       bool     `yaml:"showIcons"`
	DateFormat      string   `yaml:"dateFormat"`
	Mouse           bool     `yaml:"mouse"`
	Borders         string   `yaml:"borders"`
	IssueListFields []string `yaml:"issueListFields"`
}

type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	TTL     string `yaml:"ttl"`
}

type RefreshConfig struct {
	AutoRefresh bool   `yaml:"autoRefresh"`
	Interval    string `yaml:"interval"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		GUI: GUIConfig{
			Theme:          "default",
			Language:       "en",
			SidePanelWidth: 40,
			ShowIcons:      true,
			DateFormat:     "2006-01-02",
			Mouse:          true,
			Borders:        "rounded",
			IssueListFields: []string{
				"key", "status", "summary",
			},
		},
		IssueTabs: DefaultIssueTabs(),
		Cache: CacheConfig{
			Enabled: true,
			TTL:     "5m",
		},
		Refresh: RefreshConfig{
			AutoRefresh: true,
			Interval:    "30s",
		},
	}
}

// DefaultIssueTabs returns the default issue tab configuration.
func DefaultIssueTabs() []IssueTabConfig {
	return []IssueTabConfig{
		{Name: "All", JQL: "project = {{.ProjectKey}} AND statusCategory != Done ORDER BY updated DESC"},
		{Name: "Assigned", JQL: "project = {{.ProjectKey}} AND assignee=currentUser() AND statusCategory != Done ORDER BY priority DESC, updated DESC"},
	}
}

// ConfigDir returns the lazyjira configuration directory path.
// Priority: CONFIG_DIR env > XDG_CONFIG_HOME/lazyjira > os.UserConfigDir()/lazyjira > ~/.config/lazyjira
func ConfigDir() string {
	if dir := os.Getenv("CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "lazyjira")
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "lazyjira")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "lazyjira")
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

// Load reads the config file, merges it with defaults, and applies
// environment variable overrides for Jira credentials.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Environment variables always take precedence.
	if v := os.Getenv("JIRA_HOST"); v != "" {
		cfg.Jira.Host = v
	}
	if v := os.Getenv("JIRA_EMAIL"); v != "" {
		cfg.Jira.Email = v
	}
	if v := os.Getenv("JIRA_API_TOKEN"); v != "" {
		cfg.Jira.Token = v
	}

	return cfg, nil
}

// Save writes the config to the config file. The Jira API token is never
// persisted because the Token field carries the yaml:"-" tag.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), data, 0o644)
}
