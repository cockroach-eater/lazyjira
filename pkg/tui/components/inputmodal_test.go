package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
)

func TestInputModal_ShowAndIsVisible(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	testkit.AssertEqual(t, "initially invisible", m.IsVisible(), false)
	m.Show(testTitle, "prefill")
	testkit.AssertEqual(t, "visible after Show", m.IsVisible(), true)
}

func TestInputModal_ShowPrefillsSummary(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "prefill text")
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "prefill text") {
		t.Errorf("expected prefill in view, got %q", plain)
	}
}

func TestInputModal_HideHidesModal(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.Show(testTitle, "")
	m.Hide()
	testkit.AssertEqual(t, "hidden after Hide", m.IsVisible(), false)
}

func TestInputModal_HasHints(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.Show(testTitle, "")
	testkit.AssertEqual(t, "no hints initially", m.HasHints(), false)
	m.SetHints([]string{testBranchName1, testBranchName2})
	testkit.AssertEqual(t, "has hints after SetHints", m.HasHints(), true)
}

func TestInputModal_SetSize(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(100, 30)
	testkit.AssertEqual(t, "width stored", m.width, 100)
	testkit.AssertEqual(t, "height stored", m.height, 30)
}

func TestInputModal_EnterConfirmsText(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "hello")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "hidden after confirm", m.IsVisible(), false)
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	confirmed, ok := msg.(InputConfirmedMsg)
	if !ok {
		t.Fatalf("expected InputConfirmedMsg, got %T", msg)
	}
	testkit.AssertEqual(t, "confirmed text", confirmed.Text, "hello")
}

func TestInputModal_EscCancels(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	testkit.AssertEqual(t, "hidden after esc", m.IsVisible(), false)
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	if _, ok := cmd().(InputCancelledMsg); !ok {
		t.Error("expected InputCancelledMsg")
	}
}

func TestInputModal_TextEditingKeys(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "hello")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	testkit.AssertEqual(t, "backspace removes char", string(m.text), "hell")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	testkit.AssertEqual(t, "delete removes char at cursor", string(m.text), "hel")
}

func TestInputModal_HomeEndCtrlAE(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "hello")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	testkit.AssertEqual(t, "cursor at start after Home", m.cursor, 0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	testkit.AssertEqual(t, "cursor at end after End", m.cursor, 5)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	testkit.AssertEqual(t, "cursor at start after CtrlA", m.cursor, 0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	testkit.AssertEqual(t, "cursor at end after CtrlE", m.cursor, 5)
}

func TestInputModal_CtrlUK(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "hello world")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m.cursor = 5
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	testkit.AssertEqual(t, "ctrl+k kills to end", string(m.text), "hello")
	m.Show(testTitle, "hello world")
	m.cursor = 6
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	testkit.AssertEqual(t, "ctrl+u kills to start", string(m.text), "world")
}

func TestInputModal_SpaceAndRunes(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "hi")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	testkit.AssertEqual(t, "space inserts space", string(m.text), "hi ")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	testkit.AssertEqual(t, "runes inserted", string(m.text), "hi !")
}

func TestInputModal_ViewRendersTitle(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, testTitle) {
		t.Errorf("expected title %q in view, got %q", testTitle, plain)
	}
}

func TestInputModal_ViewInvisibleIsEmpty(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	testkit.AssertEqual(t, "empty view when invisible", m.View(), "")
}

func TestInputModal_InterceptConsumesKeyWhenVisible(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	_, consumed := m.Intercept(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	testkit.AssertEqual(t, "key consumed", consumed, true)
}

func TestInputModal_InterceptPassesThroughWhenHidden(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	_, consumed := m.Intercept(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	testkit.AssertEqual(t, "not consumed when hidden", consumed, false)
}

func TestInputModal_RenderDrawsOnBackground(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "some text")
	bg := testkit.BlankCanvas(80, 24)
	out := m.Render(bg, 80, 24)
	if !strings.Contains(stripANSI(out), testTitle) {
		t.Errorf("expected title in rendered output, got %q", stripANSI(out))
	}
}

func TestInputModal_RenderInvisibleReturnsBackground(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	bg := testBackground
	out := m.Render(bg, 80, 24)
	testkit.AssertEqual(t, "bg passthrough", out, bg)
}

func TestInputModal_HintViewEmptyWhenNoHints(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	testkit.AssertEqual(t, "empty hint view", m.HintView(), "")
}

func TestInputModal_HintViewShowsHints(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m.SetHints([]string{testBranchName1, testBranchName2})
	hint := m.HintView()
	plain := stripANSI(hint)
	if !strings.Contains(plain, testBranchName1) {
		t.Errorf("expected hint item in hint view, got %q", plain)
	}
}

func TestInputModal_TabTogglesFocusWhenHintsPresent(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m.SetHints([]string{testBranchName1})
	testkit.AssertEqual(t, "initially focus input", m.focusInput, true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "focus moved to hints", m.focusInput, false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "focus moved back to input", m.focusInput, true)
}

func TestInputModal_HintKeyNavigationJK(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m.SetHints([]string{testBranchName1, testBranchName2})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	testkit.AssertEqual(t, "hint cursor moved down", m.hintCursor, 1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	testkit.AssertEqual(t, "hint cursor moved up", m.hintCursor, 0)
}

func TestInputModal_HintEnterSelectsHint(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m.SetHints([]string{testBranchName1, testBranchName2})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "hidden after hint enter", m.IsVisible(), false)
	if cmd == nil {
		t.Fatal("expected command from hint Enter")
	}
	confirmed, ok := cmd().(InputConfirmedMsg)
	if !ok {
		t.Fatalf("expected InputConfirmedMsg, got %T", cmd())
	}
	testkit.AssertEqual(t, "confirmed hint text", confirmed.Text, testBranchName1)
}

func TestInputModal_HintEscCancels(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m.SetHints([]string{testBranchName1})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	testkit.AssertEqual(t, "hidden after hint esc", m.IsVisible(), false)
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	if _, ok := cmd().(InputCancelledMsg); !ok {
		t.Error("expected InputCancelledMsg")
	}
}

func TestInputModal_HintQCancels(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m.SetSize(80, 24)
	m.Show(testTitle, "")
	m.SetHints([]string{testBranchName1})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	testkit.AssertEqual(t, "hidden after q", m.IsVisible(), false)
	if cmd == nil {
		t.Fatal("expected cancel command from q")
	}
	if _, ok := cmd().(InputCancelledMsg); !ok {
		t.Error("expected InputCancelledMsg from q")
	}
}

func TestInputModal_UpdateInvisibleIsNoop(t *testing.T) {
	t.Parallel()
	m := NewInputModal()
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	testkit.AssertEqual(t, "still invisible", m2.IsVisible(), false)
	testkit.AssertEqual(t, "no cmd", cmd == nil, true)
}
