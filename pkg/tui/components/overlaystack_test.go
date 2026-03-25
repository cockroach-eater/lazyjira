package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeOverlay is a minimal OverlayPanel for testing.
type fakeOverlay struct {
	visible     bool
	intercepted bool
	rendered    bool
	w, h        int
}

func (f *fakeOverlay) IsVisible() bool    { return f.visible }
func (f *fakeOverlay) SetSize(w, h int)   { f.w = w; f.h = h }
func (f *fakeOverlay) Render(bg string, w, h int) string {
	f.rendered = true
	return "[overlay]"
}
func (f *fakeOverlay) Intercept(msg tea.Msg) (tea.Cmd, bool) {
	if !f.visible {
		return nil, false
	}
	f.intercepted = true
	return nil, true
}

func TestOverlayStack_InterceptRoutesToFirstVisible(t *testing.T) {
	a := &fakeOverlay{visible: false}
	b := &fakeOverlay{visible: true}
	c := &fakeOverlay{visible: true}
	stack := OverlayStack{a, b, c}

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	_, ok := stack.Intercept(key)

	if !ok {
		t.Fatal("expected intercept to return true")
	}
	if a.intercepted {
		t.Error("invisible overlay should not intercept")
	}
	if !b.intercepted {
		t.Error("first visible overlay should intercept")
	}
	if c.intercepted {
		t.Error("second visible overlay should not be reached")
	}
}

func TestOverlayStack_InterceptPassesThroughWhenNoneVisible(t *testing.T) {
	a := &fakeOverlay{visible: false}
	b := &fakeOverlay{visible: false}
	stack := OverlayStack{a, b}

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	_, ok := stack.Intercept(key)

	if ok {
		t.Error("expected intercept to pass through when no overlay visible")
	}
}

func TestOverlayStack_RenderFirstVisible(t *testing.T) {
	a := &fakeOverlay{visible: false}
	b := &fakeOverlay{visible: true}
	c := &fakeOverlay{visible: true}
	stack := OverlayStack{a, b, c}

	result := stack.Render("bg", 80, 24)

	if result != "[overlay]" {
		t.Errorf("expected [overlay], got %q", result)
	}
	if a.rendered {
		t.Error("invisible overlay should not render")
	}
	if !b.rendered {
		t.Error("first visible overlay should render")
	}
	if c.rendered {
		t.Error("second visible overlay should not render")
	}
}

func TestOverlayStack_RenderPassesThroughWhenNoneVisible(t *testing.T) {
	a := &fakeOverlay{visible: false}
	stack := OverlayStack{a}

	result := stack.Render("background", 80, 24)

	if result != "background" {
		t.Errorf("expected passthrough, got %q", result)
	}
}

func TestOverlayStack_SetSizePropagates(t *testing.T) {
	a := &fakeOverlay{}
	b := &fakeOverlay{}
	stack := OverlayStack{a, b}

	stack.SetSize(120, 40)

	if a.w != 120 || a.h != 40 {
		t.Errorf("a got %dx%d, want 120x40", a.w, a.h)
	}
	if b.w != 120 || b.h != 40 {
		t.Errorf("b got %dx%d, want 120x40", b.w, b.h)
	}
}
