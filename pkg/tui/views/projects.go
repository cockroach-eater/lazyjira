package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

type ProjectSelectedMsg struct {
	ProjectKey string
}

type ProjectHoveredMsg struct {
	Project *jira.Project
}

type ProjectList struct {
	components.ListBase
	projects    []jira.Project
	allProjects []jira.Project
	filter      string
	activeKey   string // the project currently in use
	theme       *theme.Theme
}

func NewProjectList() *ProjectList {
	return &ProjectList{theme: theme.Default}
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
	p.Cursor = 0
	p.Offset = 0
	p.SetItemCount(len(p.projects))
}

func (p *ProjectList) SetActiveKey(key string) { p.activeKey = key }

func (p *ProjectList) ContentHeight() int {
	return p.ListBase.ContentHeight(5)
}

func (p *ProjectList) SelectedProject() *jira.Project {
	if p.Cursor >= 0 && p.Cursor < len(p.projects) {
		proj := p.projects[p.Cursor]
		return &proj
	}
	return nil
}
func (p *ProjectList) Init() tea.Cmd { return nil }

func (p *ProjectList) Update(msg tea.Msg) (*ProjectList, tea.Cmd) {
	if !p.Focused {
		return p, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		if p.KeyNav(msg.String()) {
			if proj := p.SelectedProject(); proj != nil {
				return p, func() tea.Msg {
					return ProjectHoveredMsg{Project: proj}
				}
			}
		}
	}
	return p, nil
}

func (p *ProjectList) View() string {
	if p.Height <= 1 {
		footer := ""
		if n := len(p.projects); n > 0 {
			footer = fmt.Sprintf("%d of %d", p.Cursor+1, n)
		}
		return components.RenderCollapsedBar("[3] Projects", footer, p.Width, p.Focused)
	}

	contentWidth, innerHeight := components.PanelDimensions(p.Width, p.Height)

	var rows []string
	end := min(p.Offset+innerHeight, len(p.projects))
	for i := p.Offset; i < end; i++ {
		proj := p.projects[i]
		lead := ""
		if proj.Lead != nil {
			lead = " · " + proj.Lead.DisplayName
		}
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
		active := proj.Key == p.activeKey
		markerChar := " "
		if active {
			markerChar = "*"
		}

		line := fmt.Sprintf("%s%-8s %s%s", markerChar, proj.Key, namePart, lead)
		switch {
		case i == p.Cursor && p.Focused:
			rows = append(rows, p.theme.SelectedItem.Width(contentWidth).Render(line))
		case active:
			coloredMarker := lipgloss.NewStyle().Foreground(theme.ColorGreen).Render(markerChar)
			rest := fmt.Sprintf("%-8s %s%s", proj.Key, namePart, lead)
			rows = append(rows, p.theme.NormalItem.Width(contentWidth).Render(coloredMarker+rest))
		default:
			rows = append(rows, p.theme.NormalItem.Width(contentWidth).Render(line))
		}
	}

	content := strings.Join(rows, "\n")
	footer := ""
	if len(p.projects) > 0 {
		footer = fmt.Sprintf("%d of %d", p.Cursor+1, len(p.projects))
	}
	scroll := &components.ScrollInfo{Total: len(p.projects), Visible: innerHeight, Offset: p.Offset}
	return components.RenderPanelFull("[3] Projects", footer, content, p.Width, innerHeight, p.Focused, scroll)
}
