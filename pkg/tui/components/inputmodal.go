package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputConfirmedMsg is sent when the user presses Enter in the input modal.
type InputConfirmedMsg struct{ Text string }

// InputCancelledMsg is sent when the user presses Esc.
type InputCancelledMsg struct{}

// InputModal is a single-line text input popup (like lazygit branch rename).
type InputModal struct {
	title   string
	text    []rune
	cursor  int
	visible bool
	width   int
	height  int
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
}

func (m *InputModal) Hide()           { m.visible = false }
func (m *InputModal) IsVisible() bool { return m.visible }
func (m *InputModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

//nolint:gocognit // input handling with cursor movement
func (m *InputModal) Update(msg tea.Msg) (InputModal, tea.Cmd) {
	if !m.visible {
		return *m, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
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

	return RenderPanelFull(m.title, "", rendered, contentW, 1, true, nil)
}
