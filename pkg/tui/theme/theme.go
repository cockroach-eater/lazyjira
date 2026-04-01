package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Lazygit-style standard ANSI 16 palette colors
const (
	ColorGreen  = lipgloss.Color("2")  // ANSI green — active borders, accents
	ColorBlue   = lipgloss.Color("4")  // ANSI blue — help bar, selected bg
	ColorRed    = lipgloss.Color("1")  // ANSI red — errors, unstaged
	ColorYellow = lipgloss.Color("3")  // ANSI yellow — warnings, in-progress
	ColorCyan   = lipgloss.Color("6")  // ANSI cyan — search mode
	ColorWhite  = lipgloss.Color("7")  // ANSI white (light gray)
	ColorGray   = lipgloss.Color("8")   // ANSI bright black (dark gray)
	ColorOrange = lipgloss.Color("208") // ANSI 256 orange — secondary accent (names, metadata)
	ColorNone   = lipgloss.Color("-1")  // default terminal color
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
}

// Default is the singleton theme instance.
var Default = defaultTheme()

// DefaultTheme returns the singleton theme. Kept for compatibility.
func DefaultTheme() *Theme { return Default }

func defaultTheme() *Theme {
	return &Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorGreen),

		Subtitle: lipgloss.NewStyle().
			Foreground(ColorGray),

		HintBar: lipgloss.NewStyle().
			Foreground(ColorGray),

		SelectedItem: lipgloss.NewStyle().
			Bold(true).
			Background(ColorBlue),

		NormalItem: lipgloss.NewStyle(),

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen),

		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorNone),

		ErrorText: lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true),

		SuccessText: lipgloss.NewStyle().
			Foreground(ColorGreen),

		WarningText: lipgloss.NewStyle().
			Foreground(ColorYellow),

		KeyStyle: lipgloss.NewStyle().
			Foreground(ColorGreen),

		ValueStyle: lipgloss.NewStyle(),

		PriorityHigh: lipgloss.NewStyle().
			Foreground(ColorRed),

		PriorityMedium: lipgloss.NewStyle().
			Foreground(ColorYellow),

		PriorityLow: lipgloss.NewStyle().
			Foreground(ColorGreen),
	}
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
