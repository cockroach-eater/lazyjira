package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchBar is a bottom search input that filters the current panel
type SearchBar struct {
	input  TextInput
	active bool
}

func NewSearchBar() SearchBar {
	return SearchBar{}
}

func (s *SearchBar) SetWidth(w int)   { s.input.SetWidth(w) }
func (s *SearchBar) IsActive() bool   { return s.active }
func (s *SearchBar) Query() string    { return s.input.Value() }
func (s *SearchBar) Activate()        { s.active = true; s.input.SetValue("") }
func (s *SearchBar) Deactivate()      { s.active = false; s.input.SetValue("") }

// SearchChangedMsg is sent when the search query changes
type SearchChangedMsg struct{ Query string }

// SearchConfirmedMsg is sent when the user presses Enter.
type SearchConfirmedMsg struct{ Query string }

// SearchCancelledMsg is sent when the user presses Esc.
type SearchCancelledMsg struct{}

func (s *SearchBar) Update(msg tea.Msg) (SearchBar, tea.Cmd) {
	if !s.active {
		return *s, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyEnter:
			s.active = false
			q := s.input.Value()
			return *s, func() tea.Msg { return SearchConfirmedMsg{Query: q} }
		case tea.KeyEsc:
			s.active = false
			s.input.SetValue("")
			return *s, func() tea.Msg { return SearchCancelledMsg{} }
		default:
			updated, changed := s.input.Update(msg)
			s.input = updated
			if changed {
				q := s.input.Value()
				return *s, func() tea.Msg { return SearchChangedMsg{Query: q} }
			}
		}
	}
	return *s, nil
}

func (s *SearchBar) View() string {
	if !s.active {
		return ""
	}
	return RenderFilterBarInput(&s.input)
}

// RenderFilterBarInput renders filter bar using TextInput (with cursor positioning)
func RenderFilterBarInput(input *TextInput) string {
	prefixStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)

	return prefixStyle.Render(" /") + input.View()
}
