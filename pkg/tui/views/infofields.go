package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

const fieldStatus = "status"

// InfoFieldType determines which editor to use for a field
type InfoFieldType int

const (
	FieldSingleSelect InfoFieldType = iota
	FieldMultiSelect
	FieldPerson
	FieldSingleText
	FieldMultiText
)

// InfoField represents an editable field in the Info panel
type InfoField struct {
	Name    string
	FieldID string
	Type    InfoFieldType
	Value   string
}

func buildInfoFields(issue *jira.Issue, customFields []config.CustomFieldConfig) []InfoField {
	if issue == nil {
		return nil
	}
	var fields []InfoField

	statusName := unknownLabel
	if issue.Status != nil {
		statusName = issue.Status.Name
	}
	fields = append(fields, InfoField{Name: "Status", FieldID: fieldStatus, Type: FieldSingleSelect, Value: statusName})

	priorityName := noneLabelUpper
	if issue.Priority != nil {
		priorityName = issue.Priority.Name
	}
	fields = append(fields, InfoField{Name: "Priority", FieldID: "priority", Type: FieldSingleSelect, Value: priorityName})

	assignee := "None"
	if issue.Assignee != nil {
		assignee = issue.Assignee.DisplayName
	}
	fields = append(fields, InfoField{Name: "Assignee", FieldID: "assignee", Type: FieldPerson, Value: assignee})

	reporter := "Unknown"
	if issue.Reporter != nil {
		reporter = issue.Reporter.DisplayName
	}
	fields = append(fields, InfoField{Name: "Reporter", FieldID: "reporter", Type: FieldPerson, Value: reporter})

	typeName := unknownLabel
	if issue.IssueType != nil {
		typeName = issue.IssueType.Name
	}
	fields = append(fields, InfoField{Name: "Type", FieldID: "issuetype", Type: FieldSingleSelect, Value: typeName})

	sprintName := noneLabelUpper
	if issue.Sprint != nil {
		sprintName = issue.Sprint.Name
	}
	fields = append(fields, InfoField{Name: "Sprint", FieldID: "sprint", Type: FieldSingleSelect, Value: sprintName})

	if len(issue.Labels) > 0 {
		fields = append(fields, InfoField{Name: "Labels", FieldID: "labels", Type: FieldMultiSelect, Value: strings.Join(issue.Labels, ", ")})
	}

	if len(issue.Components) > 0 {
		names := make([]string, 0, len(issue.Components))
		for _, c := range issue.Components {
			names = append(names, c.Name)
		}
		fields = append(fields, InfoField{Name: "Components", FieldID: "components", Type: FieldMultiSelect, Value: strings.Join(names, ", ")})
	}

	for _, cf := range customFields {
		val := formatCustomFieldValue(issue.CustomFields[cf.ID])
		fields = append(fields, InfoField{Name: cf.Name, FieldID: cf.ID, Type: FieldSingleText, Value: val})
	}

	return fields
}

func infoFieldCount(issue *jira.Issue, customFields []config.CustomFieldConfig) int {
	if issue == nil {
		return 0
	}
	count := 6
	if len(issue.Labels) > 0 {
		count++
	}
	if len(issue.Components) > 0 {
		count++
	}
	count += len(customFields)
	return count
}

func renderInfoRows(issue *jira.Issue, customFields []config.CustomFieldConfig, th *theme.Theme, maxWidth int) []string {
	return renderInfoRowsImpl(issue, customFields, th, maxWidth)
}

func renderInfoRowsPlain(issue *jira.Issue, customFields []config.CustomFieldConfig, maxWidth int) []string {
	return renderInfoRowsImpl(issue, customFields, nil, maxWidth)
}

func renderInfoRowsImpl(issue *jira.Issue, customFields []config.CustomFieldConfig, th *theme.Theme, maxWidth int) []string {
	if issue == nil {
		return nil
	}
	styled := th != nil
	style := func(s string) string {
		if styled {
			return th.ValueStyle.Render(s)
		}
		return s
	}

	const labelWidth = 13
	fields := buildInfoFields(issue, customFields)
	rows := make([]string, 0, len(fields))
	for _, f := range fields {
		val := f.Value
		if maxVal := maxWidth - labelWidth; maxWidth > 0 && lipgloss.Width(val) > maxVal && maxVal > 1 {
			val = components.TruncateEnd(val, maxVal)
		}
		switch f.FieldID {
		case fieldStatus:
			if styled && issue.Status != nil {
				val = theme.StatusColor(issue.Status.CategoryKey).Render(val)
			}
		case "priority":
			if styled && issue.Priority != nil {
				val = theme.PriorityStyled(val)
			}
		case "assignee":
			if styled && issue.Assignee != nil {
				val = theme.AuthorRender(val)
			}
		case "reporter":
			if styled && issue.Reporter != nil {
				val = theme.AuthorRender(val)
			}
		default:
			val = style(val)
		}
		rows = append(rows, fmt.Sprintf(" %-11s %s", f.Name+":", val))
	}
	return rows
}

func formatCustomFieldValue(v any) string {
	if v == nil {
		return noneLabelUpper
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return fmt.Sprintf("%.2f", val)
	case map[string]any:
		if name, ok := val["displayName"].(string); ok {
			return name
		}
		if value, ok := val["value"].(string); ok {
			return value
		}
		if name, ok := val["name"].(string); ok {
			return name
		}
		return fmt.Sprintf("%v", val)
	case []any:
		var parts []string
		for _, item := range val {
			parts = append(parts, formatCustomFieldValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}

