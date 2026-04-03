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

type builtinFieldDef struct {
	name     string
	fieldID  string
	typ      InfoFieldType
	getValue func(issue *jira.Issue) (string, bool)
}

var builtinFieldRegistry = []builtinFieldDef{
	{"Status", "status", FieldSingleSelect, func(i *jira.Issue) (string, bool) {
		if i.Status != nil {
			return i.Status.Name, true
		}
		return unknownLabel, true
	}},
	{"Priority", "priority", FieldSingleSelect, func(i *jira.Issue) (string, bool) {
		if i.Priority != nil {
			return i.Priority.Name, true
		}
		return noneLabelUpper, true
	}},
	{"Assignee", "assignee", FieldPerson, func(i *jira.Issue) (string, bool) {
		if i.Assignee != nil {
			return i.Assignee.DisplayName, true
		}
		return noneLabelUpper, true
	}},
	{"Reporter", "reporter", FieldPerson, func(i *jira.Issue) (string, bool) {
		if i.Reporter != nil {
			return i.Reporter.DisplayName, true
		}
		return unknownLabel, true
	}},
	{"Type", "issuetype", FieldSingleSelect, func(i *jira.Issue) (string, bool) {
		if i.IssueType != nil {
			return i.IssueType.Name, true
		}
		return unknownLabel, true
	}},
	{"Sprint", "sprint", FieldSingleSelect, func(i *jira.Issue) (string, bool) {
		if i.Sprint != nil {
			return i.Sprint.Name, true
		}
		return noneLabelUpper, true
	}},
	{"Labels", "labels", FieldMultiSelect, func(i *jira.Issue) (string, bool) {
		if len(i.Labels) > 0 {
			return strings.Join(i.Labels, ", "), true
		}
		return "", false
	}},
	{"Components", "components", FieldMultiSelect, func(i *jira.Issue) (string, bool) {
		if len(i.Components) > 0 {
			names := make([]string, 0, len(i.Components))
			for _, c := range i.Components {
				names = append(names, c.Name)
			}
			return strings.Join(names, ", "), true
		}
		return "", false
	}},
}

var builtinFieldMap = func() map[string]builtinFieldDef {
	m := make(map[string]builtinFieldDef, len(builtinFieldRegistry))
	for _, def := range builtinFieldRegistry {
		m[def.fieldID] = def
	}
	return m
}()

var defaultFieldIDs = []string{"status", "priority", "assignee", "reporter", "issuetype", "sprint"}

type InfoFieldType int

const (
	FieldSingleSelect InfoFieldType = iota
	FieldMultiSelect
	FieldPerson
	FieldSingleText
	FieldMultiText
)

type InfoField struct {
	Name    string
	FieldID string
	Type    InfoFieldType
	Value   string
}

func buildInfoFields(issue *jira.Issue, cfgFields []config.FieldConfig) []InfoField {
	if issue == nil {
		return nil
	}

	if cfgFields == nil {
		return buildDefaultInfoFields(issue)
	}

	var fields []InfoField
	for _, cf := range cfgFields {
		if def, ok := builtinFieldMap[cf.ID]; ok {
			name := def.name
			if cf.Name != "" {
				name = cf.Name
			}
			val, show := def.getValue(issue)
			if !show {
				continue
			}
			fields = append(fields, InfoField{Name: name, FieldID: def.fieldID, Type: def.typ, Value: val})
		} else {
			raw := issue.CustomFields[cf.ID]
			val := formatCustomFieldValue(raw)
			ft := resolveCustomFieldType(cf.Type, raw)
			name := cf.Name
			if name == "" {
				name = cf.ID
			}
			fields = append(fields, InfoField{Name: name, FieldID: cf.ID, Type: ft, Value: val})
		}
	}
	return fields
}

func buildDefaultInfoFields(issue *jira.Issue) []InfoField {
	var fields []InfoField
	for _, id := range defaultFieldIDs {
		def := builtinFieldMap[id]
		val, _ := def.getValue(issue)
		fields = append(fields, InfoField{Name: def.name, FieldID: def.fieldID, Type: def.typ, Value: val})
	}
	for _, def := range builtinFieldRegistry {
		if def.fieldID != "labels" && def.fieldID != "components" {
			continue
		}
		if val, show := def.getValue(issue); show {
			fields = append(fields, InfoField{Name: def.name, FieldID: def.fieldID, Type: def.typ, Value: val})
		}
	}
	return fields
}

func infoFieldCount(issue *jira.Issue, cfgFields []config.FieldConfig) int {
	return len(buildInfoFields(issue, cfgFields))
}

func renderInfoRowPairs(issue *jira.Issue, cfgFields []config.FieldConfig, th *theme.Theme, maxWidth int) (styled, plain []string) {
	fields := buildInfoFields(issue, cfgFields)
	styled = renderFieldRows(fields, issue, th, maxWidth)
	plain = renderFieldRows(fields, issue, nil, maxWidth)
	return
}

var noneStyle = lipgloss.NewStyle().Foreground(theme.ColorGray)

func renderFieldRows(fields []InfoField, issue *jira.Issue, th *theme.Theme, maxWidth int) []string {
	if len(fields) == 0 {
		return nil
	}
	styled := th != nil

	labelW := 0
	for _, f := range fields {
		if w := lipgloss.Width(f.Name) + 1; w > labelW {
			labelW = w
		}
	}
	labelW += 2

	if maxLabelW := maxWidth / 2; labelW > maxLabelW && maxLabelW > 0 {
		labelW = maxLabelW
	}

	rows := make([]string, 0, len(fields))
	for _, f := range fields {
		val := f.Value
		maxVal := maxWidth - labelW - 1
		if maxVal > 0 && lipgloss.Width(val) > maxVal {
			val = components.TruncateEnd(val, maxVal)
		}

		isNone := val == noneLabelUpper || val == unknownLabel

		if styled && isNone {
			val = noneStyle.Render(val)
		} else if styled {
			switch f.FieldID {
			case fieldStatus:
				if issue.Status != nil {
					val = theme.StatusColor(issue.Status.CategoryKey).Render(val)
				}
			case "priority":
				if issue.Priority != nil {
					val = theme.PriorityStyled(val)
				}
			case "assignee":
				if issue.Assignee != nil {
					val = theme.AuthorRender(val)
				}
			case "reporter":
				if issue.Reporter != nil {
					val = theme.AuthorRender(val)
				}
			default:
				val = th.ValueStyle.Render(val)
			}
		}

		label := " " + f.Name + ":"
		if pad := labelW - lipgloss.Width(label); pad > 0 {
			label += strings.Repeat(" ", pad)
		}
		rows = append(rows, label+val)
	}
	return rows
}

func resolveCustomFieldType(configType string, raw any) InfoFieldType {
	switch configType {
	case "select":
		return FieldSingleSelect
	case "multiselect":
		return FieldMultiSelect
	case "user":
		return FieldPerson
	case "textarea":
		return FieldMultiText
	case "text":
		return FieldSingleText
	}
	return detectFieldTypeFromValue(raw)
}

func detectFieldTypeFromValue(v any) InfoFieldType {
	if v == nil {
		return FieldSingleText
	}
	switch val := v.(type) {
	case map[string]any:
		if _, ok := val["displayName"]; ok {
			return FieldPerson
		}
		if _, ok := val["value"]; ok {
			return FieldSingleSelect
		}
		if _, ok := val["name"]; ok {
			return FieldSingleSelect
		}
	case []any:
		return FieldMultiSelect
	}
	return FieldSingleText
}

func EditValueForInput(val string) string {
	if val == noneLabelUpper || val == unknownLabel {
		return ""
	}
	return val
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

