package theme

import (
	"github.com/charmbracelet/lipgloss"

	catppuccin "github.com/catppuccin/go"
)

// fromCatppuccin builds a Theme from any Catppuccin flavor.
// Surface2 is used as the selection background — a muted color
// that's readable across all four flavors (including Latte).
func fromCatppuccin(f catppuccin.Flavor) *Theme {
	hex := func(c catppuccin.Color) lipgloss.Color { return lipgloss.Color(c.Hex) }
	return buildTheme(
		ColorPalette{
			Green:     hex(f.Green()),
			Blue:      hex(f.Blue()),
			Red:       hex(f.Red()),
			Yellow:    hex(f.Yellow()),
			Cyan:      hex(f.Teal()),
			Magenta:   hex(f.Mauve()),
			White:     hex(f.Text()),
			Gray:      hex(f.Overlay0()),
			Orange:    hex(f.Peach()),
			None:      lipgloss.Color("-1"),
			Highlight: hex(f.Surface2()),
		},
		[]lipgloss.Color{
			hex(f.Rosewater()),
			hex(f.Flamingo()),
			hex(f.Pink()),
			hex(f.Mauve()),
			hex(f.Red()),
			hex(f.Maroon()),
			hex(f.Peach()),
			hex(f.Yellow()),
			hex(f.Green()),
			hex(f.Teal()),
			hex(f.Sapphire()),
			hex(f.Lavender()),
		},
	)
}

func catppuccinLatte() *Theme     { return fromCatppuccin(catppuccin.Latte) }
func catppuccinFrappe() *Theme    { return fromCatppuccin(catppuccin.Frappe) }
func catppuccinMacchiato() *Theme { return fromCatppuccin(catppuccin.Macchiato) }
func catppuccinMocha() *Theme     { return fromCatppuccin(catppuccin.Mocha) }
