package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
	"github.com/glabrego/feedbin-cli/internal/storage"
)

func TestIntegration_RefreshToggleAndLoadMore(t *testing.T) {
	if os.Getenv("FEEDBIN_INTEGRATION") != "1" {
		t.Skip("set FEEDBIN_INTEGRATION=1 to run integration tests")
	}

	email := os.Getenv("FEEDBIN_EMAIL")
	password := os.Getenv("FEEDBIN_PASSWORD")
	if email == "" || password == "" {
		t.Skip("FEEDBIN_EMAIL and FEEDBIN_PASSWORD are required")
	}

	baseURL := os.Getenv("FEEDBIN_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.feedbin.com/v2"
	}

	repo, err := storage.NewRepository(filepath.Join(t.TempDir(), "feedbin-integration.db"))
	if err != nil {
		t.Fatalf("NewRepository returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	client := feedbin.NewClient(baseURL, email, password, nil)
	svc := NewService(client, repo)

	initial, err := svc.Refresh(ctx, 1, 30)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if len(initial) == 0 {
		t.Fatal("expected at least one entry from refresh")
	}

	entry := initial[0]

	// Keep the account state stable by restoring toggled flags before test exits.
	currentUnread := entry.IsUnread
	currentStarred := entry.IsStarred
	defer func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer restoreCancel()
		if currentUnread != entry.IsUnread {
			_, _ = svc.ToggleUnread(restoreCtx, entry.ID, currentUnread)
		}
		if currentStarred != entry.IsStarred {
			_, _ = svc.ToggleStarred(restoreCtx, entry.ID, currentStarred)
		}
	}()

	nextUnread, err := svc.ToggleUnread(ctx, entry.ID, currentUnread)
	if err != nil {
		t.Fatalf("ToggleUnread returned error: %v", err)
	}
	if nextUnread == currentUnread {
		t.Fatalf("expected unread state to change from %v", currentUnread)
	}
	currentUnread = nextUnread

	nextStarred, err := svc.ToggleStarred(ctx, entry.ID, currentStarred)
	if err != nil {
		t.Fatalf("ToggleStarred returned error: %v", err)
	}
	if nextStarred == currentStarred {
		t.Fatalf("expected starred state to change from %v", currentStarred)
	}
	currentStarred = nextStarred

	more, fetchedCount, err := svc.LoadMore(ctx, 2, 30, "all", 60)
	if err != nil {
		t.Fatalf("LoadMore returned error: %v", err)
	}
	if fetchedCount < 0 {
		t.Fatalf("expected non-negative fetched count, got %d", fetchedCount)
	}
	if len(more) < len(initial) {
		t.Fatalf("expected load more size >= initial size, got %d < %d", len(more), len(initial))
	}

	unreadOnly, err := svc.ListCachedByFilter(ctx, 100, "unread")
	if err != nil {
		t.Fatalf("ListCachedByFilter unread returned error: %v", err)
	}
	for _, e := range unreadOnly {
		if !e.IsUnread {
			t.Fatalf("found non-unread entry in unread filter: %+v", e)
		}
	}
}

func TestIntegration_SearchCachedWithFilterAndClear(t *testing.T) {
	if os.Getenv("FEEDBIN_INTEGRATION") != "1" {
		t.Skip("set FEEDBIN_INTEGRATION=1 to run integration tests")
	}

	email := os.Getenv("FEEDBIN_EMAIL")
	password := os.Getenv("FEEDBIN_PASSWORD")
	if email == "" || password == "" {
		t.Skip("FEEDBIN_EMAIL and FEEDBIN_PASSWORD are required")
	}

	baseURL := os.Getenv("FEEDBIN_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.feedbin.com/v2"
	}

	repo, err := storage.NewRepositoryWithSearch(filepath.Join(t.TempDir(), "feedbin-integration-search.db"), "like")
	if err != nil {
		t.Fatalf("NewRepositoryWithSearch returned error: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := repo.Init(ctx); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	client := feedbin.NewClient(baseURL, email, password, nil)
	svc := NewService(client, repo)

	initial, err := svc.Refresh(ctx, 1, 30)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if len(initial) == 0 {
		t.Fatal("expected at least one entry from refresh")
	}

	_, _, err = svc.LoadMore(ctx, 2, 30, "all", 60)
	if err != nil {
		t.Fatalf("LoadMore returned error: %v", err)
	}

	unread, err := svc.ListCachedByFilter(ctx, 300, "unread")
	if err != nil {
		t.Fatalf("ListCachedByFilter unread returned error: %v", err)
	}
	if len(unread) == 0 {
		t.Skip("no unread entries available to validate search integration")
	}

	token := searchTokenFromTitle(unread[0].Title)
	if token == "" {
		t.Skip("could not derive a stable search token from unread title")
	}

	matchedUnread, err := svc.SearchCached(ctx, 300, "unread", token)
	if err != nil {
		t.Fatalf("SearchCached unread returned error: %v", err)
	}
	if len(matchedUnread) == 0 {
		t.Fatalf("expected at least one unread search match for token %q", token)
	}
	for _, e := range matchedUnread {
		if !e.IsUnread {
			t.Fatalf("found non-unread entry in unread search result: %+v", e)
		}
	}

	allFiltered, err := svc.ListCachedByFilter(ctx, 300, "unread")
	if err != nil {
		t.Fatalf("ListCachedByFilter unread returned error: %v", err)
	}
	cleared, err := svc.SearchCached(ctx, 300, "unread", "")
	if err != nil {
		t.Fatalf("SearchCached clear returned error: %v", err)
	}
	if len(cleared) != len(allFiltered) {
		t.Fatalf("expected cleared search size %d, got %d", len(allFiltered), len(cleared))
	}
}

func searchTokenFromTitle(title string) string {
	for _, part := range strings.Fields(strings.ToLower(title)) {
		var b strings.Builder
		for _, r := range part {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			}
		}
		token := b.String()
		if len(token) >= 4 {
			return token
		}
	}
	return ""
}
