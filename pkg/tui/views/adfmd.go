package views

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ADF node type constants shared with adf.go.
const (
	adfParagraph   = "paragraph"
	adfHeading     = "heading"
	adfBulletList  = "bulletList"
	adfOrderedList = "orderedList"
	adfCodeBlock   = "codeBlock"
	adfBlockquote  = "blockquote"
	adfRule        = "rule"
	adfTable       = "table"
	adfText        = "text"
	adfMention     = "mention"
	adfEmoji       = "emoji"
	adfHardBreak   = "hardBreak"
	adfInlineCard  = "inlineCard"
	adfListItem    = "listItem"
)

// ADFToMarkdown converts an ADF document (map[string]any) to Markdown text.
// Exported for use by the edit flow in app.go.
func ADFToMarkdown(node any) string {
	return adfToMarkdown(node)
}

// MarkdownToADF converts Markdown text to an ADF document (map[string]any).
// Exported for use by the edit flow in app.go.
func MarkdownToADF(md string) any {
	return markdownToADF(md)
}

// adfToMarkdown converts an ADF document (map[string]any) to Markdown text.
// Unsupported node types are preserved as <!-- adf:TYPE {json} --> markers.
func adfToMarkdown(node any) string {
	doc, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	content, ok := doc["content"].([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, child := range content {
		if s := blockToMarkdown(child, 0); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

// blockToMarkdown converts a single block-level ADF node to Markdown.
//
//nolint:gocognit // ADF block dispatcher complexity is inherent to the format
func blockToMarkdown(node any, indent int) string {
	block, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	nodeType, _ := block["type"].(string)
	content, _ := block["content"].([]any)
	prefix := strings.Repeat(" ", indent)

	switch nodeType {
	case "paragraph":
		text := inlineToMarkdown(content)
		return prefix + text

	case "heading":
		level := 1
		if attrs, ok := block["attrs"].(map[string]any); ok {
			if l, ok := attrs["level"].(float64); ok {
				level = int(l)
			}
		}
		text := inlineToMarkdown(content)
		return prefix + strings.Repeat("#", level) + " " + text

	case "bulletList":
		var items []string
		for _, item := range content {
			items = append(items, listItemToMarkdown(item, indent, "- "))
		}
		return strings.Join(items, "\n")

	case "orderedList":
		var items []string
		for i, item := range content {
			marker := fmt.Sprintf("%d. ", i+1)
			items = append(items, listItemToMarkdown(item, indent, marker))
		}
		return strings.Join(items, "\n")

	case "codeBlock":
		lang := ""
		if attrs, ok := block["attrs"].(map[string]any); ok {
			lang, _ = attrs["language"].(string)
		}
		text := collectPlainText(content)
		return prefix + "```" + lang + "\n" + text + "\n" + prefix + "```"

	case "blockquote":
		var lines []string
		for _, child := range content {
			md := blockToMarkdown(child, 0)
			for _, line := range strings.Split(md, "\n") {
				lines = append(lines, prefix+"> "+line)
			}
		}
		return strings.Join(lines, "\n")

	case "rule":
		return prefix + "---"

	case "table":
		return tableToMarkdown(content, indent)

	default:
		// Unsupported node type — preserve as opaque marker.
		return opaqueMarker(block)
	}
}

// listItemToMarkdown converts a listItem node to Markdown.
func listItemToMarkdown(node any, indent int, marker string) string {
	item, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	content, _ := item["content"].([]any)
	prefix := strings.Repeat(" ", indent)
	contIndent := indent + len(marker)

	var parts []string
	first := true
	for _, child := range content {
		childBlock, ok := child.(map[string]any)
		if !ok {
			continue
		}
		childType, _ := childBlock["type"].(string)

		switch childType {
		case "paragraph":
			childContent, _ := childBlock["content"].([]any)
			text := inlineToMarkdown(childContent)
			if first {
				parts = append(parts, prefix+marker+text)
				first = false
			} else {
				parts = append(parts, strings.Repeat(" ", contIndent)+text)
			}
		case "bulletList", "orderedList":
			parts = append(parts, blockToMarkdown(child, contIndent))
		default:
			parts = append(parts, blockToMarkdown(child, contIndent))
		}
	}
	return strings.Join(parts, "\n")
}

// inlineToMarkdown converts inline ADF content to Markdown text.
func inlineToMarkdown(content []any) string {
	var parts []string
	for _, child := range content {
		inline, ok := child.(map[string]any)
		if !ok {
			continue
		}
		nodeType, _ := inline["type"].(string)

		switch nodeType {
		case "text":
			text, _ := inline["text"].(string)
			marks, _ := inline["marks"].([]any)
			parts = append(parts, applyMarksMD(text, marks))

		case "mention":
			if attrs, ok := inline["attrs"].(map[string]any); ok {
				displayName, _ := attrs["text"].(string)
				accountID, _ := attrs["id"].(string)
				parts = append(parts, fmt.Sprintf("[@%s](accountid:%s)", strings.TrimPrefix(displayName, "@"), accountID))
			}

		case "emoji":
			if attrs, ok := inline["attrs"].(map[string]any); ok {
				if shortName, ok := attrs["shortName"].(string); ok {
					parts = append(parts, shortName)
				}
			}

		case "hardBreak":
			parts = append(parts, "  \n")

		case "inlineCard":
			if attrs, ok := inline["attrs"].(map[string]any); ok {
				if url, ok := attrs["url"].(string); ok {
					parts = append(parts, url)
				}
			}

		default:
			// Unknown inline — preserve as opaque.
			parts = append(parts, opaqueMarker(inline))
		}
	}
	return strings.Join(parts, "")
}

// applyMarksMD wraps text with Markdown syntax based on ADF marks.
func applyMarksMD(text string, marks []any) string {
	// Collect link href separately — link wraps the whole text.
	var linkHref string
	for _, m := range marks {
		mark, ok := m.(map[string]any)
		if !ok {
			continue
		}
		markType, _ := mark["type"].(string)
		switch markType {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "underline":
			text = "<u>" + text + "</u>"
		case "link":
			if attrs, ok := mark["attrs"].(map[string]any); ok {
				linkHref, _ = attrs["href"].(string)
			}
		}
	}
	if linkHref != "" {
		text = "[" + text + "](" + linkHref + ")"
	}
	return text
}

// tableToMarkdown converts ADF table rows to Markdown pipe table.
func tableToMarkdown(rows []any, indent int) string {
	prefix := strings.Repeat(" ", indent)
	var table [][]string
	for _, row := range rows {
		rowMap, ok := row.(map[string]any)
		if !ok {
			continue
		}
		cells, _ := rowMap["content"].([]any)
		var rowCells []string
		for _, cell := range cells {
			cellMap, ok := cell.(map[string]any)
			if !ok {
				continue
			}
			cellContent, _ := cellMap["content"].([]any)
			var cellParts []string
			for _, block := range cellContent {
				blockMap, ok := block.(map[string]any)
				if !ok {
					continue
				}
				blockContent, _ := blockMap["content"].([]any)
				cellParts = append(cellParts, inlineToMarkdown(blockContent))
			}
			rowCells = append(rowCells, strings.Join(cellParts, " "))
		}
		table = append(table, rowCells)
	}
	if len(table) == 0 {
		return ""
	}

	// Build pipe table.
	var lines []string
	for i, row := range table {
		lines = append(lines, prefix+"| "+strings.Join(row, " | ")+" |")
		if i == 0 {
			// Header separator.
			var sep []string
			for range row {
				sep = append(sep, "---")
			}
			lines = append(lines, prefix+"| "+strings.Join(sep, " | ")+" |")
		}
	}
	return strings.Join(lines, "\n")
}

// collectPlainText extracts plain text from inline content (no formatting).
func collectPlainText(content []any) string {
	var parts []string
	for _, child := range content {
		inline, ok := child.(map[string]any)
		if !ok {
			continue
		}
		nodeType, _ := inline["type"].(string)
		switch nodeType {
		case "text":
			text, _ := inline["text"].(string)
			parts = append(parts, text)
		case "hardBreak":
			parts = append(parts, "\n")
		}
	}
	return strings.Join(parts, "")
}

// opaqueMarker serializes an unsupported ADF node as an HTML comment marker.
func opaqueMarker(node map[string]any) string {
	nodeType, _ := node["type"].(string)
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Sprintf("<!-- adf:%s (marshal error) -->", nodeType)
	}
	return fmt.Sprintf("<!-- adf:%s %s -->", nodeType, string(data))
}

// ---------------------------------------------------------------------------
// Markdown → ADF
// ---------------------------------------------------------------------------

// markdownToADF converts Markdown text to an ADF document (map[string]any).
//
//nolint:gocognit,funlen // Markdown parser state machine is inherently complex
func markdownToADF(md string) any {
	lines := strings.Split(md, "\n")
	var blocks []any
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Opaque ADF marker: <!-- adf:TYPE {json} -->
		if strings.HasPrefix(strings.TrimSpace(line), "<!-- adf:") {
			if node := restoreOpaqueMarker(line); node != nil {
				blocks = append(blocks, node)
				i++
				continue
			}
		}

		// Fenced code block: ```lang
		trimmed := strings.TrimSpace(line)
		if lang, ok := strings.CutPrefix(trimmed, "```"); ok {
			var codeLines []string
			i++
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == "```" {
					i++
					break
				}
				codeLines = append(codeLines, lines[i])
				i++
			}
			blocks = append(blocks, map[string]any{
				"type":    "codeBlock",
				"attrs":   map[string]any{"language": lang},
				"content": []any{map[string]any{"type": "text", "text": strings.Join(codeLines, "\n")}},
			})
			continue
		}

		// Heading: # text
		if m := headingRe.FindStringSubmatch(line); m != nil {
			level := float64(len(m[1]))
			blocks = append(blocks, map[string]any{
				"type":    "heading",
				"attrs":   map[string]any{"level": level},
				"content": parseInline(m[2]),
			})
			i++
			continue
		}

		// Horizontal rule: ---
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			blocks = append(blocks, map[string]any{"type": "rule"})
			i++
			continue
		}

		// Blockquote: > text
		if strings.HasPrefix(trimmed, "> ") {
			var quoteLines []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "> ") {
				quoteLines = append(quoteLines, strings.TrimPrefix(strings.TrimSpace(lines[i]), "> "))
				i++
			}
			// Recursively parse the quoted content.
			inner := markdownToADF(strings.Join(quoteLines, "\n"))
			innerDoc, _ := inner.(map[string]any)
			innerContent, _ := innerDoc["content"].([]any)
			blocks = append(blocks, map[string]any{
				"type":    "blockquote",
				"content": innerContent,
			})
			continue
		}

		// Table: | cell | cell |
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed[1:], "|") {
			var tableLines []string
			for i < len(lines) {
				tl := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(tl, "|") {
					break
				}
				tableLines = append(tableLines, tl)
				i++
			}
			blocks = append(blocks, parseTable(tableLines))
			continue
		}

		// Bullet list: - text
		if strings.HasPrefix(trimmed, "- ") {
			blocks = append(blocks, parseList(lines, &i, "bullet"))
			continue
		}

		// Ordered list: 1. text
		if orderedListRe.MatchString(trimmed) {
			blocks = append(blocks, parseList(lines, &i, "ordered"))
			continue
		}

		// Empty line — skip.
		if trimmed == "" {
			i++
			continue
		}

		// Paragraph — collect consecutive non-empty, non-special lines.
		var paraLines []string
		for i < len(lines) {
			pl := lines[i]
			ptrimmed := strings.TrimSpace(pl)
			if ptrimmed == "" || strings.HasPrefix(ptrimmed, "#") || strings.HasPrefix(ptrimmed, "```") ||
				strings.HasPrefix(ptrimmed, "> ") || strings.HasPrefix(ptrimmed, "- ") ||
				orderedListRe.MatchString(ptrimmed) || strings.HasPrefix(ptrimmed, "|") ||
				ptrimmed == "---" || ptrimmed == "***" || ptrimmed == "___" ||
				strings.HasPrefix(ptrimmed, "<!-- adf:") {
				break
			}
			paraLines = append(paraLines, pl)
			i++
		}
		text := strings.Join(paraLines, "\n")
		// Split on hard breaks (trailing two spaces + newline).
		inlineContent := parseInlineWithHardBreaks(text)
		blocks = append(blocks, map[string]any{
			"type":    "paragraph",
			"content": inlineContent,
		})
	}

	return map[string]any{
		"type":    "doc",
		"version": float64(1),
		"content": blocks,
	}
}

var (
	headingRe     = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	orderedListRe = regexp.MustCompile(`^\d+\.\s+`)
	boldRe        = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe      = regexp.MustCompile(`(?:^|[^*])\*([^*]+?)\*(?:[^*]|$)`)
	codeRe        = regexp.MustCompile("`([^`]+)`")
	strikeRe      = regexp.MustCompile(`~~(.+?)~~`)
	underlineRe   = regexp.MustCompile(`<u>(.+?)</u>`)
	linkRe        = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mentionRe     = regexp.MustCompile(`\[@([^\]]+)\]\(accountid:([^)]+)\)`)
)

// parseInline converts a Markdown text string to ADF inline content.
func parseInline(text string) []any {
	return parseInlineWithHardBreaks(text)
}

// parseInlineWithHardBreaks parses inline Markdown with hard break support.
// Trailing "  \n" becomes a hardBreak node.
//
//nolint:gocognit,funlen // inline Markdown parser with multiple mark types
func parseInlineWithHardBreaks(text string) []any {
	// Split on hard breaks first.
	segments := strings.Split(text, "  \n")
	var result []any
	for si, segment := range segments {
		if si > 0 {
			result = append(result, map[string]any{"type": "hardBreak"})
		}
		// Also split on plain \n within paragraphs (non-hard-break).
		for li, line := range strings.Split(segment, "\n") {
			if li > 0 {
				result = append(result, map[string]any{"type": "text", "text": " "})
			}
			result = append(result, parseInlineSegment(line)...)
		}
	}
	return result
}

// parseInlineSegment parses a single line of Markdown into ADF inline nodes.
//
//nolint:gocognit,funlen // inline parser with nested regex matching
func parseInlineSegment(text string) []any {
	if text == "" {
		return nil
	}

	// Find the earliest match among all inline patterns.
	type match struct {
		start, end int
		node       any
	}

	var earliest *match

	// Mentions: [@Name](accountid:ID)
	if m := mentionRe.FindStringIndex(text); m != nil {
		sub := mentionRe.FindStringSubmatch(text)
		earliest = &match{m[0], m[1], map[string]any{
			"type": "mention",
			"attrs": map[string]any{
				"text": "@" + sub[1],
				"id":   sub[2],
			},
		}}
	}

	// Links: [text](url) — but not mentions (which start with [@).
	if m := linkRe.FindStringIndex(text); m != nil {
		// Skip if this is a mention.
		if !strings.HasPrefix(text[m[0]:], "[@") {
			sub := linkRe.FindStringSubmatch(text)
			node := map[string]any{
				"type": "text",
				"text": sub[1],
				"marks": []any{map[string]any{
					"type":  "link",
					"attrs": map[string]any{"href": sub[2]},
				}},
			}
			if earliest == nil || m[0] < earliest.start {
				earliest = &match{m[0], m[1], node}
			}
		}
	}

	// Bold: **text**
	if m := boldRe.FindStringIndex(text); m != nil {
		sub := boldRe.FindStringSubmatch(text)
		node := map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": "strong"}},
		}
		if earliest == nil || m[0] < earliest.start {
			earliest = &match{m[0], m[1], node}
		}
	}

	// Inline code: `text`
	if m := codeRe.FindStringIndex(text); m != nil {
		sub := codeRe.FindStringSubmatch(text)
		node := map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": "code"}},
		}
		if earliest == nil || m[0] < earliest.start {
			earliest = &match{m[0], m[1], node}
		}
	}

	// Strikethrough: ~~text~~
	if m := strikeRe.FindStringIndex(text); m != nil {
		sub := strikeRe.FindStringSubmatch(text)
		node := map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": "strike"}},
		}
		if earliest == nil || m[0] < earliest.start {
			earliest = &match{m[0], m[1], node}
		}
	}

	// Underline: <u>text</u>
	if m := underlineRe.FindStringIndex(text); m != nil {
		sub := underlineRe.FindStringSubmatch(text)
		node := map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": "underline"}},
		}
		if earliest == nil || m[0] < earliest.start {
			earliest = &match{m[0], m[1], node}
		}
	}

	// Italic: *text* — must check after bold to avoid false matches.
	if m := italicRe.FindStringIndex(text); m != nil {
		sub := italicRe.FindStringSubmatch(text)
		// Find actual position of the *text* part (italicRe may include leading char).
		actualStart := strings.Index(text[m[0]:], "*"+sub[1]+"*")
		if actualStart >= 0 {
			actualStart += m[0]
			actualEnd := actualStart + len("*"+sub[1]+"*")
			node := map[string]any{
				"type": "text", "text": sub[1],
				"marks": []any{map[string]any{"type": "em"}},
			}
			if earliest == nil || actualStart < earliest.start {
				earliest = &match{actualStart, actualEnd, node}
			}
		}
	}

	if earliest == nil {
		// No inline marks found — plain text.
		return []any{map[string]any{"type": "text", "text": text}}
	}

	var result []any
	// Text before the match.
	if earliest.start > 0 {
		result = append(result, map[string]any{"type": "text", "text": text[:earliest.start]})
	}
	// The matched node.
	result = append(result, earliest.node)
	// Recurse on remainder.
	if earliest.end < len(text) {
		result = append(result, parseInlineSegment(text[earliest.end:])...)
	}
	return result
}

// parseList parses a Markdown list (bullet or ordered) starting at lines[*idx].
func parseList(lines []string, idx *int, listType string) map[string]any {
	adfType := "bulletList"
	markerRe := regexp.MustCompile(`^(\s*)- (.*)$`)
	if listType == "ordered" {
		adfType = "orderedList"
		markerRe = regexp.MustCompile(`^(\s*)\d+\.\s+(.*)$`)
	}

	var items []any
	baseIndent := len(lines[*idx]) - len(strings.TrimLeft(lines[*idx], " "))

	for *idx < len(lines) {
		line := lines[*idx]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if currentIndent < baseIndent {
			break
		}
		if currentIndent > baseIndent {
			// Nested list — handled by the parent item.
			break
		}

		m := markerRe.FindStringSubmatch(line)
		if m == nil {
			break
		}

		text := m[2]
		paraContent := parseInline(text)

		// Check for nested list on next lines.
		*idx++
		var itemContent []any
		itemContent = append(itemContent, map[string]any{
			"type":    "paragraph",
			"content": paraContent,
		})

		// Look for nested items (higher indent).
		if *idx < len(lines) {
			nextLine := lines[*idx]
			nextTrimmed := strings.TrimSpace(nextLine)
			nextIndent := len(nextLine) - len(strings.TrimLeft(nextLine, " "))
			if nextIndent > baseIndent && (strings.HasPrefix(nextTrimmed, "- ") || orderedListRe.MatchString(nextTrimmed)) {
				nestedType := "bullet"
				if orderedListRe.MatchString(nextTrimmed) {
					nestedType = "ordered"
				}
				nested := parseList(lines, idx, nestedType)
				itemContent = append(itemContent, nested)
			}
		}

		items = append(items, map[string]any{
			"type":    "listItem",
			"content": itemContent,
		})
	}

	return map[string]any{
		"type":    adfType,
		"content": items,
	}
}

// parseTable converts Markdown pipe table lines to an ADF table node.
func parseTable(lines []string) map[string]any {
	var rows []any
	for i, line := range lines {
		cells := splitTableRow(line)
		// Skip separator row (| --- | --- |).
		if i == 1 && len(cells) > 0 && isSeparatorRow(cells) {
			continue
		}
		cellType := "tableCell"
		if i == 0 {
			cellType = "tableHeader"
		}
		var adfCells []any
		for _, cell := range cells {
			adfCells = append(adfCells, map[string]any{
				"type": cellType,
				"content": []any{
					map[string]any{
						"type":    "paragraph",
						"content": parseInline(strings.TrimSpace(cell)),
					},
				},
			})
		}
		rows = append(rows, map[string]any{
			"type":    "tableRow",
			"content": adfCells,
		})
	}
	return map[string]any{
		"type":    "table",
		"content": rows,
	}
}

// splitTableRow splits a Markdown table row into cells.
func splitTableRow(line string) []string {
	// Remove leading/trailing pipes and split.
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// isSeparatorRow checks if table cells are all dashes (header separator).
func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		stripped := strings.TrimSpace(c)
		stripped = strings.Trim(stripped, ":-")
		if stripped != "" {
			return false
		}
	}
	return true
}

// restoreOpaqueMarker parses <!-- adf:TYPE {json} --> back to an ADF node.
func restoreOpaqueMarker(line string) map[string]any {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "<!-- adf:") || !strings.HasSuffix(trimmed, "-->") {
		return nil
	}
	// Extract everything between "<!-- adf:TYPE " and " -->".
	inner := strings.TrimPrefix(trimmed, "<!-- adf:")
	inner = strings.TrimSuffix(inner, "-->")
	inner = strings.TrimSpace(inner)

	// Find the JSON part (starts with {).
	jsonStart := strings.Index(inner, "{")
	if jsonStart < 0 {
		return nil
	}
	jsonStr := inner[jsonStart:]

	var node map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &node); err != nil {
		return nil
	}
	return node
}
