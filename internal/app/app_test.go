package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type fakeClient struct {
	entries []feedbin.Entry
	err     error
}

func (f fakeClient) ListEntries(context.Context, int, int) ([]feedbin.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entries, nil
}

type fakeRepo struct {
	saved   []feedbin.Entry
	cached  []feedbin.Entry
	saveErr error
	listErr error
}

func (f *fakeRepo) SaveEntries(_ context.Context, entries []feedbin.Entry) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append([]feedbin.Entry(nil), entries...)
	return nil
}

func (f *fakeRepo) ListEntries(_ context.Context, _ int) ([]feedbin.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.cached, nil
}

func TestService_Refresh_SavesFetchedEntries(t *testing.T) {
	entry := feedbin.Entry{ID: 1, Title: "Hello", PublishedAt: time.Now().UTC()}
	client := fakeClient{entries: []feedbin.Entry{entry}}
	repo := &fakeRepo{}

	svc := NewService(client, repo)
	entries, err := svc.Refresh(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	if len(entries) != 1 || entries[0].ID != 1 {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if len(repo.saved) != 1 || repo.saved[0].ID != 1 {
		t.Fatalf("entries were not saved to repo: %+v", repo.saved)
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
