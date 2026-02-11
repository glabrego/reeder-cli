package article

import (
	"strings"

	nethtml "golang.org/x/net/html"
)

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
