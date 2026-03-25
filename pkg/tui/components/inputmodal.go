package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputConfirmedMsg is sent when the user presses Enter in the input modal.
type InputConfirmedMsg struct{ Text string }

// InputCancelledMsg is sent when the user presses Esc.
type InputCancelledMsg struct{}

// InputModal is a single-line text input popup (like lazygit branch rename).
// Optionally shows a list of hints below the input (e.g. existing branches).
// Tab toggles focus between input and hints list.
type InputModal struct {
	title      string
	text       []rune
	cursor     int
	visible    bool
	focusInput bool // true = input focused, false = hints focused
	hints      []string
	hintCursor int
	width      int
	height     int
}

func NewInputModal() InputModal {
	return InputModal{}
}

// Show opens the input modal with a title and pre-filled text.
func (m *InputModal) Show(title, prefill string) {
	m.title = title
	m.text = []rune(prefill)
	m.cursor = len(m.text)
	m.visible = true
	m.focusInput = true
	m.hints = nil
	m.hintCursor = 0
}

// SetHints sets the optional hint items shown below the input.
func (m *InputModal) SetHints(hints []string) {
	m.hints = hints
	m.hintCursor = 0
}

func (m *InputModal) Hide()           { m.visible = false }
func (m *InputModal) IsVisible() bool { return m.visible }
func (m *InputModal) HasHints() bool  { return len(m.hints) > 0 }
func (m *InputModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

//nolint:gocognit // input handling with cursor movement + hints navigation
func (m *InputModal) Update(msg tea.Msg) (InputModal, tea.Cmd) {
	if !m.visible {
		return *m, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		// Tab toggles focus between input and hints.
		if msg.Type == tea.KeyTab && len(m.hints) > 0 {
			m.focusInput = !m.focusInput
			return *m, nil
		}

		if !m.focusInput {
			// Hints list navigation.
			switch msg.Type {
			case tea.KeyEnter:
				if m.hintCursor >= 0 && m.hintCursor < len(m.hints) {
					m.visible = false
					text := m.hints[m.hintCursor]
					return *m, func() tea.Msg { return InputConfirmedMsg{Text: text} }
				}
			case tea.KeyEsc:
				m.visible = false
				return *m, func() tea.Msg { return InputCancelledMsg{} }
			case tea.KeyDown:
				if m.hintCursor < len(m.hints)-1 {
					m.hintCursor++
				}
			case tea.KeyUp:
				if m.hintCursor > 0 {
					m.hintCursor--
				}
			default:
				switch msg.String() {
				case "q":
					m.visible = false
					return *m, func() tea.Msg { return InputCancelledMsg{} }
				case "j":
					if m.hintCursor < len(m.hints)-1 {
						m.hintCursor++
					}
				case "k":
					if m.hintCursor > 0 {
						m.hintCursor--
					}
				}
			}
			return *m, nil
		}

		// Input mode.
		switch msg.Type {
		case tea.KeyEnter:
			m.visible = false
			text := string(m.text)
			return *m, func() tea.Msg { return InputConfirmedMsg{Text: text} }
		case tea.KeyEsc:
			m.visible = false
			return *m, func() tea.Msg { return InputCancelledMsg{} }
		case tea.KeyBackspace:
			if m.cursor > 0 {
				m.text = append(m.text[:m.cursor-1], m.text[m.cursor:]...)
				m.cursor--
			}
		case tea.KeyDelete:
			if m.cursor < len(m.text) {
				m.text = append(m.text[:m.cursor], m.text[m.cursor+1:]...)
			}
		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyRight:
			if m.cursor < len(m.text) {
				m.cursor++
			}
		case tea.KeyHome, tea.KeyCtrlA:
			m.cursor = 0
		case tea.KeyEnd, tea.KeyCtrlE:
			m.cursor = len(m.text)
		case tea.KeyCtrlU:
			m.text = m.text[m.cursor:]
			m.cursor = 0
		case tea.KeyCtrlK:
			m.text = m.text[:m.cursor]
		case tea.KeyRunes:
			runes := msg.Runes
			newText := make([]rune, 0, len(m.text)+len(runes))
			newText = append(newText, m.text[:m.cursor]...)
			newText = append(newText, runes...)
			newText = append(newText, m.text[m.cursor:]...)
			m.text = newText
			m.cursor += len(runes)
		default:
			// Ignore other keys.
		}
	}
	return *m, nil
}

func (m *InputModal) View() string {
	if !m.visible {
		return ""
	}

	contentW := min(max(m.width*6/10, 30), m.width-4)
	innerW := contentW - 2 // borders

	// Render text with cursor.
	textStr := string(m.text)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	var rendered string
	if m.cursor >= len(m.text) {
		rendered = " " + textStr + cursorStyle.Render("█")
	} else {
		before := string(m.text[:m.cursor])
		at := string(m.text[m.cursor : m.cursor+1])
		after := string(m.text[m.cursor+1:])
		rendered = " " + before + cursorStyle.Render(at) + after
	}

	// Truncate if too long — show window around cursor.
	if lipgloss.Width(rendered) > innerW {
		// Simple approach: show from cursor-leftward.
		visible := " " + textStr + " "
		rendered = TruncateEnd(visible, innerW)
	}

	// Pad to fill width.
	lineW := lipgloss.Width(rendered)
	if lineW < innerW {
		rendered += strings.Repeat(" ", innerW-lineW)
	}

	return RenderPanelFull(m.title, "", rendered, contentW, 1, m.focusInput, nil)
}

// Intercept handles a message if the modal is visible. Implements Overlay.
func (m *InputModal) Intercept(msg tea.Msg) (tea.Cmd, bool) {
	if !m.visible {
		return nil, false
	}
	if _, ok := msg.(tea.KeyMsg); ok {
		updated, cmd := m.Update(msg)
		*m = updated
		return cmd, true
	}
	return nil, false
}

// Render draws the input modal centered on bg with optional hint panel. Implements Overlay.
func (m *InputModal) Render(bg string, w, h int) string {
	if !m.visible {
		return bg
	}
	popup := m.View()
	return centerOverlayWithHint(bg, popup, m.HintView(), w, h)
}

// HintView returns a separate bordered panel with hint items (existing branches).
// Returns "" if no hints are set.
func (m *InputModal) HintView() string {
	if !m.visible || len(m.hints) == 0 {
		return ""
	}

	contentW := min(max(m.width*6/10, 30), m.width-4)
	innerW := contentW - 2 // borders

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	normalStyle := lipgloss.NewStyle()

	maxHints := min(5, len(m.hints))
	start := 0
	if m.hintCursor >= maxHints {
		start = m.hintCursor - maxHints + 1
	}
	end := start + maxHints
	if end > len(m.hints) {
		end = len(m.hints)
		start = max(0, end-maxHints)
	}

	var lines []string
	for i := start; i < end; i++ {
		line := " " + m.hints[i]
		line = TruncateEnd(line, innerW)
		if lineW := lipgloss.Width(line); lineW < innerW {
			line += strings.Repeat(" ", innerW-lineW)
		}
		if i == m.hintCursor && !m.focusInput {
			lines = append(lines, selStyle.Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	body := strings.Join(lines, "\n")
	title := "Existing branches"
	footer := fmt.Sprintf("%d of %d", m.hintCursor+1, len(m.hints))
	return RenderPanelFull(title, footer, body, contentW, len(lines), !m.focusInput, nil)
}
