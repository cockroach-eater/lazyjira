package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

// ModalItem is one option in the modal.
type ModalItem struct {
	ID        string
	Label     string
	Hint      string // shown below the list when this item is selected
	Internal  bool   // true = handled in-app (e.g. Jira issue), styled differently
	Separator bool   // true = non-selectable section header
	Active    bool   // true = current value, shown with green * marker
}

// ModalSelectedMsg is sent when user picks an item.
type ModalSelectedMsg struct {
	Item ModalItem
}

// ModalCancelledMsg is sent when user presses Esc.
type ModalCancelledMsg struct{}

// ChecklistConfirmedMsg is sent when user confirms a checklist selection.
type ChecklistConfirmedMsg struct {
	Selected []ModalItem
}

// Modal is a centered popup list for picking an option (transitions, etc).
type Modal struct {
	title     string
	items     []ModalItem    // currently displayed (filtered) items
	allItems  []ModalItem    // full unfiltered list
	cursor    int
	visible   bool
	readOnly  bool // scroll-only, no selection highlight
	checklist bool // multi-select checklist mode
	selected  map[string]bool // checklist: currently selected item IDs
	offset    int
	width     int
	height    int
	filter    string // current search query
	searching bool   // whether search input is active
	isError   bool   // red border for error display
}

func NewModal() Modal {
	return Modal{}
}

func (m *Modal) show(title string, items []ModalItem, readOnly bool) {
	m.title = title
	m.allItems = items
	m.items = items
	m.cursor = 0
	m.offset = 0
	m.visible = true
	m.readOnly = readOnly
	m.checklist = false
	m.selected = nil
	m.filter = ""
	m.searching = false
	m.isError = false
	// Skip initial separator.
	if !readOnly && m.cursor < len(m.items) && m.items[m.cursor].Separator {
		m.moveCursor(1)
	}
}

func (m *Modal) Show(title string, items []ModalItem)         { m.show(title, items, false) }
func (m *Modal) ShowReadOnly(title string, items []ModalItem) { m.show(title, items, true) }

// ShowError opens a read-only modal with red border.
func (m *Modal) ShowError(title string, items []ModalItem) {
	m.show(title, items, true)
	m.isError = true
}

// ShowChecklist opens a multi-select checklist modal.
func (m *Modal) ShowChecklist(title string, items []ModalItem, selected map[string]bool) {
	sel := make(map[string]bool, len(selected))
	for k, v := range selected {
		if v {
			sel[k] = true
		}
	}
	m.show(title, items, false)
	m.checklist = true
	m.selected = sel
	m.sortChecklist()
}

// sortChecklist sorts items: selected first, then unselected, preserving relative order.
// Separator items are excluded. Cursor follows the previously focused item.
func (m *Modal) sortChecklist() {
	var cursorID, cursorLabel string
	if m.cursor >= 0 && m.cursor < len(m.items) {
		cursorID = m.items[m.cursor].ID
		cursorLabel = m.items[m.cursor].Label
	}

	var selItems, unselItems []ModalItem
	for _, item := range m.items {
		if item.Separator {
			continue
		}
		if m.selected[item.ID] {
			selItems = append(selItems, item)
		} else {
			unselItems = append(unselItems, item)
		}
	}
	sorted := make([]ModalItem, 0, len(selItems)+len(unselItems))
	sorted = append(sorted, selItems...)
	sorted = append(sorted, unselItems...)
	m.items = sorted

	// Also sort allItems for filter restore.
	var allSel, allUnsel []ModalItem
	for _, item := range m.allItems {
		if item.Separator {
			continue
		}
		if m.selected[item.ID] {
			allSel = append(allSel, item)
		} else {
			allUnsel = append(allUnsel, item)
		}
	}
	allSorted := make([]ModalItem, 0, len(allSel)+len(allUnsel))
	allSorted = append(allSorted, allSel...)
	allSorted = append(allSorted, allUnsel...)
	m.allItems = allSorted

	// Restore cursor position.
	for i, item := range m.items {
		if item.ID == cursorID && item.Label == cursorLabel {
			m.cursor = i
			return
		}
	}
	if m.cursor >= len(m.items) {
		m.cursor = max(len(m.items)-1, 0)
	}
}

// moveCursor advances cursor by delta, skipping separator items.
// Wraps around: moving past the end goes to the first item, past the start goes to the last.
func (m *Modal) moveCursor(delta int) {
	n := len(m.items)
	if n == 0 {
		return
	}
	for {
		next := m.cursor + delta
		if next < 0 {
			next = n - 1
		} else if next >= n {
			next = 0
		}
		if next == m.cursor {
			return // full loop, all separators
		}
		m.cursor = next
		if !m.items[m.cursor].Separator {
			return
		}
	}
}

// applyFilter filters allItems by the current search query.
func (m *Modal) applyFilter() {
	if m.filter == "" {
		m.items = m.allItems
	} else {
		q := strings.ToLower(m.filter)
		var filtered []ModalItem
		for _, item := range m.allItems {
			if item.Separator {
				continue
			}
			if strings.Contains(strings.ToLower(item.Label), q) {
				filtered = append(filtered, item)
			}
		}
		m.items = filtered
	}
	m.cursor = 0
	m.offset = 0
	// Skip separator at cursor.
	if m.cursor < len(m.items) && m.items[m.cursor].Separator {
		m.moveCursor(1)
	}
}

// confirmSearch restores full item list and places cursor on the matched item.
func (m *Modal) confirmSearch() {
	var matchedID, matchedLabel string
	if m.cursor >= 0 && m.cursor < len(m.items) {
		matchedID = m.items[m.cursor].ID
		matchedLabel = m.items[m.cursor].Label
	}
	m.searching = false
	m.filter = ""
	m.items = m.allItems
	m.cursor = 0
	for i, item := range m.items {
		if item.ID == matchedID && item.Label == matchedLabel {
			m.cursor = i
			break
		}
	}
	m.offset = 0
}

// selectionContentW returns the content width for selection-mode modals.
func (m *Modal) selectionContentW() int {
	contentW := lipgloss.Width(m.title) + 4
	for _, item := range m.allItems {
		if w := lipgloss.Width(item.Label) + 4; w > contentW {
			contentW = w
		}
	}
	maxW := max(m.width*7/10, 30)
	return min(contentW, maxW)
}

func (m *Modal) Hide()           { m.visible = false }
func (m *Modal) IsVisible() bool    { return m.visible }
func (m *Modal) IsSearching() bool  { return m.searching }
func (m *Modal) IsChecklist() bool  { return m.checklist }

// SearchView renders the modal search bar for external use (e.g. bottom help bar).
func (m *Modal) SearchView(width int) string {
	searchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	line := "/ " + m.filter
	if m.searching {
		line += "_"
	}
	return searchStyle.Width(width).Render(line)
}

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
		if m.searching {
			return m.handleSearchKey(msg)
		}
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return *m, nil
}

func (m *Modal) handleSearchKey(msg tea.KeyMsg) (Modal, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.confirmSearch()
	case "esc":
		m.searching = false
		m.filter = ""
		m.applyFilter()
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.applyFilter()
		}
	default:
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			m.filter += ch
			m.applyFilter()
		}
	}
	return *m, nil
}

func (m *Modal) handleKey(msg tea.KeyMsg) (Modal, tea.Cmd) {
	switch msg.String() {
	case "/":
		if !m.readOnly {
			m.searching = true
			m.filter = ""
			return *m, nil
		}
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
	case " ":
		return m.handleSpace()
	case "enter":
		return m.handleEnter()
	case "esc", "q", "h":
		m.visible = false
		return *m, func() tea.Msg { return ModalCancelledMsg{} }
	}
	return *m, nil
}

func (m *Modal) handleSpace() (Modal, tea.Cmd) {
	if m.readOnly {
		m.visible = false
		return *m, func() tea.Msg { return ModalCancelledMsg{} }
	}
	if m.checklist {
		if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].Separator {
			id := m.items[m.cursor].ID
			if m.selected[id] {
				delete(m.selected, id)
			} else {
				m.selected[id] = true
			}
			m.sortChecklist()
		}
		return *m, nil
	}
	if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].Separator {
		selected := m.items[m.cursor]
		m.visible = false
		return *m, func() tea.Msg { return ModalSelectedMsg{Item: selected} }
	}
	return *m, nil
}

func (m *Modal) handleEnter() (Modal, tea.Cmd) {
	if m.readOnly {
		m.visible = false
		return *m, func() tea.Msg { return ModalCancelledMsg{} }
	}
	if m.checklist {
		var result []ModalItem
		for _, item := range m.allItems {
			if !item.Separator && m.selected[item.ID] {
				result = append(result, item)
			}
		}
		m.visible = false
		return *m, func() tea.Msg { return ChecklistConfirmedMsg{Selected: result} }
	}
	if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].Separator {
		selected := m.items[m.cursor]
		m.visible = false
		return *m, func() tea.Msg { return ModalSelectedMsg{Item: selected} }
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
				if m.checklist {
					id := m.items[m.cursor].ID
					if m.selected[id] {
						delete(m.selected, id)
					} else {
						m.selected[id] = true
					}
					m.sortChecklist()
					return *m, nil
				}
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
	if m.readOnly {
		return m.viewReadOnly()
	}
	return m.viewSelectable()
}

func (m *Modal) viewReadOnly() string {
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
	if m.isError {
		return RenderPanelWithColor(m.title, "", content, contentW, visibleH,
			&ScrollInfo{Total: totalLines, Visible: visibleH, Offset: m.offset}, theme.ColorRed)
	}
	return RenderPanelFull(m.title, "", content, contentW, visibleH, true,
		&ScrollInfo{Total: totalLines, Visible: visibleH, Offset: m.offset})
}

func (m *Modal) viewSelectable() string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

	contentW := m.selectionContentW()
	if m.checklist {
		contentW = m.checklistContentW()
	}

	lines := m.renderItems(titleStyle, contentW)
	footer := m.renderFooter()

	popupH := len(lines)
	maxH := max(m.height-2, 5)
	if popupH > maxH {
		popupH = maxH
	}

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

func (m *Modal) checklistContentW() int {
	minW := lipgloss.Width(m.title) + 2
	for _, item := range m.items {
		if w := lipgloss.Width(item.Label) + 5; w > minW {
			minW = w
		}
	}
	maxW := max(m.width*7/10, 30)
	return min(minW, maxW)
}

func (m *Modal) renderItems(titleStyle lipgloss.Style, contentW int) []string {
	var lines []string
	lines = append(lines, " "+titleStyle.Render(m.title))
	lines = append(lines, "")

	checkGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	activeMarker := checkGreen.Render("*")
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i, item := range m.items {
		if item.Separator {
			label := TruncateEnd(item.Label, contentW-4)
			pad := contentW - lipgloss.Width(label) - 4
			left := max(pad/2, 0)
			right := max(pad-left, 0)
			lines = append(lines, sepStyle.Render(strings.Repeat("─", left)+" "+label+" "+strings.Repeat("─", right)))
			continue
		}
		isCursor := i == m.cursor
		if m.checklist {
			text := TruncateMiddle(item.Label, contentW-4)
			sel := m.selected[item.ID]
			style := lipgloss.NewStyle().Width(contentW)
			switch {
			case isCursor:
				style = style.Bold(true).Background(lipgloss.Color("4"))
				if sel {
					lines = append(lines, style.Render("✓ "+text))
				} else {
					lines = append(lines, style.Render("  "+text))
				}
			case sel:
				lines = append(lines, style.Render(checkGreen.Render("✓")+" "+text))
			default:
				lines = append(lines, style.Render("  "+text))
			}
		} else {
			marker := " "
			if item.Active {
				marker = "*"
			}
			text := TruncateMiddle(item.Label, contentW-3)
			style := lipgloss.NewStyle().Width(contentW)
			switch {
			case isCursor:
				style = style.Bold(true).Background(lipgloss.Color("4"))
				lines = append(lines, style.Render(marker+text))
			case item.Active:
				lines = append(lines, style.Render(activeMarker+text))
			case item.Internal:
				lines = append(lines, style.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(" "+text)))
			default:
				lines = append(lines, style.Render(" "+text))
			}
		}
	}
	return lines
}

func (m *Modal) renderFooter() string {
	total, pos := 0, 0
	for i, item := range m.items {
		if !item.Separator {
			total++
			if i == m.cursor {
				pos = total
			}
		}
	}
	return fmt.Sprintf("%d of %d", pos, total)
}

// Intercept handles a message if the modal is visible. Implements Overlay.
func (m *Modal) Intercept(msg tea.Msg) (tea.Cmd, bool) {
	if !m.visible {
		return nil, false
	}
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		updated, cmd := m.Update(msg)
		*m = updated
		return cmd, true
	}
	return nil, false
}

// Render draws the modal centered on bg with optional hint box. Implements Overlay.
func (m *Modal) Render(bg string, w, h int) string {
	if !m.visible {
		return bg
	}
	popup := m.View()
	return centerOverlayWithHint(bg, popup, m.HintView(), w, h)
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
