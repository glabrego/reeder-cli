package article

import (
	"html"
	"strings"

	nethtml "golang.org/x/net/html"
)

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
