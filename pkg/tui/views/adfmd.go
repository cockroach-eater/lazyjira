package views

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

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

// ADFToMarkdown converts an ADF document to Markdown text
func ADFToMarkdown(node any) string {
	return adfToMarkdown(node)
}

// MarkdownToADF converts Markdown text to an ADF document
func MarkdownToADF(md string) any {
	return markdownToADF(md)
}

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

//nolint:gocognit
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
		return opaqueMarker(block)
	}
}

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
			parts = append(parts, opaqueMarker(inline))
		}
	}
	return strings.Join(parts, "")
}

func applyMarksMD(text string, marks []any) string {
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

	var lines []string
	for i, row := range table {
		lines = append(lines, prefix+"| "+strings.Join(row, " | ")+" |")
		if i == 0 {
			var sep []string
			for range row {
				sep = append(sep, "---")
			}
			lines = append(lines, prefix+"| "+strings.Join(sep, " | ")+" |")
		}
	}
	return strings.Join(lines, "\n")
}

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

func opaqueMarker(node map[string]any) string {
	nodeType, _ := node["type"].(string)
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Sprintf("<!-- adf:%s (marshal error) -->", nodeType)
	}
	return fmt.Sprintf("<!-- adf:%s %s -->", nodeType, string(data))
}

func markdownToADF(md string) any {
	lines := strings.Split(md, "\n")
	var blocks []any
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<!-- adf:") {
			if node := restoreOpaqueMarker(line); node != nil {
				blocks = append(blocks, node)
				i++
				continue
			}
		}

		if block, ok := tryParseCodeBlock(lines, &i, trimmed); ok {
			blocks = append(blocks, block)
			continue
		}

		if m := headingRe.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, map[string]any{
				"type":    "heading",
				"attrs":   map[string]any{"level": float64(len(m[1]))},
				"content": parseInline(m[2]),
			})
			i++
			continue
		}

		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			blocks = append(blocks, map[string]any{"type": "rule"})
			i++
			continue
		}

		if block, ok := tryParseBlockquote(lines, &i, trimmed); ok {
			blocks = append(blocks, block)
			continue
		}

		if block, ok := tryParseTable(lines, &i, trimmed); ok {
			blocks = append(blocks, block)
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			blocks = append(blocks, parseList(lines, &i, "bullet"))
			continue
		}
		if orderedListRe.MatchString(trimmed) {
			blocks = append(blocks, parseList(lines, &i, "ordered"))
			continue
		}

		if trimmed == "" {
			i++
			continue
		}

		blocks = append(blocks, parseParagraph(lines, &i))
	}

	return map[string]any{
		"type":    "doc",
		"version": float64(1),
		"content": blocks,
	}
}

func tryParseCodeBlock(lines []string, i *int, trimmed string) (any, bool) {
	lang, ok := strings.CutPrefix(trimmed, "```")
	if !ok {
		return nil, false
	}
	var codeLines []string
	*i++
	for *i < len(lines) {
		if strings.TrimSpace(lines[*i]) == "```" {
			*i++
			break
		}
		codeLines = append(codeLines, lines[*i])
		*i++
	}
	return map[string]any{
		"type":    "codeBlock",
		"attrs":   map[string]any{"language": lang},
		"content": []any{map[string]any{"type": "text", "text": strings.Join(codeLines, "\n")}},
	}, true
}

func tryParseBlockquote(lines []string, i *int, trimmed string) (any, bool) {
	if !strings.HasPrefix(trimmed, "> ") {
		return nil, false
	}
	var quoteLines []string
	for *i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[*i]), "> ") {
		quoteLines = append(quoteLines, strings.TrimPrefix(strings.TrimSpace(lines[*i]), "> "))
		*i++
	}
	inner := markdownToADF(strings.Join(quoteLines, "\n"))
	innerDoc, _ := inner.(map[string]any)
	innerContent, _ := innerDoc["content"].([]any)
	return map[string]any{
		"type":    "blockquote",
		"content": innerContent,
	}, true
}

func tryParseTable(lines []string, i *int, trimmed string) (any, bool) {
	if !strings.HasPrefix(trimmed, "|") || !strings.Contains(trimmed[1:], "|") {
		return nil, false
	}
	var tableLines []string
	for *i < len(lines) {
		tl := strings.TrimSpace(lines[*i])
		if !strings.HasPrefix(tl, "|") {
			break
		}
		tableLines = append(tableLines, tl)
		*i++
	}
	return parseTable(tableLines), true
}

func parseParagraph(lines []string, i *int) any {
	var paraLines []string
	for *i < len(lines) {
		pl := lines[*i]
		ptrimmed := strings.TrimSpace(pl)
		if isBlockBreak(ptrimmed) {
			break
		}
		paraLines = append(paraLines, pl)
		*i++
	}
	text := strings.Join(paraLines, "\n")
	return map[string]any{
		"type":    "paragraph",
		"content": parseInlineWithHardBreaks(text),
	}
}

func isBlockBreak(trimmed string) bool {
	return trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") ||
		strings.HasPrefix(trimmed, "> ") || strings.HasPrefix(trimmed, "- ") ||
		orderedListRe.MatchString(trimmed) || strings.HasPrefix(trimmed, "|") ||
		trimmed == "---" || trimmed == "***" || trimmed == "___" ||
		strings.HasPrefix(trimmed, "<!-- adf:")
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

func parseInline(text string) []any {
	return parseInlineWithHardBreaks(text)
}

func parseInlineWithHardBreaks(text string) []any {
	segments := strings.Split(text, "  \n")
	var result []any
	for si, segment := range segments {
		if si > 0 {
			result = append(result, map[string]any{"type": "hardBreak"})
		}
		for li, line := range strings.Split(segment, "\n") {
			if li > 0 {
				result = append(result, map[string]any{"type": "text", "text": " "})
			}
			result = append(result, parseInlineSegment(line)...)
		}
	}
	return result
}

type inlineRule struct {
	re    *regexp.Regexp
	build func(text string, loc []int, sub []string) (start, end int, node any, ok bool)
}

var inlineRules = []inlineRule{
	{mentionRe, func(_ string, loc []int, sub []string) (int, int, any, bool) {
		return loc[0], loc[1], map[string]any{
			"type":  "mention",
			"attrs": map[string]any{"text": "@" + sub[1], "id": sub[2]},
		}, true
	}},
	{linkRe, func(text string, loc []int, sub []string) (int, int, any, bool) {
		if strings.HasPrefix(text[loc[0]:], "[@") {
			return 0, 0, nil, false
		}
		return loc[0], loc[1], map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": "link", "attrs": map[string]any{"href": sub[2]}}},
		}, true
	}},
	{boldRe, markRule("strong")},
	{codeRe, markRule("code")},
	{strikeRe, markRule("strike")},
	{underlineRe, markRule("underline")},
	{italicRe, func(text string, loc []int, sub []string) (int, int, any, bool) {
		actualStart := strings.Index(text[loc[0]:], "*"+sub[1]+"*")
		if actualStart < 0 {
			return 0, 0, nil, false
		}
		actualStart += loc[0]
		actualEnd := actualStart + len("*"+sub[1]+"*")
		return actualStart, actualEnd, map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": "em"}},
		}, true
	}},
}

func markRule(markType string) func(string, []int, []string) (int, int, any, bool) {
	return func(_ string, loc []int, sub []string) (int, int, any, bool) {
		return loc[0], loc[1], map[string]any{
			"type": "text", "text": sub[1],
			"marks": []any{map[string]any{"type": markType}},
		}, true
	}
}

func parseInlineSegment(text string) []any {
	if text == "" {
		return nil
	}

	type inlineMatch struct {
		start, end int
		node       any
	}

	var earliest *inlineMatch
	for _, rule := range inlineRules {
		loc := rule.re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		sub := rule.re.FindStringSubmatch(text)
		start, end, node, ok := rule.build(text, loc, sub)
		if !ok {
			continue
		}
		if earliest == nil || start < earliest.start {
			earliest = &inlineMatch{start, end, node}
		}
	}

	if earliest == nil {
		return []any{map[string]any{"type": "text", "text": text}}
	}

	var result []any
	if earliest.start > 0 {
		result = append(result, map[string]any{"type": "text", "text": text[:earliest.start]})
	}
	result = append(result, earliest.node)
	if earliest.end < len(text) {
		result = append(result, parseInlineSegment(text[earliest.end:])...)
	}
	return result
}

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
			break
		}

		m := markerRe.FindStringSubmatch(line)
		if m == nil {
			break
		}

		text := m[2]
		paraContent := parseInline(text)

		*idx++
		var itemContent []any
		itemContent = append(itemContent, map[string]any{
			"type":    "paragraph",
			"content": paraContent,
		})

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

func parseTable(lines []string) map[string]any {
	var rows []any
	for i, line := range lines {
		cells := splitTableRow(line)
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

func splitTableRow(line string) []string {
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

func restoreOpaqueMarker(line string) map[string]any {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "<!-- adf:") || !strings.HasSuffix(trimmed, "-->") {
		return nil
	}
	inner := strings.TrimPrefix(trimmed, "<!-- adf:")
	inner = strings.TrimSuffix(inner, "-->")
	inner = strings.TrimSpace(inner)

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
