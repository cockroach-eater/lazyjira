package tui

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/views"
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
	a.detailView.SetIssue(cached)
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
	if cached, ok := a.issueCache[sel.Key]; ok {
		a.detailView.SetIssue(cached)
		a.infoPanel.SetIssue(cached)
	} else {
		a.detailView.SetIssue(sel)
		a.infoPanel.SetIssue(sel)
	}
	return a.prefetchRelated(sel)
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

func copyToClipboard(text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	name, args := platformCommand("copy", "")
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

func openBrowser(url string) {
	name, args := platformCommand("open", url)
	cmd := exec.CommandContext(context.Background(), name, args...)
	_ = cmd.Start()
}
