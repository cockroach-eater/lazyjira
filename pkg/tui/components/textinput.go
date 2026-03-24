package components

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInput is a readline-style text input component.
type TextInput struct {
	value  string
	cursor int // 0 = before first char
	width  int

	// Highlighter, if set, is called with visible runes and returns styled segments.
	// Each segment covers a contiguous range of runes.
	// The TextInput will render cursor overlay on top of the highlighted output.
	Highlighter func(text []rune) []StyledSegment
}

// StyledSegment is a contiguous run of text with a style.
type StyledSegment struct {
	Text  string
	Style lipgloss.Style
}

func NewTextInput() TextInput {
	return TextInput{}
}

func (t *TextInput) SetValue(s string) {
	t.value = s
	t.cursor = utf8.RuneCountInString(s)
}

func (t *TextInput) Value() string {
	return t.value
}

func (t *TextInput) SetWidth(w int) {
	t.width = w
}

func (t *TextInput) CursorPos() int {
	return t.cursor
}

func (t *TextInput) setCursor(pos int) {
	runeLen := len([]rune(t.value))
	if pos < 0 {
		pos = 0
	}
	if pos > runeLen {
		pos = runeLen
	}
	t.cursor = pos
}

func (t *TextInput) InsertAtCursor(s string) {
	runes := []rune(t.value)
	inserted := []rune(s)
	newRunes := make([]rune, 0, len(runes)+len(inserted))
	newRunes = append(newRunes, runes[:t.cursor]...)
	newRunes = append(newRunes, inserted...)
	newRunes = append(newRunes, runes[t.cursor:]...)
	t.value = string(newRunes)
	t.cursor += len(inserted)
}

// Update handles key events. Returns the updated TextInput and whether the value changed.
func (t *TextInput) Update(msg tea.Msg) (TextInput, bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return *t, false
	}

	runes := []rune(t.value)
	runeLen := len(runes)

	switch km.Type {
	case tea.KeyLeft:
		if t.cursor > 0 {
			t.cursor--
		}
		return *t, false

	case tea.KeyRight:
		if t.cursor < runeLen {
			t.cursor++
		}
		return *t, false

	case tea.KeyHome:
		t.cursor = 0
		return *t, false

	case tea.KeyEnd:
		t.cursor = runeLen
		return *t, false

	case tea.KeyBackspace:
		if t.cursor > 0 {
			t.value = string(runes[:t.cursor-1]) + string(runes[t.cursor:])
			t.cursor--
			return *t, true
		}
		return *t, false

	case tea.KeyDelete:
		if t.cursor < runeLen {
			t.value = string(runes[:t.cursor]) + string(runes[t.cursor+1:])
			return *t, true
		}
		return *t, false

	case tea.KeyCtrlA:
		t.cursor = 0
		return *t, false

	case tea.KeyCtrlE:
		t.cursor = runeLen
		return *t, false

	case tea.KeyCtrlW:
		if t.cursor == 0 {
			return *t, false
		}
		i := t.cursor
		for i > 0 && runes[i-1] == ' ' {
			i--
		}
		for i > 0 && runes[i-1] != ' ' {
			i--
		}
		t.value = string(runes[:i]) + string(runes[t.cursor:])
		t.cursor = i
		return *t, true

	case tea.KeyCtrlK:
		if t.cursor < runeLen {
			t.value = string(runes[:t.cursor])
			return *t, true
		}
		return *t, false

	case tea.KeyCtrlU:
		if t.cursor > 0 {
			t.value = string(runes[t.cursor:])
			t.cursor = 0
			return *t, true
		}
		return *t, false

	case tea.KeySpace:
		t.InsertAtCursor(" ")
		return *t, true

	case tea.KeyRunes:
		s := km.String()
		t.InsertAtCursor(s)
		return *t, true

	default:
		return *t, false
	}
}

// View renders the text with a visible cursor block at the cursor position.
func (t *TextInput) View() string {
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6"))

	cursorBlock := cursorStyle.Render("█")

	runes := []rune(t.value)
	runeLen := len(runes)

	avail := t.width
	if avail <= 0 {
		avail = 40
	}
	textAvail := max(avail-1, 1)

	// Compute sliding window.
	start := 0
	end := runeLen

	if runeLen > textAvail {
		half := textAvail / 2
		start = max(t.cursor-half, 0)
		end = start + textAvail
		if end > runeLen {
			end = runeLen
			start = max(end-textAvail, 0)
		}
	}

	visible := runes[start:end]
	cursorInWindow := t.cursor - start

	if t.Highlighter != nil {
		return t.viewHighlighted(visible, cursorInWindow, cursorStyle, cursorBlock)
	}

	// Plain rendering (no highlighter).
	var b strings.Builder
	if cursorInWindow >= 0 && cursorInWindow < len(visible) {
		b.WriteString(string(visible[:cursorInWindow]))
		b.WriteString(cursorStyle.Render(string(visible[cursorInWindow])))
		b.WriteString(string(visible[cursorInWindow+1:]))
	} else {
		b.WriteString(string(visible))
		b.WriteString(cursorBlock)
	}

	return b.String()
}

// viewHighlighted renders with syntax highlighting + cursor overlay.
func (t *TextInput) viewHighlighted(visible []rune, cursorInWindow int, cursorStyle lipgloss.Style, cursorBlock string) string {
	segments := t.Highlighter(visible)

	var b strings.Builder
	pos := 0 // current rune position within visible

	for _, seg := range segments {
		segRunes := []rune(seg.Text)
		segEnd := pos + len(segRunes)

		if cursorInWindow >= pos && cursorInWindow < segEnd {
			// Cursor is inside this segment — split it.
			before := segRunes[:cursorInWindow-pos]
			at := segRunes[cursorInWindow-pos]
			after := segRunes[cursorInWindow-pos+1:]

			if len(before) > 0 {
				b.WriteString(seg.Style.Render(string(before)))
			}
			// Cursor char: use cursor color, keep segment background.
			b.WriteString(cursorStyle.Render(string(at)))
			if len(after) > 0 {
				b.WriteString(seg.Style.Render(string(after)))
			}
		} else {
			b.WriteString(seg.Style.Render(seg.Text))
		}
		pos = segEnd
	}

	// Cursor at end.
	if cursorInWindow >= pos {
		b.WriteString(cursorBlock)
	}

	return b.String()
}
