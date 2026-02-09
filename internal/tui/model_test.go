package tui

import (
	"context"
	"errors"
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

func TestModelView_ShowsEntriesWithMetadata(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "First Entry",
		FeedTitle:   "Feed A",
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
	if !strings.Contains(view, "[U] [*]") {
		t.Fatalf("expected state markers in view, got: %s", view)
	}
	if !strings.Contains(view, "> ") {
		t.Fatalf("expected cursor marker in view, got: %s", view)
	}
	if !strings.Contains(view, "Mode: list | Filter: all | Page: 1 | Showing: 1 | Last fetch: 0 | Open->Read: off | Confirm: off") {
		t.Fatalf("expected footer in list view, got: %s", view)
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

func TestModelUpdate_NavigateAndSelect(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{
		{ID: 1, Title: "First", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Second", PublishedAt: time.Now().UTC()},
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
		Summary:     "Summary text",
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
	if !strings.Contains(view, "Summary text") {
		t.Fatalf("expected detail summary, got: %s", view)
	}
	if !strings.Contains(view, "Mode: detail | Filter: all | Page: 1 | Showing: 1 | Last fetch: 0 | Open->Read: off | Confirm: off") {
		t.Fatalf("expected footer in detail view, got: %s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.inDetail {
		t.Fatal("expected back from detail view")
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

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
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

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
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
	if model.cursor != 2 {
		t.Fatalf("expected cursor at bottom, got %d", model.cursor)
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
		{ID: 1, Title: "One", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Two", PublishedAt: time.Now().UTC()},
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	model := updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	model = updated.(Model)
	if model.cursor != 0 || model.selectedID != 1 {
		t.Fatalf("expected previous entry selected, cursor=%d selected=%d", model.cursor, model.selectedID)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	model = updated.(Model)
	if model.cursor != 1 || model.selectedID != 2 {
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
