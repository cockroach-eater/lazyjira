package views

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

func navResolverForDetail(key string) components.NavAction {
	switch key {
	case "j":
		return components.NavDown
	case "k":
		return components.NavUp
	case "g":
		return components.NavTop
	case "G":
		return components.NavBottom
	case "ctrl+d":
		return components.NavHalfDown
	case "ctrl+u":
		return components.NavHalfUp
	}
	return components.NavNone
}

func makeFocusedDetail() *DetailView {
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	detail.SetFocused(true)
	detail.ResolveNav = navResolverForDetail
	return detail
}

func makeDetailIssue() *jira.Issue {
	return &jira.Issue{
		Key:         testKey,
		Summary:     "test issue summary",
		Description: "description body",
		Status:      &jira.Status{Name: "Open"},
	}
}

func TestDetailView_NewDetailView_SetsDefaults(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	testkit.AssertEqual(t, "mode", detail.Mode(), ModeIssue)
}

func TestDetailView_Mode_ReturnsCurrentMode(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSplash(SplashInfo{Version: "v1"})
	testkit.AssertEqual(t, "splash mode", detail.Mode(), ModeSplash)
}

func TestDetailView_IssueKey_ReturnsKey(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(makeDetailIssue())
	testkit.AssertEqual(t, "issue key", detail.IssueKey(), testKey)
}

func TestDetailView_IssueKey_EmptyWhenNoIssue(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	testkit.AssertEqual(t, "no issue key", detail.IssueKey(), "")
}

func TestDetailView_IssueKey_EmptyInSplashMode(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(makeDetailIssue())
	detail.SetSplash(SplashInfo{Version: "v1"})
	testkit.AssertEqual(t, "splash key empty", detail.IssueKey(), "")
}

func TestDetailView_SetIssue_ResetsScrollOnNewKey(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.scrollY = 10
	detail.SetIssue(&jira.Issue{Key: testKey2, Summary: "other"})
	testkit.AssertEqual(t, "scroll reset", detail.scrollY, 0)
}

func TestDetailView_SetIssue_KeepsScrollOnSameKey(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.scrollY = 5
	detail.SetIssue(makeDetailIssue())
	testkit.AssertEqual(t, "scroll preserved", detail.scrollY, 5)
}

func TestDetailView_UpdateIssueData_ResetsScrollOnNewKey(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.scrollY = 7
	detail.UpdateIssueData(&jira.Issue{Key: testKey2, Summary: "updated"})
	testkit.AssertEqual(t, "scroll reset on new key", detail.scrollY, 0)
}

func TestDetailView_UpdateIssueData_KeepsScrollSameKey(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.scrollY = 3
	detail.UpdateIssueData(makeDetailIssue())
	testkit.AssertEqual(t, "scroll preserved same key", detail.scrollY, 3)
}

func TestDetailView_SetProject_SetsProjectMode(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetProject(&jira.Project{Key: "PROJ", Name: "Project Alpha"})
	testkit.AssertEqual(t, "project mode", detail.Mode(), ModeProject)
	testkit.AssertEqual(t, "scroll reset", detail.scrollY, 0)
}

func TestDetailView_SetSplash_SetsSplashMode(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSplash(SplashInfo{Version: "v1.0", Email: testEmail})
	testkit.AssertEqual(t, "splash mode", detail.Mode(), ModeSplash)
}

func TestDetailView_SetSize_UpdatesDimensions(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(100, 30)
	testkit.AssertEqual(t, "width", detail.width, 100)
	testkit.AssertEqual(t, "height", detail.height, 30)
}

func TestDetailView_SetFocused_ResetsListCursorOnBlur(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.listCursor = 3
	detail.SetFocused(false)
	testkit.AssertEqual(t, "cursor reset on blur", detail.listCursor, 0)
}

func TestDetailView_SetFocused_NoResetWhenGainingFocus(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(makeDetailIssue())
	detail.listCursor = 2
	detail.SetFocused(false)
	detail.listCursor = 2
	detail.SetFocused(true)
	testkit.AssertEqual(t, "cursor unchanged when gaining focus", detail.listCursor, 2)
}

func TestDetailView_ActiveTab_ReturnsTab(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	testkit.AssertEqual(t, "default tab", detail.ActiveTab(), TabDetails)
}

func TestDetailView_SetActiveTab_ChangesTab(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(makeDetailIssue())
	detail.SetActiveTab(TabComments)
	testkit.AssertEqual(t, "active tab", detail.ActiveTab(), TabComments)
	testkit.AssertEqual(t, "scroll reset", detail.scrollY, 0)
	testkit.AssertEqual(t, "cursor reset", detail.listCursor, 0)
}

func TestDetailView_SelectedComment_ReturnsComment(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Comments: []jira.Comment{
			{Body: "first comment"},
			{Body: "second comment"},
		},
	})
	detail.SetActiveTab(TabComments)
	detail.listCursor = 1
	comment := detail.SelectedComment()
	if comment == nil {
		t.Fatal("SelectedComment() should return non-nil")
	}
	testkit.AssertEqual(t, "comment body", comment.Body, "second comment")
}

func TestDetailView_SelectedComment_NilWhenNotCommentTab(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "comment"}},
	})
	if detail.SelectedComment() != nil {
		t.Error("SelectedComment() should be nil on details tab")
	}
}

func TestDetailView_Init_ReturnsNil(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	if cmd := detail.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestDetailView_NextTab_CyclesForward(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c"}},
	})
	detail.NextTab()
	testkit.AssertEqual(t, "moved to comments", detail.ActiveTab(), TabComments)
}

func TestDetailView_NextTab_WrapsAround(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c"}},
	})
	detail.NextTab()
	detail.NextTab()
	testkit.AssertEqual(t, "wrapped back to details", detail.ActiveTab(), TabDetails)
}

func TestDetailView_PrevTab_CyclesBack(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c"}},
	})
	detail.PrevTab()
	testkit.AssertEqual(t, "prev from details wraps to comments", detail.ActiveTab(), TabComments)
}

func TestDetailView_ClickTab_SwitchesToComments(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "comment"}},
	})
	detail.ClickTab(len("[0] " + testKey + " - Body - "))
	testkit.AssertEqual(t, "clicked to comments", detail.ActiveTab(), TabComments)
}

func TestDetailView_ClickTab_NilIssueNoop(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.ClickTab(20)
	testkit.AssertEqual(t, "tab unchanged", detail.ActiveTab(), TabDetails)
}

func TestDetailView_ScrollBy_ScrollsContentTab(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.ScrollBy(3)
	testkit.AssertEqual(t, "scrolled down", detail.scrollY, 3)
}

func TestDetailView_ScrollBy_ClampedAtZero(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.ScrollBy(-5)
	testkit.AssertEqual(t, "scroll clamped at 0", detail.scrollY, 0)
}

func TestDetailView_ScrollBy_MovesListCursorOnListTab(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}, {Body: "c2"}, {Body: "c3"}},
	})
	detail.SetActiveTab(TabComments)
	detail.ScrollBy(1)
	testkit.AssertEqual(t, "list cursor moved", detail.listCursor, 1)
}

func TestDetailView_ClickItem_NilWhenNotListTab(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	cmd := detail.ClickItem(2)
	if cmd != nil {
		t.Error("ClickItem on non-list tab should return nil")
	}
}

func TestDetailView_ClickItem_NilIssue(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	cmd := detail.ClickItem(2)
	if cmd != nil {
		t.Error("ClickItem with no issue should return nil")
	}
}

func TestDetailView_ClickItem_SingleClickSelectsBlock(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "first"}, {Body: "second"}},
	})
	detail.SetActiveTab(TabComments)
	detail.View()
	cmd := detail.ClickItem(2)
	if cmd != nil {
		t.Error("single click should return nil cmd (not double click)")
	}
}

func TestDetailView_ClickItem_NegativeLineNoop(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c"}},
	})
	detail.SetActiveTab(TabComments)
	cmd := detail.ClickItem(0)
	if cmd != nil {
		t.Error("click on title line should return nil")
	}
}

func TestDetailView_ClickItem_AccountsForSummaryHeader(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Summary:  "Short summary",
		Comments: []jira.Comment{{Body: "first"}, {Body: "second"}},
	})
	detail.SetActiveTab(TabComments)

	contentWidth, _ := components.PanelDimensions(detail.width, detail.height)
	header := detail.summaryHeaderLines(contentWidth)
	if len(header) == 0 {
		t.Fatal("expected a non-empty summary header")
	}

	detail.ClickItem(1 + len(header))
	testkit.AssertEqual(t, "cursor selects first block accounting for header offset", detail.listCursor, 0)
}

func TestDetailView_IsListTab_FalseForDetails(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	testkit.AssertEqual(t, "details not list tab", detail.IsListTab(), false)
}

func TestDetailView_IsListTab_TrueForComments(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetActiveTab(TabComments)
	testkit.AssertEqual(t, "comments is list tab", detail.IsListTab(), true)
}

func TestDetailView_IsListTab_TrueForHistory(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetActiveTab(TabHistory)
	testkit.AssertEqual(t, "history is list tab", detail.IsListTab(), true)
}

func TestDetailView_ListCursorUp_DecrementsCursor(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}, {Body: "c2"}},
	})
	detail.SetActiveTab(TabComments)
	detail.listCursor = 1
	detail.ListCursorUp()
	testkit.AssertEqual(t, "cursor decremented", detail.listCursor, 0)
}

func TestDetailView_ListCursorUp_ClampedAtZero(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}},
	})
	detail.SetActiveTab(TabComments)
	detail.ListCursorUp()
	testkit.AssertEqual(t, "cursor clamped at 0", detail.listCursor, 0)
}

func TestDetailView_ListCursorDown_IncrementsCursor(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}, {Body: "c2"}},
	})
	detail.SetActiveTab(TabComments)
	detail.ListCursorDown()
	testkit.AssertEqual(t, "cursor incremented", detail.listCursor, 1)
}

func TestDetailView_ListCursorDown_ClampedAtMax(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}},
	})
	detail.SetActiveTab(TabComments)
	detail.listCursor = 0
	detail.ListCursorDown()
	testkit.AssertEqual(t, "cursor clamped at last", detail.listCursor, 0)
}

func TestDetailView_VisibleRows_BasedOnHeight(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	testkit.AssertEqual(t, "visible rows", detail.VisibleRows(), 22)
}

func TestDetailView_Update_UnfocusedReturnsNilCmd(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetIssue(makeDetailIssue())
	detail.SetFocused(false)
	detail.ResolveNav = navResolverForDetail
	_, cmd := detail.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if cmd != nil {
		t.Error("unfocused Update should return nil cmd")
	}
}

func TestDetailView_Update_TabKeyCallsNextTab(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c"}},
	})
	detail.Update(tea.KeyMsg{Type: tea.KeyTab})
	testkit.AssertEqual(t, "tab switched", detail.ActiveTab(), TabComments)
}

func TestDetailView_Update_EnterOnExpandableEmitsExpandBlockMsg(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	longLines := make([]string, maxBlockLines+2)
	for i := range longLines {
		longLines[i] = "line content"
	}
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Comments: []jira.Comment{
			{Body: strings.Join(longLines, "\n")},
		},
	})
	detail.SetActiveTab(TabComments)
	detail.View()
	_, cmd := detail.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on long block should return Cmd")
	}
	msg := cmd()
	if _, ok := msg.(ExpandBlockMsg); !ok {
		t.Fatalf("expected ExpandBlockMsg, got %T", msg)
	}
}

func TestDetailView_Update_NavDown_ScrollsDescription(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	longDesc := strings.Repeat("line of text\n", 30)
	detail.SetIssue(&jira.Issue{Key: testKey, Description: longDesc})
	detail.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if detail.scrollY == 0 {
		t.Error("nav down should increase scrollY on description tab")
	}
}

func TestDetailView_Update_NavUp_ScrollsDescriptionUp(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	longDesc := strings.Repeat("line of text\n", 30)
	detail.SetIssue(&jira.Issue{Key: testKey, Description: longDesc})
	detail.scrollY = 5
	detail.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	testkit.AssertEqual(t, "scrolled up", detail.scrollY, 4)
}

func TestDetailView_Update_NavTop_ResetsScrollAndCursor(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}, {Body: "c2"}},
	})
	detail.SetActiveTab(TabComments)
	detail.listCursor = 1
	detail.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	testkit.AssertEqual(t, "cursor at top", detail.listCursor, 0)
}

func TestDetailView_Update_NavBottom_MovesListCursorToEnd(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}, {Body: "c2"}, {Body: "c3"}},
	})
	detail.SetActiveTab(TabComments)
	detail.View()
	detail.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	testkit.AssertEqual(t, "cursor at bottom", detail.listCursor, 2)
}

func TestDetailView_Update_HalfPageDown_ScrollsHalfPage(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	longDesc := strings.Repeat("line of text\n", 50)
	detail.SetIssue(&jira.Issue{Key: testKey, Description: longDesc})
	detail.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if detail.scrollY == 0 {
		t.Error("ctrl+d should scroll down half page")
	}
}

func TestDetailView_Update_HalfPageUp_ClampedAtZero(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	detail.scrollY = 0
	detail.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	testkit.AssertEqual(t, "scroll clamped at 0", detail.scrollY, 0)
}

func TestDetailView_Update_NonNavKey_NoChange(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(makeDetailIssue())
	before := detail.scrollY
	detail.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	testkit.AssertEqual(t, "no scroll change", detail.scrollY, before)
}

func TestDetailView_View_SplashMode_ContainsVersion(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	detail.SetSplash(SplashInfo{Version: "v1.2.3", Email: testEmail})
	output := stripANSI(detail.View())
	if !strings.Contains(output, "v1.2.3") {
		t.Errorf("splash view = %q, want to contain version v1.2.3", output)
	}
}

func TestDetailView_View_SplashMode_ContainsEmail(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	detail.SetSplash(SplashInfo{Email: testEmail, AuthMethod: "basic"})
	output := stripANSI(detail.View())
	if !strings.Contains(output, testEmail) {
		t.Errorf("splash view = %q, want to contain email %s", output, testEmail)
	}
}

func TestDetailView_View_SplashMode_ContainsProject(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	detail.SetSplash(SplashInfo{Project: testProject, AuthMethod: "basic"})
	output := stripANSI(detail.View())
	if !strings.Contains(output, testProject) {
		t.Errorf("splash view with project = %q, want to contain project %s", output, testProject)
	}
}

func TestDetailView_View_ProjectMode_ContainsProjectInfo(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	detail.SetProject(&jira.Project{
		Key:  "PROJ",
		Name: "Project Alpha",
		ID:   "10000",
		Lead: &jira.User{DisplayName: "Alice"},
	})
	output := stripANSI(detail.View())
	if !strings.Contains(output, "PROJ") {
		t.Errorf("project view = %q, want to contain key PROJ", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Errorf("project view = %q, want to contain lead Alice", output)
	}
	if !strings.Contains(output, "10000") {
		t.Errorf("project view = %q, want to contain ID 10000", output)
	}
}

func TestDetailView_View_NilIssue_ShowsPlaceholder(t *testing.T) {
	t.Parallel()
	detail := NewDetailView(BuiltinRenderer{})
	detail.SetSize(80, 24)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "Select an issue") {
		t.Errorf("nil issue view = %q, want placeholder", output)
	}
}

func TestDetailView_View_DescriptionBodyShown(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:         testKey,
		Description: "this is the description body text",
	})
	output := stripANSI(detail.View())
	if !strings.Contains(output, "description body text") {
		t.Errorf("detail view = %q, want description body", output)
	}
}

func TestDetailView_View_ShowsFullSummaryWrapped(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	longSummary := "This is a deliberately long issue summary that will not fit on a single narrow terminal row"
	detail.SetIssue(&jira.Issue{
		Key:         testKey,
		Summary:     longSummary,
		Description: "body text",
	})
	output := stripANSI(detail.View())
	for _, word := range strings.Fields(longSummary) {
		if !strings.Contains(output, word) {
			t.Errorf("detail view missing summary word %q; view = %q", word, output)
		}
	}
	if strings.Contains(output, "…") {
		t.Errorf("summary should wrap, not truncate with ellipsis; view = %q", output)
	}
}

func TestDetailView_View_CommentsTab_ShowsAuthor(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:     testKey,
		Summary: "Visible even on the comments tab",
		Comments: []jira.Comment{
			{
				Author: &jira.User{DisplayName: "Bob"},
				Body:   "great idea",
			},
		},
	})
	detail.SetActiveTab(TabComments)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "Bob") {
		t.Errorf("comments view = %q, want to contain author Bob", output)
	}
	if !strings.Contains(output, "great idea") {
		t.Errorf("comments view = %q, want to contain comment body", output)
	}
	if !strings.Contains(output, "Visible even on the comments tab") {
		t.Errorf("comments view = %q, want summary header present on non-Body tab", output)
	}
}

func TestDetailView_View_HistoryTab_ShowsChanges(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Changelog: []jira.ChangelogEntry{
			{
				Author:  &jira.User{DisplayName: "Alice"},
				Created: time.Now().Add(-time.Hour),
				Items: []jira.ChangeItem{
					{Field: "status", FromString: "Open", ToString: "In Progress"},
				},
			},
		},
	})
	detail.SetActiveTab(TabHistory)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "Alice") {
		t.Errorf("history view = %q, want to contain author Alice", output)
	}
	if !strings.Contains(output, "status") {
		t.Errorf("history view = %q, want to contain 'status' field", output)
	}
}

func TestDetailView_View_HistoryTab_UnknownAuthor(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Changelog: []jira.ChangelogEntry{
			{
				Created: time.Now().Add(-time.Hour),
				Items: []jira.ChangeItem{
					{Field: "summary", FromString: "old", ToString: "new"},
				},
			},
		},
	})
	detail.SetActiveTab(TabHistory)
	output := stripANSI(detail.View())
	if !strings.Contains(output, unknownLabel) {
		t.Errorf("history view = %q, want to contain %q for nil author", output, unknownLabel)
	}
}

func TestDetailView_View_TabTitleContainsTabs(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c"}},
	})
	output := stripANSI(detail.View())
	if !strings.Contains(output, "Body") {
		t.Errorf("detail view = %q, want tab label 'Body'", output)
	}
	if !strings.Contains(output, "Cmt") {
		t.Errorf("detail view = %q, want tab label 'Cmt'", output)
	}
}

func TestDetailView_View_FooterOnListTab(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key:      testKey,
		Comments: []jira.Comment{{Body: "c1"}, {Body: "c2"}},
	})
	detail.SetActiveTab(TabComments)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "of 2") {
		t.Errorf("comments view footer = %q, want to contain 'of 2'", output)
	}
}

func TestDetailView_RenderDescription_ADFRendered(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		DescriptionADF: map[string]any{
			"type": "doc",
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{"type": "text", "text": "adf content here"},
					},
				},
			},
		},
	})
	output := stripANSI(detail.View())
	if !strings.Contains(output, "adf content here") {
		t.Errorf("adf description view = %q, want to contain ADF content", output)
	}
}

func TestDetailView_RenderHistoryBlocks_StatusField(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Changelog: []jira.ChangelogEntry{
			{
				Author:  &jira.User{DisplayName: "Dev"},
				Created: time.Now().Add(-2 * time.Hour),
				Items: []jira.ChangeItem{
					{Field: fieldStatus, FromString: "Open", ToString: "Done"},
				},
			},
		},
	})
	detail.SetActiveTab(TabHistory)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "Done") {
		t.Errorf("history status field = %q, want to contain 'Done'", output)
	}
}

func TestDetailView_RenderHistoryBlocks_PersonField(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Changelog: []jira.ChangelogEntry{
			{
				Author:  &jira.User{DisplayName: "Manager"},
				Created: time.Now().Add(-time.Hour),
				Items: []jira.ChangeItem{
					{Field: "assignee", FromString: "Alice", ToString: "Bob"},
				},
			},
		},
	})
	detail.SetActiveTab(TabHistory)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "Bob") {
		t.Errorf("history person field = %q, want to contain 'Bob'", output)
	}
}

func TestDetailView_RenderHistoryBlocks_DescriptionField(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Changelog: []jira.ChangelogEntry{
			{
				Author:  &jira.User{DisplayName: "Writer"},
				Created: time.Now().Add(-time.Hour),
				Items: []jira.ChangeItem{
					{Field: "description", FromString: "old text", ToString: "new text"},
				},
			},
		},
	})
	detail.SetActiveTab(TabHistory)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "description") {
		t.Errorf("history description field = %q, want 'description'", output)
	}
}

func TestDetailView_RenderHistoryBlocks_LabelsField(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Changelog: []jira.ChangelogEntry{
			{
				Author:  &jira.User{DisplayName: "Tagger"},
				Created: time.Now().Add(-time.Hour),
				Items: []jira.ChangeItem{
					{Field: "labels", FromString: "alpha", ToString: "alpha,beta"},
				},
			},
		},
	})
	detail.SetActiveTab(TabHistory)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "beta") {
		t.Errorf("history labels field = %q, want to contain 'beta'", output)
	}
}

func TestDetailView_RenderCommentBlocks_ADFBody(t *testing.T) {
	t.Parallel()
	detail := makeFocusedDetail()
	detail.SetIssue(&jira.Issue{
		Key: testKey,
		Comments: []jira.Comment{
			{
				Author: &jira.User{DisplayName: "Commenter"},
				BodyADF: map[string]any{
					"type": "doc",
					"content": []any{
						map[string]any{
							"type": "paragraph",
							"content": []any{
								map[string]any{"type": "text", "text": "adf comment body"},
							},
						},
					},
				},
			},
		},
	})
	detail.SetActiveTab(TabComments)
	output := stripANSI(detail.View())
	if !strings.Contains(output, "adf comment body") {
		t.Errorf("adf comment view = %q, want ADF comment content", output)
	}
}

func TestIsPersonField_RecognizesPersonFields(t *testing.T) {
	t.Parallel()
	testkit.AssertEqual(t, "assignee is person", isPersonField("assignee"), true)
	testkit.AssertEqual(t, "reporter is person", isPersonField("reporter"), true)
	testkit.AssertEqual(t, "summary is not person", isPersonField("summary"), false)
}

func TestIsMultiSelectField_RecognizesMultiFields(t *testing.T) {
	t.Parallel()
	testkit.AssertEqual(t, "labels is multi", isMultiSelectField("labels"), true)
	testkit.AssertEqual(t, "components is multi", isMultiSelectField("components"), true)
	testkit.AssertEqual(t, "status is not multi", isMultiSelectField(fieldStatus), false)
}

func TestRenderMultiSelectDiff_ShowsAddedAndRemoved(t *testing.T) {
	t.Parallel()
	lines := renderMultiSelectDiff("alpha,beta", "beta,gamma")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(stripANSI(joined), "alpha") {
		t.Errorf("diff = %q, want removed 'alpha'", joined)
	}
	if !strings.Contains(stripANSI(joined), "gamma") {
		t.Errorf("diff = %q, want added 'gamma'", joined)
	}
}

func TestRenderMultiSelectDiff_EmptyNoneValues(t *testing.T) {
	t.Parallel()
	lines := renderMultiSelectDiff(noneLabel, "item")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(stripANSI(joined), "item") {
		t.Errorf("diff from none = %q, want added 'item'", joined)
	}
}

func TestStatusNameStyle_Colors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		statusName    string
		expectedColor lipgloss.TerminalColor
	}{
		{"done", theme.ColorGreen},
		{"in progress", theme.ColorYellow},
		{"todo", theme.ColorCyan},
		{"xyzzy unknown status", theme.ColorWhite},
	}
	for _, tt := range tests {
		t.Run(tt.statusName, func(t *testing.T) {
			t.Parallel()
			testkit.AssertEqual(t, "foreground", statusNameStyle(tt.statusName).GetForeground(), tt.expectedColor)
		})
	}
}

func TestRenderDiff_ShowsRemovedAndAdded(t *testing.T) {
	t.Parallel()
	lines := renderDiff("line one\nline two", "line two\nline three", 80)
	joined := strings.Join(lines, "\n")
	plainJoined := stripANSI(joined)
	if !strings.Contains(plainJoined, "line one") {
		t.Errorf("diff = %q, want removed 'line one'", plainJoined)
	}
	if !strings.Contains(plainJoined, "line three") {
		t.Errorf("diff = %q, want added 'line three'", plainJoined)
	}
}

func TestRenderDiff_ContentChangedWhenSame(t *testing.T) {
	t.Parallel()
	lines := renderDiff("unchanged", "unchanged", 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "content changed") {
		t.Errorf("no-diff result = %q, want '(content changed)'", joined)
	}
}
