package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

func makeListADF(ordered bool, items []string) map[string]any {
	listType := adfBulletList
	if ordered {
		listType = adfOrderedList
	}
	listItems := make([]any, 0, len(items))
	for _, text := range items {
		listItems = append(listItems, map[string]any{
			"type": "listItem",
			"content": []any{
				map[string]any{
					"type": adfParagraph,
					"content": []any{
						map[string]any{"type": adfText, "text": text},
					},
				},
			},
		})
	}
	return map[string]any{
		"type":    "doc",
		"content": []any{map[string]any{"type": listType, "content": listItems}},
	}
}

func TestBuiltinRenderer_RenderListItem_BulletList(t *testing.T) {
	t.Parallel()
	adf := makeListADF(false, []string{"first item", "second item"})
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) == 0 {
		t.Fatal("expected non-empty output for bullet list")
	}
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "first item") {
		t.Errorf("bullet list output = %q, want 'first item'", joined)
	}
	if !strings.Contains(joined, "second item") {
		t.Errorf("bullet list output = %q, want 'second item'", joined)
	}
}

func TestBuiltinRenderer_RenderListItem_OrderedList(t *testing.T) {
	t.Parallel()
	adf := makeListADF(true, []string{"alpha", "beta", "gamma"})
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) == 0 {
		t.Fatal("expected non-empty output for ordered list")
	}
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "alpha") {
		t.Errorf("ordered list output = %q, want 'alpha'", joined)
	}
	if !strings.Contains(joined, "beta") {
		t.Errorf("ordered list output = %q, want 'beta'", joined)
	}
}

func TestBuiltinRenderer_RenderListItem_NestedList(t *testing.T) {
	t.Parallel()
	adf := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type": adfBulletList,
				"content": []any{
					map[string]any{
						"type": "listItem",
						"content": []any{
							map[string]any{
								"type": adfParagraph,
								"content": []any{
									map[string]any{"type": adfText, "text": "outer item"},
								},
							},
							map[string]any{
								"type": adfBulletList,
								"content": []any{
									map[string]any{
										"type": "listItem",
										"content": []any{
											map[string]any{
												"type": adfParagraph,
												"content": []any{
													map[string]any{"type": adfText, "text": "nested item"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) == 0 {
		t.Fatal("expected non-empty output for nested list")
	}
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "outer item") {
		t.Errorf("nested list output = %q, want 'outer item'", joined)
	}
	if !strings.Contains(joined, "nested item") {
		t.Errorf("nested list output = %q, want 'nested item'", joined)
	}
}

type taskItemSpec struct {
	text   string
	done   bool
	nested []taskItemSpec
}

func makeTaskListADF(items []taskItemSpec) map[string]any {
	return map[string]any{
		"type":    "doc",
		"content": []any{taskListNode(items)},
	}
}

func taskListNode(items []taskItemSpec) map[string]any {
	content := make([]any, 0, len(items))
	for _, it := range items {
		state := "TODO"
		if it.done {
			state = "DONE"
		}
		content = append(content, map[string]any{
			"type":    adfTaskItem,
			"attrs":   map[string]any{"state": state},
			"content": []any{map[string]any{"type": adfText, "text": it.text}},
		})
		if len(it.nested) > 0 {
			content = append(content, taskListNode(it.nested))
		}
	}
	return map[string]any{"type": adfTaskList, "content": content}
}

func TestBuiltinRenderer_RenderTaskList(t *testing.T) {
	t.Parallel()
	adf := makeTaskListADF([]taskItemSpec{
		{text: "todo item", done: false},
		{text: "done item", done: true},
	})
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) == 0 {
		t.Fatal("expected non-empty output for task list")
	}
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "[ ] todo item") {
		t.Errorf("task list output = %q, want '[ ] todo item'", joined)
	}
	if !strings.Contains(joined, "[x] done item") {
		t.Errorf("task list output = %q, want '[x] done item'", joined)
	}
}

func TestBuiltinRenderer_RenderTaskList_Nested(t *testing.T) {
	t.Parallel()
	adf := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type": adfTaskList,
				"content": []any{
					map[string]any{
						"type":    adfTaskItem,
						"attrs":   map[string]any{"state": "TODO"},
						"content": []any{map[string]any{"type": adfText, "text": "outer task"}},
					},
					map[string]any{
						"type": adfTaskList,
						"content": []any{
							map[string]any{
								"type":    adfTaskItem,
								"attrs":   map[string]any{"state": "DONE"},
								"content": []any{map[string]any{"type": adfText, "text": "nested task"}},
							},
						},
					},
				},
			},
		},
	}
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) == 0 {
		t.Fatal("expected non-empty output for nested task list")
	}
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "outer task") {
		t.Errorf("nested task list output = %q, want 'outer task'", joined)
	}
	if !strings.Contains(joined, "nested task") {
		t.Errorf("nested task list output = %q, want 'nested task'", joined)
	}
}

func makeTableADF(rows [][]string) map[string]any {
	tableRows := make([]any, 0, len(rows))
	for i, row := range rows {
		var cells []any
		for _, cellText := range row {
			cellType := "tableCell"
			if i == 0 {
				cellType = "tableHeader"
			}
			cells = append(cells, map[string]any{
				"type": cellType,
				"content": []any{
					map[string]any{
						"type": adfParagraph,
						"content": []any{
							map[string]any{"type": adfText, "text": cellText},
						},
					},
				},
			})
		}
		tableRows = append(tableRows, map[string]any{
			"type":    "tableRow",
			"content": cells,
		})
	}
	return map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type":    adfTable,
				"content": tableRows,
			},
		},
	}
}

func TestBuiltinRenderer_RenderTable_HeaderAndRows(t *testing.T) {
	t.Parallel()
	adf := makeTableADF([][]string{
		{"Name", "Value"},
		{"alpha", "one"},
		{"beta", "two"},
	})
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) == 0 {
		t.Fatal("expected non-empty output for table")
	}
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Name") {
		t.Errorf("table output = %q, want header 'Name'", joined)
	}
	if !strings.Contains(joined, "alpha") {
		t.Errorf("table output = %q, want cell 'alpha'", joined)
	}
}

func TestBuiltinRenderer_RenderTable_EmptyTableSkipped(t *testing.T) {
	t.Parallel()
	adf := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type":    adfTable,
				"content": []any{},
			},
		},
	}
	lines := BuiltinRenderer{}.Render(adf, 80)
	if len(lines) != 0 {
		t.Errorf("empty table should produce no lines, got %v", lines)
	}
}

func TestURLStyle_CyanUnderlined(t *testing.T) {
	t.Parallel()
	style := urlStyle()
	var wantCyan lipgloss.TerminalColor = theme.ColorCyan
	testkit.AssertEqual(t, "foreground", style.GetForeground(), wantCyan)
	testkit.AssertEqual(t, "underline", style.GetUnderline(), true)
}
