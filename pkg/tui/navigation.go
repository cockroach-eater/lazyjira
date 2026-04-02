package tui

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// showCachedIssue updates the detail view with the cached version of the given issue key.
func (a *App) showCachedIssue(key string) {
	if cached, ok := a.issueCache[key]; ok {
		a.detailView.SetIssue(cached)
		a.infoPanel.SetIssue(cached)
	}
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
