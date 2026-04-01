package views

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

// renderADF converts raw ADF JSON (Atlassian Document Format) to styled terminal lines.
// Returns nil if node is nil or not a valid ADF document.
func renderADF(node any, width int) []string {
	doc, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	content, ok := doc["content"].([]any)
	if !ok {
		return nil
	}
	r := &adfRenderer{width: width}
	for _, child := range content {
		r.renderBlock(child, 0)
	}
	return r.lines
}

type adfRenderer struct {
	width int
	lines []string
}

// renderBlock handles block-level ADF nodes.
//
//nolint:gocognit // ADF block dispatcher complexity is inherent to the format
func (r *adfRenderer) renderBlock(node any, indent int) {
	block, ok := node.(map[string]any)
	if !ok {
		return
	}
	nodeType, _ := block["type"].(string)
	content, _ := block["content"].([]any)

	switch nodeType {
	case adfParagraph:
		text := r.collectInline(content)
		if text == "" {
			r.lines = append(r.lines, "")
			return
		}
		r.appendWrapped(text, indent, "")

	case adfHeading:
		level := 1
		if attrs, ok := block["attrs"].(map[string]any); ok {
			if l, ok := attrs["level"].(float64); ok {
				level = int(l)
			}
		}
		text := r.collectInlinePlain(content)
		style := headingStyle(level)
		r.lines = append(r.lines, "")
		prefix := strings.Repeat(" ", indent)
		r.lines = append(r.lines, prefix+style.Render(text))

	case adfBulletList:
		for _, item := range content {
			r.renderListItem(item, indent, "• ")
		}

	case adfOrderedList:
		for i, item := range content {
			r.renderListItem(item, indent, fmt.Sprintf("%d. ", i+1))
		}

	case adfCodeBlock:
		lang := ""
		if attrs, ok := block["attrs"].(map[string]any); ok {
			lang, _ = attrs["language"].(string)
		}
		borderStyle := lipgloss.NewStyle().Foreground(theme.ColorGray)
		if lang != "" {
			r.lines = append(r.lines, borderStyle.Render("  ┌ "+lang))
		}
		text := r.collectInlinePlain(content)
		highlighted := highlightCode(text, lang)
		for _, line := range strings.Split(highlighted, "\n") {
			r.lines = append(r.lines, borderStyle.Render("  │ ")+line)
		}
		if lang != "" {
			r.lines = append(r.lines, borderStyle.Render("  └"))
		}

	case adfBlockquote:
		quoteStyle := lipgloss.NewStyle().Foreground(theme.ColorGray)
		bar := quoteStyle.Render("│ ")
		for _, child := range content {
			// Render child blocks, then prepend quote bar.
			sub := &adfRenderer{width: r.width - 4}
			sub.renderBlock(child, 0)
			for _, line := range sub.lines {
				r.lines = append(r.lines, "  "+bar+line)
			}
		}

	case adfRule:
		w := max(r.width-4, 10)
		ruleStyle := lipgloss.NewStyle().Foreground(theme.ColorGray)
		r.lines = append(r.lines, ruleStyle.Render("  "+strings.Repeat("─", w)))

	case adfTable:
		r.renderTable(content)

	case "mediaSingle", "mediaGroup": //nolint:goconst // rare node types not worth constants
		r.lines = append(r.lines, lipgloss.NewStyle().Foreground(theme.ColorGray).Render("  [media]"))

	default:
		// Unknown block — recurse into content as fallback.
		for _, child := range content {
			r.renderBlock(child, indent)
		}
	}
}

// renderListItem renders a single list item with marker and indent.
func (r *adfRenderer) renderListItem(node any, indent int, marker string) {
	item, ok := node.(map[string]any)
	if !ok {
		return
	}
	content, _ := item["content"].([]any)
	first := true
	for _, child := range content {
		childBlock, ok := child.(map[string]any)
		if !ok {
			continue
		}
		childType, _ := childBlock["type"].(string)

		markerW := lipgloss.Width(marker)
		switch childType {
		case adfParagraph:
			childContent, _ := childBlock["content"].([]any)
			text := r.collectInline(childContent)
			if first {
				r.appendWrapped(text, indent, marker)
				first = false
			} else {
				r.appendWrapped(text, indent+markerW, "")
			}
		case adfBulletList, adfOrderedList:
			// Nested list — increase indent.
			r.renderBlock(child, indent+2)
		default:
			r.renderBlock(child, indent+markerW)
		}
	}
}

// collectInline concatenates inline nodes into a single styled string.
func (r *adfRenderer) collectInline(content []any) string {
	var parts []string
	for _, child := range content {
		if s := r.renderInline(child); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "")
}

// collectInlinePlain concatenates inline nodes as plain text (no ANSI).
func (r *adfRenderer) collectInlinePlain(content []any) string {
	var parts []string
	for _, child := range content {
		if s := r.renderInlinePlain(child); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "")
}

// renderInline returns a styled string for an inline ADF node.
func (r *adfRenderer) renderInline(node any) string {
	inline, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	nodeType, _ := inline["type"].(string)

	switch nodeType {
	case adfText:
		text, _ := inline["text"].(string)
		text = strings.ReplaceAll(text, "\r", "")
		marks, _ := inline["marks"].([]any)
		return applyMarks(text, marks)

	case adfMention:
		if attrs, ok := inline["attrs"].(map[string]any); ok {
			if text, ok := attrs["text"].(string); ok {
				return "\x00MENTION:" + text + "\x00"
			}
		}

	case adfEmoji:
		if attrs, ok := inline["attrs"].(map[string]any); ok {
			if shortName, ok := attrs["shortName"].(string); ok {
				return shortName
			}
		}

	case adfHardBreak:
		return "\n"

	case adfInlineCard:
		if attrs, ok := inline["attrs"].(map[string]any); ok {
			if url, ok := attrs["url"].(string); ok {
				return urlStyle.Render(url)
			}
		}
	}
	return ""
}

// renderInlinePlain returns plain text (no ANSI) for an inline ADF node.
func (r *adfRenderer) renderInlinePlain(node any) string {
	inline, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	nodeType, _ := inline["type"].(string)

	switch nodeType {
	case adfText:
		text, _ := inline["text"].(string)
		return strings.ReplaceAll(text, "\r", "")
	case adfMention:
		if attrs, ok := inline["attrs"].(map[string]any); ok {
			if text, ok := attrs["text"].(string); ok {
				return text
			}
		}
	case adfEmoji:
		if attrs, ok := inline["attrs"].(map[string]any); ok {
			if shortName, ok := attrs["shortName"].(string); ok {
				return shortName
			}
		}
	case adfHardBreak:
		return "\n"
	case adfInlineCard:
		if attrs, ok := inline["attrs"].(map[string]any); ok {
			if url, ok := attrs["url"].(string); ok {
				return url
			}
		}
	}
	return ""
}

// applyMarks applies ADF text marks (bold, italic, code, etc.) to text.
func applyMarks(text string, marks []any) string {
	for _, m := range marks {
		mark, ok := m.(map[string]any)
		if !ok {
			continue
		}
		markType, _ := mark["type"].(string)
		switch markType {
		case "strong":
			text = lipgloss.NewStyle().Bold(true).Render(text)
		case "em":
			text = lipgloss.NewStyle().Italic(true).Render(text)
		case "code":
			text = lipgloss.NewStyle().Foreground(theme.ColorCyan).Render(text)
		case "underline":
			text = lipgloss.NewStyle().Underline(true).Render(text)
		case "strike":
			text = lipgloss.NewStyle().Strikethrough(true).Render(text)
		case "link":
			text = urlStyle.Render(text)
		case "textColor":
			if attrs, ok := mark["attrs"].(map[string]any); ok {
				if color, ok := attrs["color"].(string); ok {
					text = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(text)
				}
			}
		}
	}
	return text
}

// headingStyle returns the lipgloss style for a heading level.
func headingStyle(level int) lipgloss.Style {
	switch level {
	case 1:
		return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorGreen)
	case 2:
		return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorGreen)
	case 3:
		return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorWhite)
	case 4:
		return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorWhite)
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorGray)
	}
}

// appendWrapped word-wraps text and appends to lines with indent and optional marker on first line.
// Uses lipgloss for ANSI-aware wrapping so styled text (bold, links, code) isn't broken.
func (r *adfRenderer) appendWrapped(text string, indent int, marker string) {
	prefix := strings.Repeat(" ", indent)
	markerW := lipgloss.Width(marker)
	contPrefix := prefix + strings.Repeat(" ", markerW)
	w := max(r.width-indent-markerW, 10)

	wrapStyle := lipgloss.NewStyle().Width(w)
	first := true
	// Split by explicit newlines (hardBreak), then wrap each paragraph.
	for _, para := range strings.Split(text, "\n") {
		wrapped := wrapStyle.Render(para)
		for _, line := range strings.Split(wrapped, "\n") {
			styled := colorMentions(line)
			if first {
				r.lines = append(r.lines, prefix+marker+styled)
				first = false
			} else {
				r.lines = append(r.lines, contPrefix+styled)
			}
		}
	}
}

// renderTable renders an ADF table (basic implementation).
func (r *adfRenderer) renderTable(rows []any) {
	if len(rows) == 0 {
		return
	}
	tblStyle := lipgloss.NewStyle().Foreground(theme.ColorGray)

	// Collect cells as plain text.
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
			// Flatten cell content to plain text.
			var cellText []string
			for _, block := range cellContent {
				blockMap, ok := block.(map[string]any)
				if !ok {
					continue
				}
				content, _ := blockMap["content"].([]any)
				cellText = append(cellText, r.collectInlinePlain(content))
			}
			rowCells = append(rowCells, strings.Join(cellText, " "))
		}
		table = append(table, rowCells)
	}

	if len(table) == 0 {
		return
	}

	// Calculate column widths.
	colCount := 0
	for _, row := range table {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	colWidths := make([]int, colCount)
	for _, row := range table {
		for i, cell := range row {
			if w := lipgloss.Width(cell); w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}
	// Cap total width.
	maxColW := max((r.width-4-colCount)/max(colCount, 1), 5)
	for i := range colWidths {
		if colWidths[i] > maxColW {
			colWidths[i] = maxColW
		}
	}

	for ri, row := range table {
		var parts []string
		for i := range colCount {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			cellW := lipgloss.Width(cell)
			if cellW > colWidths[i] {
				cell = components.TruncateEnd(cell, colWidths[i])
				cellW = lipgloss.Width(cell)
			}
			// Pad to column width using display width.
			if cellW < colWidths[i] {
				cell += strings.Repeat(" ", colWidths[i]-cellW)
			}
			parts = append(parts, cell)
		}
		line := "  " + strings.Join(parts, " │ ")
		if ri == 0 {
			r.lines = append(r.lines, lipgloss.NewStyle().Bold(true).Render(line))
			// Separator after header.
			var sepParts []string
			for _, w := range colWidths {
				sepParts = append(sepParts, strings.Repeat("─", w))
			}
			r.lines = append(r.lines, tblStyle.Render("  "+strings.Join(sepParts, "─┼─")))
		} else {
			r.lines = append(r.lines, line)
		}
	}
}

// highlightCode applies syntax highlighting using chroma.
// Falls back to plain text if language is unknown or highlighting fails.
func highlightCode(code, lang string) string {
	if lang == "" {
		return code
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return code
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	formatter := formatters.Get("terminal256")

	tokens, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, tokens); err != nil {
		return code
	}
	return strings.TrimRight(buf.String(), "\n")
}

// extractADFURLs recursively extracts all URLs from an ADF document.
// Finds URLs in link marks, inlineCard nodes, and plain text.
//
//nolint:gocognit // recursive ADF walker with multiple node types
func extractADFURLs(node any) []string {
	var urls []string
	switch v := node.(type) {
	case map[string]any:
		nodeType, _ := v["type"].(string)

		// inlineCard: {"type":"inlineCard","attrs":{"url":"..."}}
		if nodeType == "inlineCard" {
			if attrs, ok := v["attrs"].(map[string]any); ok {
				if u, ok := attrs["url"].(string); ok {
					urls = append(urls, u)
				}
			}
		}

		// text with link mark: {"type":"text","marks":[{"type":"link","attrs":{"href":"..."}}]}
		if marks, ok := v["marks"].([]any); ok {
			for _, m := range marks {
				if mark, ok := m.(map[string]any); ok {
					if mt, _ := mark["type"].(string); mt == "link" {
						if attrs, ok := mark["attrs"].(map[string]any); ok {
							if href, ok := attrs["href"].(string); ok {
								urls = append(urls, href)
							}
						}
					}
				}
			}
		}

		// Also find plain-text URLs in text nodes.
		if nodeType == "text" {
			if text, ok := v["text"].(string); ok {
				urls = append(urls, findURLs(text)...)
			}
		}

		// Recurse into content.
		if content, ok := v["content"].([]any); ok {
			for _, child := range content {
				urls = append(urls, extractADFURLs(child)...)
			}
		}

	case []any:
		for _, child := range v {
			urls = append(urls, extractADFURLs(child)...)
		}
	}
	return urls
}
