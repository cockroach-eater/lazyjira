package tui

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/views"
)

// currentIssue returns the issue the user is currently looking at: the
// previewed issue if cached, otherwise a stub carrying just the preview key.
// Falls back to the list selection only when no preview is active. User-
// initiated actions (edit, copy URL, transition, custom commands, ...)
// operate on the result so they target what is on screen even when a sub or
// link is being previewed. The stub covers the brief window between a
// preview request firing and the fetch response populating the cache; actions
// that need more than the key (edit summary/description) must accept a key-
// only stub gracefully.
func (a *App) currentIssue() *jira.Issue {
	if a.previewKey != "" {
		if cached, ok := a.issueCache[a.previewKey]; ok && cached != nil {
			return cached
		}
		return &jira.Issue{Key: a.previewKey}
	}
	return a.issuesList.SelectedIssue()
}

// showCachedIssue updates the detail view with the cached version of the
// given issue key. The InfoPanel is only updated when the key matches the
// main list selection; otherwise the panel stays with the main issue so its
// tab and cursor are preserved.
func (a *App) showCachedIssue(key string) {
	cached, ok := a.issueCache[key]
	if !ok {
		return
	}
	if !a.boardVisible() {
		a.detailView.SetIssue(cached)
	}
	if sel := a.issuesList.SelectedIssue(); sel != nil && sel.Key == key {
		a.infoPanel.SetIssue(cached)
	}
}

func (a *App) previewSelectedIssue() tea.Cmd {
	sel := a.issuesList.SelectedIssue()
	if sel == nil {
		return nil
	}
	a.previewKey = sel.Key
	// While the board owns the right panel it only tracks the cursor; the
	// InfoPanel still follows the selection.
	board := a.boardVisible()
	if board {
		a.detailView.Kanban().SelectByKey(sel.Key)
	}
	issue := sel
	if cached, ok := a.issueCache[sel.Key]; ok {
		issue = cached
	}
	a.showPreviewIssue(issue, board, true, true)
	return tea.Batch(a.prefetchRelated(sel), a.infoPanel.MaybeChildrenRequest())
}

// previewForInfoTab refreshes the preview for the current InfoPanel tab, so
// entering Sub or Lnk immediately previews its first entry. Fields reverts
// to the main issue; empty lists dispatch nothing.
func (a *App) previewForInfoTab() tea.Cmd {
	switch a.infoPanel.ActiveTab() {
	case views.InfoTabFields:
		return a.previewSelectedIssue()
	case views.InfoTabSubtasks:
		if key := a.infoPanel.SelectedSubtaskKey(); key != "" {
			return func() tea.Msg { return views.PreviewRequestMsg{Key: key} }
		}
	case views.InfoTabLinks:
		if key := a.infoPanel.SelectedLinkKey(); key != "" {
			return func() tea.Msg { return views.PreviewRequestMsg{Key: key} }
		}
	}
	return nil
}

// extractIssueKey checks if a URL points to our Jira and extracts the issue key.
// e.g. https://didlogic.atlassian.net/browse/DR-13819 → "DR-13819"
func (a *App) extractIssueKey(url string) string {
	host := strings.TrimRight(a.cfg.Jira.Host, "/")
	prefix := host + "/browse/"
	key, found := strings.CutPrefix(url, prefix)
	if found {
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
// If not, switches to All tab and tries again
func (a *App) navigateToIssue(key string) {
	// Try current tab first.
	if a.issuesList.SelectByKey(key) {
		a.side = sideLeft
		a.leftFocus = focusIssues
		a.updateFocusState()
		a.showCachedIssue(key)
		return
	}
	// Switch to first tab (typically "All") and try again.
	if a.issuesList.GetTabIndex() != 0 {
		a.issuesList.SetTabIndex(0)
		if a.issuesList.SelectByKey(key) {
			a.side = sideLeft
			a.leftFocus = focusIssues
			a.updateFocusState()
			a.showCachedIssue(key)
			return
		}
	}
	// Not in our list — open in browser as fallback.
	openBrowser(a.cfg.Jira.Host + "/browse/" + key)
}

// platformCommand returns the OS-specific command name and args for the given action.
func platformCommand(action string, arg string) (name string, args []string) {
	switch action {
	case "open":
		switch runtime.GOOS {
		case "darwin":
			return "open", []string{arg}
		case "windows":
			return "rundll32", []string{"url.dll,FileProtocolHandler", arg}
		default:
			return "xdg-open", []string{arg}
		}
	case "copy":
		switch runtime.GOOS {
		case "darwin":
			return "pbcopy", nil
		case "windows":
			return "clip", nil
		default:
			return "xclip", []string{"-selection", "clipboard"}
		}
	}
	return "", nil
}

var runExternalCommand = execExternal

func execExternal(input string, waitForExit bool, name string, args ...string) {
	if name == "" {
		return
	}
	ctx := context.Background()
	if waitForExit {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	if waitForExit {
		_ = cmd.Run()
		return
	}
	_ = cmd.Start()
}

func copyToClipboard(text string) {
	name, args := platformCommand("copy", "")
	runExternalCommand(text, true, name, args...)
}

func openBrowser(url string) {
	name, args := platformCommand("open", url)
	runExternalCommand("", false, name, args...)
}

// previewDebounce is how long a cursor must rest on an issue before its detail
// is fetched, so scrolling a list does not fire a request per row.
const previewDebounce = 150 * time.Millisecond

// handlePreviewRequest points the right-hand views at an issue. A cached issue
// is shown at once; otherwise a debounced fetch is scheduled, tagged with the
// current epoch so a later request supersedes it.
//
// While the board owns the panel the detail view is left alone: only the board
// cursor and the InfoPanel follow the selection.
func (a *App) handlePreviewRequest(msg views.PreviewRequestMsg) (tea.Model, tea.Cmd) {
	a.previewKey = msg.Key
	a.previewEpoch++

	sel := a.issuesList.SelectedIssue()
	mainListMatches := sel != nil && sel.Key == msg.Key

	board := a.boardVisible()
	if board {
		a.detailView.Kanban().SelectByKey(msg.Key)
	}

	if cached, ok := a.issueCache[msg.Key]; ok && cached != nil {
		a.showPreviewIssue(cached, board, mainListMatches, false)
		return a, nil
	}
	if mainListMatches {
		a.showPreviewIssue(sel, board, true, true)
	}

	epoch, key := a.previewEpoch, msg.Key
	return a, tea.Tick(previewDebounce, func(_ time.Time) tea.Msg {
		return previewDebounceMsg{key: key, epoch: epoch}
	})
}

// showPreviewIssue routes an issue to the detail and info panels. reset picks
// SetIssue over UpdateIssueData, which also rewinds the tab and scroll.
func (a *App) showPreviewIssue(issue *jira.Issue, board, updateInfo, reset bool) {
	if !board {
		if reset {
			a.detailView.SetIssue(issue)
		} else {
			a.detailView.UpdateIssueData(issue)
		}
	}
	if updateInfo {
		a.infoPanel.SetIssue(issue)
	}
}
