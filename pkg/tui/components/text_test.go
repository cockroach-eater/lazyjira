package components

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncateMiddle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string // exact match (empty = skip exact check)
	}{
		{"short no truncation", "hello", 10, "hello"},
		{"exact fit", "hello", 5, "hello"},
		{"basic truncation", "Start Progress to In Progress", 20, ""},
		{"unicode arrow no truncation", "Start Progress → In Progress", 30, "Start Progress → In Progress"},
		{"unicode arrow truncated", "Start Progress → In Progress", 20, ""},
		{"very small", "hello world", 4, ""},
		{"emoji", "Bug 🐛 fix needed", 12, ""},
		{"cyrillic", "Проверка кириллицы тут", 15, ""},
		{"empty", "", 10, ""},
		{"maxWidth 0", "hello", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateMiddle(tt.input, tt.maxWidth)

			if tt.want != "" && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}

			// Must fit within maxWidth display columns.
			if tt.maxWidth > 0 {
				w := lipgloss.Width(got)
				if w > tt.maxWidth {
					t.Errorf("got %q (width %d), exceeds max %d", got, w, tt.maxWidth)
				}
			}

			// Must never contain replacement character (broken UTF-8).
			for _, r := range got {
				if r == '\uFFFD' {
					t.Errorf("got %q, contains replacement char U+FFFD", got)
					break
				}
			}
		})
	}
}

func TestTruncateEnd(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
	}{
		{"short", "short", 10},
		{"needs truncation", "a longer string here", 10},
		{"unicode arrow", "→ arrow", 5},
		{"wide chars", "日本語テスト", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateEnd(tt.input, tt.maxWidth)
			w := lipgloss.Width(got)
			if w > tt.maxWidth {
				t.Errorf("got %q (width %d), exceeds max %d", got, w, tt.maxWidth)
			}
		})
	}
}
