package tui

import (
	"strings"
	"unicode"

	"github.com/textfuel/lazyjira/pkg/jira"
)

// JQL context modes returned by parseJQLContext
const (
	jqlCtxField = "field"
	jqlCtxValue = "value"
	jqlCtxNone  = "none"
)

// JQLContext holds the result of JQL input analysis
type JQLContext struct {
	Mode       string // jqlCtxField, jqlCtxValue, jqlCtxNone
	FieldName  string // field name for value suggestions
	Partial    string // partial text for filtering (sent to API)
	PartialLen int    // number of runes from cursor to replace on insert
}

// parseJQLContext analyzes the JQL input at the cursor position to determine
// what kind of autocomplete suggestions to show.
func parseJQLContext(input string, cursorPos int) JQLContext {
	if input == "" {
		return JQLContext{Mode: jqlCtxNone}
	}

	runes := []rune(input)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}
	before := string(runes[:cursorPos])

	tokens := tokenizeJQL(before)
	if len(tokens) == 0 {
		return JQLContext{Mode: jqlCtxField}
	}

	operators := map[string]bool{
		"=": true, "!=": true, "~": true, "!~": true,
		">": true, ">=": true, "<": true, "<=": true,
		"is": true, "in": true, "not": true, "was": true,
	}

	keywords := map[string]bool{
		"and": true, "or": true, "order": true, "by": true,
		"asc": true, "desc": true, "not": true,
	}

	last := tokens[len(tokens)-1]
	lastLower := strings.ToLower(last)

	endsWithSpace := len(before) > 0 && unicode.IsSpace(runes[cursorPos-1])

	// If last token is "(" or "," (no trailing space), treat as boundary.
	if !endsWithSpace && (last == "(" || last == ",") {
		if field := findINListField(tokens, true); field != "" {
			return JQLContext{Mode: jqlCtxValue, FieldName: field}
		}
	}

	// Check if we're inside an IN(...) list.
	if field := findINListField(tokens, endsWithSpace); field != "" {
		partial, partialLen := rawPartialAfterDelimiter(runes, cursorPos)
		if endsWithSpace && partialLen == 0 {
			return JQLContext{Mode: jqlCtxValue, FieldName: field}
		}
		return JQLContext{Mode: jqlCtxValue, FieldName: field, Partial: partial, PartialLen: partialLen}
	}

	if endsWithSpace {
		if operators[lastLower] {
			field := findFieldBeforeOperator(tokens)
			return JQLContext{Mode: jqlCtxValue, FieldName: field}
		}
		if keywords[lastLower] {
			return JQLContext{Mode: jqlCtxField}
		}
		return JQLContext{Mode: jqlCtxNone}
	}

	// In the middle of typing a token.
	if len(tokens) == 1 {
		return JQLContext{Mode: jqlCtxField, Partial: last, PartialLen: len([]rune(last))}
	}

	// If the current token IS an operator, suggest values for the field before it.
	if operators[lastLower] && len(tokens) >= 2 {
		field := findFieldBeforeOperator(tokens)
		return JQLContext{Mode: jqlCtxValue, FieldName: field}
	}

	// Look backward for an operator — handles multi-word values after =, ~, etc.
	for j := len(tokens) - 2; j >= 0; j-- {
		tl := strings.ToLower(tokens[j])
		if operators[tl] {
			field := findFieldBeforeOperator(tokens[:j])
			partial, partialLen := rawPartialAfterDelimiter(runes, cursorPos)
			return JQLContext{Mode: jqlCtxValue, FieldName: field, Partial: partial, PartialLen: partialLen}
		}
		if keywords[tl] {
			break // stop at keywords like AND/OR
		}
	}

	// If the current token IS a keyword, suggest fields.
	if keywords[lastLower] {
		return JQLContext{Mode: jqlCtxField}
	}

	// After a keyword like AND/OR — suggest field.
	if len(tokens) >= 2 {
		prev := strings.ToLower(tokens[len(tokens)-2])
		if keywords[prev] {
			return JQLContext{Mode: jqlCtxField, Partial: last, PartialLen: len([]rune(last))}
		}
	}

	return JQLContext{Mode: jqlCtxField, Partial: last, PartialLen: len([]rune(last))}
}

// rawPartialAfterDelimiter returns the raw text between the last value delimiter
// and the cursor, along with its rune length. Delimiters are (, , and operator+space boundaries.
// This captures multi-word partials like "ready for de" correctly
func rawPartialAfterDelimiter(runes []rune, cursorPos int) (string, int) {
	// Scan backward from cursor to find the delimiter.
	i := cursorPos - 1
	for i >= 0 {
		r := runes[i]
		if r == '(' || r == ',' {
			break
		}
		// Stop at operator chars followed by space: "= value"
		// We detect this by checking if we hit a known operator boundary.
		// Simple heuristic: stop at = ! ~ > < if followed by space.
		if (r == '=' || r == '~' || r == '>' || r == '<') && i+1 < len(runes) && runes[i+1] == ' ' {
			break
		}
		if r == '!' && i+1 < len(runes) && (runes[i+1] == '=' || runes[i+1] == '~') {
			break
		}
		i--
	}
	// i is now at the delimiter or -1. The partial starts after the delimiter.
	start := i + 1
	// Trim leading spaces.
	for start < cursorPos && runes[start] == ' ' {
		start++
	}
	partial := string(runes[start:cursorPos])
	return partial, cursorPos - start
}

// findINListField checks if we're inside an IN(...) list and returns the field name.
func findINListField(tokens []string, endsWithSpace bool) string {
	startIdx := len(tokens) - 1
	if !endsWithSpace {
		startIdx--
	}

	for i := startIdx; i >= 0; i-- {
		tok := tokens[i]
		switch tok {
		case "(":
			if i > 0 && strings.EqualFold(tokens[i-1], "in") {
				return findFieldBeforeOperator(tokens[:i])
			}
			if i > 1 && strings.EqualFold(tokens[i-1], "not") && strings.EqualFold(tokens[i-2], "in") {
				return ""
			}
			return ""
		case ")":
			return ""
		default:
			continue
		}
	}
	return ""
}

// findFieldBeforeOperator looks backward through tokens to find the field name
// that precedes an operator.
func findFieldBeforeOperator(tokens []string) string {
	operators := map[string]bool{
		"=": true, "!=": true, "~": true, "!~": true,
		">": true, ">=": true, "<": true, "<=": true,
		"is": true, "in": true, "not": true, "was": true,
	}

	for i := len(tokens) - 1; i >= 0; i-- {
		lower := strings.ToLower(tokens[i])
		if !operators[lower] {
			return tokens[i]
		}
	}
	return ""
}

// matchFieldSuggestions filters and ranks field suggestions by relevance.
// Results are ordered by exact match first, then prefix match, then contains match
func matchFieldSuggestions(fields []jira.AutocompleteField, partial string) []string {
	if partial == "" {
		result := make([]string, len(fields))
		for i, f := range fields {
			result[i] = f.Value
		}
		return result
	}

	lower := strings.ToLower(partial)
	var exact, prefix, contains []string
	for _, f := range fields {
		fLower := strings.ToLower(f.Value)
		switch {
		case fLower == lower:
			exact = append(exact, f.Value)
		case strings.HasPrefix(fLower, lower):
			prefix = append(prefix, f.Value)
		case strings.Contains(fLower, lower):
			contains = append(contains, f.Value)
		}
	}

	result := make([]string, 0, len(exact)+len(prefix)+len(contains))
	result = append(result, exact...)
	result = append(result, prefix...)
	result = append(result, contains...)
	return result
}

// tokenizeJQL splits JQL text into tokens, respecting double-quoted strings.
// Parentheses and commas are emitted as separate tokens
func tokenizeJQL(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			current.WriteRune(r)
		case unicode.IsSpace(r) && !inQuote:
			flush()
		case (r == '(' || r == ')' || r == ',') && !inQuote:
			flush()
			tokens = append(tokens, string(r))
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}
