package state

import (
	"reflect"
	"testing"
	"time"

	"github.com/glabrego/reeder-cli/internal/feedbin"
	tuitree "github.com/glabrego/reeder-cli/internal/tui/tree"
)

func TestClampCursor(t *testing.T) {
	if got := ClampCursor(-1, 3); got != 0 {
		t.Fatalf("expected clamp to 0, got %d", got)
	}
	if got := ClampCursor(3, 3); got != 2 {
		t.Fatalf("expected clamp to 2, got %d", got)
	}
	if got := ClampCursor(1, 3); got != 1 {
		t.Fatalf("expected keep 1, got %d", got)
	}
}

func TestPageStep(t *testing.T) {
	if got := PageStep(0, false); got != 10 {
		t.Fatalf("expected default step 10, got %d", got)
	}
	if got := PageStep(12, false); got != 6 {
		t.Fatalf("expected step 6, got %d", got)
	}
	if got := PageStep(12, true); got != 4 {
		t.Fatalf("expected step 4 with status, got %d", got)
	}
}

func TestCenteredWindowAndVisibleCounts(t *testing.T) {
	rows := []tuitree.Row{
		{Kind: tuitree.RowSection},
		{Kind: tuitree.RowArticle, EntryIndex: 0},
		{Kind: tuitree.RowArticle, EntryIndex: 1},
		{Kind: tuitree.RowFeed},
		{Kind: tuitree.RowArticle, EntryIndex: 2},
	}
	start, end := CenteredWindow(len(rows), 3, 3)
	if start != 2 || end != 5 {
		t.Fatalf("unexpected window: start=%d end=%d", start, end)
	}
	if got := ArticleRowsBefore(rows, start); got != 1 {
		t.Fatalf("expected 1 article row before start, got %d", got)
	}
	visible := VisibleEntryIndices(rows)
	if !reflect.DeepEqual(visible, []int{0, 1, 2}) {
		t.Fatalf("unexpected visible entry indices: %v", visible)
	}
}

func TestSelectionHelpers(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 10, PublishedAt: time.Now().UTC()},
		{ID: 20, PublishedAt: time.Now().UTC()},
	}
	if got := EntryIndexByID(entries, 20); got != 1 {
		t.Fatalf("expected entry index 1, got %d", got)
	}

	rows := []tuitree.Row{
		{Kind: tuitree.RowSection, Label: "Folders"},
		{Kind: tuitree.RowFeed, Label: "Feed"},
		{Kind: tuitree.RowArticle, EntryIndex: 1},
	}
	if got := TreeCursorForEntry(rows, 1); got != 2 {
		t.Fatalf("expected tree cursor 2, got %d", got)
	}
	if got := SyncedEntryCursor(rows, 0); got != 1 {
		t.Fatalf("expected synced entry cursor 1, got %d", got)
	}
}
