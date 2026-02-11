package article

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	nethtml "golang.org/x/net/html"
)

func (r htmlArticleRenderer) renderNodes(nodes []*nethtml.Node, listDepth int) []string {
	lines := make([]string, 0, len(nodes)*2)
	inlineParts := make([]string, 0, 4)
	flushInline := func() {
		text := normalizeInlineText(strings.Join(inlineParts, " "))
		inlineParts = inlineParts[:0]
		if text == "" {
			return
		}
		block := wrapText(text, r.width)
		if len(block) == 0 {
			return
		}
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}

	for _, node := range nodes {
		switch node.Type {
		case nethtml.TextNode:
			inlineParts = append(inlineParts, node.Data)
		case nethtml.ElementNode:
			if isBlockElement(node.Data) {
				flushInline()
				block := r.renderBlock(node, listDepth)
				if len(block) == 0 {
					continue
				}
				if len(lines) > 0 && lines[len(lines)-1] != "" {
					lines = append(lines, "")
				}
				lines = append(lines, block...)
				continue
			}
			inlineParts = append(inlineParts, r.renderInlineNode(node))
		}
	}
	flushInline()
	return trimBlankLines(lines)
}

func (r htmlArticleRenderer) renderBlock(node *nethtml.Node, listDepth int) []string {
	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "noscript":
		return nil
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		prefix := headingPrefix(level)
		text := normalizeInlineText(r.renderInlineChildren(node))
		return styleNonBlankLines(
			wrapPrefixedText(text, r.width, prefix, strings.Repeat(" ", visibleLen(prefix))),
			detailHeadingStyle,
		)
	case "p", "div", "section", "article", "main", "header", "footer", "aside", "nav":
		if hasBlockChild(node) {
			return r.renderNodes(elementChildren(node), listDepth)
		}
		text := normalizeInlineText(r.renderInlineChildren(node))
		if text != "" {
			return wrapText(text, r.width)
		}
		return r.renderNodes(elementChildren(node), listDepth)
	case "blockquote":
		inner := r.renderNodes(elementChildren(node), listDepth)
		if len(inner) == 0 {
			text := normalizeInlineText(r.renderInlineChildren(node))
			if text == "" {
				return nil
			}
			inner = wrapText(text, r.width-2)
		}
		out := make([]string, 0, len(inner))
		for _, line := range inner {
			if strings.TrimSpace(line) == "" {
				out = append(out, "")
				continue
			}
			out = append(out, detailQuotePrefix+detailQuoteText.Render(line))
		}
		return out
	case "ul":
		return r.renderList(node, false, listDepth+1)
	case "ol":
		return r.renderList(node, true, listDepth+1)
	case "table":
		return renderTableLines(node, r)
	case "figcaption", "caption":
		text := normalizeInlineText(r.renderInlineChildren(node))
		return styleNonBlankLines(
			wrapPrefixedText(text, r.width, "— ", "  "),
			detailCitation,
		)
	case "figure":
		return r.renderNodes(elementChildren(node), listDepth)
	case "img":
		if r.opts.ImageMode == ImageModeNone {
			return nil
		}
		return renderImageLabel(node, r.width)
	case "pre":
		text := strings.ReplaceAll(collectRawText(node), "\r\n", "\n")
		rawLines := strings.Split(text, "\n")
		out := make([]string, 0, len(rawLines))
		for _, line := range rawLines {
			line = strings.TrimRight(line, " \t")
			if line == "" {
				out = append(out, "")
				continue
			}
			out = append(out, "    "+line)
		}
		return trimBlankLines(out)
	case "hr":
		return []string{strings.Repeat("-", min(max(r.width, 3), 24))}
	case "dl":
		return r.renderDefinitionList(node, listDepth)
	case "li":
		return r.renderListItem(node, listDepth, "- ")
	default:
		text := normalizeInlineText(r.renderInlineChildren(node))
		if text != "" {
			return wrapText(text, r.width)
		}
		return r.renderNodes(elementChildren(node), listDepth)
	}
}

func (r htmlArticleRenderer) renderDefinitionList(node *nethtml.Node, listDepth int) []string {
	lines := make([]string, 0, 8)
	indent := strings.Repeat("  ", max(0, listDepth-1))
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.ElementNode {
			continue
		}
		switch strings.ToLower(child.Data) {
		case "dt":
			text := normalizeInlineText(r.renderInlineChildren(child))
			if text == "" {
				continue
			}
			lines = append(lines, wrapPrefixedText(text, r.width, indent+"• ", indent+"  ")...)
		case "dd":
			text := normalizeInlineText(r.renderInlineChildren(child))
			if text == "" {
				continue
			}
			lines = append(lines, wrapPrefixedText(text, r.width, indent+"  ", indent+"  ")...)
		}
	}
	return trimBlankLines(lines)
}

func (r htmlArticleRenderer) renderList(node *nethtml.Node, ordered bool, listDepth int) []string {
	lines := make([]string, 0, 16)
	itemIndex := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.ElementNode || strings.ToLower(child.Data) != "li" {
			continue
		}
		itemIndex++
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", itemIndex)
		} else {
			marker = unorderedListMarker(listDepth)
		}
		itemLines := r.renderListItem(child, listDepth, marker)
		if len(itemLines) == 0 {
			continue
		}
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, itemLines...)
	}
	return trimBlankLines(lines)
}

func (r htmlArticleRenderer) renderListItem(node *nethtml.Node, listDepth int, marker string) []string {
	indent := strings.Repeat("  ", max(0, listDepth-1))
	firstPrefix := indent + marker
	restPrefix := indent + strings.Repeat(" ", visibleLen(marker))
	lines := make([]string, 0, 8)

	textParts := make([]string, 0, 4)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode {
			tag := strings.ToLower(child.Data)
			if tag == "ul" || tag == "ol" {
				continue
			}
		}
		textParts = append(textParts, r.renderInlineNode(child))
	}
	text := normalizeInlineText(strings.Join(textParts, " "))
	if text != "" {
		lines = append(lines, wrapPrefixedText(text, r.width, firstPrefix, restPrefix)...)
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.ElementNode {
			continue
		}
		tag := strings.ToLower(child.Data)
		var nested []string
		switch tag {
		case "ul":
			nested = r.renderList(child, false, listDepth+1)
		case "ol":
			nested = r.renderList(child, true, listDepth+1)
		}
		if len(nested) == 0 {
			continue
		}
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, nested...)
	}
	return trimBlankLines(lines)
}

func wrapPrefixedText(text string, width int, firstPrefix, restPrefix string) []string {
	text = normalizeInlineText(text)
	if text == "" {
		return nil
	}
	if width < 1 {
		return []string{firstPrefix + text}
	}
	firstWidth := max(1, width-visibleLen(firstPrefix))
	restWidth := max(1, width-visibleLen(restPrefix))
	paragraphs := strings.Split(text, "\n")
	out := make([]string, 0, len(paragraphs))
	firstLine := true
	for _, p := range paragraphs {
		p = normalizeInlineText(p)
		if p == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		lineWidth := restWidth
		if firstLine {
			lineWidth = firstWidth
		}
		wrapped := wrapText(p, lineWidth)
		for i, line := range wrapped {
			if firstLine && i == 0 {
				out = append(out, firstPrefix+line)
				continue
			}
			out = append(out, restPrefix+line)
		}
		firstLine = false
	}
	return trimBlankLines(out)
}

func headingPrefix(level int) string {
	if level < 1 {
		level = 1
	}
	if level > len(detailHeadingBars) {
		level = len(detailHeadingBars)
	}
	style := detailHeadingBars[level-1]
	return style.Render("▌") + strings.Repeat(" ", max(1, level-1))
}

func unorderedListMarker(listDepth int) string {
	switch listDepth {
	case 1:
		return "• "
	case 2:
		return "◦ "
	case 3:
		return "▪ "
	default:
		return "▫ "
	}
}

func styleNonBlankLines(lines []string, style lipgloss.Style) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			out[i] = line
			continue
		}
		out[i] = style.Render(line)
	}
	return out
}

func isBlockElement(tag string) bool {
	switch strings.ToLower(tag) {
	case "h1", "h2", "h3", "h4", "h5", "h6",
		"p", "div", "section", "article", "main", "header", "footer", "aside", "nav",
		"blockquote", "ul", "ol", "li", "table", "thead", "tbody", "tfoot", "tr", "td", "th", "img",
		"dl", "dt", "dd", "pre", "figure", "figcaption", "caption", "hr":
		return true
	default:
		return false
	}
}

func hasBlockChild(node *nethtml.Node) bool {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode && isBlockElement(child.Data) {
			return true
		}
	}
	return false
}
