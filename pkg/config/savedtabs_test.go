package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSavedTabs_missingFile_returnsNil(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())

	if got := LoadSavedTabs(); got != nil {
		t.Errorf("LoadSavedTabs() = %v, want nil for missing file", got)
	}
}

func TestSaveThenLoad_roundtrip(t *testing.T) {
	t.Setenv("LAZYJIRA_CONFIG_DIR", t.TempDir())

	limit := 25
	want := []ManagedTab{
		{Name: "Bugs", JQL: "type = Bug", Project: "ABC", MaxResults: &limit},
		{Name: "Global", JQL: "assignee = currentUser()", Project: ""},
	}

	if err := SaveSavedTabs(want); err != nil {
		t.Fatalf("SaveSavedTabs() error = %v", err)
	}

	got := LoadSavedTabs()
	if len(got) != len(want) {
		t.Fatalf("loaded %d tabs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].JQL != want[i].JQL || got[i].Project != want[i].Project {
			t.Errorf("tab %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[0].MaxResults == nil || *got[0].MaxResults != limit {
		t.Errorf("tab 0 MaxResults = %v, want %d", got[0].MaxResults, limit)
	}
	if got[1].MaxResults != nil {
		t.Errorf("tab 1 MaxResults = %v, want nil", got[1].MaxResults)
	}
}

func TestSaveSavedTabs_doesNotTouchConfigYml(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LAZYJIRA_CONFIG_DIR", dir)

	sentinel := []byte("# hand-tuned config\njira:\n  url: https://example.atlassian.net\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SaveSavedTabs([]ManagedTab{{Name: "X", JQL: "project = X"}}); err != nil {
		t.Fatalf("SaveSavedTabs() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "config.yml")) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("config.yml was modified:\ngot:  %q\nwant: %q", got, sentinel)
	}
}
