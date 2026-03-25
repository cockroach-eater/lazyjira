package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModal_ShowAndSelect(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	items := []ModalItem{
		{ID: "1", Label: "First"},
		{ID: "2", Label: "Second"},
		{ID: "3", Label: "Third"},
	}
	m.Show("Pick", items)

	if !m.IsVisible() {
		t.Fatal("modal should be visible after Show")
	}

	// Navigate down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Select with enter.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsVisible() {
		t.Error("modal should hide after selection")
	}
	if cmd == nil {
		t.Fatal("expected a command from selection")
	}
	msg := cmd()
	sel, ok := msg.(ModalSelectedMsg)
	if !ok {
		t.Fatalf("expected ModalSelectedMsg, got %T", msg)
	}
	if sel.Item.ID != "3" {
		t.Errorf("selected ID = %q, want %q", sel.Item.ID, "3")
	}
}

func TestModal_Cancel(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.Show("Pick", []ModalItem{{ID: "1", Label: "One"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.IsVisible() {
		t.Error("modal should hide after esc")
	}
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	msg := cmd()
	if _, ok := msg.(ModalCancelledMsg); !ok {
		t.Fatalf("expected ModalCancelledMsg, got %T", msg)
	}
}

func TestModal_SkipsSeparators(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	items := []ModalItem{
		{Label: "Section", Separator: true},
		{ID: "1", Label: "First"},
		{ID: "2", Label: "Second"},
	}
	m.Show("Pick", items)

	// Cursor should skip separator and land on "First".
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msg := cmd()
	sel := msg.(ModalSelectedMsg)
	if sel.Item.ID != "1" {
		t.Errorf("selected ID = %q, want %q (should skip separator)", sel.Item.ID, "1")
	}
}

func TestModal_InterceptWhenVisible(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)

	// Not visible — should not intercept.
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	_, ok := m.Intercept(key)
	if ok {
		t.Error("hidden modal should not intercept")
	}

	// Visible — should intercept.
	m.Show("Pick", []ModalItem{{ID: "1", Label: "One"}})
	_, ok = m.Intercept(key)
	if !ok {
		t.Error("visible modal should intercept key messages")
	}
}

func TestModal_SearchFilter(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	m.Show("Pick", []ModalItem{
		{ID: "1", Label: "Apple"},
		{ID: "2", Label: "Banana"},
		{ID: "3", Label: "Avocado"},
	})

	// Activate search.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.IsSearching() {
		t.Fatal("search should be active after /")
	}

	// Type "av".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Confirm search, then select.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm search
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select

	msg := cmd()
	sel := msg.(ModalSelectedMsg)
	if sel.Item.ID != "3" {
		t.Errorf("selected ID = %q, want %q (Avocado)", sel.Item.ID, "3")
	}
}

func TestModal_Checklist(t *testing.T) {
	m := NewModal()
	m.SetSize(80, 24)
	items := []ModalItem{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
		{ID: "c", Label: "Gamma"},
	}
	m.ShowChecklist("Select", items, nil)

	if !m.IsChecklist() {
		t.Fatal("should be in checklist mode")
	}

	// Toggle first item with space.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	// Move down and toggle second.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	// Confirm with enter.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msg := cmd()
	confirmed, ok := msg.(ChecklistConfirmedMsg)
	if !ok {
		t.Fatalf("expected ChecklistConfirmedMsg, got %T", msg)
	}
	if len(confirmed.Selected) != 2 {
		t.Errorf("selected %d items, want 2", len(confirmed.Selected))
	}
}
