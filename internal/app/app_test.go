package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type fakeClient struct {
	entries       []feedbin.Entry
	subscriptions []feedbin.Subscription
	unreadIDs     []int64
	starredIDs    []int64
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
	saveErr    error
	listErr    error
	setUnread  map[int64]bool
	setStarred map[int64]bool
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

func (f *fakeRepo) ListEntries(_ context.Context, _ int) ([]feedbin.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
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

func TestService_Refresh_SavesMetadataAndStates(t *testing.T) {
	entry := feedbin.Entry{ID: 1, Title: "Hello", FeedID: 10, PublishedAt: time.Now().UTC()}
	client := &fakeClient{
		entries:       []feedbin.Entry{entry},
		subscriptions: []feedbin.Subscription{{ID: 10, Title: "Feed A"}},
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
	if len(repo.saved) != 1 || repo.saved[0].FeedTitle != "Feed A" || !repo.saved[0].IsUnread || !repo.saved[0].IsStarred {
		t.Fatalf("entries were not enriched/saved: %+v", repo.saved)
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

	entries, err := svc.LoadMore(context.Background(), 2, 50, "unread", 100)
	if err != nil {
		t.Fatalf("LoadMore returned error: %v", err)
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
