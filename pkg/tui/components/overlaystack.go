package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OverlayPanel is a UI element that renders on top of the main content
// and intercepts input when visible (modals, diff views, etc).
type OverlayPanel interface {
	IsVisible() bool
	SetSize(w, h int)
	// Intercept handles a message if this overlay is active.
	// Returns the command and true if the message was consumed.
	Intercept(msg tea.Msg) (tea.Cmd, bool)
	// Render draws this overlay on top of bg.
	Render(bg string, w, h int) string
}

// OverlayStack is an ordered list of overlays checked in priority order.
type OverlayStack []OverlayPanel

// Intercept routes a message to the first visible overlay.
func (s OverlayStack) Intercept(msg tea.Msg) (tea.Cmd, bool) {
	for _, o := range s {
		if cmd, ok := o.Intercept(msg); ok {
			return cmd, true
		}
	}
	return nil, false
}

// Render draws the first visible overlay on top of bg.
func (s OverlayStack) Render(bg string, w, h int) string {
	for _, o := range s {
		if o.IsVisible() {
			return o.Render(bg, w, h)
		}
	}
	return bg
}

// SetSize propagates terminal size to all overlays.
func (s OverlayStack) SetSize(w, h int) {
	for _, o := range s {
		o.SetSize(w, h)
	}
}

// centerOverlay places popup centered on bg.
func centerOverlay(bg, popup string, w, h int) string {
	popupW := lipgloss.Width(popup)
	popupH := len(strings.Split(popup, "\n"))
	x := (w - popupW) / 2
	y := (h - popupH) / 2
	return OverlayAt(bg, popup, x, y, w, h)
}

// centerOverlayWithHint places popup centered on bg, with hint below it.
func centerOverlayWithHint(bg, popup, hint string, w, h int) string {
	popupW := lipgloss.Width(popup)
	popupH := len(strings.Split(popup, "\n"))
	x := (w - popupW) / 2
	y := (h - popupH) / 2
	result := OverlayAt(bg, popup, x, y, w, h)
	if hint != "" {
		result = OverlayAt(result, hint, x, y+popupH, w, h)
	}
	return result
}
