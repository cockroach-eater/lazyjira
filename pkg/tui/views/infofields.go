package views

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/jira"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
	"github.com/textfuel/lazyjira/v2/pkg/tui/theme"
)

const (
	fieldStatus   = "status"
	fieldAssignee = "assignee"
)

type builtinFieldDef struct {
	name    string
	fieldID string
	typ     InfoFieldType
	// getValue renders the field for display in the info pane.
	getValue func(issue *jira.Issue) (string, bool)
	// setValue applies the post-save value to the cached issue.
	setValue func(issue *jira.Issue, value any)
	// editValue returns the canonical input-modal prefill when it must differ
	// from the display form (e.g. parent shows "KEY -- summary" but only the
	// key is editable). nil means the display value is also the edit value.
	editValue func(issue *jira.Issue) string
}

var builtinFieldRegistry = []builtinFieldDef{
	{
		name: "Status", fieldID: "status", typ: FieldSingleSelect,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.Status != nil {
				return i.Status.Name, true
			}
			return unknownLabel, true
		},
	},
	{
		name: "Priority", fieldID: "priority", typ: FieldSingleSelect,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.Priority != nil {
				return i.Priority.Name, true
			}
			return noneLabelUpper, true
		},
		setValue: func(i *jira.Issue, v any) {
			if v == nil {
				i.Priority = nil
			} else if p, ok := v.(*jira.Priority); ok {
				i.Priority = p
			}
		},
	},
	{
		name: "Assignee", fieldID: fieldAssignee, typ: FieldPerson,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.Assignee != nil {
				return i.Assignee.DisplayName, true
			}
			return noneLabelUpper, true
		},
		setValue: func(i *jira.Issue, v any) {
			if v == nil {
				i.Assignee = nil
			} else if u, ok := v.(*jira.User); ok {
				i.Assignee = u
			}
		},
	},
	{
		name: "Reporter", fieldID: "reporter", typ: FieldPerson,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.Reporter != nil {
				return i.Reporter.DisplayName, true
			}
			return unknownLabel, true
		},
		setValue: func(i *jira.Issue, v any) {
			if v == nil {
				i.Reporter = nil
			} else if u, ok := v.(*jira.User); ok {
				i.Reporter = u
			}
		},
	},
	{
		name: "Type", fieldID: "issuetype", typ: FieldSingleSelect,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.IssueType != nil {
				return i.IssueType.Name, true
			}
			return unknownLabel, true
		},
	},
	{
		name: "Parent", fieldID: "parent", typ: FieldSingleText,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.Parent != nil {
				if i.Parent.Summary == "" {
					return i.Parent.Key, true
				}
				return "[" + i.Parent.Key + "] " + i.Parent.Summary, true
			}
			if i.IssueType.CanHaveParent() {
				return noneLabelUpper, true
			}
			return "", false
		},
		setValue: func(i *jira.Issue, v any) {
			if v == nil {
				i.Parent = nil
			} else if p, ok := v.(*jira.Issue); ok {
				i.Parent = p
			}
		},
		editValue: func(i *jira.Issue) string {
			if i.Parent == nil {
				return ""
			}
			return i.Parent.Key
		},
	},
	{
		// Jira duration strings ("2w 3d 4h"): the length of a day is
		// per-instance config, so they are shown and edited verbatim.
		name: "Estimate", fieldID: "timetracking", typ: FieldSingleText,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.TimeTracking.IsZero() {
				return noneLabelUpper, true
			}
			value := i.TimeTracking.OriginalEstimate
			if value == "" {
				value = noneLabelUpper
			}
			if spent := i.TimeTracking.TimeSpent; spent != "" {
				value += " (spent " + spent + ")"
			}
			return value, true
		},
		setValue: func(i *jira.Issue, v any) {
			estimate, _ := v.(string)
			if estimate == "" {
				i.TimeTracking = nil
				return
			}
			if i.TimeTracking == nil {
				i.TimeTracking = &jira.TimeTracking{}
			}
			i.TimeTracking.OriginalEstimate = estimate
		},
		editValue: func(i *jira.Issue) string {
			if i.TimeTracking == nil {
				return ""
			}
			return i.TimeTracking.OriginalEstimate
		},
	},
	{
		name: "Sprint", fieldID: "sprint", typ: FieldSingleSelect,
		getValue: func(i *jira.Issue) (string, bool) {
			if i.Sprint != nil {
				return i.Sprint.Name, true
			}
			return noneLabelUpper, true
		},
		setValue: func(i *jira.Issue, v any) {
			if v == nil {
				i.Sprint = nil
			} else if s, ok := v.(*jira.Sprint); ok {
				i.Sprint = s
			}
		},
	},
	{
		name: "Labels", fieldID: "labels", typ: FieldMultiSelect,
		getValue: func(i *jira.Issue) (string, bool) {
			if len(i.Labels) > 0 {
				return strings.Join(i.Labels, ", "), true
			}
			return "", false
		},
		setValue: func(i *jira.Issue, v any) {
			if labels, ok := v.([]string); ok {
				i.Labels = labels
			}
		},
	},
	{
		name: "Components", fieldID: "components", typ: FieldMultiSelect,
		getValue: func(i *jira.Issue) (string, bool) {
			if len(i.Components) > 0 {
				names := make([]string, 0, len(i.Components))
				for _, c := range i.Components {
					names = append(names, c.Name)
				}
				return strings.Join(names, ", "), true
			}
			return "", false
		},
		setValue: func(i *jira.Issue, v any) {
			if comps, ok := v.([]map[string]string); ok {
				result := make([]jira.Component, 0, len(comps))
				for _, c := range comps {
					result = append(result, jira.Component{ID: c["id"]})
				}
				i.Components = result
			}
		},
	},
}

func SetBuiltinFieldValue(issue *jira.Issue, fieldID string, value any) bool {
	if def, ok := builtinFieldMap[fieldID]; ok && def.setValue != nil {
		def.setValue(issue, value)
		return true
	}
	return false
}

func PatchIssueFields(target, source *jira.Issue) {
	target.Summary = source.Summary
	target.Description = source.Description
	target.Status = source.Status
	target.Priority = source.Priority
	target.Assignee = source.Assignee
	target.Reporter = source.Reporter
	target.IssueType = source.IssueType
	target.Sprint = source.Sprint
	target.Labels = source.Labels
	target.Components = source.Components
	target.Updated = source.Updated
	target.CustomFields = source.CustomFields
}

var builtinFieldMap = func() map[string]builtinFieldDef {
	m := make(map[string]builtinFieldDef, len(builtinFieldRegistry))
	for _, def := range builtinFieldRegistry {
		m[def.fieldID] = def
	}
	return m
}()

var defaultFieldIDs = []string{fieldStatus, "priority", fieldAssignee, "reporter", "issuetype", "parent", "sprint", "timetracking"}

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

// extraInfoFields are the Info rows that are not Jira issue fields: the
// per-instance reviewer custom field and the local git branch.
type extraInfoFields struct {
	reviewerID string
	branch     string
	showBranch bool
}

func buildInfoFields(issue *jira.Issue, cfgFields []config.FieldConfig, extra extraInfoFields) []InfoField {
	if issue == nil {
		return nil
	}

	if cfgFields == nil {
		return extra.apply(buildDefaultInfoFields(issue), issue)
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
	return extra.apply(fields, issue)
}

// BranchFieldID marks the pseudo-field showing the git branch of an issue. It
// is not a Jira field: editing it runs the branch-creation flow.
const BranchFieldID = "_branch"

func (e extraInfoFields) apply(fields []InfoField, issue *jira.Issue) []InfoField {
	fields = e.withReviewer(fields, issue)
	if e.showBranch {
		value := e.branch
		if value == "" {
			value = noneLabelUpper
		}
		fields = append(fields, InfoField{
			Name: "Branch", FieldID: BranchFieldID, Type: FieldSingleText, Value: value,
		})
	}
	return fields
}

// withReviewer inserts the configured reviewer custom field right after the
// assignee, where a reader expects to find it. It is a no-op when no reviewer
// field is configured or the user already listed it in their own fields.
func (e extraInfoFields) withReviewer(fields []InfoField, issue *jira.Issue) []InfoField {
	if e.reviewerID == "" {
		return fields
	}
	for _, f := range fields {
		if f.FieldID == e.reviewerID {
			return fields
		}
	}

	reviewer := InfoField{
		Name:    "Reviewer",
		FieldID: e.reviewerID,
		Type:    FieldPerson,
		Value:   formatCustomFieldValue(issue.CustomFields[e.reviewerID]),
	}
	if reviewer.Value == "" {
		reviewer.Value = noneLabelUpper
	}

	for i, f := range fields {
		if f.FieldID == fieldAssignee {
			return slices.Insert(fields, i+1, reviewer)
		}
	}
	return append(fields, reviewer)
}

func buildDefaultInfoFields(issue *jira.Issue) []InfoField {
	var fields []InfoField
	for _, id := range defaultFieldIDs {
		def := builtinFieldMap[id]
		val, show := def.getValue(issue)
		if !show {
			continue
		}
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

func infoFieldCount(issue *jira.Issue, cfgFields []config.FieldConfig, extra extraInfoFields) int {
	return len(buildInfoFields(issue, cfgFields, extra))
}

func renderInfoRowPairs(issue *jira.Issue, cfgFields []config.FieldConfig, extra extraInfoFields, th *theme.Theme, maxWidth int) (styled, plain []string) {
	fields := buildInfoFields(issue, cfgFields, extra)
	styled = renderFieldRows(fields, issue, th, maxWidth)
	plain = renderFieldRows(fields, issue, nil, maxWidth)
	return
}

func noneStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.ColorGray)
}

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
			val = noneStyle().Render(val)
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

// EditValueForField returns the prefill text for an input-modal edit. Built-in
// fields can provide an editValue callback when the display form is not what
// the user should type back in. Custom fields fall through to the generic
// display-string filter.
func EditValueForField(issue *jira.Issue, fieldID, displayValue string) string {
	if def, ok := builtinFieldMap[fieldID]; ok && def.editValue != nil {
		return def.editValue(issue)
	}
	return EditValueForInput(displayValue)
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
