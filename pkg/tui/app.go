package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/views"
)

// Version is set from main at startup.
var Version = "dev"

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
type issuesLoadedMsg struct {
	issues []jira.Issue
	tab    int // which tab index this fetch was for
}
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
	client     jira.ClientInterface
	splashInfo views.SplashInfo

	statusPanel *views.StatusPanel
	issuesList  *views.IssuesList
	projectList *views.ProjectList
	detailView  *views.DetailView
	logPanel    *views.LogPanel

	keymap    Keymap
	helpBar   components.HelpBar
	searchBar components.SearchBar
	modal     components.Modal

	side        focusSide
	leftFocus   focusPanel
	projectKey  string
	showHelp    bool
	logFlag     *bool
	demoMode    bool
	issueCache  map[string]*jira.Issue

	// Cached panel sizes for mouse hit-testing.
	panelSideW    int
	panelStatusH  int
	panelIssuesH  int
	panelProjectsH int
	panelDetailH  int
	panelLogH     int

	width  int
	height int
	err    error
}

// AuthMethod describes how the user authenticated.
type AuthMethod string

const (
	AuthSaved  AuthMethod = "Saved credentials (auth.json)"
	AuthEnv    AuthMethod = "Environment variables"
	AuthWizard AuthMethod = "Setup wizard"
	AuthDemo   AuthMethod = "Demo mode"
)

func NewApp(cfg *config.Config, client jira.ClientInterface) *App {
	return NewAppWithAuth(cfg, client, AuthEnv)
}

func NewAppWithAuth(cfg *config.Config, client jira.ClientInterface, authMethod AuthMethod) *App {
	projectKey := ""
	if len(cfg.Projects) > 0 {
		projectKey = cfg.Projects[0].Key
	}

	statusPanel := views.NewStatusPanel(projectKey, cfg.Jira.Email, cfg.Jira.Host)
	issuesList := views.NewIssuesList()
	if len(cfg.GUI.IssueListFields) > 0 {
		issuesList.SetFields(cfg.GUI.IssueListFields)
	}
	issuesList.SetTabs(cfg.IssueTabs)
	issuesList.SetFocused(true)
	issuesList.SetUserEmail(cfg.Jira.Email)
	projectList := views.NewProjectList()
	detailView := views.NewDetailView()
	logPanel := views.NewLogPanel()
	helpBar := components.NewHelpBar(nil)
	searchBar := components.NewSearchBar()
	modal := components.NewModal()

	logFlag := new(bool) // shared with closure, set via app.logRequests
	client.SetOnRequest(func(rl jira.RequestLog) {
		if *logFlag {
			logPanel.AddEntry(views.LogEntry{
				Time:    time.Now(),
				Method:  rl.Method,
				Path:    rl.Path,
				Status:  rl.Status,
				Elapsed: rl.Elapsed,
			})
		}
	})

	splash := views.SplashInfo{
		Version:    Version,
		AuthMethod: string(authMethod),
		Host:       cfg.Jira.Host,
		Email:      cfg.Jira.Email,
		Project:    projectKey,
	}

	if len(cfg.CustomFields) > 0 {
		ids := make([]string, len(cfg.CustomFields))
		for i, cf := range cfg.CustomFields {
			ids[i] = cf.ID
		}
		client.SetCustomFields(ids)
		detailView.SetCustomFields(cfg.CustomFields)
	}

	app := &App{
		cfg:        cfg,
		client:     client,
		keymap:     KeymapFromConfig(cfg.Keybinding),
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
		demoMode:    authMethod == AuthDemo,
		logFlag:     logFlag,
		issueCache:  make(map[string]*jira.Issue),
	}
	app.helpBar.SetItems(app.helpBarItems())
	return app
}

func (a *App) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, fetchProjects(a.client))
	if cmd := a.fetchActiveTab(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Start autofetch timer.
	cmds = append(cmds, tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return autoFetchTickMsg{}
	}))
	return tea.Batch(cmds...)
}

//nolint:gocognit // will be refactored in Phase 5
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

	// Modal intercepts keys and mouse when visible.
	if a.modal.IsVisible() {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
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

		action := a.keymap.Match(msg.String())
		switch action {
		case ActQuit:
			return a, tea.Quit

		case ActHelp:
			a.showHelp = true
			return a, nil

		case ActSearch:
			a.searchBar.Activate()
			return a, nil

		case ActSwitchPanel:
			if a.side == sideLeft {
				a.side = sideRight
			} else {
				a.side = sideLeft
			}
			a.updateFocusState()
			return a, nil

		case ActFocusRight:
			if a.side == sideLeft {
				a.side = sideRight
				a.updateFocusState()
				return a, nil
			}

		case ActFocusLeft:
			if a.side == sideRight {
				a.side = sideLeft
				a.updateFocusState()
				return a, nil
			}

		case ActSelect:
			// Primary action: select. On sideRight, fall through to let detail handle it.
			switch {
			case a.side == sideLeft && a.leftFocus == focusIssues:
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					a.issuesList.SetActiveKey(sel.Key)
					a.side = sideRight
					a.updateFocusState()
					return a, fetchIssueDetail(a.client, sel.Key)
				}
				return a, nil
			case a.side == sideLeft && a.leftFocus == focusProjects:
				if p := a.projectList.SelectedProject(); p != nil {
					a.projectKey = p.Key
					a.statusPanel.SetProject(p.Key)
					a.projectList.SetActiveKey(p.Key)
					a.issuesList.ClearActiveKey()
					a.issuesList.InvalidateTabCache()
					a.issueCache = make(map[string]*jira.Issue)
					a.leftFocus = focusIssues
					a.updateFocusState()
					if !a.demoMode {
						go saveLastProject(p.Key)
					}
					return a, a.fetchActiveTab()
				}
				return a, nil
			}
			// sideRight: let detail view handle expand via its Update.

		case ActOpen:
			// Secondary action: open/preview without selecting.
			// On sideRight, fall through to let detail handle expand.
			switch {
			case a.side == sideLeft && a.leftFocus == focusIssues:
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					a.side = sideRight
					a.updateFocusState()
					return a, fetchIssueDetail(a.client, sel.Key)
				}
				return a, nil
			case a.side == sideLeft && a.leftFocus == focusProjects:
				if p := a.projectList.SelectedProject(); p != nil {
					a.detailView.SetProject(p)
				}
				return a, nil
			}
			// sideRight: let detail view handle expand via its Update.

		case ActPrevTab:
			if a.side == sideRight {
				a.detailView.PrevTab()
			} else if a.side == sideLeft && a.leftFocus == focusIssues {
				a.issuesList.PrevTab()
				if !a.issuesList.HasCachedTab() {
					return a, a.fetchActiveTab()
				}
			}
			return a, nil
		case ActNextTab:
			if a.side == sideRight {
				a.detailView.NextTab()
			} else if a.side == sideLeft && a.leftFocus == focusIssues {
				a.issuesList.NextTab()
				if !a.issuesList.HasCachedTab() {
					return a, a.fetchActiveTab()
				}
			}
			return a, nil

		case ActFocusDetail:
			a.side = sideRight
			a.updateFocusState()
			return a, nil

		case ActFocusStatus:
			a.side = sideLeft
			a.leftFocus = focusStatus
			a.splashInfo.Project = a.projectKey
			a.detailView.SetSplash(a.splashInfo)
			a.updateFocusState()
			return a, nil
		case ActFocusIssues:
			a.side = sideLeft
			a.leftFocus = focusIssues
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				a.showCachedIssue(sel.Key)
			}
			a.updateFocusState()
			return a, nil
		case ActFocusProj:
			a.side = sideLeft
			a.leftFocus = focusProjects
			a.updateFocusState()
			return a, nil

		case ActCopyURL:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				copyToClipboard(a.cfg.Jira.Host + "/browse/" + sel.Key)
			}
			return a, nil

		case ActBrowser:
			if sel := a.issuesList.SelectedIssue(); sel != nil && (a.leftFocus == focusIssues || a.side == sideRight) {
				openBrowser(a.cfg.Jira.Host + "/browse/" + sel.Key)
			}
			return a, nil

		case ActURLPicker:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				cached := sel
				if c, ok := a.issueCache[sel.Key]; ok {
					cached = c
				}
				groups := views.ExtractURLs(cached, a.cfg.Jira.Host)
				if len(groups) > 0 {
					var items []components.ModalItem
					for i, g := range groups {
						if i > 0 || len(groups) > 1 {
							items = append(items, components.ModalItem{Label: g.Section, Separator: true})
						}
						for _, u := range g.URLs {
							if key := a.extractIssueKey(u); key != "" {
								items = append(items, components.ModalItem{ID: u, Label: key, Internal: true})
							} else {
								items = append(items, components.ModalItem{ID: u, Label: u})
							}
						}
					}
					a.modal.SetSize(a.width, a.height)
					a.modal.Show("URLs", items)
				}
			}
			return a, nil

		case ActTransition:
			if a.side == sideLeft && a.leftFocus == focusIssues {
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					*a.logFlag = true
					return a, fetchTransitions(a.client, sel.Key)
				}
			}
			return a, nil

		case ActRefresh:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				*a.logFlag = true
				return a, fetchIssueDetail(a.client, sel.Key)
			}
			return a, nil

		case ActRefreshAll:
			return a, a.fetchActiveTab()

		default:
			// ActInfoTab and nav keys are handled by the focused panel below.
		}

	case autoFetchTickMsg:
		var fetchCmds []tea.Cmd
		if cmd := a.fetchActiveTab(); cmd != nil {
			fetchCmds = append(fetchCmds, cmd)
		}
		// Schedule next tick.
		fetchCmds = append(fetchCmds, tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
			return autoFetchTickMsg{}
		}))
		return a, tea.Batch(fetchCmds...)

	case issuesLoadedMsg:
		a.err = nil
		*a.logFlag = false
		a.statusPanel.SetOnline(true)
		a.issuesList.SetIssuesForTab(msg.tab, msg.issues)
		// Only update display + prefetch if this is still the active tab.
		if msg.tab == a.issuesList.GetTabIndex() {
			a.issuesList.SetIssues(msg.issues)
			// Prefetch details for all issues in this tab.
			for _, issue := range msg.issues {
				cmds = append(cmds, prefetchIssue(a.client, issue.Key))
			}
		}
		return a, tea.Batch(cmds...)

	case issueDetailLoadedMsg:
		*a.logFlag = false
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
		var fetchCmds []tea.Cmd
		if cmd := a.fetchActiveTab(); cmd != nil {
			fetchCmds = append(fetchCmds, cmd)
		}
		if sel := a.issuesList.SelectedIssue(); sel != nil {
			fetchCmds = append(fetchCmds, fetchIssueDetail(a.client, sel.Key))
		}
		if len(fetchCmds) > 0 {
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
			hint := ""
			if t.To != nil {
				label += " → " + t.To.Name
				hint = t.To.Description
			}
			items = append(items, components.ModalItem{ID: t.ID, Label: label, Hint: hint})
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

	case views.NavigateIssueMsg:
		a.navigateToIssue(msg.Key)
		return a, nil

	case views.ExpandBlockMsg:
		var items []components.ModalItem
		for _, line := range msg.Lines {
			items = append(items, components.ModalItem{ID: "", Label: line})
		}
		a.modal.SetSize(a.width, a.height-1)
		a.modal.ShowReadOnly(msg.Title, items)
		return a, nil

	case errorMsg:
		a.err = msg.err
		a.statusPanel.SetOnline(false)
		return a, nil

	case views.IssueSelectedMsg:
		if msg.Issue != nil {
			if cached, ok := a.issueCache[msg.Issue.Key]; ok {
				a.detailView.SetIssue(cached)
			} else {
				a.detailView.SetIssue(msg.Issue)
			}
		}
		return a, nil

	case projectsLoadedMsg:
		projects := msg.projects
		// Move last-used project to top (skip in demo mode — no credentials).
		if !a.demoMode {
			if creds, err := config.LoadCredentials(); err == nil && creds != nil && creds.LastProject != "" {
				for i, p := range projects {
					if p.Key == creds.LastProject {
						// Swap to front.
						projects[0], projects[i] = projects[i], projects[0]
						break
					}
				}
			}
		}
		a.projectList.SetProjects(projects)
		if a.projectKey == "" && len(projects) > 0 {
			a.projectKey = projects[0].Key
			a.statusPanel.SetProject(a.projectKey)
			a.projectList.SetActiveKey(a.projectKey)
			return a, a.fetchActiveTab()
		}
		return a, nil

	case views.ProjectHoveredMsg:
		if msg.Project != nil {
			a.detailView.SetProject(msg.Project)
		}
		return a, nil

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

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	if a.isVerticalLayout() {
		// Vertical: all stacked.
		content = lipgloss.JoinVertical(lipgloss.Left,
			a.statusPanel.View(),
			a.issuesList.View(),
			a.projectList.View(),
			a.detailView.View(),
			a.logPanel.View(),
		)
	} else {
		leftCol := lipgloss.JoinVertical(lipgloss.Left,
			a.statusPanel.View(),
			a.issuesList.View(),
			a.projectList.View(),
		)

		rightCol := lipgloss.JoinVertical(lipgloss.Left,
			a.detailView.View(),
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
	if a.modal.IsVisible() {
		full = a.renderModalOverlay(full)
	}
	if a.showHelp {
		full = a.renderHelpOverlay(full)
	}

	return full
}

func (a *App) renderModalOverlay(base string) string {
	popup := a.modal.View()
	popupLines := strings.Split(popup, "\n")
	popupW := lipgloss.Width(popup)
	popupH := len(popupLines)

	x := (a.width - popupW) / 2
	y := (a.height - popupH) / 2

	result := components.OverlayAt(base, popup, x, y, a.width, a.height)

	// Hint box: place right below the centered main modal.
	if hint := a.modal.HintView(); hint != "" {
		result = components.OverlayAt(result, hint, x, y+popupH, a.width, a.height)
	}
	return result
}

func (a *App) renderHelpOverlay(base string) string {
	bindings := a.ContextBindings()

	maxKey := 0
	for _, b := range bindings {
		if len(b.Key) > maxKey {
			maxKey = len(b.Key)
		}
	}

	popupW := min(maxKey+40, a.width-4)

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

	lines := make([]string, 0, len(bindings)+3)
	lines = append(lines, "")
	descMaxW := popupW - maxKey - 6 // 2 left pad + 2 gap + 2 border
	for _, b := range bindings {
		padded := b.Key
		for len(padded) < maxKey {
			padded += " "
		}
		desc := components.TruncateEnd(b.Description, descMaxW)
		lines = append(lines, "  "+keyStyle.Render(padded)+"  "+desc)
	}
	lines = append(lines, "")
	lines = append(lines, "  "+lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("press any key to close"))

	popupContent := strings.Join(lines, "\n")
	popupH := min(len(lines), a.height-4)

	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("2")).
		Width(popupW).
		Height(popupH).
		Render(popupContent)

	// Center the popup over background.
	return components.Overlay(base, popup, a.width, a.height)
}

// fetchActiveTab returns a command to fetch issues for the current active tab's JQL.
func (a *App) fetchActiveTab() tea.Cmd {
	if a.projectKey == "" {
		return nil
	}
	tab := a.issuesList.ActiveTab()
	if tab.JQL == "" {
		return nil
	}
	tabIdx := a.issuesList.GetTabIndex()
	jql := resolveTabJQL(tab, a.projectKey, a.cfg.Jira.Email)
	*a.logFlag = true
	return fetchIssuesByJQL(a.client, jql, tabIdx)
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

