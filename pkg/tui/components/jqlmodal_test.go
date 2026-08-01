package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
)

func TestJQLModal_ShowAndIsVisible(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	testkit.AssertEqual(t, "initially invisible", m.IsVisible(), false)
	m.Show(testQueryText, []string{testHistoryItem1})
	testkit.AssertEqual(t, "visible after Show", m.IsVisible(), true)
}

func TestJQLModal_ShowPrefillsInput(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.Show(testQueryText, nil)
	testkit.AssertEqual(t, "input value", m.InputValue(), testQueryText)
}

func TestJQLModal_HideHidesModal(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.Show("", nil)
	m.Hide()
	testkit.AssertEqual(t, "hidden", m.IsVisible(), false)
}

func TestJQLModal_SetSize(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	testkit.AssertEqual(t, "width", m.width, 80)
	testkit.AssertEqual(t, "height", m.height, 24)
}

func TestJQLModal_SetLoading(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetLoading(true)
	testkit.AssertEqual(t, "loading set", m.loading, true)
}

func TestJQLModal_SetError(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetLoading(true)
	m.SetError("something went wrong")
	testkit.AssertEqual(t, "error set", m.errorMsg, "something went wrong")
	testkit.AssertEqual(t, "loading cleared", m.loading, false)
}

func TestJQLModal_SetSuggestionsAndMode(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.Show("", nil)
	m.SetSuggestions([]string{"status", "priority"})
	testkit.AssertEqual(t, "mode autocomplete", m.mode, jqlModeAutocomplete)
	testkit.AssertEqual(t, "first suggestion", m.items[0], "status")
}

func TestJQLModal_SetHistory(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.Show("", nil)
	m.SetSuggestions([]string{"x"})
	m.SetHistory([]string{testHistoryItem1, testHistoryItem2})
	testkit.AssertEqual(t, "mode history", m.mode, jqlModeHistory)
	testkit.AssertEqual(t, "first history item", m.items[0], testHistoryItem1)
}

func TestJQLModal_SetACLoading(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetACLoading(true)
	testkit.AssertEqual(t, "ac loading", m.acLoading, true)
}

func TestJQLModal_SetPartialLen(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetPartialLen(4)
	testkit.AssertEqual(t, "partial len", m.partialLen, 4)
}

func TestJQLModal_InputCursorPos(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.Show("abc", nil)
	testkit.AssertEqual(t, "cursor at end", m.InputCursorPos(), 3)
}

func TestJQLModal_ViewRendersWhenVisible(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show(testQueryText, []string{testHistoryItem1})
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "JQL Query") {
		t.Errorf("expected 'JQL Query' in view, got %q", plain)
	}
}

func TestJQLModal_ViewEmptyWhenInvisible(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	testkit.AssertEqual(t, "empty view when invisible", m.View(), "")
}

func TestJQLModal_ViewEmptyWhenNoSize(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.Show("query", nil)
	testkit.AssertEqual(t, "empty view when no size", m.View(), "")
}

func TestJQLModal_ViewWithHistoryItems(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1, testHistoryItem2})
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "History") {
		t.Errorf("expected 'History' title in view, got %q", plain)
	}
}

func TestJQLModal_ViewWithACLoadingState(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m.SetACLoading(true)
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "Loading") {
		t.Errorf("expected loading message in view, got %q", plain)
	}
}

func TestJQLModal_ViewWithNoItems(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "No history") {
		t.Errorf("expected 'No history' in view, got %q", plain)
	}
}

func TestJQLModal_ViewWithNoSuggestions(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m.SetSuggestions(nil)
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "No suggestions") {
		t.Errorf("expected 'No suggestions' in view, got %q", plain)
	}
}

func TestJQLModal_ViewWithError(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m.SetError("bad query")
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "bad query") {
		t.Errorf("expected error message in view, got %q", plain)
	}
}

func TestJQLModal_EscFromInputCancels(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	testkit.AssertEqual(t, "hidden after esc", m.IsVisible(), false)
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	if _, ok := cmd().(JQLCancelMsg); !ok {
		t.Error("expected JQLCancelMsg")
	}
}

func TestJQLModal_EnterSubmitsQuery(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show(testQueryText, nil)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg := cmd()
	submitted, ok := msg.(JQLSubmitMsg)
	if !ok {
		t.Fatalf("expected JQLSubmitMsg, got %T", msg)
	}
	testkit.AssertEqual(t, "submitted query", submitted.Query, testQueryText)
}

func TestJQLModal_EnterWithEmptyQueryIsNoop(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "visible (no submit)", m.IsVisible(), true)
	testkit.AssertEqual(t, "no command", cmd == nil, true)
}

func TestJQLModal_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1})
	testkit.AssertEqual(t, "initially focus input", m.focusInput, true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "focus moved to list", m.focusInput, false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "focus returned to input", m.focusInput, true)
}

func TestJQLModal_TabInsertsSuggestionWhenOnlyOne(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("status", nil)
	m.SetSuggestions([]string{"status = Open"})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "still focus input", m.focusInput, true)
	if cmd == nil {
		t.Fatal("expected changed command")
	}
	if _, ok := cmd().(JQLInputChangedMsg); !ok {
		t.Errorf("expected JQLInputChangedMsg, got %T", cmd())
	}
}

func TestJQLModal_ctrlS_emitsSaveMsg(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show(testQueryText, nil)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("expected save command")
	}
	msg, ok := cmd().(JQLSaveTabMsg)
	if !ok {
		t.Fatalf("expected JQLSaveTabMsg, got %T", cmd())
	}
	testkit.AssertEqual(t, "save query", msg.Query, testQueryText)
}

func TestJQLModal_ctrlS_emptyInputNoop(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	testkit.AssertEqual(t, "no command when empty", cmd == nil, true)
}

func TestJQLModal_ListNavWithJK(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1, testHistoryItem2})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "cursor initially 0", m.cursor, 0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	testkit.AssertEqual(t, "cursor moved down", m.cursor, 1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	testkit.AssertEqual(t, "cursor moved up", m.cursor, 0)
}

func TestJQLModal_ListNavTopBottom(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1, testHistoryItem2})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	testkit.AssertEqual(t, "cursor at bottom", m.cursor, 1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	testkit.AssertEqual(t, "cursor at top", m.cursor, 0)
}

func TestJQLModal_EnterOnHistoryItemSetsInput(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "input set to history item", m.InputValue(), testHistoryItem1)
	testkit.AssertEqual(t, "focus returned to input", m.focusInput, true)
}

func TestJQLModal_EnterOnSuggestionInsertsIt(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("status", nil)
	m.SetSuggestions([]string{"status = Open", "status = Closed"})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "focus returned to input", m.focusInput, true)
	if cmd == nil {
		t.Fatal("expected command from autocomplete insert")
	}
	if _, ok := cmd().(JQLInputChangedMsg); !ok {
		t.Errorf("expected JQLInputChangedMsg, got %T", cmd())
	}
}

func TestJQLModal_EscFromListReturnsFocusToInput(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	testkit.AssertEqual(t, "focus back to input", m.focusInput, true)
	testkit.AssertEqual(t, "still visible", m.IsVisible(), true)
}

func TestJQLModal_TypingGeneratesInputChangedMsg(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if cmd == nil {
		t.Fatal("expected command after typing")
	}
	if _, ok := cmd().(JQLInputChangedMsg); !ok {
		t.Errorf("expected JQLInputChangedMsg, got %T", cmd())
	}
	testkit.AssertEqual(t, "input contains typed char", m.InputValue(), "p")
}

func TestJQLModal_MouseWheelScrollsList(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	history := make([]string, 20)
	for i := range history {
		history[i] = testHistoryItem1
	}
	m.Show("", history)
	m, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	testkit.AssertEqual(t, "offset increased", m.offset, 1)
	m, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	testkit.AssertEqual(t, "offset back to 0", m.offset, 0)
}

func TestJQLModal_MouseClickSelectsItem(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", []string{testHistoryItem1, testHistoryItem2})
	m, _ = m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      5,
		Y:      6,
	})
	testkit.AssertEqual(t, "focus moved to list", m.focusInput, false)
}

func TestJQLModal_InterceptConsumesKeyWhenVisible(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	_, consumed := m.Intercept(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	testkit.AssertEqual(t, "key consumed", consumed, true)
}

func TestJQLModal_InterceptPassesThroughWhenHidden(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	_, consumed := m.Intercept(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	testkit.AssertEqual(t, "not consumed when hidden", consumed, false)
}

func TestJQLModal_RenderDrawsOnBackground(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("q", nil)
	bg := testkit.BlankCanvas(80, 24)
	out := m.Render(bg, 80, 24)
	if !strings.Contains(stripANSI(out), "JQL Query") {
		t.Errorf("expected modal content in render, got %q", stripANSI(out))
	}
}

func TestJQLModal_RenderInvisibleReturnsBackground(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	bg := "background"
	out := m.Render(bg, 80, 24)
	testkit.AssertEqual(t, "bg passthrough", out, bg)
}

func TestJQLModal_SelectedSuggestionEmpty_WhenFocusInput(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m.SetSuggestions([]string{"priority = High"})
	testkit.AssertEqual(t, "no selection when focused on input", m.SelectedSuggestion(), "")
}

func TestJQLModal_SelectedSuggestion_WhenFocusList(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("", nil)
	m.SetSuggestions([]string{"priority = High", "priority = Low"})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	sel := m.SelectedSuggestion()
	testkit.AssertEqual(t, "first suggestion selected", sel, "priority = High")
}

func TestJQLModal_LoadingBlocksSubmit(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show(testQueryText, nil)
	m.SetLoading(true)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "no command while loading", cmd == nil, true)
	testkit.AssertEqual(t, "still visible while loading", m.IsVisible(), true)
}

func TestJQLModal_ErrorClearedOnTyping(t *testing.T) {
	t.Parallel()
	m := NewJQLModal()
	m.SetSize(80, 24)
	m.Show("query", nil)
	m.SetError("oops")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	testkit.AssertEqual(t, "error cleared on typing", m.errorMsg, "")
}
