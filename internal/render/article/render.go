package article

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	nethtml "golang.org/x/net/html"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var reANSICodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var reHTTPURL = regexp.MustCompile(`https?://[^\s)]+`)

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
