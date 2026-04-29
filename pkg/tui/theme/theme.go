package theme

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ColorPalette holds the semantic color values for a theme.
// The default theme uses ANSI 16 codes; Catppuccin themes use hex values.
type ColorPalette struct {
	Green     lipgloss.Color
	Blue      lipgloss.Color
	Red       lipgloss.Color
	Yellow    lipgloss.Color
	Cyan      lipgloss.Color
	Magenta   lipgloss.Color
	White     lipgloss.Color
	Gray      lipgloss.Color
	Orange    lipgloss.Color
	None      lipgloss.Color
	Highlight lipgloss.Color // selection/cursor background
}

// Package-level color variables. These are kept in sync with Default.Colors
// by SetTheme so that existing call sites (theme.ColorBlue, etc.) continue
// to work without changes.
var (
	ColorGreen     = lipgloss.Color("2")   // ANSI green — active borders, accents
	ColorBlue      = lipgloss.Color("4")   // ANSI blue — help bar, selected bg
	ColorRed       = lipgloss.Color("1")   // ANSI red — errors, unstaged
	ColorYellow    = lipgloss.Color("3")   // ANSI yellow — warnings, in-progress
	ColorCyan      = lipgloss.Color("6")   // ANSI cyan — search mode
	ColorMagenta   = lipgloss.Color("5")   // ANSI magenta — JQL keywords
	ColorWhite     = lipgloss.Color("7")   // ANSI white (light gray)
	ColorGray      = lipgloss.Color("8")   // ANSI bright black (dark gray)
	ColorOrange    = lipgloss.Color("208") // ANSI 256 orange — secondary accent (names, metadata)
	ColorNone      = lipgloss.Color("-1")  // default terminal color
	ColorHighlight = lipgloss.Color("4")   // selection/cursor background (same as blue by default)
)

type Theme struct {
	Title          lipgloss.Style
	Subtitle       lipgloss.Style
	HintBar        lipgloss.Style
	SelectedItem   lipgloss.Style
	NormalItem     lipgloss.Style
	ActiveBorder   lipgloss.Style
	InactiveBorder lipgloss.Style
	ErrorText      lipgloss.Style
	SuccessText    lipgloss.Style
	WarningText    lipgloss.Style
	KeyStyle       lipgloss.Style
	ValueStyle     lipgloss.Style
	PriorityHigh   lipgloss.Style
	PriorityMedium lipgloss.Style
	PriorityLow    lipgloss.Style

	Colors        ColorPalette
	AuthorPalette []lipgloss.Color
}

// Default is the singleton theme instance
var Default = defaultTheme()

// DefaultTheme returns the singleton theme. Kept for compatibility
func DefaultTheme() *Theme { return Default }

// defaultPalette returns the ANSI 16 color palette used by the default theme.
func defaultPalette() ColorPalette {
	return ColorPalette{
		Green:     lipgloss.Color("2"),
		Blue:      lipgloss.Color("4"),
		Red:       lipgloss.Color("1"),
		Yellow:    lipgloss.Color("3"),
		Cyan:      lipgloss.Color("6"),
		Magenta:   lipgloss.Color("5"),
		White:     lipgloss.Color("7"),
		Gray:      lipgloss.Color("8"),
		Orange:    lipgloss.Color("208"),
		None:      lipgloss.Color("-1"),
		Highlight: lipgloss.Color("4"), // same as blue for default theme
	}
}

// defaultAuthorPalette returns the ANSI 256 author colors for the default theme.
func defaultAuthorPalette() []lipgloss.Color {
	return []lipgloss.Color{
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
}

func defaultTheme() *Theme {
	return buildTheme(defaultPalette(), defaultAuthorPalette())
}

// buildTheme constructs a Theme from a color palette and author palette.
func buildTheme(p ColorPalette, authors []lipgloss.Color) *Theme {
	return &Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(p.Green),

		Subtitle: lipgloss.NewStyle().
			Foreground(p.Gray),

		HintBar: lipgloss.NewStyle().
			Foreground(p.Gray),

		SelectedItem: lipgloss.NewStyle().
			Bold(true).
			Background(p.Highlight),

		NormalItem: lipgloss.NewStyle(),

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Green),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.None),

		ErrorText: lipgloss.NewStyle().
			Foreground(p.Red).
			Bold(true),

		SuccessText: lipgloss.NewStyle().
			Foreground(p.Green),

		WarningText: lipgloss.NewStyle().
			Foreground(p.Yellow),

		KeyStyle: lipgloss.NewStyle().
			Foreground(p.Green),

		ValueStyle: lipgloss.NewStyle(),

		PriorityHigh: lipgloss.NewStyle().
			Foreground(p.Red),

		PriorityMedium: lipgloss.NewStyle().
			Foreground(p.Yellow),

		PriorityLow: lipgloss.NewStyle().
			Foreground(p.Green),

		Colors:        p,
		AuthorPalette: authors,
	}
}

// syncColors updates the package-level color variables and the author palette
// to match the current Default theme.
func syncColors() {
	ColorGreen = Default.Colors.Green
	ColorBlue = Default.Colors.Blue
	ColorRed = Default.Colors.Red
	ColorYellow = Default.Colors.Yellow
	ColorCyan = Default.Colors.Cyan
	ColorMagenta = Default.Colors.Magenta
	ColorWhite = Default.Colors.White
	ColorGray = Default.Colors.Gray
	ColorOrange = Default.Colors.Orange
	ColorNone = Default.Colors.None
	ColorHighlight = Default.Colors.Highlight

	authorPalette = Default.AuthorPalette
	authorCache = make(map[string]lipgloss.Style)
}

// SetTheme selects a theme by name and updates the global Default instance
// along with all package-level color variables. Must be called before the
// TUI starts.
//
// Supported names: "default", "catppuccin-latte", "catppuccin-frappe",
// "catppuccin-macchiato", "catppuccin-mocha".
func SetTheme(name string) error {
	switch name {
	case "", "default":
		Default = defaultTheme()
	case "catppuccin-latte":
		Default = catppuccinLatte()
	case "catppuccin-frappe":
		Default = catppuccinFrappe()
	case "catppuccin-macchiato":
		Default = catppuccinMacchiato()
	case "catppuccin-mocha":
		Default = catppuccinMocha()
	default:
		return fmt.Errorf("unknown theme: %q", name)
	}
	syncColors()
	return nil
}

// PriorityStyled applies priority color based on name
func PriorityStyled(name string) string {
	switch strings.ToLower(name) {
	case "highest", "high", "critical", "blocker":
		return Default.PriorityHigh.Render(name)
	case "medium":
		return Default.PriorityMedium.Render(name)
	default:
		return Default.PriorityLow.Render(name)
	}
}

func StatusColor(categoryKey string) lipgloss.Style {
	switch categoryKey {
	case "done":
		return lipgloss.NewStyle().Foreground(ColorGreen)
	case "indeterminate":
		return lipgloss.NewStyle().Foreground(ColorYellow)
	case "new":
		return lipgloss.NewStyle().Foreground(ColorBlue)
	default:
		return lipgloss.NewStyle().Foreground(ColorGray)
	}
}
