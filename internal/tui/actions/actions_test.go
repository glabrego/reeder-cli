package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

type fakeService struct {
	refreshEntries []feedbin.Entry
	refreshErr     error

	filterEntries []feedbin.Entry
	filterErr     error

	searchEntries []feedbin.Entry
	searchErr     error

	loadMoreEntries []feedbin.Entry
	loadMoreFetched int
	loadMoreErr     error

	toggleUnreadNext bool
	toggleUnreadErr  error

	toggleStarredNext bool
	toggleStarredErr  error

	lastRefreshDeadline     time.Time
	lastFilterDeadline      time.Time
	lastSearchDeadline      time.Time
	lastLoadMoreDeadline    time.Time
	lastToggleUnreadDL      time.Time
	lastToggleStarredDL     time.Time
	lastFilter              string
	lastSearchFilter        string
	lastSearchQuery         string
	lastLoadMoreFilter      string
	lastToggleUnreadEntryID int64
}

func (f *fakeService) Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error) {
	if dl, ok := ctx.Deadline(); ok {
		f.lastRefreshDeadline = dl
	}
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	return f.refreshEntries, nil
}

func (f *fakeService) ListCachedByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error) {
	if dl, ok := ctx.Deadline(); ok {
		f.lastFilterDeadline = dl
	}
	f.lastFilter = filter
	if f.filterErr != nil {
		return nil, f.filterErr
	}
	return f.filterEntries, nil
}

func (f *fakeService) SearchCached(ctx context.Context, limit int, filter, query string) ([]feedbin.Entry, error) {
	if dl, ok := ctx.Deadline(); ok {
		f.lastSearchDeadline = dl
	}
	f.lastSearchFilter = filter
	f.lastSearchQuery = query
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.searchEntries, nil
}

func (f *fakeService) LoadMore(ctx context.Context, page, perPage int, filter string, limit int) ([]feedbin.Entry, int, error) {
	if dl, ok := ctx.Deadline(); ok {
		f.lastLoadMoreDeadline = dl
	}
	f.lastLoadMoreFilter = filter
	if f.loadMoreErr != nil {
		return nil, 0, f.loadMoreErr
	}
	return f.loadMoreEntries, f.loadMoreFetched, nil
}

func (f *fakeService) ToggleUnread(ctx context.Context, entryID int64, currentUnread bool) (bool, error) {
	if dl, ok := ctx.Deadline(); ok {
		f.lastToggleUnreadDL = dl
	}
	f.lastToggleUnreadEntryID = entryID
	if f.toggleUnreadErr != nil {
		return false, f.toggleUnreadErr
	}
	return f.toggleUnreadNext, nil
}

func (f *fakeService) ToggleStarred(ctx context.Context, entryID int64, currentStarred bool) (bool, error) {
	if dl, ok := ctx.Deadline(); ok {
		f.lastToggleStarredDL = dl
	}
	if f.toggleStarredErr != nil {
		return false, f.toggleStarredErr
	}
	return f.toggleStarredNext, nil
}

func TestRefreshCmd(t *testing.T) {
	svc := &fakeService{refreshEntries: []feedbin.Entry{{ID: 1}}}
	msg := RefreshCmd(svc, 20, "manual")()
	success, ok := msg.(RefreshSuccessMsg)
	if !ok {
		t.Fatalf("expected RefreshSuccessMsg, got %T", msg)
	}
	if success.Source != "manual" || len(success.Entries) != 1 {
		t.Fatalf("unexpected success payload: %+v", success)
	}
	if svc.lastRefreshDeadline.IsZero() {
		t.Fatal("expected refresh context deadline to be set")
	}
}

func TestLoadFilterAndSearchCmd(t *testing.T) {
	svc := &fakeService{
		filterEntries: []feedbin.Entry{{ID: 10}},
		searchEntries: []feedbin.Entry{{ID: 20}},
	}

	msg := LoadFilterCmd(svc, "unread", 100)()
	filterMsg, ok := msg.(FilterLoadSuccessMsg)
	if !ok {
		t.Fatalf("expected FilterLoadSuccessMsg, got %T", msg)
	}
	if filterMsg.Filter != "unread" || svc.lastFilter != "unread" {
		t.Fatalf("unexpected filter payload: %+v", filterMsg)
	}

	msg = LoadSearchCmd(svc, "starred", "go", 100)()
	searchMsg, ok := msg.(SearchLoadSuccessMsg)
	if !ok {
		t.Fatalf("expected SearchLoadSuccessMsg, got %T", msg)
	}
	if searchMsg.Filter != "starred" || searchMsg.Query != "go" {
		t.Fatalf("unexpected search payload: %+v", searchMsg)
	}
	if svc.lastSearchFilter != "starred" || svc.lastSearchQuery != "go" {
		t.Fatalf("unexpected search args captured by service: filter=%s query=%s", svc.lastSearchFilter, svc.lastSearchQuery)
	}
}

func TestLoadMoreAndToggleCmds(t *testing.T) {
	svc := &fakeService{
		loadMoreEntries:   []feedbin.Entry{{ID: 33}},
		loadMoreFetched:   1,
		toggleUnreadNext:  true,
		toggleStarredNext: true,
	}

	msg := LoadMoreCmd(svc, 2, 20, "all", 40)()
	loadMsg, ok := msg.(LoadMoreSuccessMsg)
	if !ok {
		t.Fatalf("expected LoadMoreSuccessMsg, got %T", msg)
	}
	if loadMsg.Page != 2 || loadMsg.FetchedCount != 1 {
		t.Fatalf("unexpected load more payload: %+v", loadMsg)
	}

	msg = ToggleUnreadCmd(svc, 99, false)()
	unreadMsg, ok := msg.(ToggleUnreadSuccessMsg)
	if !ok {
		t.Fatalf("expected ToggleUnreadSuccessMsg, got %T", msg)
	}
	if unreadMsg.EntryID != 99 || unreadMsg.Status != "Marked as unread" {
		t.Fatalf("unexpected toggle unread payload: %+v", unreadMsg)
	}

	msg = ToggleStarredCmd(svc, 99, false)()
	starMsg, ok := msg.(ToggleStarredSuccessMsg)
	if !ok {
		t.Fatalf("expected ToggleStarredSuccessMsg, got %T", msg)
	}
	if starMsg.Status != "Starred entry" {
		t.Fatalf("unexpected toggle starred payload: %+v", starMsg)
	}
}

func TestActionErrors(t *testing.T) {
	svc := &fakeService{
		refreshErr:       errors.New("refresh failed"),
		filterErr:        errors.New("filter failed"),
		searchErr:        errors.New("search failed"),
		loadMoreErr:      errors.New("load failed"),
		toggleUnreadErr:  errors.New("toggle unread failed"),
		toggleStarredErr: errors.New("toggle starred failed"),
	}

	if _, ok := RefreshCmd(svc, 20, "manual")().(RefreshErrorMsg); !ok {
		t.Fatal("expected RefreshErrorMsg")
	}
	if _, ok := LoadFilterCmd(svc, "all", 10)().(FilterLoadErrorMsg); !ok {
		t.Fatal("expected FilterLoadErrorMsg")
	}
	if _, ok := LoadSearchCmd(svc, "all", "go", 10)().(SearchLoadErrorMsg); !ok {
		t.Fatal("expected SearchLoadErrorMsg")
	}
	if _, ok := LoadMoreCmd(svc, 2, 20, "all", 20)().(LoadMoreErrorMsg); !ok {
		t.Fatal("expected LoadMoreErrorMsg")
	}
	if _, ok := ToggleUnreadCmd(svc, 1, false)().(ToggleActionErrorMsg); !ok {
		t.Fatal("expected ToggleActionErrorMsg for unread")
	}
	if _, ok := ToggleStarredCmd(svc, 1, false)().(ToggleActionErrorMsg); !ok {
		t.Fatal("expected ToggleActionErrorMsg for starred")
	}
}

func TestOpenURLCmd_Fallbacks(t *testing.T) {
	msg := OpenURLCmd(1, true, "https://example.com",
		func(string) error { return nil },
		func(string) error { return nil },
	)()
	success, ok := msg.(OpenURLSuccessMsg)
	if !ok || !success.Opened {
		t.Fatalf("expected opened success, got %T %+v", msg, success)
	}

	msg = OpenURLCmd(1, true, "https://example.com",
		func(string) error { return errors.New("open failed") },
		func(string) error { return nil },
	)()
	success, ok = msg.(OpenURLSuccessMsg)
	if !ok || success.Opened {
		t.Fatalf("expected copy fallback success, got %T %+v", msg, success)
	}

	msg = OpenURLCmd(1, true, "https://example.com",
		func(string) error { return errors.New("open failed") },
		func(string) error { return errors.New("copy failed") },
	)()
	if _, ok := msg.(OpenURLErrorMsg); !ok {
		t.Fatalf("expected OpenURLErrorMsg, got %T", msg)
	}
}

func TestCopyURLCmd(t *testing.T) {
	msg := CopyURLCmd("https://example.com", func(string) error { return nil })()
	if _, ok := msg.(OpenURLSuccessMsg); !ok {
		t.Fatalf("expected OpenURLSuccessMsg, got %T", msg)
	}
	msg = CopyURLCmd("https://example.com", func(string) error { return errors.New("copy failed") })()
	if _, ok := msg.(OpenURLErrorMsg); !ok {
		t.Fatalf("expected OpenURLErrorMsg, got %T", msg)
	}
}
