package theme

import (
	"crypto/md5"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// authorPalette is sourced from the active theme's AuthorPalette.
// SetTheme refreshes it on theme change.
var authorPalette = Default.AuthorPalette

var authorCache = make(map[string]lipgloss.Style)

// authorKey normalizes a name for consistent color by stripping @ prefix and trimming spaces
func authorKey(name string) string {
	return strings.TrimSpace(strings.TrimPrefix(name, "@"))
}

// AuthorStyle returns a deterministic color for a given author name
// "@Aleksandr Savinykh" and "Aleksandr Savinykh" get the same color
func AuthorStyle(name string) lipgloss.Style {
	key := authorKey(name)
	if s, ok := authorCache[key]; ok {
		return s
	}
	hash := md5.Sum([]byte(key))
	idx := int(hash[0]) % len(authorPalette)
	s := lipgloss.NewStyle().Foreground(authorPalette[idx])
	authorCache[key] = s
	return s
}

// AuthorRender renders a name with its deterministic color
func AuthorRender(name string) string {
	return AuthorStyle(name).Render(name)
}
