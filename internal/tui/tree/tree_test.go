package tree

import (
	"reflect"
	"testing"
	"time"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestSortEntries_GroupsByCollectionAndFeed(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, FeedFolder: "Folder B", FeedTitle: "Z Feed", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, FeedFolder: "Folder A", FeedTitle: "A Feed", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 3, FeedFolder: "Folder A", FeedTitle: "A Feed", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
		{ID: 4, FeedTitle: "Top Feed", PublishedAt: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)},
	}
	SortEntries(entries)
	got := []int64{entries[0].ID, entries[1].ID, entries[2].ID, entries[3].ID}
	want := []int64{3, 2, 1, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected order: got=%v want=%v", got, want)
	}
}

func TestBuildRows_SectionsAndCollapsedState(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Folder 1", FeedFolder: "Formula 1", FeedTitle: "Race", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Top 1", FeedTitle: "Top Feed", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	rows := BuildRows(entries, BuildOptions{
		CollapsedFolders:  map[string]bool{"Formula 1": true},
		CollapsedFeeds:    map[string]bool{FeedKey("", "Top Feed"): true},
		CollapsedSections: map[string]bool{},
	})
	if len(rows) < 4 {
		t.Fatalf("expected section + collection rows, got %+v", rows)
	}
	if rows[0].Kind != RowSection || rows[0].Label != "Folders" {
		t.Fatalf("expected Folders section at top, got %+v", rows[0])
	}
	if rows[1].Kind != RowFolder || rows[1].Folder != "Formula 1" {
		t.Fatalf("expected Formula 1 folder row, got %+v", rows[1])
	}
	for _, row := range rows {
		if row.Kind == RowArticle {
			t.Fatalf("expected no article rows when folder/top-feed are collapsed, got %+v", rows)
		}
	}
}

func TestBuildRows_CompactModeSortedByDateThenTitle(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Bravo", FeedTitle: "Feed", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Alpha", FeedTitle: "Feed", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
		{ID: 3, Title: "Newest", FeedTitle: "Feed", PublishedAt: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)},
	}
	rows := BuildRows(entries, BuildOptions{
		Compact:           true,
		CollapsedFolders:  map[string]bool{},
		CollapsedFeeds:    map[string]bool{},
		CollapsedSections: map[string]bool{},
	})
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	got := []int{rows[0].EntryIndex, rows[1].EntryIndex, rows[2].EntryIndex}
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected compact order: got=%v want=%v", got, want)
	}
}

func TestFirstArticleRow(t *testing.T) {
	rows := []Row{
		{Kind: RowSection, Label: "Folders"},
		{Kind: RowFolder, Folder: "Formula 1"},
		{Kind: RowArticle, EntryIndex: 7},
	}
	if got := FirstArticleRow(rows); got != 2 {
		t.Fatalf("expected first article row at index 2, got %d", got)
	}
}
