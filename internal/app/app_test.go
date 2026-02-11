package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

type fakeClient struct {
	entries       []feedbin.Entry
	entriesByIDs  []feedbin.Entry
	subscriptions []feedbin.Subscription
	taggings      []feedbin.Tagging
	unreadIDs     []int64
	starredIDs    []int64
	updatedIDs    []int64
	markUnreadIDs []int64
	markReadIDs   []int64
	starIDs       []int64
	unstarIDs     []int64
	err           error
}

func (f fakeClient) ListEntries(context.Context, int, int) ([]feedbin.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]feedbin.Entry(nil), f.entries...), nil
}

func (f fakeClient) ListEntriesByIDs(context.Context, []int64) ([]feedbin.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]feedbin.Entry(nil), f.entriesByIDs...), nil
}

func (f fakeClient) ListSubscriptions(context.Context) ([]feedbin.Subscription, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]feedbin.Subscription(nil), f.subscriptions...), nil
}

func (f fakeClient) ListUnreadEntryIDs(context.Context) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]int64(nil), f.unreadIDs...), nil
}

func (f fakeClient) ListStarredEntryIDs(context.Context) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]int64(nil), f.starredIDs...), nil
}

func (f fakeClient) ListTaggings(context.Context) ([]feedbin.Tagging, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]feedbin.Tagging(nil), f.taggings...), nil
}

func (f fakeClient) ListUpdatedEntryIDsSince(context.Context, time.Time) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]int64(nil), f.updatedIDs...), nil
}

func (f *fakeClient) MarkEntriesUnread(_ context.Context, entryIDs []int64) error {
	if f.err != nil {
		return f.err
	}
	f.markUnreadIDs = append([]int64(nil), entryIDs...)
	return nil
}

func (f *fakeClient) MarkEntriesRead(_ context.Context, entryIDs []int64) error {
	if f.err != nil {
		return f.err
	}
	f.markReadIDs = append([]int64(nil), entryIDs...)
	return nil
}

func (f *fakeClient) StarEntries(_ context.Context, entryIDs []int64) error {
	if f.err != nil {
		return f.err
	}
	f.starIDs = append([]int64(nil), entryIDs...)
	return nil
}

func (f *fakeClient) UnstarEntries(_ context.Context, entryIDs []int64) error {
	if f.err != nil {
		return f.err
	}
	f.unstarIDs = append([]int64(nil), entryIDs...)
	return nil
}

type fakeRepo struct {
	subs       []feedbin.Subscription
	saved      []feedbin.Entry
	unreadIDs  []int64
	starredIDs []int64
	cached     []feedbin.Entry
	listLimit  int
	searchTerm string
	appState   map[string]string
	saveErr    error
	listErr    error
	setUnread  map[int64]bool
	setStarred map[int64]bool
	syncCursor map[string]time.Time
}

func (f *fakeRepo) SaveSubscriptions(_ context.Context, subscriptions []feedbin.Subscription) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.subs = append([]feedbin.Subscription(nil), subscriptions...)
	return nil
}

func (f *fakeRepo) SaveEntries(_ context.Context, entries []feedbin.Entry) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append([]feedbin.Entry(nil), entries...)
	return nil
}

func (f *fakeRepo) SaveEntryStates(_ context.Context, unreadIDs, starredIDs []int64) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.unreadIDs = append([]int64(nil), unreadIDs...)
	f.starredIDs = append([]int64(nil), starredIDs...)
	return nil
}

func (f *fakeRepo) SetEntryUnread(_ context.Context, entryID int64, unread bool) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.setUnread == nil {
		f.setUnread = make(map[int64]bool)
	}
	f.setUnread[entryID] = unread
	return nil
}

func (f *fakeRepo) SetEntryStarred(_ context.Context, entryID int64, starred bool) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.setStarred == nil {
		f.setStarred = make(map[int64]bool)
	}
	f.setStarred[entryID] = starred
	return nil
}

func (f *fakeRepo) GetSyncCursor(_ context.Context, key string) (time.Time, error) {
	if f.syncCursor == nil {
		return time.Time{}, nil
	}
	return f.syncCursor[key], nil
}

func (f *fakeRepo) SetSyncCursor(_ context.Context, key string, value time.Time) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.syncCursor == nil {
		f.syncCursor = make(map[string]time.Time)
	}
	f.syncCursor[key] = value
	return nil
}

func (f *fakeRepo) GetAppState(_ context.Context, key string) (string, error) {
	if f.appState == nil {
		return "", sql.ErrNoRows
	}
	value, ok := f.appState[key]
	if !ok {
		return "", sql.ErrNoRows
	}
	return value, nil
}

func (f *fakeRepo) SetAppState(_ context.Context, key, value string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.appState == nil {
		f.appState = make(map[string]string)
	}
	f.appState[key] = value
	return nil
}

func (f *fakeRepo) ListEntries(_ context.Context, limit int) ([]feedbin.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.listLimit = limit
	return append([]feedbin.Entry(nil), f.cached...), nil
}

func (f *fakeRepo) ListEntriesByFilter(_ context.Context, _ int, filter string) ([]feedbin.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	switch filter {
	case "unread":
		out := make([]feedbin.Entry, 0, len(f.cached))
		for _, entry := range f.cached {
			if entry.IsUnread {
				out = append(out, entry)
			}
		}
		return out, nil
	case "starred":
		out := make([]feedbin.Entry, 0, len(f.cached))
		for _, entry := range f.cached {
			if entry.IsStarred {
				out = append(out, entry)
			}
		}
		return out, nil
	default:
		return append([]feedbin.Entry(nil), f.cached...), nil
	}
}

func (f *fakeRepo) SearchEntriesByFilter(_ context.Context, _ int, filter, query string) ([]feedbin.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.searchTerm = query
	if strings.TrimSpace(query) == "" {
		return f.ListEntriesByFilter(context.Background(), 0, filter)
	}
	q := strings.ToLower(query)
	out := make([]feedbin.Entry, 0, len(f.cached))
	for _, entry := range f.cached {
		if filter == "unread" && !entry.IsUnread {
			continue
		}
		if filter == "starred" && !entry.IsStarred {
			continue
		}
		haystack := strings.ToLower(entry.Title + " " + entry.Summary + " " + entry.Content + " " + entry.Author + " " + entry.URL + " " + entry.FeedTitle + " " + entry.FeedFolder)
		if strings.Contains(haystack, q) {
			out = append(out, entry)
		}
	}
	return out, nil
}

func TestService_Refresh_SavesMetadataAndStates(t *testing.T) {
	entry := feedbin.Entry{ID: 1, Title: "Hello", FeedID: 10, PublishedAt: time.Now().UTC()}
	client := &fakeClient{
		entries:       []feedbin.Entry{entry},
		subscriptions: []feedbin.Subscription{{ID: 10, Title: "Feed A"}},
		taggings:      []feedbin.Tagging{{FeedID: 10, Name: "Formula 1"}},
		unreadIDs:     []int64{1},
		starredIDs:    []int64{1},
	}
	repo := &fakeRepo{cached: []feedbin.Entry{{ID: 1, Title: "Hello", FeedTitle: "Feed A", IsUnread: true, IsStarred: true}}}

	svc := NewService(client, repo)
	entries, err := svc.Refresh(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if len(repo.subs) != 1 || repo.subs[0].ID != 10 {
		t.Fatalf("subscriptions were not saved to repo: %+v", repo.subs)
	}
	if repo.subs[0].Folder != "Formula 1" {
		t.Fatalf("expected folder from tagging, got %q", repo.subs[0].Folder)
	}
	if len(repo.saved) != 1 || repo.saved[0].ID != 1 {
		t.Fatalf("entries were not saved: %+v", repo.saved)
	}
	if len(repo.unreadIDs) != 1 || repo.unreadIDs[0] != 1 {
		t.Fatalf("unread state not saved: %+v", repo.unreadIDs)
	}
	if len(repo.starredIDs) != 1 || repo.starredIDs[0] != 1 {
		t.Fatalf("starred state not saved: %+v", repo.starredIDs)
	}
	if len(entries) != 1 || entries[0].FeedTitle != "Feed A" {
		t.Fatalf("unexpected returned entries: %+v", entries)
	}
	if repo.syncCursor["updated_entries_since"].IsZero() {
		t.Fatal("expected sync cursor to be persisted")
	}
}

func TestApplyTaggingsToSubscriptions(t *testing.T) {
	subs := []feedbin.Subscription{{ID: 1, Title: "A"}, {ID: 2, Title: "B"}}
	taggings := []feedbin.Tagging{
		{FeedID: 1, Name: "Z"},
		{FeedID: 1, Name: "Formula 1"},
	}
	applyTaggingsToSubscriptions(subs, taggings)
	if subs[0].Folder != "Formula 1" {
		t.Fatalf("expected deterministic smallest tag name, got %q", subs[0].Folder)
	}
	if subs[1].Folder != "" {
		t.Fatalf("expected empty folder for untagged feed, got %q", subs[1].Folder)
	}
}

func TestService_LoadMore_UsesIncrementalUpdatedEntries(t *testing.T) {
	client := &fakeClient{
		entries:      []feedbin.Entry{{ID: 50, Title: "Page 2", FeedID: 10, PublishedAt: time.Now().UTC()}},
		updatedIDs:   []int64{99},
		entriesByIDs: []feedbin.Entry{{ID: 99, Title: "Updated item", FeedID: 10, PublishedAt: time.Now().UTC()}},
		unreadIDs:    []int64{50},
	}
	repo := &fakeRepo{cached: []feedbin.Entry{{ID: 2, Title: "Unread", IsUnread: true}}}
	svc := NewService(client, repo)
	svc.lastStateSyncAt = time.Now().UTC().Add(-1 * time.Minute)

	_, _, err := svc.LoadMore(context.Background(), 2, 50, "all", 100)
	if err != nil {
		t.Fatalf("LoadMore returned error: %v", err)
	}

	foundUpdated := false
	for _, e := range repo.saved {
		if e.ID == 99 {
			foundUpdated = true
		}
	}
	if !foundUpdated {
		t.Fatalf("expected updated entry to be saved, got %+v", repo.saved)
	}
}

func TestService_Refresh_HydratesUnreadEntriesOutsidePage(t *testing.T) {
	client := &fakeClient{
		entries:       []feedbin.Entry{{ID: 10, Title: "Page item", FeedID: 1, PublishedAt: time.Now().UTC()}},
		subscriptions: []feedbin.Subscription{{ID: 1, Title: "Feed"}},
		unreadIDs:     []int64{99},
		entriesByIDs:  []feedbin.Entry{{ID: 99, Title: "Unread missing", FeedID: 1, PublishedAt: time.Now().UTC()}},
	}
	repo := &fakeRepo{cached: []feedbin.Entry{{ID: 10, Title: "Page item"}, {ID: 99, Title: "Unread missing", IsUnread: true}}}
	svc := NewService(client, repo)

	_, err := svc.Refresh(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	foundHydrated := false
	for _, e := range repo.saved {
		if e.ID == 99 {
			foundHydrated = true
			break
		}
	}
	if !foundHydrated {
		t.Fatalf("expected unread entry to be hydrated into cache, saved=%+v", repo.saved)
	}
}

func TestService_Refresh_UsesDefaultCacheLimitForReturnedList(t *testing.T) {
	client := &fakeClient{
		entries:       []feedbin.Entry{{ID: 1, Title: "Page item", FeedID: 1, PublishedAt: time.Now().UTC()}},
		subscriptions: []feedbin.Subscription{{ID: 1, Title: "Feed"}},
	}
	repo := &fakeRepo{cached: []feedbin.Entry{{ID: 1, Title: "Cached"}}}
	svc := NewService(client, repo)

	_, err := svc.Refresh(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if repo.listLimit != DefaultCacheLimit {
		t.Fatalf("expected refresh list limit %d, got %d", DefaultCacheLimit, repo.listLimit)
	}
}

func TestService_Refresh_PropagatesFetchError(t *testing.T) {
	svc := NewService(&fakeClient{err: errors.New("boom")}, &fakeRepo{})

	_, err := svc.Refresh(context.Background(), 1, 20)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestService_ListCached(t *testing.T) {
	repo := &fakeRepo{cached: []feedbin.Entry{{ID: 2, Title: "Cached"}}}
	svc := NewService(&fakeClient{}, repo)

	entries, err := svc.ListCached(context.Background(), 20)
	if err != nil {
		t.Fatalf("ListCached returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != 2 {
		t.Fatalf("unexpected cached entries: %+v", entries)
	}
}

func TestService_ListCachedByFilter(t *testing.T) {
	repo := &fakeRepo{cached: []feedbin.Entry{
		{ID: 1, Title: "All"},
		{ID: 2, Title: "Unread", IsUnread: true},
	}}
	svc := NewService(&fakeClient{}, repo)

	entries, err := svc.ListCachedByFilter(context.Background(), 20, "unread")
	if err != nil {
		t.Fatalf("ListCachedByFilter returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != 2 {
		t.Fatalf("unexpected filtered entries: %+v", entries)
	}
}

func TestService_SearchCached(t *testing.T) {
	repo := &fakeRepo{cached: []feedbin.Entry{
		{ID: 1, Title: "Go release notes"},
		{ID: 2, Title: "Rust update"},
	}}
	svc := NewService(&fakeClient{}, repo)

	entries, err := svc.SearchCached(context.Background(), 20, "all", "go")
	if err != nil {
		t.Fatalf("SearchCached returned error: %v", err)
	}
	if repo.searchTerm != "go" {
		t.Fatalf("expected search term to be forwarded, got %q", repo.searchTerm)
	}
	if len(entries) != 1 || entries[0].ID != 1 {
		t.Fatalf("unexpected search entries: %+v", entries)
	}
}

func TestService_LoadMore_UsesCachedFilterResult(t *testing.T) {
	client := &fakeClient{
		entries:       []feedbin.Entry{{ID: 50, Title: "Page 2", FeedID: 10, PublishedAt: time.Now().UTC()}},
		subscriptions: []feedbin.Subscription{{ID: 10, Title: "Feed A"}},
		unreadIDs:     []int64{50},
	}
	repo := &fakeRepo{cached: []feedbin.Entry{
		{ID: 2, Title: "Unread", IsUnread: true},
		{ID: 1, Title: "Read", IsUnread: false},
	}}
	svc := NewService(client, repo)

	entries, fetchedCount, err := svc.LoadMore(context.Background(), 2, 50, "unread", 100)
	if err != nil {
		t.Fatalf("LoadMore returned error: %v", err)
	}
	if fetchedCount != 1 {
		t.Fatalf("expected fetched count 1, got %d", fetchedCount)
	}
	if len(entries) != 1 || entries[0].ID != 2 {
		t.Fatalf("unexpected LoadMore entries: %+v", entries)
	}
}

func TestService_ToggleUnread(t *testing.T) {
	client := &fakeClient{}
	repo := &fakeRepo{}
	svc := NewService(client, repo)

	next, err := svc.ToggleUnread(context.Background(), 42, true)
	if err != nil {
		t.Fatalf("ToggleUnread returned error: %v", err)
	}
	if next {
		t.Fatal("expected next unread state to be false")
	}
	if len(client.markReadIDs) != 1 || client.markReadIDs[0] != 42 {
		t.Fatalf("expected mark read call, got %+v", client.markReadIDs)
	}
	if v, ok := repo.setUnread[42]; !ok || v {
		t.Fatalf("expected cache unread state false, got %+v", repo.setUnread)
	}
}

func TestService_ToggleStarred(t *testing.T) {
	client := &fakeClient{}
	repo := &fakeRepo{}
	svc := NewService(client, repo)

	next, err := svc.ToggleStarred(context.Background(), 7, false)
	if err != nil {
		t.Fatalf("ToggleStarred returned error: %v", err)
	}
	if !next {
		t.Fatal("expected next starred state to be true")
	}
	if len(client.starIDs) != 1 || client.starIDs[0] != 7 {
		t.Fatalf("expected star call, got %+v", client.starIDs)
	}
	if v, ok := repo.setStarred[7]; !ok || !v {
		t.Fatalf("expected cache starred state true, got %+v", repo.setStarred)
	}
}

func TestService_UIPreferences_DefaultFalseWhenMissing(t *testing.T) {
	svc := NewService(&fakeClient{}, &fakeRepo{})

	prefs, err := svc.LoadUIPreferences(context.Background())
	if err != nil {
		t.Fatalf("LoadUIPreferences returned error: %v", err)
	}
	if prefs.Compact || prefs.MarkReadOnOpen || prefs.ConfirmOpenRead || !prefs.RelativeTime || prefs.ShowNumbers {
		t.Fatalf("expected compact/mark/confirm/showNumbers=false and relative=true by default, got %+v", prefs)
	}
}

func TestService_UIPreferences_SaveAndLoadRoundTrip(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(&fakeClient{}, repo)

	want := UIPreferences{
		Compact:         true,
		MarkReadOnOpen:  true,
		ConfirmOpenRead: true,
		RelativeTime:    false,
		ShowNumbers:     true,
	}
	if err := svc.SaveUIPreferences(context.Background(), want); err != nil {
		t.Fatalf("SaveUIPreferences returned error: %v", err)
	}

	got, err := svc.LoadUIPreferences(context.Background())
	if err != nil {
		t.Fatalf("LoadUIPreferences returned error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected preferences: got %+v want %+v", got, want)
	}
}
