package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpItem represents a single keybinding hint.
type HelpItem struct {
	Key         string
	Description string
}

// HelpBar displays context-sensitive keybinding hints at the bottom of the screen.
type HelpBar struct {
	items []HelpItem
	width int
}

// NewHelpBar creates a help bar with the given items.
func NewHelpBar(items []HelpItem) HelpBar {
	return HelpBar{items: items}
}

// SetItems replaces the current help items.
func (h *HelpBar) SetItems(items []HelpItem) {
	h.items = items
}

// SetWidth updates the help bar width.
func (h *HelpBar) SetWidth(w int) {
	h.width = w
}

func (h HelpBar) Init() tea.Cmd {
	return nil
}

func (h HelpBar) Update(msg tea.Msg) (HelpBar, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		h.width = msg.Width
	}
	return h, nil
}

func (h HelpBar) View() string {
	// All in the same blue like lazygit options bar.
	blueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	sepStyle := blueStyle

	sep := sepStyle.Render(" | ")

	var parts []string
	totalWidth := 0
	for _, item := range h.items {
		part := blueStyle.Render(item.Description+": "+item.Key)
		partWidth := lipgloss.Width(part) + 3 // " | "
		if h.width > 0 && totalWidth+partWidth > h.width {
			break
		}
		parts = append(parts, part)
		totalWidth += partWidth
	}

	return " " + strings.Join(parts, sep)
}
