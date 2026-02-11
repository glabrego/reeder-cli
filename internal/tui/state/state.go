package state

import (
	"github.com/glabrego/reeder-cli/internal/feedbin"
	tuitree "github.com/glabrego/reeder-cli/internal/tui/tree"
)

func ClampCursor(cursor, size int) int {
	if size <= 0 {
		return 0
	}
	if cursor >= size {
		return size - 1
	}
	if cursor < 0 {
		return 0
	}
	return cursor
}

func PageStep(height int, hasStatus bool) int {
	if height <= 0 {
		return 10
	}
	headerLines := 6
	if hasStatus {
		headerLines += 2
	}
	step := height - headerLines
	if step < 3 {
		step = 3
	}
	return step
}

func CenteredWindow(totalRows, cursor, height int) (int, int) {
	if totalRows <= 0 {
		return 0, 0
	}
	if height <= 0 || totalRows <= height {
		return 0, totalRows
	}
	cursor = ClampCursor(cursor, totalRows)
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	maxStart := totalRows - height
	if start > maxStart {
		start = maxStart
	}
	return start, start + height
}

func ArticleRowsBefore(rows []tuitree.Row, end int) int {
	if end <= 0 || len(rows) == 0 {
		return 0
	}
	if end > len(rows) {
		end = len(rows)
	}
	count := 0
	for i := 0; i < end; i++ {
		if rows[i].Kind == tuitree.RowArticle {
			count++
		}
	}
	return count
}

func VisibleEntryIndices(rows []tuitree.Row) []int {
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.Kind == tuitree.RowArticle {
			out = append(out, row.EntryIndex)
		}
	}
	return out
}

func EntryIndexByID(entries []feedbin.Entry, entryID int64) int {
	for i, entry := range entries {
		if entry.ID == entryID {
			return i
		}
	}
	return -1
}

func TreeCursorForEntry(rows []tuitree.Row, entryIndex int) int {
	for i, row := range rows {
		if row.Kind == tuitree.RowArticle && row.EntryIndex == entryIndex {
			return i
		}
	}
	return -1
}

func SyncedEntryCursor(rows []tuitree.Row, treeCursor int) int {
	if len(rows) == 0 {
		return 0
	}
	treeCursor = ClampCursor(treeCursor, len(rows))
	if rows[treeCursor].Kind == tuitree.RowArticle {
		return rows[treeCursor].EntryIndex
	}
	for i := treeCursor + 1; i < len(rows); i++ {
		if rows[i].Kind == tuitree.RowArticle {
			return rows[i].EntryIndex
		}
	}
	for i := treeCursor - 1; i >= 0; i-- {
		if rows[i].Kind == tuitree.RowArticle {
			return rows[i].EntryIndex
		}
	}
	return 0
}
