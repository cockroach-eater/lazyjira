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

// editCtx tracks what edit operation is in progress.
type editCtx struct {
	kind      string // "description", "comment-new", "comment-edit", "summary", "field", "field-text"
	issueKey  string
	commentID string // for "comment-edit"
	fieldID   string // for "field" / "field-text" (e.g. "customfield_10015")
}

// Modal kind constants for ModalSelectedMsg dispatch.
const (
	modalPriority  = "priority"
	modalAssignee  = "assignee"
	modalReporter  = "reporter"
	modalIssueType = "issuetype"
	modalLabels    = "labels"
	modalComps     = "components"
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
	jqlModal  components.JQLModal
	diffView   components.DiffView
	inputModal components.InputModal

	// JQL autocomplete cache.
	jqlFields []jira.AutocompleteField

	// Edit session state.
	editTempPath string // temp file path for cleanup
	editContext  editCtx
	modalKind    string // modal* constants

	side        focusSide
	leftFocus   focusPanel
	projectKey  string
	projectID   string
	showHelp    bool
	helpCursor  int
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
	diffView := components.NewDiffView()
	inputModal := components.NewInputModal()
	jqlModal := components.NewJQLModal()

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
		jqlModal:    jqlModal,
		diffView:    diffView,
		inputModal:  inputModal,
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

	// JQL modal intercepts all keys and mouse when visible.
	if a.jqlModal.IsVisible() {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			updated, cmd := a.jqlModal.Update(msg)
			a.jqlModal = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
	}

	// Input modal intercepts keys when visible.
	if a.inputModal.IsVisible() {
		if _, ok := msg.(tea.KeyMsg); ok {
			updated, cmd := a.inputModal.Update(msg)
			a.inputModal = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
	}

	// Diff view intercepts keys when visible.
	if a.diffView.IsVisible() {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			updated, cmd := a.diffView.Update(msg)
			a.diffView = updated
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
		a.jqlModal.SetSize(msg.Width, msg.Height)
		a.diffView.SetSize(msg.Width, msg.Height)
		a.inputModal.SetSize(msg.Width, msg.Height)
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
			// Select top filtered project and load its issues.
			if p := a.projectList.SelectedProject(); p != nil {
				a.projectKey = p.Key
				a.projectID = p.ID
				a.statusPanel.SetProject(p.Key)
				a.projectList.SetActiveKey(p.Key)
				a.issuesList.ClearActiveKey()
				a.issuesList.InvalidateTabCache()
				a.issueCache = make(map[string]*jira.Issue)
				if !a.demoMode {
					go saveLastProject(p.Key)
				}
				cmds = append(cmds, a.fetchActiveTab())
			}
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
		// Help popup: j/k navigate, esc/q close, other keys ignored.
		if a.showHelp {
			bindings := a.ContextBindings()
			switch msg.String() {
			case "j", "down":
				if a.helpCursor < len(bindings)-1 {
					a.helpCursor++
				}
				return a, nil
			case "k", "up":
				if a.helpCursor > 0 {
					a.helpCursor--
				}
				return a, nil
			case "esc", "q", "?":
				a.showHelp = false
				return a, nil
			default:
				// Ignore other keys — keep help open.
				return a, nil
			}
		}

		action := a.keymap.Match(msg.String())
		switch action {
		case ActQuit:
			return a, tea.Quit

		case ActHelp:
			a.showHelp = true
			a.helpCursor = 0
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
							display := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
							if key := a.extractIssueKey(u); key != "" {
								items = append(items, components.ModalItem{ID: u, Label: key + " " + display, Internal: true})
							} else {
								items = append(items, components.ModalItem{ID: u, Label: display})
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

		case ActComments:
			sel := a.issuesList.SelectedIssue()
			if sel == nil {
				return a, nil
			}
			a.side = sideRight
			a.detailView.SetActiveTab(views.TabComments)
			a.updateFocusState()
			if _, ok := a.issueCache[sel.Key]; !ok {
				return a, fetchIssueDetail(a.client, sel.Key)
			}
			return a, nil

		case ActAddComment:
			sel := a.issuesList.SelectedIssue()
			if sel == nil || a.side != sideRight || a.detailView.ActiveTab() != views.TabComments {
				return a, nil
			}
			a.editContext = editCtx{kind: "comment-new", issueKey: sel.Key}
			return a, launchEditor("", ".md")

		case ActEdit:
			sel := a.issuesList.SelectedIssue()
			if sel == nil {
				return a, nil
			}
			if a.side == sideLeft && a.leftFocus == focusIssues {
				// Issues panel → edit summary (inline input).
				a.inputModal.SetSize(a.width, a.height)
				a.inputModal.Show("Edit Summary", sel.Summary)
				a.editContext = editCtx{kind: "summary", issueKey: sel.Key}
				return a, nil
			}
			// Detail panel — context-aware by tab.
			if a.side == sideRight && a.detailView.ActiveTab() == views.TabComments {
				// Cmt tab → edit comment under cursor.
				cmt := a.detailView.SelectedComment()
				if cmt == nil {
					return a, nil
				}
				md := ""
				if cmt.BodyADF != nil {
					md = views.ADFToMarkdown(cmt.BodyADF)
				} else if cmt.Body != "" {
					md = cmt.Body
				}
				a.editContext = editCtx{kind: "comment-edit", issueKey: sel.Key, commentID: cmt.ID}
				return a, launchEditor(md, ".md")
			}
			// Info tab → type-aware field editing.
			if a.side == sideRight && a.detailView.ActiveTab() == views.TabInfo {
				return a.editInfoField(sel)
			}
			// Detail panel (Body tab) → edit description ($EDITOR).
			cached := sel
			if c, ok := a.issueCache[sel.Key]; ok {
				cached = c
			}
			md := ""
			if cached.DescriptionADF != nil {
				md = views.ADFToMarkdown(cached.DescriptionADF)
			} else if cached.Description != "" {
				md = cached.Description
			}
			a.editContext = editCtx{kind: "description", issueKey: sel.Key}
			return a, launchEditor(md, ".md")

		case ActEditPriority:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				*a.logFlag = true
				return a, fetchPriorities(a.client)
			}
			return a, nil

		case ActEditAssignee:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				*a.logFlag = true
				return a, fetchUsers(a.client, a.projectKey, sel.Key)
			}
			return a, nil

		case ActJQLSearch:
			history := LoadJQLHistory()
			prefill := ""
			if a.projectKey != "" {
				prefill = "project = " + a.projectKey + " AND "
			}
			a.jqlModal.SetSize(a.width, a.height)
			a.jqlModal.Show(prefill, history)
			// Fetch autocomplete data if not cached.
			if a.jqlFields == nil {
				cmds = append(cmds, fetchJQLAutocompleteData(a.client))
			}
			return a, tea.Batch(cmds...)

		case ActCloseJQLTab:
			if a.side == sideLeft && a.leftFocus == focusIssues && a.issuesList.IsJQLTab() {
				a.issuesList.RemoveJQLTab()
				if !a.issuesList.HasCachedTab() {
					return a, a.fetchActiveTab()
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
		a.modalKind = ""
		a.modal.Show("Transition: "+msg.issueKey, items)
		return a, nil

	case prioritiesLoadedMsg:
		if len(msg.priorities) == 0 {
			return a, nil
		}
		var items []components.ModalItem
		for _, p := range msg.priorities {
			items = append(items, components.ModalItem{ID: p.ID, Label: p.Name})
		}
		a.modal.SetSize(a.width, a.height)
		a.modalKind = modalPriority
		a.modal.Show("Priority", items)
		return a, nil

	case usersLoadedMsg:
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return a, nil
		}
		// Build assignee list: Unassigned, self, then others.
		var items []components.ModalItem
		items = append(items, components.ModalItem{ID: "", Label: "Unassigned"})
		email := a.cfg.Jira.Email
		for _, u := range msg.users {
			if u.Email == email {
				items = append(items, components.ModalItem{ID: u.AccountID, Label: "→ " + u.DisplayName})
				break
			}
		}
		for _, u := range msg.users {
			if u.Email != email {
				items = append(items, components.ModalItem{ID: u.AccountID, Label: u.DisplayName})
			}
		}
		a.modal.SetSize(a.width, a.height)
		a.modalKind = modalAssignee
		a.modal.Show("Assignee: "+sel.Key, items)
		return a, nil

	case labelsLoadedMsg:
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return a, nil
		}
		cached := sel
		if c, ok := a.issueCache[sel.Key]; ok {
			cached = c
		}
		// Build items and selected map from current labels.
		selected := make(map[string]bool)
		for _, l := range cached.Labels {
			selected[l] = true
		}
		var items []components.ModalItem
		for _, l := range msg.labels {
			items = append(items, components.ModalItem{ID: l, Label: l})
		}
		a.modal.SetSize(a.width, a.height)
		a.modal.ShowChecklist("Labels: "+sel.Key, items, selected)
		return a, nil

	case componentsLoadedMsg:
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return a, nil
		}
		cached := sel
		if c, ok := a.issueCache[sel.Key]; ok {
			cached = c
		}
		selected := make(map[string]bool)
		for _, c := range cached.Components {
			selected[c.ID] = true
		}
		var items []components.ModalItem
		for _, c := range msg.components {
			items = append(items, components.ModalItem{ID: c.ID, Label: c.Name})
		}
		a.modal.SetSize(a.width, a.height)
		a.modal.ShowChecklist("Components: "+sel.Key, items, selected)
		return a, nil

	case issueTypesLoadedMsg:
		var items []components.ModalItem
		for _, t := range msg.issueTypes {
			items = append(items, components.ModalItem{ID: t.ID, Label: t.Name})
		}
		a.modal.SetSize(a.width, a.height)
		a.modal.Show("Issue Type", items)
		return a, nil

	case components.ChecklistConfirmedMsg:
		kind := a.modalKind
		a.modalKind = ""
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return a, nil
		}
		switch kind {
		case modalLabels:
			var labels []string
			for _, item := range msg.Selected {
				labels = append(labels, item.ID)
			}
			return a, updateIssueField(a.client, sel.Key, "labels", labels)
		case modalComps:
			var comps []map[string]string
			for _, item := range msg.Selected {
				comps = append(comps, map[string]string{"id": item.ID})
			}
			return a, updateIssueField(a.client, sel.Key, "components", comps)
		}
		return a, nil

	case components.ModalSelectedMsg:
		kind := a.modalKind
		a.modalKind = ""
		id := msg.Item.ID
		switch kind {
		case modalPriority:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return a, updateIssueField(a.client, sel.Key, "priority", map[string]string{"id": id})
			}
		case modalAssignee, modalReporter:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				fieldName := kind
				if id == "" {
					return a, updateIssueField(a.client, sel.Key, fieldName, nil)
				}
				return a, updateIssueField(a.client, sel.Key, fieldName, map[string]string{"accountId": id})
			}
		case modalIssueType:
			if sel := a.issuesList.SelectedIssue(); sel != nil {
				return a, updateIssueField(a.client, sel.Key, "issuetype", map[string]string{"id": id})
			}
		default:
			// Transition or URL picker.
			if strings.HasPrefix(id, "http") {
				if issueKey := a.extractIssueKey(id); issueKey != "" {
					a.navigateToIssue(issueKey)
					return a, nil
				}
				openBrowser(id)
			} else {
				if sel := a.issuesList.SelectedIssue(); sel != nil {
					return a, doTransition(a.client, sel.Key, id)
				}
			}
		}
		return a, nil

	case components.ModalCancelledMsg:
		a.modalKind = ""
		return a, nil

	case editorFinishedMsg:
		// Re-enable mouse after editor exits (tea.ExecProcess may disable it).
		cmds = append(cmds, tea.EnableMouseCellMotion)
		content, changed, err := readAndCheckEditor(msg)
		if err != nil {
			cleanupEditor(msg.tempPath)
			a.editTempPath = ""
			a.err = err
			return a, tea.Batch(cmds...)
		}
		if !changed {
			cleanupEditor(msg.tempPath)
			a.editTempPath = ""
			return a, tea.Batch(cmds...)
		}
		// Store temp path for cleanup after diff decision.
		a.editTempPath = msg.tempPath
		a.diffView.SetSize(a.width, a.height)
		original := msg.original
		a.diffView.Show("Confirm changes", original, content)
		return a, tea.Batch(cmds...)

	case components.DiffConfirmedMsg:
		cleanupEditor(a.editTempPath)
		a.editTempPath = ""
		cmd := a.applyEdit(msg.Content)
		return a, cmd

	case components.InputConfirmedMsg:
		ctx := a.editContext
		a.editContext = editCtx{}
		switch ctx.kind {
		case "summary":
			if msg.Text != "" {
				return a, updateIssueField(a.client, ctx.issueKey, "summary", msg.Text)
			}
		case "field":
			if msg.Text != "" {
				return a, updateIssueField(a.client, ctx.issueKey, ctx.fieldID, msg.Text)
			}
		}
		return a, nil

	case components.InputCancelledMsg:
		a.editContext = editCtx{}
		return a, nil

	case components.DiffCancelledMsg:
		cleanupEditor(a.editTempPath)
		a.editTempPath = ""
		return a, nil

	case issueUpdatedMsg:
		// Re-fetch both the issue detail and the issue list to reflect changes.
		return a, tea.Batch(
			fetchIssueDetail(a.client, msg.issueKey),
			a.fetchActiveTab(),
		)

	case commentAddedMsg:
		// Re-fetch the issue to show new comment.
		return a, fetchIssueDetail(a.client, msg.issueKey)

	case commentUpdatedMsg:
		// Re-fetch the issue to show updated comment.
		return a, fetchIssueDetail(a.client, msg.issueKey)

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

	case components.JQLSubmitMsg:
		*a.logFlag = true
		a.jqlModal.SetLoading(true)
		return a, fetchJQLSearch(a.client, msg.Query)

	case jqlSearchResultMsg:
		*a.logFlag = false
		a.jqlModal.Hide()
		a.issuesList.AddJQLTab(msg.jql)
		a.issuesList.SetIssues(msg.issues)
		// Save to history.
		history := LoadJQLHistory()
		history = AddToHistory(history, msg.jql)
		_ = SaveJQLHistory(history)
		// Focus issues panel on JQL tab.
		a.side = sideLeft
		a.leftFocus = focusIssues
		a.updateFocusState()
		// Prefetch details.
		for _, issue := range msg.issues {
			cmds = append(cmds, prefetchIssue(a.client, issue.Key))
		}
		return a, tea.Batch(cmds...)

	case jqlSearchErrorMsg:
		*a.logFlag = false
		a.jqlModal.SetError(msg.err)
		return a, nil

	case components.JQLCancelMsg:
		return a, nil

	case components.JQLInputChangedMsg:
		// Parse context for autocomplete.
		ctx := parseJQLContext(msg.Text, msg.CursorPos)
		a.jqlModal.SetPartialLen(ctx.PartialLen)
		switch ctx.Mode {
		case jqlCtxField:
			if a.jqlFields != nil {
				suggestions := matchFieldSuggestions(a.jqlFields, ctx.Partial)
				if len(suggestions) > 0 {
					a.jqlModal.SetSuggestions(suggestions)
				} else {
					a.jqlModal.SetHistory(LoadJQLHistory())
				}
			}
		case jqlCtxValue:
			a.jqlModal.SetACLoading(true)
			return a, fetchJQLSuggestions(a.client, ctx.FieldName, ctx.Partial)
		default:
			a.jqlModal.SetHistory(LoadJQLHistory())
		}
		return a, nil

	case jqlFieldsLoadedMsg:
		a.jqlFields = msg.fields
		return a, nil

	case jqlSuggestionsMsg:
		if a.jqlModal.IsVisible() {
			var items []string
			for _, s := range msg.suggestions {
				items = append(items, s.Value)
			}
			if len(items) > 0 {
				a.jqlModal.SetSuggestions(items)
			} else {
				a.jqlModal.SetHistory(LoadJQLHistory())
			}
		}
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
			a.projectID = projects[0].ID
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

	// Refresh help bar hints for current context (overlays, panels).
	a.helpBar.SetItems(a.helpBarItems())

	var bottomBar string
	switch {
	case a.searchBar.IsActive():
		bottomBar = a.searchBar.View()
	case a.jqlModal.IsVisible():
		bottomBar = a.helpBar.View()
	case a.modal.IsVisible() && a.modal.IsSearching():
		bottomBar = a.modal.SearchView(a.width)
	default:
		bottomBar = a.helpBar.View()
	}

	full := lipgloss.JoinVertical(lipgloss.Left, content, bottomBar)

	// Overlays (mutually exclusive — only one shown at a time).
	switch {
	case a.inputModal.IsVisible():
		popup := a.inputModal.View()
		popupW := lipgloss.Width(popup)
		popupH := len(strings.Split(popup, "\n"))
		x := (a.width - popupW) / 2
		y := (a.height - popupH) / 2
		full = components.OverlayAt(full, popup, x, y, a.width, a.height)
	case a.diffView.IsVisible():
		diff := a.diffView.View()
		diffW := lipgloss.Width(diff)
		diffH := len(strings.Split(diff, "\n"))
		x := (a.width - diffW) / 2
		y := (a.height - diffH) / 2
		full = components.OverlayAt(full, diff, x, y, a.width, a.height)
	case a.jqlModal.IsVisible():
		popup := a.jqlModal.View()
		full = components.Overlay(full, popup, a.width, a.height)
	case a.modal.IsVisible():
		full = a.renderModalOverlay(full)
	}
	if a.showHelp {
		full = a.renderHelpOverlay(full)
	}
	return full
}

// editInfoField dispatches field editing by type when `e` is pressed on the Info tab.
func (a *App) editInfoField(sel *jira.Issue) (tea.Model, tea.Cmd) {
	field := a.detailView.SelectedInfoField()
	if field == nil {
		return a, nil
	}
	*a.logFlag = true
	switch field.Type {
	case views.FieldSingleSelect:
		switch field.FieldID {
		case "status":
			// Reuse transition flow.
			return a, fetchTransitions(a.client, sel.Key)
		case "priority":
			return a, fetchPriorities(a.client)
		case "issuetype":
			if a.projectID != "" {
				a.modalKind = modalIssueType
				return a, fetchIssueTypes(a.client, a.projectID)
			}
		}
	case views.FieldPerson:
		switch field.FieldID {
		case "assignee":
			a.modalKind = modalAssignee
			return a, fetchUsers(a.client, a.projectKey, sel.Key)
		case "reporter":
			a.modalKind = modalReporter
			return a, fetchUsers(a.client, a.projectKey, sel.Key)
		}
	case views.FieldMultiSelect:
		switch field.FieldID {
		case "labels":
			a.modalKind = modalLabels
			return a, fetchLabels(a.client)
		case "components":
			a.modalKind = modalComps
			return a, fetchComponents(a.client, a.projectKey)
		}
	case views.FieldSingleText:
		// InputModal for single-line text (summary, custom fields).
		a.inputModal.SetSize(a.width, a.height)
		a.inputModal.Show("Edit "+field.Name, field.Value)
		a.editContext = editCtx{kind: "field", issueKey: sel.Key, fieldID: field.FieldID}
		return a, nil
	case views.FieldMultiText:
		// $EDITOR for multi-line text.
		a.editContext = editCtx{kind: "field-text", issueKey: sel.Key, fieldID: field.FieldID}
		return a, launchEditor(field.Value, ".md")
	}
	return a, nil
}

// applyEdit sends the edited content to the Jira API based on editContext.
func (a *App) applyEdit(mdContent string) tea.Cmd {
	ctx := a.editContext
	a.editContext = editCtx{} // clear

	switch ctx.kind {
	case "description":
		adf := views.MarkdownToADF(mdContent)
		return updateIssueField(a.client, ctx.issueKey, "description", adf)
	case "comment-new":
		adf := views.MarkdownToADF(mdContent)
		return addComment(a.client, ctx.issueKey, adf)
	case "comment-edit":
		adf := views.MarkdownToADF(mdContent)
		return updateComment(a.client, ctx.issueKey, ctx.commentID, adf)
	case "field-text":
		return updateIssueField(a.client, ctx.issueKey, ctx.fieldID, mdContent)
	}
	return nil
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

	// Clamp cursor.
	if a.helpCursor >= len(bindings) {
		a.helpCursor = len(bindings) - 1
	}
	if a.helpCursor < 0 {
		a.helpCursor = 0
	}

	maxKey := 0
	for _, b := range bindings {
		if len(b.Key) > maxKey {
			maxKey = len(b.Key)
		}
	}

	popupW := min(maxKey+40, a.width-4)

	keyNormal := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	keySel := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Background(lipgloss.Color("4"))
	descSel := lipgloss.NewStyle().Background(lipgloss.Color("4"))

	lines := make([]string, 0, len(bindings)+2)
	lines = append(lines, "")
	descMaxW := popupW - maxKey - 6 // 2 left pad + 2 gap + 2 border
	for i, b := range bindings {
		padded := b.Key
		for len(padded) < maxKey {
			padded += " "
		}
		desc := components.TruncateEnd(b.Description, descMaxW)
		// Pad desc to fill remaining width.
		for len(desc) < descMaxW {
			desc += " "
		}
		var line string
		if i == a.helpCursor {
			line = descSel.Render("  ") + keySel.Render(padded) + descSel.Render("  "+desc)
		} else {
			line = "  " + keyNormal.Render(padded) + "  " + desc
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")

	popupH := min(len(lines), a.height-4)
	footer := fmt.Sprintf("%d of %d", a.helpCursor+1, len(bindings))

	popupContent := strings.Join(lines, "\n")
	content := components.RenderPanelFull("Keybindings", footer, popupContent, popupW, popupH, true, nil)

	// Center the popup over background.
	return components.Overlay(base, content, a.width, a.height)
}

// fetchActiveTab returns a command to fetch issues for the current active tab's JQL.
func (a *App) fetchActiveTab() tea.Cmd {
	// JQL tab: use stored raw JQL directly.
	if a.issuesList.IsJQLTab() {
		jql := a.issuesList.JQLQuery()
		if jql == "" {
			return nil
		}
		tabIdx := a.issuesList.GetTabIndex()
		*a.logFlag = true
		return fetchIssuesByJQL(a.client, jql, tabIdx)
	}
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

