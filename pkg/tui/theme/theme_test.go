package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSetThemeDefault(t *testing.T) {
	// Ensure SetTheme("default") restores ANSI palette.
	if err := SetTheme("default"); err != nil {
		t.Fatalf("SetTheme(default): %v", err)
	}
	if ColorGreen != lipgloss.Color("2") {
		t.Errorf("ColorGreen = %q, want %q", ColorGreen, "2")
	}
	if ColorBlue != lipgloss.Color("4") {
		t.Errorf("ColorBlue = %q, want %q", ColorBlue, "4")
	}
}

func TestSetThemeEmpty(t *testing.T) {
	if err := SetTheme(""); err != nil {
		t.Fatalf("SetTheme(''): %v", err)
	}
	if ColorGreen != lipgloss.Color("2") {
		t.Errorf("ColorGreen = %q, want %q", ColorGreen, "2")
	}
}

func TestSetThemeCatppuccinMocha(t *testing.T) {
	if err := SetTheme("catppuccin-mocha"); err != nil {
		t.Fatalf("SetTheme(catppuccin-mocha): %v", err)
	}
	if ColorGreen != lipgloss.Color("#a6e3a1") {
		t.Errorf("ColorGreen = %q, want %q", ColorGreen, "#a6e3a1")
	}
	if ColorBlue != lipgloss.Color("#89b4fa") {
		t.Errorf("ColorBlue = %q, want %q", ColorBlue, "#89b4fa")
	}
	if Default.Colors.Red != lipgloss.Color("#f38ba8") {
		t.Errorf("Default.Colors.Red = %q, want %q", Default.Colors.Red, "#f38ba8")
	}

	// Restore default for other tests.
	_ = SetTheme("default")
}

func TestSetThemeAllFlavors(t *testing.T) {
	flavors := []string{
		"catppuccin-latte",
		"catppuccin-frappe",
		"catppuccin-macchiato",
		"catppuccin-mocha",
	}
	for _, name := range flavors {
		t.Run(name, func(t *testing.T) {
			if err := SetTheme(name); err != nil {
				t.Fatalf("SetTheme(%s): %v", name, err)
			}
			// Verify palette is populated (not empty).
			if Default.Colors.Green == "" {
				t.Error("Colors.Green is empty")
			}
			if len(Default.AuthorPalette) != 12 {
				t.Errorf("AuthorPalette has %d entries, want 12", len(Default.AuthorPalette))
			}
		})
	}
	_ = SetTheme("default")
}

func TestSetThemeUnknown(t *testing.T) {
	err := SetTheme("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown theme")
	}
}

func TestSetThemeSyncsColors(t *testing.T) {
	_ = SetTheme("catppuccin-mocha")
	// Package-level vars must match Default.Colors.
	if ColorGreen != Default.Colors.Green {
		t.Errorf("ColorGreen not synced: %q != %q", ColorGreen, Default.Colors.Green)
	}
	if ColorBlue != Default.Colors.Blue {
		t.Errorf("ColorBlue not synced: %q != %q", ColorBlue, Default.Colors.Blue)
	}
	if ColorOrange != Default.Colors.Orange {
		t.Errorf("ColorOrange not synced: %q != %q", ColorOrange, Default.Colors.Orange)
	}
	if ColorMagenta != Default.Colors.Magenta {
		t.Errorf("ColorMagenta not synced: %q != %q", ColorMagenta, Default.Colors.Magenta)
	}
	_ = SetTheme("default")
}

func TestSetThemeSyncsAuthorPalette(t *testing.T) {
	_ = SetTheme("default")
	defaultFirst := authorPalette[0]

	_ = SetTheme("catppuccin-mocha")
	if authorPalette[0] == defaultFirst {
		t.Error("authorPalette did not switch when theme changed")
	}
	if len(authorPalette) != len(Default.AuthorPalette) {
		t.Errorf("authorPalette length %d != Default.AuthorPalette length %d",
			len(authorPalette), len(Default.AuthorPalette))
	}
	_ = SetTheme("default")
}

func TestSetThemeResetsAuthorCache(t *testing.T) {
	_ = SetTheme("default")
	// Prime the author cache.
	_ = AuthorStyle("Alice")
	if len(authorCache) == 0 {
		t.Fatal("author cache should have an entry")
	}

	// Switching theme must clear the cache.
	_ = SetTheme("catppuccin-mocha")
	if len(authorCache) != 0 {
		t.Error("author cache should be empty after theme switch")
	}
	_ = SetTheme("default")
}
