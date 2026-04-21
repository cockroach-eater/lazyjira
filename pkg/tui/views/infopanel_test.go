package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
)

// navDownResolver is a minimal NavResolver that maps "j" to NavDown.
func navDownResolver(key string) components.NavAction {
	if key == "j" {
		return components.NavDown
	}
	return components.NavNone
}

func makeInfoPanelFocused() *InfoPanel {
	p := NewInfoPanel()
	p.ResolveNav = navDownResolver
	p.Focused = true
	return p
}

func pressJ() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
}

// TestInfoPanel_SubTab_CursorMove_DispatchesPreviewRequestMsg verifies that
// pressing a nav key while in the Sub tab emits PreviewRequestMsg with
// the sub-issue key now under the cursor.
func TestInfoPanel_SubTab_CursorMove_DispatchesPreviewRequestMsg(t *testing.T) {
	p := makeInfoPanelFocused()

	issue := &jira.Issue{
		Key: "MAIN-1",
		Subtasks: []jira.Issue{
			{Key: "SUB-1", Summary: "first subtask"},
			{Key: "SUB-2", Summary: "second subtask"},
		},
	}
	p.SetIssue(issue)

	for p.activeTab != InfoTabSubtasks {
		p.NextTab()
	}

	// Cursor starts at 0 (SUB-1). Press "j" to move to SUB-2.
	_, cmd := p.Update(pressJ())
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd after cursor move in Sub tab, got nil")
	}

	msg := cmd()
	prm, ok := msg.(PreviewRequestMsg)
	if !ok {
		t.Fatalf("expected PreviewRequestMsg, got %T", msg)
	}
	if prm.Key != "SUB-2" {
		t.Errorf("PreviewRequestMsg.Key = %q, want %q", prm.Key, "SUB-2")
	}
}

// TestInfoPanel_LnkTab_CursorMove_OutwardLink verifies that a cursor move in
// the Lnk tab emits PreviewRequestMsg with the outward link key.
func TestInfoPanel_LnkTab_CursorMove_OutwardLink(t *testing.T) {
	p := makeInfoPanelFocused()

	issue := &jira.Issue{
		Key: "MAIN-1",
		IssueLinks: []jira.IssueLink{
			{
				Type:         &jira.IssueLinkType{Name: "Blocks", Outward: "blocks", Inward: "is blocked by"},
				OutwardIssue: &jira.Issue{Key: "OUT-1", Summary: "outward issue"},
			},
			{
				Type:         &jira.IssueLinkType{Name: "Blocks", Outward: "blocks", Inward: "is blocked by"},
				OutwardIssue: &jira.Issue{Key: "OUT-2", Summary: "second outward"},
			},
		},
	}
	p.SetIssue(issue)

	for p.activeTab != InfoTabLinks {
		p.NextTab()
	}

	// Cursor starts at 0 (OUT-1). Press "j" to move to OUT-2.
	_, cmd := p.Update(pressJ())
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd after cursor move in Lnk tab, got nil")
	}

	msg := cmd()
	prm, ok := msg.(PreviewRequestMsg)
	if !ok {
		t.Fatalf("expected PreviewRequestMsg, got %T", msg)
	}
	if prm.Key != "OUT-2" {
		t.Errorf("PreviewRequestMsg.Key = %q, want %q", prm.Key, "OUT-2")
	}
}

// TestInfoPanel_LnkTab_CursorMove_InwardLink verifies that a cursor move to an
// inward link emits the inward issue's key.
func TestInfoPanel_LnkTab_CursorMove_InwardLink(t *testing.T) {
	p := makeInfoPanelFocused()

	issue := &jira.Issue{
		Key: "MAIN-1",
		IssueLinks: []jira.IssueLink{
			{
				Type:         &jira.IssueLinkType{Name: "Blocks", Outward: "blocks", Inward: "is blocked by"},
				OutwardIssue: &jira.Issue{Key: "OUT-1", Summary: "outward"},
				InwardIssue:  &jira.Issue{Key: "IN-1", Summary: "inward"},
			},
		},
	}
	p.SetIssue(issue)

	for p.activeTab != InfoTabLinks {
		p.NextTab()
	}

	// Two items: OUT-1 (index 0), IN-1 (index 1). Cursor at 0, press "j".
	_, cmd := p.Update(pressJ())
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd after cursor move in Lnk tab (inward), got nil")
	}

	msg := cmd()
	prm, ok := msg.(PreviewRequestMsg)
	if !ok {
		t.Fatalf("expected PreviewRequestMsg, got %T", msg)
	}
	if prm.Key != "IN-1" {
		t.Errorf("PreviewRequestMsg.Key = %q, want %q", prm.Key, "IN-1")
	}
}

// TestInfoPanel_FieldsTab_CursorMove_NoPreviewRequestMsg verifies that cursor
// movement in the Fields tab does NOT dispatch PreviewRequestMsg.
func TestInfoPanel_FieldsTab_CursorMove_NoPreviewRequestMsg(t *testing.T) {
	p := makeInfoPanelFocused()

	issue := &jira.Issue{
		Key:     "MAIN-1",
		Summary: "something",
	}
	p.SetIssue(issue)

	if p.activeTab != InfoTabFields {
		t.Fatal("expected InfoTabFields as default tab")
	}

	_, cmd := p.Update(pressJ())

	if cmd == nil {
		return // nil is acceptable: no preview dispatch
	}

	msg := cmd()
	if _, ok := msg.(PreviewRequestMsg); ok {
		t.Error("Fields tab must not dispatch PreviewRequestMsg on cursor move")
	}
}
