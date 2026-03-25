package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

// JQLSubmitMsg is sent when the user submits a JQL query.
type JQLSubmitMsg struct{ Query string }

// JQLCancelMsg is sent when the user cancels the JQL modal.
type JQLCancelMsg struct{}

// JQLInputChangedMsg is sent when the input text changes (for autocomplete).
type JQLInputChangedMsg struct {
	Text      string
	CursorPos int
}

// Bottom panel modes.
const (
	jqlModeHistory      = "history"
	jqlModeAutocomplete = "autocomplete"
)

// JQLModal is a full-screen two-panel modal for JQL search.
// Top panel: TextInput for JQL query.
// Bottom panel: suggestions (autocomplete) or history list.
type JQLModal struct {
	input      TextInput
	items      []string // current items (history or suggestions)
	cursor     int      // cursor in items list
	offset     int      // scroll offset in items list
	focusInput bool     // true = input focused, false = list focused
	visible    bool
	loading    bool   // search in progress
	acLoading  bool   // autocomplete loading
	errorMsg   string // error from last search
	mode       string // jqlModeHistory or jqlModeAutocomplete
	partialLen int    // length of partial text to replace on suggestion insert
	width      int
	height     int
}

func NewJQLModal() JQLModal {
	ti := NewTextInput()
	ti.Highlighter = HighlightJQL
	return JQLModal{
		input:      ti,
		focusInput: true,
		mode:       jqlModeHistory,
	}
}

// Show opens the modal with prefilled text and history items.
func (m *JQLModal) Show(prefill string, history []string) {
	m.visible = true
	m.focusInput = true
	m.input.SetValue(prefill)
	m.items = history
	m.cursor = 0
	m.offset = 0
	m.loading = false
	m.acLoading = false
	m.errorMsg = ""
	m.mode = jqlModeHistory
}

// Hide closes the modal.
func (m *JQLModal) Hide() {
	m.visible = false
}

func (m *JQLModal) IsVisible() bool { return m.visible }

func (m *JQLModal) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.SetWidth(w - 6) // border + padding
}

func (m *JQLModal) SetLoading(v bool) { m.loading = v }
func (m *JQLModal) SetError(msg string) {
	m.errorMsg = msg
	m.loading = false
}

// SetSuggestions switches the bottom panel to autocomplete mode with suggestions.
func (m *JQLModal) SetSuggestions(suggestions []string) {
	m.items = suggestions
	m.mode = jqlModeAutocomplete
	m.cursor = 0
	m.offset = 0
	m.acLoading = false
}

// SetHistory switches the bottom panel to history mode.
func (m *JQLModal) SetHistory(history []string) {
	m.items = history
	m.mode = jqlModeHistory
	m.cursor = 0
	m.offset = 0
	m.acLoading = false
}

// SetACLoading sets autocomplete loading state.
func (m *JQLModal) SetACLoading(v bool) { m.acLoading = v }

// SetPartialLen sets how many characters of partial text to replace on insert.
func (m *JQLModal) SetPartialLen(n int) { m.partialLen = n }

// InputValue returns the current input text.
func (m *JQLModal) InputValue() string { return m.input.Value() }

// InputCursorPos returns the cursor position in the input.
func (m *JQLModal) InputCursorPos() int { return m.input.CursorPos() }

func (m *JQLModal) listHeight() int {
	h := m.height - 8
	if m.errorMsg != "" {
		h--
	}
	return max(h, 3)
}

// Update handles key/mouse events. Returns updated modal and optional command.
func (m *JQLModal) Update(msg tea.Msg) (JQLModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return *m, nil
}

func (m *JQLModal) handleKey(msg tea.KeyMsg) (JQLModal, tea.Cmd) {
	// Clear error on any keystroke when input focused.
	if m.errorMsg != "" && m.focusInput {
		m.errorMsg = ""
	}

	switch msg.Type {
	case tea.KeyEsc:
		if !m.focusInput {
			m.focusInput = true
			return *m, nil
		}
		m.Hide()
		return *m, func() tea.Msg { return JQLCancelMsg{} }

	case tea.KeyTab:
		// If exactly 1 suggestion and input is focused, auto-insert it.
		if m.focusInput && m.mode == jqlModeAutocomplete && len(m.items) == 1 {
			m.insertSuggestion(m.items[0])
			return *m, func() tea.Msg {
				return JQLInputChangedMsg{
					Text:      m.input.Value(),
					CursorPos: m.input.CursorPos(),
				}
			}
		}
		m.focusInput = !m.focusInput
		return *m, nil

	case tea.KeyEnter:
		return m.handleEnter()
	default:
		// handled below
	}

	if m.focusInput {
		updated, changed := m.input.Update(msg)
		m.input = updated
		if changed {
			return *m, func() tea.Msg {
				return JQLInputChangedMsg{
					Text:      m.input.Value(),
					CursorPos: m.input.CursorPos(),
				}
			}
		}
		return *m, nil
	}

	// List navigation (j/k/g/G and arrow keys).
	m.handleListNav(msg)
	return *m, nil
}

func (m *JQLModal) handleEnter() (JQLModal, tea.Cmd) {
	if m.focusInput {
		if m.loading {
			return *m, nil
		}
		q := m.input.Value()
		if strings.TrimSpace(q) == "" {
			return *m, nil
		}
		m.loading = true
		return *m, func() tea.Msg { return JQLSubmitMsg{Query: q} }
	}
	// List focused — select item.
	if m.cursor >= 0 && m.cursor < len(m.items) {
		selected := m.items[m.cursor]
		if m.mode == jqlModeHistory {
			m.input.SetValue(selected)
			m.focusInput = true
			return *m, nil
		}
		// Autocomplete — insert suggestion at cursor, replacing partial text.
		m.insertSuggestion(selected)
		m.focusInput = true
		return *m, func() tea.Msg {
			return JQLInputChangedMsg{
				Text:      m.input.Value(),
				CursorPos: m.input.CursorPos(),
			}
		}
	}
	return *m, nil
}

func (m *JQLModal) handleListNav(msg tea.KeyMsg) {
	moveDown := msg.String() == "j" || msg.Type == tea.KeyDown
	moveUp := msg.String() == "k" || msg.Type == tea.KeyUp
	switch {
	case moveDown:
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m.adjustListOffset()
		}
	case moveUp:
		if m.cursor > 0 {
			m.cursor--
			m.adjustListOffset()
		}
	case msg.String() == "g":
		m.cursor = 0
		m.offset = 0
	case msg.String() == "G":
		if len(m.items) > 0 {
			m.cursor = len(m.items) - 1
			m.adjustListOffset()
		}
	}
}

func (m *JQLModal) handleMouse(msg tea.MouseMsg) (JQLModal, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		if m.offset > 0 {
			m.offset--
		}
	case msg.Button == tea.MouseButtonWheelDown:
		maxOffset := max(len(m.items)-m.listHeight(), 0)
		if m.offset < maxOffset {
			m.offset++
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		listTop := 5
		if m.errorMsg != "" {
			listTop++
		}
		listH := m.listHeight()
		relY := msg.Y - listTop
		if relY >= 0 && relY < listH {
			idx := m.offset + relY
			if idx < len(m.items) {
				m.cursor = idx
				m.focusInput = false
			}
		}
	}
	return *m, nil
}

// jqlOperators lists JQL operators for context detection during insertion.
var jqlOperators = map[string]bool{
	"=": true, "!=": true, "~": true, "!~": true,
	">": true, ">=": true, "<": true, "<=": true,
	"is": true, "in": true, "not": true, "was": true,
}

// insertSuggestion replaces the partial text before the cursor with the selected suggestion.
// Context-aware: auto-adds parentheses for IN lists, auto-quotes values with spaces.
func (m *JQLModal) insertSuggestion(value string) {
	// Strip existing surrounding quotes (API may return pre-quoted values).
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	// Auto-quote if value contains spaces or special chars.
	if strings.ContainsAny(value, " \"=!~<>()[]") {
		value = `"` + value + `"`
	}

	text := m.input.Value()
	runes := []rune(text)
	cursor := min(m.input.CursorPos(), len(runes))

	// Use partialLen to determine how much text to replace.
	// partialLen is set by the autocomplete context parser and accounts for
	// multi-word partials like "ready for de" correctly.
	start := max(cursor-m.partialLen, 0)

	// The token being replaced.
	tokenBeingReplaced := strings.ToLower(string(runes[start:cursor]))

	// If the token is an operator (e.g. "in", "="), don't replace it — append after it.
	if jqlOperators[tokenBeingReplaced] {
		start = cursor // don't replace the operator
	}

	// Detect what's immediately before the insertion point (skip spaces).
	beforePartial := strings.TrimRight(string(runes[:start]), " ")
	beforeLower := strings.ToLower(beforePartial)

	// Determine prefix and suffix based on context.
	prefix := " " // default: space before value
	suffix := " " // default: space after value

	switch {
	case strings.HasSuffix(beforeLower, " in") || strings.HasSuffix(beforeLower, "\tin"):
		// After "in" operator — open paren, value, comma for next.
		prefix = " ("
		suffix = ", "
	case len(beforePartial) > 0 && beforePartial[len(beforePartial)-1] == '(':
		// Right after opening paren.
		prefix = ""
		suffix = ", "
	case len(beforePartial) > 0 && beforePartial[len(beforePartial)-1] == ',':
		// After comma inside IN list.
		prefix = " "
		suffix = ", "
	default:
		// After a regular operator like "=", "~", etc.
		// If start == cursor (we didn't replace anything), add a space before.
		// If start < cursor (we replaced a partial), no extra space needed.
		if start < cursor {
			prefix = ""
		}
	}

	insertion := prefix + value + suffix
	newText := string(runes[:start]) + insertion + string(runes[cursor:])
	m.input.SetValue(newText)
	m.input.setCursor(start + len([]rune(insertion)))
}

func (m *JQLModal) adjustListOffset() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
}

// Intercept handles a message if the modal is visible. Implements Overlay.
func (m *JQLModal) Intercept(msg tea.Msg) (tea.Cmd, bool) {
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

// Render draws the JQL modal centered on bg. Implements Overlay.
func (m *JQLModal) Render(bg string, w, h int) string {
	if !m.visible {
		return bg
	}
	return Overlay(bg, m.View(), w, h)
}

// SelectedSuggestion returns the currently selected suggestion (for autocomplete mode).
func (m *JQLModal) SelectedSuggestion() string {
	if m.mode != jqlModeAutocomplete || m.focusInput {
		return ""
	}
	if m.cursor >= 0 && m.cursor < len(m.items) {
		return m.items[m.cursor]
	}
	return ""
}

// View renders the full-screen JQL modal overlay.
func (m *JQLModal) View() string {
	if !m.visible || m.width == 0 || m.height == 0 {
		return ""
	}

	contentW := max(m.width-4, 10)
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen)

	// === Top Panel: Input ===
	inputContent := m.input.View()
	if m.loading {
		inputContent += lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("  Searching...")
	}
	inputPanel := RenderPanelFull("JQL Query", "", inputContent, m.width-2, 1, m.focusInput, nil)

	// Error line.
	errorLine := ""
	if m.errorMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
		errorLine = " " + errStyle.Render(m.errorMsg)
	}

	// === Bottom Panel: List ===
	listH := m.listHeight()
	listContent := m.renderListContent(listH, contentW)

	listTitle := "History"
	if m.mode == jqlModeAutocomplete {
		listTitle = "Suggestions"
	}
	listFooter := ""
	if len(m.items) > 0 {
		listFooter = fmt.Sprintf("%d of %d", m.cursor+1, len(m.items))
	}
	scroll := &ScrollInfo{Total: len(m.items), Visible: listH, Offset: m.offset}
	listPanel := RenderPanelFull(listTitle, listFooter, listContent, m.width-2, listH, !m.focusInput, scroll)

	// === Combine ===
	var parts []string
	parts = append(parts, inputPanel)
	if errorLine != "" {
		parts = append(parts, errorLine)
	}
	parts = append(parts, listPanel)
	inner := strings.Join(parts, "\n")

	// Wrap in outer border.
	innerLines := strings.Split(inner, "\n")
	topLine := borderStyle.Render("╭" + strings.Repeat("─", m.width-2) + "╮")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", m.width-2) + "╯")

	var b strings.Builder
	b.WriteString(topLine + "\n")
	for _, line := range innerLines {
		lineW := lipgloss.Width(line)
		pad := max(m.width-2-lineW, 0)
		b.WriteString(borderStyle.Render("│") + line + strings.Repeat(" ", pad) + borderStyle.Render("│") + "\n")
	}
	b.WriteString(bottomLine)

	return b.String()
}

func (m *JQLModal) renderListContent(listH, contentW int) string {
	switch {
	case m.acLoading:
		lines := make([]string, listH)
		lines[0] = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("  Loading...")
		return strings.Join(lines, "\n")

	case len(m.items) == 0:
		emptyMsg := "No history"
		if m.mode == jqlModeAutocomplete {
			emptyMsg = "No suggestions"
		}
		lines := make([]string, listH)
		lines[0] = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  " + emptyMsg)
		return strings.Join(lines, "\n")

	default:
		var rows []string
		end := min(m.offset+listH, len(m.items))
		for i := m.offset; i < end; i++ {
			item := m.items[i]
			iw := contentW - 2
			if len(item) > iw {
				item = item[:iw]
			}
			if i == m.cursor && !m.focusInput {
				row := lipgloss.NewStyle().
					Background(lipgloss.Color("4")).
					Width(contentW).
					Render(" " + item)
				rows = append(rows, row)
			} else {
				rows = append(rows, " "+item)
			}
		}
		return strings.Join(rows, "\n")
	}
}
