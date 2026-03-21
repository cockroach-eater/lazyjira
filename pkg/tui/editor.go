package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// editorFinishedMsg is sent when $EDITOR exits.
type editorFinishedMsg struct {
	original string // original content (trimmed) for comparison
	tempPath string // path to temp file
	err      error  // non-nil if editor launch failed
}

var errNoEditor = errors.New("no editor found — set $EDITOR environment variable")

// resolveEditor returns the editor command: $EDITOR → $VISUAL → vi.
func resolveEditor() (string, error) {
	if e := os.Getenv("EDITOR"); e != "" {
		return e, nil
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e, nil
	}
	if path, err := exec.LookPath("vi"); err == nil {
		return path, nil
	}
	return "", errNoEditor
}

// launchEditor writes content to a temp file and opens it in $EDITOR.
// Returns a tea.Cmd that suspends the TUI via tea.ExecProcess.
func launchEditor(content, suffix string) tea.Cmd {
	editor, err := resolveEditor()
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	tmpFile, err := os.CreateTemp("", "lazyjira-*"+suffix)
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}
	_ = tmpFile.Close()

	original := strings.TrimRight(content, "\n")
	cmd := exec.CommandContext(context.Background(), editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{
			original: original,
			tempPath: tmpPath,
			err:      err,
		}
	})
}

// readAndCheckEditor reads the temp file and checks if content changed.
func readAndCheckEditor(msg editorFinishedMsg) (string, bool, error) {
	if msg.err != nil {
		return "", false, msg.err
	}
	data, err := os.ReadFile(msg.tempPath)
	if err != nil {
		return "", false, err
	}
	newContent := strings.TrimRight(string(data), "\n")
	changed := newContent != msg.original
	return newContent, changed, nil
}

// cleanupEditor removes the temp file.
func cleanupEditor(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
