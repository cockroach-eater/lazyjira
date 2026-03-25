package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

type StatusPanel struct {
	project string
	user    string
	host    string
	online  bool
	width int
	height  int
	focused bool
	theme   *theme.Theme
}

func NewStatusPanel(project, user, host string) *StatusPanel {
	return &StatusPanel{
		project: project,
		user:    user,
		host:    host,
		online:  true,
		theme:   theme.Default,
	}
}

func (s *StatusPanel) SetProject(project string) { s.project = project }
func (s *StatusPanel) SetOnline(online bool) { s.online = online }
func (s *StatusPanel) SetSize(w, h int)          { s.width = w; s.height = h }
func (s *StatusPanel) SetFocused(focused bool)   { s.focused = focused }

func (s *StatusPanel) Init() tea.Cmd                              { return nil }
func (s *StatusPanel) Update(msg tea.Msg) (*StatusPanel, tea.Cmd) { return s, nil }

func (s *StatusPanel) View() string {
	if s.height <= 1 {
		return components.RenderCollapsedBar("[1] Status", s.project, s.width, s.focused)
	}

	_, innerHeight := components.PanelDimensions(s.width, s.height)

	indicator := theme.StatusColor("done").Render("✓")
	if !s.online {
		indicator = theme.StatusColor("").Render("✗")
	}

	// Shrink email to fit: " ✓ email → PROJECT" must fit in width-2
	contentW := s.width - 2
	fixedChars := 3 + 1 + 3 + len(s.project) // " ✓ " + " → " + project
	maxEmail := contentW - fixedChars
	email := s.user
	if maxEmail > 5 && len(email) > maxEmail {
		side := (maxEmail - 3) / 2
		email = email[:side+1] + "..." + email[len(email)-side:]
	}
	line := fmt.Sprintf("%s %s → %s", indicator, email, s.project)
	return components.RenderPanel("[1] Status", line, s.width, innerHeight, s.focused)
}
