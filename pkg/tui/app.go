package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/git"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/navstack"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

var Version = "dev"

type focusPanel int

const (
	focusStatus focusPanel = iota
	focusIssues
	focusInfo
	focusProjects
)

type focusSide int

const (
	sideLeft focusSide = iota
	sideRight
)

const (
	fldPriority    = "priority"
	fldSprint      = "sprint"
	fldLabels      = "labels"
	fldComponents  = "components"
	fldAssignee    = "assignee"
	fldAccountID   = "accountId"
	fldName        = "name"
	fldDescription = "description"
)

type editKind int

const (
	editNone editKind = iota
	editDesc
	editCommentNew
	editCommentMod
	editSummary
	editField
	editFieldText
	editBranch
	editCreateField
	editCreateDesc
	editTabName
)

type editCtx struct {
	kind           editKind
	issueKey       string
	commentID      string
	fieldID        string
	fieldIndex     int
	converterState any
	tabJQL         string // captured at prompt time; source for editTabName confirm
	tabMaxResults  *int   // promote carries the config tab's page size; nil for save
	// returnToJQLModal marks the save-tab prompt opened via Ctrl+S from the JQL
	// search modal: cancelling reopens the search with tabJQL instead of dropping it.
	returnToJQLModal bool
	// prevEditingManagedTab restores the managed-tab-edit context on that reopen,
	// so the reopened search behaves and labels itself as it did before Ctrl+S.
	prevEditingManagedTab int
}

type createCtx struct {
	intent        bool
	projectKey    string
	projectID     string
	issueTypeID   string
	issueTypeName string
	parentKey     string
	duplicateFrom *jira.Issue
}

type onSelectFunc func(components.ModalItem) tea.Cmd
type onChecklistFunc func([]components.ModalItem) tea.Cmd

type issuesLoadedMsg struct {
	issues []jira.Issue
	tab    int
	epoch  int
}
type issueDetailLoadedMsg struct{ issue *jira.Issue }

// previewDetailLoadedMsg carries the response of a preview-triggered fetch.
// See App.previewEpoch.
type previewDetailLoadedMsg struct {
	issue *jira.Issue
	epoch int
}

// previewDebounceMsg is delivered when a PreviewRequestMsg's debounce tick
// expires. See App.previewEpoch.
type previewDebounceMsg struct {
	key   string
	epoch int
}

// childrenLoadedMsg carries the response of a Cloud GetChildren fetch.
// See App.childrenEpoch.
type childrenLoadedMsg struct {
	key    string
	issues []jira.Issue
	err    error
	epoch  int
}

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

	keymap     Keymap
	helpBar    components.HelpBar
	searchBar  components.SearchBar
	modal      components.Modal
	jqlModal   components.JQLModal
	diffView   components.DiffView
	inputModal components.InputModal
	createForm components.CreateForm
	overlays   components.OverlayStack

	jqlFields []jira.AutocompleteField

	editTempPath string
	editContext  editCtx

	// pendingMention holds a write deferred until the user cache is loaded.
	pendingMention *pendingMention
	converter      ADFConverter

	onSelect    onSelectFunc
	onChecklist onChecklistFunc

	side            focusSide
	leftFocus       focusPanel
	projectKey      string
	projectID       string
	boardID         int
	boards          []jira.Board
	showHelp        bool
	helpCursor      int
	helpSearching   bool
	helpSearch      components.TextInput
	helpFilter      string
	logFlag         *bool
	isCloud         bool
	demoMode        bool
	currentUser     *jira.User
	usersCache      map[string][]jira.User
	issueCache      map[string]*jira.Issue
	childrenCache   map[string][]jira.Issue
	createMetaCache map[string][]jira.CreateMetaField
	// previewKey identifies the issue displayed in the right-side views.
	// Empty means nothing is displayed.
	previewKey string
	// previewEpoch is bumped on every PreviewRequestMsg. Debounce ticks
	// and fetch responses carry the epoch of the intent that spawned
	// them; handlers drop anything whose epoch no longer matches. This
	// is how we simulate "cancel the previous intent", which bubbletea
	// does not provide natively for tea.Cmd.
	previewEpoch int
	// parentEpoch is bumped on every showParent invocation. fetchParent
	// responses carry the epoch of the intent that spawned them; the
	// parentLoadedMsg handler drops anything whose epoch no longer
	// matches. Same staleness-cancellation pattern as previewEpoch.
	parentEpoch int
	// childrenEpoch is the analogous counter for Cloud Sub-tab GetChildren
	// fetches. Bumped on every ChildrenRequestMsg; responses with stale
	// epoch are dropped.
	childrenEpoch int
	pendingWalk   pendingWalk
	createCtx     createCtx

	gitRepoPath    string
	gitBranch      string
	gitDetectedKey string

	customCmds []config.ResolvedCustomCommand

	// savedTabs is the source of truth for the managed-tab store. Mutations
	// persist to saved_tabs.yml and re-sync issuesList via SetSavedTabs.
	savedTabs []config.ManagedTab

	// editingManagedTab is the store index whose query an open JQL modal will
	// overwrite on submit, or -1 when the modal performs a fresh search.
	editingManagedTab int

	panelSideW     int
	panelStatusH   int
	panelIssuesH   int
	panelInfoH     int
	panelProjectsH int
	panelDetailH   int
	panelLogH      int

	ctx    context.Context
	cancel context.CancelFunc
	cmdWg  sync.WaitGroup

	width  int
	height int
}

// AuthMethod describes how the user authenticated
type AuthMethod string

const (
	AuthSaved  AuthMethod = "Saved credentials (auth.json)"
	AuthEnv    AuthMethod = "Environment variables"
	AuthWizard AuthMethod = "Setup wizard"
	AuthDemo   AuthMethod = "Demo mode"
)

// newADFRenderer selects the ADF renderer impl based on cfg.Renderer.
// cfg.Renderer is validated at load time; "" and "builtin" both map to
// BuiltinRenderer. cfg.RendererStyle is resolved to a concrete Glamour
// style name here so terminal background detection happens once.
func newADFRenderer(cfg *config.Config) views.ADFRenderer {
	if cfg.Renderer == config.RendererGlamour {
		return views.GlamourRenderer{Style: views.ResolveGlamourStyle(cfg.RendererStyle)}
	}
	return views.BuiltinRenderer{}
}

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
	if len(cfg.GUI.TypeIcons) > 0 {
		issuesList.SetTypeIcons(cfg.GUI.TypeIcons)
	}
	if len(cfg.GUI.StatusIcons) > 0 {
		issuesList.SetStatusIcons(cfg.GUI.StatusIcons)
	}
	if len(cfg.GUI.PriorityIcons) > 0 {
		issuesList.SetPriorityIcons(cfg.GUI.PriorityIcons)
	}
	issuesList.SetTabs(cfg.IssueTabs)
	savedTabs := config.LoadSavedTabs()
	issuesList.SetSavedTabs(savedTabs)
	// Assemble for the initial project so the list's project filter matches
	// a.projectKey from the start; otherwise managed tabs stay hidden (and
	// fail to shadow their config namesakes) until the first project switch.
	issuesList.RebuildTabs(projectKey)
	issuesList.SetFocused(true)
	issuesList.SetUserEmail(cfg.Jira.Email)
	infoPanel := views.NewInfoPanel()
	if len(cfg.GUI.TypeIcons) > 0 {
		infoPanel.SetTypeIcons(cfg.GUI.TypeIcons)
	}
	projectList := views.NewProjectList()
	adfRenderer := newADFRenderer(cfg)
	detailView := views.NewDetailView(adfRenderer)
	logPanel := views.NewLogPanel()
	helpBar := components.NewHelpBar(nil)
	searchBar := components.NewSearchBar()
	modal := components.NewModal()
	diffView := components.NewDiffView()
	inputModal := components.NewInputModal()
	jqlModal := components.NewJQLModal()
	createForm := components.NewCreateForm(adfRenderer.Render)

	logFlag := new(bool)
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

	if len(cfg.Fields) > 0 {
		var customIDs []string
		for _, f := range cfg.Fields {
			if isCustomField(f.ID) {
				customIDs = append(customIDs, f.ID)
			}
		}
		if len(customIDs) > 0 {
			client.SetCustomFields(customIDs)
		}
		infoPanel.SetFields(cfg.Fields)
	}

	app := &App{
		cfg:               cfg,
		client:            client,
		keymap:            KeymapFromConfig(cfg.Keybinding),
		splashInfo:        splash,
		statusPanel:       statusPanel,
		issuesList:        issuesList,
		infoPanel:         infoPanel,
		projectList:       projectList,
		detailView:        detailView,
		logPanel:          logPanel,
		helpBar:           helpBar,
		searchBar:         searchBar,
		modal:             modal,
		jqlModal:          jqlModal,
		diffView:          diffView,
		inputModal:        inputModal,
		createForm:        createForm,
		savedTabs:         savedTabs,
		editingManagedTab: -1,
		side:              sideLeft,
		leftFocus:         focusIssues,
		projectKey:        projectKey,
		isCloud:           cfg.Jira.IsCloud(),
		demoMode:          authMethod == AuthDemo,
		logFlag:           logFlag,
		usersCache:        make(map[string][]jira.User),
		issueCache:        make(map[string]*jira.Issue),
		childrenCache:     make(map[string][]jira.Issue),
		createMetaCache:   make(map[string][]jira.CreateMetaField),
		converter:         BuiltinConverter{},
	}
	// cfg.Converter is validated at config-load time; "" and "builtin"
	// both fall through to the BuiltinConverter set above.
	if cfg.Converter == config.ConverterAdfConverter {
		app.converter = AdfConvConverter{}
	}
	app.ctx, app.cancel = context.WithCancel(context.Background()) //nolint:gosec // cancel is called in Shutdown()
	navResolver := app.keymap.MatchNav
	app.issuesList.ResolveNav = navResolver
	app.infoPanel.ResolveNav = navResolver
	app.infoPanel.SetCloud(app.isCloud)
	app.projectList.ResolveNav = navResolver
	app.detailView.ResolveNav = navResolver

	app.initCustomCommands()

	if warning := quitReachableWarning(app.keymap, app.customCmds); warning != "" {
		fmt.Fprintln(os.Stderr, "lazyjira:", warning)
		app.statusPanel.SetError(warning)
	}

	isCloud := cfg.Jira.IsCloud()
	app.createForm.SetDescRenderer(func(text string, width int) []string {
		return views.RenderDescriptionPreview(text, width, isCloud, adfRenderer)
	})

	app.overlays = components.OverlayStack{
		&app.createForm,
		&app.jqlModal,
		&app.inputModal,
		&app.diffView,
		&app.modal,
	}

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

// Shutdown cancels the app-lifetime context, signalling any background
// processes spawned with a.ctx to terminate, and waits for them to exit.
// Safe to call multiple times.
func (a *App) Shutdown() {
	if a.cancel != nil {
		a.cancel()
	}
	// Wait for background custom commands to finish (they receive the
	// cancel signal via exec.CommandContext). Give up after 3 seconds
	// so we never hang on exit.
	done := make(chan struct{})
	go func() { a.cmdWg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		fetchMyself(a.client),
		fetchFieldDiscovery(a.client),
		fetchProjects(a.client),
		fetchBoards(a.client),
		tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
			return autoFetchTickMsg{}
		}),
	}
	if cmd := a.fetchActiveTab(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.searchBar.IsActive() {
		if km, ok := msg.(tea.KeyMsg); ok && km.Type != tea.KeyUp && km.Type != tea.KeyDown && km.Type != tea.KeyCtrlJ && km.Type != tea.KeyCtrlK {
			updated, cmd := a.searchBar.Update(msg)
			a.searchBar = updated
			return a, cmd
		}
	}

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
	case myselfLoadedMsg:
		a.currentUser = msg.user
		return a, nil
	case fieldsDiscoveredMsg:
		if msg.err != nil {
			a.statusPanel.SetError(msg.err.Error())
		}
		return a, nil
	case boardsLoadedMsg:
		return a.handleBoardsLoaded(msg)
	case sprintsLoadedMsg:
		return a.handleSprintsLoaded(msg)
	case prefetchUsersMsg:
		if msg.projectKey == a.projectKey {
			if _, ok := a.usersCache[msg.projectKey]; !ok {
				return a, fetchUsers(a.client, msg.projectKey, "")
			}
		}
		return a, nil
	case usersLoadedMsg:
		return a.handleUsersLoaded(msg)
	case mentionUsersLoadedMsg:
		return a.handleMentionUsersLoaded(msg)
	case labelsLoadedMsg:
		return a.handleLabelsLoaded(msg)
	case componentsLoadedMsg:
		return a.handleComponentsLoaded(msg)
	case issueTypesLoadedMsg:
		return a.handleIssueTypesLoaded(msg)
	case customFieldOptionsMsg:
		return a.handleCustomFieldOptions(msg)
	case createMetaLoadedMsg:
		return a.handleCreateMetaLoaded(msg)
	case issueCreatedMsg:
		return a.handleIssueCreated(msg)
	case createErrorMsg:
		a.createForm.Resume()
		a.createForm.SetError(msg.err.Error())
		return a, nil
	case createPreFormErrorMsg:
		return a.handleCreatePreFormError(msg)
	case issueUpdatedMsg:
		return a.handleIssueUpdated(msg)
	case commentAddedMsg:
		return a, fetchIssueDetail(a.client, msg.issueKey)
	case commentUpdatedMsg:
		return a, fetchIssueDetail(a.client, msg.issueKey)

	case components.CreateFormTypeSelectedMsg:
		return a.handleCreateFormTypeSelected(msg)
	case components.CreateFormEditTextMsg:
		return a.handleCreateFormEditText(msg)
	case components.CreateFormEditExternalMsg:
		return a.handleCreateFormEditExternal(msg)
	case components.CreateFormPickerMsg:
		return a.handleCreateFormPicker(msg)
	case components.CreateFormChecklistMsg:
		return a.handleCreateFormChecklist(msg)
	case components.CreateFormSubmitMsg:
		return a.handleCreateFormSubmit(msg)
	case components.CreateFormCancelMsg:
		a.createCtx = createCtx{}
		return a, nil

	case components.ModalSelectedMsg:
		return a.handleModalSelected(msg)
	case components.ChecklistConfirmedMsg:
		return a.handleChecklistConfirmed(msg)
	case components.ModalCancelledMsg:
		return a.handleModalCancelled()

	case editorFinishedMsg:
		return a.handleEditorFinished(msg)
	case customCommandFinishedMsg:
		return a.handleCustomCommandFinished(msg)
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
	case components.JQLSaveTabMsg:
		return a.handleJQLSaveTab(msg)
	case jqlSearchResultMsg:
		return a.handleJQLSearchResult(msg)
	case jqlSearchErrorMsg:
		return a.handleJQLSearchError(msg)
	case components.JQLCancelMsg:
		a.editingManagedTab = -1
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
		a.createForm.Resume()
		errText := msg.err.Error()
		a.statusPanel.SetError(errText)
		a.modal.ShowError("Error", []components.ModalItem{
			{Label: errText},
		})
		return a, nil

	case views.IssueSelectedMsg:
		if msg.Issue == nil {
			return a, nil
		}
		if cached, ok := a.issueCache[msg.Issue.Key]; ok {
			a.infoPanel.SetIssue(cached)
		} else {
			a.infoPanel.SetIssue(msg.Issue)
		}
		_, previewCmd := a.Update(views.PreviewRequestMsg{Key: msg.Issue.Key})
		return a, tea.Batch(previewCmd, a.prefetchRelated(msg.Issue), a.infoPanel.MaybeChildrenRequest())

	case views.PreviewRequestMsg:
		a.previewKey = msg.Key
		a.previewEpoch++
		sel := a.issuesList.SelectedIssue()
		mainListMatches := sel != nil && sel.Key == msg.Key
		if cached, ok := a.issueCache[msg.Key]; ok && cached != nil {
			a.detailView.UpdateIssueData(cached)
			if mainListMatches {
				a.infoPanel.SetIssue(cached)
			}
			return a, nil
		}
		if mainListMatches {
			a.detailView.SetIssue(sel)
			a.infoPanel.SetIssue(sel)
		}
		epoch := a.previewEpoch
		key := msg.Key
		return a, tea.Tick(150*time.Millisecond, func(_ time.Time) tea.Msg {
			return previewDebounceMsg{key: key, epoch: epoch}
		})

	case previewDebounceMsg:
		if msg.epoch != a.previewEpoch {
			return a, nil
		}
		return a, fetchPreviewDetail(a.client, msg.key, a.previewEpoch)

	case views.ChildrenRequestMsg:
		if !a.isCloud || msg.Key == "" {
			return a, nil
		}
		if cached, ok := a.childrenCache[msg.Key]; ok {
			a.infoPanel.SetChildren(msg.Key, cached)
			return a, nil
		}
		a.childrenEpoch++
		return a, fetchChildren(a.client, msg.Key, a.childrenEpoch)

	case childrenWalkRequestMsg:
		if !a.isCloud || msg.key == "" {
			return a, nil
		}
		a.childrenEpoch++
		a.pendingWalk = pendingWalk{key: msg.key, epoch: a.childrenEpoch}
		return a, fetchChildren(a.client, msg.key, a.childrenEpoch)

	case childrenLoadedMsg:
		return a.handleChildrenLoaded(msg)

	case parentLoadedMsg:
		if msg.epoch != a.parentEpoch {
			return a, nil
		}
		if msg.err != nil || msg.parent == nil {
			return a, nil
		}
		a.pushNav("Parent", msg.parent.Key, navstack.SourceParent, []jira.Issue{*msg.parent})
		return a, a.previewAfterNav()

	case previewDetailLoadedMsg:
		if msg.epoch != a.previewEpoch {
			return a, nil
		}
		if msg.issue == nil {
			return a, nil
		}
		a.statusPanel.SetError("")
		*a.logFlag = false
		a.statusPanel.SetOnline(true)
		a.issueCache[msg.issue.Key] = msg.issue
		// DetailView only: InfoPanel belongs to the main list issue.
		a.detailView.UpdateIssueData(msg.issue)
		a.issuesList.PatchIssue(msg.issue)
		return a, nil
	case views.ProjectHoveredMsg:
		if msg.Project != nil {
			a.detailView.SetProject(msg.Project)
		}
		return a, nil
	}

	return a, a.routeToPanel(msg)
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	if a.isVerticalLayout() {
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

	a.helpBar.SetItems(a.helpBarItems())

	var bottomBar string
	switch {
	case a.searchBar.IsActive():
		bottomBar = a.searchBar.View()
	case a.createForm.IsVisible() && a.createForm.IsFiltering():
		bottomBar = a.createForm.FilterBarView()
	case a.jqlModal.IsVisible():
		bottomBar = a.helpBar.View()
	case a.modal.IsVisible() && a.modal.IsSearching():
		bottomBar = a.modal.SearchView(a.width)
	case a.showHelp && a.helpSearching:
		bottomBar = components.RenderFilterBarInput(&a.helpSearch)
	default:
		bottomBar = a.helpBar.View()
	}

	full := lipgloss.JoinVertical(lipgloss.Left, content, bottomBar)

	full = a.overlays.Render(full, a.width, a.height)
	if a.showHelp {
		full = a.renderHelpOverlay(full)
	}
	return full
}

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
			return a, fetchTransitions(a.client, sel.Key)
		case fldPriority:
			return a, fetchPriorities(a.client)
		case "issuetype":
			if a.projectID != "" {
				a.onSelect = a.makeFieldSelectCallback(sel.Key, "issuetype")
				return a, fetchIssueTypes(a.client, a.projectID)
			}
		case fldSprint:
			if a.boardID != 0 {
				return a, fetchSprints(a.client, a.boardID)
			}
			a.statusPanel.SetError("no agile board found for this project")
			return a, nil
		default:
			if isCustomField(field.FieldID) {
				return a.fetchCustomFieldOptionsForEdit(sel, field)
			}
		}
	case views.FieldPerson:
		a.onSelect = a.makePersonSelectCallback(sel.Key, field.FieldID)
		if cached, ok := a.usersCache[a.projectKey]; ok {
			return a.handleUsersLoaded(usersLoadedMsg{users: cached, issueKey: sel.Key})
		}
		return a, fetchUsers(a.client, a.projectKey, sel.Key)
	case views.FieldMultiSelect:
		issueKey := sel.Key
		switch field.FieldID {
		case fldLabels:
			a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
				labels := make([]string, 0, len(selected))
				for _, item := range selected {
					labels = append(labels, item.ID)
				}
				a.optimisticFieldUpdate(issueKey, fldLabels, labels)
				return updateIssueField(a.client, issueKey, fldLabels, labels)
			}
			return a, fetchLabels(a.client)
		case fldComponents:
			a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
				comps := make([]map[string]string, 0, len(selected))
				for _, item := range selected {
					comps = append(comps, map[string]string{"id": item.ID})
				}
				a.optimisticFieldUpdate(issueKey, fldComponents, comps)
				return updateIssueField(a.client, issueKey, fldComponents, comps)
			}
			return a, fetchComponents(a.client, a.projectKey)
		default:
			if isCustomField(field.FieldID) {
				return a.fetchCustomFieldOptionsForEdit(sel, field)
			}
		}
	case views.FieldSingleText:
		if isCustomField(field.FieldID) {
			return a.fetchCustomFieldOptionsForEdit(sel, field)
		}
		a.inputModal.Show("Edit "+field.Name, views.EditValueForField(sel, field.FieldID, field.Value))
		a.editContext = editCtx{kind: editField, issueKey: sel.Key, fieldID: field.FieldID}
		return a, nil
	case views.FieldMultiText:
		a.editContext = editCtx{kind: editFieldText, issueKey: sel.Key, fieldID: field.FieldID}
		return a, launchEditor(views.EditValueForField(sel, field.FieldID, field.Value), ".md")
	}
	return a, nil
}

func (a *App) optimisticFieldUpdate(issueKey, fieldID string, value any) {
	cached, ok := a.issueCache[issueKey]
	if !ok {
		return
	}
	switch fieldID {
	case "summary":
		if s, ok := value.(string); ok {
			cached.Summary = s
		}
	case fldDescription:
		if s, ok := value.(string); ok {
			cached.Description = s
		}
	default:
		if views.SetBuiltinFieldValue(cached, fieldID, value) {
			break
		}
		if strings.HasPrefix(fieldID, "customfield_") {
			if cached.CustomFields == nil {
				cached.CustomFields = make(map[string]any)
			}
			cached.CustomFields[fieldID] = value
		}
	}
	a.issueCache[issueKey] = cached
	if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == issueKey {
		a.infoPanel.SetIssue(cached)
	}
	a.issuesList.PatchIssue(cached)
}

func (a *App) applyEdit(mdContent string) tea.Cmd {
	ctx := a.editContext
	a.editContext = editCtx{}

	if a.isCloud && mentionsApply(ctx.kind) && hasMentionCandidate(mdContent) {
		pk := projectKeyFromIssueKey(ctx.issueKey)
		if pk == "" {
			pk = a.projectKey
		}
		if users, ok := a.projectUsers(pk); ok {
			return a.completeApplyEdit(pendingMention{content: mdContent, editContext: ctx, projectKey: pk}, users)
		}
		a.pendingMention = &pendingMention{content: mdContent, editContext: ctx, projectKey: pk}
		return fetchUsersForMention(a.client, pk)
	}
	return a.convertAndSubmit(ctx, mdContent)
}

// convertAndSubmit converts mdContent to ADF on cloud (no mention resolution)
// and submits the edit. Used for non-cloud edits and cloud edits without any
// @-mention to resolve.
func (a *App) convertAndSubmit(ctx editCtx, mdContent string) tea.Cmd {
	body := any(mdContent)
	if a.isCloud {
		adf, err := a.converter.FromMarkdown(mdContent, ctx.converterState)
		if err != nil {
			a.statusPanel.SetError("convert markdown: " + err.Error())
			return nil
		}
		body = adf
	}
	return a.submitEdit(ctx, body, mdContent)
}

// submitEdit dispatches a converted edit to the right write command. body is the
// payload sent to the API (ADF on cloud, markdown otherwise); md is the raw
// markdown used for optimistic local updates.
func (a *App) submitEdit(ctx editCtx, body any, md string) tea.Cmd {
	switch ctx.kind { //nolint:exhaustive
	case editDesc:
		a.optimisticFieldUpdate(ctx.issueKey, fldDescription, md)
		return updateIssueField(a.client, ctx.issueKey, fldDescription, body)
	case editCommentNew:
		return addComment(a.client, ctx.issueKey, body)
	case editCommentMod:
		return updateComment(a.client, ctx.issueKey, ctx.commentID, body)
	case editFieldText:
		a.optimisticFieldUpdate(ctx.issueKey, ctx.fieldID, md)
		return updateIssueField(a.client, ctx.issueKey, ctx.fieldID, md)
	}
	return nil
}

func (a *App) makePersonSelectCallback(issueKey, fieldID string) onSelectFunc {
	return func(item components.ModalItem) tea.Cmd {
		if item.ID == "" {
			a.optimisticFieldUpdate(issueKey, fieldID, nil)
			return updateIssueField(a.client, issueKey, fieldID, nil)
		}
		key := fldName
		if a.isCloud {
			key = fldAccountID
		}
		a.optimisticFieldUpdate(issueKey, fieldID, &jira.User{DisplayName: item.Label})
		return updateIssueField(a.client, issueKey, fieldID, map[string]string{key: item.ID})
	}
}

func (a *App) makeFieldSelectCallback(issueKey, fieldID string) onSelectFunc {
	return func(item components.ModalItem) tea.Cmd {
		a.optimisticFieldUpdate(issueKey, fieldID, map[string]any{"id": item.ID, "value": item.Label, "name": item.Label})
		return updateIssueField(a.client, issueKey, fieldID, map[string]string{"id": item.ID})
	}
}

func isCustomField(fieldID string) bool {
	return strings.HasPrefix(fieldID, "customfield_")
}

func (a *App) fieldMultilineEnabled(fieldID string) bool {
	for _, f := range a.cfg.Fields {
		if f.ID == fieldID {
			return f.Multiline
		}
	}
	return false
}

func (a *App) configuredFieldType(fieldID string) string {
	for _, f := range a.cfg.Fields {
		if f.ID == fieldID {
			return f.Type
		}
	}
	return ""
}

func (a *App) fetchCustomFieldOptionsForEdit(sel *jira.Issue, field *views.InfoField) (tea.Model, tea.Cmd) {
	if sel.IssueType == nil {
		a.statusPanel.SetError("issue type unknown")
		return a, nil
	}
	multiline := a.fieldMultilineEnabled(field.FieldID)
	cfgType := a.configuredFieldType(field.FieldID)
	if cfgType == "text" || cfgType == "textarea" {
		if multiline || cfgType == "textarea" {
			a.editContext = editCtx{kind: editFieldText, issueKey: sel.Key, fieldID: field.FieldID}
			return a, launchEditor(views.EditValueForInput(field.Value), ".md")
		}
		a.inputModal.Show("Edit "+field.Name, views.EditValueForInput(field.Value))
		a.editContext = editCtx{kind: editField, issueKey: sel.Key, fieldID: field.FieldID}
		return a, nil
	}
	info := customFieldOptionsMsg{
		issueKey:     sel.Key,
		fieldID:      field.FieldID,
		fieldName:    field.Name,
		fieldType:    field.Type,
		currentValue: field.Value,
		useEditor:    multiline,
	}
	cacheKey := a.projectKey + ":" + sel.IssueType.ID
	if cached, ok := a.createMetaCache[cacheKey]; ok {
		found := false
		for _, f := range cached {
			if f.FieldID == field.FieldID {
				info.options = f.AllowedValues
				info.schemaType = f.Schema.Type
				info.schemaItems = f.Schema.Items
				found = true
				break
			}
		}
		info.fieldNotFound = !found
		return a.handleCustomFieldOptions(info)
	}
	return a, fetchCustomFieldOptions(a.client, a.projectKey, sel.IssueType.ID, info)
}

func (a *App) handleCustomFieldOptions(msg customFieldOptionsMsg) (tea.Model, tea.Cmd) {
	if len(msg.allFields) > 0 && msg.issueTypeID != "" && msg.projectKey != "" {
		a.createMetaCache[msg.projectKey+":"+msg.issueTypeID] = msg.allFields
	}
	if msg.fieldNotFound {
		if msg.useEditor {
			a.editContext = editCtx{kind: editFieldText, issueKey: msg.issueKey, fieldID: msg.fieldID}
			return a, launchEditor(views.EditValueForInput(msg.currentValue), ".md")
		}
		a.inputModal.Show("Edit "+msg.fieldName, views.EditValueForInput(msg.currentValue))
		a.editContext = editCtx{kind: editField, issueKey: msg.issueKey, fieldID: msg.fieldID}
		return a, nil
	}
	if a.isPersonSchema(msg.schemaType, msg.schemaItems) {
		a.onSelect = a.makePersonSelectCallback(msg.issueKey, msg.fieldID)
		if cached, ok := a.usersCache[a.projectKey]; ok {
			return a.handleUsersLoaded(usersLoadedMsg{users: cached, issueKey: msg.issueKey})
		}
		return a, fetchUsers(a.client, a.projectKey, msg.issueKey)
	}

	items := make([]components.ModalItem, 0, len(msg.options))
	for _, v := range msg.options {
		items = append(items, components.ModalItem{ID: v.ID, Label: v.Name})
	}

	if len(items) == 0 {
		if msg.useEditor {
			a.editContext = editCtx{kind: editFieldText, issueKey: msg.issueKey, fieldID: msg.fieldID}
			return a, launchEditor(views.EditValueForInput(msg.currentValue), ".md")
		}
		a.inputModal.Show("Edit "+msg.fieldName, views.EditValueForInput(msg.currentValue))
		a.editContext = editCtx{kind: editField, issueKey: msg.issueKey, fieldID: msg.fieldID}
		return a, nil
	}

	switch msg.fieldType {
	case views.FieldMultiSelect:
		a.onChecklist = func(selected []components.ModalItem) tea.Cmd {
			vals := make([]map[string]string, 0, len(selected))
			for _, item := range selected {
				vals = append(vals, map[string]string{"id": item.ID})
			}
			a.optimisticFieldUpdate(msg.issueKey, msg.fieldID, vals)
			return updateIssueField(a.client, msg.issueKey, msg.fieldID, vals)
		}
		preselected := make(map[string]bool)
		if raw, ok := a.issueCache[msg.issueKey]; ok {
			if arr, ok := raw.CustomFields[msg.fieldID].([]any); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]any); ok {
						if id, ok := m["id"].(string); ok {
							preselected[id] = true
						}
					}
				}
			}
		}
		a.modal.ShowChecklist(msg.fieldName, items, preselected)
	default:
		a.onSelect = a.makeFieldSelectCallback(msg.issueKey, msg.fieldID)
		a.modal.Show(msg.fieldName, items)
	}
	return a, nil
}

func (a *App) isPersonSchema(schemaType, schemaItems string) bool {
	return schemaType == schemaUser || (schemaType == schemaArray && schemaItems == schemaUser)
}

func (a *App) renderHelpOverlay(base string) string {
	bindings := a.filteredHelpBindings()

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

	keyNormal := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	keySel := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true).Background(theme.ColorHighlight)
	descSel := lipgloss.NewStyle().Background(theme.ColorHighlight)

	lines := make([]string, 0, len(bindings)+2)
	lines = append(lines, "")
	descMaxW := popupW - maxKey - 6
	for i, b := range bindings {
		padded := b.Key
		for len(padded) < maxKey {
			padded += " "
		}
		desc := components.TruncateEnd(b.Description, descMaxW)
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

	return components.Overlay(base, content, a.width, a.height)
}

func (a *App) handleGitBranchSwitch(name string) (tea.Model, tea.Cmd) {
	a.gitBranch = name
	a.helpBar.SetStatusMsg(name)
	if a.cfg.Git.CloseOnCheckout {
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) fetchActiveTab() tea.Cmd {
	if a.issuesList.IsJQLTab() {
		jql := a.issuesList.JQLQuery()
		if jql == "" {
			return nil
		}
		tabIdx := a.issuesList.GetTabIndex()
		*a.logFlag = true
		return fetchIssuesByJQL(a.client, jql, tabIdx, a.cfg.ResolveGlobalMaxResults(), a.issuesList.TabEpoch())
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
	return fetchIssuesByJQL(a.client, jql, tabIdx, a.cfg.ResolveMaxResults(tab), a.issuesList.TabEpoch())
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
