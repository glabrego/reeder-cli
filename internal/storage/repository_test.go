package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

func TestRepository_SaveAndListEntries(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feedbin.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := context.Background()
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	subs := []feedbin.Subscription{{ID: 10, Title: "Feed A", FeedURL: "https://example.com/feed.xml"}}
	if err := repo.SaveSubscriptions(ctx, subs); err != nil {
		t.Fatalf("SaveSubscriptions returned error: %v", err)
	}

	entries := []feedbin.Entry{
		{
			ID:          1,
			Title:       "Older",
			URL:         "https://example.com/old",
			FeedID:      10,
			PublishedAt: time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
			IsUnread:    true,
		},
		{
			ID:          2,
			Title:       "Newer",
			URL:         "https://example.com/new",
			FeedID:      10,
			PublishedAt: time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC),
			IsStarred:   true,
		},
	}

	if err := repo.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("SaveEntries returned error: %v", err)
	}

	listed, err := repo.ListEntries(ctx, 10)
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	if len(listed) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(listed))
	}
	if listed[0].ID != 2 {
		t.Fatalf("expected newest first, got id=%d", listed[0].ID)
	}
	if listed[0].FeedTitle != "Feed A" {
		t.Fatalf("expected feed title from subscription, got %q", listed[0].FeedTitle)
	}
	if !listed[0].IsStarred {
		t.Fatal("expected starred state persisted")
	}
}

func TestRepository_SaveEntries_Upserts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feedbin.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := context.Background()
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	entry := feedbin.Entry{
		ID:          10,
		Title:       "Original",
		URL:         "https://example.com/10",
		FeedID:      99,
		PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := repo.SaveEntries(ctx, []feedbin.Entry{entry}); err != nil {
		t.Fatalf("initial SaveEntries returned error: %v", err)
	}

	entry.Title = "Updated"
	entry.IsUnread = true
	if err := repo.SaveEntries(ctx, []feedbin.Entry{entry}); err != nil {
		t.Fatalf("second SaveEntries returned error: %v", err)
	}

	listed, err := repo.ListEntries(ctx, 1)
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(listed))
	}
	if listed[0].Title != "Updated" {
		t.Fatalf("expected updated title, got %q", listed[0].Title)
	}
	if !listed[0].IsUnread {
		t.Fatal("expected unread flag to be updated")
	}
}

func TestRepository_SaveEntryStates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feedbin.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := context.Background()
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	entries := []feedbin.Entry{
		{ID: 1, Title: "One", URL: "https://example.com/1", FeedID: 1, PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", URL: "https://example.com/2", FeedID: 1, PublishedAt: time.Now().UTC()},
	}
	if err := repo.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("SaveEntries returned error: %v", err)
	}

	if err := repo.SaveEntryStates(ctx, []int64{2}, []int64{1}); err != nil {
		t.Fatalf("SaveEntryStates returned error: %v", err)
	}

	listed, err := repo.ListEntries(ctx, 10)
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	var entry1, entry2 feedbin.Entry
	for _, entry := range listed {
		if entry.ID == 1 {
			entry1 = entry
		}
		if entry.ID == 2 {
			entry2 = entry
		}
	}

	if !entry1.IsStarred || entry1.IsUnread {
		t.Fatalf("unexpected state for entry 1: unread=%v starred=%v", entry1.IsUnread, entry1.IsStarred)
	}
	if !entry2.IsUnread || entry2.IsStarred {
		t.Fatalf("unexpected state for entry 2: unread=%v starred=%v", entry2.IsUnread, entry2.IsStarred)
	}
}

func TestRepository_SetEntryUnreadAndStarred(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feedbin.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := context.Background()
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	entry := feedbin.Entry{
		ID:          99,
		Title:       "Entry",
		URL:         "https://example.com/entry",
		FeedID:      1,
		PublishedAt: time.Now().UTC(),
	}
	if err := repo.SaveEntries(ctx, []feedbin.Entry{entry}); err != nil {
		t.Fatalf("SaveEntries returned error: %v", err)
	}

	if err := repo.SetEntryUnread(ctx, 99, true); err != nil {
		t.Fatalf("SetEntryUnread returned error: %v", err)
	}
	if err := repo.SetEntryStarred(ctx, 99, true); err != nil {
		t.Fatalf("SetEntryStarred returned error: %v", err)
	}

	listed, err := repo.ListEntries(ctx, 1)
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(listed))
	}
	if !listed[0].IsUnread || !listed[0].IsStarred {
		t.Fatalf("expected both flags true, got unread=%v starred=%v", listed[0].IsUnread, listed[0].IsStarred)
	}
}

func TestRepository_ListEntriesByFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feedbin.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := context.Background()
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	entries := []feedbin.Entry{
		{ID: 1, Title: "All", URL: "https://example.com/1", FeedID: 1, PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Unread", URL: "https://example.com/2", FeedID: 1, PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC), IsUnread: true},
		{ID: 3, Title: "Starred", URL: "https://example.com/3", FeedID: 1, PublishedAt: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC), IsStarred: true},
	}
	if err := repo.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("SaveEntries returned error: %v", err)
	}

	unread, err := repo.ListEntriesByFilter(ctx, 20, "unread")
	if err != nil {
		t.Fatalf("ListEntriesByFilter unread returned error: %v", err)
	}
	if len(unread) != 1 || unread[0].ID != 2 {
		t.Fatalf("unexpected unread entries: %+v", unread)
	}

	starred, err := repo.ListEntriesByFilter(ctx, 20, "starred")
	if err != nil {
		t.Fatalf("ListEntriesByFilter starred returned error: %v", err)
	}
	if len(starred) != 1 || starred[0].ID != 3 {
		t.Fatalf("unexpected starred entries: %+v", starred)
	}
}
