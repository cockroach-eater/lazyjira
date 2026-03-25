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
	items     []HelpItem
	statusMsg string // transient message shown left of hints (green)
	width     int
}

// NewHelpBar creates a help bar with the given items.
func NewHelpBar(items []HelpItem) HelpBar {
	return HelpBar{items: items}
}

// SetItems replaces the current help items.
func (h *HelpBar) SetItems(items []HelpItem) {
	h.items = items
}

// SetStatusMsg sets a transient message shown left of hints.
func (h *HelpBar) SetStatusMsg(msg string) {
	h.statusMsg = msg
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
	blueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	sep := blueStyle.Render(" | ")

	availW := h.width

	// Status message (green) on the left.
	prefix := ""
	if h.statusMsg != "" {
		greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
		prefix = " " + greenStyle.Render(h.statusMsg) + " "
		availW -= lipgloss.Width(prefix)
	}

	var parts []string
	totalWidth := 0
	truncated := false
	for _, item := range h.items {
		part := blueStyle.Render(item.Description + ": " + item.Key)
		partWidth := lipgloss.Width(part) + 3 // " | "
		if availW > 0 && totalWidth+partWidth > availW {
			truncated = true
			break
		}
		parts = append(parts, part)
		totalWidth += partWidth
	}

	result := strings.Join(parts, sep)
	if truncated {
		result += blueStyle.Render(" ...")
	}
	if prefix != "" {
		return prefix + result
	}
	return " " + result
}
