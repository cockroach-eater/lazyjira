package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalItem is one option in the modal.
type ModalItem struct {
	ID        string
	Label     string
	Hint      string // shown below the list when this item is selected
	Internal  bool   // true = handled in-app (e.g. Jira issue), styled differently
	Separator bool   // true = non-selectable section header
}

// ModalSelectedMsg is sent when user picks an item.
type ModalSelectedMsg struct {
	Item ModalItem
}

// ModalCancelledMsg is sent when user presses Esc.
type ModalCancelledMsg struct{}

// Modal is a centered popup list for picking an option (transitions, etc).
type Modal struct {
	title   string
	items   []ModalItem
	cursor  int
	visible  bool
	readOnly bool // scroll-only, no selection highlight
	offset   int
	width    int
	height   int
}

func NewModal() Modal {
	return Modal{}
}

func (m *Modal) show(title string, items []ModalItem, readOnly bool) {
	m.title = title
	m.items = items
	m.cursor = 0
	m.offset = 0
	m.visible = true
	m.readOnly = readOnly
	// Skip initial separator.
	if !readOnly && m.cursor < len(m.items) && m.items[m.cursor].Separator {
		m.moveCursor(1)
	}
}

func (m *Modal) Show(title string, items []ModalItem)         { m.show(title, items, false) }
func (m *Modal) ShowReadOnly(title string, items []ModalItem) { m.show(title, items, true) }

// moveCursor advances cursor by delta, skipping separator items.
func (m *Modal) moveCursor(delta int) {
	for {
		next := m.cursor + delta
		if next < 0 || next >= len(m.items) {
			return
		}
		m.cursor = next
		if !m.items[m.cursor].Separator {
			return
		}
	}
}

// selectionContentW returns the content width for selection-mode modals.
func (m *Modal) selectionContentW() int {
	contentW := lipgloss.Width(m.title) + 2
	for _, item := range m.items {
		if w := lipgloss.Width(item.Label) + 2; w > contentW {
			contentW = w
		}
	}
	return min(contentW, min(55, m.width-6))
}

func (m *Modal) Hide()          { m.visible = false }
func (m *Modal) IsVisible() bool { return m.visible }

func (m *Modal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Modal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	if !m.visible {
		return *m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.readOnly {
				m.offset++
			} else {
				m.moveCursor(1)
			}
		case "k", "up":
			if m.readOnly {
				if m.offset > 0 {
					m.offset--
				}
			} else {
				m.moveCursor(-1)
			}
		case "enter", " ":
			if m.readOnly {
				m.visible = false
				return *m, func() tea.Msg { return ModalCancelledMsg{} }
			}
			if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].Separator {
				selected := m.items[m.cursor]
				m.visible = false
				return *m, func() tea.Msg { return ModalSelectedMsg{Item: selected} }
			}
		case "esc", "q", "h":
			m.visible = false
			return *m, func() tea.Msg { return ModalCancelledMsg{} }
		}
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return *m, nil
}

func (m *Modal) handleMouse(msg tea.MouseMsg) (Modal, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseButtonWheelDown:
		if m.readOnly {
			m.offset++
		} else {
			m.moveCursor(1)
		}
	case msg.Button == tea.MouseButtonWheelUp:
		if m.readOnly {
			if m.offset > 0 {
				m.offset--
			}
		} else {
			m.moveCursor(-1)
		}
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		if !m.readOnly {
			clickY := msg.Y
			idx := clickY - 3 // rough: border + title + blank
			if m.height > 0 {
				mainBoxH := min(len(m.items)+4, m.height-2) + 2 // content + borders
				topOffset := (m.height - mainBoxH) / 2
				idx = clickY - topOffset - 3
			}
			if idx >= 0 && idx < len(m.items) && !m.items[idx].Separator {
				m.cursor = idx
				selected := m.items[m.cursor]
				m.visible = false
				return *m, func() tea.Msg { return ModalSelectedMsg{Item: selected} }
			}
		}
	}
	return *m, nil
}

func (m *Modal) View() string {
	if !m.visible || len(m.items) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")).
		Bold(true)

	if m.readOnly {
		// Auto-size width: fit content, max 70% of available width.
		maxW := m.width * 7 / 10
		if maxW < 40 {
			maxW = min(m.width-4, 40)
		}
		contentW := lipgloss.Width(m.title) + 2
		for _, item := range m.items {
			if w := lipgloss.Width(item.Label) + 3; w > contentW {
				contentW = w
			}
		}
		if contentW > maxW {
			contentW = maxW
		}

		// Collect item lines, word-wrapped to fit (preserves ANSI colors).
		innerW := contentW - 3 // border (2) + leading space (1)
		wrapStyle := lipgloss.NewStyle().Width(innerW)
		var lines []string
		for _, item := range m.items {
			wrapped := wrapStyle.Render(item.Label)
			for _, w := range strings.Split(wrapped, "\n") {
				lines = append(lines, " "+w)
			}
		}

		totalLines := len(lines)
		visibleH := min(max(m.height-4, 3), totalLines)
		if m.offset > totalLines-visibleH {
			m.offset = totalLines - visibleH
		}
		if m.offset < 0 {
			m.offset = 0
		}
		scrolled := lines
		if m.offset < len(scrolled) {
			scrolled = scrolled[m.offset:]
		}
		if len(scrolled) > visibleH {
			scrolled = scrolled[:visibleH]
		}
		content := strings.Join(scrolled, "\n")
		return RenderPanelFull(m.title, "", content, contentW, visibleH, true,
			&ScrollInfo{Total: totalLines, Visible: visibleH, Offset: m.offset})
	}

	contentW := m.selectionContentW()

	// Normal modal (selection): title + blank + items.
	var lines []string
	lines = append(lines, " "+titleStyle.Render(m.title))
	lines = append(lines, "")

	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i, item := range m.items {
		if item.Separator {
			// Centered gray header: "── Label ──"
			label := TruncateEnd(item.Label, contentW-4)
			pad := contentW - lipgloss.Width(label) - 4
			left := max(pad/2, 0)
			right := max(pad-left, 0)
			lines = append(lines, sepStyle.Render(strings.Repeat("─", left)+" "+label+" "+strings.Repeat("─", right)))
			continue
		}
		label := " " + TruncateMiddle(item.Label, contentW-3)
		style := lipgloss.NewStyle().Width(contentW)
		switch {
		case i == m.cursor:
			style = style.Bold(true).Background(lipgloss.Color("4"))
		case item.Internal:
			style = style.Foreground(lipgloss.Color("2"))
		}
		lines = append(lines, style.Render(label))
	}

	popupH := len(lines)
	maxH := max(m.height-2, 5)
	if popupH > maxH {
		popupH = maxH
	}

	// Count selectable items and cursor position among them.
	total, pos := 0, 0
	for i, item := range m.items {
		if !item.Separator {
			total++
			if i == m.cursor {
				pos = total
			}
		}
	}
	footer := fmt.Sprintf("%d of %d", pos, total)

	// Pad lines to popupH.
	for len(lines) < popupH {
		lines = append(lines, "")
	}
	if len(lines) > popupH {
		lines = lines[:popupH]
	}

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	bv := borderStyle.Render("│")

	// Top border.
	topLine := borderStyle.Render("╭" + strings.Repeat("─", contentW) + "╮")

	// Content lines with side borders.
	var body strings.Builder
	body.WriteString(topLine + "\n")
	for _, line := range lines {
		lineW := lipgloss.Width(line)
		if lineW < contentW {
			line += strings.Repeat(" ", contentW-lineW)
		}
		body.WriteString(bv + line + bv + "\n")
	}

	// Bottom border with footer counter.
	footerStyled := borderStyle.Render(footer)
	footerLen := lipgloss.Width(footerStyled)
	pad := max(contentW-footerLen, 0)
	bottomLine := borderStyle.Render("╰"+strings.Repeat("─", pad)) +
		footerStyled +
		borderStyle.Render("╯")
	body.WriteString(bottomLine)

	return body.String()
}

// HintView returns the hint box for the currently selected item, or "" if none.
func (m *Modal) HintView() string {
	if !m.visible || m.readOnly {
		return ""
	}
	hint := ""
	if m.cursor >= 0 && m.cursor < len(m.items) {
		hint = m.items[m.cursor].Hint
	}
	if hint == "" {
		return ""
	}

	contentW := m.selectionContentW()
	const hintH = 2
	hintContent := lipgloss.NewStyle().Width(contentW).Render(" " + hint)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("7")).
		Width(contentW).
		Height(hintH).
		Render(hintContent)
}
