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

type fakeRepo struct {
	subs       []feedbin.Subscription
	saved      []feedbin.Entry
	unreadIDs  []int64
	starredIDs []int64
	cached     []feedbin.Entry
	saveErr    error
	listErr    error
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

func (f *fakeRepo) ListEntries(_ context.Context, _ int) ([]feedbin.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]feedbin.Entry(nil), f.cached...), nil
}

func TestService_Refresh_SavesMetadataAndStates(t *testing.T) {
	entry := feedbin.Entry{ID: 1, Title: "Hello", FeedID: 10, PublishedAt: time.Now().UTC()}
	client := fakeClient{
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
	svc := NewService(fakeClient{err: errors.New("boom")}, &fakeRepo{})

	_, err := svc.Refresh(context.Background(), 1, 20)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestService_ListCached(t *testing.T) {
	repo := &fakeRepo{cached: []feedbin.Entry{{ID: 2, Title: "Cached"}}}
	svc := NewService(fakeClient{}, repo)

	entries, err := svc.ListCached(context.Background(), 20)
	if err != nil {
		t.Fatalf("ListCached returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != 2 {
		t.Fatalf("unexpected cached entries: %+v", entries)
	}
}
