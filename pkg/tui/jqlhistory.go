package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/textfuel/lazyjira/pkg/config"
)

const (
	jqlHistoryFile    = "jql_history"
	jqlHistoryMaxSize = 50
)

// LoadJQLHistory loads JQL queries from the history file.
// Returns empty slice on error or missing file
func LoadJQLHistory() []string {
	path := filepath.Join(config.ConfigDir(), jqlHistoryFile)
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// SaveJQLHistory writes queries to the history file
func SaveJQLHistory(queries []string) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, jqlHistoryFile)
	content := strings.Join(queries, "\n")
	return os.WriteFile(path, []byte(content), 0o644)
}

// AddToHistory prepends a new query to the history, deduplicating.
// Returns the updated history capped at 50 entries
func AddToHistory(history []string, newQuery string) []string {
	newQuery = strings.TrimSpace(newQuery)
	if newQuery == "" {
		return history
	}
	var result []string
	result = append(result, newQuery)
	for _, q := range history {
		if q != newQuery {
			result = append(result, q)
		}
	}
	if len(result) > jqlHistoryMaxSize {
		result = result[:jqlHistoryMaxSize]
	}
	return result
}
