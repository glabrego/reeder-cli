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

	entries := []feedbin.Entry{
		{
			ID:          1,
			Title:       "Older",
			URL:         "https://example.com/old",
			FeedID:      10,
			PublishedAt: time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:          2,
			Title:       "Newer",
			URL:         "https://example.com/new",
			FeedID:      10,
			PublishedAt: time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC),
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
}
