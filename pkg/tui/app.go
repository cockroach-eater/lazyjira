package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cockroach-eater/lazyjira/pkg/config"
	"github.com/cockroach-eater/lazyjira/pkg/jira"
	"github.com/cockroach-eater/lazyjira/pkg/tui/components"
	"github.com/cockroach-eater/lazyjira/pkg/tui/views"
)

type focusPanel int

const (
	focusStatus   focusPanel = iota
	focusIssues
	focusProjects
)

type focusSide int

const (
	sideLeft focusSide = iota
	sideRight
)

// Async messages.
type issuesLoadedMsg struct{ issues []jira.Issue }
type issueDetailLoadedMsg struct{ issue *jira.Issue }
type transitionDoneMsg struct{}
type errorMsg struct{ err error }
type projectsLoadedMsg struct{ projects []jira.Project }
type issuePrefetchedMsg struct {
	issue *jira.Issue
}
type autoFetchTickMsg struct{}
type transitionsLoadedMsg struct {
	issueKey    string
	transitions []jira.Transition
}

type App struct {
	cfg        *config.Config
	client     *jira.Client
	splashInfo views.SplashInfo

	statusPanel *views.StatusPanel
	issuesList  *views.IssuesList
	projectList *views.ProjectList
	detailView  *views.DetailView
	logPanel    *views.LogPanel

	helpBar   components.HelpBar
	searchBar components.SearchBar
	modal     components.Modal

	side        focusSide
	leftFocus   focusPanel
	projectKey  string
	showHelp    bool // ? popup visible
	issueCache  map[string]*jira.Issue

	width  int
	height int
	err    error
}

// AuthMethod describes how the user authenticated.
type AuthMethod string

const (
	AuthSaved AuthMethod = "Saved credentials (auth.json)"
	AuthEnv   AuthMethod = "Environment variables"
	AuthWizard AuthMethod = "Setup wizard"
)

func NewApp(cfg *config.Config, client *jira.Client) *App {
	return NewAppWithAuth(cfg, client, AuthEnv)
}

func NewAppWithAuth(cfg *config.Config, client *jira.Client, authMethod AuthMethod) *App {
	projectKey := ""
	if len(cfg.Projects) > 0 {
		projectKey = cfg.Projects[0].Key
	}

	statusPanel := views.NewStatusPanel(projectKey, cfg.Jira.Email, cfg.Jira.Host)
	issuesList := views.NewIssuesList()
	issuesList.SetFocused(true)
	issuesList.SetUserEmail(cfg.Jira.Email)
	projectList := views.NewProjectList()
	detailView := views.NewDetailView()
	logPanel := views.NewLogPanel()
	helpBar := components.NewHelpBar(nil)
	searchBar := components.NewSearchBar()
	modal := components.NewModal()

	client.SetOnRequest(func(rl jira.RequestLog) {
		logPanel.AddEntry(views.LogEntry{
			Time:    time.Now(),
			Method:  rl.Method,
			Path:    rl.Path,
			Status:  rl.Status,
			Elapsed: rl.Elapsed,
		})
	})

	splash := views.SplashInfo{
		AuthMethod: string(authMethod),
		Host:       cfg.Jira.Host,
		Email:      cfg.Jira.Email,
		Project:    projectKey,
	}

	app := &App{
		cfg:        cfg,
		client:     client,
		splashInfo: splash,
		statusPanel: statusPanel,
		issuesList:  issuesList,
		projectList: projectList,
		detailView:  detailView,
		logPanel:    logPanel,
		helpBar:     helpBar,
		searchBar:   searchBar,
		modal:       modal,
		side:        sideLeft,
		leftFocus:   focusIssues,
		projectKey:  projectKey,
		issueCache:  make(map[string]*jira.Issue),
	}
	app.helpBar.SetItems(app.helpBarItems())
	return app
}

func (a *App) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, fetchProjects(a.client))
	if a.projectKey != "" {
		cmds = append(cmds, fetchIssues(a.client, a.projectKey))
	}
	// Start autofetch timer.
	cmds = append(cmds, tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return autoFetchTickMsg{}
	}))
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Search bar intercepts all keys when active.
	if a.searchBar.IsActive() {
		if _, ok := msg.(tea.KeyMsg); ok {
			updated, cmd := a.searchBar.Update(msg)
			a.searchBar = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
	}

	// Modal intercepts all keys when visible.
	if a.modal.IsVisible() {
		if _, ok := msg.(tea.KeyMsg); ok {
			updated, cmd := a.modal.Update(msg)
			a.modal = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.modal.SetSize(msg.Width, msg.Height)
		a.layoutPanels()
		a.helpBar.SetWidth(msg.Width)
		a.searchBar.SetWidth(msg.Width)
		return a, nil

	case tea.MouseMsg:
		return a.handleMouse(msg)

	case components.SearchChangedMsg:
		// Filter only the active panel.
		if a.side == sideLeft && a.leftFocus == focusIssues {
			a.issuesList.SetFilter(msg.Query)
		} else if a.side == sideLeft && a.leftFocus == focusProjects {
			a.projectList.SetFilter(msg.Query)
		}
		return a, nil

	case components.SearchConfirmedMsg:
		var cmd tea.Cmd
		if a.side == sideLeft && a.leftFocus == focusIssues {
			selectedIssue := a.issuesList.SelectedIssue()
			a.issuesList.ClearFilter()
			if selectedIssue != nil {
				a.issuesList.SelectByKey(selectedIssue.Key)
				cmds = append(cmds, fetchIssueDetail(a.client, selectedIssue.Key))
			}
		} else if a.side == sideLeft && a.leftFocus == focusProjects {
			updated, c := a.projectList.Update(tea.KeyMsg{Type: tea.KeyEnter})
			a.projectList = updated
			cmd = c
			a.projectList.SetFilter("")
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case components.SearchCancelledMsg:
		a.issuesList.SetFilter("")
		a.projectList.SetFilter("")
		return a, nil

	case tea.KeyMsg:
		// Help popup: any key closes it (except / for search).
		if a.showHelp {
			if msg.String() == "/" {
				a.showHelp = false
				a.searchBar.Activate()
				return a, nil
			}
			a.showHelp = false
			return a, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit

		case "?":
			a.showHelp = true
			return a, nil

		case "/":
			a.searchBar.Activate()
			return a, nil

		case "tab":
			if a.side == sideLeft {
				a.side = sideRight
			} else {
				a.side = sideLeft
			}
			a.updateFocusState()
			return a, nil

		case "l", "right":
			if a.side == sideLeft {
				a.side = sideRight
				a.updateFocusState()
				return a, nil
			}

		case "h", "left", "esc":
			if a.side == sideRight {
				a.side = sideLeft
				a.updateFocusState()
				return a, nil
			}

		case "enter", " ":
			if a.side == sideLeft && a.leftFocus == focusIssues {
				a.side = sideRight
				a.updateFocusState()
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					return a, fetchIssueDetail(a.client, sel.Key)
				}
				return a, nil
			}

		case "[":
			if a.side == sideRight {
				a.detailView.PrevTab()
			} else if a.side == sideLeft && a.leftFocus == focusIssues {
				a.issuesList.PrevTab()
			}
			return a, nil
		case "]":
			if a.side == sideRight {
				a.detailView.NextTab()
			} else if a.side == sideLeft && a.leftFocus == focusIssues {
				a.issuesList.NextTab()
			}
			return a, nil

		case "0":
			a.side = sideRight
			a.updateFocusState()
			return a, nil

		case "1":
			a.side = sideLeft
			a.leftFocus = focusStatus
			// Update splash with current project.
			a.splashInfo.Project = a.projectKey
			a.detailView.SetSplash(a.splashInfo)
			a.updateFocusState()
			return a, nil
		case "2":
			a.side = sideLeft
			a.leftFocus = focusIssues
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				if cached, ok := a.issueCache[sel.Key]; ok {
					a.detailView.SetIssue(cached)
				}
			}
			a.updateFocusState()
			return a, nil
		case "3":
			a.side = sideLeft
			a.leftFocus = focusProjects
			a.updateFocusState()
			return a, nil

		case "y":
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				copyToClipboard(a.cfg.Jira.Host + "/browse/" + sel.Key)
			}
			return a, nil

		case "o":
			if sel := a.issuesList.SelectedIssue(); sel != nil && (a.leftFocus == focusIssues || a.side == sideRight) {
				openBrowser(a.cfg.Jira.Host + "/browse/" + sel.Key)
			}
			return a, nil

		case "u":
			// URL picker modal.
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				cached := sel
				if c, ok := a.issueCache[sel.Key]; ok {
					cached = c
				}
				urls := views.ExtractURLs(cached, a.cfg.Jira.Host)
				if len(urls) > 0 {
					var items []components.ModalItem
					for _, u := range urls {
						if key := a.extractIssueKey(u); key != "" {
							// Jira issue — show key with marker.
							items = append(items, components.ModalItem{ID: u, Label: key, Internal: true})
						} else {
							items = append(items, components.ModalItem{ID: u, Label: ellipsisMiddle(u, 50)})
						}
					}
					a.modal.SetSize(a.width, a.height)
					a.modal.Show("URLs", items)
				}
			}
			return a, nil

		case "t":
			// Transition: fetch available transitions for selected issue.
			if a.side == sideLeft && a.leftFocus == focusIssues {
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					return a, fetchTransitions(a.client, sel.Key)
				}
			}
			return a, nil

		case "r":
			// Refresh current issue detail.
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return a, fetchIssueDetail(a.client, sel.Key)
			}
			return a, nil

		case "R":
			// Full refresh: issues list + all details.
			if a.projectKey != "" {
				return a, fetchIssues(a.client, a.projectKey)
			}
			return a, nil
		}

	case autoFetchTickMsg:
		var fetchCmds []tea.Cmd
		if a.projectKey != "" {
			fetchCmds = append(fetchCmds, fetchIssues(a.client, a.projectKey))
		}
		// Schedule next tick.
		fetchCmds = append(fetchCmds, tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
			return autoFetchTickMsg{}
		}))
		return a, tea.Batch(fetchCmds...)

	case issuesLoadedMsg:
		a.err = nil
		a.statusPanel.SetOnline(true)
		a.issueCache = make(map[string]*jira.Issue) // clear cache on refresh
		a.issuesList.SetIssues(msg.issues)
		// Prefetch all issue details in background.
		for _, issue := range msg.issues {
			cmds = append(cmds, prefetchIssue(a.client, issue.Key))
		}
		return a, tea.Batch(cmds...)

	case issueDetailLoadedMsg:
		if msg.issue != nil {
			a.issueCache[msg.issue.Key] = msg.issue
		}
		if a.leftFocus == focusIssues || a.side == sideRight {
			a.detailView.SetIssue(msg.issue)
		} else {
			a.detailView.UpdateIssueData(msg.issue)
		}
		return a, nil

	case issuePrefetchedMsg:
		if msg.issue != nil {
			a.issueCache[msg.issue.Key] = msg.issue
			// If this is the currently selected issue, update detail.
			if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == msg.issue.Key {
				if a.leftFocus == focusIssues || a.side == sideRight {
					a.detailView.SetIssue(msg.issue)
				}
			}
		}
		return a, nil

	case transitionDoneMsg:
		if a.projectKey != "" {
			var fetchCmds []tea.Cmd
			fetchCmds = append(fetchCmds, fetchIssues(a.client, a.projectKey))
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				fetchCmds = append(fetchCmds, fetchIssueDetail(a.client, sel.Key))
			}
			return a, tea.Batch(fetchCmds...)
		}
		return a, nil

	case transitionsLoadedMsg:
		if len(msg.transitions) == 0 {
			return a, nil
		}
		var items []components.ModalItem
		for _, t := range msg.transitions {
			label := t.Name
			if t.To != nil {
				label += " → " + t.To.Name
			}
			items = append(items, components.ModalItem{ID: t.ID, Label: label})
		}
		a.modal.SetSize(a.width, a.height)
		a.modal.Show("Transition: "+msg.issueKey, items)
		return a, nil

	case components.ModalSelectedMsg:
		id := msg.Item.ID
		if strings.HasPrefix(id, "http") {
			// Check if it's a Jira issue URL on our host.
			if issueKey := a.extractIssueKey(id); issueKey != "" {
				// Navigate to the issue in [2].
				a.navigateToIssue(issueKey)
				return a, nil
			}
			// External URL — open in browser.
			openBrowser(id)
		} else {
			// Transition picker — execute transition.
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return a, doTransition(a.client, sel.Key, id)
			}
		}
		return a, nil

	case components.ModalCancelledMsg:
		return a, nil

	case errorMsg:
		a.err = msg.err
		a.statusPanel.SetOnline(false)
		return a, nil

	case views.IssueSelectedMsg:
		if msg.Issue != nil {
			// Use cache — instant render, no API call.
			if cached, ok := a.issueCache[msg.Issue.Key]; ok {
				a.detailView.SetIssue(cached)
			} else {
				// Fallback: show basic info from list, detail will arrive from prefetch.
				a.detailView.SetIssue(msg.Issue)
			}
		}
		return a, nil

	case projectsLoadedMsg:
		projects := msg.projects
		// Move last-used project to top.
		if creds, err := config.LoadCredentials(); err == nil && creds != nil && creds.LastProject != "" {
			for i, p := range projects {
				if p.Key == creds.LastProject {
					// Swap to front.
					projects[0], projects[i] = projects[i], projects[0]
					break
				}
			}
		}
		a.projectList.SetProjects(projects)
		if a.projectKey == "" && len(projects) > 0 {
			a.projectKey = projects[0].Key
			a.statusPanel.SetProject(a.projectKey)
			return a, fetchIssues(a.client, a.projectKey)
		}
		return a, nil

	case views.ProjectHoveredMsg:
		if msg.Project != nil {
			a.detailView.SetProject(msg.Project)
		}
		return a, nil

	case views.ProjectSelectedMsg:
		a.projectKey = msg.ProjectKey
		a.statusPanel.SetProject(msg.ProjectKey)
		a.leftFocus = focusIssues
		a.updateFocusState()
		// Save last project for next launch.
		go saveLastProject(msg.ProjectKey)
		return a, fetchIssues(a.client, a.projectKey)
	}

	// Route input to focused panel.
	if a.side == sideLeft {
		switch a.leftFocus {
		case focusIssues:
			updated, cmd := a.issuesList.Update(msg)
			a.issuesList = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case focusProjects:
			updated, cmd := a.projectList.Update(msg)
			a.projectList = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case focusStatus:
			updated, cmd := a.statusPanel.Update(msg)
			a.statusPanel = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	} else {
		updated, cmd := a.detailView.Update(msg)
		a.detailView = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	sideW := a.sideWidth()

	x, y := msg.X, msg.Y

	switch msg.Type {
	case tea.MouseWheelUp:
		if x < sideW {
			a.focusLeftPanelAt(y)
			return a.scrollFocused(-3)
		}
		a.side = sideRight
		a.updateFocusState()
		return a.scrollFocused(-3)

	case tea.MouseWheelDown:
		if x < sideW {
			a.focusLeftPanelAt(y)
			return a.scrollFocused(3)
		}
		a.side = sideRight
		a.updateFocusState()
		return a.scrollFocused(3)

	case tea.MouseLeft:
		if x < sideW {
			a.side = sideLeft
			a.focusLeftPanelAt(y)
			a.updateFocusState()
			// Click on an item in the list.
			if a.leftFocus == focusIssues {
				a.issuesList.ClickAt(y - a.statusPanelHeight())
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					return a, fetchIssueDetail(a.client, sel.Key)
				}
			} else if a.leftFocus == focusProjects {
				a.projectList.ClickAt(y - a.statusPanelHeight() - a.issuesPanelHeight())
			}
		} else {
			a.side = sideRight
			a.updateFocusState()
			// Check if click is on title bar (y == 0) → tab click.
			if y == 0 {
				relX := x - sideW
				a.detailView.ClickTab(relX)
			}
		}
		return a, nil
	}

	return a, nil
}

func (a *App) scrollFocused(delta int) (tea.Model, tea.Cmd) {
	if a.side == sideLeft && a.leftFocus == focusIssues {
		a.issuesList.ScrollBy(delta)
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			return a, fetchIssueDetail(a.client, sel.Key)
		}
	} else if a.side == sideLeft && a.leftFocus == focusProjects {
		a.projectList.ScrollBy(delta)
	} else if a.side == sideRight {
		a.detailView.ScrollBy(delta)
	}
	return a, nil
}

func (a *App) statusPanelHeight() int { return 3 }

func (a *App) issuesPanelHeight() int {
	totalH := a.height - 1
	remaining := totalH - 3 // minus status
	switch a.leftFocus {
	case focusIssues:
		return remaining - 5
	case focusProjects:
		return 5
	default:
		return remaining / 2
	}
}

func (a *App) focusLeftPanelAt(y int) {
	statusH := a.statusPanelHeight()
	issuesH := a.issuesPanelHeight()

	if y < statusH {
		a.leftFocus = focusStatus
	} else if y < statusH+issuesH {
		a.leftFocus = focusIssues
	} else {
		a.leftFocus = focusProjects
	}
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	if a.isVerticalLayout() {
		// Vertical: all stacked.
		var detailArea string
		if a.modal.IsVisible() {
			detailH := a.height - 1 - 3 - 5 - 3 - 5 // rough: total - status - issues - projects - log
			if detailH < 5 {
				detailH = 5
			}
			detailArea = lipgloss.Place(a.width, detailH,
				lipgloss.Center, lipgloss.Center,
				a.modal.View(),
			)
		} else {
			detailArea = a.detailView.View()
		}
		content = lipgloss.JoinVertical(lipgloss.Left,
			a.statusPanel.View(),
			a.issuesList.View(),
			a.projectList.View(),
			detailArea,
			a.logPanel.View(),
		)
	} else {
		leftCol := lipgloss.JoinVertical(lipgloss.Left,
			a.statusPanel.View(),
			a.issuesList.View(),
			a.projectList.View(),
		)

		var detailArea string
		if a.modal.IsVisible() {
			sideW := a.sideWidth()
			mainW := a.width - sideW
			detailH := a.height - 1 - 8
			if detailH < 5 {
				detailH = 5
			}
			popup := a.modal.View()
			detailArea = lipgloss.Place(mainW, detailH,
				lipgloss.Center, lipgloss.Center,
				popup,
			)
		} else {
			detailArea = a.detailView.View()
		}
		rightCol := lipgloss.JoinVertical(lipgloss.Left,
			detailArea,
			a.logPanel.View(),
		)

		content = lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
	}

	if a.err != nil {
		errLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true).
			Width(a.width).
			Render(fmt.Sprintf(" Error: %v", a.err))
		content = lipgloss.JoinVertical(lipgloss.Left, content, errLine)
	}

	var bottomBar string
	if a.searchBar.IsActive() {
		bottomBar = a.searchBar.View()
	} else {
		bottomBar = a.helpBar.View()
	}

	full := lipgloss.JoinVertical(lipgloss.Left, content, bottomBar)

	// Overlays.
	if a.showHelp {
		full = a.renderHelpOverlay(full)
	}
	// Modal is rendered inline in the right column, not as overlay.

	return full
}

func (a *App) renderHelpOverlay(base string) string {
	bindings := a.ContextBindings()

	maxKey := 0
	for _, b := range bindings {
		if len(b.Key) > maxKey {
			maxKey = len(b.Key)
		}
	}

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	descStyle := lipgloss.NewStyle()

	var lines []string
	lines = append(lines, "")
	for _, b := range bindings {
		padded := b.Key
		for len(padded) < maxKey {
			padded += " "
		}
		lines = append(lines, "  "+keyStyle.Render(padded)+"  "+descStyle.Render(b.Description))
	}
	lines = append(lines, "")
	lines = append(lines, "  "+lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("press any key to close"))

	popupContent := strings.Join(lines, "\n")

	popupW := maxKey + 40
	if popupW > a.width-4 {
		popupW = a.width - 4
	}
	popupH := len(lines)
	if popupH > a.height-4 {
		popupH = a.height - 4
	}

	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("2")).
		Width(popupW).
		Height(popupH).
		Render(popupContent)

	// Center the popup.
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, popup,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)
}

// extractIssueKey checks if a URL points to our Jira and extracts the issue key.
// e.g. https://didlogic.atlassian.net/browse/DR-13819 → "DR-13819"
func (a *App) extractIssueKey(url string) string {
	host := strings.TrimRight(a.cfg.Jira.Host, "/")
	prefix := host + "/browse/"
	if strings.HasPrefix(url, prefix) {
		key := strings.TrimPrefix(url, prefix)
		// Strip any trailing query params or fragments.
		if idx := strings.IndexAny(key, "?#&/"); idx != -1 {
			key = key[:idx]
		}
		if key != "" {
			return key
		}
	}
	return ""
}

// navigateToIssue switches to the issue in the issues list.
// If found in current tab (All/Assigned), selects it there.
// If not, switches to All tab and tries again.
func (a *App) navigateToIssue(key string) {
	// Try current tab first.
	if a.issuesList.SelectByKey(key) {
		a.side = sideLeft
		a.leftFocus = focusIssues
		a.updateFocusState()
		if cached, ok := a.issueCache[key]; ok {
			a.detailView.SetIssue(cached)
		}
		return
	}
	// Switch to All tab and try again.
	if a.issuesList.GetTab() != views.IssueTabAll {
		a.issuesList.SetTab(views.IssueTabAll)
		if a.issuesList.SelectByKey(key) {
			a.side = sideLeft
			a.leftFocus = focusIssues
			a.updateFocusState()
			if cached, ok := a.issueCache[key]; ok {
				a.detailView.SetIssue(cached)
			}
			return
		}
	}
	// Not in our list — open in browser as fallback.
	openBrowser(a.cfg.Jira.Host + "/browse/" + key)
}

func saveLastProject(projectKey string) {
	creds, err := config.LoadCredentials()
	if err != nil || creds == nil {
		return
	}
	creds.LastProject = projectKey
	config.SaveCredentials(creds)
}

// ellipsisMiddle truncates a string keeping start and end visible: "abcdef...xyz"
func ellipsisMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 5 {
		return s[:maxLen]
	}
	side := (maxLen - 3) / 2
	return s[:side+1] + "..." + s[len(s)-side:]
}

func copyToClipboard(text string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	cmd.Stdin = strings.NewReader(text)
	cmd.Run()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// isVerticalLayout returns true when terminal is too narrow for side-by-side.
func (a *App) isVerticalLayout() bool {
	return a.width < 80
}

// sideWidth calculates the left panel width, shrinking for narrow terminals.
func (a *App) sideWidth() int {
	if a.isVerticalLayout() {
		return a.width
	}
	sideW := a.cfg.GUI.SidePanelWidth
	if sideW <= 0 {
		sideW = 40
	}
	// Shrink side panel for medium terminals to fit [0] tabs.
	if a.width < 120 && sideW > a.width*35/100 {
		sideW = a.width * 35 / 100
	}
	if sideW > a.width/2 {
		sideW = a.width / 2
	}
	if sideW < 25 {
		sideW = 25
	}
	return sideW
}

func (a *App) layoutPanels() {
	totalH := a.height - 1

	if a.isVerticalLayout() {
		// Vertical: all panels stacked, full width. Accordion-style.
		w := a.width
		statusH := 3
		logH := 5
		remaining := totalH - statusH - logH

		var issuesH, projectsH, detailH int
		collapsed := 3

		if a.side == sideRight {
			// Detail focused — issues and projects collapse.
			issuesH = collapsed
			projectsH = collapsed
			detailH = remaining - issuesH - projectsH
		} else {
			switch a.leftFocus {
			case focusIssues:
				projectsH = collapsed
				detailH = 6
				issuesH = remaining - projectsH - detailH
			case focusProjects:
				issuesH = collapsed
				detailH = 6
				projectsH = remaining - issuesH - detailH
			default:
				issuesH = remaining / 3
				projectsH = remaining / 3
				detailH = remaining - issuesH - projectsH
			}
		}

		if issuesH < collapsed {
			issuesH = collapsed
		}
		if projectsH < collapsed {
			projectsH = collapsed
		}
		if detailH < collapsed {
			detailH = collapsed
		}

		a.statusPanel.SetSize(w, statusH)
		a.issuesList.SetSize(w, issuesH)
		a.projectList.SetSize(w, projectsH)
		a.detailView.SetSize(w, detailH)
		a.logPanel.SetSize(w, logH)
		return
	}

	sideW := a.sideWidth()
	mainW := a.width - sideW

	statusH := 3
	remaining := totalH - statusH
	var issuesH, projectsH int

	switch a.leftFocus {
	case focusStatus:
		issuesH = remaining / 2
		projectsH = remaining - issuesH
	case focusIssues:
		projectsH = 5
		issuesH = remaining - projectsH
	case focusProjects:
		issuesH = 5
		projectsH = remaining - issuesH
	}

	if issuesH < 3 {
		issuesH = 3
	}
	if projectsH < 3 {
		projectsH = 3
	}

	a.statusPanel.SetSize(sideW, statusH)
	a.issuesList.SetSize(sideW, issuesH)
	a.projectList.SetSize(sideW, projectsH)

	logH := 8
	detailH := totalH - logH
	if detailH < 5 {
		detailH = 5
	}

	a.detailView.SetSize(mainW, detailH)
	a.logPanel.SetSize(mainW, logH)
}

func (a *App) updateFocusState() {
	a.statusPanel.SetFocused(false)
	a.issuesList.SetFocused(false)
	a.projectList.SetFocused(false)
	a.detailView.SetFocused(false)

	if a.side == sideLeft {
		switch a.leftFocus {
		case focusStatus:
			a.statusPanel.SetFocused(true)
		case focusIssues:
			a.issuesList.SetFocused(true)
		case focusProjects:
			a.projectList.SetFocused(true)
		}
	} else {
		a.detailView.SetFocused(true)
	}

	a.helpBar.SetItems(a.helpBarItems())
	a.layoutPanels()
}

// Commands.

func fetchIssues(client *jira.Client, projectKey string) tea.Cmd {
	return func() tea.Msg {
		jql := fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey)
		result, err := client.SearchIssues(context.Background(), jql, 0, 50)
		if err != nil {
			return errorMsg{err: err}
		}
		return issuesLoadedMsg{issues: result.Issues}
	}
}

func fetchIssueDetail(client *jira.Client, key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(context.Background(), key)
		if err != nil {
			return errorMsg{err: err}
		}
		comments, err := client.GetComments(context.Background(), key)
		if err == nil {
			issue.Comments = comments
		}
		changelog, err := client.GetChangelog(context.Background(), key)
		if err == nil {
			issue.Changelog = changelog
		}
		return issueDetailLoadedMsg{issue: issue}
	}
}

func fetchProjects(client *jira.Client) tea.Cmd {
	return func() tea.Msg {
		projects, err := client.GetProjects(context.Background())
		if err != nil {
			return errorMsg{err: err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

func prefetchIssue(client *jira.Client, key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(context.Background(), key)
		if err != nil {
			return nil // silent fail for prefetch
		}
		comments, err := client.GetComments(context.Background(), key)
		if err == nil {
			issue.Comments = comments
		}
		changelog, err := client.GetChangelog(context.Background(), key)
		if err == nil {
			issue.Changelog = changelog
		}
		return issuePrefetchedMsg{issue: issue}
	}
}

func fetchTransitions(client *jira.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		transitions, err := client.GetTransitions(context.Background(), issueKey)
		if err != nil {
			return errorMsg{err: err}
		}
		return transitionsLoadedMsg{issueKey: issueKey, transitions: transitions}
	}
}

func doTransition(client *jira.Client, key, transitionID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DoTransition(context.Background(), key, transitionID)
		if err != nil {
			return errorMsg{err: err}
		}
		return transitionDoneMsg{}
	}
}
