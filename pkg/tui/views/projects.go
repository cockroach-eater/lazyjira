package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cockroach-eater/lazyjira/pkg/jira"
	"github.com/cockroach-eater/lazyjira/pkg/tui/components"
	"github.com/cockroach-eater/lazyjira/pkg/tui/theme"
)

type ProjectSelectedMsg struct {
	ProjectKey string
}

type ProjectHoveredMsg struct {
	Project *jira.Project
}

type ProjectList struct {
	projects    []jira.Project
	allProjects []jira.Project
	filter      string
	cursor      int
	width       int
	height      int
	focused     bool
	theme       *theme.Theme
}

func NewProjectList() *ProjectList {
	return &ProjectList{theme: theme.DefaultTheme()}
}

func (p *ProjectList) SetProjects(projects []jira.Project) {
	p.allProjects = projects
	p.applyFilter()
}

func (p *ProjectList) SetFilter(query string) {
	p.filter = query
	p.applyFilter()
}

func (p *ProjectList) applyFilter() {
	if p.filter == "" {
		p.projects = p.allProjects
	} else {
		q := strings.ToLower(p.filter)
		var filtered []jira.Project
		for _, proj := range p.allProjects {
			haystack := strings.ToLower(proj.Key + " " + proj.Name)
			if strings.Contains(haystack, q) {
				filtered = append(filtered, proj)
			}
		}
		p.projects = filtered
	}
	p.cursor = 0
}

func (p *ProjectList) SetSize(w, h int)       { p.width = w; p.height = h }
func (p *ProjectList) SetFocused(focused bool) { p.focused = focused }
func (p *ProjectList) Init() tea.Cmd           { return nil }

func (p *ProjectList) ScrollBy(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.projects) {
		p.cursor = len(p.projects) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *ProjectList) ClickAt(relY int) {
	idx := relY - 1 // -1 for top border
	if idx >= 0 && idx < len(p.projects) {
		p.cursor = idx
	}
}

func (p *ProjectList) Update(msg tea.Msg) (*ProjectList, tea.Cmd) {
	if !p.focused {
		return p, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		prevCursor := p.cursor
		switch msg.String() {
		case "j", "down":
			if p.cursor < len(p.projects)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "enter", " ":
			if p.cursor >= 0 && p.cursor < len(p.projects) {
				selected := p.projects[p.cursor]
				return p, func() tea.Msg {
					return ProjectSelectedMsg{ProjectKey: selected.Key}
				}
			}
		}
		if prevCursor != p.cursor && p.cursor >= 0 && p.cursor < len(p.projects) {
			proj := p.projects[p.cursor]
			return p, func() tea.Msg {
				return ProjectHoveredMsg{Project: &proj}
			}
		}
	}
	return p, nil
}

func (p *ProjectList) View() string {
	contentWidth := p.width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}
	innerHeight := p.height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	var rows []string
	for i, proj := range p.projects {
		if i >= innerHeight {
			break
		}
		lead := ""
		if proj.Lead != nil {
			lead = " · " + proj.Lead.DisplayName
		}
		line := fmt.Sprintf(" %-8s %s", proj.Key, proj.Name)
		// Truncate name+lead to fit.
		maxName := contentWidth - 10 - len(lead)
		if maxName < 5 {
			lead = "" // drop lead if no space
			maxName = contentWidth - 10
		}
		namePart := proj.Name
		if len(namePart) > maxName {
			namePart = namePart[:maxName-1] + "…"
		}
		line = fmt.Sprintf(" %-8s %s%s", proj.Key, namePart, lead)
		if i == p.cursor {
			rows = append(rows, p.theme.SelectedItem.Width(contentWidth).Render(line))
		} else {
			rows = append(rows, p.theme.NormalItem.Width(contentWidth).Render(line))
		}
	}

	content := strings.Join(rows, "\n")
	footer := ""
	if len(p.projects) > 0 {
		footer = fmt.Sprintf("%d of %d", p.cursor+1, len(p.projects))
	}
	return components.RenderPanelWithFooter("[3] Projects", footer, content, p.width, innerHeight, p.focused)
}
