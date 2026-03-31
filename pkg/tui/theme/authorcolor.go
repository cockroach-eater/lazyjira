package theme

import (
	"crypto/md5"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Distinct colors from ANSI 256 palette readable on dark backgrounds
var authorPalette = []lipgloss.Color{
	lipgloss.Color("208"), // orange
	lipgloss.Color("176"), // pink/magenta
	lipgloss.Color("114"), // light green
	lipgloss.Color("216"), // salmon
	lipgloss.Color("81"),  // sky blue
	lipgloss.Color("222"), // gold
	lipgloss.Color("183"), // lavender
	lipgloss.Color("150"), // sage
	lipgloss.Color("209"), // coral
	lipgloss.Color("117"), // light cyan
	lipgloss.Color("180"), // tan
	lipgloss.Color("147"), // periwinkle
}

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
