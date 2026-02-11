package view

import (
	"strings"

	tuitree "github.com/glabrego/reeder-cli/internal/tui/tree"
)

type ListRenderInput struct {
	Rows                []tuitree.Row
	Start               int
	End                 int
	VisiblePos          int
	TreeCursor          int
	SectionUnreadCounts map[string]int
	FolderUnreadCounts  map[string]int
	FeedUnreadCounts    map[string]int
	CollapsedFolders    map[string]bool
	CollapsedFeeds      map[string]bool

	RenderSectionLine  func(label string, unreadCount int, active bool) string
	RenderTreeNodeLine func(left string, unreadCount int, active bool) string
	RenderEntryLine    func(entryIndex, visiblePos int, active bool) string
	FeedKeyFn          func(folder, feed string) string
}

func RenderListBody(in ListRenderInput) string {
	if len(in.Rows) == 0 || in.Start >= in.End || in.Start < 0 {
		return ""
	}
	var b strings.Builder
	visiblePos := in.VisiblePos
	for i := in.Start; i < in.End; i++ {
		row := in.Rows[i]
		switch row.Kind {
		case tuitree.RowSection:
			b.WriteString(in.RenderSectionLine(row.Label, in.SectionUnreadCounts[row.Label], i == in.TreeCursor))
			b.WriteString("\n")
		case tuitree.RowFolder:
			prefix := "▾ "
			if in.CollapsedFolders[row.Folder] {
				prefix = "▸ "
			}
			b.WriteString(in.RenderTreeNodeLine(prefix+row.Label, in.FolderUnreadCounts[row.Folder], i == in.TreeCursor))
			b.WriteString("\n")
		case tuitree.RowFeed:
			prefix := "  ▾ "
			if row.Folder == "" {
				prefix = "▾ "
			}
			if in.CollapsedFeeds[in.FeedKeyFn(row.Folder, row.Feed)] {
				if row.Folder == "" {
					prefix = "▸ "
				} else {
					prefix = "  ▸ "
				}
			}
			b.WriteString(in.RenderTreeNodeLine(prefix+row.Label, in.FeedUnreadCounts[in.FeedKeyFn(row.Folder, row.Feed)], i == in.TreeCursor))
			b.WriteString("\n")
		case tuitree.RowArticle:
			b.WriteString(in.RenderEntryLine(row.EntryIndex, visiblePos, i == in.TreeCursor))
			b.WriteString("\n")
			visiblePos++
		}
	}
	return b.String()
}
