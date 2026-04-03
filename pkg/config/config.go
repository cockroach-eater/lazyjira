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
	Fields           []FieldConfig      `yaml:"fields"`
	DeprecatedFields []FieldConfig      `yaml:"customFields,omitempty"`
	Git              GitConfig          `yaml:"git"`
}

type GitConfig struct {
	CloseOnCheckout bool               `yaml:"closeOnCheckout"`
	AsciiOnly       bool               `yaml:"asciiOnly"`
	BranchFormat    []BranchFormatRule `yaml:"branchFormat"`
}

type BranchFormatRule struct {
	When     BranchFormatCondition `yaml:"when"`
	Template string                `yaml:"template"`
}

type BranchFormatCondition struct {
	Type string `yaml:"type"`
}

type IssueTabConfig struct {
	Name string `yaml:"name"`
	JQL  string `yaml:"jql"`
}

type FieldConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Multiline bool   `yaml:"multiline"`
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
	FocusInfo   string `yaml:"focusInfo"`
	FocusProj   string `yaml:"focusProjects"`
	JQLSearch   string `yaml:"jqlSearch"`
}

type IssueKeys struct {
	Select       string `yaml:"select"`
	Open         string `yaml:"open"`
	FocusRight   string `yaml:"focusRight"`
	Transition   string `yaml:"transition"`
	Browser      string `yaml:"browser"`
	URLPicker    string `yaml:"urlPicker"`
	CopyURL      string `yaml:"copyURL"`
	CloseJQLTab  string `yaml:"closeJQLTab"`
	CreateBranch string `yaml:"createBranch"`
	CreateIssue  string `yaml:"createIssue"`
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
	Host       string    `yaml:"host"`
	Email      string    `yaml:"email"`
	Token      string    `yaml:"-"`
	ServerType string    `yaml:"serverType"`
	TLS        TLSConfig `yaml:"tls"`
}

// IsCloud returns true if this is a Jira Cloud instance (or unset, which defaults to Cloud)
func (j JiraConfig) IsCloud() bool {
	return j.ServerType == "" || j.ServerType == "cloud"
}

type TLSConfig struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
	CAFile   string `yaml:"caFile"`
	Insecure bool   `yaml:"insecure"`
}

type ProjectConfig struct {
	Key     string `yaml:"key"`
	BoardID int    `yaml:"boardId"` // TODO not yet wired up
}

type GUIConfig struct {
	Theme           string   `yaml:"theme"`    // TODO not yet wired up
	Language        string   `yaml:"language"` // TODO not yet wired up
	SidePanelWidth  int      `yaml:"sidePanelWidth"`
	ShowIcons       bool     `yaml:"showIcons"`  // TODO not yet wired up
	DateFormat      string   `yaml:"dateFormat"` // TODO not yet wired up
	Mouse           bool     `yaml:"mouse"`      // TODO not yet wired up
	Borders         string   `yaml:"borders"`    // TODO not yet wired up
	IssueListFields    []string `yaml:"issueListFields"`
	PrefillFromTab     *bool    `yaml:"prefillFromTab"`
	SelectCreatedIssue *bool    `yaml:"selectCreatedIssue"`
}

// ShouldPrefillFromTab returns true when the creation form should prefill from tab JQL
func (g GUIConfig) ShouldPrefillFromTab() bool {
	return g.PrefillFromTab == nil || *g.PrefillFromTab
}

// ShouldSelectCreatedIssue returns true when the app should auto-select a newly created issue
func (g GUIConfig) ShouldSelectCreatedIssue() bool {
	return g.SelectCreatedIssue == nil || *g.SelectCreatedIssue
}

// TODO not yet wired up
type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	TTL     string `yaml:"ttl"`
}

// TODO not yet wired up
type RefreshConfig struct {
	AutoRefresh bool   `yaml:"autoRefresh"`
	Interval    string `yaml:"interval"`
}

// DefaultConfig returns a Config populated with sensible defaults
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

// DefaultIssueTabs returns the default issue tab configuration
func DefaultIssueTabs() []IssueTabConfig {
	return []IssueTabConfig{
		{Name: "All", JQL: "project = {{.ProjectKey}} AND statusCategory != Done ORDER BY updated DESC"},
		{Name: "Assigned", JQL: "project = {{.ProjectKey}} AND assignee=currentUser() AND statusCategory != Done ORDER BY priority DESC, updated DESC"},
	}
}

// ConfigDir returns the lazyjira configuration directory path
// Order of precedence: CONFIG_DIR env, XDG_CONFIG_HOME/lazyjira, os.UserConfigDir()/lazyjira, ~/.config/lazyjira
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

// ConfigPath returns the full path to the config file
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

// Load reads the config file, merges it with defaults, and applies
// environment variable overrides for Jira credentials
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

	if cfg.Fields == nil && len(cfg.DeprecatedFields) > 0 {
		cfg.Fields = cfg.DeprecatedFields
		cfg.DeprecatedFields = nil
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
	if v := os.Getenv("JIRA_SERVER_TYPE"); v != "" {
		cfg.Jira.ServerType = v
	}
	if v := os.Getenv("JIRA_TLS_CERT"); v != "" {
		cfg.Jira.TLS.CertFile = v
	}
	if v := os.Getenv("JIRA_TLS_KEY"); v != "" {
		cfg.Jira.TLS.KeyFile = v
	}
	if v := os.Getenv("JIRA_TLS_CA"); v != "" {
		cfg.Jira.TLS.CAFile = v
	}
	if v := os.Getenv("JIRA_TLS_INSECURE"); v == "1" || v == "true" {
		cfg.Jira.TLS.Insecure = true
	}

	return cfg, nil
}

// Save writes the config to the config file. The Jira API token is never
// persisted because the Token field carries the yaml:"-" tag
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
