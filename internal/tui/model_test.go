package tui

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type fakeRefresher struct {
	entries      []feedbin.Entry
	err          error
	unreadResult bool
	starResult   bool
	pageResults  map[int][]feedbin.Entry
}

func (f fakeRefresher) Refresh(context.Context, int, int) ([]feedbin.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entries, nil
}

func (f fakeRefresher) ListCachedByFilter(_ context.Context, _ int, filter string) ([]feedbin.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	switch filter {
	case "unread":
		out := make([]feedbin.Entry, 0, len(f.entries))
		for _, entry := range f.entries {
			if entry.IsUnread {
				out = append(out, entry)
			}
		}
		return out, nil
	case "starred":
		out := make([]feedbin.Entry, 0, len(f.entries))
		for _, entry := range f.entries {
			if entry.IsStarred {
				out = append(out, entry)
			}
		}
		return out, nil
	default:
		return f.entries, nil
	}
}

func (f fakeRefresher) LoadMore(_ context.Context, page, _ int, _ string, _ int) ([]feedbin.Entry, int, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	if f.pageResults != nil {
		if entries, ok := f.pageResults[page]; ok {
			return entries, len(entries), nil
		}
	}
	return f.entries, len(f.entries), nil
}

func (f fakeRefresher) ToggleUnread(context.Context, int64, bool) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.unreadResult, nil
}

func (f fakeRefresher) ToggleStarred(context.Context, int64, bool) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.starResult, nil
}

type openWorkflowService struct {
	unreadResult bool
	unreadCalls  int
}

func (s *openWorkflowService) Refresh(context.Context, int, int) ([]feedbin.Entry, error) {
	return nil, nil
}

func (s *openWorkflowService) ListCachedByFilter(context.Context, int, string) ([]feedbin.Entry, error) {
	return nil, nil
}

func (s *openWorkflowService) LoadMore(context.Context, int, int, string, int) ([]feedbin.Entry, int, error) {
	return nil, 0, nil
}

func (s *openWorkflowService) ToggleUnread(context.Context, int64, bool) (bool, error) {
	s.unreadCalls++
	return s.unreadResult, nil
}

func (s *openWorkflowService) ToggleStarred(context.Context, int64, bool) (bool, error) {
	return false, nil
}

type initRefreshService struct {
	called  bool
	page    int
	perPage int
}

func (s *initRefreshService) Refresh(_ context.Context, page, perPage int) ([]feedbin.Entry, error) {
	s.called = true
	s.page = page
	s.perPage = perPage
	return []feedbin.Entry{{ID: 100, Title: "From refresh", PublishedAt: time.Now().UTC()}}, nil
}

func (s *initRefreshService) ListCachedByFilter(context.Context, int, string) ([]feedbin.Entry, error) {
	return nil, nil
}

func (s *initRefreshService) LoadMore(context.Context, int, int, string, int) ([]feedbin.Entry, int, error) {
	return nil, 0, nil
}

func (s *initRefreshService) ToggleUnread(context.Context, int64, bool) (bool, error) {
	return false, nil
}

func (s *initRefreshService) ToggleStarred(context.Context, int64, bool) (bool, error) {
	return false, nil
}

func TestModelView_ShowsEntriesWithMetadata(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "First Entry",
		FeedTitle:   "Feed A",
		FeedFolder:  "Formula 1",
		URL:         "https://example.com/1",
		IsUnread:    true,
		IsStarred:   true,
		PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}})

	view := m.View()
	if !strings.Contains(view, "First Entry") {
		t.Fatalf("expected entry title in view, got: %s", view)
	}
	if !strings.Contains(view, "Feed A") {
		t.Fatalf("expected feed title in view, got: %s", view)
	}
	if !strings.Contains(view, "▾ Formula 1") {
		t.Fatalf("expected folder grouping header in view, got: %s", view)
	}
	if !strings.Contains(view, "  ▾ Feed A") {
		t.Fatalf("expected feed grouping header in view, got: %s", view)
	}
	if !strings.Contains(view, "\x1b[1;3mFirst Entry\x1b[0m") {
		t.Fatalf("expected bold italic styled title for unread+starred entry, got: %s", view)
	}
	if !strings.Contains(view, "> ") {
		t.Fatalf("expected cursor marker in view, got: %s", view)
	}
	if !strings.Contains(view, "\x1b[7m") {
		t.Fatalf("expected active row highlight in view, got: %q", view)
	}
	if !strings.Contains(view, "Mode: list | Filter: all | Page: 1 | Showing: 1 | Last fetch: 0 | Time: relative | Open->Read: off | Confirm: off") {
		t.Fatalf("expected footer in list view, got: %s", view)
	}
}

func TestSortEntriesForTree_GroupsByFolderThenFeed(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, FeedTitle: "Z Feed", FeedFolder: "Folder B", URL: "https://b.example.com/a", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, FeedTitle: "A Feed", FeedFolder: "Folder A", URL: "https://a.example.com/a", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 3, FeedTitle: "A Feed", FeedFolder: "Folder A", URL: "https://a.example.com/b", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
	}

	sortEntriesForTree(entries)

	if entries[0].ID != 3 || entries[1].ID != 2 || entries[2].ID != 1 {
		t.Fatalf("unexpected sort order: %+v", []int64{entries[0].ID, entries[1].ID, entries[2].ID})
	}
}

func TestModelInit_RefreshesInBackgroundWithDefaultPageSize(t *testing.T) {
	service := &initRefreshService{}
	m := NewModel(service, []feedbin.Entry{{ID: 1, Title: "Cached", PublishedAt: time.Now().UTC()}})

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected init refresh command")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	model := updated.(Model)

	if !service.called {
		t.Fatal("expected refresh to be called on init")
	}
	if service.page != 1 || service.perPage != 20 {
		t.Fatalf("unexpected refresh args page=%d perPage=%d", service.page, service.perPage)
	}
	if len(model.entries) == 0 || model.entries[0].ID != 100 {
		t.Fatalf("expected refreshed entries in model, got %+v", model.entries)
	}
	if !model.initialRefreshDone {
		t.Fatal("expected initial refresh metrics to be marked done")
	}
	if model.initialRefreshDuration <= 0 {
		t.Fatalf("expected initial refresh duration > 0, got %v", model.initialRefreshDuration)
	}
}

func TestModelMessagePanel_IncludesStartupMetrics(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, Title: "Cached", PublishedAt: time.Now().UTC()}})
	m.SetStartupCacheStats(150*time.Millisecond, 1)
	m.initialRefreshDone = true
	m.initialRefreshDuration = 700 * time.Millisecond

	view := m.View()
	if !strings.Contains(view, "Startup: cache 150ms (1 entries), initial refresh 700ms") {
		t.Fatalf("expected startup metrics in message panel, got: %s", view)
	}
}

func TestModelUpdate_RefreshError(t *testing.T) {
	m := NewModel(fakeRefresher{err: errors.New("network")}, nil)

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}

	msg := cmd()
	updatedModel, _ = updatedModel.Update(msg)
	finalModel := updatedModel.(Model)
	if finalModel.err == nil {
		t.Fatal("expected refresh error")
	}
}

func TestModelUpdate_RefreshWithUppercaseR(t *testing.T) {
	m := NewModel(fakeRefresher{err: errors.New("network")}, nil)

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}

	msg := cmd()
	updatedModel, _ = updatedModel.Update(msg)
	finalModel := updatedModel.(Model)
	if finalModel.err == nil {
		t.Fatal("expected refresh error")
	}
}

func TestModelUpdate_NavigateAndSelect(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{
		{ID: 1, Title: "First", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Second", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.selectedID != 2 {
		t.Fatalf("expected selected id 2, got %d", model.selectedID)
	}
	if !model.inDetail {
		t.Fatal("expected detail mode enabled after enter")
	}
}

func TestModelView_DetailAndBack(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "First Entry",
		FeedTitle:   "Feed A",
		URL:         "https://example.com/entry-1",
		Summary:     "Summary fallback",
		Content:     "<p>Content text <strong>with HTML</strong>.</p><p><img src=\"https://example.com/image.jpg\"/></p>",
		IsUnread:    true,
		IsStarred:   false,
		PublishedAt: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
	}})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	view := model.View()
	if !strings.Contains(view, "y: copy URL") {
		t.Fatalf("expected detail key hint, got: %s", view)
	}
	if !strings.Contains(view, "URL: https://example.com/entry-1") {
		t.Fatalf("expected detail URL, got: %s", view)
	}
	if !strings.Contains(view, "Content text with HTML.") {
		t.Fatalf("expected converted full content, got: %s", view)
	}
	if !strings.Contains(view, "Images:") || !strings.Contains(view, "https://example.com/image.jpg") {
		t.Fatalf("expected image URLs section, got: %s", view)
	}
	if !strings.Contains(view, "Mode: detail | Filter: all | Page: 1 | Showing: 1 | Last fetch: 0 | Time: relative | Open->Read: off | Confirm: off") {
		t.Fatalf("expected footer in detail view, got: %s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.inDetail {
		t.Fatal("expected back from detail view")
	}
}

func TestArticleTextFromEntry_FallsBackToSummary(t *testing.T) {
	entry := feedbin.Entry{
		Summary: "Only summary",
		Content: "",
	}
	got := articleTextFromEntry(entry)
	if got != "Only summary" {
		t.Fatalf("expected summary fallback, got %q", got)
	}
}

func TestImageURLsFromContent_OnlyHTTPAndDeduplicated(t *testing.T) {
	content := `<p><img src="https://example.com/a.jpg"><img src="https://example.com/a.jpg"><img src='http://example.com/b.png'><img src="data:image/png;base64,abc"></p>`
	got := imageURLsFromContent(content)
	if len(got) != 2 {
		t.Fatalf("expected 2 URLs, got %d (%+v)", len(got), got)
	}
	if got[0] != "https://example.com/a.jpg" || got[1] != "http://example.com/b.png" {
		t.Fatalf("unexpected image URLs: %+v", got)
	}
}

func TestWrapText_HardWrapLongWord(t *testing.T) {
	lines := wrapText("abcdefghij", 4)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d (%+v)", len(lines), lines)
	}
	if lines[0] != "abcd" || lines[1] != "efgh" || lines[2] != "ij" {
		t.Fatalf("unexpected wrapped lines: %+v", lines)
	}
}

func TestDetailScrollWithJK(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "Entry",
		Summary:     "one two three four five six seven eight nine ten eleven twelve",
		PublishedAt: time.Now().UTC(),
	}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 24, Height: 10})
	model := updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !model.inDetail {
		t.Fatal("expected detail mode")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	if model.detailTop == 0 {
		t.Fatal("expected detail to scroll down")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	if model.detailTop != 0 {
		t.Fatalf("expected detail to scroll back to top, got %d", model.detailTop)
	}
}

func TestModelUpdate_ToggleUnreadAction(t *testing.T) {
	m := NewModel(fakeRefresher{unreadResult: false}, []feedbin.Entry{{
		ID:          1,
		Title:       "Entry",
		IsUnread:    true,
		PublishedAt: time.Now().UTC(),
	}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})
	if cmd == nil {
		t.Fatal("expected unread command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.entries[0].IsUnread {
		t.Fatal("expected entry to be marked read")
	}
	if !strings.Contains(model.status, "Marked as read") {
		t.Fatalf("unexpected status: %s", model.status)
	}
}

func TestModelUpdate_ToggleStarredActionInDetail(t *testing.T) {
	m := NewModel(fakeRefresher{starResult: true}, []feedbin.Entry{{
		ID:          2,
		Title:       "Entry",
		IsStarred:   false,
		PublishedAt: time.Now().UTC(),
	}})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("expected starred command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model = updated.(Model)
	if !model.entries[0].IsStarred {
		t.Fatal("expected entry to be starred")
	}
	if !strings.Contains(model.status, "Starred entry") {
		t.Fatalf("unexpected status: %s", model.status)
	}
}

func TestModelUpdate_SwitchFilterUnread(t *testing.T) {
	m := NewModel(fakeRefresher{entries: []feedbin.Entry{
		{ID: 1, Title: "All", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Unread", IsUnread: true, PublishedAt: time.Now().UTC()},
	}}, nil)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected filter load command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.filter != "unread" {
		t.Fatalf("expected unread filter, got %s", model.filter)
	}
	if len(model.entries) != 1 || model.entries[0].ID != 2 {
		t.Fatalf("unexpected filtered entries: %+v", model.entries)
	}
}

func TestModelUpdate_ToggleUnreadFilterBackToAll(t *testing.T) {
	m := NewModel(fakeRefresher{entries: []feedbin.Entry{
		{ID: 1, Title: "All", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Unread", IsUnread: true, PublishedAt: time.Now().UTC()},
	}}, nil)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected unread filter command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.filter != "unread" {
		t.Fatalf("expected unread filter, got %s", model.filter)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected toggle back to all command")
	}
	msg = cmd()
	updated, _ = updated.Update(msg)
	model = updated.(Model)
	if model.filter != "all" {
		t.Fatalf("expected all filter after toggle, got %s", model.filter)
	}
}

func TestModelUpdate_ToggleStarredFilterBackToAll(t *testing.T) {
	m := NewModel(fakeRefresher{entries: []feedbin.Entry{
		{ID: 1, Title: "All", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Star", IsStarred: true, PublishedAt: time.Now().UTC()},
	}}, nil)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}})
	if cmd == nil {
		t.Fatal("expected starred filter command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.filter != "starred" {
		t.Fatalf("expected starred filter, got %s", model.filter)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}})
	if cmd == nil {
		t.Fatal("expected toggle back to all command")
	}
	msg = cmd()
	updated, _ = updated.Update(msg)
	model = updated.(Model)
	if model.filter != "all" {
		t.Fatalf("expected all filter after toggle, got %s", model.filter)
	}
}

func TestModelUpdate_LoadMore(t *testing.T) {
	m := NewModel(fakeRefresher{pageResults: map[int][]feedbin.Entry{
		2: {
			{ID: 1, Title: "First", PublishedAt: time.Now().UTC()},
			{ID: 2, Title: "Second", PublishedAt: time.Now().UTC()},
		},
	}}, []feedbin.Entry{
		{ID: 1, Title: "First", PublishedAt: time.Now().UTC()},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected load more command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.page != 2 {
		t.Fatalf("expected page 2, got %d", model.page)
	}
	if len(model.entries) != 2 {
		t.Fatalf("expected 2 entries after load more, got %d", len(model.entries))
	}
}

func TestModelUpdate_LoadMoreNoMoreEntries(t *testing.T) {
	m := NewModel(fakeRefresher{pageResults: map[int][]feedbin.Entry{
		2: {},
	}}, []feedbin.Entry{
		{ID: 1, Title: "First", PublishedAt: time.Now().UTC()},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected load more command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.page != 1 {
		t.Fatalf("expected page to stay at 1, got %d", model.page)
	}
	if !strings.Contains(model.status, "No more entries") {
		t.Fatalf("unexpected status: %s", model.status)
	}
}

func TestModelUpdate_LoadMoreKeepsFilterAndSelection(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Read", IsUnread: false, PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Unread A", IsUnread: true, PublishedAt: time.Now().UTC()},
	}
	page2Filtered := []feedbin.Entry{
		{ID: 2, Title: "Unread A", IsUnread: true, PublishedAt: time.Now().UTC()},
		{ID: 3, Title: "Unread B", IsUnread: true, PublishedAt: time.Now().UTC()},
	}
	m := NewModel(fakeRefresher{entries: entries, pageResults: map[int][]feedbin.Entry{2: page2Filtered}}, entries)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected unread filter command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	model.selectedID = 2

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected load more command")
	}
	msg = cmd()
	updated, _ = updated.Update(msg)
	model = updated.(Model)

	if model.filter != "unread" {
		t.Fatalf("expected unread filter, got %s", model.filter)
	}
	if model.selectedID != 2 {
		t.Fatalf("expected selected id 2 to remain, got %d", model.selectedID)
	}
	if len(model.entries) != 2 {
		t.Fatalf("expected 2 unread entries after load more, got %d", len(model.entries))
	}
}

func TestModelUpdate_PreserveSelectionAcrossFilter(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "First", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Unread", IsUnread: true, PublishedAt: time.Now().UTC()},
	}
	m := NewModel(fakeRefresher{entries: entries}, entries)
	m.cursor = 1
	m.selectedID = 2

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected filter command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if model.selectedID != 2 {
		t.Fatalf("expected selected id 2, got %d", model.selectedID)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor moved to selected filtered item, got %d", model.cursor)
	}
}

func TestModelUpdate_OpenURLFallbackToCopy(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, URL: "https://example.com", PublishedAt: time.Now().UTC()}})
	m.inDetail = true
	m.openURLFn = func(string) error { return errors.New("open failed") }
	m.copyURLFn = func(string) error { return nil }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected open URL command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if !strings.Contains(model.status, "copied to clipboard") {
		t.Fatalf("expected copy fallback status, got %s", model.status)
	}
}

func TestModelUpdate_CopyURLDirectly(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, URL: "https://example.com", PublishedAt: time.Now().UTC()}})
	m.copyURLFn = func(string) error { return nil }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected copy URL command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	if !strings.Contains(model.status, "URL copied to clipboard") {
		t.Fatalf("unexpected status: %s", model.status)
	}
}

func TestModelUpdate_CopyURLInvalidScheme(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, URL: "ftp://example.com", PublishedAt: time.Now().UTC()}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected status clear command for invalid URL")
	}
	model := updated.(Model)
	if !strings.Contains(model.status, "unsupported URL scheme") {
		t.Fatalf("unexpected status: %s", model.status)
	}
}

func TestModelRenderEntryLine_DateRightAlignedInList(t *testing.T) {
	now := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	m := NewModel(nil, []feedbin.Entry{
		{
			ID:          1,
			Title:       "A very long article title that should be truncated to keep date aligned",
			PublishedAt: now.Add(-2 * time.Hour),
			IsUnread:    true,
		},
	})
	m.width = 60
	m.compact = false
	m.nowFn = func() time.Time { return now }

	line := m.renderEntryLine(0, 0, false)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(line, "")
	if !strings.HasSuffix(plain, "[2 hours ago]") {
		t.Fatalf("expected date suffix at right edge, got %q", plain)
	}
	if got := len([]rune(plain)); got != m.contentWidth() {
		t.Fatalf("expected visible line width %d, got %d (%q)", m.contentWidth(), got, plain)
	}
}

func TestModelRenderEntryLine_AbsoluteDateWhenRelativeDisabled(t *testing.T) {
	now := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	m := NewModel(nil, []feedbin.Entry{
		{
			ID:          1,
			Title:       "Absolute date rendering",
			PublishedAt: now.Add(-2 * time.Hour),
			IsUnread:    true,
		},
	})
	m.width = 60
	m.compact = false
	m.nowFn = func() time.Time { return now }
	m.relativeTime = false

	line := m.renderEntryLine(0, 0, false)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(line, "")
	if !strings.HasSuffix(plain, "[2026-02-09]") {
		t.Fatalf("expected absolute date suffix at right edge, got %q", plain)
	}
}

func TestRelativeTimeLabel(t *testing.T) {
	now := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		then time.Time
		want string
	}{
		{then: now.Add(-30 * time.Second), want: "just now"},
		{then: now.Add(-1 * time.Minute), want: "1 minute ago"},
		{then: now.Add(-3 * time.Minute), want: "3 minutes ago"},
		{then: now.Add(-1 * time.Hour), want: "1 hour ago"},
		{then: now.Add(-7 * time.Hour), want: "7 hours ago"},
		{then: now.Add(-1 * 24 * time.Hour), want: "1 day ago"},
		{then: now.Add(-7 * 24 * time.Hour), want: "7 days ago"},
	}
	for _, tc := range cases {
		if got := relativeTimeLabel(now, tc.then); got != tc.want {
			t.Fatalf("relativeTimeLabel(%s) = %q, want %q", tc.then.UTC().Format(time.RFC3339), got, tc.want)
		}
	}
}

func TestModelUpdate_ListNavigationExtras(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", PublishedAt: time.Now().UTC()},
		{ID: 3, Title: "Three", PublishedAt: time.Now().UTC()},
	}
	m := NewModel(nil, entries)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	model = updated.(Model)
	lastVisible := model.visibleEntryIndices()
	if len(lastVisible) == 0 || model.cursor != lastVisible[len(lastVisible)-1] {
		t.Fatalf("expected cursor at last visible entry, got cursor=%d visible=%+v", model.cursor, lastVisible)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	model = updated.(Model)
	if model.cursor != 0 {
		t.Fatalf("expected cursor at top, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	if model.cursor == 0 {
		t.Fatal("expected page down to move cursor")
	}
}

func TestModelUpdate_CollapseExpandTreeWithHL(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Feed A", URL: "https://example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Feed A", URL: "https://example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)
	m.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model := updated.(Model)
	if len(model.visibleEntryIndices()) != 0 {
		t.Fatalf("expected feed to collapse and hide entries, visible=%d", len(model.visibleEntryIndices()))
	}
	if !strings.Contains(model.status, "Collapsed feed") {
		t.Fatalf("expected collapsed status, got %q", model.status)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	if len(model.visibleEntryIndices()) != 2 {
		t.Fatalf("expected entries visible after expand, got %d", len(model.visibleEntryIndices()))
	}
	if !strings.Contains(model.status, "Expanded feed") {
		t.Fatalf("expected expanded status, got %q", model.status)
	}
}

func TestModelUpdate_CollapseWithHMovesCursorToParents(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Race", FeedFolder: "Formula 1", URL: "https://example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Race", FeedFolder: "Formula 1", URL: "https://example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model := updated.(Model)
	rows := model.treeRows()
	if rows[model.treeCursor].Kind != treeRowFeed || rows[model.treeCursor].Feed != "Race" || rows[model.treeCursor].Folder != "Formula 1" {
		t.Fatalf("expected cursor on feed row after first collapse, got kind=%s folder=%q feed=%q", rows[model.treeCursor].Kind, rows[model.treeCursor].Folder, rows[model.treeCursor].Feed)
	}
	if !model.collapsedFeeds[treeFeedKey("Formula 1", "Race")] {
		t.Fatal("expected feed collapsed after first h")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(Model)
	rows = model.treeRows()
	if rows[model.treeCursor].Kind != treeRowFolder || rows[model.treeCursor].Folder != "Formula 1" {
		t.Fatalf("expected cursor on folder row after second collapse, got kind=%s folder=%q", rows[model.treeCursor].Kind, rows[model.treeCursor].Folder)
	}
	if !model.collapsedFolders["Formula 1"] {
		t.Fatal("expected folder collapsed after second h")
	}
}

func TestModelUpdate_CollapseTopFeedWithHMovesCursorToFeed(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Top Feed", URL: "https://example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Top Feed", URL: "https://example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model := updated.(Model)
	rows := model.treeRows()
	if rows[model.treeCursor].Kind != treeRowFeed || rows[model.treeCursor].Feed != "Top Feed" || rows[model.treeCursor].Folder != "" {
		t.Fatalf("expected cursor on top-feed row after collapse, got kind=%s folder=%q feed=%q", rows[model.treeCursor].Kind, rows[model.treeCursor].Folder, rows[model.treeCursor].Feed)
	}
	if !model.collapsedFeeds[treeFeedKey("", "Top Feed")] {
		t.Fatal("expected top-level feed collapsed")
	}
}

func TestModelUpdate_ExpandWithLMovesCursorToChildren(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Race", FeedFolder: "Formula 1", URL: "https://example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Race", FeedFolder: "Formula 1", URL: "https://example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)
	m.collapsedFolders["Formula 1"] = true
	m.ensureCursorVisible()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model := updated.(Model)
	rows := model.treeRows()
	if rows[model.treeCursor].Kind != treeRowFeed || rows[model.treeCursor].Feed != "Race" || rows[model.treeCursor].Folder != "Formula 1" {
		t.Fatalf("expected cursor on feed row after folder expand, got kind=%s folder=%q feed=%q", rows[model.treeCursor].Kind, rows[model.treeCursor].Folder, rows[model.treeCursor].Feed)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(Model)
	rows = model.treeRows()
	if rows[model.treeCursor].Kind != treeRowArticle || rows[model.treeCursor].Feed != "Race" || rows[model.treeCursor].Folder != "Formula 1" {
		t.Fatalf("expected cursor on article row after feed descend, got kind=%s folder=%q feed=%q", rows[model.treeCursor].Kind, rows[model.treeCursor].Folder, rows[model.treeCursor].Feed)
	}
}

func TestModelUpdate_ExpandTopFeedWithLMovesCursorToArticle(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Top Feed", URL: "https://example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Top Feed", URL: "https://example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)
	m.collapsedFeeds[treeFeedKey("", "Top Feed")] = true
	m.ensureCursorVisible()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model := updated.(Model)
	rows := model.treeRows()
	if rows[model.treeCursor].Kind != treeRowArticle || rows[model.treeCursor].Feed != "Top Feed" || rows[model.treeCursor].Folder != "" {
		t.Fatalf("expected cursor on top-feed article after expand, got kind=%s folder=%q feed=%q", rows[model.treeCursor].Kind, rows[model.treeCursor].Folder, rows[model.treeCursor].Feed)
	}
}

func TestModelUpdate_CursorMovesAcrossVisibleEntries(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Feed A", URL: "https://example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Feed A", URL: "https://example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
		{ID: 3, Title: "Three", FeedTitle: "Feed B", URL: "https://example.com/3", PublishedAt: time.Now().UTC().Add(-2 * time.Minute)},
	}
	m := NewModel(nil, entries)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	model := updated.(Model)
	lastVisible := model.visibleEntryIndices()
	if len(lastVisible) == 0 || model.cursor != lastVisible[len(lastVisible)-1] {
		t.Fatalf("expected cursor on last visible entry, got cursor=%d visible=%+v", model.cursor, lastVisible)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	model = updated.(Model)
	firstVisible := model.visibleEntryIndices()
	if len(firstVisible) == 0 || model.cursor != firstVisible[0] {
		t.Fatalf("expected cursor on first visible entry, got cursor=%d visible=%+v", model.cursor, firstVisible)
	}
}

func TestModelUpdate_ExpandCanRecoverAllCollapsedFolders(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Feed A", FeedFolder: "Formula 1", URL: "https://a.example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", FeedTitle: "Feed B", FeedFolder: "Motorsport", URL: "https://b.example.com/2", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)

	m.collapsedFolders["Formula 1"] = true
	m.collapsedFolders["Motorsport"] = true

	if len(m.visibleEntryIndices()) != 0 {
		t.Fatalf("expected no visible entries, got %d", len(m.visibleEntryIndices()))
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := updated.(Model)
	if len(model.visibleEntryIndices()) == 0 {
		t.Fatal("expected at least one folder expanded after first right")
	}

	for i := 0; i < 4 && (model.collapsedFolders["Formula 1"] || model.collapsedFolders["Motorsport"]); i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
		model = updated.(Model)
	}
	if model.collapsedFolders["Formula 1"] || model.collapsedFolders["Motorsport"] {
		t.Fatalf("expected both folders expanded, got collapsed=%+v", model.collapsedFolders)
	}
}

func TestModelView_TopCollectionsStayVisibleWhenCollapsed(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Folder entry", FeedTitle: "Feed A", FeedFolder: "Formula 1", URL: "https://folder.example.com/1", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Top feed entry", FeedTitle: "Lone Feed", URL: "", PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)
	m.collapsedFolders["Formula 1"] = true
	m.collapsedFeeds[treeFeedKey("", "Lone Feed")] = true

	view := m.View()
	if !strings.Contains(view, "▸ Formula 1") {
		t.Fatalf("expected collapsed folder collection header visible, got: %s", view)
	}
	if !strings.Contains(view, "▸ Lone Feed") {
		t.Fatalf("expected collapsed top-level feed header visible, got: %s", view)
	}
}

func TestModelUpdate_CollectionsAreNavigableAndHighlighted(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Article A", FeedTitle: "Feed A", FeedFolder: "Formula 1", URL: "https://example.com/a", PublishedAt: time.Now().UTC()},
	}
	m := NewModel(nil, entries)

	// Move from first article row up to feed row, then folder row.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(Model)
	view := model.View()
	if !strings.Contains(view, "\x1b[7m  ▾ Feed A\x1b[0m") {
		t.Fatalf("expected feed row to be highlighted, got: %s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	view = model.View()
	if !strings.Contains(view, "\x1b[7m▾ Formula 1\x1b[0m") {
		t.Fatalf("expected folder row to be highlighted, got: %s", view)
	}
}

func TestTreeRows_CollectionsAlphabeticalRegardlessOfStatus(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Read alpha", FeedTitle: "Alpha Feed", FeedFolder: "Alpha", IsUnread: false, PublishedAt: time.Now().UTC().Add(-4 * time.Minute)},
		{ID: 2, Title: "Unread zoo", FeedTitle: "Zoo Feed", FeedFolder: "Zoo", IsUnread: true, PublishedAt: time.Now().UTC().Add(-3 * time.Minute)},
		{ID: 3, Title: "Unread beta", FeedTitle: "Beta", IsUnread: true, PublishedAt: time.Now().UTC().Add(-2 * time.Minute)},
		{ID: 4, Title: "Read delta", FeedTitle: "Delta", IsUnread: false, PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)

	rows := m.treeRows()
	top := make([]string, 0, 4)
	for _, row := range rows {
		if row.Kind == treeRowFolder {
			top = append(top, row.Label)
			continue
		}
		if row.Kind == treeRowFeed && row.Folder == "" {
			top = append(top, row.Label)
		}
	}
	expected := []string{"Alpha", "Beta", "Delta", "Zoo"}
	if len(top) != len(expected) {
		t.Fatalf("expected %d top collections, got %d (%v)", len(expected), len(top), top)
	}
	for i := range expected {
		if top[i] != expected[i] {
			t.Fatalf("unexpected top collection order at %d: got %q want %q (all=%v)", i, top[i], expected[i], top)
		}
	}
}

func TestTreeRows_FolderFeedsAlphabeticalRegardlessOfStatus(t *testing.T) {
	entries := []feedbin.Entry{
		{ID: 1, Title: "Read race", FeedTitle: "Race", FeedFolder: "Formula 1", IsUnread: false, PublishedAt: time.Now().UTC().Add(-3 * time.Minute)},
		{ID: 2, Title: "Unread autosport", FeedTitle: "Autosport", FeedFolder: "Formula 1", IsUnread: true, PublishedAt: time.Now().UTC().Add(-2 * time.Minute)},
		{ID: 3, Title: "Unread boxbox", FeedTitle: "Boxbox", FeedFolder: "Formula 1", IsUnread: true, PublishedAt: time.Now().UTC().Add(-time.Minute)},
	}
	m := NewModel(nil, entries)

	rows := m.treeRows()
	feeds := make([]string, 0, 3)
	for _, row := range rows {
		if row.Kind == treeRowFeed && row.Folder == "Formula 1" {
			feeds = append(feeds, row.Label)
		}
	}
	expected := []string{"Autosport", "Boxbox", "Race"}
	if len(feeds) != len(expected) {
		t.Fatalf("expected %d feeds, got %d (%v)", len(expected), len(feeds), feeds)
	}
	for i := range expected {
		if feeds[i] != expected[i] {
			t.Fatalf("unexpected feed order at %d: got %q want %q (all=%v)", i, feeds[i], expected[i], feeds)
		}
	}
}

func TestModelUpdate_CompactAndMarkReadOnOpenToggles(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, Title: "One", PublishedAt: time.Now().UTC()}})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)
	if !model.compact {
		t.Fatal("expected compact mode on")
	}
	if !strings.Contains(model.status, "Compact mode: on") {
		t.Fatalf("unexpected status: %s", model.status)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = updated.(Model)
	if !model.markReadOnOpen {
		t.Fatal("expected mark-read-on-open on")
	}
	if !strings.Contains(model.footer(), "Open->Read: on") {
		t.Fatalf("unexpected footer: %s", model.footer())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(Model)
	if !model.confirmOpenRead {
		t.Fatal("expected confirm mode on")
	}
	if !strings.Contains(model.footer(), "Confirm: on") {
		t.Fatalf("unexpected footer: %s", model.footer())
	}
}

func TestModelUpdate_HelpToggle(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, Title: "One", PublishedAt: time.Now().UTC()}})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model := updated.(Model)
	if !model.showHelp {
		t.Fatal("expected help mode on")
	}
	view := model.View()
	if !strings.Contains(view, "Help (? to close)") {
		t.Fatalf("expected help view, got: %s", view)
	}
}

func TestModelUpdate_DetailPrevNext(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{
		{ID: 1, Title: "One", FeedTitle: "Feed A", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Two", FeedTitle: "Feed A", PublishedAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	model = updated.(Model)
	if model.cursor != 0 || model.selectedID != 2 {
		t.Fatalf("expected previous entry selected, cursor=%d selected=%d", model.cursor, model.selectedID)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	model = updated.(Model)
	if model.cursor != 1 || model.selectedID != 1 {
		t.Fatalf("expected next entry selected, cursor=%d selected=%d", model.cursor, model.selectedID)
	}
}

func TestModelUpdate_OpenWithConfirmMarkRead(t *testing.T) {
	m := NewModel(fakeRefresher{unreadResult: false}, []feedbin.Entry{{
		ID:          1,
		Title:       "Entry",
		URL:         "https://example.com",
		IsUnread:    true,
		PublishedAt: time.Now().UTC(),
	}})
	m.confirmOpenRead = true
	m.markReadOnOpen = true
	m.inDetail = true
	m.openURLFn = func(string) error { return nil }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected open command")
	}
	msg := cmd()
	updated, cmd = updated.Update(msg)
	model := updated.(Model)
	if model.pendingOpenReadEntryID != 1 {
		t.Fatalf("expected pending mark read for 1, got %d", model.pendingOpenReadEntryID)
	}
	if cmd == nil {
		// clear status tick is expected
		t.Fatal("expected clear status command")
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}})
	if cmd == nil {
		t.Fatal("expected confirm toggle command")
	}
	msg = cmd()
	updated, _ = updated.Update(msg)
	model = updated.(Model)
	if model.entries[0].IsUnread {
		t.Fatal("expected entry marked read after confirm")
	}
}

func TestModelUpdate_OpenDebounceSkipsSecondMarkRead(t *testing.T) {
	service := &openWorkflowService{unreadResult: false}
	m := NewModel(service, []feedbin.Entry{{
		ID:          1,
		Title:       "Entry",
		URL:         "https://example.com",
		IsUnread:    true,
		PublishedAt: time.Now().UTC(),
	}})
	m.markReadOnOpen = true
	m.inDetail = true
	m.openURLFn = func(string) error { return nil }
	now := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	m.nowFn = func() time.Time { return now }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected open command")
	}
	msg := cmd()
	updated, cmd = updated.Update(msg)
	if cmd == nil {
		t.Fatal("expected mark-read command")
	}
	msg = cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)

	if service.unreadCalls != 1 {
		t.Fatalf("expected one unread toggle call after first open, got %d", service.unreadCalls)
	}
	if model.entries[0].IsUnread {
		t.Fatal("expected entry read after first open")
	}

	// Simulate feed state still reporting unread and reopen quickly.
	model.entries[0].IsUnread = true
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected second open command")
	}
	msg = cmd()
	updated, cmd = updated.Update(msg)
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected status clear command after debounce")
	}
	if service.unreadCalls != 1 {
		t.Fatalf("expected unread toggle to be debounced, got %d calls", service.unreadCalls)
	}
	if !strings.Contains(model.status, "debounced") {
		t.Fatalf("expected debounced status, got %q", model.status)
	}
}

func TestModelUpdate_PreferenceTogglesPersist(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{ID: 1, Title: "One", PublishedAt: time.Now().UTC()}})

	saved := make([]Preferences, 0, 4)
	m.SetPreferencesSaver(func(p Preferences) error {
		saved = append(saved, p)
		return nil
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("expected preference save command after compact toggle")
	}
	_ = cmd()
	model := updated.(Model)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if cmd == nil {
		t.Fatal("expected preference save command after mark-read toggle")
	}
	_ = cmd()
	model = updated.(Model)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("expected preference save command after confirm toggle")
	}
	_ = cmd()
	model = updated.(Model)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("expected preference save command after time-format toggle")
	}
	_ = cmd()

	if len(saved) != 4 {
		t.Fatalf("expected 4 persisted preference snapshots, got %d", len(saved))
	}
	if !saved[0].Compact {
		t.Fatalf("expected compact true after first save, got %+v", saved[0])
	}
	if !saved[1].MarkReadOnOpen {
		t.Fatalf("expected mark-read-on-open true after second save, got %+v", saved[1])
	}
	if !saved[2].ConfirmOpenRead {
		t.Fatalf("expected confirm true after third save, got %+v", saved[2])
	}
	if saved[3].RelativeTime {
		t.Fatalf("expected relative-time false after fourth save, got %+v", saved[3])
	}
}

func TestModelUpdate_InlineImagePreviewSuccess(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "Entry",
		Content:     `<p>Hello</p><img src="https://example.com/image.png">`,
		PublishedAt: time.Now().UTC(),
	}})
	m.renderImageFn = func(string, int) (string, error) {
		return "PREVIEW-ART", nil
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected inline image preview command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Inline image preview:") {
		t.Fatalf("expected inline preview header, got %s", view)
	}
	if !strings.Contains(view, "PREVIEW-ART") {
		t.Fatalf("expected rendered preview content, got %s", view)
	}
}

func TestModelUpdate_InlineImagePreviewError(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "Entry",
		Content:     `<img src="https://example.com/image.png">`,
		PublishedAt: time.Now().UTC(),
	}})
	m.renderImageFn = func(string, int) (string, error) {
		return "", errors.New("renderer unavailable")
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected inline image preview command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)
	model := updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Inline image preview unavailable") {
		t.Fatalf("expected inline preview error in detail view, got %s", view)
	}
}

func TestStyleArticleTitle_ByState(t *testing.T) {
	unread := styleArticleTitle(feedbin.Entry{IsUnread: true}, "Unread")
	if !strings.Contains(unread, "\x1b[1m") {
		t.Fatalf("expected unread title to be bold, got %q", unread)
	}

	starredRead := styleArticleTitle(feedbin.Entry{IsStarred: true}, "Starred")
	if !strings.Contains(starredRead, "\x1b[3;90m") {
		t.Fatalf("expected starred read title to be italic grey, got %q", starredRead)
	}

	read := styleArticleTitle(feedbin.Entry{}, "Read")
	if !strings.Contains(read, "\x1b[90m") {
		t.Fatalf("expected read title to be grey, got %q", read)
	}

	unreadStarred := styleArticleTitle(feedbin.Entry{IsUnread: true, IsStarred: true}, "Both")
	if !strings.Contains(unreadStarred, "\x1b[1;3m") {
		t.Fatalf("expected unread starred title to be bold italic, got %q", unreadStarred)
	}
}
