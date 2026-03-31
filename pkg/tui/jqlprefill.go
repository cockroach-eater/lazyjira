package tui

import (
	"regexp"
	"strings"

	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui/components"
)

const currentUserMarker = "__currentUser__"

var (
	jqlAndRe = regexp.MustCompile(`(?i)\s+AND\s+`)
	jqlEqRe  = regexp.MustCompile(`^\s*(\w+)\s*=\s*(.+?)\s*$`)
)

// ParseJQLPrefill extracts simple equality clauses from JQL
// Skips OR, NOT, IN and complex functions except currentUser()
func ParseJQLPrefill(jql string) map[string]string {
	result := make(map[string]string)
	if jql == "" {
		return result
	}

	upper := strings.ToUpper(jql)
	if strings.Contains(upper, " OR ") || strings.Contains(upper, " NOT ") {
		return result
	}

	parts := jqlAndRe.Split(jql, -1)

	for _, part := range parts {
		// remove ORDER BY and everything after
		if idx := strings.Index(strings.ToUpper(part), "ORDER BY"); idx >= 0 {
			part = part[:idx]
		}
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		m := jqlEqRe.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		field := strings.ToLower(m[1])
		value := strings.Trim(m[2], `"'`)

		// handle currentUser()
		if strings.EqualFold(value, "currentUser()") {
			result[field] = currentUserMarker
			continue
		}

		// skip other functions
		if strings.Contains(value, "(") {
			continue
		}

		result[field] = value
	}
	return result
}

// ApplyPrefill sets form field values from parsed JQL prefill
func ApplyPrefill(fields []components.CreateFormField, prefill map[string]string, currentUser *jira.User, isCloud bool) {
	for i := range fields {
		fid := strings.ToLower(fields[i].FieldID)
		val, ok := prefill[fid]
		if !ok {
			continue
		}

		if val == currentUserMarker && currentUser != nil {
			fields[i].DisplayValue = currentUser.DisplayName
			key := fldName
			if isCloud {
				key = fldAccountID
			}
			fields[i].Value = map[string]string{key: currentUser.AccountID}
			continue
		}

		// for select fields try to match by name in allowed values
		if len(fields[i].AllowedValues) > 0 {
			for _, av := range fields[i].AllowedValues {
				if strings.EqualFold(av.Label, val) {
					fields[i].DisplayValue = av.Label
					fields[i].Value = map[string]string{"id": av.ID}
					break
				}
			}
			continue
		}

		// text fields
		fields[i].DisplayValue = val
		fields[i].Value = val
	}
}
