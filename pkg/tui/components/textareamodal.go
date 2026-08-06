package components

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

// TextAreaConfirmedMsg carries the text the user submitted.
type TextAreaConfirmedMsg struct{ Text string }

// TextAreaCancelledMsg is sent when the user dismisses the popup.
type TextAreaCancelledMsg struct{}

// TextAreaHandoffMsg asks the app to reopen the current text in $EDITOR.
type TextAreaHandoffMsg struct{ Text string }

const (
	textAreaMinLines = 5
	textAreaMaxLines = 14
)

// TextAreaModal is a multi-line text popup.
//
// Enter inserts a newline, so submitting needs its own key: ctrl+d (and
// ctrl+s) send, esc cancels, and ctrl+e hands the buffer to $EDITOR for
// anything longer than a couple of paragraphs.
type TextAreaModal struct {
	title         string
	lines         []string
	row, col      int // cursor, in runes
	offset        int // first visible line
	visible       bool
	width, height int
}

func NewTextAreaModal() TextAreaModal {
	return TextAreaModal{}
}

// Show opens the popup with prefill as its initial content, cursor at the end.
func (m *TextAreaModal) Show(title, prefill string) {
	m.title = title
	m.lines = strings.Split(prefill, "\n")
	m.row = len(m.lines) - 1
	m.col = len([]rune(m.lines[m.row]))
	m.offset = 0
	m.visible = true
}

func (m *TextAreaModal) Hide()           { m.visible = false }
func (m *TextAreaModal) IsVisible() bool { return m.visible }
func (m *TextAreaModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Text returns the current buffer.
func (m *TextAreaModal) Text() string { return strings.Join(m.lines, "\n") }

func (m *TextAreaModal) Update(msg tea.Msg) (TextAreaModal, tea.Cmd) {
	if !m.visible {
		return *m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return *m, nil
	}

	switch key.Type {
	case tea.KeyCtrlD, tea.KeyCtrlS:
		m.visible = false
		text := m.Text()
		return *m, func() tea.Msg { return TextAreaConfirmedMsg{Text: text} }
	case tea.KeyEsc:
		m.visible = false
		return *m, func() tea.Msg { return TextAreaCancelledMsg{} }
	case tea.KeyCtrlE:
		m.visible = false
		text := m.Text()
		return *m, func() tea.Msg { return TextAreaHandoffMsg{Text: text} }
	case tea.KeyEnter:
		m.splitLine()
	case tea.KeyBackspace:
		m.backspace()
	case tea.KeyDelete:
		m.deleteForward()
	case tea.KeyLeft:
		m.moveLeft()
	case tea.KeyRight:
		m.moveRight()
	case tea.KeyUp:
		m.moveVertical(-1)
	case tea.KeyDown:
		m.moveVertical(1)
	case tea.KeyHome, tea.KeyCtrlA:
		m.col = 0
	case tea.KeyEnd:
		m.col = m.lineLen(m.row)
	case tea.KeyCtrlU:
		m.lines[m.row] = string([]rune(m.lines[m.row])[m.col:])
		m.col = 0
	case tea.KeyCtrlK:
		m.lines[m.row] = string([]rune(m.lines[m.row])[:m.col])
	case tea.KeySpace:
		m.insert(" ")
	case tea.KeyRunes:
		m.insert(string(key.Runes))
	default:
	}
	return *m, nil
}

func (m *TextAreaModal) lineLen(row int) int { return len([]rune(m.lines[row])) }

func (m *TextAreaModal) insert(s string) {
	runes := []rune(m.lines[m.row])
	m.lines[m.row] = string(runes[:m.col]) + s + string(runes[m.col:])
	m.col += len([]rune(s))
}

func (m *TextAreaModal) splitLine() {
	runes := []rune(m.lines[m.row])
	head, tail := string(runes[:m.col]), string(runes[m.col:])
	m.lines = append(m.lines[:m.row], append([]string{head, tail}, m.lines[m.row+1:]...)...)
	m.row++
	m.col = 0
}

func (m *TextAreaModal) backspace() {
	if m.col > 0 {
		runes := []rune(m.lines[m.row])
		m.lines[m.row] = string(runes[:m.col-1]) + string(runes[m.col:])
		m.col--
		return
	}
	if m.row == 0 {
		return
	}
	// Joining with the line above puts the cursor at the seam.
	m.col = m.lineLen(m.row - 1)
	m.lines[m.row-1] += m.lines[m.row]
	m.lines = append(m.lines[:m.row], m.lines[m.row+1:]...)
	m.row--
}

func (m *TextAreaModal) deleteForward() {
	runes := []rune(m.lines[m.row])
	if m.col < len(runes) {
		m.lines[m.row] = string(runes[:m.col]) + string(runes[m.col+1:])
		return
	}
	if m.row < len(m.lines)-1 {
		m.lines[m.row] += m.lines[m.row+1]
		m.lines = append(m.lines[:m.row+1], m.lines[m.row+2:]...)
	}
}

func (m *TextAreaModal) moveLeft() {
	switch {
	case m.col > 0:
		m.col--
	case m.row > 0:
		m.row--
		m.col = m.lineLen(m.row)
	}
}

func (m *TextAreaModal) moveRight() {
	switch {
	case m.col < m.lineLen(m.row):
		m.col++
	case m.row < len(m.lines)-1:
		m.row++
		m.col = 0
	}
}

func (m *TextAreaModal) moveVertical(delta int) {
	target := m.row + delta
	if target < 0 || target >= len(m.lines) {
		return
	}
	m.row = target
	m.col = min(m.col, m.lineLen(m.row))
}

// visibleLines is how many text rows the popup shows at the current height.
func (m *TextAreaModal) visibleLines() int {
	budget := textAreaMaxLines
	if m.height > 0 {
		budget = min(budget, max(m.height-6, textAreaMinLines))
	}
	return max(min(budget, max(len(m.lines), textAreaMinLines)), 1)
}

func (m *TextAreaModal) View() string {
	if !m.visible {
		return ""
	}

	contentW := min(max(m.width*7/10, 40), max(m.width-4, 20))
	innerW := contentW - 2
	visible := m.visibleLines()

	// Keep the cursor line on screen.
	if m.row < m.offset {
		m.offset = m.row
	} else if m.row >= m.offset+visible {
		m.offset = m.row - visible + 1
	}
	m.offset = min(max(m.offset, 0), max(len(m.lines)-visible, 0))

	cursorStyle := lipgloss.NewStyle().Foreground(theme.ColorCyan)
	rendered := make([]string, 0, visible)
	for i := m.offset; i < m.offset+visible; i++ {
		if i >= len(m.lines) {
			rendered = append(rendered, strings.Repeat(" ", innerW))
			continue
		}
		line := m.lines[i]
		if i == m.row {
			runes := []rune(line)
			switch {
			case m.col >= len(runes):
				line = string(runes) + cursorStyle.Render("█")
			default:
				line = string(runes[:m.col]) + cursorStyle.Render(string(runes[m.col])) + string(runes[m.col+1:])
			}
		}
		line = TruncateEnd(line, innerW)
		if w := lipgloss.Width(line); w < innerW {
			line += strings.Repeat(" ", innerW-w)
		}
		rendered = append(rendered, line)
	}

	footer := "ctrl+d send · ctrl+e editor · esc cancel"
	if len(m.lines) > visible {
		footer = strconv.Itoa(m.row+1) + "/" + strconv.Itoa(len(m.lines)) + " · " + footer
	}
	return RenderPanelFull(m.title, footer, strings.Join(rendered, "\n"), contentW, visible, true, nil)
}

func (m *TextAreaModal) Intercept(msg tea.Msg) (tea.Cmd, bool) {
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

func (m *TextAreaModal) Render(bg string, w, h int) string {
	if !m.visible {
		return bg
	}
	return centerOverlay(bg, m.View(), w, h)
}
