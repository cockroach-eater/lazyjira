package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// testBackground stands in for whatever the app already rendered underneath
// an overlay.
const testBackground = "background"

func taKey(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }
func taRunes(s string) tea.KeyMsg    { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func taSend(m *TextAreaModal, k tea.KeyMsg) tea.Cmd {
	updated, cmd := m.Update(k)
	*m = updated
	return cmd
}

func taType(m *TextAreaModal, s string) {
	for _, r := range s {
		if r == '\n' {
			taSend(m, taKey(tea.KeyEnter))
			continue
		}
		if r == ' ' {
			taSend(m, taKey(tea.KeySpace))
			continue
		}
		taSend(m, taRunes(string(r)))
	}
}

func newShownTextArea(prefill string) *TextAreaModal {
	m := NewTextAreaModal()
	m.SetSize(80, 24)
	m.Show("New comment", prefill)
	return &m
}

func TestTextAreaShowStartsWithCursorAtEnd(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("hello")

	if !m.IsVisible() {
		t.Fatal("modal is not visible after Show")
	}
	taType(m, "!")
	if got := m.Text(); got != "hello!" {
		t.Errorf("Text = %q, want the cursor to have started at the end", got)
	}
}

func TestTextAreaEnterInsertsNewline(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("")
	taType(m, "one\ntwo")

	if got := m.Text(); got != "one\ntwo" {
		t.Errorf("Text = %q", got)
	}
	if !m.IsVisible() {
		t.Error("enter closed the popup; it must insert a newline instead")
	}
}

func TestTextAreaCtrlDConfirms(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("")
	taType(m, "ship it")

	cmd := taSend(m, taKey(tea.KeyCtrlD))
	if cmd == nil {
		t.Fatal("ctrl+d produced no message")
	}
	msg, ok := cmd().(TextAreaConfirmedMsg)
	if !ok {
		t.Fatalf("message = %T, want TextAreaConfirmedMsg", cmd())
	}
	if msg.Text != "ship it" {
		t.Errorf("Text = %q", msg.Text)
	}
	if m.IsVisible() {
		t.Error("the popup stayed open after confirming")
	}
}

func TestTextAreaCtrlSAlsoConfirms(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("body")

	cmd := taSend(m, taKey(tea.KeyCtrlS))
	if _, ok := cmd().(TextAreaConfirmedMsg); !ok {
		t.Errorf("ctrl+s produced %T, want TextAreaConfirmedMsg", cmd())
	}
}

func TestTextAreaEscCancels(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("draft")

	cmd := taSend(m, taKey(tea.KeyEsc))
	if _, ok := cmd().(TextAreaCancelledMsg); !ok {
		t.Errorf("message = %T, want TextAreaCancelledMsg", cmd())
	}
	if m.IsVisible() {
		t.Error("esc left the popup open")
	}
}

func TestTextAreaCtrlEHandsOffToEditor(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("")
	taType(m, "long text")

	cmd := taSend(m, taKey(tea.KeyCtrlE))
	msg, ok := cmd().(TextAreaHandoffMsg)
	if !ok {
		t.Fatalf("message = %T, want TextAreaHandoffMsg", cmd())
	}
	if msg.Text != "long text" {
		t.Errorf("handoff text = %q, want the current buffer", msg.Text)
	}
	if m.IsVisible() {
		t.Error("the popup stayed open while handing off to the editor")
	}
}

func TestTextAreaBackspaceJoinsLines(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("")
	taType(m, "ab\ncd")

	taSend(m, taKey(tea.KeyHome))
	taSend(m, taKey(tea.KeyBackspace))

	if got := m.Text(); got != "abcd" {
		t.Errorf("Text = %q, want the lines joined", got)
	}
	// The cursor should sit at the seam, so typing lands between b and c.
	taType(m, "-")
	if got := m.Text(); got != "ab-cd" {
		t.Errorf("Text = %q, want the cursor left at the join point", got)
	}
}

func TestTextAreaBackspaceAtStartIsNoOp(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("abc")
	taSend(m, taKey(tea.KeyHome))
	taSend(m, taKey(tea.KeyBackspace))

	if got := m.Text(); got != "abc" {
		t.Errorf("Text = %q, want it untouched at the very start", got)
	}
}

func TestTextAreaDeleteForwardJoinsLines(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("ab\ncd")
	taSend(m, taKey(tea.KeyUp))
	taSend(m, taKey(tea.KeyEnd))
	taSend(m, taKey(tea.KeyDelete))

	if got := m.Text(); got != "abcd" {
		t.Errorf("Text = %q", got)
	}
}

func TestTextAreaVerticalMovementClampsColumn(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("short\nmuch longer line")
	// Cursor is at the end of the long line; going up must clamp.
	taSend(m, taKey(tea.KeyUp))
	taType(m, "!")

	if got := m.Text(); got != "short!\nmuch longer line" {
		t.Errorf("Text = %q, want the column clamped to the shorter line", got)
	}
}

func TestTextAreaVerticalMovementStopsAtEdges(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("only line")
	taSend(m, taKey(tea.KeyUp))
	taSend(m, taKey(tea.KeyDown))
	taType(m, "!")

	if got := m.Text(); !strings.HasSuffix(got, "!") {
		t.Errorf("Text = %q, want the cursor still on the single line", got)
	}
}

func TestTextAreaHorizontalMovementWrapsLines(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("ab\ncd")
	taSend(m, taKey(tea.KeyHome))
	taSend(m, taKey(tea.KeyLeft)) // to the end of the previous line
	taType(m, "!")

	if got := m.Text(); got != "ab!\ncd" {
		t.Errorf("Text = %q, want left at column 0 to wrap to the line above", got)
	}
}

func TestTextAreaCtrlUAndCtrlK(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("hello world")
	taSend(m, taKey(tea.KeyHome))
	for range 6 {
		taSend(m, taKey(tea.KeyRight))
	}
	taSend(m, taKey(tea.KeyCtrlU))
	if got := m.Text(); got != "world" {
		t.Errorf("after ctrl+u Text = %q", got)
	}

	taSend(m, taKey(tea.KeyCtrlK))
	if got := m.Text(); got != "" {
		t.Errorf("after ctrl+k Text = %q", got)
	}
}

func TestTextAreaIgnoresInputWhenHidden(t *testing.T) {
	t.Parallel()
	m := NewTextAreaModal()
	if cmd, handled := m.Intercept(taRunes("x")); handled || cmd != nil {
		t.Error("a hidden popup intercepted a key")
	}
}

func TestTextAreaViewShowsContentAndHints(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("first line")
	view := m.View()

	for _, want := range []string{"New comment", "first line", "ctrl+d send", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}

func TestTextAreaViewScrollsToCursor(t *testing.T) {
	t.Parallel()
	m := newShownTextArea("")
	m.SetSize(80, 14) // room for a handful of lines
	for range 30 {
		taType(m, "line")
		taSend(m, taKey(tea.KeyEnter))
	}
	taType(m, "LAST")

	view := m.View()
	if !strings.Contains(view, "LAST") {
		t.Errorf("the view did not scroll to the cursor line:\n%s", view)
	}
}

func TestTextAreaRenderLeavesBackgroundWhenHidden(t *testing.T) {
	t.Parallel()
	m := NewTextAreaModal()
	bg := testBackground
	if got := m.Render(bg, 80, 24); got != bg {
		t.Errorf("Render = %q, want the untouched background", got)
	}
}
