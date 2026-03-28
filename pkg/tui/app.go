package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/git"
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
	focusInfo
	focusProjects
)

type focusSide int

const (
	sideLeft focusSide = iota
	sideRight
)

// editKind identifies the type of edit in progress.
type editKind int

const (
	editNone       editKind = iota
	editDesc                // description via $EDITOR
	editCommentNew          // new comment via $EDITOR
	editCommentMod          // edit existing comment via $EDITOR
	editSummary             // summary via InputModal
	editField               // custom field via InputModal
	editFieldText           // multi-line custom field via $EDITOR
	editBranch              // git branch via InputModal
)

// editCtx tracks what edit operation is in progress.
type editCtx struct {
	kind      editKind
	issueKey  string
	commentID string // for editCommentMod
	fieldID   string // for editField / editFieldText
}

// Modal callback types for result dispatch.
type onSelectFunc func(components.ModalItem) tea.Cmd
type onChecklistFunc func([]components.ModalItem) tea.Cmd

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
type batchPrefetchedMsg struct {
	issues []jira.Issue
}
type autoFetchTickMsg struct{}
type boardsLoadedMsg struct{ boards []jira.Board }
type sprintsLoadedMsg struct{ sprints []jira.Sprint }
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
	infoPanel   *views.InfoPanel
	projectList *views.ProjectList
	detailView  *views.DetailView
	logPanel    *views.LogPanel

	keymap    Keymap
	helpBar   components.HelpBar
	searchBar components.SearchBar
	modal      components.Modal
	jqlModal   components.JQLModal
	diffView   components.DiffView
	inputModal components.InputModal
	overlays   components.OverlayStack

	// JQL autocomplete cache.
	jqlFields []jira.AutocompleteField

	// Edit session state.
	editTempPath string // temp file path for cleanup
	editContext  editCtx

	// Modal callbacks: set before showing modal, consumed on result.
	onSelect    onSelectFunc
	onChecklist onChecklistFunc

	side        focusSide
	leftFocus   focusPanel
	projectKey  string
	projectID   string
	boardID     int         // agile board for current project (0 = unknown)
	boards      []jira.Board // cached boards from API
	showHelp    bool
	helpCursor  int
	logFlag     *bool
	demoMode    bool
	issueCache  map[string]*jira.Issue

	// Git integration.
	gitRepoPath    string // empty = not a git repo
	gitBranch      string // current branch name
	gitDetectedKey string // issue key from auto-detect (consumed once)

	// Cached panel sizes for mouse hit-testing.
	panelSideW     int
	panelStatusH   int
	panelIssuesH   int
	panelInfoH     int
	panelProjectsH int
	panelDetailH   int
	panelLogH      int

	width  int
	height int
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
	infoPanel := views.NewInfoPanel()
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
		infoPanel.SetCustomFields(cfg.CustomFields)
	}

	app := &App{
		cfg:        cfg,
		client:     client,
		keymap:     KeymapFromConfig(cfg.Keybinding),
		splashInfo: splash,
		statusPanel: statusPanel,
		issuesList:  issuesList,
		infoPanel:   infoPanel,
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
	// Overlay stack: checked in priority order for input interception and rendering.
	app.overlays = components.OverlayStack{
		&app.jqlModal,
		&app.inputModal,
		&app.diffView,
		&app.modal,
	}

	// Detect git repo (sync, <10ms).
	if git.GitAvailable() {
		cwd, _ := os.Getwd()
		if git.IsRepo(cwd) {
			app.gitRepoPath = cwd
			if branch, err := git.CurrentBranch(cwd); err == nil && branch != "" {
				app.gitBranch = branch
				app.gitDetectedKey = git.ExtractIssueKey(branch)

			}
		}
	}

	app.helpBar.SetItems(app.helpBarItems())
	return app
}

func (a *App) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, fetchProjects(a.client))
	cmds = append(cmds, fetchBoards(a.client))
	if cmd := a.fetchActiveTab(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Start autofetch timer.
	cmds = append(cmds, tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return autoFetchTickMsg{}
	}))
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Search bar intercepts all keys when active, except arrow keys
	// which pass through for navigating filtered results.
	if a.searchBar.IsActive() {
		if km, ok := msg.(tea.KeyMsg); ok && km.Type != tea.KeyUp && km.Type != tea.KeyDown {
			updated, cmd := a.searchBar.Update(msg)
			a.searchBar = updated
			return a, cmd
		}
	}

	// Overlays (JQL modal, input modal, diff view, selection modal) intercept input.
	if cmd, ok := a.overlays.Intercept(msg); ok {
		return a, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleResize(msg)
	case tea.MouseMsg:
		return a.handleMouse(msg)
	case tea.KeyMsg:
		if m, cmd := a.handleKeyMsg(msg); m != nil {
			return m, cmd
		}

	case components.SearchChangedMsg:
		return a.handleSearchChanged(msg)
	case components.SearchConfirmedMsg:
		return a.handleSearchConfirmed()
	case components.SearchCancelledMsg:
		return a.handleSearchCancelled()

	case autoFetchTickMsg:
		return a.handleAutoFetch()
	case issuesLoadedMsg:
		return a.handleIssuesLoaded(msg)
	case issueDetailLoadedMsg:
		return a.handleIssueDetailLoaded(msg)
	case issuePrefetchedMsg:
		return a.handleIssuePrefetched(msg)
	case batchPrefetchedMsg:
		return a.handleBatchPrefetched(msg)
	case projectsLoadedMsg:
		return a.handleProjectsLoaded(msg)

	case transitionDoneMsg:
		return a.handleTransitionDone()
	case transitionsLoadedMsg:
		return a.handleTransitionsLoaded(msg)
	case prioritiesLoadedMsg:
		return a.handlePrioritiesLoaded(msg)
	case boardsLoadedMsg:
		return a.handleBoardsLoaded(msg)
	case sprintsLoadedMsg:
		return a.handleSprintsLoaded(msg)
	case usersLoadedMsg:
		return a.handleUsersLoaded(msg)
	case labelsLoadedMsg:
		return a.handleLabelsLoaded(msg)
	case componentsLoadedMsg:
		return a.handleComponentsLoaded(msg)
	case issueTypesLoadedMsg:
		return a.handleIssueTypesLoaded(msg)
	case issueUpdatedMsg:
		return a.handleIssueUpdated(msg)
	case commentAddedMsg:
		return a, fetchIssueDetail(a.client, msg.issueKey)
	case commentUpdatedMsg:
		return a, fetchIssueDetail(a.client, msg.issueKey)

	case components.ModalSelectedMsg:
		return a.handleModalSelected(msg)
	case components.ChecklistConfirmedMsg:
		return a.handleChecklistConfirmed(msg)
	case components.ModalCancelledMsg:
		return a.handleModalCancelled()

	case editorFinishedMsg:
		return a.handleEditorFinished(msg)
	case components.DiffConfirmedMsg:
		return a.handleDiffConfirmed(msg)
	case components.DiffCancelledMsg:
		return a.handleDiffCancelled()
	case components.InputConfirmedMsg:
		return a.handleInputConfirmed(msg)
	case components.InputCancelledMsg:
		return a.handleInputCancelled()

	case components.JQLSubmitMsg:
		return a.handleJQLSubmit(msg)
	case jqlSearchResultMsg:
		return a.handleJQLSearchResult(msg)
	case jqlSearchErrorMsg:
		return a.handleJQLSearchError(msg)
	case components.JQLCancelMsg:
		return a, nil
	case components.JQLInputChangedMsg:
		return a.handleJQLInputChanged(msg)
	case jqlFieldsLoadedMsg:
		return a.handleJQLFieldsLoaded(msg)
	case jqlSuggestionsMsg:
		return a.handleJQLSuggestions(msg)

	case views.NavigateIssueMsg:
		a.navigateToIssue(msg.Key)
		return a, nil
	case views.ExpandBlockMsg:
		return a.handleExpandBlock(msg)

	case gitBranchCreatedMsg:
		return a.handleGitBranchSwitch(msg.name)
	case gitCheckoutDoneMsg:
		return a.handleGitBranchSwitch(msg.name)
	case gitErrorMsg:
		a.statusPanel.SetError(msg.err.Error())
		return a, nil
	case errorMsg:
		a.statusPanel.SetError(msg.err.Error())
		a.statusPanel.SetOnline(false)
		return a, nil

	case views.IssueSelectedMsg:
		if msg.Issue != nil {
			if cached, ok := a.issueCache[msg.Issue.Key]; ok {
				a.detailView.SetIssue(cached)
				a.infoPanel.SetIssue(cached)
			} else {
				a.detailView.SetIssue(msg.Issue)
				a.infoPanel.SetIssue(msg.Issue)
			}
		}
		return a, nil
	case views.ProjectHoveredMsg:
		if msg.Project != nil {
			a.detailView.SetProject(msg.Project)
		}
		return a, nil
	}

	// Route remaining input to focused panel.
	return a, a.routeToPanel(msg)
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
			a.infoPanel.View(),
			a.projectList.View(),
			a.detailView.View(),
			a.logPanel.View(),
		)
	} else {
		leftCol := lipgloss.JoinVertical(lipgloss.Left,
			a.statusPanel.View(),
			a.issuesList.View(),
			a.infoPanel.View(),
			a.projectList.View(),
		)

		rightCol := lipgloss.JoinVertical(lipgloss.Left,
			a.detailView.View(),
			a.logPanel.View(),
		)

		content = lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
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

	// Render the first visible overlay (mutually exclusive).
	full = a.overlays.Render(full, a.width, a.height)
	if a.showHelp {
		full = a.renderHelpOverlay(full)
	}
	return full
}

// editInfoField dispatches field editing by type when `e` is pressed on the Info panel or tab.
func (a *App) editInfoField(sel *jira.Issue) (tea.Model, tea.Cmd) {
	field := a.infoPanel.SelectedInfoField()
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
				a.onSelect = a.makeFieldSelectCallback(sel.Key, "issuetype")
				return a, fetchIssueTypes(a.client, a.projectID)
			}
		case "sprint":
			if a.boardID != 0 {
				return a, fetchSprints(a.client, a.boardID)
			}
			a.statusPanel.SetError("no agile board found for this project")
			return a, nil
		}
	case views.FieldPerson:
		a.onSelect = a.makePersonSelectCallback(field.FieldID)
		return a, fetchUsers(a.client, a.projectKey, sel.Key)
	case views.FieldMultiSelect:
		issueKey := sel.Key
		switch field.FieldID {
		case "labels":
			a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
				labels := make([]string, 0, len(selected))
				for _, item := range selected {
					labels = append(labels, item.ID)
				}
				return updateIssueField(a.client, issueKey, "labels", labels)
			}
			return a, fetchLabels(a.client)
		case "components":
			a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
				comps := make([]map[string]string, 0, len(selected))
				for _, item := range selected {
					comps = append(comps, map[string]string{"id": item.ID})
				}
				return updateIssueField(a.client, issueKey, "components", comps)
			}
			return a, fetchComponents(a.client, a.projectKey)
		}
	case views.FieldSingleText:
		// InputModal for single-line text (summary, custom fields).
		a.inputModal.Show("Edit "+field.Name, field.Value)
		a.editContext = editCtx{kind: editField, issueKey: sel.Key, fieldID: field.FieldID}
		return a, nil
	case views.FieldMultiText:
		// $EDITOR for multi-line text.
		a.editContext = editCtx{kind: editFieldText, issueKey: sel.Key, fieldID: field.FieldID}
		return a, launchEditor(field.Value, ".md")
	}
	return a, nil
}

// applyEdit sends the edited content to the Jira API based on editContext.
func (a *App) applyEdit(mdContent string) tea.Cmd {
	ctx := a.editContext
	a.editContext = editCtx{} // clear

	switch ctx.kind { //nolint:exhaustive // only editor-based kinds handled here
	case editDesc:
		adf := views.MarkdownToADF(mdContent)
		return updateIssueField(a.client, ctx.issueKey, "description", adf)
	case editCommentNew:
		adf := views.MarkdownToADF(mdContent)
		return addComment(a.client, ctx.issueKey, adf)
	case editCommentMod:
		adf := views.MarkdownToADF(mdContent)
		return updateComment(a.client, ctx.issueKey, ctx.commentID, adf)
	case editFieldText:
		return updateIssueField(a.client, ctx.issueKey, ctx.fieldID, mdContent)
	}
	return nil
}


// makePersonSelectCallback creates a callback for person field (assignee/reporter).
func (a *App) makePersonSelectCallback(fieldID string) onSelectFunc {
	return func(item components.ModalItem) tea.Cmd {
		sel := a.issuesList.SelectedIssue()
		if sel == nil {
			return nil
		}
		if item.ID == "" {
			return updateIssueField(a.client, sel.Key, fieldID, nil)
		}
		return updateIssueField(a.client, sel.Key, fieldID, map[string]string{"accountId": item.ID})
	}
}

// makeFieldSelectCallback creates a callback for simple field selection (priority, issuetype).
func (a *App) makeFieldSelectCallback(issueKey, fieldID string) onSelectFunc {
	return func(item components.ModalItem) tea.Cmd {
		return updateIssueField(a.client, issueKey, fieldID, map[string]string{"id": item.ID})
	}
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
func (a *App) handleGitBranchSwitch(name string) (tea.Model, tea.Cmd) {
	a.gitBranch = name
	a.helpBar.SetStatusMsg(name)
	if a.cfg.Git.CloseOnCheckout {
		return a, tea.Quit
	}
	return a, nil
}

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
	a.infoPanel.SetFocused(false)
	a.projectList.SetFocused(false)
	a.detailView.SetFocused(false)

	if a.side == sideLeft {
		switch a.leftFocus {
		case focusStatus:
			a.statusPanel.SetFocused(true)
		case focusIssues:
			a.issuesList.SetFocused(true)
		case focusInfo:
			a.infoPanel.SetFocused(true)
		case focusProjects:
			a.projectList.SetFocused(true)
		}
	} else {
		a.detailView.SetFocused(true)
	}

	a.helpBar.SetItems(a.helpBarItems())
	a.layoutPanels()
}

