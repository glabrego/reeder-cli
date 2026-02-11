package article

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	nethtml "golang.org/x/net/html"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var reANSICodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var reHTTPURL = regexp.MustCompile(`https?://[^\s)]+`)

var (
	cpMauve    = lipgloss.Color("#cba6f7")
	cpPeach    = lipgloss.Color("#fab387")
	cpYellow   = lipgloss.Color("#f9e2af")
	cpGreen    = lipgloss.Color("#a6e3a1")
	cpTeal     = lipgloss.Color("#94e2d5")
	cpBlue     = lipgloss.Color("#89b4fa")
	cpLavender = lipgloss.Color("#b4befe")
	cpSubtext0 = lipgloss.Color("#a6adc8")
	cpSubtext1 = lipgloss.Color("#bac2de")
	cpOverlay0 = lipgloss.Color("#6c7086")
	cpOverlay1 = lipgloss.Color("#7f849c")
	cpSurface2 = lipgloss.Color("#585b70")

	detailHeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(cpLavender)
	detailHeadingBars  = []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(cpBlue),
		lipgloss.NewStyle().Bold(true).Foreground(cpMauve),
		lipgloss.NewStyle().Bold(true).Foreground(cpTeal),
		lipgloss.NewStyle().Bold(true).Foreground(cpGreen),
		lipgloss.NewStyle().Bold(true).Foreground(cpYellow),
		lipgloss.NewStyle().Bold(true).Foreground(cpPeach),
	}
	detailLinkURL     = lipgloss.NewStyle().Foreground(cpBlue).Faint(true)
	detailQuotePrefix = lipgloss.NewStyle().Foreground(cpOverlay1).Render("│ ")
	detailQuoteText   = lipgloss.NewStyle().Italic(true).Foreground(cpSubtext0)
	detailCitation    = lipgloss.NewStyle().Italic(true).Foreground(cpOverlay0).Faint(true)
	detailCodeStyle   = lipgloss.NewStyle().Foreground(cpPeach)
	detailTableBorder = lipgloss.NewStyle().Foreground(cpSurface2)
	detailTableHeader = lipgloss.NewStyle().Bold(true).Foreground(cpYellow)
	detailImageLabel  = lipgloss.NewStyle().Foreground(cpMauve).Faint(true).Italic(true)
	detailImageText   = lipgloss.NewStyle().Foreground(cpSubtext1).Italic(true)
)

type readerFilterRuleSet struct {
	skipParagraphContains []string
	skipParagraphEquals   []string
	endBeforeContains     []string
	endBeforeEquals       []string
	replaceAll            map[string]string
}

type ImageMode int

const (
	ImageModeLabel ImageMode = iota
	ImageModeNone
)

type Options struct {
	StyleLinks          bool
	ApplyPostprocessing bool
	ImageMode           ImageMode
}

var DefaultOptions = Options{
	StyleLinks:          true,
	ApplyPostprocessing: true,
	ImageMode:           ImageModeLabel,
}

func withDefaults(opts Options) Options {
	out := opts
	if out.ImageMode != ImageModeLabel && out.ImageMode != ImageModeNone {
		out.ImageMode = DefaultOptions.ImageMode
	}
	return out
}

type htmlArticleRenderer struct {
	width int
	opts  Options
}

func ContentLines(entry feedbin.Entry, width int) []string {
	return ContentLinesWithOptions(entry, width, DefaultOptions)
}

func ContentLinesWithOptions(entry feedbin.Entry, width int, opts Options) []string {
	content := strings.TrimSpace(entry.Content)
	if content == "" {
		summary := strings.TrimSpace(entry.Summary)
		if summary == "" {
			return nil
		}
		return wrapText(summary, width)
	}
	lines := renderHTMLFragmentLines(content, width, entry.URL, withDefaults(opts))
	if len(lines) > 0 {
		return lines
	}
	text := TextFromEntryWithOptions(entry, opts)
	if text == "" {
		return nil
	}
	return wrapText(text, width)
}

func TextFromEntry(entry feedbin.Entry) string {
	return TextFromEntryWithOptions(entry, DefaultOptions)
}

func TextFromEntryWithOptions(entry feedbin.Entry, opts Options) string {
	content := strings.TrimSpace(entry.Content)
	if content != "" {
		if lines := renderHTMLFragmentLines(content, 80, entry.URL, withDefaults(opts)); len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}
	return strings.TrimSpace(entry.Summary)
}

func renderHTMLFragmentLines(raw string, width int, articleURL string, opts Options) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	doc, err := nethtml.Parse(strings.NewReader("<html><body>" + raw + "</body></html>"))
	if err != nil {
		return wrapText(strings.TrimSpace(html.UnescapeString(raw)), width)
	}
	body := findBodyNode(doc)
	if body == nil {
		return wrapText(strings.TrimSpace(html.UnescapeString(raw)), width)
	}
	renderer := htmlArticleRenderer{width: max(1, width), opts: opts}
	lines := trimBlankLines(renderer.renderNodes(elementChildren(body), 0))
	if opts.ApplyPostprocessing {
		lines = applyReaderPostprocessing(lines, articleURL)
	}
	if opts.StyleLinks {
		lines = styleDetailLinks(lines)
	}
	return lines
}

func applyReaderPostprocessing(lines []string, articleURL string) []string {
	if len(lines) == 0 {
		return nil
	}
	rules := readerFilterRules(articleURL)
	if len(rules.replaceAll) > 0 {
		for i := range lines {
			for old, newVal := range rules.replaceAll {
				lines[i] = strings.ReplaceAll(lines[i], old, newVal)
			}
		}
	}
	paragraphs := paragraphsFromLines(lines)
	if len(paragraphs) == 0 {
		return nil
	}
	kept := make([][]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		plain := normalizeRuleText(strings.Join(paragraph, " "))
		if plain == "" {
			continue
		}
		if matchesAnyContains(plain, rules.endBeforeContains) || matchesAnyEquals(plain, rules.endBeforeEquals) {
			break
		}
		if matchesAnyContains(plain, rules.skipParagraphContains) || matchesAnyEquals(plain, rules.skipParagraphEquals) {
			continue
		}
		kept = append(kept, paragraph)
	}
	if len(kept) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	for i, p := range kept {
		out = append(out, p...)
		if i < len(kept)-1 {
			out = append(out, "")
		}
	}
	return trimBlankLines(out)
}

func readerFilterRules(articleURL string) readerFilterRuleSet {
	host := strings.ToLower(strings.TrimSpace(articleURL))
	if parsed, err := url.Parse(articleURL); err == nil && parsed.Host != "" {
		host = strings.ToLower(parsed.Hostname())
	}
	rules := readerFilterRuleSet{}
	switch {
	case strings.Contains(host, "wikipedia.org"):
		rules.replaceAll = map[string]string{"[edit]": ""}
		rules.endBeforeEquals = []string{"references", "footnotes", "see also", "notes"}
	case strings.Contains(host, "nytimes.com"):
		rules.skipParagraphContains = []string{"credit:", "this is a developing story. check back for updates."}
		rules.skipParagraphEquals = []string{"credit", "image"}
	case strings.Contains(host, "wired.com"), strings.Contains(host, "wired.co.uk"):
		rules.skipParagraphContains = []string{"read more:", "do you use social media regularly? take our short survey."}
		rules.endBeforeEquals = []string{"more great wired stories"}
	case strings.Contains(host, "theguardian.com"):
		rules.skipParagraphContains = []string{"photograph:"}
	case strings.Contains(host, "arstechnica.com"):
		rules.skipParagraphContains = []string{"enlarge/", "this story originally appeared on"}
	case strings.Contains(host, "axios.com"):
		rules.skipParagraphContains = []string{
			"sign up for our daily briefing",
			"download for free.",
			"sign up for free.",
			"axios on your phone",
		}
	}
	return rules
}

func paragraphsFromLines(lines []string) [][]string {
	paragraphs := make([][]string, 0, 8)
	current := make([]string, 0, 4)
	for _, line := range lines {
		if strings.TrimSpace(stripANSI(line)) == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, current)
				current = make([]string, 0, 4)
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}
	return paragraphs
}

func normalizeRuleText(s string) string {
	s = strings.ToLower(normalizeInlineText(stripANSI(s)))
	for _, prefix := range []string{"▌", "│", "•", "◦", "▪", "▫", "-", "—", ">", "#"} {
		s = strings.TrimLeft(s, " ")
		s = strings.TrimPrefix(s, prefix)
	}
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func matchesAnyContains(text string, needles []string) bool {
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(needle))) {
			return true
		}
	}
	return false
}

func matchesAnyEquals(text string, candidates []string) bool {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if text == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func styleDetailLinks(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = reHTTPURL.ReplaceAllStringFunc(line, func(m string) string {
			return detailLinkURL.Render(m)
		})
	}
	return out
}

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

func (r htmlArticleRenderer) renderInlineChildren(node *nethtml.Node) string {
	parts := make([]string, 0, 4)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parts = append(parts, r.renderInlineNode(child))
	}
	return strings.Join(parts, " ")
}

func (r htmlArticleRenderer) renderInlineNode(node *nethtml.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type {
	case nethtml.TextNode:
		return node.Data
	case nethtml.ElementNode:
		tag := strings.ToLower(node.Data)
		switch tag {
		case "script", "style", "noscript", "img":
			return ""
		case "br":
			return "\n"
		case "a":
			text := normalizeInlineText(r.renderInlineChildren(node))
			href := strings.TrimSpace(nodeAttr(node, "href"))
			switch {
			case href == "":
				return text
			case text == "":
				return href
			case strings.EqualFold(text, href):
				return href
			default:
				return text + " (" + href + ")"
			}
		case "q":
			text := normalizeInlineText(r.renderInlineChildren(node))
			if text == "" {
				return ""
			}
			return `"` + text + `"`
		case "code", "kbd", "samp":
			text := normalizeInlineText(r.renderInlineChildren(node))
			if text == "" {
				return ""
			}
			return detailCodeStyle.Render("`" + text + "`")
		default:
			return r.renderInlineChildren(node)
		}
	default:
		return ""
	}
}

func renderTableLines(tableNode *nethtml.Node, renderer htmlArticleRenderer) []string {
	rows := tableRows(tableNode)
	if len(rows) == 0 {
		return nil
	}
	lines := make([]string, 0, len(rows)+2)
	for i, row := range rows {
		rowToRender := row
		if i == 0 && rowHasHeader(tableNode) {
			rowToRender = make([]string, len(row))
			for idx := range row {
				rowToRender[idx] = detailTableHeader.Render(row[idx])
			}
		}
		cellLine := detailTableBorder.Render("|") + " " + strings.Join(rowToRender, " "+detailTableBorder.Render("|")+" ") + " " + detailTableBorder.Render("|")
		lines = append(lines, wrapText(cellLine, renderer.width)...)
		if i == 0 && rowHasHeader(tableNode) {
			sep := make([]string, len(row))
			for j := range sep {
				sep[j] = "---"
			}
			sepLine := detailTableBorder.Render("|") + " " + detailTableBorder.Render(strings.Join(sep, " | ")) + " " + detailTableBorder.Render("|")
			lines = append(lines, wrapText(sepLine, renderer.width)...)
		}
	}
	return trimBlankLines(lines)
}

func renderImageLabel(imgNode *nethtml.Node, width int) []string {
	if imgNode == nil {
		return nil
	}
	label := detailImageLabel.Render("◌◌◌ Image")
	alt := normalizeInlineText(nodeAttr(imgNode, "alt"))
	title := normalizeInlineText(nodeAttr(imgNode, "title"))
	text := alt
	if text == "" {
		text = title
	}
	line := label
	if text != "" {
		line += " " + detailImageText.Render(text)
	}
	return wrapText(line, max(1, width))
}

func tableRows(tableNode *nethtml.Node) [][]string {
	rows := make([][]string, 0, 8)
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil {
			return
		}
		if node.Type == nethtml.ElementNode && strings.ToLower(node.Data) == "tr" {
			row := make([]string, 0, 4)
			renderer := htmlArticleRenderer{width: 1000}
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != nethtml.ElementNode {
					continue
				}
				tag := strings.ToLower(c.Data)
				if tag != "th" && tag != "td" {
					continue
				}
				row = append(row, normalizeInlineText(renderer.renderInlineChildren(c)))
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(tableNode)
	return rows
}

func rowHasHeader(tableNode *nethtml.Node) bool {
	for node := tableNode.FirstChild; node != nil; node = node.NextSibling {
		if hasHeaderCell(node) {
			return true
		}
	}
	return hasHeaderCell(tableNode)
}

func hasHeaderCell(node *nethtml.Node) bool {
	if node == nil {
		return false
	}
	if node.Type == nethtml.ElementNode && strings.ToLower(node.Data) == "th" {
		return true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if hasHeaderCell(child) {
			return true
		}
	}
	return false
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

func normalizeInlineText(s string) string {
	s = html.UnescapeString(s)
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Join(strings.Fields(part), " ")
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	normalized := strings.Join(out, "\n")
	replacer := strings.NewReplacer(
		" .", ".",
		" ,", ",",
		" ;", ";",
		" :", ":",
		" !", "!",
		" ?", "?",
		" )", ")",
		"( ", "(",
	)
	return replacer.Replace(normalized)
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

func elementChildren(node *nethtml.Node) []*nethtml.Node {
	children := make([]*nethtml.Node, 0, 4)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.TextNode && strings.TrimSpace(child.Data) == "" {
			continue
		}
		children = append(children, child)
	}
	return children
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

func nodeAttr(node *nethtml.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func collectRawText(node *nethtml.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == nethtml.TextNode {
		return node.Data
	}
	var b strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		b.WriteString(collectRawText(child))
	}
	return b.String()
}

func hasBlockChild(node *nethtml.Node) bool {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode && isBlockElement(child.Data) {
			return true
		}
	}
	return false
}

func findBodyNode(node *nethtml.Node) *nethtml.Node {
	if node == nil {
		return nil
	}
	if node.Type == nethtml.ElementNode && strings.EqualFold(node.Data, "body") {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findBodyNode(child); found != nil {
			return found
		}
	}
	return nil
}

func ImageURLsFromContent(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	reImgSrc := regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']?([^"'\s>]+)`)
	matches := reImgSrc.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(html.UnescapeString(m[1]))
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out
}

func trimBlankLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines) - 1
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end < start {
		return nil
	}
	out := make([]string, 0, end-start+1)
	prevBlank := false
	for i := start; i <= end; i++ {
		blank := strings.TrimSpace(lines[i]) == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, lines[i])
		prevBlank = blank
	}
	return out
}

func wrapText(text string, width int) []string {
	if width < 1 {
		return []string{text}
	}
	paragraphs := strings.Split(text, "\n")
	out := make([]string, 0, len(paragraphs))

	for _, p := range paragraphs {
		if p == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(p)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range words {
			for len(word) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				out = append(out, word[:width])
				word = word[width:]
			}

			if line == "" {
				line = word
				continue
			}
			if len(line)+1+len(word) <= width {
				line += " " + word
				continue
			}
			out = append(out, line)
			line = word
		}
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

func visibleLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func stripANSI(s string) string {
	return reANSICodes.ReplaceAllString(s, "")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
