package components

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// JQL syntax highlighting colors (matching Jira Cloud JQL editor).
var (
	jqlFieldStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	jqlKeywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // magenta/purple
	jqlOperatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	jqlStringStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow for quoted strings
	jqlDefaultStyle  = lipgloss.NewStyle()
)

// JQL operators (word-based).
var jqlWordOperators = map[string]bool{
	"in": true, "not": true, "is": true, "was": true,
}

// JQL keywords.
var jqlKeywords = map[string]bool{
	"and": true, "or": true, "order": true, "by": true,
	"asc": true, "desc": true, "empty": true, "null": true,
}

// JQL symbol operators.
var jqlSymbolOperators = map[string]bool{
	"=": true, "!=": true, "~": true, "!~": true,
	">": true, ">=": true, "<": true, "<=": true,
}

// JQL known field names (common ones for highlighting without API data).
var jqlKnownFields = map[string]bool{
	"project": true, "status": true, "assignee": true, "reporter": true,
	"priority": true, "issuetype": true, "summary": true, "description": true,
	"created": true, "updated": true, "resolved": true, "due": true,
	"labels": true, "component": true, "fixversion": true, "affectedversion": true,
	"sprint": true, "epic": true, "parent": true, "type": true,
	"resolution": true, "statusCategory": true, "text": true, "key": true,
	"id": true, "issuekey": true, "filter": true, "watcher": true,
	"voter": true, "comment": true, "level": true, "originalestimate": true,
	"remainingestimate": true, "timespent": true, "workratio": true,
	"lastviewed": true, "createddate": true, "updateddate": true,
	"statuscategorychangeddate": true,
}

// HighlightJQL returns styled segments for JQL syntax highlighting.
func HighlightJQL(runes []rune) []StyledSegment {
	text := string(runes)
	var segments []StyledSegment
	i := 0
	n := len(runes)

	for i < n {
		r := runes[i]

		// Quoted string.
		if r == '"' {
			j := i + 1
			for j < n && runes[j] != '"' {
				j++
			}
			if j < n {
				j++ // include closing quote
			}
			segments = append(segments, StyledSegment{
				Text:  string(runes[i:j]),
				Style: jqlStringStyle,
			})
			i = j
			continue
		}

		// Whitespace — pass through unstyled.
		if unicode.IsSpace(r) {
			j := i + 1
			for j < n && unicode.IsSpace(runes[j]) {
				j++
			}
			segments = append(segments, StyledSegment{
				Text:  string(runes[i:j]),
				Style: jqlDefaultStyle,
			})
			i = j
			continue
		}

		// Parentheses and commas — unstyled.
		if r == '(' || r == ')' || r == ',' {
			segments = append(segments, StyledSegment{
				Text:  string(r),
				Style: jqlDefaultStyle,
			})
			i++
			continue
		}

		// Symbol operators: =, !=, ~, !~, >, >=, <, <=
		if r == '=' || r == '>' || r == '<' || r == '~' {
			segments = append(segments, StyledSegment{
				Text:  string(r),
				Style: jqlOperatorStyle,
			})
			i++
			continue
		}
		if r == '!' && i+1 < n && (runes[i+1] == '=' || runes[i+1] == '~') {
			segments = append(segments, StyledSegment{
				Text:  string(runes[i : i+2]),
				Style: jqlOperatorStyle,
			})
			i += 2
			continue
		}

		// Word token.
		j := i
		for j < n && !unicode.IsSpace(runes[j]) && runes[j] != '(' && runes[j] != ')' &&
			runes[j] != ',' && runes[j] != '"' && runes[j] != '=' && runes[j] != '!' &&
			runes[j] != '~' && runes[j] != '>' && runes[j] != '<' {
			j++
		}
		if j == i {
			// Single unrecognized char.
			segments = append(segments, StyledSegment{
				Text:  string(r),
				Style: jqlDefaultStyle,
			})
			i++
			continue
		}

		word := string(runes[i:j])
		wordLower := strings.ToLower(word)

		style := classifyJQLWord(wordLower, text, i)
		segments = append(segments, StyledSegment{
			Text:  word,
			Style: style,
		})
		i = j
	}

	return segments
}

// classifyJQLWord determines the style for a word token based on context.
func classifyJQLWord(wordLower, fullText string, pos int) lipgloss.Style {
	if jqlKeywords[wordLower] {
		return jqlKeywordStyle
	}
	if jqlWordOperators[wordLower] {
		return jqlOperatorStyle
	}

	// Check if this looks like a field name:
	// 1. Known field names
	// 2. Starts with "customfield_"
	// 3. Appears before an operator (heuristic: followed by space+operator or space+keyword)
	if jqlKnownFields[wordLower] || strings.HasPrefix(wordLower, "customfield_") {
		return jqlFieldStyle
	}

	// Heuristic: if preceded by an operator or at start, likely a value (default).
	// If followed by an operator, likely a field (blue).
	after := strings.TrimLeft(fullText[pos+len(wordLower):], " ")
	afterLower := strings.ToLower(after)
	for op := range jqlSymbolOperators {
		if strings.HasPrefix(after, op) {
			return jqlFieldStyle
		}
	}
	for op := range jqlWordOperators {
		if strings.HasPrefix(afterLower, op+" ") || strings.HasPrefix(afterLower, op+"(") || afterLower == op {
			return jqlFieldStyle
		}
	}

	// Functions like currentUser(), now().
	if strings.HasSuffix(wordLower, "()") {
		return jqlDefaultStyle
	}

	return jqlDefaultStyle
}
